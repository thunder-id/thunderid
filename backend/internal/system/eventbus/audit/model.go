// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import "github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

// Ref names an identity or a resource by type and, where one exists, ID. The zero Ref names none.
type Ref = providers.AuditRef

// Actor is the principal that made the attempt. An anonymous actor has no ID.
type Actor = providers.AuditActor

// Outcome is how an attempt ended. Reason is a stable code, required unless the attempt succeeded.
type Outcome = providers.AuditOutcome

// Client is where a request came from, recorded only when the TCP peer is a public address, and the zero
// Client otherwise.
type Client = providers.AuditClient
