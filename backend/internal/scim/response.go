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

package scim

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/log"
)

// scimContentType is the SCIM-specific content type required by RFC 7644.
const scimContentType = "application/scim+json"

// SCIMErrorResponse is the SCIM-standard error payload shape (RFC 7643 §3.12).
// This is what goes over the wire to SCIM clients — never internal error codes.
type SCIMErrorResponse struct {
	Schemas  []string `json:"schemas"`
	Status   string   `json:"status"`
	ScimType string   `json:"scimType,omitempty"`
	Detail   string   `json:"detail,omitempty"`
}

// writeSCIMSuccessResponse writes a SCIM-compliant success response.
// Uses application/scim+json as required by RFC 7644, and uses a
// buffer-first pattern to avoid sending headers before encoding succeeds.
func writeSCIMSuccessResponse(ctx context.Context, w http.ResponseWriter, data any) {
	logger := log.GetLogger()

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(data); err != nil {
		logger.Error(ctx, "Failed to encode SCIM response", log.Error(err))
		w.Header().Set("Content-Type", scimContentType)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", scimContentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

// writeSCIMErrorResponse writes a SCIM-standard error response.
// Uses the same buffer-first pattern as writeSCIMSuccessResponse so that
// headers are never committed before encoding is confirmed to succeed.
// Always sends the SCIM wire format — never internal ThunderID error codes.
func writeSCIMErrorResponse(ctx context.Context, w http.ResponseWriter, statusCode int, scimErr SCIMErrorResponse) {
	logger := log.GetLogger()

	if len(scimErr.Schemas) == 0 {
		scimErr.Schemas = []string{SCIMErrorSchemaURN}
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(scimErr); err != nil {
		logger.Error(ctx, "Failed to encode SCIM error response", log.Error(err))
		w.Header().Set("Content-Type", scimContentType)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", scimContentType)
	w.WriteHeader(statusCode)
	_, _ = w.Write(buf.Bytes())
}
