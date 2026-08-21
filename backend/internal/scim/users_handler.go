// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package scim

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	scimconfig "github.com/thunder-id/thunderid/internal/scim/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

const usersHandlerLoggerComponentName = "SCIMUsersHandler"

// scimUsersHandler handles all /scim/v2/Users HTTP requests.
type scimUsersHandler struct {
	svc     SCIMUsersServiceInterface
	baseURL string
}

// newSCIMUsersHandler creates a new scimUsersHandler.
func newSCIMUsersHandler(svc SCIMUsersServiceInterface, baseURL string) *scimUsersHandler {
	return &scimUsersHandler{svc: svc, baseURL: baseURL}
}

// HandleUsersListRequest handles GET /scim/v2/Users
func (h *scimUsersHandler) HandleUsersListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, usersHandlerLoggerComponentName))

	if !scimconfig.SortSupported && (r.URL.Query().Get("sortBy") != "" || r.URL.Query().Get("sortOrder") != "") {
		handleSCIMError(w, r, &ErrorSortNotSupported)
		return
	}

	// Parse optional SCIM filter — "eq" expressions joined by "and" are supported.
	var parsedFilters map[string]interface{}
	if filterStr := r.URL.Query().Get("filter"); filterStr != "" {
		var svcErr *tidcommon.ServiceError
		parsedFilters, svcErr = parseSCIMFilterForEq(filterStr)
		if svcErr != nil {
			handleSCIMError(w, r, svcErr)
			return
		}
	}
	startIndex, count := parseSCIMPaginationQueryParams(r)
	attributes := parseCSVQueryParam(r.URL.Query().Get("attributes"))
	excludedAttributes := parseCSVQueryParam(r.URL.Query().Get("excludedAttributes"))
	if svcErr := validateAttributesParams(attributes, excludedAttributes); svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}

	listResp, svcErr := h.svc.ListUsers(ctx, startIndex, count, parsedFilters, h.baseURL)
	if svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}

	h.writeUserListResponse(ctx, w, listResp, attributes, excludedAttributes)
	logger.Debug(ctx, "SCIM Users list sent", log.Int("totalResults", listResp.TotalResults))
}

// HandleUsersSearchRequest handles POST /scim/v2/Users/.search (RFC 7644 §3.4.3).
func (h *scimUsersHandler) HandleUsersSearchRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, usersHandlerLoggerComponentName))

	if svcErr := validateSCIMContentType(r); svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		handleSCIMError(w, r, &ErrorInvalidRequestBody)
		return
	}
	var searchReq SCIMSearchRequest
	if err := json.Unmarshal(body, &searchReq); err != nil {
		handleSCIMError(w, r, &ErrorInvalidRequestBody)
		return
	}
	if !scimconfig.SortSupported && (searchReq.SortBy != "" || searchReq.SortOrder != "") {
		handleSCIMError(w, r, &ErrorSortNotSupported)
		return
	}
	if !hasSchemaURN(searchReq.Schemas, SCIMSearchSchemaURN) {
		svcErr := ErrorMissingSchemas
		svcErr.ErrorDescription = tidcommon.I18nMessage{
			Key:          ErrorMissingSchemas.ErrorDescription.Key,
			DefaultValue: fmt.Sprintf("The schemas array must include %q", SCIMSearchSchemaURN),
		}
		handleSCIMError(w, r, &svcErr)
		return
	}
	if svcErr := validateAttributesParams(searchReq.Attributes, searchReq.ExcludedAttributes); svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}

	var parsedFilters map[string]interface{}
	if searchReq.Filter != "" {
		var svcErr *tidcommon.ServiceError
		parsedFilters, svcErr = parseSCIMFilterForEq(searchReq.Filter)
		if svcErr != nil {
			handleSCIMError(w, r, svcErr)
			return
		}
	}

	startIndex, count := normalizeSCIMPagination(searchReq.StartIndex, searchReq.Count)

	listResp, svcErr := h.svc.ListUsers(ctx, startIndex, count, parsedFilters, h.baseURL)
	if svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}

	h.writeUserListResponse(ctx, w, listResp, searchReq.Attributes, searchReq.ExcludedAttributes)
	logger.Debug(ctx, "SCIM Users search sent", log.Int("totalResults", listResp.TotalResults))
}

