package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"
)

// NewTransport creates a transport that supports the given endpoint. The
// endpoint is a URI with a scheme and an optional mode, for example
// "https+get://infura.io/".
func NewTransport(endpoint string, timeout time.Duration) (Transport, error) {
	url, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	scheme, mode := url.Scheme, ""
	if parts := strings.Split(scheme, "+"); len(parts) > 1 {
		scheme, mode = parts[0], parts[1]
	}
	var t Transport
	switch scheme {
	case "http", "https":
		url.Scheme = scheme
		t = &httpTransport{
			Client:   http.Client{Timeout: timeout},
			endpoint: url.String(),
			bodyReader: func(body io.ReadCloser) ([]byte, error) {
				defer body.Close()
				return ioutil.ReadAll(body)
			},
		}
	case "ws", "wss":
		// TODO: Implement
		t = &websocketTransport{}
	case "noop":
		t = &noopTransport{}
	default:
		return nil, fmt.Errorf("unsupported transport: %s", scheme)
	}

	if mode == "" {
		return t, nil
	}
	if modalTransport, ok := t.(Modal); ok {
		return t, modalTransport.Mode(mode)
	}
	return nil, fmt.Errorf("transport is not modal: %s", scheme)
}

// Modal is a type of Transport that has multiple modes for interpreting the
// payloads sent to it. Not all transports support modes.
type Modal interface {
	Mode(string) error
}
type Transport interface {
	// TODO: Add context?
	// TODO: Should this be: Do(Request) (Response, error)?
	Send(body []byte) ([]byte, error)
	MinLatency() time.Duration
}

type httpTransport struct {
	http.Client

	contentType string
	endpoint    string

	getHost string
	getPath string

	bodyReader func(io.ReadCloser) ([]byte, error)

	mu         sync.Mutex
	minLatency time.Duration
}

func (t *httpTransport) Mode(m string) error {
	switch strings.ToLower(m) {
	case "post":
		t.getHost = ""
	case "get":
		url, err := url.Parse(t.endpoint)
		if err != nil {
			return err
		}
		t.getPath = url.Path
		if t.getPath == "" {
			t.getPath = "/"
		}
		url.Path = ""
		t.getHost = url.String()
	default:
		return fmt.Errorf("invalid mode for http transport: %s", m)
	}
	return nil
}

func (t *httpTransport) Send(body []byte) ([]byte, error) {
	var req *http.Request
	var err error
	if t.getHost != "" {
		url := t.getHost + path.Join(t.getPath, string(body))
		req, err = http.NewRequest("GET", url, nil)
	} else {
		req, err = http.NewRequest("POST", t.endpoint, bytes.NewReader(body))
		req.Header.Set("Content-Type", t.contentType)
	}
	if err != nil {
		return nil, err
	}
	var elapsed time.Duration
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), traceRequestDuration(&elapsed)))
	resp, err := t.Client.Do(req)
	if err != nil {
		return nil, err
	}
	t.mu.Lock()
	if t.minLatency == 0 || t.minLatency > elapsed {
		t.minLatency = elapsed
	}
	t.mu.Unlock()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("bad status code: %d", resp.StatusCode)
	}
	if t.bodyReader == nil {
		resp.Body.Close()
		return nil, nil
	}
	// TODO: Avoid reading the whole body into memory
	return t.bodyReader(resp.Body)
}

func (t *httpTransport) MinLatency() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.minLatency
}

func traceRequestDuration(d *time.Duration) *httptrace.ClientTrace {
	var start time.Time
	return &httptrace.ClientTrace{
		GotFirstResponseByte: func() {
			elapsed := time.Now().Sub(start)
			*d = elapsed
		},
		GetConn: func(string) {
			if start.IsZero() {
				start = time.Now()
			}
		},
	}
}

type websocketTransport struct {
}

func (t *websocketTransport) Send(body []byte) ([]byte, error) {
	return nil, errors.New("websocketTransport: not implemented")
}

func (t *websocketTransport) MinLatency() time.Duration {
	return 0
}

type noopTransport struct{}

func (t *noopTransport) Send(body []byte) ([]byte, error) {
	return nil, nil
}

func (t *noopTransport) MinLatency() time.Duration {
	return 0
}
