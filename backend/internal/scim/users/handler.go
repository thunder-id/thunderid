// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package users implements the SCIM Users and Me endpoints per RFC 7643/7644.
package users

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/internal/entitytype"
	scim "github.com/thunder-id/thunderid/internal/scim/common"
	scimconfig "github.com/thunder-id/thunderid/internal/scim/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/user"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

const usersHandlerLoggerComponentName = "SCIMUsersHandler"

// Handler handles all /scim/v2/Users HTTP requests.
type Handler struct {
	svc     SCIMUsersServiceInterface
	baseURL string
	logger  log.Logger
}

// NewHandler builds the users service internally and returns its handler,
// the sole exported entry point the root scim package's composition root needs.
func NewHandler(
	userService user.UserServiceInterface,
	userTypeService entitytype.EntityTypeServiceInterface,
	cfg scimconfig.SCIMConfig,
) *Handler {
	svc := newSCIMUsersService(userService, userTypeService, cfg)
	return &Handler{
		svc:     svc,
		baseURL: cfg.PublicURL,
		logger:  *log.GetLogger().With(log.String(log.LoggerKeyComponentName, usersHandlerLoggerComponentName)),
	}
}

// HandleUsersListRequest handles GET /scim/v2/Users
func (h *Handler) HandleUsersListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if !scimconfig.SortSupported && (r.URL.Query().Get("sortBy") != "" || r.URL.Query().Get("sortOrder") != "") {
		scim.HandleSCIMError(w, r, &scim.ErrorSortNotSupported, usersHandlerLoggerComponentName)
		return
	}

	// Parse optional SCIM filter — "eq" expressions joined by "and" are supported.
	var parsedFilters map[string]interface{}
	if filterStr := r.URL.Query().Get("filter"); filterStr != "" {
		var svcErr *tidcommon.ServiceError
		parsedFilters, svcErr = scim.ParseSCIMFilterForEq(filterStr, usersFilterAttrRules)
		if svcErr != nil {
			scim.HandleSCIMError(w, r, svcErr, usersHandlerLoggerComponentName)
			return
		}
	}
	startIndex, count := scim.ParseSCIMPaginationQueryParams(r)
	attributes := parseCSVQueryParam(r.URL.Query().Get("attributes"))
	excludedAttributes := parseCSVQueryParam(r.URL.Query().Get("excludedAttributes"))
	if svcErr := validateAttributesParams(attributes, excludedAttributes); svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, usersHandlerLoggerComponentName)
		return
	}

	listResp, svcErr := h.svc.ListUsers(ctx, startIndex, count, parsedFilters, h.baseURL)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, usersHandlerLoggerComponentName)
		return
	}

	h.writeUserListResponse(ctx, w, listResp, attributes, excludedAttributes)
	h.logger.Debug(ctx, "SCIM Users list sent", log.Int("totalResults", listResp.TotalResults))
}

