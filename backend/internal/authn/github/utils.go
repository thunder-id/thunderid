/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package github

import (
	"encoding/json"
	"io"
	"net/http"

	sysconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// buildUserEmailRequest constructs the HTTP request to fetch user emails from GitHub.
func buildUserEmailRequest(userEmailEndpoint string, accessToken string, logger *log.Logger) (
	*http.Request, *serviceerror.ServiceError) {
	req, err := http.NewRequest(http.MethodGet, userEmailEndpoint, nil)
	if err != nil {
		logger.Error("Failed to create user email request", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	req.Header.Set(sysconst.AuthorizationHeaderName, sysconst.TokenTypeBearer+" "+accessToken)
	req.Header.Set(sysconst.AcceptHeaderName, sysconst.ContentTypeJSON)

	return req, nil
}

// sendUserEmailRequest sends the user email request to GitHub and processes the response.
func sendUserEmailRequest(httpReq *http.Request, httpClient syshttp.HTTPClientInterface, logger *log.Logger) (
	[]map[string]interface{}, *serviceerror.ServiceError) {
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		logger.Error("User email request to GitHub failed", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Error("Failed to close user email response body", log.Error(closeErr))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		logger.Error("User email endpoint returned an error response",
			log.Int("statusCode", resp.StatusCode), log.String("response", string(body)))
		return nil, &serviceerror.InternalServerError
	}

	var emails []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		logger.Error("Failed to decode user email response", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	return emails, nil
}
