// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mockservices

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

// GithubOAuthServer provides a mock GitHub OAuth server for testing
type GithubOAuthServer struct {
	handler       http.Handler
	baseURL       string
	mutex         sync.RWMutex
	authCodes     map[string]*GithubAuthCodeData
	accessTokens  map[string]*GithubTokenData
	users         map[string]*GithubUserInfo
	emails        map[string][]*GithubEmail
	clientID      string
	clientSecret  string
	authorizeFunc func(login string) (string, error) // Custom authorize function for tests
}

// GithubAuthCodeData stores information about authorization codes
type GithubAuthCodeData struct {
	Code        string
	Login       string
	Scopes      []string
	State       string
	ExpiresAt   time.Time
	RedirectURI string
}

// GithubTokenData stores information about access tokens
type GithubTokenData struct {
	AccessToken string
	TokenType   string
	Login       string
	Scopes      []string
	ExpiresAt   time.Time
}

// GithubUserInfo represents GitHub user information
type GithubUserInfo struct {
	Login             string  `json:"login"`
	ID                int64   `json:"id"`
	NodeID            string  `json:"node_id"`
	AvatarURL         string  `json:"avatar_url"`
	GravatarID        string  `json:"gravatar_id"`
	URL               string  `json:"url"`
	HTMLURL           string  `json:"html_url"`
	FollowersURL      string  `json:"followers_url"`
	FollowingURL      string  `json:"following_url"`
	GistsURL          string  `json:"gists_url"`
	StarredURL        string  `json:"starred_url"`
	SubscriptionsURL  string  `json:"subscriptions_url"`
	OrganizationsURL  string  `json:"organizations_url"`
	ReposURL          string  `json:"repos_url"`
	EventsURL         string  `json:"events_url"`
	ReceivedEventsURL string  `json:"received_events_url"`
	Type              string  `json:"type"`
	SiteAdmin         bool    `json:"site_admin"`
	Name              string  `json:"name"`
	Company           string  `json:"company"`
	Blog              string  `json:"blog"`
	Location          string  `json:"location"`
	Email             *string `json:"email"`
	Hireable          *bool   `json:"hireable"`
	Bio               string  `json:"bio"`
	TwitterUsername   *string `json:"twitter_username"`
	PublicRepos       int     `json:"public_repos"`
	PublicGists       int     `json:"public_gists"`
	Followers         int     `json:"followers"`
	Following         int     `json:"following"`
	CreatedAt         string  `json:"created_at"`
	UpdatedAt         string  `json:"updated_at"`
}

// GithubEmail represents GitHub email information
type GithubEmail struct {
	Email      string `json:"email"`
	Primary    bool   `json:"primary"`
	Verified   bool   `json:"verified"`
	Visibility string `json:"visibility,omitempty"`
}

// NewGithubOAuthServer creates a new mock GitHub OAuth server
func NewGithubOAuthServer(baseURL, clientID, clientSecret string) *GithubOAuthServer {
	m := &GithubOAuthServer{
		baseURL:      baseURL,
		authCodes:    make(map[string]*GithubAuthCodeData),
		accessTokens: make(map[string]*GithubTokenData),
		users:        make(map[string]*GithubUserInfo),
		emails:       make(map[string][]*GithubEmail),
		clientID:     clientID,
		clientSecret: clientSecret,
	}
	m.handler = m.routes()
	return m
}

// routes maps the mock's endpoints.
func (m *GithubOAuthServer) routes() http.Handler {
	mux := http.NewServeMux()

	// OAuth authorization endpoint
	mux.HandleFunc("/login/oauth/authorize", m.handleAuthorize)

	// OAuth token endpoint
	mux.HandleFunc("/login/oauth/access_token", m.handleAccessToken)

	// User API endpoint
	mux.HandleFunc("/user", m.handleUser)

	// User emails API endpoint
	mux.HandleFunc("/user/emails", m.handleUserEmails)

	// Token check endpoint
	mux.HandleFunc("/applications/", m.handleApplications)

	return mux
}

// GetURL returns the base URL of the mock server
func (m *GithubOAuthServer) GetURL() string {
	return m.baseURL
}

// GetAuthorizeURL returns the authorization endpoint URL
func (m *GithubOAuthServer) GetAuthorizeURL() string {
	return fmt.Sprintf("%s/login/oauth/authorize", m.GetURL())
}

// GetAccessTokenURL returns the access token endpoint URL
func (m *GithubOAuthServer) GetAccessTokenURL() string {
	return fmt.Sprintf("%s/login/oauth/access_token", m.GetURL())
}

// GetAPIURL returns the API base URL
func (m *GithubOAuthServer) GetAPIURL() string {
	return m.GetURL()
}

// SetAuthorizeFunc sets a custom authorization function for testing
func (m *GithubOAuthServer) SetAuthorizeFunc(fn func(login string) (string, error)) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.authorizeFunc = fn
}

// AddUser adds a test user to the mock server
func (m *GithubOAuthServer) AddUser(user *GithubUserInfo, emails []*GithubEmail) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.users[user.Login] = user
	if emails != nil {
		m.emails[user.Login] = emails
	}
}

