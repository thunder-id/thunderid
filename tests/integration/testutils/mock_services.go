// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/thunder-id/thunderid/tests/integration/mockservices"
)

// mockInstance is a suite's own instance of a mock service the test runner hosts.
type mockInstance struct {
	kind mockservices.Kind
	id   string
	url  string
}

// newMockInstance creates an instance of a kind of mock service.
func newMockInstance(kind mockservices.Kind, clientID, clientSecret string) (*mockInstance, error) {
	payload, err := json.Marshal(mockservices.NewInstanceRequest{ClientID: clientID, ClientSecret: clientSecret})
	if err != nil {
		return nil, err
	}
	resp, err := http.Post(mockservices.BaseURL(kind)+mockservices.ControlPath, "application/json",
		bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("the %s mock service is not reachable, start the tests with the test runner: %w",
			kind, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to create a %s mock: %s", kind, bytes.TrimSpace(body))
	}

	var created mockservices.NewInstanceResponse
	if err := json.Unmarshal(body, &created); err != nil {
		return nil, err
	}
	return &mockInstance{kind: kind, id: created.ID, url: created.URL}, nil
}

// remove deletes the instance from its host.
func (i *mockInstance) remove() error {
	req, err := http.NewRequest(http.MethodDelete,
		mockservices.BaseURL(i.kind)+mockservices.ControlPath+"/"+i.id, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

// call runs a control operation on the instance and decodes its result into result, if given.
func (i *mockInstance) call(op string, args interface{}, result interface{}) error {
	payload, err := json.Marshal(args)
	if err != nil {
		return err
	}
	resp, err := http.Post(mockservices.BaseURL(i.kind)+mockservices.ControlPath+"/"+i.id+"/"+op,
		"application/json", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("%s mock %s failed: %w", i.kind, op, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s mock %s failed: %s", i.kind, op, bytes.TrimSpace(body))
	}
	if result == nil {
		return nil
	}
	return json.Unmarshal(body, result)
}

// mustCall runs a control operation whose failure means the mock service itself is broken. The
// methods that drive a mock return no error, so it panics, which fails the running test.
func (i *mockInstance) mustCall(op string, args interface{}, result interface{}) {
	if err := i.call(op, args, result); err != nil {
		panic(err)
	}
}

// setHook makes the instance call fn whenever it runs the named hook. A nil fn clears the hook.
func (i *mockInstance) setHook(name string, fn func(args json.RawMessage) (interface{}, error)) {
	url := ""
	if fn != nil {
		url = hooks.register(fn)
	}
	i.mustCall(mockservices.OpHook, mockservices.HookRequest{Name: name, URL: url}, nil)
}

// hooks serves the functions tests set on their mocks, which a mock service runs by calling back
// into the test process.
var hooks = &hookServer{funcs: map[string]func(json.RawMessage) (interface{}, error){}}

type hookServer struct {
	once  sync.Once
	url   string
	err   error
	next  atomic.Int64
	mutex sync.RWMutex
	funcs map[string]func(json.RawMessage) (interface{}, error)
}

// register serves fn and returns the URL that runs it.
func (h *hookServer) register(fn func(json.RawMessage) (interface{}, error)) string {
	h.once.Do(h.start)
	if h.err != nil {
		panic(fmt.Errorf("failed to start the mock hook server: %w", h.err))
	}
	id := strconv.FormatInt(h.next.Add(1), 10)
	h.mutex.Lock()
	h.funcs[id] = fn
	h.mutex.Unlock()
	return h.url + "/" + id
}

func (h *hookServer) start() {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		h.err = err
		return
	}
	h.url = "http://" + listener.Addr().String()
	go func() { _ = http.Serve(listener, h) }()
}

func (h *hookServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mutex.RLock()
	fn := h.funcs[r.URL.Path[1:]]
	h.mutex.RUnlock()
	if fn == nil {
		http.NotFound(w, r)
		return
	}

	args, _ := io.ReadAll(r.Body)
	var reply mockservices.HookReply
	value, err := fn(args)
	if err != nil {
		reply.Error = err.Error()
	} else if reply.Value, err = json.Marshal(value); err != nil {
		reply.Error = err.Error()
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(reply)
}

// authorizeHook adapts a test's authorize function to a hook.
func authorizeHook(fn func(string) (string, error)) func(json.RawMessage) (interface{}, error) {
	if fn == nil {
		return nil
	}
	return func(json.RawMessage) (interface{}, error) { return fn("") }
}

// responseHook adapts a test's response override to a hook.
func responseHook(fn func() (int, string)) func(json.RawMessage) (interface{}, error) {
	if fn == nil {
		return nil
	}
	return func(json.RawMessage) (interface{}, error) {
		status, body := fn()
		return mockservices.ResponseOverride{Status: status, Body: body}, nil
	}
}
