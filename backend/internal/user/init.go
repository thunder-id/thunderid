// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/identitygovernance"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the user service and registers its routes.
//
// valueCapturer is optional. Given one, the service hands it every credential it sets on a user, so
// the values the user's export refers to are kept where a reference finds them.
func Initialize(
	mux *http.ServeMux,
	entityService entity.EntityServiceInterface,
	ouService oupkg.OrganizationUnitServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	authzService sysauthz.SystemAuthorizationServiceInterface,
	governance identitygovernance.IdentityGovernanceServiceInterface,
	sharingService sharing.SharingServiceInterface,
	valueCapturer declarativeresource.ValueCapturer,
) (UserServiceInterface, oupkg.OUUserResolver, declarativeresource.ResourceExporter, error) {
	// Step 1: Create service with entity service
	userService := newUserService(authzService, entityService, ouService, entityTypeService, governance,
		valueCapturer, sharingService)
	sharingService.RegisterResourceType(newUserSharingDeclaration(userService))
	entityService.RegisterSharedCategory(providers.EntityCategoryUser, UserSharingType)

	// Step 2: Load user-specific indexed attributes into the entity store.
	if err := entityService.LoadIndexedAttributes(getUserIndexedAttributes()); err != nil {
		return nil, nil, nil, err
	}

	// Step 3: Load declarative resources if user store mode requires it.
	storeMode := getUserStoreMode()
	if storeMode == serverconst.StoreModeDeclarative || storeMode == serverconst.StoreModeComposite {
		if err := entityService.LoadDeclarativeResources(makeUserDeclarativeConfig(userService)); err != nil {
			return nil, nil, nil, err
		}
		// After the users, since a declared policy names a user that must already be there.
		if err := sharingService.LoadDeclarativeResources(
			context.Background(), makeUserSharingConfig(userService)); err != nil {
			return nil, nil, nil, err
		}
	}

	userHandler := newUserHandler(userService)
	registerRoutes(mux, userHandler)

	// Create resolver for OU package to query user data without cross-DB access
	ouUserResolver := newOUUserResolver(entityService, entityTypeService)

	// Create and return exporter
	exporter := newUserExporter(userService, entityService, entityTypeService, sharingService)
	return userService, ouUserResolver, exporter, nil
}

// getUserStoreMode determines the store mode for users from config.
func getUserStoreMode() serverconst.StoreMode {
	store := strings.ToLower(strings.TrimSpace(config.GetServerRuntime().Config.User.Store))
	switch serverconst.StoreMode(store) {
	case serverconst.StoreModeMutable, serverconst.StoreModeDeclarative, serverconst.StoreModeComposite:
		return serverconst.StoreMode(store)
	}
	if declarativeresource.IsDeclarativeModeEnabled() {
		return serverconst.StoreModeDeclarative
	}
	return serverconst.StoreModeMutable
}

// isDeclarativeModeEnabled reports whether users load from resource files only.
func isDeclarativeModeEnabled() bool {
	return getUserStoreMode() == serverconst.StoreModeDeclarative
}

// getUserIndexedAttributes returns the indexed attributes configured for users.
func getUserIndexedAttributes() []string {
	return config.GetServerRuntime().Config.User.IndexedAttributes
}