// handleAuthorize handles the OAuth authorization endpoint
func (m *GithubOAuthServer) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query()
	clientID := query.Get("client_id")
	redirectURI := query.Get("redirect_uri")
	state := query.Get("state")
	scope := query.Get("scope")

	// Validate client ID
	if clientID != m.clientID {
		http.Error(w, "Invalid client_id", http.StatusBadRequest)
		return
	}

	// Parse scopes
	var scopes []string
	if scope != "" {
		scopes = strings.Split(scope, ",")
	}

	// Use custom authorize function if set
	var login string
	var err error
	if m.authorizeFunc != nil {
		login, err = m.authorizeFunc("")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		// Default: use first user or create a default one
		m.mutex.RLock()
		if len(m.users) == 0 {
			// Create default user
			login = "testuser"
			m.mutex.RUnlock()
			email := "test@example.com"
			m.AddUser(&GithubUserInfo{
				Login:     login,
				ID:        12345,
				NodeID:    "MDQ6VXNlcjEyMzQ1",
				AvatarURL: "https://avatars.githubusercontent.com/u/12345?v=4",
				URL:       fmt.Sprintf("%s/users/%s", m.GetURL(), login),
				HTMLURL:   fmt.Sprintf("https://github.com/%s", login),
				Type:      "User",
				SiteAdmin: false,
				Name:      "Test User",
				Email:     &email,
				CreatedAt: "2020-01-01T00:00:00Z",
				UpdatedAt: "2024-01-01T00:00:00Z",
			}, []*GithubEmail{
				{
					Email:    email,
					Primary:  true,
					Verified: true,
				},
			})
		} else {
			// Use first user
			for _, user := range m.users {
				login = user.Login
				break
			}
			m.mutex.RUnlock()
		}
	}

	// Generate authorization code
	code := generateGithubRandomString(32)
	m.mutex.Lock()
	m.authCodes[code] = &GithubAuthCodeData{
		Code:        code,
		Login:       login,
		Scopes:      scopes,
		State:       state,
		ExpiresAt:   time.Now().Add(10 * time.Minute),
		RedirectURI: redirectURI,
	}
	m.mutex.Unlock()

	// Redirect to redirect_uri with code and state
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

// handleAccessToken handles the OAuth access token endpoint
func (m *GithubOAuthServer) handleAccessToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	clientID := r.FormValue("client_id")
	clientSecret := r.FormValue("client_secret")
	code := r.FormValue("code")
	redirectURI := r.FormValue("redirect_uri")

	// Validate client credentials
	if clientID != m.clientID || clientSecret != m.clientSecret {
		writeGithubTokenError(w, r.Header.Get("Accept"), "bad_verification_code",
			"The client_id and/or client_secret passed are incorrect.")
		return
	}

	// Validate authorization code
	m.mutex.Lock()
	authCodeData, exists := m.authCodes[code]
	if !exists {
		m.mutex.Unlock()
		writeGithubTokenError(w, r.Header.Get("Accept"), "bad_verification_code",
			"The code passed is incorrect or expired.")
		return
	}

	// Check if code is expired
	if time.Now().After(authCodeData.ExpiresAt) {
		delete(m.authCodes, code)
		m.mutex.Unlock()
		writeGithubTokenError(w, r.Header.Get("Accept"), "bad_verification_code",
			"The code passed is incorrect or expired.")
		return
	}

	// Validate redirect URI if provided
	if redirectURI != "" && authCodeData.RedirectURI != redirectURI {
		m.mutex.Unlock()
		writeGithubTokenError(w, r.Header.Get("Accept"), "redirect_uri_mismatch",
			"The redirect_uri does not match.")
		return
	}

	// Delete used authorization code
	login := authCodeData.Login
	scopes := authCodeData.Scopes
	delete(m.authCodes, code)
	m.mutex.Unlock()

	// Generate access token
	accessToken := "gho_" + generateGithubRandomString(36)

	// Store access token
	m.mutex.Lock()
	m.accessTokens[accessToken] = &GithubTokenData{
		AccessToken: accessToken,
		TokenType:   "bearer",
		Login:       login,
		Scopes:      scopes,
		ExpiresAt:   time.Now().Add(8 * time.Hour),
	}
	m.mutex.Unlock()

	// Determine response format based on Accept header
	acceptHeader := r.Header.Get("Accept")
	scopeStr := strings.Join(scopes, ",")

	if strings.Contains(acceptHeader, "application/json") {
		response := map[string]interface{}{
			"access_token": accessToken,
			"token_type":   "bearer",
			"scope":        scopeStr,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	} else {
		// Default to form-encoded response
		response := url.Values{}
		response.Set("access_token", accessToken)
		response.Set("token_type", "bearer")
		response.Set("scope", scopeStr)

		w.Header().Set("Content-Type", "application/x-www-form-urlencoded")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response.Encode()))
	}
}

