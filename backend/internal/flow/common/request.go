// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type currentRequestContextKey struct{}

// WithCurrentRequest returns a context carrying the HTTP request driving the current flow step so
// flow executors can resolve current-request placeholders. It is transient: it rides on the context
// for one execution and never persists with the flow context.
func WithCurrentRequest(ctx context.Context, req *providers.CurrentRequest) context.Context {
	if req == nil {
		return ctx
	}
	return context.WithValue(ctx, currentRequestContextKey{}, req)
}

// CurrentRequestFrom returns the current flow step's HTTP request carried on the context, or nil
// when none was attached.
func CurrentRequestFrom(ctx context.Context) *providers.CurrentRequest {
	if ctx == nil {
		return nil
	}
	req, _ := ctx.Value(currentRequestContextKey{}).(*providers.CurrentRequest)
	return req
}
