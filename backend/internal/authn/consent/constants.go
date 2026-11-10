// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package consent

import "math"

const (
	// MaxValidityPeriod is the longest consent validity period, in seconds, that is recorded, so the expiry time
	// stays a valid timestamp. The consent executor treats a period of this length as consent that does not expire.
	MaxValidityPeriod = int64(math.MaxInt32)

	// consentSessionTokenAudience is the JWT audience for consent session tokens
	consentSessionTokenAudience = "consent-svc"

	// consentSessionTokenValidityPeriod is the validity period of consent session tokens in seconds
	consentSessionTokenValidityPeriod = int64(3600)

	// consentSessionClaimKey is the JWT claim key for consent session data
	consentSessionClaimKey = "consent_session"
)
