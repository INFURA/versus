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

	v := versus{options.Endpoints, options.Concurrency}
	if err := v.Serve(ctx, os.Stdin); err != nil {
		exit(2, "failed to start versus: %s", err)
	}
}

// versus is a main helper for launching the process that is testable.
type versus struct {
	Endpoints   []string
	Concurrency int
}

func (v *versus) Serve(ctx context.Context, r io.Reader) error {
	// Launch clients
	clients := make(Clients, 0, len(v.Endpoints))
	for _, endpoint := range v.Endpoints {
		c, err := NewClient(endpoint, v.Concurrency)
		if err != nil {
			return err
		}
		clients = append(clients, *c)
	}

	// Separate loop just to avoid leaking goroutines if NewClient fails
	g, ctx := errgroup.WithContext(ctx)
	for _, c := range clients {
		g.Go(func() error {
			return c.Serve(ctx)
		})
	}

	g.Go(func() error {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			line := scanner.Bytes()
			if line == "" { // Done
				return nil
			}

			if err := clients.Send(line); err != nil {
				return err
			}
		}

		if err := scanner.Err(); err != nil {
			return err
		}
		return nil
	})

	return g.Wait()
}
