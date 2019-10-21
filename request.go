package main

import (
	"bytes"
	"fmt"
	"strings"
	"time"
)

type Request struct {
	client *Client
	id     int

	Line      []byte
	Timestamp time.Time
}

func (req *Request) Do() {
	body, err := req.client.t.Send(req.Line)
	resp := Response{
		req.client, body, err, time.Now().Sub(req.Timestamp),
	}
	req.client.out <- resp
}

type Response struct {
	client *Client

	Body []byte
	Err  error

	Elapsed time.Duration
}

func (r *Response) Equal(other Response) bool {
	return r.Err.Error() == other.Err.Error() && bytes.Equal(r.Body, other.Body)
}

type Responses []Response

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
