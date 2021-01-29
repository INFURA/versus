package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/gorilla/websocket"
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
			Client:      http.Client{Timeout: timeout},
			endpoint:    url.String(),
			contentType: "application/json",
			bodyReader: func(body io.ReadCloser) ([]byte, error) {
				defer body.Close()
				return ioutil.ReadAll(body)
			},
		}
	case "ws", "wss":
		conn, _, err := websocket.DefaultDialer.Dial(url.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("Got: %s when connecting to ws", err)
		}
		t = &websocketTransport{
			ws: conn,
		}
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
}

type httpTransport struct {
	http.Client

	contentType string
	endpoint    string

	getHost string
	getPath string

	bodyReader func(io.ReadCloser) ([]byte, error)
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
	var resp *http.Response
	var err error
	if t.getHost != "" {
		url := t.getHost + path.Join(t.getPath, string(body))
		resp, err = t.Client.Get(url)
	} else {
		resp, err = t.Client.Post(t.endpoint, t.contentType, bytes.NewReader(body))
	}
	if err != nil {
		return nil, err
	}
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

type websocketTransport struct {
	ws *websocket.Conn
}

func (t *websocketTransport) Send(body []byte) ([]byte, error) {
	err := t.ws.WriteMessage(websocket.TextMessage, body)
	if err != nil {
		return nil, err
	}
	_, message, err := t.ws.ReadMessage()
	_ = message
	if err != nil {
		return nil, err
	}

	return nil, nil
}

type noopTransport struct{}

func (t *noopTransport) Send(body []byte) ([]byte, error) {
	return nil, nil
}
