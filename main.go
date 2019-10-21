package main // import "github.com/INFURA/versus"

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	flags "github.com/jessevdk/go-flags"
)

const chanBuffer = 20

// Options contains the flag options
type Options struct {
	Endpoints []string `positional-args:"yes"`
	Duration  string   `long:"duration" description:"Stop after some duration."`
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
		timeoutCtx, cancel := context.WithTimeout(ctx, d)
		defer cancel()
		ctx = timeoutCtx
	}

	// Launch clients
	clients := make(Clients, 0, len(options.Endpoints))
	for _, endpoint := range options.Endpoints {
		c, err := NewClient(endpoint)
		if err != nil {
			exit(2, "failed to create client: %s", err)
		}
		go func() {
			if err := c.Serve(ctx); err != nil {
				exit(2, "failed to start client: %s", err)
			}
		}()
		clients = append(clients, *c)
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := clients.Send(scanner.Bytes()); err != nil {
			log.Print(err)
			return
		}
	}

	if err := scanner.Err(); err != nil {
		log.Print(err)
		return
	}
}
