// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import "context"

// recoveryFlowContextKey is the context key that marks a recovery flow.
type recoveryFlowContextKey struct{}

// WithRecoveryFlow returns a context that marks authentication as a step of a recovery flow.
func WithRecoveryFlow(ctx context.Context) context.Context {
	return context.WithValue(ctx, recoveryFlowContextKey{}, true)
}

// IsRecoveryFlow reports whether the context belongs to a recovery flow.
func IsRecoveryFlow(ctx context.Context) bool {
	recovery, _ := ctx.Value(recoveryFlowContextKey{}).(bool)
	return recovery
}
