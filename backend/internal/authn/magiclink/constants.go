// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package magiclink

const (
	// DefaultExpirySeconds is the default expiry time for magic link tokens in seconds.
	DefaultExpirySeconds = 300

	// tokenAudience is the audience claim for magic link tokens.
	tokenAudience = "magiclink-svc"

	// ClaimMagicLinkUsedJti is the authenticated claim key for the used magic link JTI.
	ClaimMagicLinkUsedJti = "magicLinkUsedJti"

	// CredentialKeyUsedJti is the credential map key for the magic link used JTI.
	CredentialKeyUsedJti = "usedJti"

	// CredentialKeyToken is the credential map key for the magic link token.
	CredentialKeyToken = "token"

	// CredentialKeySubjectAttribute is the credential map key for the magic link subject attribute.
	CredentialKeySubjectAttribute = "subjectAttribute"
)
