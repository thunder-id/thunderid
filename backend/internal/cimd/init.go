// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package cimd

import (
	"net/http"

	syshttp "github.com/thunder-id/thunderid/internal/system/http"

	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize initializes the Client ID Metadata Document service and registers its routes. A document's
// URL is the client identifier itself, which anyone presenting a client_id can name, so documents are
// retrieved through strictClients, whose clients always refuse private and loopback addresses on the
// connected address.
func Initialize(mux *http.ServeMux, strictClients syshttp.OutboundClientFactoryInterface) CIMDServiceInterface {
	cimdService := newCIMDService(strictClients)
	registerRoutes(mux, newCIMDHandler(cimdService))
	return cimdService
}

// registerRoutes registers the routes for Client ID Metadata Document operations.
func registerRoutes(mux *http.ServeMux, cimdHandler *cimdHandler) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST /cimd/preview", cimdHandler.HandlePreviewRequest, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /cimd/preview",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
}
