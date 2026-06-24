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

package serverconfig

import (
	"encoding/json"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	testServerURL   = testutils.TestServerURL
	serverConfigURL = testServerURL + "/server-config"

	// sampleOrigin is a valid origin used to exercise the store's read/write round-trip. The only
	// registered config consumer is CORS, so stored values must be valid CORS origins.
	sampleOrigin = "https://app.example.com"
)

// i18nMessage mirrors the i18n message structure returned in API error responses.
type i18nMessage struct {
	Key          string `json:"key"`
	DefaultValue string `json:"defaultValue"`
}

// apiErrorResponse mirrors apierror.ErrorResponse for decoding error responses.
type apiErrorResponse struct {
	Code        string      `json:"code"`
	Message     i18nMessage `json:"message"`
	Description i18nMessage `json:"description"`
}

// serverConfigResponse mirrors the server-config response; cors is always present.
type serverConfigResponse struct {
	CORS []json.RawMessage `json:"cors"`
}
