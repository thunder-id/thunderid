// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package cimd

import (
	"time"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// documentGrantTypes are the grants a CIMD client takes from its document. Any other grant the document
// rules permit is added by an administrator.
var documentGrantTypes = []providers.GrantType{
	providers.GrantTypeAuthorizationCode,
	providers.GrantTypeRefreshToken,
}

// privateJWKMembers are the JWK members that carry private or symmetric key material (RFC 7518, section 6).
var privateJWKMembers = []string{"d", "p", "q", "dp", "dq", "qi", "oth", "k"}

const (
	// maxClientIdentifierURLLength bounds a Client Identifier URL.
	maxClientIdentifierURLLength = 2048
	// cimdFetchTimeout bounds the retrieval of a Client ID Metadata Document.
	cimdFetchTimeout = 5 * time.Second
	// cimdMaxDocumentSize is the largest document processed, as the CIMD draft recommends.
	cimdMaxDocumentSize = 5 * 1024
	// maxApplicationNameLength is the longest name an application create request accepts.
	maxApplicationNameLength = 100
	// minCertificateValueLength and maxCertificateValueLength bound an inline JWKS or a JWKS URI, as the
	// certificate store does.
	minCertificateValueLength = 10
	maxCertificateValueLength = 4096

	schemeHTTPS = "https"
)
