package main

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

type clientStats struct {
	mu sync.Mutex

	numTotal   int           // Number of requests
	numErrors  int           // Number of errors
	timeErrors time.Duration // Duration of error responses specifically
	timeTotal  time.Duration // Total duration of requests
	errors     map[string]int

	timeMin float64
	timeMax float64
	timeAll []float64
}

func (stats *clientStats) Count(err error, elapsed time.Duration) {
	stats.mu.Lock()
	defer stats.mu.Unlock()

	stats.numTotal += 1
	if err != nil {
		stats.numErrors += 1
		stats.timeErrors += elapsed

		if stats.errors == nil {
			stats.errors = map[string]int{}
		}
		stats.errors[err.Error()] += 1
	}
	stats.timeTotal += elapsed
	seconds := elapsed.Seconds()
	stats.timeAll = append(stats.timeAll, seconds)
	if stats.timeMin > seconds {
		stats.timeMin = seconds
	}
	if stats.timeMax < seconds {
		stats.timeMax = seconds
	}
}

func (stats *clientStats) Render(w io.Writer) error {
	if stats.numTotal == 0 {
		fmt.Fprintf(w, "   No requests.")
	}
	var errRate, rps float64

	errRate = float64(stats.numErrors*100) / float64(stats.numTotal)
	rps = float64(stats.numTotal) / stats.timeTotal.Seconds()
	reqAvg := stats.timeTotal / time.Duration(stats.numTotal)

	fmt.Fprintf(w, "   Requests/Sec: %0.2f", rps)
	if stats.numErrors > 0 && stats.numErrors != stats.numTotal {
		errAvg := stats.timeErrors / time.Duration(stats.numErrors)
		fmt.Fprintf(w, ", %s per error", errAvg)
	}
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "   Average:      %s\n", reqAvg)
	fmt.Fprintf(w, "   Errors:       %0.2f%%\n", errRate)

	for msg, num := range stats.errors {
		fmt.Fprintf(w, "   * [%d] %q\n", num, msg)
	}

	return nil
}

func NewClient(endpoint string, concurrency int) (*Client, error) {
	c := Client{
		Endpoint:    endpoint,
		Concurrency: concurrency,
		In:          make(chan Request, 2*concurrency),
	}
	return &c, nil
}

func NewClients(endpoints []string, concurrency int, timeout time.Duration) (Clients, error) {
	clients := make(Clients, 0, len(endpoints))
	for _, endpoint := range endpoints {
		c, err := NewClient(endpoint, concurrency)
		if err != nil {
			return nil, err
		}
		c.Timeout = timeout
		clients = append(clients, c)
	}
	return clients, nil
}

type Client struct {
	Endpoint    string
	Concurrency int           // Number of goroutines to make requests with. Must be >=1.
	Timeout     time.Duration // Timeout of each request

	In    chan Request
	Stats clientStats
}

func (client *Client) Handle(req Request) {
	client.In <- req
}

// Serve starts the async request and response goroutine consumers.
func (client *Client) Serve(ctx context.Context, out chan<- Response) error {
	g, ctx := errgroup.WithContext(ctx)

	if client.Concurrency < 1 {
		client.Concurrency = 1
	}

	logger.Debug().Str("endpoint", client.Endpoint).Int("concurrency", client.Concurrency).Msg("starting client")

	for i := 0; i < client.Concurrency; i++ {
		g.Go(func() error {
			// Consume requests
			t, err := NewTransport(client.Endpoint, client.Timeout)
			if err != nil {
				return err
			}
			for {
				select {
				case <-ctx.Done():
					logger.Debug().Str("endpoint", client.Endpoint).Msg("aborting client")
					return nil
				case req := <-client.In:
					if req.ID == -1 {
						// Final request received, shutdown
						logger.Debug().Str("endpoint", client.Endpoint).Msg("received final request, shutting down")
						return nil
					}
					resp := req.Do(t)
					client.Stats.Count(resp.Err, resp.Elapsed)
					select {
					case out <- resp:
					default:
						logger.Warn().Msg("response channel is overloaded, please open an issue")
						out <- resp
					}
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
		for i := 0; i < client.Concurrency; i++ {
			// Signal each client instance to shut down
			client.In <- Request{
				ID: -1,
			}
		}
	}
}

func (c Clients) Send(ctx context.Context, line []byte) error {
	id += 1
	for _, client := range c {
		select {
		case client.In <- Request{
			client: client,
			ID:     id,

			Line:      line,
			Timestamp: time.Now(),
		}:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (clients Clients) Serve(ctx context.Context, out chan Response) error {
	g, ctx := errgroup.WithContext(ctx)

	for _, c := range clients {
		c := c // Otherwise c gets mutated per iteration and we get a race
		g.Go(func() error {
			return c.Serve(ctx, out)
		})
	}

	return g.Wait()
}
