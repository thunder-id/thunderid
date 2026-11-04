// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mockservices

import "net/http"

func (m *OIDCServer) ServeHTTP(w http.ResponseWriter, r *http.Request)  { m.handler.ServeHTTP(w, r) }
func (m *OAuthServer) ServeHTTP(w http.ResponseWriter, r *http.Request) { m.handler.ServeHTTP(w, r) }
func (m *GoogleOIDCServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.handler.ServeHTTP(w, r)
}
func (m *GithubOAuthServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.handler.ServeHTTP(w, r)
}
func (m *NotificationServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.handler.ServeHTTP(w, r)
}
func (m *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) { m.handler.ServeHTTP(w, r) }
