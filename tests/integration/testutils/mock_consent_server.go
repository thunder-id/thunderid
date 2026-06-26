/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package testutils

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// --- DTOs that mirror the API contract (matching default_client.go expectations) ---

type mockConsentElementDTO struct {
	ElementID   string            `json:"elementId"`
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace,omitempty"`
	Type        string            `json:"type"`
	Version     string            `json:"version,omitempty"`
	DisplayName string            `json:"displayName,omitempty"`
	Description string            `json:"description,omitempty"`
	Properties  map[string]string `json:"properties,omitempty"`
	CreatedTime int64             `json:"createdTime,omitempty"`
}

type mockConsentElementCreateDTO struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace,omitempty"`
	Type        string            `json:"type"`
	DisplayName string            `json:"displayName,omitempty"`
	Description string            `json:"description,omitempty"`
	Properties  map[string]string `json:"properties,omitempty"`
}

type mockConsentElementVersionDTO struct {
	DisplayName string            `json:"displayName,omitempty"`
	Description string            `json:"description,omitempty"`
	Properties  map[string]string `json:"properties,omitempty"`
}

type mockBulkElementResultDTO struct {
	Status  string                 `json:"status"`
	Element *mockConsentElementDTO `json:"element,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

type mockElementsCreateResponseDTO struct {
	Results []mockBulkElementResultDTO `json:"results"`
}

type mockConsentPurposeElementDTO struct {
	ElementID string `json:"elementId,omitempty"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Version   string `json:"version,omitempty"`
	Mandatory bool   `json:"mandatory"`
}

type mockConsentPurposeCreateDTO struct {
	Name        string                         `json:"name"`
	DisplayName string                         `json:"displayName,omitempty"`
	Description string                         `json:"description,omitempty"`
	Properties  map[string]string              `json:"properties,omitempty"`
	Elements    []mockConsentPurposeElementDTO `json:"elements"`
}

type mockConsentPurposeVersionDTO struct {
	DisplayName string                         `json:"displayName,omitempty"`
	Description string                         `json:"description,omitempty"`
	Properties  map[string]string              `json:"properties,omitempty"`
	Elements    []mockConsentPurposeElementDTO `json:"elements"`
}

type mockConsentPurposeDTO struct {
	PurposeID   string                         `json:"purposeId"`
	Name        string                         `json:"name"`
	GroupID     string                         `json:"groupId"`
	Version     string                         `json:"version,omitempty"`
	DisplayName string                         `json:"displayName,omitempty"`
	Description string                         `json:"description"`
	Properties  map[string]string              `json:"properties,omitempty"`
	Elements    []mockConsentPurposeElementDTO `json:"elements"`
	CreatedTime int64                          `json:"createdTime"`
	UpdatedTime int64                          `json:"updatedTime"`
}

// --- Internal mock state types ---

type mockConsentElement struct {
	mockConsentElementDTO
}

// mockConsentPurpose is the internal storage type for a consent purpose in the mock server.
type mockConsentPurpose = mockConsentPurposeDTO

// MockConsentServer provides a lightweight mock of the default consent management
// REST API. It stores consent elements and purposes in memory, allowing integration tests
// to verify that the server correctly syncs consent state on application lifecycle events.
type MockConsentServer struct {
	server   *http.Server
	port     int
	mu       sync.Mutex
	elements map[string]*mockConsentElement // elementID -> element
	purposes map[string]*mockConsentPurpose // purposeID -> purpose
	idSeq    int
}

// NewMockConsentServer creates a new mock consent server that listens on the given port.
func NewMockConsentServer(port int) *MockConsentServer {
	return &MockConsentServer{
		port:     port,
		elements: make(map[string]*mockConsentElement),
		purposes: make(map[string]*mockConsentPurpose),
	}
}

// nextIDLocked generates the next mock ID. Must be called with mu held.
func (s *MockConsentServer) nextIDLocked() string {
	s.idSeq++
	return fmt.Sprintf("mock-consent-%04d", s.idSeq)
}

// GetURL returns the base API URL of the mock server.
func (s *MockConsentServer) GetURL() string {
	return fmt.Sprintf("http://localhost:%d/api/v1", s.port)
}

// Reset clears all stored elements and purposes.
func (s *MockConsentServer) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.elements = make(map[string]*mockConsentElement)
	s.purposes = make(map[string]*mockConsentPurpose)
	s.idSeq = 0
}

// Start starts the mock consent server in the background.
func (s *MockConsentServer) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/consent-elements/", s.handleElementByID)
	mux.HandleFunc("/api/v1/consent-elements", s.handleElements)
	mux.HandleFunc("/api/v1/consent-purposes/", s.handlePurposeByID)
	mux.HandleFunc("/api/v1/consent-purposes", s.handlePurposes)

	// Test inspection endpoints — NOT part of the real OpenFGC API.
	// These allow test suites to query and reset the mock state without holding
	// a reference to the server struct (tests only have the base URL).
	mux.HandleFunc("/test/purposes", s.handleTestPurposes)
	mux.HandleFunc("/test/reset", s.handleTestReset)

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	ln, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return err
	}

	go func() {
		log.Printf("Starting mock consent server on port %d", s.port)
		if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("Mock consent server error: %v", err)
		}
	}()

	return nil
}

