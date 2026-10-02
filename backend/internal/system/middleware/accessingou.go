// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"net/http"

	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// PathParamOUID is the path parameter an organization-unit-prefixed route names, as in
// /ou/{ouId}/oauth2/token. Routes and the middleware share it so a rename cannot leave the pattern
// and the lookup disagreeing, which would read as every request arriving without an organization
// unit rather than as a broken route.
const PathParamOUID = "ouId"

// AccessingOUMiddleware resolves the organization unit named in the path and records it on the
// request context, where admission and claim resolution both read it from.
//
// The existence check belongs in middleware rather than in any one handler because it is a property
// of the request: a policy check alone cannot reject an id that names nothing, since a
// deployment-wide policy matches every organization unit without consulting the tree. Without this,
// such an id would survive admission and fail much later, as an internal error.
//
// A route carrying no organization unit passes through completely untouched, so an endpoint can
// serve both its bare and its prefixed form from one handler.
//
// refuse writes the rejection, and is supplied by the caller because its shape belongs to the
// protocol the endpoint speaks: the token endpoint answers in OAuth2's error envelope and
// deliberately gives the same answer here as it gives a client that was never shared, so that the
// endpoint cannot be used to tell which organization units exist. An endpoint speaking a different
// protocol answers in its own shape, and decides for itself whether it has that concern.
func AccessingOUMiddleware(
	ouService providers.OrganizationUnitProvider,
	refuse func(w http.ResponseWriter, r *http.Request, ouID string),
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			accessingOUID := r.PathValue(PathParamOUID)
			if accessingOUID == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Fail closed: without a resolver there is no way to establish the organization unit
			// exists, and admitting it would let an unknown id through to a blanket policy.
			if ouService == nil {
				refuse(w, r, accessingOUID)
				return
			}
			ctx := r.Context()
			if _, svcErr := ouService.GetOrganizationUnit(ctx, accessingOUID); svcErr != nil {
				refuse(w, r, accessingOUID)
				return
			}

			next.ServeHTTP(w, r.WithContext(syscontext.WithAccessingOUID(ctx, accessingOUID)))
		})
	}
}
