package main

import (
	"time"
)

type requestID int

type Request struct {
	client *Client

	ID        requestID
	Line      []byte
	Timestamp time.Time
}

func (req *Request) Do(t Transport) Response {
	timeStarted := time.Now()
	body, err := t.Send(req.Line)
	return Response{
		client: req.client,

		Request: req,
		ID:      req.ID,
		Body:    body,
		Err:     err,

		Elapsed: time.Now().Sub(timeStarted),
	}
}
