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
	//CompareResponse string `long:"compare-response" description:"Load all response bodies and compare between endpoints, will affect throughput." default:"on"`

	//Source string `long:"source" description:"Where requests come from (options: stdin-post, stdin-get)" default:"stdin-jsons"` // Someday: stdin-tcpdump, file://foo.json, ws://remote-endpoint

	// TODO: Specify additional headers/configs per-endpoint (e.g. auth headers)
	// TODO: Periodic reporting for long-running tests?
	// TODO: Toggle compare results? Could probably reach higher throughput without result comparison.
	// TODO: Add latency offcheck set before starting

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
		exit(2, "error during run: %s\n", err)
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

	if options.Concurrency < 1 {
		logger.Info().Int("concurrency", options.Concurrency).Msg("concurrency is less than 1, overriding to 1")
		options.Concurrency = 1
	}

	g, ctx := errgroup.WithContext(ctx)

	respBuffer := options.Concurrency * 4
	if respBuffer < 50 {
		respBuffer = 50
	}
	// responses is closed when clients are shut down
	responses := make(chan Response, respBuffer)

	// Launch clients
	clients, err := NewClients(options.Args.Endpoints, options.Concurrency, timeout)
	if err != nil {
		return fmt.Errorf("failed to create clients: %w", err)
	}

	r := report{Clients: clients}
	g.Go(func() error {
		return r.Serve(ctx, responses)
	})
	if len(options.Verbose) > 0 {
		r.MismatchedResponse = func(resps []Response) {
			logger.Info().Int("id", int(resps[0].ID)).Msgf("mismatched responses: %s", Responses(resps).String())
		}
	}

	g.Go(func() error {
		defer close(responses)
		return clients.Serve(ctx, responses)
	})

	logger.Info().Int("clients", len(clients)).Msg("started endpoint clients, waiting for stdin")

	g.Go(func() error {
		return pump(ctx, os.Stdin, clients, stopAfter)
	})

	if err := g.Wait(); err == context.Canceled {
		// Shutting down
	} else if err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	// Report
	return r.Render(os.Stdout)
}

// pump takes lines from a reader and pumps them into the clients
func pump(ctx context.Context, r io.Reader, clients Clients, stopAfter int) error {
	defer clients.Finalize()

	scanner := bufio.NewScanner(r)
	// Some lines are really long, let's allocate a big fat megabyte for lines.
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, cap(buf))

	n := 0
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 { // Done
			logger.Debug().Msg("reached end of feed")
			return nil
		}
		if err := clients.Send(ctx, line); err != nil {
			return err
		}
		n += 1

		if stopAfter > 0 && n >= stopAfter {
			logger.Info().Msgf("stopping request feed after %d requests", n)
			return nil
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}
