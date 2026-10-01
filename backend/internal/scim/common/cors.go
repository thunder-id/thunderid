// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// ReadOnlyCORSOptions are the CORS options for read-only SCIM endpoints.
var ReadOnlyCORSOptions = middleware.CORSOptions{
	AllowedMethods:   []string{"GET"},
	AllowedHeaders:   middleware.DefaultAllowedHeaders,
	AllowCredentials: true,
	MaxAge:           600,
}

// CRUDCORSOptions are the CORS options for SCIM resource endpoints.
var CRUDCORSOptions = middleware.CORSOptions{
	AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
	AllowedHeaders:   middleware.DefaultAllowedHeaders,
	AllowCredentials: true,
	MaxAge:           600,
}

// HandlePreflightRequest responds to a CORS preflight request with 204 No Content.
func HandlePreflightRequest(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}
