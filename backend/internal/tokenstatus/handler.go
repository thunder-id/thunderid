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

package tokenstatus

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// statusListMediaTypeJWT is the media type of a JWT-encoded Status List Token (draft-ietf-oauth-status-list).
const statusListMediaTypeJWT = "application/statuslist+jwt"

// statusListMediaTypeCWT is the CWT media type, not yet produced; a request that accepts only it gets 406.
const statusListMediaTypeCWT = "application/statuslist+cwt"

// statusListRoute is the path pattern of the publish endpoint. The path is neutral (not under /oauth2)
// because the URI is immutable once stamped into tokens and the artifact is credential-format-agnostic.
const statusListRoute = "/statuslists/{id}"

// handler serves the public, unauthenticated Status List Token endpoint. What makes the response
// trustworthy is the signature on the token, not access control on the route.
type handler struct {
	service ServiceInterface
	logger  *log.Logger
}

func newHandler(service ServiceInterface) *handler {
	return &handler{
		service: service,
		logger:  log.GetLogger().With(log.String(log.LoggerKeyComponentName, "StatusListHandler")),
	}
}

// handleGet serves GET /statuslists/{id}: it produces the signed Status List Token and returns it with
// the matching content type and a cache lifetime derived from the token's ttl.
func (h *handler) handleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	if !acceptsJWT(r.Header.Get("Accept")) {
		http.Error(w, "only "+statusListMediaTypeJWT+" is supported", http.StatusNotAcceptable)
		return
	}

	token, ttl, err := h.service.Produce(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrListNotFound) {
			http.NotFound(w, r)
			return
		}
		h.logger.Error(r.Context(), "Failed to produce status list token", log.Error(err))
		http.Error(w, "failed to produce status list token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", statusListMediaTypeJWT)
	w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d", ttl))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(token))
}

// acceptsJWT reports whether the Accept header permits the JWT representation. It serves JWT by default
// and only declines (406) when the client explicitly restricts itself to the not-yet-supported CWT form.
func acceptsJWT(accept string) bool {
	if accept == "" ||
		strings.Contains(accept, statusListMediaTypeJWT) ||
		strings.Contains(accept, "*/*") ||
		strings.Contains(accept, "application/*") {
		return true
	}
	return !strings.Contains(accept, statusListMediaTypeCWT)
}

// RegisterRoutes registers the public status list publish endpoint on mux. It is called by the
// composition root when the subsystem acts as a Status Provider. The endpoint is unauthenticated and
// CORS-enabled (no credentials) so the signed artifact can be shared and cached without defeating herd
// privacy.
func RegisterRoutes(mux *http.ServeMux, service ServiceInterface) {
	h := newHandler(service)
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "OPTIONS"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: false,
		MaxAge:           600,
	}

	pattern, wrapped := middleware.WithCORS("GET "+statusListRoute, h.handleGet, opts)
	mux.HandleFunc(pattern, wrapped)
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+statusListRoute,
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
}
