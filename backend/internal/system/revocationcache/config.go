// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocationcache

import "time"

// Config holds the Resource Server token-revocation cache settings, mapped by the caller from the
// deployment server security config. It is intentionally decoupled from system/config so this
// package does not depend on the global configuration type.
type Config struct {
	// Enabled turns RS revocation enforcement on. When false, Initialize returns a no-op enforcer
	// and a no-op syncer.
	Enabled bool
	// Source selects where the deny list comes from: "db" syncs it from the runtime persistent database,
	// "events" fills it from revocation events pushed by the trusted issuer.
	Source string
	// SyncInterval is how often the cache is refreshed from the source. The "events" source is not
	// refreshed on a schedule.
	SyncInterval time.Duration
	// TrustedIssuer identifies the issuer whose pushed events the "events" source accepts.
	TrustedIssuer TrustedIssuer
}

// TrustedIssuer is the issuer that signs the revocation events the "events" source accepts, and the
// audience those events must name.
type TrustedIssuer struct {
	Issuer   string
	JWKSURL  string
	Audience string
}

// isConfigured reports whether every field needed to verify an event is set.
func (t TrustedIssuer) isConfigured() bool {
	return t.Issuer != "" && t.JWKSURL != "" && t.Audience != ""
}
