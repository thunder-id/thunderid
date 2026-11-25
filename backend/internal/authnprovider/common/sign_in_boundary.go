// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import "context"

// SignInBoundary is where a sign-in may land: one OU when OUID is set, else wherever ApplicationID is accessible.
// EntityID is the subject signed in so far, or empty before sign-in.
type SignInBoundary struct {
	OUID          string
	ApplicationID string
	EntityID      string
}

type signInBoundaryContextKey struct{}

// WithSignInBoundary returns a context carrying the sign-in boundary of the current authentication.
func WithSignInBoundary(ctx context.Context, b SignInBoundary) context.Context {
	return context.WithValue(ctx, signInBoundaryContextKey{}, b)
}

// SignInBoundaryFrom returns the sign-in boundary carried on the context. The second return value
// reports whether one was set; callers must skip the boundary check when it was not.
func SignInBoundaryFrom(ctx context.Context) (SignInBoundary, bool) {
	b, ok := ctx.Value(signInBoundaryContextKey{}).(SignInBoundary)
	return b, ok
}
