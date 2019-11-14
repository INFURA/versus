package main

import (
	"context"
	"time"

	"golang.org/x/sync/errgroup"
)

type clientStats struct {
	NumTotal  int           // Number of requests
	NumErrors int           // Number of errors
	TimeTotal time.Duration // Total duration of requests
}

func (stats *clientStats) Add(err error, elapsed time.Duration) {
	stats.NumTotal += 1
	if err != nil {
		stats.NumErrors += 1
	}
	stats.TimeTotal += elapsed
}

const chanBuffer = 20

type Client struct {
	Endpoint    string
	Concurrency int // Number of goroutines to make requests with. Must be >=1.
	In          chan Request
	Out         chan Response
	Stats       clientStats
}

// Serve starts the async request and response goroutine consumers.
func (client *Client) Serve(ctx context.Context) error {
	respCh := make(chan Response, chanBuffer)
	defer close(respCh)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		for {
			select {
			case resp := <-respCh:
				client.Stats.Add(resp.Err, resp.Elapsed)
				// TODO: Relay response to client.Out
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	})

	if client.Concurrency < 1 {
		client.Concurrency = 1
	}

	for i := 0; i < client.Concurrency; i++ {
		g.Go(func() error {
			// Consume requests
			t, err := NewTransport(client.Endpoint)
			if err != nil {
				return err
			}
			for {
				select {
				case <-ctx.Done():
					logger.Debug().Str("endpoint", client.Endpoint).Msg("shutting down client")
					return nil
				case req := <-client.In:
					respCh <- req.Do(t)
				}
			}
		})
	}

	return g.Wait()
}

var id int

type Clients []Client

func (c Clients) Send(line []byte) error {
	id += 1
	for _, client := range c {
		client.In <- Request{
			client: &client,
			id:     id,

			Line:      line,
			Timestamp: time.Now(),
		}
	}
	return nil
}
