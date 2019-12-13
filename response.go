package main

import (
	"bytes"
	"fmt"
	"strings"
	"time"
)

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

type Responses []Response

func (resps Responses) String() string {
	var buf strings.Builder

	// TODO: Sort before printing
	last := resps[0]
	for i, resp := range resps {
		fmt.Fprintf(&buf, "\t%s", resp.Elapsed)

		if resp.Err == nil && last.Err == nil {
			if !bytes.Equal(resp.Body, last.Body) {
				fmt.Fprintf(&buf, "[%d: body mismatch: %s != %s]", i, resp.Body[:50], last.Body[:50])
			}
		} else if resp.Err != nil && last.Err != nil && resp.Err.Error() != last.Err.Error() {
			fmt.Fprintf(&buf, "[%d: error mismatch: %s != %s]", i, resp.Err, last.Err)
		} else {
			fmt.Fprintf(&buf, "[%d: error mismatch: %s != %s]", i, resp.Err, last.Err)
		}

	}

	return buf.String()
}
