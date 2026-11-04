// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mockservices

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

var errUnknownOperation = errors.New("unknown control operation")

// Control operations. A hook operation registers a URL in the test process that the instance calls
// to run the function the test set, and an empty URL clears it.
const (
	OpAddUser         = "add-user"
	OpHook            = "hook"
	OpClearOverrides  = "clear-overrides"
	OpSignJWT         = "sign-jwt"
	OpSetStatus       = "set-status"
	OpMessages        = "messages"
	OpLastMessage     = "last-message"
	OpClearMessages   = "clear-messages"
	OpRequests        = "requests"
	OpClearRequests   = "clear-requests"
	HookAuthorize     = "authorize"
	HookIDToken       = "id-token"
	HookTokenResponse = "token-response"
	HookUserInfo      = "userinfo-response"
	HookJWKSResponse  = "jwks-response"
)

// OIDCUserRequest carries an OIDC or OAuth user with the custom claims its JSON form leaves out.
type OIDCUserRequest struct {
	User   *OIDCUserInfo          `json:"user"`
	Custom map[string]interface{} `json:"custom,omitempty"`
}

// OAuthUserRequest carries an OAuth user with the custom fields its JSON form leaves out.
type OAuthUserRequest struct {
	User   *OAuthUserInfo         `json:"user"`
	Custom map[string]interface{} `json:"custom,omitempty"`
}

// GithubUserRequest carries a GitHub user and the email addresses the API lists for them.
type GithubUserRequest struct {
	User   *GithubUserInfo `json:"user"`
	Emails []*GithubEmail  `json:"emails"`
}

// HookRequest names a hook and the URL that runs it.
type HookRequest struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// IDTokenHookArgs is what an ID token hook is called with.
type IDTokenHookArgs struct {
	User   *OIDCUserInfo          `json:"user"`
	Custom map[string]interface{} `json:"custom,omitempty"`
	Nonce  string                 `json:"nonce"`
}

// ResponseOverride is a status and body a hook makes an endpoint answer with.
type ResponseOverride struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}

// HookReply is what a hook returns: a value, or the error the function returned.
type HookReply struct {
	Value json.RawMessage `json:"value,omitempty"`
	Error string          `json:"error,omitempty"`
}

// SignJWTRequest is a token for an OIDC instance to sign with its key.
type SignJWTRequest struct {
	Header map[string]interface{} `json:"header"`
	Claims map[string]interface{} `json:"claims"`
}

// StatusRequest sets the status a notification instance answers with.
type StatusRequest struct {
	Status int `json:"status"`
}

func (m *OIDCServer) control(op string, body []byte) (interface{}, error) {
	switch op {
	case OpAddUser:
		var req OIDCUserRequest
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		req.User.Custom = req.Custom
		m.AddUser(req.User)
		return nil, nil
	case OpHook:
		var req HookRequest
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		switch req.Name {
		case HookAuthorize:
			m.SetAuthorizeFunc(authorizeHook(req.URL))
		case HookIDToken:
			m.SetIDTokenOverride(idTokenHook(req.URL))
		case HookTokenResponse:
			m.SetTokenResponseOverride(responseHook(req.URL))
		case HookUserInfo:
			m.SetUserInfoResponseOverride(responseHook(req.URL))
		case HookJWKSResponse:
			m.SetJWKSOverride(responseHook(req.URL))
		default:
			return nil, fmt.Errorf("%w: hook %s", errUnknownOperation, req.Name)
		}
		return nil, nil
	case OpClearOverrides:
		m.ClearOverrides()
		return nil, nil
	case OpSignJWT:
		var req SignJWTRequest
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		if req.Header == nil {
			req.Header = map[string]interface{}{}
		}
		return m.SignJWT(req.Header, req.Claims)
	}
	return nil, fmt.Errorf("%w: %s", errUnknownOperation, op)
}

func (m *OAuthServer) control(op string, body []byte) (interface{}, error) {
	switch op {
	case OpAddUser:
		var req OAuthUserRequest
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		req.User.Custom = req.Custom
		m.AddUser(req.User)
		return nil, nil
	case OpHook:
		var req HookRequest
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		switch req.Name {
		case HookAuthorize:
			m.SetAuthorizeFunc(authorizeHook(req.URL))
		case HookTokenResponse:
			m.SetTokenResponseOverride(responseHook(req.URL))
		case HookUserInfo:
			m.SetUserInfoResponseOverride(responseHook(req.URL))
		default:
			return nil, fmt.Errorf("%w: hook %s", errUnknownOperation, req.Name)
		}
		return nil, nil
	case OpClearOverrides:
		m.ClearOverrides()
		return nil, nil
	}
	return nil, fmt.Errorf("%w: %s", errUnknownOperation, op)
}

func (m *GoogleOIDCServer) control(op string, body []byte) (interface{}, error) {
	switch op {
	case OpAddUser:
		var user GoogleUserInfo
		if err := json.Unmarshal(body, &user); err != nil {
			return nil, err
		}
		m.AddUser(&user)
		return nil, nil
	case OpHook:
		var req HookRequest
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		if req.Name != HookAuthorize {
			return nil, fmt.Errorf("%w: hook %s", errUnknownOperation, req.Name)
		}
		m.SetAuthorizeFunc(authorizeHook(req.URL))
		return nil, nil
	}
	return nil, fmt.Errorf("%w: %s", errUnknownOperation, op)
}