// Stop stops the mock consent server.
func (s *MockConsentServer) Stop() error {
	if s.server != nil {
		return s.server.Close()
	}

	return nil
}

// writeJSON writes a JSON response with the given status code.
func (s *MockConsentServer) writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// writeError writes a plain-text error response.
func (s *MockConsentServer) writeError(w http.ResponseWriter, status int, msg string) {
	http.Error(w, msg, status)
}

// --- Consent Elements handlers ---

// handleElements handles POST (bulk create) and GET (list) on /api/v1/consent-elements.
func (s *MockConsentServer) handleElements(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleElementsCreate(w, r)
	case http.MethodGet:
		s.handleElementsList(w, r)
	default:
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleElementsCreate handles POST /api/v1/consent-elements.
// Bulk-creates consent elements and returns the v0.3.0 partial-success response shape.
func (s *MockConsentServer) handleElementsCreate(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	var inputs []mockConsentElementCreateDTO
	if err := json.Unmarshal(body, &inputs); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	s.mu.Lock()
	results := make([]mockBulkElementResultDTO, 0, len(inputs))
	for _, inp := range inputs {
		el := mockConsentElement{
			mockConsentElementDTO: mockConsentElementDTO{
				ElementID:   s.nextIDLocked(),
				Name:        inp.Name,
				Namespace:   inp.Namespace,
				Type:        inp.Type,
				Version:     "v1",
				DisplayName: inp.DisplayName,
				Description: inp.Description,
				Properties:  inp.Properties,
				CreatedTime: time.Now().UnixMilli(),
			},
		}
		s.elements[el.ElementID] = &el
		created := el.mockConsentElementDTO
		results = append(results, mockBulkElementResultDTO{
			Status:  "SUCCESS",
			Element: &created,
		})
	}
	s.mu.Unlock()

	s.writeJSON(w, http.StatusOK, mockElementsCreateResponseDTO{Results: results})
}

// handleElementsList handles GET /api/v1/consent-elements with optional ?name= filter.
func (s *MockConsentServer) handleElementsList(w http.ResponseWriter, r *http.Request) {
	nameFilter := r.URL.Query().Get("name")

	s.mu.Lock()
	list := make([]mockConsentElementDTO, 0, len(s.elements))
	for _, el := range s.elements {
		if nameFilter == "" || el.Name == nameFilter {
			list = append(list, el.mockConsentElementDTO)
		}
	}
	s.mu.Unlock()

	s.writeJSON(w, http.StatusOK, map[string]interface{}{"data": list})
}

// handleElementByID routes requests under /api/v1/consent-elements/{id}.
// Supports POST on /{id}/versions (update via new version) and DELETE on
// /{id}/versions/{version}
func (s *MockConsentServer) handleElementByID(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/v1/consent-elements/")
	if rest == "" {
		s.writeError(w, http.StatusBadRequest, "missing element ID")
		return
	}

	parts := strings.SplitN(rest, "/", 3)
	elementID := parts[0]
	sub := ""
	if len(parts) >= 2 {
		sub = parts[1]
	}

	switch {
	case sub == "versions" && len(parts) == 2 && r.Method == http.MethodPost:
		s.handleElementVersionCreate(w, r, elementID)
	case sub == "versions" && len(parts) == 3 && r.Method == http.MethodDelete:
		s.handleElementDelete(w, elementID)
	default:
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleElementVersionCreate handles POST /api/v1/consent-elements/{id}/versions.
// The mock collapses versioning into an in-place update — Thunder reads the latest as the
// only version, which matches the previous edit-in-place semantics.
func (s *MockConsentServer) handleElementVersionCreate(w http.ResponseWriter, r *http.Request, elementID string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	var inp mockConsentElementVersionDTO
	if err := json.Unmarshal(body, &inp); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	s.mu.Lock()
	el, exists := s.elements[elementID]
	if !exists {
		s.mu.Unlock()
		s.writeError(w, http.StatusNotFound, "consent element not found")
		return
	}

	el.DisplayName = inp.DisplayName
	el.Description = inp.Description
	el.Properties = inp.Properties
	el.Version = "v2"
	resp := el.mockConsentElementDTO
	s.mu.Unlock()

	s.writeJSON(w, http.StatusCreated, resp)
}

// handleElementDelete handles DELETE /api/v1/consent-elements/{id}/versions/{version}.
func (s *MockConsentServer) handleElementDelete(w http.ResponseWriter, elementID string) {
	s.mu.Lock()
	_, exists := s.elements[elementID]
	if !exists {
		s.mu.Unlock()
		s.writeError(w, http.StatusNotFound, "consent element not found")
		return
	}

	delete(s.elements, elementID)
	s.mu.Unlock()

	w.WriteHeader(http.StatusNoContent)
}

// --- Consent Purposes handlers ---

// handlePurposes handles POST (create) and GET (list) on /api/v1/consent-purposes.
func (s *MockConsentServer) handlePurposes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handlePurposeCreate(w, r)
	case http.MethodGet:
		s.handlePurposesList(w, r)
	default:
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handlePurposeCreate handles POST /api/v1/consent-purposes.
// The group-id header carries the application (group) ID per the v0.3.0 contract.
func (s *MockConsentServer) handlePurposeCreate(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	var inp mockConsentPurposeCreateDTO
	if err := json.Unmarshal(body, &inp); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	groupID := r.Header.Get("group-id")
	now := time.Now().UnixMilli()

	s.mu.Lock()
	p := &mockConsentPurpose{
		PurposeID:   s.nextIDLocked(),
		Name:        inp.Name,
		GroupID:     groupID,
		Version:     "v1",
		DisplayName: inp.DisplayName,
		Description: inp.Description,
		Properties:  inp.Properties,
		Elements:    inp.Elements,
		CreatedTime: now,
		UpdatedTime: now,
	}
	s.purposes[p.PurposeID] = p
	resp := *p
	s.mu.Unlock()

	s.writeJSON(w, http.StatusCreated, resp)
}

// handlePurposesList handles GET /api/v1/consent-purposes with optional ?groupIds= filter.
func (s *MockConsentServer) handlePurposesList(w http.ResponseWriter, r *http.Request) {
	groupIDFilter := r.URL.Query().Get("groupIds")

	s.mu.Lock()
	list := make([]mockConsentPurposeDTO, 0, len(s.purposes))
	for _, p := range s.purposes {
		if groupIDFilter == "" || p.GroupID == groupIDFilter {
			list = append(list, *p)
		}
	}
	s.mu.Unlock()

	s.writeJSON(w, http.StatusOK, map[string]interface{}{"data": list})
}

// handlePurposeByID routes requests under /api/v1/consent-purposes/{id}.
// Supports GET on /{id} (single fetch) and POST on /{id}/versions (update via new version).
func (s *MockConsentServer) handlePurposeByID(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/v1/consent-purposes/")
	if rest == "" {
		s.writeError(w, http.StatusBadRequest, "missing purpose ID")
		return
	}

	parts := strings.SplitN(rest, "/", 2)
	purposeID := parts[0]
	sub := ""
	if len(parts) == 2 {
		sub = parts[1]
	}

	switch {
	case sub == "" && r.Method == http.MethodGet:
		s.handlePurposeGet(w, purposeID)
	case sub == "versions" && r.Method == http.MethodPost:
		s.handlePurposeVersionCreate(w, r, purposeID)
	default:
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handlePurposeGet handles GET /api/v1/consent-purposes/{id}.
func (s *MockConsentServer) handlePurposeGet(w http.ResponseWriter, purposeID string) {
	s.mu.Lock()
	p, exists := s.purposes[purposeID]
	if !exists {
		s.mu.Unlock()
		s.writeError(w, http.StatusNotFound, "consent purpose not found")
		return
	}
	resp := *p
	s.mu.Unlock()

	s.writeJSON(w, http.StatusOK, resp)
}

// handlePurposeVersionCreate handles POST /api/v1/consent-purposes/{id}/versions.
// As with element versioning, the mock collapses this into an in-place update.
func (s *MockConsentServer) handlePurposeVersionCreate(w http.ResponseWriter, r *http.Request, purposeID string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	var inp mockConsentPurposeVersionDTO
	if err := json.Unmarshal(body, &inp); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	groupID := r.Header.Get("group-id")

	s.mu.Lock()
	p, exists := s.purposes[purposeID]
	if !exists {
		s.mu.Unlock()
		s.writeError(w, http.StatusNotFound, "consent purpose not found")
		return
	}

	p.DisplayName = inp.DisplayName
	p.Description = inp.Description
	p.Properties = inp.Properties
	p.Elements = inp.Elements
	p.Version = "v2"
	p.UpdatedTime = time.Now().UnixMilli()
	if groupID != "" {
		p.GroupID = groupID
	}
	resp := *p
	s.mu.Unlock()

	s.writeJSON(w, http.StatusCreated, resp)
}

// --- Test inspection endpoints (not part of OpenFGC API) ---

// handleTestPurposes handles GET /test/purposes?groupIds=<appID>.
// Returns all purposes stored for the given group ID so tests can verify state.
func (s *MockConsentServer) handleTestPurposes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	groupIDFilter := r.URL.Query().Get("groupIds")

	s.mu.Lock()
	list := make([]mockConsentPurposeDTO, 0)
	for _, p := range s.purposes {
		if groupIDFilter == "" || p.GroupID == groupIDFilter {
			list = append(list, *p)
		}
	}
	s.mu.Unlock()

	s.writeJSON(w, http.StatusOK, list)
}

// handleTestReset handles POST /test/reset.
// Clears all stored elements and purposes.
func (s *MockConsentServer) handleTestReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	s.mu.Lock()
	s.elements = make(map[string]*mockConsentElement)
	s.purposes = make(map[string]*mockConsentPurpose)
	s.idSeq = 0
	s.mu.Unlock()

	s.writeJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}
