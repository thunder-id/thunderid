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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// scimRequest issues an authenticated HTTP request against the SCIM API and
// returns the raw status code and response body. Content-Type is set to
// application/scim+json whenever a body is present, matching what a real
// SCIM client sends.
func scimRequest(method, path string, body []byte, headers map[string]string) (int, []byte, error) {
	client := testutils.GetHTTPClient()

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, scimBaseURL+path, reader)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/scim+json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read response body: %w", err)
	}
	return resp.StatusCode, respBody, nil
}

// scimRequestUnauthenticated issues a request against the SCIM API with no
// token-injecting transport, so an empty or garbage Authorization header
// (headers) actually reaches the server as-is. testutils.GetHTTPClient's
// transport unconditionally overwrites Authorization on every call, which
// makes it unusable for no-token/invalid-token tests.
func scimRequestUnauthenticated(method, path string, headers map[string]string) (int, []byte, error) {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest(method, scimBaseURL+path, nil)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to build request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read response body: %w", err)
	}
	return resp.StatusCode, respBody, nil
}

// discoverExtensionSchema finds the ThunderID SCIM extension schema for the
// given entity type name via GET /Schemas — the same discovery step a real
// SCIM client performs before provisioning into a given user type. Returns
// the schema's URN (the "schemas" array entry / extension object key) and the
// names of its required attributes.
func discoverExtensionSchema(entityTypeName string) (urn string, required []string, err error) {
	status, body, err := scimRequest(http.MethodGet, "/Schemas", nil, nil)
	if err != nil {
		return "", nil, err
	}
	if status != http.StatusOK {
		return "", nil, fmt.Errorf("GET /Schemas returned %d: %s", status, body)
	}

	var list scimSchemaListResponse
	if err := json.Unmarshal(body, &list); err != nil {
		return "", nil, fmt.Errorf("failed to parse Schemas list: %w", err)
	}

	for _, s := range list.Resources {
		if !strings.EqualFold(s.Name, entityTypeName) {
			continue
		}
		for _, attr := range s.Attributes {
			if attr.Required {
				required = append(required, attr.Name)
			}
		}
		return s.ID, required, nil
	}
	return "", nil, fmt.Errorf("no SCIM extension schema found for entity type %q", entityTypeName)
}