// HandleUsersSearchRequest handles POST /scim/v2/Users/.search (RFC 7644 §3.4.3).
func (h *Handler) HandleUsersSearchRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if svcErr := scim.ValidateSCIMContentType(r); svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, usersHandlerLoggerComponentName)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, scim.MaxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		scim.HandleSCIMError(w, r, &scim.ErrorInvalidRequestBody, usersHandlerLoggerComponentName)
		return
	}
	var searchReq scim.SCIMSearchRequest
	if err := json.Unmarshal(body, &searchReq); err != nil {
		scim.HandleSCIMError(w, r, &scim.ErrorInvalidRequestBody, usersHandlerLoggerComponentName)
		return
	}
	if !scimconfig.SortSupported && (searchReq.SortBy != "" || searchReq.SortOrder != "") {
		scim.HandleSCIMError(w, r, &scim.ErrorSortNotSupported, usersHandlerLoggerComponentName)
		return
	}
	if !scim.HasSchemaURN(searchReq.Schemas, scim.SCIMSearchSchemaURN) {
		svcErr := scim.ErrorMissingSchemas
		svcErr.ErrorDescription = tidcommon.I18nMessage{
			Key:          scim.ErrorMissingSchemas.ErrorDescription.Key,
			DefaultValue: fmt.Sprintf("The schemas array must include %q", scim.SCIMSearchSchemaURN),
		}
		scim.HandleSCIMError(w, r, &svcErr, usersHandlerLoggerComponentName)
		return
	}
	if svcErr := validateAttributesParams(searchReq.Attributes, searchReq.ExcludedAttributes); svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, usersHandlerLoggerComponentName)
		return
	}

	var parsedFilters map[string]interface{}
	if searchReq.Filter != "" {
		var svcErr *tidcommon.ServiceError
		parsedFilters, svcErr = scim.ParseSCIMFilterForEq(searchReq.Filter, usersFilterAttrRules)
		if svcErr != nil {
			scim.HandleSCIMError(w, r, svcErr, usersHandlerLoggerComponentName)
			return
		}
	}

	startIndex, count := scim.NormalizeSCIMPagination(searchReq.StartIndex, searchReq.Count)

	listResp, svcErr := h.svc.ListUsers(ctx, startIndex, count, parsedFilters, h.baseURL)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, usersHandlerLoggerComponentName)
		return
	}

	h.writeUserListResponse(ctx, w, listResp, searchReq.Attributes, searchReq.ExcludedAttributes)
	h.logger.Debug(ctx, "SCIM Users search sent", log.Int("totalResults", listResp.TotalResults))
}

// HandleUsersCreateRequest handles POST /scim/v2/Users
func (h *Handler) HandleUsersCreateRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if svcErr := scim.ValidateSCIMContentType(r); svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, usersHandlerLoggerComponentName)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, scim.MaxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		scim.HandleSCIMError(w, r, &scim.ErrorInvalidRequestBody, usersHandlerLoggerComponentName)
		return
	}
	payload, svcErr := parseAndValidateSCIMUserRequest(body)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, usersHandlerLoggerComponentName)
		return
	}
	attributes := parseCSVQueryParam(r.URL.Query().Get("attributes"))
	excludedAttributes := parseCSVQueryParam(r.URL.Query().Get("excludedAttributes"))
	if svcErr := validateAttributesParams(attributes, excludedAttributes); svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, usersHandlerLoggerComponentName)
		return
	}

	created, svcErr := h.svc.CreateUser(ctx, payload, h.baseURL)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, usersHandlerLoggerComponentName)
		return
	}

	w.Header().Set("Location", created.Meta.Location)
	w.Header().Set("ETag", created.Meta.Version)
	h.writeUserResponse(ctx, w, http.StatusCreated, created, attributes, excludedAttributes)
	h.logger.Debug(ctx, "SCIM User created", log.String("userID", created.ID))
}

// HandleUsersGetRequest handles GET /scim/v2/Users/{id}
func (h *Handler) HandleUsersGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.PathValue("id")
	if userID == "" {
		scim.HandleSCIMError(w, r, &scim.ErrorUserNotFound, usersHandlerLoggerComponentName)
		return
	}
	attributes := parseCSVQueryParam(r.URL.Query().Get("attributes"))
	excludedAttributes := parseCSVQueryParam(r.URL.Query().Get("excludedAttributes"))
	if svcErr := validateAttributesParams(attributes, excludedAttributes); svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, usersHandlerLoggerComponentName)
		return
	}
	scimUser, svcErr := h.svc.GetUser(ctx, userID, h.baseURL)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, usersHandlerLoggerComponentName)
		return
	}
	w.Header().Set("ETag", scimUser.Meta.Version)
	h.writeUserResponse(ctx, w, http.StatusOK, scimUser, attributes, excludedAttributes)
	h.logger.Debug(ctx, "SCIM User GET sent", log.String("userID", userID))
}

