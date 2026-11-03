// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	adminTokenState *TokenResponse
	tokenInitOnce   sync.Once
	tokenInitErr    error
)

// authTransport wraps http.RoundTripper to inject authorization headers
type authTransport struct {
	base     http.RoundTripper
	getToken func() (string, error)
}

// DirectAuthHeaderName is the header carrying the Direct Auth Secret on Direct API requests.
const DirectAuthHeaderName = "Direct-Auth-Secret"

// DirectAuthHeaderValue is the Direct Auth Secret the integration server is configured with. The
// harness passes it to setup.sh via the DIRECT_AUTH_SECRET env (see RunSetupScript) so every setup
// run writes this exact value deterministically; test clients inject it on Direct API requests.
const DirectAuthHeaderValue = "integration-direct-auth-secret"

// AdminUsername and AdminPassword are the admin credentials the integration server is seeded with.
// The harness passes them to setup.sh via the ADMIN_USERNAME/ADMIN_PASSWORD env (see RunSetupScript)
// so every setup run seeds this exact password deterministically, rather than the randomly generated
// one setup.sh produces when these are left unset.
const (
	AdminUsername = "admin"
	AdminPassword = "integration-admin-password"
)

// DeclaredGatewayToken is the management token the declared gateway holds
// (resources/declarative_resources/gateways/declared-gateway.yaml). That file names its key as the
// environment reference "{{.GATEWAY_TOKEN}}" rather than carrying the credential, so the harness
// sets GATEWAY_TOKEN to this on every path that starts the server. Unset, the reference fails to
// resolve and the server refuses to start.
const DeclaredGatewayToken = "declared-integration-management-token"

// isDirectAuthPath reports whether the path is one of the Direct API endpoints gated by the Direct API
// Secret.
func isDirectAuthPath(path string) bool {
	return strings.HasPrefix(path, "/auth/") ||
		strings.HasPrefix(path, "/register/passkey/") ||
		strings.HasPrefix(path, "/access/")
}

// RoundTrip implements http.RoundTripper interface
func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	reqClone := req.Clone(req.Context())

	// Inject the Direct Auth Secret on Direct API requests (unless the caller already set one), since
	// the integration server runs with the gate enabled (secure by default).
	if isDirectAuthPath(reqClone.URL.Path) && reqClone.Header.Get(DirectAuthHeaderName) == "" {
		reqClone.Header.Set(DirectAuthHeaderName, DirectAuthHeaderValue)
	}

	// Skip auth for public endpoints
	if isPublicEndpoint(reqClone.URL.Path) {
		return t.base.RoundTrip(reqClone)
	}

	// Get token (auto-refreshes if needed)
	token, err := t.getToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	reqClone.Header.Set("Authorization", "Bearer "+token)

	return t.base.RoundTrip(reqClone)
}

// GetRawHTTPClient returns an HTTP client with no automatic auth or Direct Auth Secret injection, for
// tests that need full control over request headers. TLS verification is skipped for local servers.
func GetRawHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

// isPublicEndpoint determines if an endpoint requires authentication
func isPublicEndpoint(path string) bool {
	// The organization-unit-scoped token and authorize endpoints are public for the same reason the bare
	// forms are: the token caller authenticates as an OAuth client, and authorize is a browser redirect.
	// Injecting the admin bearer here would overwrite or misrepresent that. Mirrors the server's own
	// allow-list, which names these two paths.
	if strings.HasPrefix(path, "/ou/") &&
		(strings.HasSuffix(path, "/oauth2/token") || strings.HasSuffix(path, "/oauth2/authorize")) {
		return true
	}

	publicPrefixes := []string{
		"/health/",
		"/auth/",
		"/flow/execute",
		"/flow/meta",
		"/oauth2/",
		"/.well-known/openid-configuration",
		"/.well-known/oauth-authorization-server",
		"/gate/",    // Gate application (login UI)
		"/console/", // Console application
		"/error",
	}

	for _, prefix := range publicPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	return false
}

// NewHTTPClientWithTokenProvider builds an HTTP client that injects Authorization headers using the provided token
// provider and skips TLS verification to work with local test servers.
func NewHTTPClientWithTokenProvider(getToken func() (string, error)) *http.Client {
	return &http.Client{
		Transport: &authTransport{
			base: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
			getToken: getToken,
		},
	}
}

// GetHTTPClientWithToken returns an HTTP client that always uses the provided bearer token.
func GetHTTPClientWithToken(token string) *http.Client {
	return NewHTTPClientWithTokenProvider(func() (string, error) {
		if token == "" {
			return "", fmt.Errorf("token is empty")
		}
		return token, nil
	})
}

// OUSignInRedirectURI is the redirect URI of the applications CreateOUSignInApplication creates.
const OUSignInRedirectURI = "https://localhost:3000/ou-sign-in"

