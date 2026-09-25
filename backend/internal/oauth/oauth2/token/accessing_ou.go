// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// accessingOUMiddleware resolves the organization unit named in the path and records it on the
// request context.
//
// The existence check belongs here rather than in a grant handler because it is a property of the
// request, not of any one grant type: `allOus` matches every organization unit without consulting
// the tree, so a policy check alone cannot reject an id that names nothing. Without this, such an id
// would survive admission and fail much later, during claim resolution, as a 500.
//
// The bare /oauth2/token path carries no value here and passes through completely untouched.
func accessingOUMiddleware(ouService providers.OrganizationUnitProvider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			accessingOUID := r.PathValue(constants.PathParamOUID)
			if accessingOUID == "" {
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()
			// Fail closed: without a resolver there is no way to establish the organization unit
			// exists, and admitting it would let an unknown id through to a blanket policy.
			if ouService == nil {
				utils.WriteJSONError(ctx, w, constants.ErrorInvalidRequest,
					constants.OUAccessRefusalDescription(accessingOUID), http.StatusBadRequest, nil)
				return
			}
			if _, svcErr := ouService.GetOrganizationUnit(ctx, accessingOUID); svcErr != nil {
				utils.WriteJSONError(ctx, w, constants.ErrorInvalidRequest,
					constants.OUAccessRefusalDescription(accessingOUID), http.StatusBadRequest, nil)
				return
			}

			next.ServeHTTP(w, r.WithContext(syscontext.WithAccessingOUID(ctx, accessingOUID)))
		})
	}
}