// HandleUsersReplaceRequest handles PUT /scim/v2/Users/{id}
func (h *Handler) HandleUsersReplaceRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.PathValue("id")

	if userID == "" {
		scim.HandleSCIMError(w, r, &scim.ErrorUserNotFound, usersHandlerLoggerComponentName)
		return
	}
	if svcErr := scim.ValidateSCIMContentType(r); svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, usersHandlerLoggerComponentName)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, scim.MaxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		scim.HandleSCIMError(w, r, &scim.ErrorInvalidRequestBody, usersHandlerLoggerComponentName)
		return
	}
	payload, svcErr := parseAndValidateSCIMUserRequest(body)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, usersHandlerLoggerComponentName)
		return
	}
	attributes := parseCSVQueryParam(r.URL.Query().Get("attributes"))
	excludedAttributes := parseCSVQueryParam(r.URL.Query().Get("excludedAttributes"))
	if svcErr := validateAttributesParams(attributes, excludedAttributes); svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, usersHandlerLoggerComponentName)
		return
	}

	replaced, svcErr := h.svc.ReplaceUser(ctx, userID, payload, r.Header.Get("If-Match"), h.baseURL, false)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, usersHandlerLoggerComponentName)
		return
	}
	w.Header().Set("ETag", replaced.Meta.Version)
	h.writeUserResponse(ctx, w, http.StatusOK, replaced, attributes, excludedAttributes)
	h.logger.Debug(ctx, "SCIM User replaced", log.String("userID", userID))
}

// HandleMeGetRequest handles GET /scim/v2/Me (RFC 7644 §3.11).
// Resolves the authenticated subject and processes the request directly
// against the aliased User resource, per RFC 7644 §3.11 option 3.
func (h *Handler) HandleMeGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := strings.TrimSpace(security.GetSubject(ctx))
	if userID == "" {
		scim.HandleSCIMError(w, r, &scim.ErrorUnauthenticated, usersHandlerLoggerComponentName)
		return
	}
	attributes := parseCSVQueryParam(r.URL.Query().Get("attributes"))
	excludedAttributes := parseCSVQueryParam(r.URL.Query().Get("excludedAttributes"))
	if svcErr := validateAttributesParams(attributes, excludedAttributes); svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, usersHandlerLoggerComponentName)
		return
	}
	scimUser, svcErr := h.svc.GetUser(ctx, userID, h.baseURL)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, usersHandlerLoggerComponentName)
		return
	}
	w.Header().Set("Location", scimUser.Meta.Location)
	w.Header().Set("ETag", scimUser.Meta.Version)
	h.writeUserResponse(ctx, w, http.StatusOK, scimUser, attributes, excludedAttributes)
	h.logger.Debug(ctx, "SCIM Me GET sent", log.String("userID", userID))
}

// HandleMeReplaceRequest handles PUT /scim/v2/Me (RFC 7644 §3.11).
func (h *Handler) HandleMeReplaceRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := strings.TrimSpace(security.GetSubject(ctx))
	if userID == "" {
		scim.HandleSCIMError(w, r, &scim.ErrorUnauthenticated, usersHandlerLoggerComponentName)
		return
	}
	if svcErr := scim.ValidateSCIMContentType(r); svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, usersHandlerLoggerComponentName)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, scim.MaxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		scim.HandleSCIMError(w, r, &scim.ErrorInvalidRequestBody, usersHandlerLoggerComponentName)
		return
	}
	payload, svcErr := parseAndValidateSCIMUserRequest(body)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, usersHandlerLoggerComponentName)
		return
	}
	attributes := parseCSVQueryParam(r.URL.Query().Get("attributes"))
	excludedAttributes := parseCSVQueryParam(r.URL.Query().Get("excludedAttributes"))
	if svcErr := validateAttributesParams(attributes, excludedAttributes); svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, usersHandlerLoggerComponentName)
		return
	}
	replaced, svcErr := h.svc.ReplaceUser(ctx, userID, payload, r.Header.Get("If-Match"), h.baseURL, true)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, usersHandlerLoggerComponentName)
		return
	}
	w.Header().Set("Location", replaced.Meta.Location)
	w.Header().Set("ETag", replaced.Meta.Version)
	h.writeUserResponse(ctx, w, http.StatusOK, replaced, attributes, excludedAttributes)
	h.logger.Debug(ctx, "SCIM Me replaced", log.String("userID", userID))
}

