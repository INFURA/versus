package main

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/valyala/fasthttp"
)

func NewTransport(endpoint string) (Transport, error) {
	url, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	switch url.Scheme {
	case "http", "https":
		return &fasthttpTransport{
			endpoint: endpoint,
		}, nil
	case "ws", "wss":
		// TODO: Implement
		return &websocketTransport{}, nil
	}
	return nil, fmt.Errorf("unsupported transport: %s", url.Scheme)
}

type Transport interface {
	Send(body []byte) ([]byte, error)
}

type fasthttpTransport struct {
	endpoint string
	fasthttp.Client
}

func (t *fasthttpTransport) Send(body []byte) ([]byte, error) {
	code, resp, err := t.Post(body, t.endpoint, nil)
	if err != nil {
		return nil, err
	}
	if code >= 400 {
		return nil, fmt.Errorf("bad status code: %d", code)
	}
	return resp, nil
}

type websocketTransport struct {
}

func (t *websocketTransport) Send(body []byte) ([]byte, error) {
	return nil, errors.New("websocketTransport: not implemented")
}
