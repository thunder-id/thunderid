// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"encoding/json"

	"github.com/thunder-id/thunderid/tests/integration/mockservices"
)

// The mock services' data types, shared with the services the test runner hosts.
type (
	OIDCUserInfo   = mockservices.OIDCUserInfo
	OAuthUserInfo  = mockservices.OAuthUserInfo
	GoogleUserInfo = mockservices.GoogleUserInfo
	GithubUserInfo = mockservices.GithubUserInfo
	GithubEmail    = mockservices.GithubEmail
	SMSMessage     = mockservices.SMSMessage
	HTTPRequest    = mockservices.HTTPRequest
)

// MockOIDCServer is a suite's own OIDC identity provider on the shared OIDC mock service.
type MockOIDCServer struct {
	*mockInstance
}

// NewMockOIDCServer creates an OIDC identity provider for the given client.
func NewMockOIDCServer(clientID, clientSecret string) (*MockOIDCServer, error) {
	instance, err := newMockInstance(mockservices.KindOIDC, clientID, clientSecret)
	if err != nil {
		return nil, err
	}
	return &MockOIDCServer{instance}, nil
}

// Stop removes the identity provider.
func (m *MockOIDCServer) Stop() error { return m.remove() }

// GetURL returns the issuer and base URL.
func (m *MockOIDCServer) GetURL() string { return m.url }

// GetDiscoveryURL returns the discovery endpoint URL.
func (m *MockOIDCServer) GetDiscoveryURL() string { return m.url + "/.well-known/openid-configuration" }

// GetAuthorizeURL returns the authorization endpoint URL.
func (m *MockOIDCServer) GetAuthorizeURL() string { return m.url + "/authorize" }

// GetTokenURL returns the token endpoint URL.
func (m *MockOIDCServer) GetTokenURL() string { return m.url + "/token" }

// GetUserInfoURL returns the userinfo endpoint URL.
func (m *MockOIDCServer) GetUserInfoURL() string { return m.url + "/userinfo" }

// GetJWKSURL returns the JWKS endpoint URL.
func (m *MockOIDCServer) GetJWKSURL() string { return m.url + "/jwks" }

// AddUser adds a user the provider can sign in.
func (m *MockOIDCServer) AddUser(user *OIDCUserInfo) {
	m.mustCall(mockservices.OpAddUser, mockservices.OIDCUserRequest{User: user, Custom: user.Custom}, nil)
}

// SetAuthorizeFunc sets the function that picks the user the authorization endpoint signs in.
func (m *MockOIDCServer) SetAuthorizeFunc(fn func(userID string) (string, error)) {
	m.setHook(mockservices.HookAuthorize, authorizeHook(fn))
}

// SetIDTokenOverride replaces the ID token the token endpoint issues. The nonce the server issued is
// passed in, because it is validated independently of the signature: a token omitting it is rejected
// for that reason alone, which would make a scenario about anything else pass for the wrong cause.
// Returning an error makes the endpoint fail; returning a string serves it verbatim, so a test can
// mint an expired, unsigned or structurally broken token with SignJWT and have the server hand it over.
func (m *MockOIDCServer) SetIDTokenOverride(fn func(user *OIDCUserInfo, nonce string) (string, error)) {
	var hook func(json.RawMessage) (interface{}, error)
	if fn != nil {
		hook = func(raw json.RawMessage) (interface{}, error) {
			var args mockservices.IDTokenHookArgs
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, err
			}
			args.User.Custom = args.Custom
			return fn(args.User, args.Nonce)
		}
	}
	m.setHook(mockservices.HookIDToken, hook)
}

// SetTokenResponseOverride replaces the whole token response with the given status and body.
func (m *MockOIDCServer) SetTokenResponseOverride(fn func() (int, string)) {
	m.setHook(mockservices.HookTokenResponse, responseHook(fn))
}

// SetUserInfoResponseOverride replaces the whole userinfo response with the given status and body.
func (m *MockOIDCServer) SetUserInfoResponseOverride(fn func() (int, string)) {
	m.setHook(mockservices.HookUserInfo, responseHook(fn))
}

// SetJWKSOverride replaces the JWKS document, so a test can serve a malformed key set, one missing the
// signing key, or a rotated one.
func (m *MockOIDCServer) SetJWKSOverride(fn func() (int, string)) {
	m.setHook(mockservices.HookJWKSResponse, responseHook(fn))
}

// ClearOverrides restores normal behaviour on every endpoint. A suite shares one mock across its tests,
// so this runs between them rather than leaving one scenario's injected failure to break the next.
func (m *MockOIDCServer) ClearOverrides() {
	m.mustCall(mockservices.OpClearOverrides, nil, nil)
}

