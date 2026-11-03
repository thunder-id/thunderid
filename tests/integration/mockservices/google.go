// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mockservices

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// mockGoogleOIDCSharedKeyPEM is a throwaway RSA-2048 private key used by every
// GoogleOIDCServer instance across every integration test package. Real OIDC
// issuers honor the contract that a given `kid` maps to exactly one key forever;
// when they rotate they publish a new `kid` alongside the old one. The test mock
// always advertises the same `kid` ("mock-key-id"), so it must also always advertise
// the same underlying key material across every process that spins up the mock.
// Embedding a single PEM via go:embed keeps the JWKS response byte-identical across
// every mock instance in every test package, so server's process-wide JWKS cache
// stays coherent as suites create and remove instances behind the same base URL.
// Not a secret; must only be used by the test mock.
//
//go:embed testdata/mock_google_oidc_key.pem
var mockGoogleOIDCSharedKeyPEM []byte

var (
	mockGoogleOIDCSharedKeyOnce sync.Once
	mockGoogleOIDCSharedKey     *rsa.PrivateKey
	mockGoogleOIDCSharedKeyErr  error
)

func getMockGoogleOIDCSharedKey() (*rsa.PrivateKey, error) {
	mockGoogleOIDCSharedKeyOnce.Do(func() {
		block, _ := pem.Decode(mockGoogleOIDCSharedKeyPEM)
		if block == nil {
			mockGoogleOIDCSharedKeyErr = fmt.Errorf("failed to decode mock Google OIDC test key PEM")
			return
		}
		mockGoogleOIDCSharedKey, mockGoogleOIDCSharedKeyErr = x509.ParsePKCS1PrivateKey(block.Bytes)
	})
	return mockGoogleOIDCSharedKey, mockGoogleOIDCSharedKeyErr
}

// GoogleOIDCServer provides a mock Google OIDC server for testing
type GoogleOIDCServer struct {
	handler       http.Handler
	baseURL       string
	mutex         sync.RWMutex
	privateKey    *rsa.PrivateKey
	authCodes     map[string]*AuthCodeData
	accessTokens  map[string]*TokenData
	users         map[string]*GoogleUserInfo
	clientID      string
	clientSecret  string
	issuer        string
	authorizeFunc func(email string) (string, error) // Custom authorize function for tests
}

// AuthCodeData stores information about authorization codes
type AuthCodeData struct {
	Code         string
	Email        string
	Scopes       []string
	State        string
	Nonce        string
	ExpiresAt    time.Time
	RedirectURI  string
	CodeVerifier string // For PKCE
}

// TokenData stores information about access tokens
type TokenData struct {
	AccessToken  string
	RefreshToken string
	IDToken      string
	Email        string
	Scopes       []string
	ExpiresAt    time.Time
}

// GoogleUserInfo represents Google user information
type GoogleUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Locale        string `json:"locale"`
	HD            string `json:"hd,omitempty"` // Hosted domain
}

// DiscoveryDocument represents OIDC discovery document
type DiscoveryDocument struct {
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

// JWKS represents JSON Web Key Set
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWK represents a JSON Web Key
type JWK struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// NewGoogleOIDCServer creates a new mock Google OIDC server
func NewGoogleOIDCServer(baseURL, clientID, clientSecret string) (*GoogleOIDCServer, error) {
	privateKey, err := getMockGoogleOIDCSharedKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA key: %w", err)
	}

	// Use Google's actual issuer value for compatibility with Google auth service
	issuer := "accounts.google.com"

	m := &GoogleOIDCServer{
		baseURL:      baseURL,
		privateKey:   privateKey,
		authCodes:    make(map[string]*AuthCodeData),
		accessTokens: make(map[string]*TokenData),
		users:        make(map[string]*GoogleUserInfo),
		clientID:     clientID,
		clientSecret: clientSecret,
		issuer:       issuer,
	}
	m.handler = m.routes()
	return m, nil
}

// routes maps the mock's endpoints.
func (m *GoogleOIDCServer) routes() http.Handler {
	mux := http.NewServeMux()

	// OIDC discovery endpoint
	mux.HandleFunc("/.well-known/openid-configuration", m.handleDiscovery)

	// Authorization endpoint
	mux.HandleFunc("/o/oauth2/v2/auth", m.handleAuthorize)

	// Token endpoint
	mux.HandleFunc("/token", m.handleToken)

	// Userinfo endpoint
	mux.HandleFunc("/v1/userinfo", m.handleUserInfo)

	// JWKS endpoint
	mux.HandleFunc("/oauth2/v3/certs", m.handleJWKS)

	// Token info endpoint (for validation)
	mux.HandleFunc("/oauth2/v3/tokeninfo", m.handleTokenInfo)

	return mux
}