// HandleUsersCreateRequest handles POST /scim/v2/Users
func (h *scimUsersHandler) HandleUsersCreateRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, usersHandlerLoggerComponentName))

	if svcErr := validateSCIMContentType(r); svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		handleSCIMError(w, r, &ErrorInvalidRequestBody)
		return
	}
	payload, svcErr := parseAndValidateSCIMUserRequest(body)
	if svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}
	attributes := parseCSVQueryParam(r.URL.Query().Get("attributes"))
	excludedAttributes := parseCSVQueryParam(r.URL.Query().Get("excludedAttributes"))
	if svcErr := validateAttributesParams(attributes, excludedAttributes); svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}

	created, svcErr := h.svc.CreateUser(ctx, payload, h.baseURL)
	if svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}

	w.Header().Set("Location", created.Meta.Location)
	w.Header().Set("ETag", created.Meta.Version)
	h.writeUserResponse(ctx, w, http.StatusCreated, created, attributes, excludedAttributes)
	logger.Debug(ctx, "SCIM User created", log.String("userID", created.ID))
}

// HandleUsersGetRequest handles GET /scim/v2/Users/{id}
func (h *scimUsersHandler) HandleUsersGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, usersHandlerLoggerComponentName))

	userID := r.PathValue("id")
	if userID == "" {
		handleSCIMError(w, r, &ErrorUserNotFound)
		return
	}
	attributes := parseCSVQueryParam(r.URL.Query().Get("attributes"))
	excludedAttributes := parseCSVQueryParam(r.URL.Query().Get("excludedAttributes"))
	if svcErr := validateAttributesParams(attributes, excludedAttributes); svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}
	scimUser, svcErr := h.svc.GetUser(ctx, userID, h.baseURL)
	if svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}
	w.Header().Set("ETag", scimUser.Meta.Version)
	h.writeUserResponse(ctx, w, http.StatusOK, scimUser, attributes, excludedAttributes)
	logger.Debug(ctx, "SCIM User GET sent", log.String("userID", userID))
}

// HandleUsersReplaceRequest handles PUT /scim/v2/Users/{id}
func (h *scimUsersHandler) HandleUsersReplaceRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, usersHandlerLoggerComponentName))

	userID := r.PathValue("id")

	if userID == "" {
		handleSCIMError(w, r, &ErrorUserNotFound)
		return
	}
	if svcErr := validateSCIMContentType(r); svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		handleSCIMError(w, r, &ErrorInvalidRequestBody)
		return
	}
	payload, svcErr := parseAndValidateSCIMUserRequest(body)
	if svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}
	attributes := parseCSVQueryParam(r.URL.Query().Get("attributes"))
	excludedAttributes := parseCSVQueryParam(r.URL.Query().Get("excludedAttributes"))
	if svcErr := validateAttributesParams(attributes, excludedAttributes); svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}

	replaced, svcErr := h.svc.ReplaceUser(ctx, userID, payload, r.Header.Get("If-Match"), h.baseURL, false)
	if svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}
	w.Header().Set("ETag", replaced.Meta.Version)
	h.writeUserResponse(ctx, w, http.StatusOK, replaced, attributes, excludedAttributes)
	logger.Debug(ctx, "SCIM User replaced", log.String("userID", userID))
}

// HandleMeGetRequest handles GET /scim/v2/Me (RFC 7644 §3.11).
// Resolves the authenticated subject and processes the request directly
// against the aliased User resource, per RFC 7644 §3.11 option 3.
func (h *scimUsersHandler) HandleMeGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, usersHandlerLoggerComponentName))

	userID := strings.TrimSpace(security.GetSubject(ctx))
	if userID == "" {
		handleSCIMError(w, r, &ErrorUnauthenticated)
		return
	}
	attributes := parseCSVQueryParam(r.URL.Query().Get("attributes"))
	excludedAttributes := parseCSVQueryParam(r.URL.Query().Get("excludedAttributes"))
	if svcErr := validateAttributesParams(attributes, excludedAttributes); svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}
	scimUser, svcErr := h.svc.GetUser(ctx, userID, h.baseURL)
	if svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}
	w.Header().Set("Location", scimUser.Meta.Location)
	w.Header().Set("ETag", scimUser.Meta.Version)
	h.writeUserResponse(ctx, w, http.StatusOK, scimUser, attributes, excludedAttributes)
	logger.Debug(ctx, "SCIM Me GET sent", log.String("userID", userID))
}

