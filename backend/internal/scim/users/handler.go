// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package users

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	scim "github.com/thunder-id/thunderid/internal/scim/common"
	scimconfig "github.com/thunder-id/thunderid/internal/scim/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

const usersHandlerLoggerComponentName = "SCIMUsersHandler"

// scimUsersHandler handles all /scim/v2/Users HTTP requests.
type scimUsersHandler struct {
	svc             SCIMUsersServiceInterface
	baseURL         string
	logger          log.Logger
	sortSupported   bool
	schemaURNPrefix string
}

// newSCIMUsersHandler creates the users handler around the given service.
func newSCIMUsersHandler(svc SCIMUsersServiceInterface, cfg scimconfig.SCIMConfig) *scimUsersHandler {
	return &scimUsersHandler{
		svc:             svc,
		baseURL:         cfg.PublicURL,
		sortSupported:   cfg.SortSupported,
		schemaURNPrefix: cfg.SchemaURNPrefix,
		logger:          *log.GetLogger().With(log.String(log.LoggerKeyComponentName, usersHandlerLoggerComponentName)),
	}
}

// filterAttrRules returns the Users filter rules with schema URN validation bound to ctx.
func (h *scimUsersHandler) filterAttrRules(ctx context.Context) scim.FilterAttrRules {
	rules := usersFilterAttrRules
	rules.ValidatePrefixedAttr = func(prefix, attr string) *tidcommon.ServiceError {
		return h.svc.ValidateFilterSchemaAttribute(ctx, prefix, attr)
	}
	return rules
}

// HandleUsersListRequest handles GET /scim/v2/Users
func (h *scimUsersHandler) HandleUsersListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if !h.sortSupported && (r.URL.Query().Get("sortBy") != "" || r.URL.Query().Get("sortOrder") != "") {
		scim.HandleSCIMError(w, r, &scim.ErrorSortNotSupported, h.logger)
		return
	}

	// Parse optional SCIM filter — "eq" expressions joined by "and" are supported.
	var parsedFilters map[string]interface{}
	if filterStr := r.URL.Query().Get("filter"); filterStr != "" {
		var svcErr *tidcommon.ServiceError
		parsedFilters, svcErr = scim.ParseSCIMFilterForEq(filterStr, h.filterAttrRules(ctx))
		if svcErr != nil {
			scim.HandleSCIMError(w, r, svcErr, h.logger)
			return
		}
	}
	startIndex, count := scim.ParseSCIMPaginationQueryParams(r)
	attributes := parseCSVQueryParam(r.URL.Query().Get("attributes"))
	excludedAttributes := parseCSVQueryParam(r.URL.Query().Get("excludedAttributes"))
	if svcErr := h.svc.ValidateAttributePaths(ctx, attributes, excludedAttributes); svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, h.logger)
		return
	}

	listResp, svcErr := h.svc.ListUsers(ctx, startIndex, count, parsedFilters, h.baseURL)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, h.logger)
		return
	}

	h.writeUserListResponse(ctx, w, listResp, attributes, excludedAttributes)
	h.logger.Debug(ctx, "SCIM Users list sent", log.Int("totalResults", listResp.TotalResults))
}

// HandleUsersSearchRequest handles POST /scim/v2/Users/.search (RFC 7644 §3.4.3).
func (h *scimUsersHandler) HandleUsersSearchRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if svcErr := scim.ValidateSCIMContentType(r); svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, h.logger)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, scim.MaxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		scim.HandleSCIMError(w, r, &scim.ErrorInvalidRequestBody, h.logger)
		return
	}
	var searchReq scim.SCIMSearchRequest
	if err := json.Unmarshal(body, &searchReq); err != nil {
		scim.HandleSCIMError(w, r, &scim.ErrorInvalidRequestBody, h.logger)
		return
	}
	if !h.sortSupported && (searchReq.SortBy != "" || searchReq.SortOrder != "") {
		scim.HandleSCIMError(w, r, &scim.ErrorSortNotSupported, h.logger)
		return
	}
	if !scim.HasSchemaURN(searchReq.Schemas, scim.SCIMSearchSchemaURN) {
		svcErr := scim.ErrorMissingSchemas
		svcErr.ErrorDescription = tidcommon.I18nMessage{
			Key:          scim.ErrorMissingSchemas.ErrorDescription.Key,
			DefaultValue: fmt.Sprintf("The schemas array must include %q", scim.SCIMSearchSchemaURN),
		}
		scim.HandleSCIMError(w, r, &svcErr, h.logger)
		return
	}
	if svcErr := h.svc.ValidateAttributePaths(ctx, searchReq.Attributes, searchReq.ExcludedAttributes); svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, h.logger)
		return
	}

	var parsedFilters map[string]interface{}
	if searchReq.Filter != "" {
		var svcErr *tidcommon.ServiceError
		parsedFilters, svcErr = scim.ParseSCIMFilterForEq(searchReq.Filter, h.filterAttrRules(ctx))
		if svcErr != nil {
			scim.HandleSCIMError(w, r, svcErr, h.logger)
			return
		}
	}

	startIndex, count := scim.NormalizeSCIMPagination(searchReq.StartIndex, searchReq.Count)

	listResp, svcErr := h.svc.ListUsers(ctx, startIndex, count, parsedFilters, h.baseURL)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, h.logger)
		return
	}

	h.writeUserListResponse(ctx, w, listResp, searchReq.Attributes, searchReq.ExcludedAttributes)
	h.logger.Debug(ctx, "SCIM Users search sent", log.Int("totalResults", listResp.TotalResults))
}

