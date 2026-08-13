// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// OIDCUserInfo represents OIDC user information
type OIDCUserInfo struct {
	Sub           string                 `json:"sub"`
	Email         string                 `json:"email,omitempty"`
	EmailVerified bool                   `json:"email_verified,omitempty"`
	Name          string                 `json:"name,omitempty"`
	GivenName     string                 `json:"given_name,omitempty"`
	FamilyName    string                 `json:"family_name,omitempty"`
	Picture       string                 `json:"picture,omitempty"`
	Locale        string                 `json:"locale,omitempty"`
	Custom        map[string]interface{} `json:"-"` // Additional custom fields
}

// OIDCDiscoveryDocument represents OIDC discovery document
type OIDCDiscoveryDocument struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	UserinfoEndpoint                  string   `json:"userinfo_endpoint"`
	JwksURI                           string   `json:"jwks_uri"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	SubjectTypesSupported             []string `json:"subject_types_supported"`
	IDTokenSigningAlgValuesSupported  []string `json:"id_token_signing_alg_values_supported"`
	ScopesSupported                   []string `json:"scopes_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	ClaimsSupported                   []string `json:"claims_supported"`
}

// OIDCJWKS represents JSON Web Key Set
type OIDCJWKS struct {
	Keys []OIDCJWK `json:"keys"`
}

// OIDCJWK represents a JSON Web Key
type OIDCJWK struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// OIDCAuthCodeData stores information about OIDC authorization codes
type OIDCAuthCodeData struct {
	Code         string
	UserID       string
	Scopes       []string
	State        string
	Nonce        string
	ExpiresAt    time.Time
	RedirectURI  string
	CodeVerifier string // For PKCE
}

// OIDCTokenData stores information about OIDC tokens
type OIDCTokenData struct {
	AccessToken  string
	RefreshToken string
	IDToken      string
	UserID       string
	Scopes       []string
	ExpiresAt    time.Time
}

// MockOIDCServer provides a base mock OIDC server
type MockOIDCServer struct {
	server        *http.Server
	port          int
	mutex         sync.RWMutex
	privateKey    *rsa.PrivateKey
	authCodes     map[string]*OIDCAuthCodeData
	accessTokens  map[string]*OIDCTokenData
	users         map[string]*OIDCUserInfo
	clientID      string
	clientSecret  string
	issuer        string
	authorizeFunc func(userID string) (string, error)

	// Response overrides let a test drive the failure paths a well-behaved provider never exercises:
	// a bad or missing ID token, a token or userinfo endpoint that errors or returns nonsense, and a
	// JWKS document that cannot be used. Each is nil by default, leaving normal behaviour untouched.
	idTokenOverride  func(user *OIDCUserInfo, nonce string) (string, error)
	tokenOverride    func() (int, string)
	userInfoOverride func() (int, string)
	jwksOverride     func() (int, string)

	// Configurable endpoints
	discoveryPath string
	authorizePath string
	tokenPath     string
	userInfoPath  string
	jwksPath      string
}

// NewMockOIDCServer creates a new mock OIDC server
func NewMockOIDCServer(port int, clientID, clientSecret string) (*MockOIDCServer, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA key: %w", err)
	}

	issuer := fmt.Sprintf("http://localhost:%d", port)

	return &MockOIDCServer{
		port:          port,
		privateKey:    privateKey,
		authCodes:     make(map[string]*OIDCAuthCodeData),
		accessTokens:  make(map[string]*OIDCTokenData),
		users:         make(map[string]*OIDCUserInfo),
		clientID:      clientID,
		clientSecret:  clientSecret,
		issuer:        issuer,
		discoveryPath: "/.well-known/openid-configuration",
		authorizePath: "/authorize",
		tokenPath:     "/token",
		userInfoPath:  "/userinfo",
		jwksPath:      "/jwks",
	}, nil
}

// Start starts the mock OIDC server
func (m *MockOIDCServer) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc(m.discoveryPath, m.handleDiscovery)
	mux.HandleFunc(m.authorizePath, m.handleAuthorize)
	mux.HandleFunc(m.tokenPath, m.handleToken)
	mux.HandleFunc(m.userInfoPath, m.handleUserInfo)
	mux.HandleFunc(m.jwksPath, m.handleJWKS)

	m.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", m.port),
		Handler: mux,
	}

	go func() {
		if err := m.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Mock OIDC server error: %v\n", err)
		}
	}()

	return nil
}

