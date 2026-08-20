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

package secretstore

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// StoreServer serves a writable secret store.
//
// Writes exist because a secret is propagated the moment it is created or updated on a control plane,
// not when configuration is next promoted: a credential that reaches a data plane late is a credential
// that rejects logins in between.
type StoreServer struct {
	store *Store
	token string
}

// NewStoreServer builds a StoreServer.
func NewStoreServer(store *Store, token string) *StoreServer {
	return &StoreServer{store: store, token: token}
}

// Handler returns the HTTP handler with all routes registered.
func (s *StoreServer) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		// Counted from what is already held rather than by reading the backing store, so a health
		// check cannot turn into a call to the key vault on every poll.
		writeJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "secrets": s.store.Count()})
	})

	mux.HandleFunc("GET /secrets", s.guard(s.list))
	// Registered before the {name} route so the literal segment wins.
	mux.HandleFunc("GET /secrets/names", s.guard(s.names))
	mux.HandleFunc("PUT /secrets/{name}", s.guard(s.put))
	mux.HandleFunc("GET /secrets/{name}", s.guard(s.get))
	mux.HandleFunc("DELETE /secrets/{name}", s.guard(s.delete))
	mux.HandleFunc("POST /secrets/{name}/verify", s.guard(s.verify))
	return mux
}

// list returns every secret, which is what a data plane loads at startup.
func (s *StoreServer) list(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{"secrets": s.store.All(r.Context())})
}

// names returns the stored names and kinds without any value.
//
// It exists so a caller can check that every credential a configuration needs is present before
// applying it. Answering that question with the full listing would mean shipping secrets to something
// that only needs to know they exist.
func (s *StoreServer) names(w http.ResponseWriter, r *http.Request) {
	all := s.store.All(r.Context())
	kinds := make(map[string]Kind, len(all))
	for name, secret := range all {
		kinds[name] = secret.Kind
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"names": s.store.Names(r.Context()), "kinds": kinds})
}

// get returns one secret, which is what a data plane refetches on a cache miss.
func (s *StoreServer) get(w http.ResponseWriter, r *http.Request) {
	secret, ok := s.store.Get(r.Context(), r.PathValue("name"))
	if !ok {
		writeError(w, http.StatusNotFound, "no such secret")
		return
	}
	writeJSON(w, http.StatusOK, secret)
}

// putSecretRequest is the body of a write. The name comes from the path.
type putSecretRequest struct {
	Kind        Kind           `json:"kind"`
	Value       string         `json:"value"`
	Algorithm   string         `json:"algorithm,omitempty"`
	Parameters  HashParameters `json:"parameters,omitempty"`
	Description string         `json:"description,omitempty"`
}

// put stores or replaces a secret.
func (s *StoreServer) put(w http.ResponseWriter, r *http.Request) {
	var req putSecretRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	secret := Secret{
		Name:        r.PathValue("name"),
		Kind:        req.Kind,
		Value:       req.Value,
		Algorithm:   req.Algorithm,
		Parameters:  req.Parameters,
		Description: req.Description,
	}
	if err := s.store.Put(r.Context(), secret); err != nil {
		if errors.Is(err, ErrInvalidSecret) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to store the secret")
		return
	}
	// The stored value is not echoed back.
	writeJSON(w, http.StatusOK, secret.Redacted())
}

// delete removes a secret.
func (s *StoreServer) delete(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Delete(r.Context(), r.PathValue("name")); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete the secret")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// verify reports whether a presented value matches a stored hash.
//
// It exists so a caller can check a credential without holding the hash. Verification is refused for a
// readable secret: those are meant to be replayed to a third party, and answering here would turn this
// into an oracle for guessing them.
func (s *StoreServer) verify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	secret, ok := s.store.Get(r.Context(), r.PathValue("name"))
	if !ok {
		writeError(w, http.StatusNotFound, "no such secret")
		return
	}
	if !secret.IsVerifiable() {
		writeError(w, http.StatusBadRequest, "this secret is stored as a readable value and cannot be verified")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"matches": verifyHash(req.Value, secret)})
}

// guard applies the shared-token check.
func (s *StoreServer) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.token == "" {
			next(w, r)
			return
		}
		if !tokenMatches(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), s.token) {
			writeError(w, http.StatusUnauthorized, "a valid bearer token is required")
			return
		}
		next(w, r)
	}
}

// writeJSON writes a JSON response. Secrets must never be cached by an intermediary.
func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// writeError writes a JSON error body with the given status.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
