// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mockservices

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// HTTPServer provides a mock HTTP server for testing HTTP request executor
type HTTPServer struct {
	handler  http.Handler
	baseURL  string
	requests []HTTPRequest
	mutex    sync.RWMutex
}

// HTTPRequest represents a captured HTTP request
type HTTPRequest struct {
	Method  string                 `json:"method"`
	Path    string                 `json:"path"`
	Headers map[string][]string    `json:"headers"`
	Body    map[string]interface{} `json:"body,omitempty"`
}

// NewHTTPServer creates a new mock HTTP server
func NewHTTPServer(baseURL string) *HTTPServer {
	m := &HTTPServer{
		baseURL:  baseURL,
		requests: make([]HTTPRequest, 0),
	}
	m.handler = m.routes()
	return m
}

// routes maps the mock's endpoints.
func (m *HTTPServer) routes() http.Handler {
	mux := http.NewServeMux()

	// Handle notification endpoint
	mux.HandleFunc("/api/notifications", m.handleNotifications)

	// Handle user creation endpoint
	mux.HandleFunc("/api/users", m.handleUsers)

	// Handle error endpoint
	mux.HandleFunc("/api/error", m.handleError)

	// Handle generic success endpoint
	mux.HandleFunc("/api/success", m.handleSuccess)

	// Handle endpoint for retrieving captured requests (for test verification)
	mux.HandleFunc("/test/requests", m.handleGetRequests)

	// Handle endpoint for clearing captured requests
	mux.HandleFunc("/test/clear", m.handleClearRequests)

	return mux
}

// GetURL returns the base URL of the mock server
func (m *HTTPServer) GetURL() string {
	return m.baseURL
}

// GetNotificationsURL returns the notifications endpoint URL
func (m *HTTPServer) GetNotificationsURL() string {
	return fmt.Sprintf("%s/api/notifications", m.GetURL())
}

// GetUsersURL returns the users endpoint URL
func (m *HTTPServer) GetUsersURL() string {
	return fmt.Sprintf("%s/api/users", m.GetURL())
}

// GetErrorURL returns the error endpoint URL
func (m *HTTPServer) GetErrorURL() string {
	return fmt.Sprintf("%s/api/error", m.GetURL())
}

// captureRequest captures an incoming request for later verification
func (m *HTTPServer) captureRequest(r *http.Request, body map[string]interface{}) {
	capturedRequest := HTTPRequest{
		Method:  r.Method,
		Path:    r.URL.Path,
		Headers: r.Header,
		Body:    body,
	}

	m.mutex.Lock()
	m.requests = append(m.requests, capturedRequest)
	m.mutex.Unlock()
}

// handleNotifications handles notification API requests
func (m *HTTPServer) handleNotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body map[string]interface{}
	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err == nil && len(bodyBytes) > 0 {
			json.Unmarshal(bodyBytes, &body)
		}
	}

	m.captureRequest(r, body)

	log.Printf("Mock notification received: %v", body)

	response := map[string]interface{}{
		"id":     fmt.Sprintf("notif-%d", time.Now().Unix()),
		"status": "sent",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleUsers handles user creation API requests
func (m *HTTPServer) handleUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body map[string]interface{}
	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err == nil && len(bodyBytes) > 0 {
			json.Unmarshal(bodyBytes, &body)
		}
	}

	m.captureRequest(r, body)

	log.Printf("Mock user creation received: %v", body)

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"userId":     fmt.Sprintf("ext-user-%d", time.Now().Unix()),
			"profileUrl": "https://external-system.com/profile/123",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// handleError simulates an error response
func (m *HTTPServer) handleError(w http.ResponseWriter, r *http.Request) {
	var body map[string]interface{}
	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err == nil && len(bodyBytes) > 0 {
			json.Unmarshal(bodyBytes, &body)
		}
	}

	m.captureRequest(r, body)

	log.Printf("Mock error endpoint called: %v", body)

	response := map[string]interface{}{
		"error": "Internal server error",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(response)
}

// handleSuccess simulates a generic success response
func (m *HTTPServer) handleSuccess(w http.ResponseWriter, r *http.Request) {
	var body map[string]interface{}
	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err == nil && len(bodyBytes) > 0 {
			json.Unmarshal(bodyBytes, &body)
		}
	}

	m.captureRequest(r, body)

	response := map[string]interface{}{
		"status":  "success",
		"message": "Request processed successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleGetRequests handles requests to retrieve captured requests (for test verification)
func (m *HTTPServer) handleGetRequests(w http.ResponseWriter, r *http.Request) {
	m.mutex.RLock()
	requests := make([]HTTPRequest, len(m.requests))
	copy(requests, m.requests)
	m.mutex.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(requests)
}

// handleClearRequests handles requests to clear all captured requests
func (m *HTTPServer) handleClearRequests(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	m.mutex.Lock()
	m.requests = make([]HTTPRequest, 0)
	m.mutex.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "cleared"})
}

// GetCapturedRequests returns all captured requests
func (m *HTTPServer) GetCapturedRequests() []HTTPRequest {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	requests := make([]HTTPRequest, len(m.requests))
	copy(requests, m.requests)
	return requests
}

// GetRequestByPath returns the first captured request matching the given path
func (m *HTTPServer) GetRequestByPath(path string) *HTTPRequest {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for _, req := range m.requests {
		if req.Path == path {
			return &req
		}
	}
	return nil
}

// ClearRequests clears all captured requests
func (m *HTTPServer) ClearRequests() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.requests = make([]HTTPRequest, 0)
}
