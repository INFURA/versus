package main // import "github.com/INFURA/versus"

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	flags "github.com/jessevdk/go-flags"
)

const chanBuffer = 20

// Options contains the flag options
type Options struct {
	Endpoints []string `positional-args:"yes"`
}

func exit(code int, format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(code)
}

func main() {
	options := Options{}
	p, err := flags.NewParser(&options, flags.Default).ParseArgs(os.Args[1:])
	if err != nil {
		if p == nil {
			fmt.Println(err)
		}
		return
	}

	clients := make(Clients, 0, len(options.Endpoints))
	for _, endpoint := range options.Endpoints {
		c, err := NewClient(endpoint)
		if err != nil {
			exit(1, "failed to create client: %s", err)
		}
		clients = append(clients, *c)
	}

	ctx := context.Background()

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := clients.Send(scanner.Bytes()); err != nil {
			log.Print(err)
			return
		}
	}

	if err := scanner.Err(); err != nil {
		log.Print(err)
		return
	}
}

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
