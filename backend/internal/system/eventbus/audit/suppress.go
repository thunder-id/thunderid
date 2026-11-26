// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import "context"

type suppressedKey struct{}

// WithSuppressed marks ctx so that no audit event built on it is written. The event is still validated.
// It is for a write another record already covers, such as a resource a control plane deployed after
// recording the change itself.
func WithSuppressed(ctx context.Context) context.Context {
	return context.WithValue(ctx, suppressedKey{}, true)
}

func isSuppressed(ctx context.Context) bool {
	suppressed, _ := ctx.Value(suppressedKey{}).(bool)
	return suppressed
}
