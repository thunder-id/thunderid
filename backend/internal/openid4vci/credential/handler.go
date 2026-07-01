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

package credential

import (
	"context"
	"net/http"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

const configurationsPath = "/openid4vci/credential-configurations"

// configurationHandler serves the management API for credential configurations.
type configurationHandler struct {
	service CredentialConfigurationServiceInterface
}

// newConfigurationHandler builds the credential-configuration management handler.
func newConfigurationHandler(service CredentialConfigurationServiceInterface) *configurationHandler {
	return &configurationHandler{service: service}
}

// HandleCreate creates a credential configuration.
func (h *configurationHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	req, err := sysutils.DecodeJSONBody[credentialConfigurationRequest](r)
	if err != nil {
		writeConfigurationError(r.Context(), w, &ErrorConfigurationInvalidRequest)
		return
	}
	created, svcErr := h.service.CreateCredentialConfiguration(r.Context(), requestToDTO(req))
	if svcErr != nil {
		writeConfigurationError(r.Context(), w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(r.Context(), w, http.StatusCreated, toResponse(*created))
}

// HandleList returns a minimal summary of all credential configurations.
func (h *configurationHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	summaries, svcErr := h.service.ListCredentialConfigurationSummaries(r.Context())
	if svcErr != nil {
		writeConfigurationError(r.Context(), w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(r.Context(), w, http.StatusOK, summaries)
}

// HandleGet returns a single credential configuration.
func (h *configurationHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeConfigurationError(r.Context(), w, &ErrorConfigurationInvalidRequest)
		return
	}
	dto, svcErr := h.service.GetCredentialConfiguration(r.Context(), id)
	if svcErr != nil {
		writeConfigurationError(r.Context(), w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(r.Context(), w, http.StatusOK, toResponse(*dto))
}

// HandleUpdate updates a credential configuration.
func (h *configurationHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeConfigurationError(r.Context(), w, &ErrorConfigurationInvalidRequest)
		return
	}
	req, err := sysutils.DecodeJSONBody[credentialConfigurationRequest](r)
	if err != nil {
		writeConfigurationError(r.Context(), w, &ErrorConfigurationInvalidRequest)
		return
	}
	updated, svcErr := h.service.UpdateCredentialConfiguration(r.Context(), id, requestToDTO(req))
	if svcErr != nil {
		writeConfigurationError(r.Context(), w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(r.Context(), w, http.StatusOK, toResponse(*updated))
}

// HandleDelete deletes a credential configuration.
func (h *configurationHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeConfigurationError(r.Context(), w, &ErrorConfigurationInvalidRequest)
		return
	}
	if svcErr := h.service.DeleteCredentialConfiguration(r.Context(), id); svcErr != nil {
		writeConfigurationError(r.Context(), w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(r.Context(), w, http.StatusNoContent, nil)
}

// requestToDTO maps and sanitizes an API request to a managed DTO.
func requestToDTO(req *credentialConfigurationRequest) *CredentialConfigurationDTO {
	return &CredentialConfigurationDTO{
		Handle:          sysutils.SanitizeString(req.Handle),
		OUID:            sysutils.SanitizeString(req.OUID),
		OUHandle:        sysutils.SanitizeString(req.OUHandle),
		Format:          sysutils.SanitizeString(req.Format),
		VCT:             sysutils.SanitizeString(req.VCT),
		Claims:          sanitizeClaims(req.Claims),
		Display:         sanitizeDisplay(req.Display),
		ValiditySeconds: req.ValiditySeconds,
	}
}

// sanitizeClaims trims claim names and display names, dropping entries with no name.
func sanitizeClaims(in []ClaimMapping) []ClaimMapping {
	out := make([]ClaimMapping, 0, len(in))
	for _, c := range in {
		name := sysutils.SanitizeString(c.Name)
		if name == "" {
			continue
		}
		out = append(out, ClaimMapping{Name: name, DisplayName: sysutils.SanitizeString(c.DisplayName)})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// sanitizeDisplay trims the display fields, returning nil when all are empty.
func sanitizeDisplay(in *CredentialDisplay) *CredentialDisplay {
	if in == nil {
		return nil
	}
	d := CredentialDisplay{
		Name:    sysutils.SanitizeString(in.Name),
		Locale:  sysutils.SanitizeString(in.Locale),
		LogoURI: sysutils.SanitizeString(in.LogoURI),
	}
	if d.Name == "" && d.Locale == "" && d.LogoURI == "" {
		return nil
	}
	return &d
}

// writeConfigurationError writes a service error to the response with the appropriate HTTP status code.
func writeConfigurationError(ctx context.Context, w http.ResponseWriter, svcErr *tidcommon.ServiceError) {
	status := http.StatusInternalServerError
	if svcErr.Type == tidcommon.ClientErrorType {
		status = configurationClientErrorStatus(svcErr.Code)
	}
	sysutils.WriteErrorResponse(ctx, w, status, apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	})
}
