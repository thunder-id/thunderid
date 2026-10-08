// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// notificationTemplateHandler is the HTTP handler for notification template operations.
type notificationTemplateHandler struct {
	service NotificationTemplateServiceInterface
	logger  *log.Logger
}

// newNotificationTemplateHandler creates a new handler instance.
func newNotificationTemplateHandler(
	service NotificationTemplateServiceInterface) *notificationTemplateHandler {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "NotificationTemplateHandler"))
	return &notificationTemplateHandler{
		service: service,
		logger:  logger,
	}
}

// HandleTemplateListRequest handles GET /notification-templates/{channel}/templates.
func (h *notificationTemplateHandler) HandleTemplateListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	channel := ChannelType(r.PathValue("channel"))

	limit, offset, svcErr := parsePaginationParams(r.URL.Query())
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	list, svcErr := h.service.ListTemplates(ctx, channel, limit, offset)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, list)
	h.logger.Debug(ctx, "Successfully listed notification templates", log.String("channel", string(channel)),
		log.Int("count", len(list.Templates)))
}

// parsePaginationParams reads the limit and offset query parameters, applying the default page size
// when limit is unspecified. Non-numeric values are rejected; range checks happen in the service.
func parsePaginationParams(query url.Values) (int, int, *tidcommon.ServiceError) {
	limit := 0
	offset := 0
	if limitStr := query.Get("limit"); limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil {
			return 0, 0, &ErrorInvalidLimit
		}
		limit = parsed
	}
	if offsetStr := query.Get("offset"); offsetStr != "" {
		parsed, err := strconv.Atoi(offsetStr)
		if err != nil {
			return 0, 0, &ErrorInvalidOffset
		}
		offset = parsed
	}
	if limit == 0 {
		limit = serverconst.DefaultPageSize
	}
	return limit, offset, nil
}

// HandleTemplatePostRequest handles POST /notification-templates/{channel}/templates.
func (h *notificationTemplateHandler) HandleTemplatePostRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	channel := ChannelType(r.PathValue("channel"))

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

	w.Header().Set(serverconst.LocationHeaderName, buildSelf(channel, created.ID))
	sysutils.WriteSuccessResponse(ctx, w, http.StatusCreated, created)
	h.logger.Debug(ctx, "Successfully created notification template", log.String("id", created.ID))
}

// HandleTemplateGetRequest handles GET /notification-templates/{channel}/templates/{id}.
func (h *notificationTemplateHandler) HandleTemplateGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	channel := ChannelType(r.PathValue("channel"))
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
	channel := ChannelType(r.PathValue("channel"))
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
	channel := ChannelType(r.PathValue("channel"))
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
		case ErrorTemplateInUse.Code, ErrorTemplateHandleConflict.Code:
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