// HandleUsersDeleteRequest handles DELETE /scim/v2/Users/{id}
func (h *Handler) HandleUsersDeleteRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.PathValue("id")
	if userID == "" {
		scim.HandleSCIMError(w, r, &scim.ErrorUserNotFound, usersHandlerLoggerComponentName)
		return
	}
	svcErr := h.svc.DeleteUser(ctx, userID, r.Header.Get("If-Match"))
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, usersHandlerLoggerComponentName)
		return
	}
	scim.WriteSCIMSuccessResponse(ctx, w, http.StatusNoContent, nil, usersHandlerLoggerComponentName)
	h.logger.Debug(ctx, "SCIM User deleted", log.String("userID", userID))
}

// validateAttributesParams enforces RFC 7644 §3.9: "attributes" and
// "excludedAttributes" are mutually exclusive, and per RFC 7643 §3.10 any path
// naming a custom/extension attribute must be qualified with its schema URN prefix.
func validateAttributesParams(attributes, excludedAttributes []string) *tidcommon.ServiceError {
	if len(attributes) > 0 && len(excludedAttributes) > 0 {
		return &scim.ErrorConflictingAttributesParams
	}
	// At most one of the two is non-empty at this point, so a single pass covers both.
	for _, attr := range append(attributes, excludedAttributes...) {
		if svcErr := validateAttributePathRequiresURN(attr); svcErr != nil {
			return svcErr
		}
	}
	return nil
}

// validateAttributePathRequiresURN rejects a bare (unqualified) attribute path that
// does not resolve to a recognized core User schema attribute, since such a path can
// only refer to a custom/extension attribute and RFC 7643 §3.10 requires those to be
// qualified with their schema URN. A qualified path must carry a URN this server
// actually recognizes, the core User schema URN or a well-formed ThunderID extension
// schema URN, rather than any arbitrary colon-containing string.
func validateAttributePathRequiresURN(attr string) *tidcommon.ServiceError {
	if idx := strings.LastIndex(attr, ":"); idx >= 0 {
		urn := attr[:idx]
		if strings.EqualFold(urn, scim.SCIMCoreUserSchemaURN) {
			return nil
		}
		if _, ok := scim.ParseUserTypeFromSchemaURN(urn); ok {
			return nil
		}
		return scim.NewUnrecognizedSchemaURNError(attr)
	}
	if isCoreSCIMAttrPath(attr) {
		return nil
	}
	return scim.NewCustomAttributeRequiresURNError(attr)
}

// writeProjectedResponse writes original, or projected in its place when
// attribute projection (RFC 7644 §3.9) actually pruned something. Shared by
// writeUserResponse and writeUserListResponse so the projection-error/write
// logic isn't duplicated for the single-resource and list-response cases.
func (h *Handler) writeProjectedResponse(
	ctx context.Context, w http.ResponseWriter, status int,
	original interface{}, projected map[string]interface{}, err error,
) {
	if err != nil {
		h.logger.Error(ctx, "SCIM attribute projection failed", log.Any("error", err))
		scim.WriteSCIMErrorResponse(ctx, w, http.StatusInternalServerError, scim.SCIMErrorResponse{
			Schemas: []string{scim.SCIMErrorSchemaURN},
			Status:  "500",
			Detail:  "failed to build response",
		}, usersHandlerLoggerComponentName)
		return
	}
	if projected != nil {
		scim.WriteSCIMSuccessResponse(ctx, w, status, projected, usersHandlerLoggerComponentName)
		return
	}
	scim.WriteSCIMSuccessResponse(ctx, w, status, original, usersHandlerLoggerComponentName)
}

// writeUserResponse writes a single SCIM User resource, applying attribute
// projection (RFC 7644 §3.9) when attributes/excludedAttributes were requested.
func (h *Handler) writeUserResponse(
	ctx context.Context, w http.ResponseWriter, status int, scimUser *SCIMUser,
	attributes, excludedAttributes []string,
) {
	projected, err := projectSCIMUserResource(*scimUser, attributes, excludedAttributes)
	h.writeProjectedResponse(ctx, w, status, scimUser, projected, err)
}

// writeUserListResponse writes listResp, applying attribute projection
// (RFC 7644 §3.9) when attributes/excludedAttributes were requested.
func (h *Handler) writeUserListResponse(
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
