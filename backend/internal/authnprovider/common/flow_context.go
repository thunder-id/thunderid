// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import "context"

// recoveryFlowContextKey is the context key that marks a recovery flow.
type recoveryFlowContextKey struct{}

// verificationFlowContextKey is the context key that marks a contact verification.
type verificationFlowContextKey struct{}

// WithRecoveryFlow returns a context that marks authentication as a step of a recovery flow.
func WithRecoveryFlow(ctx context.Context) context.Context {
	return context.WithValue(ctx, recoveryFlowContextKey{}, true)
}

// IsRecoveryFlow reports whether the context belongs to a recovery flow.
func IsRecoveryFlow(ctx context.Context) bool {
	recovery, _ := ctx.Value(recoveryFlowContextKey{}).(bool)
	return recovery
}

// WithVerificationFlow returns a context that marks a proof as verifying a contact attribute.
func WithVerificationFlow(ctx context.Context) context.Context {
	return context.WithValue(ctx, verificationFlowContextKey{}, true)
}

// IsVerificationFlow reports whether the context's proof verifies a contact attribute.
func IsVerificationFlow(ctx context.Context) bool {
	verification, _ := ctx.Value(verificationFlowContextKey{}).(bool)
	return verification
}