// SignJWT signs an arbitrary JWT with the provider's RSA private key. Header
// defaults: alg=RS256, kid=oidc-key-1 (matching the JWKS endpoint).
func (m *MockOIDCServer) SignJWT(header, claims map[string]interface{}) (string, error) {
	var token string
	err := m.call(mockservices.OpSignJWT, mockservices.SignJWTRequest{Header: header, Claims: claims}, &token)
	return token, err
}

// MockOAuthServer is a suite's own OAuth 2.0 provider on the shared OAuth mock service.
type MockOAuthServer struct {
	*mockInstance
}

// NewMockOAuthServer creates an OAuth 2.0 provider for the given client.
func NewMockOAuthServer(clientID, clientSecret string) (*MockOAuthServer, error) {
	instance, err := newMockInstance(mockservices.KindOAuth, clientID, clientSecret)
	if err != nil {
		return nil, err
	}
	return &MockOAuthServer{instance}, nil
}

// Stop removes the provider.
func (m *MockOAuthServer) Stop() error { return m.remove() }

// GetURL returns the base URL.
func (m *MockOAuthServer) GetURL() string { return m.url }

// GetAuthorizeURL returns the authorization endpoint URL.
func (m *MockOAuthServer) GetAuthorizeURL() string { return m.url + "/oauth/authorize" }

// GetTokenURL returns the token endpoint URL.
func (m *MockOAuthServer) GetTokenURL() string { return m.url + "/oauth/token" }

// GetUserInfoURL returns the userinfo endpoint URL.
func (m *MockOAuthServer) GetUserInfoURL() string { return m.url + "/oauth/userinfo" }

// AddUser adds a user the provider can sign in.
func (m *MockOAuthServer) AddUser(user *OAuthUserInfo) {
	m.mustCall(mockservices.OpAddUser, mockservices.OAuthUserRequest{User: user, Custom: user.Custom}, nil)
}

// SetAuthorizeFunc sets the function that picks the user the authorization endpoint signs in.
func (m *MockOAuthServer) SetAuthorizeFunc(fn func(userID string) (string, error)) {
	m.setHook(mockservices.HookAuthorize, authorizeHook(fn))
}

// SetTokenResponseOverride replaces the whole token response with the given status and body.
func (m *MockOAuthServer) SetTokenResponseOverride(fn func() (int, string)) {
	m.setHook(mockservices.HookTokenResponse, responseHook(fn))
}

// SetUserInfoResponseOverride replaces the whole userinfo response with the given status and body.
func (m *MockOAuthServer) SetUserInfoResponseOverride(fn func() (int, string)) {
	m.setHook(mockservices.HookUserInfo, responseHook(fn))
}

// ClearOverrides restores normal behaviour on every endpoint.
func (m *MockOAuthServer) ClearOverrides() {
	m.mustCall(mockservices.OpClearOverrides, nil, nil)
}

// MockGoogleOIDCServer is a suite's own Google client on the shared Google mock service. The server
// reaches Google at a fixed base URL, so the service tells suites apart by client id, and each suite
// needs a client id of its own.
type MockGoogleOIDCServer struct {
	*mockInstance
}

// NewMockGoogleOIDCServer creates the Google client.
func NewMockGoogleOIDCServer(clientID, clientSecret string) (*MockGoogleOIDCServer, error) {
	instance, err := newMockInstance(mockservices.KindGoogle, clientID, clientSecret)
	if err != nil {
		return nil, err
	}
	return &MockGoogleOIDCServer{instance}, nil
}

// Stop removes the Google client.
func (m *MockGoogleOIDCServer) Stop() error { return m.remove() }

// GetURL returns the base URL of the Google mock.
func (m *MockGoogleOIDCServer) GetURL() string { return m.url }

// SetAuthorizeFunc sets the function that picks the email of the user Google signs in.
func (m *MockGoogleOIDCServer) SetAuthorizeFunc(fn func(email string) (string, error)) {
	m.setHook(mockservices.HookAuthorize, authorizeHook(fn))
}

// AddUser adds a user Google can sign in.
func (m *MockGoogleOIDCServer) AddUser(user *GoogleUserInfo) {
	m.mustCall(mockservices.OpAddUser, user, nil)
}

// MockGithubOAuthServer is a suite's own GitHub OAuth app on the shared GitHub mock service. The
// server reaches GitHub at a fixed base URL, so the service tells suites apart by client id, and
// each suite needs a client id of its own.
type MockGithubOAuthServer struct {
	*mockInstance
}

// NewMockGithubOAuthServer creates the GitHub OAuth app.
func NewMockGithubOAuthServer(clientID, clientSecret string) (*MockGithubOAuthServer, error) {
	instance, err := newMockInstance(mockservices.KindGitHub, clientID, clientSecret)
	if err != nil {
		return nil, err
	}
	return &MockGithubOAuthServer{instance}, nil
}

// Stop removes the GitHub OAuth app.
func (m *MockGithubOAuthServer) Stop() error { return m.remove() }

