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

func (stats *clientStats) Count(err error, elapsed time.Duration) {
	stats.NumTotal += 1
	if err != nil {
		stats.NumErrors += 1
	}
	stats.TimeTotal += elapsed
}

const chanBuffer = 20

func NewClient(endpoint string, concurrency int) (*Client, error) {
	c := Client{
		Endpoint:    endpoint,
		Concurrency: concurrency,
		In:          make(chan Request, chanBuffer),
	}
	return &c, nil
}

func NewClients(endpoints []string, concurrency int) (Clients, error) {
	clients := make(Clients, 0, len(endpoints))
	for _, endpoint := range endpoints {
		c, err := NewClient(endpoint, concurrency)
		if err != nil {
			return nil, err
		}
		clients = append(clients, c)
	}
	return clients, nil
}

type Client struct {
	Endpoint    string
	Concurrency int // Number of goroutines to make requests with. Must be >=1.
	In          chan Request
	Stats       clientStats

	ResponseHandler func(Response)
}

func (client *Client) Handle(req Request) {
	client.In <- req
}

// Serve starts the async request and response goroutine consumers.
func (client *Client) Serve(ctx context.Context) error {
	respCh := make(chan Response, chanBuffer)
	defer close(respCh)

	g, ctx := errgroup.WithContext(ctx)
	ctx, finalized := context.WithCancel(ctx)

	g.Go(func() error {
		for {
			select {
			case resp := <-respCh:
				client.Stats.Count(resp.Err, resp.Elapsed)
				if client.ResponseHandler != nil {
					client.ResponseHandler(resp)
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
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
					if req.ID == -1 {
						// Final request received, shutdown
						finalized()
						return nil
					}
					respCh <- req.Do(t)
				}
			}
		})
	}

	return g.Wait()
}

var id requestID

type Clients []*Client

// Finalize sends a request with ID -1 which signals the end of the stream, so
// serving will end cleanly.
func (c Clients) Finalize() {
	for _, client := range c {
		client.In <- Request{
			ID: -1,
		}
	}
}

func (c Clients) Send(line []byte) error {
	id += 1
	for _, client := range c {
		client.In <- Request{
			client: client,
			ID:     id,

			Line:      line,
			Timestamp: time.Now(),
		}
	}
	return nil
}

func (clients Clients) Serve(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	for _, c := range clients {
		g.Go(func() error {
			return c.Serve(ctx)
		})
	}

	return g.Wait()
}
