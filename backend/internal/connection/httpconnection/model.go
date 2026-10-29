// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package httpconnection

import (
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
)

// Header is a single HTTP header name and value.
type Header struct {
	Name  string `json:"name"  yaml:"name"`
	Value string `json:"value" yaml:"value"`
}

// HTTPConnection is an HTTP endpoint connection. AuthProperties holds its outbound authentication
// in the outboundauth property form, with secret values encrypted. IsReadOnly marks a connection
// defined in a declarative file.
type HTTPConnection struct {
	ID          string
	Name        string
	Description string
	OUID        string
	// OUHandle names the owner in an import or declarative document; it is resolved to OUID.
	OUHandle       string
	IsReadOnly     bool
	URL            string
	TimeoutMS      int
	Headers        []Header
	AuthProperties []cmodels.Property
}

// ResolvedHTTPConnection is a connection ready to call: its URL, its timeout, and its headers with
// the authentication applied. Header values can carry a decrypted credential, so a resolved
// connection is built for one call and never stored or logged.
type ResolvedHTTPConnection struct {
	URL     string
	Timeout time.Duration
	Headers http.Header
}
