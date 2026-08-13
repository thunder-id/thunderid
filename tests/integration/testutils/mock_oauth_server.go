// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// OAuthAuthCodeData stores information about OAuth authorization codes
type OAuthAuthCodeData struct {
	Code         string
	UserID       string
	Scopes       []string
	State        string
	ExpiresAt    time.Time
	RedirectURI  string
	CodeVerifier string // For PKCE
	Nonce        string // For OIDC
}

// OAuthTokenData stores information about OAuth access tokens
type OAuthTokenData struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	UserID       string
	Scopes       []string
	ExpiresAt    time.Time
	IDToken      string // For OIDC
}

// OAuthUserInfo represents generic OAuth user information
type OAuthUserInfo struct {
	Sub     string                 `json:"sub"`
	Email   string                 `json:"email,omitempty"`
	Name    string                 `json:"name,omitempty"`
	Picture string                 `json:"picture,omitempty"`
	Custom  map[string]interface{} `json:"-"` // Additional custom fields
}

// MockOAuthServer provides a base mock OAuth 2.0 server
type MockOAuthServer struct {
	server        *http.Server
	port          int
	mutex         sync.RWMutex
	authCodes     map[string]*OAuthAuthCodeData
	accessTokens  map[string]*OAuthTokenData
	users         map[string]*OAuthUserInfo
	clientID      string
	clientSecret  string
	baseURL       string
	authorizeFunc func(userID string) (string, error)

	// Response overrides drive the transport failures a well-behaved provider never produces.
	tokenOverride    func() (int, string)
	userInfoOverride func() (int, string)

	// Configurable endpoints
	authorizePath string
	tokenPath     string
	userInfoPath  string
}

// NewMockOAuthServer creates a new mock OAuth 2.0 server
func NewMockOAuthServer(port int, clientID, clientSecret string) *MockOAuthServer {
	return &MockOAuthServer{
		port:          port,
		authCodes:     make(map[string]*OAuthAuthCodeData),
		accessTokens:  make(map[string]*OAuthTokenData),
		users:         make(map[string]*OAuthUserInfo),
		clientID:      clientID,
		clientSecret:  clientSecret,
		baseURL:       fmt.Sprintf("http://localhost:%d", port),
		authorizePath: "/oauth/authorize",
		tokenPath:     "/oauth/token",
		userInfoPath:  "/oauth/userinfo",
	}
}

// Start starts the mock OAuth server
func (m *MockOAuthServer) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc(m.authorizePath, m.handleAuthorize)
	mux.HandleFunc(m.tokenPath, m.handleToken)
	mux.HandleFunc(m.userInfoPath, m.handleUserInfo)

	m.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", m.port),
		Handler: mux,
	}

	go func() {
		if err := m.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Mock OAuth server error: %v\n", err)
		}
	}()

	return nil
}

// Stop stops the mock OAuth server
func (m *MockOAuthServer) Stop() error {
	if m.server != nil {
		return m.server.Close()
	}
	return nil
}

// GetURL returns the base URL
func (m *MockOAuthServer) GetURL() string {
	return m.baseURL
}

// GetAuthorizeURL returns the authorization endpoint URL
func (m *MockOAuthServer) GetAuthorizeURL() string {
	return m.baseURL + m.authorizePath
}

// GetTokenURL returns the token endpoint URL
func (m *MockOAuthServer) GetTokenURL() string {
	return m.baseURL + m.tokenPath
}

// GetUserInfoURL returns the userinfo endpoint URL
func (m *MockOAuthServer) GetUserInfoURL() string {
	return m.baseURL + m.userInfoPath
}

// SetAuthorizeFunc sets a custom authorization function
func (m *MockOAuthServer) SetAuthorizeFunc(fn func(userID string) (string, error)) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.authorizeFunc = fn
}

// SetTokenResponseOverride replaces the whole token response with the given status and body.
func (m *MockOAuthServer) SetTokenResponseOverride(fn func() (int, string)) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.tokenOverride = fn
}

// SetUserInfoResponseOverride replaces the whole userinfo response with the given status and body.
func (m *MockOAuthServer) SetUserInfoResponseOverride(fn func() (int, string)) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.userInfoOverride = fn
}

// ClearOverrides restores normal behaviour on both endpoints between tests.
func (m *MockOAuthServer) ClearOverrides() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.tokenOverride = nil
	m.userInfoOverride = nil
}

// writeOAuthOverride serves an override response and reports whether it handled the request.
func writeOAuthOverride(w http.ResponseWriter, fn func() (int, string)) bool {
	if fn == nil {
		return false
	}
	status, body := fn()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
	return true
}

// AddUser adds a user to the mock server
func (m *MockOAuthServer) AddUser(user *OAuthUserInfo) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.users[user.Sub] = user
}

