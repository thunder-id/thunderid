package scim

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/user"
)

// Initialize sets up the SCIM module and registers all /scim/v2 routes.
func Initialize(
	mux *http.ServeMux,
	userService user.UserServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	baseURL string,
	scimCfg config.SCIMConfig,
) {
	svc := newSCIMService(userService, entityTypeService, scimCfg)
	h := newSCIMHandler(svc, baseURL)
	registerSCIMRoutes(mux, h)
}

// registerSCIMRoutes registers all /scim/v2 routes using the same
// middleware.WithCORS pattern as all other ThunderID modules.
func registerSCIMRoutes(mux *http.ServeMux, h *scimHandler) {
	optsGet := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	// ServiceProviderConfig — Phase 1 implemented endpoint.
	mux.HandleFunc(middleware.WithCORS(
		"GET /scim/v2/ServiceProviderConfig",
		h.HandleServiceProviderConfigGetRequest,
		optsGet,
	))
	mux.HandleFunc(middleware.WithCORS(
		"OPTIONS /scim/v2/ServiceProviderConfig",
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		optsGet,
	))

	// Unimplemented endpoints — return 501 per SCIM spec.
	for _, pattern := range []string{
		"GET /scim/v2/Users",
		"POST /scim/v2/Users",
		"PUT /scim/v2/Users",
		"DELETE /scim/v2/Users",
		"GET /scim/v2/Groups",
		"POST /scim/v2/Groups",
		"PUT /scim/v2/Groups",
		"DELETE /scim/v2/Groups",
		"POST /scim/v2/Bulk",
		"POST /scim/v2/.search",
	} {
		mux.HandleFunc(pattern, h.HandleUnsupportedRequest)
	}
}
