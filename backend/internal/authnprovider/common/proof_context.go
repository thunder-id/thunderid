// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import "context"

// recoveryProofContextKey is the context key that marks a proof submitted to recover an account.
type recoveryProofContextKey struct{}

// contactProofContextKey is the context key that marks a proof submitted for a contact verification.
type contactProofContextKey struct{}

// WithRecoveryProof returns a context that marks a proof as submitted to recover an account.
func WithRecoveryProof(ctx context.Context) context.Context {
	return context.WithValue(ctx, recoveryProofContextKey{}, true)
}

// IsRecoveryProof reports whether the context marks a proof submitted to recover an account.
func IsRecoveryProof(ctx context.Context) bool {
	recovery, _ := ctx.Value(recoveryProofContextKey{}).(bool)
	return recovery
}

// WithContactProof returns a context that marks a proof as submitted for a contact verification.
func WithContactProof(ctx context.Context) context.Context {
	return context.WithValue(ctx, contactProofContextKey{}, true)
}

// IsContactProof reports whether the context marks a proof submitted for a contact verification.
func IsContactProof(ctx context.Context) bool {
	contact, _ := ctx.Value(contactProofContextKey{}).(bool)
	return contact
}
