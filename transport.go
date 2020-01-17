package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

func NewTransport(endpoint string, timeout time.Duration) (Transport, error) {
	url, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	switch url.Scheme {
	case "http", "https":
		return &httpTransport{
			Client:   http.Client{Timeout: timeout},
			endpoint: endpoint,
		}, nil
	case "ws", "wss":
		// TODO: Implement
		return &websocketTransport{}, nil
	case "noop":
		return &noopTransport{}, nil
	}
	return nil, fmt.Errorf("unsupported transport: %s", url.Scheme)
}

type Transport interface {
	// TODO: Add context?
	// TODO: Should this be: Do(Request) (Response, error)?
	Send(body []byte) ([]byte, error)
}

type httpTransport struct {
	http.Client

	endpoint string
}

func (t *httpTransport) Send(body []byte) ([]byte, error) {
	resp, err := t.Client.Post(t.endpoint, "", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("bad status code: %d", resp.StatusCode)
	}
	// TODO: Avoid reading the entire body into memory
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

type websocketTransport struct {
}

func (t *websocketTransport) Send(body []byte) ([]byte, error) {
	return nil, errors.New("websocketTransport: not implemented")
}

type noopTransport struct{}

func (t *noopTransport) Send(body []byte) ([]byte, error) {
	return nil, nil
}
