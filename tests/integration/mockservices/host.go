// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package mockservices runs the external services the integration tests stand in for: OAuth and
// OIDC identity providers, Google, GitHub, an SMS gateway and a generic HTTP endpoint. The test
// runner starts one host per kind of service for the whole run, and every test group shares it.
// A suite creates its own instance on a host, served under the instance's path and driven through
// the host's control API, so suites running at the same time never see each other's state.
package mockservices

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// Kind is a kind of external service.
type Kind string

const (
	KindOIDC         Kind = "oidc"
	KindOAuth        Kind = "oauth"
	KindGoogle       Kind = "google"
	KindGitHub       Kind = "github"
	KindNotification Kind = "notification"
	KindHTTP         Kind = "http"
)

// Ports is where the host of each kind answers. Google and GitHub must match the base URLs the
// server is configured with, since the server calls them at the root rather than at an instance.
var Ports = map[Kind]int{
	KindGitHub:       8092,
	KindGoogle:       8093,
	KindNotification: 8099,
	KindOIDC:         8100,
	KindOAuth:        8101,
	KindHTTP:         9091,
}

// ControlPath is the root of a host's control API:
//
//	POST   /_mock/instances              creates an instance, returning its id and base URL
//	DELETE /_mock/instances/{id}         removes it
//	POST   /_mock/instances/{id}/{op}    runs a control operation on it
const ControlPath = "/_mock/instances"

// NewInstanceRequest creates an instance. Identity providers check the client credentials.
type NewInstanceRequest struct {
	ClientID     string `json:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
}

// NewInstanceResponse identifies a created instance.
type NewInstanceResponse struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

// instance is one suite's mock on a host.
type instance interface {
	http.Handler
	control(op string, body []byte) (interface{}, error)
}

// rootInstance is an instance the server reaches at the host's root URL. It reports whether a
// request at the root is meant for it.
type rootInstance interface {
	instance
	owns(r *http.Request) bool
	client() string
}

// host serves every instance of one kind on one port.
type host struct {
	kind    Kind
	baseURL string
	server  *http.Server

	mutex     sync.RWMutex
	instances map[string]instance
	order     []string
	next      int
}

// Start starts a host for every kind of service, failing if any port is taken. The returned
// function stops them.
func Start() (func(), error) {
	var hosts []*host
	stop := func() {
		for _, h := range hosts {
			_ = h.server.Close()
		}
	}
	for kind, port := range Ports {
		listener, err := net.Listen("tcp", ":"+strconv.Itoa(port))
		if err != nil {
			stop()
			return nil, fmt.Errorf("failed to start the %s mock service on port %d: %w", kind, port, err)
		}
		h := &host{
			kind:      kind,
			baseURL:   "http://localhost:" + strconv.Itoa(port),
			instances: map[string]instance{},
		}
		h.server = &http.Server{Handler: h}
		hosts = append(hosts, h)
		go func() { _ = h.server.Serve(listener) }()
	}
	return stop, nil
}

// BaseURL returns the root URL of the host of a kind.
func BaseURL(kind Kind) string {
	return "http://localhost:" + strconv.Itoa(Ports[kind])
}

func (h *host) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, ControlPath) {
		h.serveControl(w, r, strings.Trim(strings.TrimPrefix(r.URL.Path, ControlPath), "/"))
		return
	}

	id, _, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
	h.mutex.RLock()
	inst := h.instances[id]
	h.mutex.RUnlock()
	if inst != nil {
		http.StripPrefix("/"+id, inst).ServeHTTP(w, r)
		return
	}

	if root := h.rootInstanceFor(r); root != nil {
		root.ServeHTTP(w, r)
		return
	}
	http.NotFound(w, r)
}

// rootInstanceFor finds the instance a request at the root is meant for, checking the most recent
// instance first. The body is buffered so each instance can read it.
func (h *host) rootInstanceFor(r *http.Request) rootInstance {
	var body []byte
	if r.Body != nil {
		body, _ = io.ReadAll(r.Body)
		_ = r.Body.Close()
	}
	reset := func() { r.Body = io.NopCloser(bytes.NewReader(body)) }

	h.mutex.RLock()
	defer h.mutex.RUnlock()
	for i := len(h.order) - 1; i >= 0; i-- {
		root, ok := h.instances[h.order[i]].(rootInstance)
		if !ok {
			continue
		}
		reset()
		probe := r.Clone(r.Context())
		probe.Form, probe.PostForm = nil, nil
		if root.owns(probe) {
			reset()
			return root
		}
	}
	reset()
	return nil
}

func (h *host) serveControl(w http.ResponseWriter, r *http.Request, path string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	switch {
	case path == "" && r.Method == http.MethodPost:
		var req NewInstanceRequest
		if len(body) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		created, err := h.create(req)
		writeControlResult(w, created, err)
	case r.Method == http.MethodDelete && !strings.Contains(path, "/"):
		h.remove(path)
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodPost && strings.Contains(path, "/"):
		id, op, _ := strings.Cut(path, "/")
		h.mutex.RLock()
		inst := h.instances[id]
		h.mutex.RUnlock()
		if inst == nil {
			http.Error(w, "unknown mock instance "+id, http.StatusNotFound)
			return
		}
		result, err := inst.control(op, body)
		writeControlResult(w, result, err)
	default:
		http.NotFound(w, r)
	}
}

func (h *host) create(req NewInstanceRequest) (*NewInstanceResponse, error) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.next++
	id := "i" + strconv.Itoa(h.next)
	url := h.baseURL + "/" + id

	var inst instance
	var err error
	switch h.kind {
	case KindOIDC:
		inst, err = NewOIDCServer(url, req.ClientID, req.ClientSecret)
	case KindOAuth:
		inst = NewOAuthServer(url, req.ClientID, req.ClientSecret)
	case KindGoogle:
		inst, err = NewGoogleOIDCServer(h.baseURL, req.ClientID, req.ClientSecret)
		url = h.baseURL
	case KindGitHub:
		inst = NewGithubOAuthServer(h.baseURL, req.ClientID, req.ClientSecret)
		url = h.baseURL
	case KindNotification:
		inst = NewNotificationServer(url)
	case KindHTTP:
		inst = NewHTTPServer(url)
	}
	if err != nil {
		return nil, err
	}

	// The server reaches Google and GitHub at the root, so only the client tells instances apart.
	if root, ok := inst.(rootInstance); ok {
		for _, existing := range h.instances {
			if other, ok := existing.(rootInstance); ok && other.client() == root.client() {
				return nil, fmt.Errorf("a %s mock for client %q already exists: each suite needs its own client id",
					h.kind, root.client())
			}
		}
	}

	h.instances[id] = inst
	h.order = append(h.order, id)
	return &NewInstanceResponse{ID: id, URL: url}, nil
}

func (h *host) remove(id string) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	delete(h.instances, id)
	for i, existing := range h.order {
		if existing == id {
			h.order = append(h.order[:i], h.order[i+1:]...)
			break
		}
	}
}

func writeControlResult(w http.ResponseWriter, result interface{}, err error) {
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errUnknownOperation) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
