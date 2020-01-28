package main

import (
	"context"
	"net/http"
	"net/http/httptrace"
	"time"
)

var latencyTimeout = 2 * time.Second
var latencySamples = 5

type latencyResult struct {
	Latency time.Duration
	Err     error
}

// HTTPLatency sends sample number of concurrent HEAD requests to the domain of
// the endpoint and returns the lowest response time. This is approximately the
// latency of the server.
func HTTPLatency(endpoint string) (time.Duration, error) {
	resCh := make(chan latencyResult, latencySamples)
	for i := 0; i < latencySamples; i++ {
		go func() {
			var start time.Time
			var elapsed time.Duration
			trace := &httptrace.ClientTrace{
				GotFirstResponseByte: func() {
					elapsed = time.Now().Sub(start)
				},
				GetConn: func(string) {
					if start.IsZero() {
						start = time.Now()
					}
				},
			}
			req, err := http.NewRequest("HEAD", endpoint, nil)
			if err != nil {
				resCh <- latencyResult{
					Err: err,
				}
				return
			}
			ctx, cancel := context.WithTimeout(req.Context(), latencyTimeout)
			defer cancel()
			req = req.WithContext(httptrace.WithClientTrace(ctx, trace))
			resp, err := http.DefaultTransport.RoundTrip(req)
			defer resp.Body.Close()
			resCh <- latencyResult{
				Latency: elapsed,
				Err:     err,
			}
		}()
	}
	var minLatency time.Duration = latencyTimeout
	var lastErr error
	for i := 0; i < latencySamples; i++ {
		res := <-resCh
		if res.Err != nil {
			lastErr = res.Err
		} else if minLatency > res.Latency {
			minLatency = res.Latency
		}
	}
	return minLatency, lastErr
}
