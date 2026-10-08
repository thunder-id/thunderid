// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"context"
	"net/http"

	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// PathParamOUID is the path parameter an organization-unit-prefixed route names, as in
// /ou/{ouId}/oauth2/token. Routes and the middleware share it so a rename cannot leave them
// disagreeing.
const PathParamOUID = "ouId"

// AccessingOUResponse is one answer an endpoint gives when a request may not proceed, in the
// vocabulary of the protocol that endpoint speaks.
type AccessingOUResponse struct {
	Code        string
	Description string
	StatusCode  int
}

// write sends the response.
func (r AccessingOUResponse) write(ctx context.Context, w http.ResponseWriter) {
	utils.WriteJSONError(ctx, w, r.Code, r.Description, r.StatusCode, nil)
}

// AccessingOUMiddleware resolves the organization unit named in the path and records it on the
// request context. A route carrying none passes through untouched.
//
// An unknown id is refused here because a policy check cannot reject one: a deployment-wide policy
// matches every organization unit without consulting the tree. refusal answers that, failure a
// lookup that broke, both in the endpoint's own protocol vocabulary.
func AccessingOUMiddleware(
	ouService providers.OrganizationUnitProvider,
	refusal, failure AccessingOUResponse,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			accessingOUID := r.PathValue(PathParamOUID)
			if accessingOUID == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Fail closed: with no resolver, an unknown id would reach a blanket policy. The
			// deployment did not wire one, which is not the caller's doing.
			if ouService == nil {
				failure.write(ctx, w)
				return
			}
			if _, svcErr := ouService.GetOrganizationUnit(ctx, accessingOUID); svcErr != nil {
				if svcErr.Type == tidcommon.ServerErrorType {
					failure.write(ctx, w)
					return
				}
				refusal.write(ctx, w)
				return
			}

			next.ServeHTTP(w, r.WithContext(syscontext.WithAccessingOUID(ctx, accessingOUID)))
		})
	}
}