// GetURL returns the base URL of the mock server
func (m *GoogleOIDCServer) GetURL() string {
	return m.baseURL
}

// SetAuthorizeFunc sets a custom authorization function for testing
func (m *GoogleOIDCServer) SetAuthorizeFunc(fn func(email string) (string, error)) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.authorizeFunc = fn
}

// AddUser adds a test user to the mock server
func (m *GoogleOIDCServer) AddUser(user *GoogleUserInfo) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.users[user.Email] = user
}

// handleDiscovery handles OIDC discovery endpoint
func (m *GoogleOIDCServer) handleDiscovery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	baseURL := m.GetURL()
	doc := DiscoveryDocument{
		Issuer:                baseURL,
		AuthorizationEndpoint: fmt.Sprintf("%s/o/oauth2/v2/auth", baseURL),
		TokenEndpoint:         fmt.Sprintf("%s/token", baseURL),
		UserinfoEndpoint:      fmt.Sprintf("%s/v1/userinfo", baseURL),
		JwksURI:               fmt.Sprintf("%s/oauth2/v3/certs", baseURL),
		ResponseTypesSupported: []string{"code", "token", "id_token", "code token", "code id_token",
			"token id_token", "code token id_token"},
		SubjectTypesSupported:             []string{"public"},
		IDTokenSigningAlgValuesSupported:  []string{"RS256"},
		ScopesSupported:                   []string{"openid", "email", "profile"},
		TokenEndpointAuthMethodsSupported: []string{"client_secret_post", "client_secret_basic"},
		ClaimsSupported: []string{"aud", "email", "email_verified", "exp", "family_name",
			"given_name", "iat", "iss", "locale", "name", "picture", "sub"},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(doc)
}