// HandleUsersCreateRequest handles POST /scim/v2/Users
func (h *scimUsersHandler) HandleUsersCreateRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if svcErr := scim.ValidateSCIMContentType(r); svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, h.logger)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, scim.MaxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		scim.HandleSCIMError(w, r, &scim.ErrorInvalidRequestBody, h.logger)
		return
	}
	payload, svcErr := parseAndValidateSCIMUserRequest(body, h.schemaURNPrefix)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, h.logger)
		return
	}
	attributes := parseCSVQueryParam(r.URL.Query().Get("attributes"))
	excludedAttributes := parseCSVQueryParam(r.URL.Query().Get("excludedAttributes"))
	if svcErr := h.svc.ValidateAttributePaths(ctx, attributes, excludedAttributes); svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, h.logger)
		return
	}

	created, svcErr := h.svc.CreateUser(ctx, payload, h.baseURL)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, h.logger)
		return
	}

	w.Header().Set("Location", created.Meta.Location)
	h.writeUserResponse(ctx, w, http.StatusCreated, created, attributes, excludedAttributes)
	h.logger.Debug(ctx, "SCIM User created", log.MaskedString(log.LoggerKeyUserID, created.ID))
}

// HandleUsersGetRequest handles GET /scim/v2/Users/{id}
func (h *scimUsersHandler) HandleUsersGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.PathValue("id")
	if userID == "" {
		scim.HandleSCIMError(w, r, &scim.ErrorUserNotFound, h.logger)
		return
	}
	attributes := parseCSVQueryParam(r.URL.Query().Get("attributes"))
	excludedAttributes := parseCSVQueryParam(r.URL.Query().Get("excludedAttributes"))
	if svcErr := h.svc.ValidateAttributePaths(ctx, attributes, excludedAttributes); svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, h.logger)
		return
	}
	scimUser, svcErr := h.svc.GetUser(ctx, userID, h.baseURL)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, h.logger)
		return
	}
	h.writeUserResponse(ctx, w, http.StatusOK, scimUser, attributes, excludedAttributes)
	h.logger.Debug(ctx, "SCIM User GET sent", log.MaskedString(log.LoggerKeyUserID, userID))
}

// HandleUsersReplaceRequest handles PUT /scim/v2/Users/{id}
func (h *scimUsersHandler) HandleUsersReplaceRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.PathValue("id")

	if userID == "" {
		scim.HandleSCIMError(w, r, &scim.ErrorUserNotFound, h.logger)
		return
	}
	if svcErr := scim.ValidateSCIMContentType(r); svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, h.logger)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, scim.MaxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		scim.HandleSCIMError(w, r, &scim.ErrorInvalidRequestBody, h.logger)
		return
	}
	payload, svcErr := parseAndValidateSCIMUserRequest(body, h.schemaURNPrefix)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, h.logger)
		return
	}
	attributes := parseCSVQueryParam(r.URL.Query().Get("attributes"))
	excludedAttributes := parseCSVQueryParam(r.URL.Query().Get("excludedAttributes"))
	if svcErr := h.svc.ValidateAttributePaths(ctx, attributes, excludedAttributes); svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, h.logger)
		return
	}

	replaced, svcErr := h.svc.ReplaceUser(ctx, userID, payload, h.baseURL, false)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, h.logger)
		return
	}
	h.writeUserResponse(ctx, w, http.StatusOK, replaced, attributes, excludedAttributes)
	h.logger.Debug(ctx, "SCIM User replaced", log.MaskedString(log.LoggerKeyUserID, userID))
}

