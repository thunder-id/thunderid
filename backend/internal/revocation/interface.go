// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package revocation defines dependency-light contracts shared by revocation producers and consumers.
// It holds no implementation: the OAuth revocation service produces against these contracts and the
// flow executors and Resource Server cache consume them, without either side importing the other.
package revocation

import "context"

// CriteriaRevoker persists criteria-based revocations. It is the seam a producer of revocation intent
// writes through, so a caller need not depend on the OAuth revocation implementation.
type CriteriaRevoker interface {
	RevokeByCriteria(ctx context.Context, revocation CriteriaRevocation) error
	// RevokeCriteriaBatch records a set of criteria revocations in as few round trips as the deny list
	// allows, rather than one round trip per criterion. A producer that can revoke many criteria in one
	// trusted operation — an administration flow's revocation plan, notably — writes through this
	// instead of calling RevokeByCriteria once per criterion.
	RevokeCriteriaBatch(ctx context.Context, revocations []CriteriaRevocation) error
}
