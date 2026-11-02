// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"context"
	"net/http"
	"strconv"

	"github.com/thunder-id/thunderid/internal/ou"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

const projectsPath = "/projects"

// projectHandler serves the project management API.
type projectHandler struct {
	service ProjectServiceInterface
}

// newProjectHandler creates a project handler.
func newProjectHandler(service ProjectServiceInterface) *projectHandler {
	return &projectHandler{service: service}
}

// HandleProjectPostRequest creates a project.
func (h *projectHandler) HandleProjectPostRequest(w http.ResponseWriter, r *http.Request) {
	request, err := sysutils.DecodeJSONBody[ProjectRequest](r)
	if err != nil {
		writeProjectError(r.Context(), w, &ErrorInvalidRequestFormat)
		return
	}

	created, svcErr := h.service.CreateProject(r.Context(), sanitizeProjectRequest(*request))
	if svcErr != nil {
		writeProjectError(r.Context(), w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(r.Context(), w, http.StatusCreated, created)
}

// HandleProjectListRequest lists projects.
func (h *projectHandler) HandleProjectListRequest(w http.ResponseWriter, r *http.Request) {
	limit, offset, svcErr := parsePaginationParams(r)
	if svcErr != nil {
		writeProjectError(r.Context(), w, svcErr)
		return
	}

	projects, svcErr := h.service.GetProjectList(r.Context(), limit, offset)
	if svcErr != nil {
		writeProjectError(r.Context(), w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(r.Context(), w, http.StatusOK, projects)
}

// HandleProjectGetRequest returns a project.
func (h *projectHandler) HandleProjectGetRequest(w http.ResponseWriter, r *http.Request) {
	project, svcErr := h.service.GetProject(r.Context(), r.PathValue("id"))
	if svcErr != nil {
		writeProjectError(r.Context(), w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(r.Context(), w, http.StatusOK, project)
}

// HandleProjectPutRequest updates a project.
func (h *projectHandler) HandleProjectPutRequest(w http.ResponseWriter, r *http.Request) {
	request, err := sysutils.DecodeJSONBody[ProjectRequest](r)
	if err != nil {
		writeProjectError(r.Context(), w, &ErrorInvalidRequestFormat)
		return
	}

	updated, svcErr := h.service.UpdateProject(r.Context(), r.PathValue("id"), sanitizeProjectRequest(*request))
	if svcErr != nil {
		writeProjectError(r.Context(), w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(r.Context(), w, http.StatusOK, updated)
}

// HandleProjectDeleteRequest deletes a project.
func (h *projectHandler) HandleProjectDeleteRequest(w http.ResponseWriter, r *http.Request) {
	if svcErr := h.service.DeleteProject(r.Context(), r.PathValue("id")); svcErr != nil {
		writeProjectError(r.Context(), w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(r.Context(), w, http.StatusNoContent, nil)
}

// sanitizeProjectRequest sanitizes the string fields of a project request.
func sanitizeProjectRequest(request ProjectRequest) ProjectRequest {
	return ProjectRequest{
		ID:          sysutils.SanitizeString(request.ID),
		Handle:      sysutils.SanitizeString(request.Handle),
		DisplayName: sysutils.SanitizeString(request.DisplayName),
		Description: sysutils.SanitizeString(request.Description),
	}
}

// parsePaginationParams reads the limit and offset query parameters, defaulting the limit.
func parsePaginationParams(r *http.Request) (int, int, *tidcommon.ServiceError) {
	query := r.URL.Query()
	limit := serverconst.DefaultPageSize
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
	return limit, offset, nil
}

// writeProjectError maps a service error to an HTTP status and writes the error response. Errors
// from the organization unit service, raised while managing the root organization unit, pass through.
func writeProjectError(ctx context.Context, w http.ResponseWriter, svcErr *tidcommon.ServiceError) {
	status := http.StatusInternalServerError
	if svcErr.Type == tidcommon.ClientErrorType {
		switch svcErr.Code {
		case ErrorProjectNotFound.Code:
			status = http.StatusNotFound
		case ErrorProjectIDConflict.Code,
			ou.ErrorOrganizationUnitHandleConflict.Code, ou.ErrorOrganizationUnitNameConflict.Code:
			status = http.StatusConflict
		case tidcommon.ErrorUnauthorized.Code:
			status = http.StatusForbidden
		default:
			status = http.StatusBadRequest
		}
	}
	sysutils.WriteErrorResponse(ctx, w, status, apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	})
}
