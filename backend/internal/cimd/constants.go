// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package cimd

import "time"

const (
	// maxClientIdentifierURLLength bounds a Client Identifier URL.
	maxClientIdentifierURLLength = 2048
	// cimdFetchTimeout bounds the retrieval of a Client ID Metadata Document.
	cimdFetchTimeout = 5 * time.Second
	// cimdMaxDocumentSize is the largest document processed, as the CIMD draft recommends.
	cimdMaxDocumentSize = 5 * 1024
	// maxApplicationNameLength is the longest name an application create request accepts.
	maxApplicationNameLength = 100

	schemeHTTPS = "https"
)
