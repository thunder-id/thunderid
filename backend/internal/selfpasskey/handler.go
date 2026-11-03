// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package selfpasskey

import (
	"context"
	"net/http"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/authn/passkey"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/security"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

const (
	// claimClientID names the OAuth client an access token was issued to.
	claimClientID = "client_id"
	// claimActor marks a token that an actor uses on behalf of the subject.
	claimActor = "act"
)

// selfPasskeyHandler handles the self-service passkey API requests.
type selfPasskeyHandler struct {
	service selfPasskeyServiceInterface
}

// newSelfPasskeyHandler creates a new self-service passkey handler.
func newSelfPasskeyHandler(service selfPasskeyServiceInterface) *selfPasskeyHandler {
	return &selfPasskeyHandler{service: service}
}

// HandleRegistrationStartRequest handles the request to start adding a passkey.
func (h *selfPasskeyHandler) HandleRegistrationStartRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, svcErr := callerUserID(ctx)
	if svcErr != nil {
		writeError(ctx, w, svcErr)
		return
	}

	req, err := sysutils.DecodeJSONBody[registrationStartRequest](r)
	if err != nil {
		writeError(ctx, w, &ErrorInvalidRequestFormat)
		return
	}

	data, svcErr := h.service.StartRegistration(ctx, userID, callerClientID(ctx), req.RelyingPartyID)
	if svcErr != nil {
		writeError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, data)
}

// HandleRegistrationFinishRequest handles the request to save a new passkey.
func (h *selfPasskeyHandler) HandleRegistrationFinishRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, svcErr := callerUserID(ctx)
	if svcErr != nil {
		writeError(ctx, w, svcErr)
		return
	}

	req, err := sysutils.DecodeJSONBody[registrationFinishRequest](r)
	if err != nil {
		writeError(ctx, w, &ErrorInvalidRequestFormat)
		return
	}

	saved, svcErr := h.service.FinishRegistration(ctx, userID, callerClientID(ctx), req)
	if svcErr != nil {
		writeError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusCreated, saved)
}

// HandleListRequest handles the request to list the user's passkeys.
func (h *selfPasskeyHandler) HandleListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, svcErr := callerUserID(ctx)
	if svcErr != nil {
		writeError(ctx, w, svcErr)
		return
	}

	passkeys, svcErr := h.service.ListPasskeys(ctx, userID)
	if svcErr != nil {
		writeError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, passkeyListResponse{Passkeys: passkeys})
}

// HandleDeleteRequest handles the request to remove one of the user's passkeys.
func (h *selfPasskeyHandler) HandleDeleteRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, svcErr := callerUserID(ctx)
	if svcErr != nil {
		writeError(ctx, w, svcErr)
		return
	}

	if svcErr := h.service.DeletePasskey(ctx, userID, r.PathValue("id")); svcErr != nil {
		writeError(ctx, w, svcErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// callerUserID returns the signed-in user, refusing tokens that act on the user's behalf.
func callerUserID(ctx context.Context) (string, *tidcommon.ServiceError) {
	userID := security.GetSubject(ctx)
	if strings.TrimSpace(userID) == "" {
		return "", &ErrorAuthenticationRequired
	}
	if security.GetAttribute(ctx, claimActor) != nil {
		return "", &ErrorDelegatedTokenNotAllowed
	}
	return userID, nil
}

// callerClientID returns the OAuth client the caller's token was issued to.
func callerClientID(ctx context.Context) string {
	clientID, _ := security.GetAttribute(ctx, claimClientID).(string)
	return clientID
}

// writeError writes a service error with its HTTP status.
func writeError(ctx context.Context, w http.ResponseWriter, svcErr *tidcommon.ServiceError) {
	statusCode := http.StatusInternalServerError
	if svcErr.Type == tidcommon.ClientErrorType {
		switch svcErr.Code {
		case ErrorAuthenticationRequired.Code:
			statusCode = http.StatusUnauthorized
		case ErrorDelegatedTokenNotAllowed.Code, common.ErrorAccountSuspended.Code, common.ErrorAccountExpired.Code:
			statusCode = http.StatusForbidden
		case passkey.ErrorCredentialNotFound.Code, passkey.ErrorUserNotFound.Code:
			statusCode = http.StatusNotFound
		default:
			statusCode = http.StatusBadRequest
		}
	}

	sysutils.WriteErrorResponse(ctx, w, statusCode, apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	})
}