// handleAuthorize handles authorization endpoint
func (m *GoogleOIDCServer) handleAuthorize(w http.ResponseWriter, r *http.Request) {
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

	// Validate response_type
	if responseType != "code" {
		http.Error(w, "Unsupported response_type", http.StatusBadRequest)
		return
	}

	// Parse scopes
	scopes := strings.Split(scope, " ")

	// Use custom authorize function if set
	var email string
	var err error
	if m.authorizeFunc != nil {
		email, err = m.authorizeFunc("")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		// Default: use first user or create a default one
		m.mutex.RLock()
		if len(m.users) == 0 {
			// Create default user
			email = "test@example.com"
			m.mutex.RUnlock()
			m.AddUser(&GoogleUserInfo{
				Sub:           "test-user-id",
				Email:         email,
				EmailVerified: true,
				Name:          "Test User",
				GivenName:     "Test",
				FamilyName:    "User",
				Picture:       "https://example.com/picture.jpg",
				Locale:        "en",
			})
		} else {
			// Use first user
			for _, user := range m.users {
				email = user.Email
				break
			}
			m.mutex.RUnlock()
		}
	}

	// Generate authorization code
	code := generateRandomString(32)
	m.mutex.Lock()
	m.authCodes[code] = &AuthCodeData{
		Code:        code,
		Email:       email,
		Scopes:      scopes,
		State:       state,
		Nonce:       nonce,
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

// handleToken handles token endpoint
func (m *GoogleOIDCServer) handleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
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
		writeTokenError(w, "unsupported_grant_type", "Grant type not supported")
		return
	}

	// Validate client credentials
	if clientID != m.clientID || clientSecret != m.clientSecret {
		writeTokenError(w, "invalid_client", "Invalid client credentials")
		return
	}

	// Validate authorization code
	m.mutex.Lock()
	authCodeData, exists := m.authCodes[code]
	if !exists {
		m.mutex.Unlock()
		writeTokenError(w, "invalid_grant", "Invalid authorization code")
		return
	}

	// Check if code is expired
	if time.Now().After(authCodeData.ExpiresAt) {
		delete(m.authCodes, code)
		m.mutex.Unlock()
		writeTokenError(w, "invalid_grant", "Authorization code expired")
		return
	}

	// Validate redirect URI
	if authCodeData.RedirectURI != redirectURI {
		m.mutex.Unlock()
		writeTokenError(w, "invalid_grant", "Redirect URI mismatch")
		return
	}

	// Get user info
	user, exists := m.users[authCodeData.Email]
	if !exists {
		m.mutex.Unlock()
		writeTokenError(w, "invalid_grant", "User not found")
		return
	}

	// Delete used authorization code
	delete(m.authCodes, code)
	m.mutex.Unlock()

	// Generate tokens
	accessToken := generateRandomString(64)
	refreshToken := generateRandomString(64)

	// Generate ID token
	idToken, err := m.generateIDToken(user, authCodeData.Nonce, accessToken)
	if err != nil {
		log.Printf("Failed to generate ID token: %v", err)
		http.Error(w, "Failed to generate ID token", http.StatusInternalServerError)
		return
	}

	// Store access token
	m.mutex.Lock()
	m.accessTokens[accessToken] = &TokenData{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		IDToken:      idToken,
		Email:        authCodeData.Email,
		Scopes:       authCodeData.Scopes,
		ExpiresAt:    time.Now().Add(1 * time.Hour),
	}
	m.mutex.Unlock()

	// Return token response
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
func (m *GoogleOIDCServer) handleUserInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract access token from Authorization header
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

	// Validate access token
	m.mutex.RLock()
	tokenData, exists := m.accessTokens[accessToken]
	if !exists {
		m.mutex.RUnlock()
		http.Error(w, "Invalid access token", http.StatusUnauthorized)
		return
	}

	// Check if token is expired
	if time.Now().After(tokenData.ExpiresAt) {
		m.mutex.RUnlock()
		http.Error(w, "Access token expired", http.StatusUnauthorized)
		return
	}

	// Get user info
	user, exists := m.users[tokenData.Email]
	m.mutex.RUnlock()
	if !exists {
		http.Error(w, "User not found", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(user)
}

// handleJWKS handles JWKS endpoint
func (m *GoogleOIDCServer) handleJWKS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Generate JWK from public key
	publicKey := &m.privateKey.PublicKey

	jwks := JWKS{
		Keys: []JWK{
			{
				Kty: "RSA",
				Use: "sig",
				Kid: "mock-key-id",
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

// handleTokenInfo handles token info endpoint
func (m *GoogleOIDCServer) handleTokenInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var idToken string
	if r.Method == http.MethodGet {
		idToken = r.URL.Query().Get("id_token")
	} else {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}
		idToken = r.FormValue("id_token")
	}

	if idToken == "" {
		http.Error(w, "Missing id_token", http.StatusBadRequest)
		return
	}

	// Parse JWT manually (simple validation)
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		http.Error(w, "Invalid token format", http.StatusBadRequest)
		return
	}

	// Decode payload
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		http.Error(w, "Invalid token payload", http.StatusBadRequest)
		return
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		http.Error(w, "Invalid token claims", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(claims)
}

// generateIDToken generates a signed ID token
func (m *GoogleOIDCServer) generateIDToken(user *GoogleUserInfo, nonce, accessToken string) (string, error) {
	now := time.Now()
	claims := map[string]interface{}{
		"iss":            m.issuer,
		"sub":            user.Sub,
		"aud":            m.clientID,
		"exp":            now.Add(1 * time.Hour).Unix(),
		"iat":            now.Unix(),
		"email":          user.Email,
		"email_verified": user.EmailVerified,
		"name":           user.Name,
		"given_name":     user.GivenName,
		"family_name":    user.FamilyName,
		"picture":        user.Picture,
		"locale":         user.Locale,
	}

	if nonce != "" {
		claims["nonce"] = nonce
	}

	if user.HD != "" {
		claims["hd"] = user.HD
	}

	// Add at_hash for access token
	if accessToken != "" {
		claims["at_hash"] = generateRandomString(16)
	}

	// Create JWT header
	header := map[string]interface{}{
		"alg": "RS256",
		"typ": "JWT",
		"kid": "mock-key-id",
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

	// Return complete JWT
	return signingInput + "." + signatureEncoded, nil
}

// writeTokenError writes a token error response
func writeTokenError(w http.ResponseWriter, errorCode, errorDescription string) {
	response := map[string]string{
		"error":             errorCode,
		"error_description": errorDescription,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(response)
}

// generateRandomString generates a random string of specified length
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}
