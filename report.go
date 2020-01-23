package main

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"
)

// report creates reporting by comparing responses from clients.
type report struct {
	Clients Clients

	// MismatchedResponse is called when a response set does not match across clients
	MismatchedResponse func([]Response)

	skipCompare      bool
	once             sync.Once
	pendingResponses map[requestID][]Response

	requests   int // Number of requests
	errors     int // Number of errors
	mismatched int // Number of mismatched responses
	completed  int // Number of completed responses across clients
	overloaded int // Number of times reporting channel was overloaded

	started time.Time     // Time when the report serving started
	elapsed time.Duration // Total duration of requests
}

func (r *report) Render(w io.Writer) error {
	fmt.Fprintf(w, "Endpoints:\n")
	for i, c := range r.Clients {
		fmt.Fprintf(w, "\n%d. %q\n", i, c.Endpoint)
		if err := c.Stats.Render(w); err != nil {
			return err
		}
	}

	fmt.Fprintf(w, "\n** Summary for %d endpoints:\n", len(r.Clients))
	fmt.Fprintf(w, "   Completed:  %d results with %d total requests\n", r.completed, r.requests)
	if r.requests > 0 {
		fmt.Fprintf(w, "   Timing:     %s request avg, %s total run time\n", r.elapsed/time.Duration(r.requests), time.Now().Sub(r.started))
		fmt.Fprintf(w, "   Errors:     %d (%0.2f%%)\n", r.errors, float64(r.errors*100)/float64(r.requests))
	}
	fmt.Fprintf(w, "   Mismatched: %d\n", r.mismatched)

	if r.overloaded > 0 {
		fmt.Fprintf(w, "** Reporting consumer was overloaded %d times. Please open an issue.\n", r.overloaded)
	}

	if len(r.pendingResponses) != 0 {
		fmt.Fprintf(w, "** %d incomplete responses:\n", len(r.pendingResponses))
		for reqID, resps := range r.pendingResponses {
			fmt.Fprintf(w, " * %d: %v\n", reqID, resps)
		}
		fmt.Fprintf(w, "\n")
	}

	return nil
}

func (r *report) count(err error, elapsed time.Duration) {
	r.requests += 1
	if err != nil {
		r.errors += 1
	}
	r.elapsed += elapsed
}

func (r *report) compareResponses(resp Response) {
	// Are we waiting for more responses?
	if len(r.pendingResponses[resp.ID]) < len(r.Clients)-1 {
		r.pendingResponses[resp.ID] = append(r.pendingResponses[resp.ID], resp)
		return
	}

	// All set, let's compare
	otherResponses := r.pendingResponses[resp.ID]
	delete(r.pendingResponses, resp.ID) // TODO: Reuse these arrays
	r.completed += 1

	durations := make([]time.Duration, 0, len(r.Clients))
	durations = append(durations, resp.Elapsed)

	for _, other := range otherResponses {
		durations = append(durations, other.Elapsed)

		if !other.Equal(resp) {
			// Mismatch found, report the whole response set
			r.mismatched += 1
			if r.MismatchedResponse != nil {
				otherResponses = append(otherResponses, resp)
				r.MismatchedResponse(otherResponses)
			}
		}
	}

	// TODO: Check for JSONRPC error objects?

	l := logger.Debug().Int("id", int(resp.ID)).Int("mismatched", r.mismatched).Durs("ms", durations).Err(resp.Err)
	// For super-debugging:
	// l = l.Bytes("req", resp.Request.Line).Bytes("resp", resp.Body)
	l.Msg("result")
}

func (r *report) handle(resp Response) error {
	r.count(resp.Err, resp.Elapsed)
	if r.skipCompare {
		return nil
	}

	r.compareResponses(resp)
	return nil
}

func (r *report) init() {
	r.once.Do(func() {
		r.pendingResponses = make(map[requestID][]Response)
	})
}

func (r *report) Serve(ctx context.Context, respCh <-chan Response) error {
	r.init()

	r.started = time.Now()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case resp, ok := <-respCh:
			if !ok {
				return nil
			}
			if err := r.handle(resp); err != nil {
				return err
			}
		}
	}
}