func (m *GithubOAuthServer) control(op string, body []byte) (interface{}, error) {
	switch op {
	case OpAddUser:
		var req GithubUserRequest
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		m.AddUser(req.User, req.Emails)
		return nil, nil
	case OpHook:
		var req HookRequest
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		if req.Name != HookAuthorize {
			return nil, fmt.Errorf("%w: hook %s", errUnknownOperation, req.Name)
		}
		m.SetAuthorizeFunc(authorizeHook(req.URL))
		return nil, nil
	}
	return nil, fmt.Errorf("%w: %s", errUnknownOperation, op)
}

func (m *NotificationServer) control(op string, body []byte) (interface{}, error) {
	switch op {
	case OpSetStatus:
		var req StatusRequest
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		m.SetResponseStatus(req.Status)
		return nil, nil
	case OpMessages:
		return m.GetMessages(), nil
	case OpLastMessage:
		return m.GetLastMessage(), nil
	case OpClearMessages:
		m.ClearMessages()
		return nil, nil
	}
	return nil, fmt.Errorf("%w: %s", errUnknownOperation, op)
}

func (m *HTTPServer) control(op string, _ []byte) (interface{}, error) {
	switch op {
	case OpRequests:
		return m.GetCapturedRequests(), nil
	case OpClearRequests:
		m.ClearRequests()
		return nil, nil
	}
	return nil, fmt.Errorf("%w: %s", errUnknownOperation, op)
}

func (m *GoogleOIDCServer) client() string { return m.clientID }

// owns reports whether a request at the Google root belongs to this instance: by its client, by
// an access token it issued, or for the discovery document, keys and token info every instance
// serves alike.
func (m *GoogleOIDCServer) owns(r *http.Request) bool {
	switch r.URL.Path {
	case "/.well-known/openid-configuration", "/oauth2/v3/certs", "/oauth2/v3/tokeninfo":
		return true
	}
	if requestClientID(r) == m.clientID {
		return true
	}
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	_, ok := m.accessTokens[bearerToken(r)]
	return ok
}

func (m *GithubOAuthServer) client() string { return m.clientID }

// owns reports whether a request at the GitHub root belongs to this instance: by its client,
// including one named in an /applications/{client_id} path, or by an access token it issued.
func (m *GithubOAuthServer) owns(r *http.Request) bool {
	if requestClientID(r) == m.clientID || strings.HasPrefix(r.URL.Path, "/applications/"+m.clientID+"/") {
		return true
	}
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	_, ok := m.accessTokens[bearerToken(r)]
	return ok
}

// requestClientID returns the client a request names in its query, form or basic credentials.
func requestClientID(r *http.Request) string {
	if id := r.URL.Query().Get("client_id"); id != "" {
		return id
	}
	if user, _, ok := r.BasicAuth(); ok {
		return user
	}
	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		return r.PostFormValue("client_id")
	}
	return ""
}

// bearerToken returns the access token a request carries in its Authorization header or query.
func bearerToken(r *http.Request) string {
	if header := r.Header.Get("Authorization"); header != "" {
		if _, token, ok := strings.Cut(header, " "); ok {
			return token
		}
	}
	return r.URL.Query().Get("access_token")
}

// authorizeHook returns the authorize function a hook URL runs, or nil to clear it.
func authorizeHook(url string) func(string) (string, error) {
	if url == "" {
		return nil
	}
	return func(string) (string, error) {
		var subject string
		err := callHook(url, nil, &subject)
		return subject, err
	}
}

// idTokenHook returns the ID token function a hook URL runs, or nil to clear it.
func idTokenHook(url string) func(*OIDCUserInfo, string) (string, error) {
	if url == "" {
		return nil
	}
	return func(user *OIDCUserInfo, nonce string) (string, error) {
		var token string
		err := callHook(url, IDTokenHookArgs{User: user, Custom: user.Custom, Nonce: nonce}, &token)
		return token, err
	}
}

// responseHook returns the response override a hook URL runs, or nil to clear it.
func responseHook(url string) func() (int, string) {
	if url == "" {
		return nil
	}
	return func() (int, string) {
		var override ResponseOverride
		if err := callHook(url, nil, &override); err != nil {
			return http.StatusInternalServerError, fmt.Sprintf(`{"error":%q}`, err.Error())
		}
		return override.Status, override.Body
	}
}

// callHook calls the test process back to run a hook, and decodes the value it returns.
func callHook(url string, args interface{}, value interface{}) error {
	payload, err := json.Marshal(args)
	if err != nil {
		return err
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to call the test hook: %w", err)
	}
	defer resp.Body.Close()

	var reply HookReply
	if err := json.NewDecoder(resp.Body).Decode(&reply); err != nil {
		return fmt.Errorf("failed to read the test hook reply: %w", err)
	}
	if reply.Error != "" {
		return errors.New(reply.Error)
	}
	return json.Unmarshal(reply.Value, value)
}