// GetURL returns the base URL of the GitHub mock.
func (m *MockGithubOAuthServer) GetURL() string { return m.url }

// GetAuthorizeURL returns the authorization endpoint URL.
func (m *MockGithubOAuthServer) GetAuthorizeURL() string { return m.url + "/login/oauth/authorize" }

// GetAccessTokenURL returns the access token endpoint URL.
func (m *MockGithubOAuthServer) GetAccessTokenURL() string {
	return m.url + "/login/oauth/access_token"
}

// GetAPIURL returns the API base URL.
func (m *MockGithubOAuthServer) GetAPIURL() string { return m.url }

// SetAuthorizeFunc sets the function that picks the login of the user GitHub signs in.
func (m *MockGithubOAuthServer) SetAuthorizeFunc(fn func(login string) (string, error)) {
	m.setHook(mockservices.HookAuthorize, authorizeHook(fn))
}

// AddUser adds a user GitHub can sign in, with the email addresses its API lists.
func (m *MockGithubOAuthServer) AddUser(user *GithubUserInfo, emails []*GithubEmail) {
	m.mustCall(mockservices.OpAddUser, mockservices.GithubUserRequest{User: user, Emails: emails}, nil)
}

// MockNotificationServer is a suite's own SMS gateway on the shared notification mock service.
type MockNotificationServer struct {
	*mockInstance
}

// NewMockNotificationServer creates the SMS gateway.
func NewMockNotificationServer() (*MockNotificationServer, error) {
	instance, err := newMockInstance(mockservices.KindNotification, "", "")
	if err != nil {
		return nil, err
	}
	return &MockNotificationServer{instance}, nil
}

// Stop removes the SMS gateway.
func (m *MockNotificationServer) Stop() error { return m.remove() }

// GetURL returns the base URL.
func (m *MockNotificationServer) GetURL() string { return m.url }

// GetSendSMSURL returns the SMS sending endpoint URL.
func (m *MockNotificationServer) GetSendSMSURL() string { return m.url + "/send-sms" }

// SetResponseStatus makes the gateway answer with the given status. One of 400 or above fails every
// send; any other restores success.
func (m *MockNotificationServer) SetResponseStatus(status int) {
	m.mustCall(mockservices.OpSetStatus, mockservices.StatusRequest{Status: status}, nil)
}

// GetLastMessage removes and returns the last message received, or nil if there is none.
func (m *MockNotificationServer) GetLastMessage() *SMSMessage {
	var message *SMSMessage
	m.mustCall(mockservices.OpLastMessage, nil, &message)
	return message
}

// GetMessages returns every message held, oldest first, without consuming them.
func (m *MockNotificationServer) GetMessages() []SMSMessage {
	var messages []SMSMessage
	m.mustCall(mockservices.OpMessages, nil, &messages)
	return messages
}

// ClearMessages clears all stored messages.
func (m *MockNotificationServer) ClearMessages() {
	m.mustCall(mockservices.OpClearMessages, nil, nil)
}

// MockHTTPServer is a suite's own HTTP endpoint on the shared HTTP mock service, for the HTTP
// request executor to call.
type MockHTTPServer struct {
	*mockInstance
}

// NewMockHTTPServer creates the HTTP endpoint.
func NewMockHTTPServer() (*MockHTTPServer, error) {
	instance, err := newMockInstance(mockservices.KindHTTP, "", "")
	if err != nil {
		return nil, err
	}
	return &MockHTTPServer{instance}, nil
}

// Stop removes the HTTP endpoint.
func (m *MockHTTPServer) Stop() error { return m.remove() }

// GetURL returns the base URL.
func (m *MockHTTPServer) GetURL() string { return m.url }

// GetNotificationsURL returns the notifications endpoint URL.
func (m *MockHTTPServer) GetNotificationsURL() string { return m.url + "/api/notifications" }

// GetUsersURL returns the users endpoint URL.
func (m *MockHTTPServer) GetUsersURL() string { return m.url + "/api/users" }

// GetErrorURL returns the error endpoint URL.
func (m *MockHTTPServer) GetErrorURL() string { return m.url + "/api/error" }

// GetCapturedRequests returns all captured requests.
func (m *MockHTTPServer) GetCapturedRequests() []HTTPRequest {
	var requests []HTTPRequest
	m.mustCall(mockservices.OpRequests, nil, &requests)
	return requests
}

// GetRequestByPath returns the first captured request matching the given path.
func (m *MockHTTPServer) GetRequestByPath(path string) *HTTPRequest {
	for _, req := range m.GetCapturedRequests() {
		if req.Path == path {
			return &req
		}
	}
	return nil
}

// ClearRequests clears all captured requests.
func (m *MockHTTPServer) ClearRequests() {
	m.mustCall(mockservices.OpClearRequests, nil, nil)
}