// HandleMeReplaceRequest handles PUT /scim/v2/Me (RFC 7644 §3.11).
func (h *scimUsersHandler) HandleMeReplaceRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, usersHandlerLoggerComponentName))

	userID := strings.TrimSpace(security.GetSubject(ctx))
	if userID == "" {
		handleSCIMError(w, r, &ErrorUnauthenticated)
		return
	}
	if svcErr := validateSCIMContentType(r); svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		handleSCIMError(w, r, &ErrorInvalidRequestBody)
		return
	}
	payload, svcErr := parseAndValidateSCIMUserRequest(body)
	if svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}
	attributes := parseCSVQueryParam(r.URL.Query().Get("attributes"))
	excludedAttributes := parseCSVQueryParam(r.URL.Query().Get("excludedAttributes"))
	if svcErr := validateAttributesParams(attributes, excludedAttributes); svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}
	replaced, svcErr := h.svc.ReplaceUser(ctx, userID, payload, r.Header.Get("If-Match"), h.baseURL, true)
	if svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}
	w.Header().Set("Location", replaced.Meta.Location)
	w.Header().Set("ETag", replaced.Meta.Version)
	h.writeUserResponse(ctx, w, http.StatusOK, replaced, attributes, excludedAttributes)
	logger.Debug(ctx, "SCIM Me replaced", log.String("userID", userID))
}

// HandleUsersDeleteRequest handles DELETE /scim/v2/Users/{id}
func (h *scimUsersHandler) HandleUsersDeleteRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, usersHandlerLoggerComponentName))

	userID := r.PathValue("id")
	if userID == "" {
		handleSCIMError(w, r, &ErrorUserNotFound)
		return
	}
	svcErr := h.svc.DeleteUser(ctx, userID, r.Header.Get("If-Match"))
	if svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}
	writeSCIMSuccessResponse(ctx, w, http.StatusNoContent, nil)
	logger.Debug(ctx, "SCIM User deleted", log.String("userID", userID))
}

// validateAttributesParams enforces RFC 7644 §3.9: "attributes" and
// "excludedAttributes" are mutually exclusive.
func validateAttributesParams(attributes, excludedAttributes []string) *tidcommon.ServiceError {
	if len(attributes) > 0 && len(excludedAttributes) > 0 {
		return &ErrorConflictingAttributesParams
	}
	return nil
}

// writeProjectedResponse writes original, or projected in its place when
// attribute projection (RFC 7644 §3.9) actually pruned something. Shared by
// writeUserResponse and writeUserListResponse so the projection-error/write
// logic isn't duplicated for the single-resource and list-response cases.
func (h *scimUsersHandler) writeProjectedResponse(
	ctx context.Context, w http.ResponseWriter, status int,
	original interface{}, projected map[string]interface{}, err error,
) {
	if err != nil {
		log.GetLogger().With(log.String(log.LoggerKeyComponentName, usersHandlerLoggerComponentName)).
			Error(ctx, "SCIM attribute projection failed", log.Any("error", err))
		writeSCIMErrorResponse(ctx, w, http.StatusInternalServerError, SCIMErrorResponse{
			Schemas: []string{SCIMErrorSchemaURN},
			Status:  "500",
			Detail:  "failed to build response",
		})
		return
	}
	if projected != nil {
		writeSCIMSuccessResponse(ctx, w, status, projected)
		return
	}
	writeSCIMSuccessResponse(ctx, w, status, original)
}

// writeUserResponse writes a single SCIM User resource, applying attribute
// projection (RFC 7644 §3.9) when attributes/excludedAttributes were requested.
func (h *scimUsersHandler) writeUserResponse(
	ctx context.Context, w http.ResponseWriter, status int, scimUser *SCIMUser,
	attributes, excludedAttributes []string,
) {
	projected, err := projectSCIMUserResource(*scimUser, attributes, excludedAttributes)
	h.writeProjectedResponse(ctx, w, status, scimUser, projected, err)
}

// writeUserListResponse writes listResp, applying attribute projection
// (RFC 7644 §3.9) when attributes/excludedAttributes were requested.
func (h *scimUsersHandler) writeUserListResponse(
	ctx context.Context, w http.ResponseWriter, listResp SCIMUserListResponse,
	attributes, excludedAttributes []string,
) {
	projected, err := projectSCIMUserListResponse(listResp, attributes, excludedAttributes)
	h.writeProjectedResponse(ctx, w, http.StatusOK, listResp, projected, err)
}

// parseCSVQueryParam splits a comma-separated query value into trimmed,
// non-empty entries.
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