// handleUser handles the user API endpoint
func (m *GithubOAuthServer) handleUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract access token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		// Also check for access_token query parameter (deprecated but still supported)
		accessToken := r.URL.Query().Get("access_token")
		if accessToken == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"message": "Requires authentication",
			})
			return
		}
		authHeader = "token " + accessToken
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || (parts[0] != "token" && parts[0] != "Bearer") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Bad credentials",
		})
		return
	}

	accessToken := parts[1]

	// Validate access token
	m.mutex.RLock()
	tokenData, exists := m.accessTokens[accessToken]
	if !exists {
		m.mutex.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Bad credentials",
		})
		return
	}

	// Check if token is expired
	if time.Now().After(tokenData.ExpiresAt) {
		m.mutex.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Bad credentials",
		})
		return
	}

	// Get user info
	user, exists := m.users[tokenData.Login]
	m.mutex.RUnlock()
	if !exists {
		http.Error(w, "User not found", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(user)
}

// handleUserEmails handles the user emails API endpoint
func (m *GithubOAuthServer) handleUserEmails(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract access token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Requires authentication",
		})
		return
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || (parts[0] != "token" && parts[0] != "Bearer") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Bad credentials",
		})
		return
	}

	accessToken := parts[1]

	// Validate access token
	m.mutex.RLock()
	tokenData, exists := m.accessTokens[accessToken]
	if !exists {
		m.mutex.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Bad credentials",
		})
		return
	}

	// Check if token is expired
	if time.Now().After(tokenData.ExpiresAt) {
		m.mutex.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Bad credentials",
		})
		return
	}

	// Check if user:email scope is granted
	hasEmailScope := false
	for _, scope := range tokenData.Scopes {
		if scope == "user:email" || scope == "user" {
			hasEmailScope = true
			break
		}
	}

	if !hasEmailScope {
		m.mutex.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Must have user:email scope",
		})
		return
	}

	// Get user emails
	emails, exists := m.emails[tokenData.Login]
	m.mutex.RUnlock()
	if !exists {
		emails = []*GithubEmail{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(emails)
}

// handleApplications handles the applications API endpoint
func (m *GithubOAuthServer) handleApplications(w http.ResponseWriter, r *http.Request) {
	// Extract path to check which endpoint is being called
	path := r.URL.Path

	// Check token endpoint: POST /applications/{client_id}/token
	if strings.HasSuffix(path, "/token") && r.Method == http.MethodPost {
		m.handleCheckToken(w, r)
		return
	}

	// Delete token endpoint: DELETE /applications/{client_id}/token
	if strings.HasSuffix(path, "/token") && r.Method == http.MethodDelete {
		m.handleDeleteToken(w, r)
		return
	}

	http.Error(w, "Not found", http.StatusNotFound)
}

// handleCheckToken handles checking a token
func (m *GithubOAuthServer) handleCheckToken(w http.ResponseWriter, r *http.Request) {
	// Validate basic auth
	username, password, ok := r.BasicAuth()
	if !ok || username != m.clientID || password != m.clientSecret {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Bad credentials",
		})
		return
	}

	// Parse request body
	var body struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate access token
	m.mutex.RLock()
	tokenData, exists := m.accessTokens[body.AccessToken]
	if !exists {
		m.mutex.RUnlock()
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	user, exists := m.users[tokenData.Login]
	m.mutex.RUnlock()
	if !exists {
		http.Error(w, "User not found", http.StatusInternalServerError)
		return
	}

	// Return token info
	response := map[string]interface{}{
		"id":     1,
		"token":  body.AccessToken,
		"scopes": tokenData.Scopes,
		"app": map[string]string{
			"name":      "Test App",
			"url":       m.GetURL(),
			"client_id": m.clientID,
		},
		"user": map[string]interface{}{
			"login":      user.Login,
			"id":         user.ID,
			"avatar_url": user.AvatarURL,
			"type":       user.Type,
		},
		"created_at": "2020-01-01T00:00:00Z",
		"updated_at": time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleDeleteToken handles deleting a token
func (m *GithubOAuthServer) handleDeleteToken(w http.ResponseWriter, r *http.Request) {
	// Validate basic auth
	username, password, ok := r.BasicAuth()
	if !ok || username != m.clientID || password != m.clientSecret {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Bad credentials",
		})
		return
	}

	// Parse request body
	var body struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Delete access token
	m.mutex.Lock()
	delete(m.accessTokens, body.AccessToken)
	m.mutex.Unlock()

	w.WriteHeader(http.StatusNoContent)
}

// writeGithubTokenError writes a GitHub token error response
func writeGithubTokenError(w http.ResponseWriter, acceptHeader, errorCode, errorDescription string) {
	if strings.Contains(acceptHeader, "application/json") {
		response := map[string]string{
			"error":             errorCode,
			"error_description": errorDescription,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
	} else {
		// Form-encoded response
		response := url.Values{}
		response.Set("error", errorCode)
		response.Set("error_description", errorDescription)
		w.Header().Set("Content-Type", "application/x-www-form-urlencoded")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(response.Encode()))
	}
}

// generateGithubRandomString generates a random string of specified length
func generateGithubRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}
