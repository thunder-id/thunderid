// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package bff

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

const maxOIDCResponseBytes = 1 << 20

// providerMetadata is the part of the OpenID Provider metadata the BFF uses.
type providerMetadata struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	EndSessionEndpoint    string `json:"end_session_endpoint"`
}

// tokenResponse is the token endpoint's response.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
}

// tokenError is the token endpoint's error response.
type tokenError struct {
	Status      int
	Code        string `json:"error"`
	Description string `json:"error_description"`
}

func (e *tokenError) Error() string {
	return fmt.Sprintf("token endpoint returned %d %s: %s", e.Status, e.Code, e.Description)
}

// oidcClient talks to the authorization server as the BFF's OAuth client.
type oidcClient struct {
	httpClient *http.Client
	issuerURL  string
	clientID   string

	mu       sync.Mutex
	metadata *providerMetadata
}

// getMetadata returns the provider metadata, fetching it on first use. A failed fetch is not
// cached, so the next call retries.
func (c *oidcClient) getMetadata(ctx context.Context) (*providerMetadata, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.metadata != nil {
		return c.metadata, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.issuerURL+"/.well-known/openid-configuration", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery returned %d", resp.StatusCode)
	}

	var md providerMetadata
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxOIDCResponseBytes)).Decode(&md); err != nil {
		return nil, fmt.Errorf("failed to decode discovery response: %w", err)
	}
	if md.AuthorizationEndpoint == "" || md.TokenEndpoint == "" {
		return nil, fmt.Errorf("discovery response is missing the authorization or token endpoint")
	}
	c.metadata = &md
	return c.metadata, nil
}

// exchangeCode redeems an authorization code.
func (c *oidcClient) exchangeCode(ctx context.Context, code, verifier, redirectURI string) (*tokenResponse, error) {
	return c.requestToken(ctx, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {redirectURI},
	})
}

// refresh runs the refresh token grant.
func (c *oidcClient) refresh(ctx context.Context, refreshToken string) (*tokenResponse, error) {
	return c.requestToken(ctx, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	})
}

func (c *oidcClient) requestToken(ctx context.Context, form url.Values) (*tokenResponse, error) {
	md, err := c.getMetadata(ctx)
	if err != nil {
		return nil, err
	}
	form.Set("client_id", c.clientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, md.TokenEndpoint,
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body := io.LimitReader(resp.Body, maxOIDCResponseBytes)

	if resp.StatusCode != http.StatusOK {
		tokenErr := &tokenError{Status: resp.StatusCode}
		_ = json.NewDecoder(body).Decode(tokenErr)
		return nil, tokenErr
	}

	var tokens tokenResponse
	if err := json.NewDecoder(body).Decode(&tokens); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}
	if tokens.AccessToken == "" {
		return nil, fmt.Errorf("token response has no access token")
	}
	return &tokens, nil
}
