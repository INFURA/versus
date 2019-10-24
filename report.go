package main

import "time"

type report struct {
	numTotal  int // Number of requests
	numErrors int // Number of errors

	timeTotal time.Duration // Total duration of requests
}

func (r *report) Add(err error, elapsed time.Duration) {
	r.numTotal += 1
	if err != nil {
		r.numErrors += 1
	}
	r.timeTotal += elapsed
}

func (r *report) MergeInto(into *report) *report {
	if into == nil {
		into = &report{}
	}

	into.numTotal += r.numTotal
	into.numErrors += r.numErrors
	into.timeTotal += r.timeTotal
	return into
}