// CreateOUSignInApplication creates a public client owned by ouID that signs users in through the default flow
// and puts their OU in its access tokens, as the Console does for the default OU. A sign-in is bounded to its
// application's OU, so a user outside the default OU signs in through one of these instead of the Console.
func CreateOUSignInApplication(ouID, clientID string) (string, error) {
	return CreateApplication(Application{
		Name: "OU Sign-in " + clientID,
		OUID: ouID,
		Type: "browser",
		InboundAuthConfig: []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                clientID,
					"redirectUris":            []string{OUSignInRedirectURI},
					"grantTypes":              []string{"authorization_code"},
					"responseTypes":           []string{"code"},
					"pkceRequired":            true,
					"tokenEndpointAuthMethod": "none",
					"publicClient":            true,
					"token": map[string]interface{}{
						"accessToken": map[string]interface{}{
							"userConfig": map[string]interface{}{
								"attributes": []string{"given_name", "family_name", "email", "groups", "name", "ouId"},
							},
						},
					},
				},
			},
		},
	})
}

// OUSignInApps creates one CreateOUSignInApplication client per organization unit, on first use, for a suite whose
// users live outside the default OU. DeleteAll removes them, before the suite deletes the units.
type OUSignInApps struct {
	appIDs    map[string]string
	clientIDs map[string]string
}

// ClientID returns the client ID of ouID's sign-in application, creating the application on first use.
func (a *OUSignInApps) ClientID(ouID string) (string, error) {
	if clientID, ok := a.clientIDs[ouID]; ok {
		return clientID, nil
	}
	clientID := "ou_sign_in_" + ouID
	appID, err := CreateOUSignInApplication(ouID, clientID)
	if err != nil {
		return "", err
	}
	if a.appIDs == nil {
		a.appIDs = map[string]string{}
		a.clientIDs = map[string]string{}
	}
	a.appIDs[ouID] = appID
	a.clientIDs[ouID] = clientID
	return clientID, nil
}

// ObtainAccessToken is ObtainAccessTokenWithPassword through ouID's sign-in application, with PKCE and the
// RFC 8707 resource, which may be empty.
func (a *OUSignInApps) ObtainAccessToken(ouID, scope, username, password, resource string) (*TokenResponse,
	error) {
	clientID, err := a.ClientID(ouID)
	if err != nil {
		return nil, err
	}
	return ObtainAccessTokenWithPassword(clientID, OUSignInRedirectURI, scope, username, password, true, "",
		resource)
}

// HTTPClientForUser is GetHTTPClientForUser through ouID's sign-in application.
func (a *OUSignInApps) HTTPClientForUser(ouID, username, password string) (*http.Client, error) {
	clientID, err := a.ClientID(ouID)
	if err != nil {
		return nil, err
	}
	return GetHTTPClientForOUUser(clientID, username, password)
}

// DeleteAll deletes every sign-in application created, returning the first failure.
func (a *OUSignInApps) DeleteAll() error {
	var firstErr error
	for ouID, appID := range a.appIDs {
		if err := DeleteApplication(appID); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(a.appIDs, ouID)
		delete(a.clientIDs, ouID)
	}
	return firstErr
}

// GetHTTPClientForUser obtains a token using password grant (via CONSOLE app) and returns an HTTP client that
// injects that token. This keeps token generation out of individual tests.
func GetHTTPClientForUser(username, password string) (*http.Client, error) {
	return getHTTPClientForUser("CONSOLE", "https://localhost:8095/console", username, password)
}

// GetHTTPClientForOUUser is GetHTTPClientForUser through a CreateOUSignInApplication client, for a user outside
// the default OU.
func GetHTTPClientForOUUser(clientID, username, password string) (*http.Client, error) {
	return getHTTPClientForUser(clientID, OUSignInRedirectURI, username, password)
}

// getHTTPClientForUser signs the user in through the client and returns an HTTP client that injects the token.
func getHTTPClientForUser(clientID, redirectURI, username, password string) (*http.Client, error) {
	if username == "" || password == "" {
		return nil, fmt.Errorf("username and password are required")
	}

	tokenResp, err := ObtainAccessTokenWithPassword(
		clientID,
		redirectURI,
		"openid",
		username,
		password,
		true,
		"",
		SystemResourceIdentifier,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain token for user %s: %w", username, err)
	}

	if tokenResp == nil || tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("no access token returned for user %s", username)
	}

	return GetHTTPClientWithToken(tokenResp.AccessToken), nil
}

