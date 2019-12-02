package main

import "time"

type report struct {
	clients          Clients
	pendingResponses map[requestID][]Response

	requests   int           // Number of requests
	errors     int           // Number of errors
	mismatched int           // Number of mismatched responses
	elapsed    time.Duration // Total duration of requests

	MismatchedResponse func([]Response)
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
	if len(r.pendingResponses[resp.ID]) < len(r.clients)-1 {
		r.pendingResponses[resp.ID] = append(r.pendingResponses[resp.ID], resp)
		return
	}

	// All set, let's compare
	otherResponses := r.pendingResponses[resp.ID]
	delete(r.pendingResponses, resp.ID) // TODO: Reuse these arrays

	for _, other := range otherResponses {
		if !other.Equal(resp) {
			// Mismatch found, report the whole response set
			if r.MismatchedResponse != nil {
				otherResponses = append(otherResponses, resp)
				r.MismatchedResponse(otherResponses)
			}
		}
	}
}

// TODO: Need a separate service to compare returned bodies
func (r *report) Serve(out <-chan Response) error {
	for resp := range out {
		r.count(resp.Err, resp.Elapsed)
		r.compareResponses(resp)
	}
	return nil
}
