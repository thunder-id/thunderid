// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package httpconnection

import (
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
)

const (
	defaultTimeoutMS = 10000
	maxTimeoutMS     = 20000

	// Bounds of the NAME and DESCRIPTION columns, in characters.
	maxNameLength        = 255
	maxDescriptionLength = 500

	urlSchemeHTTP    = "http"
	urlSchemeHTTPS   = "https"
	defaultPortHTTP  = "80"
	defaultPortHTTPS = "443"

	loggerComponentName = "HTTPConnectionService"
)

// Names of the headers a connection may not set that have no shared constant: those that commonly
// carry an API key or token, and those the HTTP client manages.
const (
	headerXAPIKey          = "X-Api-Key"      // #nosec G101 -- header name, not a secret
	headerXAuthToken       = "X-Auth-Token"   // #nosec G101 -- header name, not a secret
	headerXAccessToken     = "X-Access-Token" // #nosec G101 -- header name, not a secret
	headerContentLength    = "Content-Length"
	headerTransferEncoding = "Transfer-Encoding"
	headerConnection       = "Connection"
	headerTE               = "TE"
	headerTrailer          = "Trailer"
	headerUpgrade          = "Upgrade"
	headerKeepAlive        = "Keep-Alive"
	headerExpect           = "Expect"
)

// reservedHeaderNames are the plain header names a connection may not set, keyed in lower case:
// those that commonly carry credentials, which belong in the connection's authentication, and
// those the HTTP client manages itself.
var reservedHeaderNames = lowerCaseSet(
	serverconst.AuthorizationHeaderName, serverconst.ProxyAuthorizationHeaderName, serverconst.CookieHeaderName,
	serverconst.APIKeyHeaderName, headerXAPIKey, headerXAuthToken, headerXAccessToken,
	serverconst.ContentTypeHeaderName, serverconst.HostHeaderName, headerContentLength, headerTransferEncoding,
	headerConnection, headerTE, headerTrailer, headerUpgrade, headerKeepAlive, headerExpect,
)