// ObtainAdminAccessToken obtains an admin access token using the CONSOLE app and stores it globally
func ObtainAdminAccessToken() error {
	log.Println("Obtaining admin access token...")
	adminUsername := os.Getenv("ADMIN_USERNAME")
	if adminUsername == "" {
		adminUsername = AdminUsername
	}
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		adminPassword = AdminPassword
	}
	var err error
	adminTokenState, err = ObtainAccessTokenWithPassword(
		"CONSOLE",
		"https://localhost:8095/console",
		"system",
		adminUsername,
		adminPassword,
		true,
	)
	if err != nil {
		return fmt.Errorf("failed to obtain access token: %w", err)
	}
	now := time.Now()
	adminTokenState.ExpiresAt = now.Add(time.Duration(adminTokenState.ExpiresIn) * time.Second)

	// Export complete token state to environment variable for other test packages
	if err := exportTokenStateToEnv(); err != nil {
		return fmt.Errorf("failed to export token state to environment: %w", err)
	}

	log.Printf("Access token obtained successfully")
	return nil
}

// GetAccessToken returns the current access token, refreshing it if necessary
func GetAccessToken() (string, error) {
	// First try to load complete token from environment (set by main runner)
	// This allows token refresh to work across test packages
	if adminTokenState == nil {
		tokenState, err := loadTokenStateFromEnv()
		if err != nil {
			return "", fmt.Errorf("failed to load token state from environment: %w", err)
		}
		if tokenState != nil {
			adminTokenState = tokenState
		}
	}

	// Fallback: Initialize token if not available (for running individual test packages)
	if adminTokenState == nil {
		// Use sync.Once to ensure token is obtained only once even with concurrent calls.
		// Persist the first initialization error so subsequent callers return a clean error
		// instead of dereferencing a nil token state.
		tokenInitOnce.Do(func() {
			log.Println("No token available, obtaining access token automatically...")
			tokenInitErr = ObtainAdminAccessToken()
		})
		if tokenInitErr != nil {
			return "", fmt.Errorf("failed to obtain access token: %w", tokenInitErr)
		}
		if adminTokenState == nil {
			return "", fmt.Errorf("failed to obtain access token: token state is not initialized")
		}
	}

	// Check if token needs refresh
	if err := RefreshTokenIfNeeded(); err != nil {
		return "", fmt.Errorf("failed to refresh token: %w", err)
	}

	return adminTokenState.AccessToken, nil
}

// RefreshTokenIfNeeded checks if the token is expired or expiring soon and refreshes it
func RefreshTokenIfNeeded() error {
	if adminTokenState == nil {
		return nil // No token to refresh
	}

	// Check if refresh is needed (expired or within buffer time)
	if !shouldRefresh(adminTokenState) {
		return nil // Token is still valid
	}

	refreshToken := adminTokenState.RefreshToken

	log.Println("Token expired or expiring soon, refreshing...")

	// Refresh the token
	var err error
	adminTokenState, err = RefreshAccessTokenWithClientCredentialsInBody("CONSOLE", "", refreshToken)
	if err != nil {
		return fmt.Errorf("failed to refresh access token: %w", err)
	}

	now := time.Now()
	adminTokenState.ExpiresAt = now.Add(time.Duration(adminTokenState.ExpiresIn) * time.Second)

	// Update environment variable so other test packages can use refreshed token
	if err := exportTokenStateToEnv(); err != nil {
		log.Printf("Warning: Failed to update token state in environment: %v\n", err)
		// Don't fail - the refresh was successful, just the env update failed
	}

	log.Printf("Access token refreshed successfully")
	return nil
}

func shouldRefresh(tokenState *TokenResponse) bool {
	if tokenState == nil {
		return false
	}
	now := time.Now()
	// Refresh if within 5 minutes of expiry
	return now.After(tokenState.ExpiresAt.Add(-5 * time.Minute))
}

// exportTokenStateToEnv serializes the current global token state and exports it to environment
func exportTokenStateToEnv() error {
	if adminTokenState == nil {
		return fmt.Errorf("no token state available to export")
	}

	// Serialize to JSON
	jsonBytes, err := json.Marshal(adminTokenState)
	if err != nil {
		return fmt.Errorf("failed to serialize token state: %w", err)
	}

	// Encode as base64 for safe environment variable storage
	encoded := base64.StdEncoding.EncodeToString(jsonBytes)
	os.Setenv("TEST_ADMIN_TOKEN", encoded)

	log.Printf("Token state exported to environment")
	return nil
}

// loadTokenStateFromEnv deserializes token state from environment variable
func loadTokenStateFromEnv() (*TokenResponse, error) {
	encoded := os.Getenv("TEST_ADMIN_TOKEN")
	if encoded == "" {
		return nil, nil // No token state in environment
	}

	jsonBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode token state: %w", err)
	}

	// Deserialize from JSON
	var tokenState TokenResponse
	if err := json.Unmarshal(jsonBytes, &tokenState); err != nil {
		return nil, fmt.Errorf("failed to deserialize token state: %w", err)
	}

	log.Printf("Token state loaded from environment")
	return &tokenState, nil
}
