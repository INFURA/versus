package main

import (
	"context"
	"sync"
	"time"
)

const chanBuffer = 20

func NewClient(endpoint string, concurrency int) (*Client, error) {
	t, err := NewTransport(endpoint)
	if err != nil {
		return nil, err
	}
	return &Client{
		Endpoint:    endpoint,
		Concurrency: concurrency,

		t:   t,
		in:  make(chan Request, chanBuffer),
		out: make(chan Response, chanBuffer),
	}, nil
}

type Client struct {
	Endpoint    string
	Concurrency int // Number of goroutines to make requests with. Must be >=1.

	t      Transport
	in     chan Request
	out    chan Response
	report report
}

// Serve starts the async request and response goroutine consumers.
func (client *Client) Serve(ctx context.Context) error {
	wg := sync.WaitGroup{}

	go func() {
		// Consume responses
		wg.Add(1)
		defer wg.Done()

		for {
			select {
			case <-ctx.Done():
				return
			case resp := <-client.out:
				client.report.Add(resp.Err, resp.Elapsed)
			}
		}
	}()

	if client.Concurrency < 1 {
		client.Concurrency = 1
	}

	for i := 0; i < client.Concurrency; i++ {
		go func() {
			// Consume requests
			wg.Add(1)
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				case req := <-client.in:
					req.Do()
				}
			}
		}()
	}

	wg.Wait()
	return nil
}

var id int

type Clients []Client

func (c Clients) Send(line []byte) error {
	id += 1
	for _, client := range c {
		client.in <- Request{
			client: &client,
			id:     id,

			Line:      line,
			Timestamp: time.Now(),
		}
	}
	return nil
}
