package main // import "github.com/INFURA/versus"

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"time"

	flags "github.com/jessevdk/go-flags"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

// Version of the binary, assigned during build.
var Version string = "dev"

// Options contains the flag options
type Options struct {
	Args struct {
		Endpoints []string `positional-arg-name:"endpoint" description:"API endpoint to load test, such as \"http://localhost:8080/\""`
	} `positional-args:"yes"`

	Timeout     string `long:"timeout" description:"Abort request after duration" default:"30s"`
	StopAfter   string `long:"stop-after" description:"Stop after N requests per endpoint, N can be a number or duration."`
	Concurrency int    `long:"concurrency" description:"Concurrent requests per endpoint" default:"1"`

	//Source      string `long:"source" description:"Where to get requests from (options: stdin-jsons, ethspam)" default:"stdin-jsons"` // Someday: stdin-tcpdump, file://foo.json, ws://remote-endpoint

	Verbose []bool `long:"verbose" short:"v" description:"Show verbose logging."`
	Version bool   `long:"version" description:"Print version and exit."`
}

func exit(code int, format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(code)
}

func main() {
	options := Options{}
	p, err := flags.NewParser(&options, flags.Default).ParseArgs(os.Args[1:])
	if err != nil {
		if p == nil {
			fmt.Println(err)
		}
		return
	}

	if options.Version {
		fmt.Println(Version)
		os.Exit(0)
	}

	if len(options.Args.Endpoints) == 0 {
		exit(1, "must specify at least one endpoint\n")
	}

	switch len(options.Verbose) {
	case 0:
		logger = logger.Level(zerolog.WarnLevel)
	case 1:
		logger = logger.Level(zerolog.InfoLevel)
	default:
		logger = logger.Level(zerolog.DebugLevel)
	}

	// Setup signals
	ctx, abort := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func(abort context.CancelFunc) {
		<-sigCh
		logger.Warn().Msg("interrupt received, shutting down")
		abort()
		<-sigCh
		logger.Error().Msg("second interrupt received, panicking")
		panic("aborted")
	}(abort)

	if err := run(ctx, options); err != nil {
		exit(2, "error during run: %s", err)
	}
}

func parseStopAfter(s string) (time.Duration, int, error) {
	n, err := strconv.ParseUint(s, 10, 32)
	if err == nil {
		return 0, int(n), nil
	}

	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse duration: %w", err)
	}

	return d, 0, nil
}

func run(ctx context.Context, options Options) error {
	var stopAfter int
	if options.StopAfter != "" {
		d, n, err := parseStopAfter(options.StopAfter)
		if err != nil {
			return err
		}
		if d > 0 {
			timeoutCtx, cancel := context.WithTimeout(ctx, d)
			defer cancel()
			ctx = timeoutCtx
		}
		stopAfter = n
	}

	var timeout time.Duration
	if options.Timeout != "" {
		d, err := time.ParseDuration(options.Timeout)
		if err != nil {
			return fmt.Errorf("failed to parse request timeout: %w", err)
		}
		timeout = d
	}

	ctx, done := context.WithCancel(ctx)
	defer done()

	g, ctx := errgroup.WithContext(ctx)

	// Launch clients
	clients, err := NewClients(options.Args.Endpoints, options.Concurrency, timeout)
	if err != nil {
		return fmt.Errorf("failed to create clients: %w", err)
	}

	r, err := Report(clients)
	if err != nil {
		return fmt.Errorf("failed to create report: %w", err)
	}

	g.Go(func() error {
		return r.Serve(ctx)
	})

	if len(options.Verbose) > 0 {
		r.MismatchedResponse = func(resps []Response) {
			logger.Info().Msgf("mismatched responses: %v", resps)
		}
	}

	g.Go(func() error {
		return clients.Serve(ctx)
	})

	logger.Info().Int("clients", len(clients)).Msg("started endpoint clients, pumping stdin")

	g.Go(func() error {
		return pump(ctx, os.Stdin, clients, stopAfter)
	})

	if err := g.Wait(); err == context.Canceled {
		// Shutting down
	} else if err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	// TODO: Make a report
	for i, client := range clients {
		fmt.Printf("endpoint[%d]: %+v\n", i, client.Stats)
	}

	fmt.Printf("report: %+v\n", r)
	return nil
}

// pump takes lines from a reader and pumps them into the clients
func pump(ctx context.Context, r io.Reader, c Clients, stopAfter int) error {
	scanner := bufio.NewScanner(r)
	n := 0
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 { // Done
			return nil
		}

		if err := c.Send(line); err != nil {
			return err
		}
		n += 1

		if stopAfter > 0 && n >= stopAfter {
			logger.Info().Msgf("stopping after %d requests", n)
			c.Finalize()
			return nil
		}
	}

	c.Finalize()

	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}
