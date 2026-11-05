// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

// Ref names an identity or a resource by type and, where one exists, ID. The zero Ref names none.
type Ref struct {
	Type string
	ID   string
}

// Actor is the principal that made the attempt. An anonymous actor has no ID.
type Actor struct {
	Type ActorType
	ID   string
}

// Outcome is how an attempt ended. Reason is a stable code, required unless the attempt succeeded.
type Outcome struct {
	Result Result
	Reason string
}

// Client is where a request came from.
type Client struct {
	// Address is the TCP peer address, so behind a proxy it is the proxy's.
	Address string
	// UserAgent is the request's user agent, capped at 256 characters.
	UserAgent string
}