// handleAuthorize handles the OAuth authorization endpoint
func (m *MockOAuthServer) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query()
	clientID := query.Get("client_id")
	redirectURI := query.Get("redirect_uri")
	state := query.Get("state")
	scope := query.Get("scope")
	responseType := query.Get("response_type")
	nonce := query.Get("nonce")

	// Validate client ID
	if clientID != m.clientID {
		http.Error(w, "Invalid client_id", http.StatusBadRequest)
		return
	}

	// Validate response type
	if responseType != "code" {
		http.Error(w, "Unsupported response_type", http.StatusBadRequest)
		return
	}

	// Parse scopes
	var scopes []string
	if scope != "" {
		scopes = strings.Split(scope, " ")
	}

	// Get user to authenticate
	var userID string
	var err error
	if m.authorizeFunc != nil {
		userID, err = m.authorizeFunc("")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		// Use first user or create default
		m.mutex.RLock()
		if len(m.users) == 0 {
			userID = "default-user"
			m.mutex.RUnlock()
			m.AddUser(&OAuthUserInfo{
				Sub:   userID,
				Email: "user@example.com",
				Name:  "Test User",
			})
		} else {
			for _, user := range m.users {
				userID = user.Sub
				break
			}
			m.mutex.RUnlock()
		}
	}

	// Generate authorization code
	code := generateOAuthRandomString(32)
	m.mutex.Lock()
	m.authCodes[code] = &OAuthAuthCodeData{
		Code:        code,
		UserID:      userID,
		Scopes:      scopes,
		State:       state,
		Nonce:       nonce,
		ExpiresAt:   time.Now().Add(10 * time.Minute),
		RedirectURI: redirectURI,
	}
	m.mutex.Unlock()

	// Redirect with code
	redirectURL, err := url.Parse(redirectURI)
	if err != nil {
		http.Error(w, "Invalid redirect_uri", http.StatusBadRequest)
		return
	}

	q := redirectURL.Query()
	q.Set("code", code)
	if state != "" {
		q.Set("state", state)
	}
	redirectURL.RawQuery = q.Encode()

	http.Redirect(w, r, redirectURL.String(), http.StatusFound)
}

// handleToken handles the OAuth token endpoint
func (m *MockOAuthServer) handleToken(w http.ResponseWriter, r *http.Request) {
	m.mutex.RLock()
	override := m.tokenOverride
	m.mutex.RUnlock()
	if writeOAuthOverride(w, override) {
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	grantType := r.FormValue("grant_type")
	code := r.FormValue("code")
	clientID := r.FormValue("client_id")
	clientSecret := r.FormValue("client_secret")
	redirectURI := r.FormValue("redirect_uri")

	// Validate grant type
	if grantType != "authorization_code" {
		writeOAuthTokenError(w, "unsupported_grant_type", "Grant type not supported")
		return
	}

	// Validate client credentials
	if clientID != m.clientID || clientSecret != m.clientSecret {
		writeOAuthTokenError(w, "invalid_client", "Invalid client credentials")
		return
	}

	// Validate authorization code
	m.mutex.Lock()
	authCodeData, exists := m.authCodes[code]
	if !exists {
		m.mutex.Unlock()
		writeOAuthTokenError(w, "invalid_grant", "Invalid authorization code")
		return
	}

	// Check expiration
	if time.Now().After(authCodeData.ExpiresAt) {
		delete(m.authCodes, code)
		m.mutex.Unlock()
		writeOAuthTokenError(w, "invalid_grant", "Authorization code expired")
		return
	}

	// Validate redirect URI
	if authCodeData.RedirectURI != redirectURI {
		m.mutex.Unlock()
		writeOAuthTokenError(w, "invalid_grant", "Redirect URI mismatch")
		return
	}

	// Get user
	user, exists := m.users[authCodeData.UserID]
	if !exists {
		m.mutex.Unlock()
		writeOAuthTokenError(w, "invalid_grant", "User not found")
		return
	}

	// Delete used code
	delete(m.authCodes, code)
	m.mutex.Unlock()

	// Generate tokens
	accessToken := "oauth_" + generateOAuthRandomString(40)
	refreshToken := "refresh_" + generateOAuthRandomString(40)

	// Store token
	m.mutex.Lock()
	m.accessTokens[accessToken] = &OAuthTokenData{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		UserID:       authCodeData.UserID,
		Scopes:       authCodeData.Scopes,
		ExpiresAt:    time.Now().Add(1 * time.Hour),
	}
	m.mutex.Unlock()

	// Build response
	response := map[string]interface{}{
		"access_token":  accessToken,
		"token_type":    "Bearer",
		"expires_in":    3600,
		"refresh_token": refreshToken,
		"scope":         strings.Join(authCodeData.Scopes, " "),
	}

	// Add user info in response if requested
	if contains(authCodeData.Scopes, "openid") {
		response["id"] = user.Sub
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleUserInfo handles the OAuth userinfo endpoint
func (m *MockOAuthServer) handleUserInfo(w http.ResponseWriter, r *http.Request) {
	m.mutex.RLock()
	override := m.userInfoOverride
	m.mutex.RUnlock()
	if writeOAuthOverride(w, override) {
		return
	}

	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract access token
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Missing authorization header", http.StatusUnauthorized)
		return
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		http.Error(w, "Invalid authorization header", http.StatusUnauthorized)
		return
	}

	accessToken := parts[1]

	// Validate token
	m.mutex.RLock()
	tokenData, exists := m.accessTokens[accessToken]
	if !exists {
		m.mutex.RUnlock()
		http.Error(w, "Invalid access token", http.StatusUnauthorized)
		return
	}

	if time.Now().After(tokenData.ExpiresAt) {
		m.mutex.RUnlock()
		http.Error(w, "Access token expired", http.StatusUnauthorized)
		return
	}

	// Get user
	user, exists := m.users[tokenData.UserID]
	m.mutex.RUnlock()
	if !exists {
		http.Error(w, "User not found", http.StatusInternalServerError)
		return
	}

	// Build response
	response := map[string]interface{}{
		"sub": user.Sub,
	}

	if user.Email != "" {
		response["email"] = user.Email
	}
	if user.Name != "" {
		response["name"] = user.Name
	}
	if user.Picture != "" {
		response["picture"] = user.Picture
	}

	// Add custom fields
	for k, v := range user.Custom {
		response[k] = v
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// writeOAuthTokenError writes a token error response
func writeOAuthTokenError(w http.ResponseWriter, errorCode, errorDescription string) {
	response := map[string]string{
		"error":             errorCode,
		"error_description": errorDescription,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(response)
}

// generateOAuthRandomString generates a random string
func generateOAuthRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

// contains checks if a string slice contains a value
func contains(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
