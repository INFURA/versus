package main

import "testing"

func TestHistogram(t *testing.T) {
	h := histogram{}

	for i := 1; i <= 1000; i++ {
		h.Add(float64(i))
	}

	if got, want := h.Total(), 500500.0; got != want {
		t.Errorf("got: %0.4f; want: %0.4f", got, want)
	}
	if got, want := h.Average(), 500.5; got != want {
		t.Errorf("got: %0.4f; want: %0.4f", got, want)
	}
	if got, want := h.Min(), 1.0; got != want {
		t.Errorf("got: %0.4f; want: %0.4f", got, want)
	}
	if got, want := h.Max(), 1000.0; got != want {
		t.Errorf("got: %0.4f; want: %0.4f", got, want)
	}

	percentiles := h.Percentiles(5, 50, 99, 100)
	if got, want := percentiles[0], 51.0; got != want {
		t.Errorf("got: %0.4f; want: %0.4f", got, want)
	}
	if got, want := percentiles[1], 501.0; got != want {
		t.Errorf("got: %0.4f; want: %0.4f", got, want)
	}
	if got, want := percentiles[2], 991.0; got != want {
		t.Errorf("got: %0.4f; want: %0.4f", got, want)
	}
	if got, want := percentiles[3], 1000.0; got != want {
		t.Errorf("got: %0.4f; want: %0.4f", got, want)
	}
}
