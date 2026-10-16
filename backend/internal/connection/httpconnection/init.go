// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package httpconnection manages HTTP endpoint connections: a named outbound HTTP endpoint with its
// URL, timeout, plain headers and authentication, whose secrets are stored encrypted.
package httpconnection

// Initialize creates the HTTP connection service over the configuration database.
func Initialize() HTTPConnectionServiceInterface {
	return newHTTPConnectionService(newHTTPConnectionStore())
}
