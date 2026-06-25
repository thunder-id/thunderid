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

package connection

import (
	"context"
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// maskedSecretValue is the placeholder used for secret property values on read. A value of
// this on update means "keep the stored secret".
const maskedSecretValue = "******"

// appendProperty appends a property built from the given field, skipping empty values
// (the IdP service rejects properties with empty values). Secret values are encrypted by
// cmodels.NewProperty.
func appendProperty(props []cmodels.Property, name, value string, isSecret bool) ([]cmodels.Property, error) {
	if strings.TrimSpace(value) == "" {
		return props, nil
	}
	property, err := cmodels.NewProperty(name, value, isSecret)
	if err != nil {
		return nil, err
	}
	return append(props, *property), nil
}

// propertyValues returns a name→value map for the given properties. Secret values are
// replaced with the mask placeholder; non-secret values are returned in plain text.
func propertyValues(props []cmodels.Property) (map[string]string, error) {
	values := make(map[string]string, len(props))
	for i := range props {
		property := props[i]
		if property.IsSecret() {
			values[property.GetName()] = maskedSecretValue
			continue
		}
		value, err := property.GetValue()
		if err != nil {
			return nil, err
		}
		values[property.GetName()] = value
	}
	return values, nil
}

// mergeStoredSecrets keeps secret values that the request omits: secret properties are
// optional on update, so any secret present in the stored connection but absent from the
// incoming request is carried over unchanged. A secret that IS present in the request is
// used verbatim (presence-based — the value is not inspected).
func mergeStoredSecrets(incoming, existing []cmodels.Property) []cmodels.Property {
	incomingNames := make(map[string]bool, len(incoming))
	for i := range incoming {
		incomingNames[incoming[i].GetName()] = true
	}

	merged := make([]cmodels.Property, 0, len(incoming)+len(existing))
	merged = append(merged, incoming...)
	for i := range existing {
		if existing[i].IsSecret() && !incomingNames[existing[i].GetName()] {
			merged = append(merged, existing[i])
		}
	}
	return merged
}

// connectionTypeName returns the lowercase connection-type identifier (e.g. "google") that
// matches the /connections/{type} path, derived from the underlying IdP type.
func connectionTypeName(idpType providers.IDPType) string {
	return strings.ToLower(string(idpType))
}

// joinScopes serializes scopes to the comma-separated form stored in the IdP property.
func joinScopes(scopes []string) string {
	return strings.Join(scopes, ",")
}

// splitScopes parses the comma-separated scopes property into a slice, trimming whitespace
// and dropping empty entries (guards against externally-seeded values like "openid, email").
func splitScopes(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	scopes := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			scopes = append(scopes, trimmed)
		}
	}
	return scopes
}

// writeServiceError maps a service error from the underlying service to an HTTP response.
func writeServiceError(ctx context.Context, w http.ResponseWriter, svcErr *tidcommon.ServiceError) {
	status := http.StatusInternalServerError
	if svcErr.Type == tidcommon.ClientErrorType {
		switch svcErr.Code {
		case idp.ErrorIDPNotFound.Code:
			status = http.StatusNotFound
		case idp.ErrorIDPAlreadyExists.Code:
			status = http.StatusConflict
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

// writeInvalidBody writes a 400 response for a malformed request body.
func writeInvalidBody(ctx context.Context, w http.ResponseWriter) {
	sysutils.WriteErrorResponse(ctx, w, http.StatusBadRequest, apierror.ErrorResponse{
		Code:        idp.ErrorInvalidRequestFormat.Code,
		Message:     idp.ErrorInvalidRequestFormat.Error,
		Description: idp.ErrorInvalidRequestFormat.ErrorDescription,
	})
}
