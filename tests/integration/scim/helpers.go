// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
// scimRequest handles scim request.
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
// scimRequestUnauthenticated handles scim request unauthenticated.
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

// extensionStringValue reads a string attribute out of a decoded SCIM user
// response's extension object (the object keyed by the extension URN). This
// is the raw, always-present attribute representation, distinct from the
// mapped core fields (e.g. top-level "emails") which are only populated when
// scim.core_attrs_on_get is enabled.
// extensionStringValue handles extension string value.
func extensionStringValue(resp map[string]interface{}, extensionURN, attr string) (string, bool) {
	ext, ok := resp[extensionURN].(map[string]interface{})
	if !ok {
		return "", false
	}
	v, ok := ext[attr].(string)
	return v, ok
}

// discoverExtensionSchema finds the ThunderID SCIM extension schema for the
// given entity type name via GET /Schemas — the same discovery step a real
// SCIM client performs before provisioning into a given user type. Returns
// the schema's URN (the "schemas" array entry / extension object key) and the
// names of its required attributes.
// discoverExtensionSchema handles discover extension schema.
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