// HandleMeGetRequest handles GET /scim/v2/Me (RFC 7644 §3.11).
func (h *scimUsersHandler) HandleMeGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := strings.TrimSpace(security.GetSubject(ctx))
	if userID == "" {
		scim.HandleSCIMError(w, r, &scim.ErrorUnauthenticated, h.logger)
		return
	}
	attributes := parseCSVQueryParam(r.URL.Query().Get("attributes"))
	excludedAttributes := parseCSVQueryParam(r.URL.Query().Get("excludedAttributes"))
	if svcErr := h.svc.ValidateAttributePaths(ctx, attributes, excludedAttributes); svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, h.logger)
		return
	}
	scimUser, svcErr := h.svc.GetUser(ctx, userID, h.baseURL)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, h.logger)
		return
	}
	w.Header().Set("Location", scimUser.Meta.Location)
	h.writeUserResponse(ctx, w, http.StatusOK, scimUser, attributes, excludedAttributes)
	h.logger.Debug(ctx, "SCIM Me GET sent", log.MaskedString(log.LoggerKeyUserID, userID))
}

// HandleMeReplaceRequest handles PUT /scim/v2/Me (RFC 7644 §3.11).
func (h *scimUsersHandler) HandleMeReplaceRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := strings.TrimSpace(security.GetSubject(ctx))
	if userID == "" {
		scim.HandleSCIMError(w, r, &scim.ErrorUnauthenticated, h.logger)
		return
	}
	if svcErr := scim.ValidateSCIMContentType(r); svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, h.logger)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, scim.MaxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		scim.HandleSCIMError(w, r, &scim.ErrorInvalidRequestBody, h.logger)
		return
	}
	payload, svcErr := parseAndValidateSCIMUserRequest(body, h.schemaURNPrefix)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, h.logger)
		return
	}
	attributes := parseCSVQueryParam(r.URL.Query().Get("attributes"))
	excludedAttributes := parseCSVQueryParam(r.URL.Query().Get("excludedAttributes"))
	if svcErr := h.svc.ValidateAttributePaths(ctx, attributes, excludedAttributes); svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, h.logger)
		return
	}
	replaced, svcErr := h.svc.ReplaceUser(ctx, userID, payload, h.baseURL, true)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, h.logger)
		return
	}
	w.Header().Set("Location", replaced.Meta.Location)
	h.writeUserResponse(ctx, w, http.StatusOK, replaced, attributes, excludedAttributes)
	h.logger.Debug(ctx, "SCIM Me replaced", log.MaskedString(log.LoggerKeyUserID, userID))
}

// HandleUsersDeleteRequest handles DELETE /scim/v2/Users/{id}
func (h *scimUsersHandler) HandleUsersDeleteRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.PathValue("id")
	if userID == "" {
		scim.HandleSCIMError(w, r, &scim.ErrorUserNotFound, h.logger)
		return
	}
	svcErr := h.svc.DeleteUser(ctx, userID)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, h.logger)
		return
	}
	scim.WriteSCIMSuccessResponse(ctx, w, http.StatusNoContent, nil, h.logger)
	h.logger.Debug(ctx, "SCIM User deleted", log.MaskedString(log.LoggerKeyUserID, userID))
}

// writeProjectedResponse writes projected if projection produced a result, otherwise writes original.
func (h *scimUsersHandler) writeProjectedResponse(
	ctx context.Context, w http.ResponseWriter, status int,
	original interface{}, projected map[string]interface{}, err error,
) {
	if err != nil {
		h.logger.Error(ctx, "SCIM attribute projection failed", log.Any("error", err))
		scim.WriteSCIMErrorResponse(ctx, w, http.StatusInternalServerError, scim.SCIMErrorResponse{
			Schemas: []string{scim.SCIMErrorSchemaURN},
			Status:  "500",
			Detail:  "failed to build response",
		}, h.logger)
		return
	}
	if projected != nil {
		scim.WriteSCIMSuccessResponse(ctx, w, status, projected, h.logger)
		return
	}
	scim.WriteSCIMSuccessResponse(ctx, w, status, original, h.logger)
}

// writeUserResponse writes a single SCIM User resource, applying attribute projection when requested.
func (h *scimUsersHandler) writeUserResponse(
	ctx context.Context, w http.ResponseWriter, status int, scimUser *SCIMUser,
	attributes, excludedAttributes []string,
) {
	projected, err := projectSCIMUserResource(*scimUser, attributes, excludedAttributes)
	h.writeProjectedResponse(ctx, w, status, scimUser, projected, err)
}

// writeUserListResponse writes a SCIM list response, applying attribute projection when requested.
func (h *scimUsersHandler) writeUserListResponse(
	ctx context.Context, w http.ResponseWriter, listResp SCIMUserListResponse,
	attributes, excludedAttributes []string,
) {
	projected, err := projectSCIMUserListResponse(listResp, attributes, excludedAttributes)
	h.writeProjectedResponse(ctx, w, http.StatusOK, listResp, projected, err)
}

// parseCSVQueryParam splits a comma-separated query value into trimmed, non-empty entries.
func parseCSVQueryParam(rawValue string) []string {
	if rawValue == "" {
		return nil
	}
	parts := strings.Split(rawValue, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}
