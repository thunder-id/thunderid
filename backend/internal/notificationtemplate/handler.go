// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"context"
	"net/http"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

const handlerLoggerComponentName = "NotificationTemplateHandler"

// notificationTemplateHandler is the HTTP handler for notification template operations.
type notificationTemplateHandler struct {
	service NotificationTemplateServiceInterface
	logger  *log.Logger
}

// newNotificationTemplateHandler creates a new handler instance.
func newNotificationTemplateHandler(
	service NotificationTemplateServiceInterface) *notificationTemplateHandler {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))
	return &notificationTemplateHandler{
		service: service,
		logger:  logger,
	}
}

// HandleTemplateListRequest handles GET /notification-templates/{channel}/templates.
func (h *notificationTemplateHandler) HandleTemplateListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	channel := r.PathValue("channel")

	list, svcErr := h.service.ListTemplates(ctx, channel)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, list)
	h.logger.Debug(ctx, "Successfully listed notification templates", log.String("channel", channel),
		log.Int("count", len(list.Templates)))
}

// HandleTemplatePostRequest handles POST /notification-templates/{channel}/templates.
func (h *notificationTemplateHandler) HandleTemplatePostRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	channel := r.PathValue("channel")

	request, err := sysutils.DecodeJSONBody[CreateTemplateRequest](r)
	if err != nil {
		handleError(ctx, w, &ErrorInvalidTemplateData)
		return
	}

	created, svcErr := h.service.CreateTemplate(ctx, channel, *request)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	w.Header().Set("Location", buildSelf(channel, created.ID))
	sysutils.WriteSuccessResponse(ctx, w, http.StatusCreated, created)
	h.logger.Debug(ctx, "Successfully created notification template", log.String("id", created.ID))
}

// HandleTemplateGetRequest handles GET /notification-templates/{channel}/templates/{id}.
func (h *notificationTemplateHandler) HandleTemplateGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	channel := r.PathValue("channel")
	id := r.PathValue("id")

	template, svcErr := h.service.GetTemplate(ctx, channel, id)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, template)
	h.logger.Debug(ctx, "Successfully retrieved notification template", log.String("id", id))
}

// HandleTemplatePutRequest handles PUT /notification-templates/{channel}/templates/{id}.
func (h *notificationTemplateHandler) HandleTemplatePutRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	channel := r.PathValue("channel")
	id := r.PathValue("id")

	request, err := sysutils.DecodeJSONBody[UpdateTemplateRequest](r)
	if err != nil {
		handleError(ctx, w, &ErrorInvalidTemplateData)
		return
	}

	updated, svcErr := h.service.UpdateTemplate(ctx, channel, id, *request)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, updated)
	h.logger.Debug(ctx, "Successfully updated notification template", log.String("id", id))
}

// HandleTemplateDeleteRequest handles DELETE /notification-templates/{channel}/templates/{id}.
func (h *notificationTemplateHandler) HandleTemplateDeleteRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	channel := r.PathValue("channel")
	id := r.PathValue("id")

	svcErr := h.service.DeleteTemplate(ctx, channel, id)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusNoContent, nil)
	h.logger.Debug(ctx, "Successfully deleted notification template", log.String("id", id))
}

// handleError maps a service error to the appropriate HTTP status and error body.
func handleError(ctx context.Context, w http.ResponseWriter, svcErr *tidcommon.ServiceError) {
	statusCode := http.StatusInternalServerError
	if svcErr.Type == tidcommon.ClientErrorType {
		switch svcErr.Code {
		case ErrorTemplateNotFound.Code:
			statusCode = http.StatusNotFound
		case ErrorTemplateInUse.Code, ErrorTemplateNameConflict.Code:
			statusCode = http.StatusConflict
		default:
			statusCode = http.StatusBadRequest
		}
	}

	errResp := apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	}

	sysutils.WriteErrorResponse(ctx, w, statusCode, errResp)
}
