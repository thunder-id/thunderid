// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
)

// ErrInProcessHandlerNotSet is returned by an in-process request made before the server handler is set.
var ErrInProcessHandlerNotSet = errors.New("the in-process handler is not set")

// InProcessTransport is an http.RoundTripper that serves requests to the server's own origin with the
// server's own handler chain, in process. The requests get the same middleware a network request
// gets, without a network hop, a private address, or a TLS handshake against the server's own
// certificate.
type InProcessTransport struct {
	handler atomic.Pointer[http.Handler]
}

// SetHandler sets the handler requests are served by. The composition root calls it once the
// server's handler chain is built, which happens after the routes are registered.
func (u *InProcessTransport) SetHandler(handler http.Handler) {
	u.handler.Store(&handler)
}

// RoundTrip serves the request with the server handler and returns the buffered response.
func (u *InProcessTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	handler := u.handler.Load()
	if handler == nil {
		return nil, ErrInProcessHandlerNotSet
	}

	inbound := req.Clone(req.Context())
	inbound.RequestURI = req.URL.RequestURI()
	inbound.Host = req.URL.Host
	inbound.RemoteAddr = "127.0.0.1:0"
	if inbound.Body == nil {
		inbound.Body = http.NoBody
	}

	rec := &bufferedResponse{header: http.Header{}}
	(*handler).ServeHTTP(rec, inbound)
	if rec.status == 0 {
		rec.status = http.StatusOK
	}

	return &http.Response{
		Status:        http.StatusText(rec.status),
		StatusCode:    rec.status,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        rec.header,
		Body:          io.NopCloser(bytes.NewReader(rec.body.Bytes())),
		ContentLength: int64(rec.body.Len()),
		Request:       req,
	}, nil
}

// bufferedResponse is the http.ResponseWriter the in-process handler writes into.
type bufferedResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (b *bufferedResponse) Header() http.Header {
	return b.header
}

func (b *bufferedResponse) WriteHeader(status int) {
	if b.status == 0 {
		b.status = status
	}
}

func (b *bufferedResponse) Write(p []byte) (int, error) {
	if b.status == 0 {
		b.status = http.StatusOK
	}
	return b.body.Write(p)
}
