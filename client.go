package main

import (
	"context"
	"fmt"
	"io"
	"math"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

type clientStats struct {
	Concurrency int // Divide total time by concurrency to get rps

	mu        sync.Mutex
	numTotal  int // Number of requests
	numErrors int // Number of errors

	timeErrors time.Duration // Duration of error responses specifically
	errors     map[string]int

	timing histogram
}

func (stats *clientStats) Count(err error, elapsed time.Duration) {
	stats.mu.Lock()
	defer stats.mu.Unlock()

	stats.numTotal += 1
	stats.timing.Add(elapsed.Seconds())
	if err != nil {
		stats.numErrors += 1
		stats.timeErrors += elapsed

		if stats.errors == nil {
			stats.errors = map[string]int{}
		}
		stats.errors[err.Error()] += 1
	}
}

func (stats *clientStats) Render(w io.Writer) error {
	// TODO: Use templating?
	// TODO: Support JSON
	if stats.numTotal == 0 {
		fmt.Fprintf(w, "   No requests.")
	}
	var errRate, rps float64
	concurrency := 1
	if stats.Concurrency > 0 {
		concurrency = stats.Concurrency
	}
	errRate = float64(stats.numErrors*100) / float64(stats.numTotal)
	rps = float64(stats.numTotal*concurrency) / stats.timing.Total()

	fmt.Fprintf(w, "\n   Requests:   %0.2f per second", rps)
	if stats.numErrors > 0 && stats.numErrors != stats.numTotal {
		errAvg := float64(stats.numErrors) / stats.timeErrors.Seconds()
		fmt.Fprintf(w, ", %0.2f per second for errors", errAvg)
	}

	variance := stats.timing.Variance()
	stddev := math.Sqrt(variance)

	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "   Timing:     %0.4fs avg, %0.4fs min, %0.4fs max\n", stats.timing.Average(), stats.timing.Min(), stats.timing.Max())
	fmt.Fprintf(w, "               %0.4fs standard deviation\n", stddev)

	fmt.Fprintf(w, "\n   Percentiles:\n")
	buckets := []int{25, 50, 75, 90, 95, 99}
	percentiles := stats.timing.Percentiles(buckets...)
	for i, bucket := range buckets {
		fmt.Fprintf(w, "     %d%% in %0.4fs\n", bucket, percentiles[i])
	}

	fmt.Fprintf(w, "\n   Errors: %0.2f%%\n", errRate)

	for msg, num := range stats.errors {
		fmt.Fprintf(w, "     %d × %q\n", num, msg)
	}

	return nil
}

func NewClient(endpoint string, concurrency int) (*Client, error) {
	c := Client{
		Endpoint:    endpoint,
		Concurrency: concurrency,
		In:          make(chan Request, 2*concurrency),
	}
	c.Stats.Concurrency = concurrency
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