// Stop stops the mock OIDC server
func (m *MockOIDCServer) Stop() error {
	if m.server != nil {
		return m.server.Close()
	}
	return nil
}

// GetURL returns the base URL
func (m *MockOIDCServer) GetURL() string {
	return m.issuer
}

// GetDiscoveryURL returns the discovery endpoint URL
func (m *MockOIDCServer) GetDiscoveryURL() string {
	return m.issuer + m.discoveryPath
}

// GetAuthorizeURL returns the authorization endpoint URL
func (m *MockOIDCServer) GetAuthorizeURL() string {
	return m.issuer + m.authorizePath
}

// GetTokenURL returns the token endpoint URL
func (m *MockOIDCServer) GetTokenURL() string {
	return m.issuer + m.tokenPath
}

// GetUserInfoURL returns the userinfo endpoint URL
func (m *MockOIDCServer) GetUserInfoURL() string {
	return m.issuer + m.userInfoPath
}

// GetJWKSURL returns the JWKS endpoint URL
func (m *MockOIDCServer) GetJWKSURL() string {
	return m.issuer + m.jwksPath
}

// SetAuthorizeFunc sets a custom authorization function
func (m *MockOIDCServer) SetAuthorizeFunc(fn func(userID string) (string, error)) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.authorizeFunc = fn
}

// SetIDTokenOverride replaces the ID token the token endpoint issues. The nonce the server issued is
// passed in, because it is validated independently of the signature: a token omitting it is rejected
// for that reason alone, which would make a scenario about anything else pass for the wrong cause.
// SetIDTokenOverride replaces the ID token the token endpoint issues. Returning an error makes the
// endpoint fail; returning a string serves it verbatim, so a test can mint an expired, unsigned or
// structurally broken token with SignJWT and have the server hand it over.
func (m *MockOIDCServer) SetIDTokenOverride(fn func(user *OIDCUserInfo, nonce string) (string, error)) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.idTokenOverride = fn
}

// SetTokenResponseOverride replaces the whole token response with the given status and body.
func (m *MockOIDCServer) SetTokenResponseOverride(fn func() (int, string)) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.tokenOverride = fn
}

// SetUserInfoResponseOverride replaces the whole userinfo response with the given status and body.
func (m *MockOIDCServer) SetUserInfoResponseOverride(fn func() (int, string)) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.userInfoOverride = fn
}

// SetJWKSOverride replaces the JWKS document, so a test can serve a malformed key set, one missing the
// signing key, or a rotated one.
func (m *MockOIDCServer) SetJWKSOverride(fn func() (int, string)) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.jwksOverride = fn
}

// ClearOverrides restores normal behaviour on every endpoint. A suite shares one mock across its tests,
// so this runs between them rather than leaving one scenario's injected failure to break the next.
func (m *MockOIDCServer) ClearOverrides() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.idTokenOverride = nil
	m.tokenOverride = nil
	m.userInfoOverride = nil
	m.jwksOverride = nil
}

// writeOIDCOverride serves an override response and reports whether it handled the request.
func writeOIDCOverride(w http.ResponseWriter, fn func() (int, string)) bool {
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
func (m *MockOIDCServer) AddUser(user *OIDCUserInfo) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.users[user.Sub] = user
}

