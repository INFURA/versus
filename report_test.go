package main

import (
	"testing"
	"time"
)

func TestReport(t *testing.T) {
	clients, err := NewClients([]string{
		"noop://foo",
		"noop://bar",
	}, 1, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	r := report{Clients: clients}
	r.init()

	r.handle(Response{
		client: clients[0],
		ID:     1,
	})
	r.handle(Response{
		client: clients[0],
		ID:     2,
	})
	if got, want := r.requests, 2; got != want {
		t.Errorf("got: %d; want: %d", got, want)
	}
	if got, want := len(r.pendingResponses), 2; got != want {
		t.Errorf("got: %d; want: %d", got, want)
	}
	if got, want := r.completed, 0; got != want {
		t.Errorf("got: %d; want: %d", got, want)
	}
	r.handle(Response{
		client: clients[1],
		ID:     2,
	})
	if got, want := r.requests, 3; got != want {
		t.Errorf("got: %d; want: %d", got, want)
	}
	if got, want := len(r.pendingResponses), 1; got != want {
		t.Errorf("got: %d; want: %d", got, want)
	}
	if got, want := r.completed, 1; got != want {
		t.Errorf("got: %d; want: %d", got, want)
	}
	if got, want := r.mismatched, 0; got != want {
		t.Errorf("got: %d; want: %d", got, want)
	}
	r.handle(Response{
		client: clients[1],
		ID:     1,
		Body:   []byte("foo"),
	})
	if got, want := r.requests, 4; got != want {
		t.Errorf("got: %d; want: %d", got, want)
	}
	if got, want := len(r.pendingResponses), 0; got != want {
		t.Errorf("got: %d; want: %d", got, want)
	}
	if got, want := r.completed, 2; got != want {
		t.Errorf("got: %d; want: %d", got, want)
	}
	if got, want := r.mismatched, 1; got != want {
		t.Errorf("got: %d; want: %d", got, want)
	}
}
