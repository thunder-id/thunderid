// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package clientauth

import (
	"context"
	"net/http"

	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// ClientOUAdmissionMiddleware refuses a request whose authenticated client may not act for the
// organization unit the request named, and must be mounted after ClientAuthMiddleware.
func ClientOUAdmissionMiddleware(actorProvider providers.ActorProvider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			accessingOUID := syscontext.GetAccessingOUID(ctx)
			if accessingOUID == "" {
				next.ServeHTTP(w, r)
				return
			}

			clientInfo := GetOAuthClient(ctx)
			if actorProvider == nil || clientInfo == nil {
				writeAuthError(ctx, w, errClientNotAuthorizedForOU)
				return
			}

			accessible, svcErr := actorProvider.IsOAuthClientAccessibleFromOU(
				ctx, clientInfo.OAuthApp, accessingOUID)
			if svcErr != nil {
				log.GetLogger().Error(ctx, "Failed to resolve client access for an organization unit",
					log.MaskedString("clientID", clientInfo.ClientID),
					log.String("ouId", accessingOUID), log.Any("error", svcErr))
				writeAuthError(ctx, w, errOUAccessUnresolved)
				return
			}
			if !accessible {
				log.GetLogger().Debug(ctx, "Client is not authorized for the requested organization unit",
					log.MaskedString("clientID", clientInfo.ClientID), log.String("ouId", accessingOUID))
				writeAuthError(ctx, w, errClientNotAuthorizedForOU)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// writeAuthError answers with one of this package's client-authentication errors.
func writeAuthError(ctx context.Context, w http.ResponseWriter, authErr *authError) {
	utils.WriteJSONError(ctx, w, authErr.ErrorCode, authErr.ErrorDescription, authErr.StatusCode, nil)
}
