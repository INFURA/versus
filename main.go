package main // import "github.com/INFURA/versus"

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	flags "github.com/jessevdk/go-flags"
	"golang.org/x/sync/errgroup"
)

// Version of the binary, assigned during build.
var Version string = "dev"

// Options contains the flag options
type Options struct {
	Endpoints   []string `positional-args:"yes"`
	Duration    string   `long:"duration" description:"Stop after duration (example: 60s)"`
	Requests    int      `long:"requests" description:"Stop after N requests per endpoint"`
	Concurrency int      `long:"concurrency" description:"Concurrent requests per endpoint" default:"1"`
	Source      string   `long:"source" description:"Where to get requests from (options: stdin-jsons, ethspam)" default:"stdin-jsons"` // Someday: stdin-tcpdump, file://foo.json, ws://remote-endpoint

	Version bool `long:"version" description:"Print version and exit."`
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

	// Setup signals
	ctx, abort := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func(abort context.CancelFunc) {
		<-sigCh
		abort()
		<-sigCh
		panic("aborted")
	}(abort)

	if options.Duration != "" {
		d, err := time.ParseDuration(options.Duration)
		if err != nil {
			exit(1, "failed to parse duration: %s", err)
		}
		if d > 0 {
			timeoutCtx, cancel := context.WithTimeout(ctx, d)
			defer cancel()
			ctx = timeoutCtx
		}
	}

	g, ctx := errgroup.WithContext(ctx)

	// Launch clients
	clients := make(Clients, 0, len(options.Endpoints))
	for _, endpoint := range options.Endpoints {
		c := Client{
			Endpoint:    endpoint,
			Concurrency: options.Concurrency,
			In:          make(chan Request, chanBuffer),
			Out:         make(chan Response, chanBuffer),
		}
		clients = append(clients, c)
		g.Go(func() error {
			return c.Serve(ctx)
		})
	}

	g.Go(func() error {
		return pump(ctx, os.Stdin, clients)
	})

	if err := g.Wait(); err != nil {
		exit(3, "failed: %s", err)
	}

	// TODO: Make a report
	for _, client := range clients {
		fmt.Println(client.Stats)
	}
}

// pump takes lines from a reader and pumps them into the clients
func pump(ctx context.Context, r io.Reader, c Clients) error {
	scanner := bufio.NewScanner(r)
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
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}
