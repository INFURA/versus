package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
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
		// TODO: Use github.com/nsf/jsondiff to detect subsets and for pretty printing diffs?
		return bytes.Equal(r.Body, other.Body) || jsonEqual(r.Body, other.Body)
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
				fmt.Fprintf(&buf, "[%d: body mismatch:\n%s\n\t%s\n%s\n\t%s]", i, resp.client.Endpoint, resp.Body, last.client.Endpoint, last.Body)
			}
		} else if resp.Err != nil && last.Err != nil && resp.Err.Error() != last.Err.Error() {
			fmt.Fprintf(&buf, "[%d: error mismatch: %s != %s]", i, resp.Err, last.Err)
		} else {
			fmt.Fprintf(&buf, "[%d: error mismatch: %s != %s]", i, resp.Err, last.Err)
		}

	}

	return buf.String()
}

// jsonEqual returns true if a and b are JSON objects (starting with '{') and equal.
func jsonEqual(a, b []byte) bool {
	if len(a) == 0 || a[0] != '{' {
		return false
	}

	// Normalize a and b
	var aObj, bObj interface{}
	if err := json.Unmarshal(a, &aObj); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &bObj); err != nil {
		return false
	}

	return reflect.DeepEqual(aObj, bObj)
}
