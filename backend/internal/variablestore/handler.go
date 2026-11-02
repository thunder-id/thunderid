// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package variablestore

import (
	"context"
	"errors"
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

const loggerComponentName = "VariableStoreHandler"

type handler struct {
	service ServiceInterface
}

func newHandler(service ServiceInterface) *handler {
	return &handler{service: service}
}

// HandleVariableListRequest lists variables.
func (h *handler) HandleVariableListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query, svcErr := parseListQuery(r.URL.Query())
	if svcErr != nil {
		h.handleError(ctx, w, svcErr)
		return
	}

	response, svcErr := h.service.ListVariables(ctx, query)
	if svcErr != nil {
		h.handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, response)
}

// HandleVariablePostRequest creates a variable.
func (h *handler) HandleVariablePostRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	request, ok := decodeBody[VariableRequest](ctx, w, r)
	if !ok {
		return
	}

	created, svcErr := h.service.CreateVariable(ctx, *request)
	if svcErr != nil {
		h.handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusCreated, created)
	logger.Debug(ctx, "Created variable", log.String("name", created.Name))
}

// HandleVariableGetRequest returns one variable.
func (h *handler) HandleVariableGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	variable, svcErr := h.service.GetVariable(ctx, r.PathValue("name"))
	if svcErr != nil {
		h.handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, variable)
}

// HandleVariablePutRequest creates a variable or replaces the one already there.
func (h *handler) HandleVariablePutRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	request, ok := decodeBody[VariableUpdateRequest](ctx, w, r)
	if !ok {
		return
	}

	updated, created, svcErr := h.service.UpdateVariable(ctx, r.PathValue("name"), *request)
	if svcErr != nil {
		h.handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, statusFor(created), updated)
	logger.Debug(ctx, "Stored variable", log.String("name", updated.Name), log.Bool("created", created))
}

// HandleVariableDeleteRequest removes a variable, whether or not one was there.
func (h *handler) HandleVariableDeleteRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	name := r.PathValue("name")
	if svcErr := h.service.DeleteVariable(ctx, name); svcErr != nil {
		h.handleError(ctx, w, svcErr)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	logger.Debug(ctx, "Deleted variable", log.String("name", name))
}

// HandleSecretListRequest lists the secrets held, by name.
func (h *handler) HandleSecretListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query, svcErr := parseListQuery(r.URL.Query())
	if svcErr != nil {
		h.handleError(ctx, w, svcErr)
		return
	}

	response, svcErr := h.service.ListSecrets(ctx, query)
	if svcErr != nil {
		h.handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, response)
}

// HandleSecretPostRequest stores a secret.
func (h *handler) HandleSecretPostRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	request, ok := decodeBody[SecretRequest](ctx, w, r)
	if !ok {
		return
	}

	created, svcErr := h.service.CreateSecret(ctx, *request)
	if svcErr != nil {
		h.handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusCreated, created)
	// The name is logged; the value is not, here or anywhere else.
	logger.Debug(ctx, "Created secret", log.String("name", created.Name))
}

// HandleSecretGetRequest reports whether a secret is held. It never returns the value.
func (h *handler) HandleSecretGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	secret, svcErr := h.service.GetSecret(ctx, r.PathValue("name"))
	if svcErr != nil {
		h.handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, secret)
}

// HandleSecretPutRequest stores a secret or rotates the one already there.
func (h *handler) HandleSecretPutRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	request, ok := decodeBody[SecretUpdateRequest](ctx, w, r)
	if !ok {
		return
	}

	updated, created, svcErr := h.service.UpdateSecret(ctx, r.PathValue("name"), *request)
	if svcErr != nil {
		h.handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, statusFor(created), updated)
	logger.Debug(ctx, "Stored secret", log.String("name", updated.Name), log.Bool("created", created))
}

// HandleSecretDeleteRequest removes a secret, whether or not one was there.
func (h *handler) HandleSecretDeleteRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	name := r.PathValue("name")
	if svcErr := h.service.DeleteSecret(ctx, name); svcErr != nil {
		h.handleError(ctx, w, svcErr)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	logger.Debug(ctx, "Deleted secret", log.String("name", name))
}

// statusFor reports the status a create-or-replace should answer with.
func statusFor(created bool) int {
	if created {
		return http.StatusCreated
	}
	return http.StatusOK
}

// decodeBody reads a request body, writing the response itself when it cannot. It reports whether
// the caller should carry on.
func decodeBody[T any](ctx context.Context, w http.ResponseWriter, r *http.Request) (*T, bool) {
	decoded, err := sysutils.DecodeJSONBody[T](r)
	if err == nil {
		return decoded, true
	}

	var valErr *sysutils.ValidationError
	if errors.As(err, &valErr) {
		sysutils.WriteStructuredErrorResponse(w, http.StatusBadRequest, "Validation Failed", valErr.Errors)
		return nil, false
	}

	sysutils.WriteErrorResponse(ctx, w, http.StatusBadRequest, apierror.ErrorResponse{
		Code:        ErrorInvalidRequestFormat.Code,
		Message:     ErrorInvalidRequestFormat.Error,
		Description: ErrorInvalidRequestFormat.ErrorDescription,
	})
	return nil, false
}

// handleError turns a service error into a response.
func (h *handler) handleError(ctx context.Context, w http.ResponseWriter, svcErr *tidcommon.ServiceError) {
	statusCode := http.StatusInternalServerError
	if svcErr.Type == tidcommon.ClientErrorType {
		switch svcErr.Code {
		case ErrorNotFound.Code:
			statusCode = http.StatusNotFound
		case ErrorAlreadyExists.Code:
			statusCode = http.StatusConflict
		case tidcommon.ErrorUnauthorized.Code:
			statusCode = http.StatusForbidden
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
