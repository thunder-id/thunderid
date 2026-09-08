// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// dataPlaneTimeout bounds a call to the data plane. An apply carries the whole configuration, so it
// is generous, but it is bounded: a data plane that never answers must not hold the request open.
const dataPlaneTimeout = 60 * time.Second

// ImportOutcome is one resource's result, as the data plane reports it.
type ImportOutcome struct {
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	ResourceName string `json:"resourceName"`
	Operation    string `json:"operation"`
	Status       string `json:"status"`
	Code         string `json:"code,omitempty"`
	Message      string `json:"message,omitempty"`
}

// ImportSummary is the aggregate result of an apply.
type ImportSummary struct {
	TotalDocuments int    `json:"totalDocuments"`
	Imported       int    `json:"imported"`
	Failed         int    `json:"failed"`
	ImportedAt     string `json:"importedAt,omitempty"`
}

// ImportResult is what the data plane answers an apply with.
type ImportResult struct {
	Summary *ImportSummary  `json:"summary,omitempty"`
	Results []ImportOutcome `json:"results,omitempty"`
}

// dataPlaneClient applies configuration to one data plane.
//
// It authenticates as the machine to machine client registered with the gateway, so the control
// plane reaches the data plane over its ordinary API and neither needs an inbound path to the other.
type dataPlaneClient struct {
	http *http.Client
}

func newDataPlaneClient() *dataPlaneClient {
	return &dataPlaneClient{http: &http.Client{Timeout: dataPlaneTimeout}}
}

// Apply posts the configuration to the data plane's import API.
func (c *dataPlaneClient) Apply(
	ctx context.Context, gw *Gateway, secret, content string, variables map[string]string,
) (*ImportResult, error) {
	token, err := c.token(ctx, gw, secret)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(map[string]interface{}{
		"content":   content,
		"variables": variables,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to build the apply request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, gw.BaseURL+"/import", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to build the apply request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("the gateway could not be reached: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read the gateway's answer: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("the gateway answered %d to the apply", resp.StatusCode)
	}

	var result ImportResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("the gateway returned an unreadable answer to the apply")
	}
	return &result, nil
}

// token obtains an access token from the data plane by client credentials.
func (c *dataPlaneClient) token(ctx context.Context, gw *Gateway, secret string) (string, error) {
	form := url.Values{"grant_type": {"client_credentials"}}
	if gw.Scope != "" {
		form.Set("scope", gw.Scope)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		gw.BaseURL+"/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to build the token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// The pair is presented as HTTP Basic auth, per RFC 6749 section 2.3.1.
	req.SetBasicAuth(gw.ClientID, secret)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("the gateway's token endpoint could not be reached: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read the token response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// The body is not quoted back: a token endpoint error can echo the credential.
		return "", fmt.Errorf("the gateway's token endpoint answered %d", resp.StatusCode)
	}

	var token struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &token); err != nil || token.AccessToken == "" {
		return "", fmt.Errorf("the gateway returned no access token")
	}
	return token.AccessToken, nil
}