// SignJWT signs an arbitrary JWT with the server's RSA private key. Header
// defaults: alg=RS256, kid=oidc-key-1 (matching the JWKS endpoint).
func (m *MockOIDCServer) SignJWT(header, claims map[string]interface{}) (string, error) {
	if _, ok := header["alg"]; !ok {
		header["alg"] = "RS256"
	}
	if _, ok := header["kid"]; !ok {
		header["kid"] = "oidc-key-1"
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	headerEncoded := base64.RawURLEncoding.EncodeToString(headerJSON)

	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payloadEncoded := base64.RawURLEncoding.EncodeToString(payloadJSON)

	signingInput := headerEncoded + "." + payloadEncoded

	hashed := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, m.privateKey, crypto.SHA256, hashed[:])
	if err != nil {
		return "", err
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

// handleDiscovery handles OIDC discovery endpoint
func (m *MockOIDCServer) handleDiscovery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	doc := OIDCDiscoveryDocument{
		Issuer:                m.issuer,
		AuthorizationEndpoint: m.issuer + m.authorizePath,
		TokenEndpoint:         m.issuer + m.tokenPath,
		UserinfoEndpoint:      m.issuer + m.userInfoPath,
		JwksURI:               m.issuer + m.jwksPath,
		ResponseTypesSupported: []string{
			"code", "token", "id_token", "code token", "code id_token", "token id_token",
			"code token id_token",
		},
		SubjectTypesSupported:             []string{"public"},
		IDTokenSigningAlgValuesSupported:  []string{"RS256"},
		ScopesSupported:                   []string{"openid", "email", "profile"},
		TokenEndpointAuthMethodsSupported: []string{"client_secret_post", "client_secret_basic"},
		ClaimsSupported: []string{
			"sub", "email", "email_verified", "name", "given_name", "family_name", "picture",
			"locale",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(doc)
}

// handleAuthorize handles authorization endpoint
func (m *MockOIDCServer) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query()
	clientID := query.Get("client_id")
	redirectURI := query.Get("redirect_uri")
	state := query.Get("state")
	scope := query.Get("scope")
	nonce := query.Get("nonce")
	responseType := query.Get("response_type")

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
			m.AddUser(&OIDCUserInfo{
				Sub:           userID,
				Email:         "user@example.com",
				EmailVerified: true,
				Name:          "Test User",
				GivenName:     "Test",
				FamilyName:    "User",
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
	code := generateOIDCRandomString(32)
	m.mutex.Lock()
	m.authCodes[code] = &OIDCAuthCodeData{
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

// handleToken handles token endpoint
func (m *MockOIDCServer) handleToken(w http.ResponseWriter, r *http.Request) {
	m.mutex.RLock()
	override := m.tokenOverride
	m.mutex.RUnlock()
	if writeOIDCOverride(w, override) {
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
		writeOIDCTokenError(w, "unsupported_grant_type", "Grant type not supported")
		return
	}

	// Validate client credentials
	if clientID != m.clientID || clientSecret != m.clientSecret {
		writeOIDCTokenError(w, "invalid_client", "Invalid client credentials")
		return
	}

	// Validate authorization code
	m.mutex.Lock()
	authCodeData, exists := m.authCodes[code]
	if !exists {
		m.mutex.Unlock()
		writeOIDCTokenError(w, "invalid_grant", "Invalid authorization code")
		return
	}

	// Check expiration
	if time.Now().After(authCodeData.ExpiresAt) {
		delete(m.authCodes, code)
		m.mutex.Unlock()
		writeOIDCTokenError(w, "invalid_grant", "Authorization code expired")
		return
	}

	// Validate redirect URI
	if authCodeData.RedirectURI != redirectURI {
		m.mutex.Unlock()
		writeOIDCTokenError(w, "invalid_grant", "Redirect URI mismatch")
		return
	}

	// Get user
	user, exists := m.users[authCodeData.UserID]
	if !exists {
		m.mutex.Unlock()
		writeOIDCTokenError(w, "invalid_grant", "User not found")
		return
	}

	// Delete used code
	delete(m.authCodes, code)
	m.mutex.Unlock()

	// Generate tokens
	accessToken := "oidc_" + generateOIDCRandomString(40)
	refreshToken := "refresh_" + generateOIDCRandomString(40)

	// Generate ID token
	m.mutex.RLock()
	idTokenOverride := m.idTokenOverride
	m.mutex.RUnlock()

	var idToken string
	var err error
	if idTokenOverride != nil {
		idToken, err = idTokenOverride(user, authCodeData.Nonce)
	} else {
		idToken, err = m.generateIDToken(user, authCodeData.Nonce, accessToken)
	}
	if err != nil {
		http.Error(w, "Failed to generate ID token", http.StatusInternalServerError)
		return
	}

	// Store token
	m.mutex.Lock()
	m.accessTokens[accessToken] = &OIDCTokenData{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		IDToken:      idToken,
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
		"id_token":      idToken,
		"scope":         strings.Join(authCodeData.Scopes, " "),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleUserInfo handles userinfo endpoint
func (m *MockOIDCServer) handleUserInfo(w http.ResponseWriter, r *http.Request) {
	m.mutex.RLock()
	override := m.userInfoOverride
	m.mutex.RUnlock()
	if writeOIDCOverride(w, override) {
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

	// Build response based on scopes
	response := map[string]interface{}{
		"sub": user.Sub,
	}

	if containsOIDC(tokenData.Scopes, "email") {
		if user.Email != "" {
			response["email"] = user.Email
			response["email_verified"] = user.EmailVerified
		}
	}

	if containsOIDC(tokenData.Scopes, "profile") {
		if user.Name != "" {
			response["name"] = user.Name
		}
		if user.GivenName != "" {
			response["given_name"] = user.GivenName
		}
		if user.FamilyName != "" {
			response["family_name"] = user.FamilyName
		}
		if user.Picture != "" {
			response["picture"] = user.Picture
		}
		if user.Locale != "" {
			response["locale"] = user.Locale
		}
	}

	// Add custom fields
	for k, v := range user.Custom {
		response[k] = v
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleJWKS handles JWKS endpoint
func (m *MockOIDCServer) handleJWKS(w http.ResponseWriter, r *http.Request) {
	m.mutex.RLock()
	override := m.jwksOverride
	m.mutex.RUnlock()
	if writeOIDCOverride(w, override) {
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	publicKey := &m.privateKey.PublicKey

	jwks := OIDCJWKS{
		Keys: []OIDCJWK{
			{
				Kty: "RSA",
				Use: "sig",
				Kid: "oidc-key-1",
				Alg: "RS256",
				N:   base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes()),
				E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(publicKey.E)).Bytes()),
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(jwks)
}

// generateIDToken generates a signed ID token
func (m *MockOIDCServer) generateIDToken(user *OIDCUserInfo, nonce, accessToken string) (string, error) {
	now := time.Now()
	claims := map[string]interface{}{
		"iss": m.issuer,
		"sub": user.Sub,
		"aud": m.clientID,
		"exp": now.Add(1 * time.Hour).Unix(),
		"iat": now.Unix(),
		"nbf": now.Unix(), // Add nbf (not before) claim required by server's JWT validation
	}

	if user.Email != "" {
		claims["email"] = user.Email
		claims["email_verified"] = user.EmailVerified
	}
	if user.Name != "" {
		claims["name"] = user.Name
	}
	if user.GivenName != "" {
		claims["given_name"] = user.GivenName
	}
	if user.FamilyName != "" {
		claims["family_name"] = user.FamilyName
	}
	if user.Picture != "" {
		claims["picture"] = user.Picture
	}
	if user.Locale != "" {
		claims["locale"] = user.Locale
	}
	if nonce != "" {
		claims["nonce"] = nonce
	}
	if accessToken != "" {
		claims["at_hash"] = generateOIDCRandomString(16)
	}

	// Add custom claims
	for k, v := range user.Custom {
		claims[k] = v
	}

	// Create JWT header
	header := map[string]interface{}{
		"alg": "RS256",
		"typ": "JWT",
		"kid": "oidc-key-1",
	}

	// Encode header
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	headerEncoded := base64.RawURLEncoding.EncodeToString(headerJSON)

	// Encode payload
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payloadEncoded := base64.RawURLEncoding.EncodeToString(payloadJSON)

	// Create signing input
	signingInput := headerEncoded + "." + payloadEncoded

	// Sign with RSA private key
	hashed := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, m.privateKey, crypto.SHA256, hashed[:])
	if err != nil {
		return "", err
	}

	signatureEncoded := base64.RawURLEncoding.EncodeToString(signature)

	return signingInput + "." + signatureEncoded, nil
}

// writeOIDCTokenError writes a token error response
func writeOIDCTokenError(w http.ResponseWriter, errorCode, errorDescription string) {
	response := map[string]string{
		"error":             errorCode,
		"error_description": errorDescription,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(response)
}

// generateOIDCRandomString generates a random string
func generateOIDCRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

// containsOIDC checks if a string slice contains a value
func containsOIDC(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