// registerRoutes registers the routes for user management operations.
func registerRoutes(mux *http.ServeMux, userHandler *userHandler) {
	opts1 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST /users", userHandler.HandleUserPostRequest, opts1))
	mux.HandleFunc(middleware.WithCORS("GET /users", userHandler.HandleUserListRequest, opts1))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /users", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}, opts1))

	opts2 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /users/",
		func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, "/users/")
			segments := strings.Split(path, "/")
			r.SetPathValue("id", segments[0])

			if len(segments) == 1 {
				userHandler.HandleUserGetRequest(w, r)
			} else if len(segments) == 2 && segments[1] == "groups" {
				userHandler.HandleUserGroupsGetRequest(w, r)
			} else if len(segments) == 2 && segments[1] == "usages" {
				userHandler.HandleUserUsagesGetRequest(w, r)
			} else if len(segments) == 2 && segments[1] == sharingPoliciesSegment {
				userHandler.HandleSharingPolicyListRequest(w, r)
			} else if isSharingPolicyPath(segments) {
				r.SetPathValue("policyId", segments[2])
				userHandler.HandleSharingPolicyGetRequest(w, r)
			} else if len(segments) == 2 && segments[1] == "overlay-rules" {
				userHandler.HandleOverlayRuleGetRequest(w, r)
			} else {
				http.NotFound(w, r)
			}
		}, opts2))
	// PUT and DELETE dispatch on the whole path, so a sub-path never reaches the user's own handler.
	mux.HandleFunc(middleware.WithCORS("PUT /users/",
		func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, "/users/")
			segments := strings.Split(path, "/")
			r.SetPathValue("id", segments[0])

			if len(segments) == 1 {
				userHandler.HandleUserPutRequest(w, r)
			} else if isSharingPolicyPath(segments) {
				r.SetPathValue("policyId", segments[2])
				userHandler.HandleSharingPolicyPutRequest(w, r)
			} else {
				http.NotFound(w, r)
			}
		}, opts2))
	mux.HandleFunc(middleware.WithCORS("DELETE /users/",
		func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, "/users/")
			segments := strings.Split(path, "/")
			r.SetPathValue("id", segments[0])

			if len(segments) == 1 {
				userHandler.HandleUserDeleteRequest(w, r)
			} else if isSharingPolicyPath(segments) {
				r.SetPathValue("policyId", segments[2])
				userHandler.HandleSharingPolicyDeleteRequest(w, r)
			} else {
				http.NotFound(w, r)
			}
		}, opts2))
	mux.HandleFunc(middleware.WithCORS("POST /users/",
		func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, "/users/")
			segments := strings.Split(path, "/")
			if len(segments) == 2 && segments[1] == "update-credentials" {
				r.SetPathValue("id", segments[0])
				userHandler.HandleUserCredentialUpdateRequest(w, r)
			} else if len(segments) == 2 && segments[1] == sharingPoliciesSegment {
				r.SetPathValue("id", segments[0])
				userHandler.HandleSharingPolicyPostRequest(w, r)
			} else {
				http.NotFound(w, r)
			}
		}, opts2))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /users/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}, opts2))

	optsSelf := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /users/me", userHandler.HandleSelfUserGetRequest, optsSelf))
	mux.HandleFunc(middleware.WithCORS("PUT /users/me", userHandler.HandleSelfUserPutRequest, optsSelf))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /users/me", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}, optsSelf))

	optsMeta := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /users/me/meta", userHandler.HandleSelfUserMetadataGetRequest, optsMeta))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /users/me/meta", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}, optsMeta))

	mux.HandleFunc(middleware.WithCORS("GET /users/me/ous",
		userHandler.HandleSelfOrganizationUnitsGetRequest, optsMeta))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /users/me/ous",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, optsMeta))

	optsSelfCredentials := middleware.CORSOptions{
		AllowedMethods:   []string{"POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST /users/me/update-credentials",
		userHandler.HandleSelfUserCredentialUpdateRequest, optsSelfCredentials))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /users/me/update-credentials",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, optsSelfCredentials))

	opts3 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /users/tree/{path...}",
		userHandler.HandleUserListByPathRequest, opts3))
	mux.HandleFunc(middleware.WithCORS("POST /users/tree/{path...}",
		userHandler.HandleUserPostByPathRequest, opts3))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /users/tree/{path...}",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts3))
}

// isSharingPolicyPath reports whether path segments name one sharing policy: {id}/sharing-policies/{policyId}.
func isSharingPolicyPath(segments []string) bool {
	return len(segments) == 3 && segments[1] == sharingPoliciesSegment && segments[2] != ""
}
