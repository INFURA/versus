package main

import (
	"bytes"
	"fmt"
	"strings"
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

type Response struct {
	client  *Client
	Request *Request

	ID   requestID
	Body []byte
	Err  error

	Elapsed time.Duration
}

func (r *Response) Equal(other Response) bool {
	if r.Err == nil && other.Err == nil {
		return bytes.Equal(r.Body, other.Body)
	}
	if r.Err != nil && other.Err != nil {
		return r.Err.Error() == other.Err.Error() && bytes.Equal(r.Body, other.Body)
	}
	return false
}

type Responses []*Response

func (r Responses) String() string {
	var buf strings.Builder

	// TODO: Sort before printing
	last := r[0]
	for i, resp := range r {
		fmt.Fprintf(&buf, "\t%s", resp.Elapsed)

		if resp.Err.Error() != last.Err.Error() {
			fmt.Fprintf(&buf, "[%d: error mismatch: %s != %s]", i, resp.Err, last.Err)
		}

		if !bytes.Equal(resp.Body, last.Body) {
			fmt.Fprintf(&buf, "[%d: body mismatch: %s != %s]", i, resp.Body[:20], last.Body[:20])
		}
	}

	return buf.String()
}
