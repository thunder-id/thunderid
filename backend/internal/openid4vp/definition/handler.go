/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package definition

import (
	"context"
	"net/http"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

const definitionsPath = "/openid4vp/presentation-definitions"

// definitionHandler serves the management API for presentation definitions.
type definitionHandler struct {
	service PresentationDefinitionServiceInterface
}

// newDefinitionHandler builds the presentation-definition management HTTP handler.
func newDefinitionHandler(service PresentationDefinitionServiceInterface) *definitionHandler {
	return &definitionHandler{service: service}
}

// HandleCreate creates a presentation definition.
func (h *definitionHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	req, err := sysutils.DecodeJSONBody[presentationDefinitionRequest](r)
	if err != nil {
		writeDefinitionError(r.Context(), w, &ErrorDefinitionInvalidRequest)
		return
	}
	created, svcErr := h.service.CreatePresentationDefinition(r.Context(), requestToDTO(req))
	if svcErr != nil {
		writeDefinitionError(r.Context(), w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(r.Context(), w, http.StatusCreated, toResponse(*created))
}

// HandleList returns a minimal summary of all presentation definitions.
func (h *definitionHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	summaries, svcErr := h.service.ListPresentationDefinitionSummaries(r.Context())
	if svcErr != nil {
		writeDefinitionError(r.Context(), w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(r.Context(), w, http.StatusOK, summaries)
}

// HandleGet returns a single presentation definition.
func (h *definitionHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeDefinitionError(r.Context(), w, &ErrorDefinitionInvalidRequest)
		return
	}
	dto, svcErr := h.service.GetPresentationDefinition(r.Context(), id)
	if svcErr != nil {
		writeDefinitionError(r.Context(), w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(r.Context(), w, http.StatusOK, toResponse(*dto))
}

// HandleUpdate updates a presentation definition.
func (h *definitionHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeDefinitionError(r.Context(), w, &ErrorDefinitionInvalidRequest)
		return
	}
	req, err := sysutils.DecodeJSONBody[presentationDefinitionRequest](r)
	if err != nil {
		writeDefinitionError(r.Context(), w, &ErrorDefinitionInvalidRequest)
		return
	}
	updated, svcErr := h.service.UpdatePresentationDefinition(r.Context(), id, requestToDTO(req))
	if svcErr != nil {
		writeDefinitionError(r.Context(), w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(r.Context(), w, http.StatusOK, toResponse(*updated))
}

// HandleDelete deletes a presentation definition.
func (h *definitionHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeDefinitionError(r.Context(), w, &ErrorDefinitionInvalidRequest)
		return
	}
	if svcErr := h.service.DeletePresentationDefinition(r.Context(), id); svcErr != nil {
		writeDefinitionError(r.Context(), w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(r.Context(), w, http.StatusNoContent, nil)
}

// requestToDTO maps and sanitizes an API request to a managed DTO.
func requestToDTO(req *presentationDefinitionRequest) *PresentationDefinitionDTO {
	return &PresentationDefinitionDTO{
		Handle:               sysutils.SanitizeString(req.Handle),
		OUID:                 sysutils.SanitizeString(req.OUID),
		OUHandle:             sysutils.SanitizeString(req.OUHandle),
		DisplayName:          sysutils.SanitizeString(req.DisplayName),
		VCT:                  sysutils.SanitizeString(req.VCT),
		Format:               sysutils.SanitizeString(req.Format),
		RequestedClaims:      sanitizeStrings(req.RequestedClaims),
		MandatoryClaims:      sanitizeStrings(req.MandatoryClaims),
		OptionalClaims:       sanitizeStrings(req.OptionalClaims),
		ClaimValues:          sanitizeClaimValues(req.ClaimValues),
		EnforceTrustedIssuer: req.EnforceTrustedIssuer,
		TrustedAuthorities:   sanitizeStrings(req.TrustedAuthorities),
	}
}

// sanitizeStrings returns a new slice with each input string sanitized.
func sanitizeStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, sysutils.SanitizeString(s))
	}
	return out
}

// sanitizeClaimValues sanitizes claim path keys and their allowed values,
// dropping empty values and any entry left with no path or no values.
func sanitizeClaimValues(in map[string][]string) map[string][]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string][]string, len(in))
	for path, values := range in {
		cleanPath := sysutils.SanitizeString(path)
		if cleanPath == "" {
			continue
		}
		cleanValues := make([]string, 0, len(values))
		for _, v := range values {
			if s := sysutils.SanitizeString(v); s != "" {
				cleanValues = append(cleanValues, s)
			}
		}
		if len(cleanValues) > 0 {
			out[cleanPath] = cleanValues
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// writeDefinitionError maps a service error to an HTTP status and writes the corresponding error response.
func writeDefinitionError(ctx context.Context, w http.ResponseWriter, svcErr *tidcommon.ServiceError) {
	status := http.StatusInternalServerError
	if svcErr.Type == tidcommon.ClientErrorType {
		status = definitionClientErrorStatus(svcErr.Code)
	}
	sysutils.WriteErrorResponse(ctx, w, status, apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	})
}
