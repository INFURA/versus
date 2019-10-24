package main

import (
	"context"
	"time"

	"golang.org/x/sync/errgroup"
)

const chanBuffer = 20

type Client struct {
	Endpoint    string
	Concurrency int // Number of goroutines to make requests with. Must be >=1.
	In          chan Request
	Report      report
}

// Serve starts the async request and response goroutine consumers.
func (client *Client) Serve(ctx context.Context) error {
	respCh := make(chan Response, chanBuffer)

	g, ctx := errgroup.WithContext(ctx)

	go func() {
		// Consume responses
		rep := report{}
		for resp := range respCh {
			rep.Add(resp.Err, resp.Elapsed)
		}
	}()

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
					return ctx.Err()
				case req := <-client.In:
					respCh <- req.Do(t)
				}
			}
		})
	}

	err := g.Wait()
	close(respCh)
	return err
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
