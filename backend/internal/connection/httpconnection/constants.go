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
	minPort          = 1
	maxPort          = 65535

	// DNS limits on a host name and on each of its labels.
	maxHostLength  = 253
	maxLabelLength = 63

	loggerComponentName = "HTTPConnectionService"
)

// Names of the HTTP-managed headers a connection may not set.
const (
	headerHost             = "Host"
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
// those that carry credentials, which belong in the connection's authentication, and those the
// HTTP client manages itself.
var reservedHeaderNames = lowerCaseSet(
	serverconst.AuthorizationHeaderName, serverconst.ProxyAuthorizationHeaderName, serverconst.CookieHeaderName,
	serverconst.ContentTypeHeaderName, headerHost, headerContentLength, headerTransferEncoding, headerConnection,
	headerTE, headerTrailer, headerUpgrade, headerKeepAlive, headerExpect,
)
