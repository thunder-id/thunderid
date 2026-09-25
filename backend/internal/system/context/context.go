// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package context provides utilities for managing trace IDs (correlation IDs)
package context

import (
	"context"
	"crypto/rand"
	"fmt"
)

type contextKey string

const (
	// TraceIDKey is the context key for storing the trace ID (correlation ID).
	TraceIDKey contextKey = "trace_id"

	// CSPNonceKey is the context key for storing the per-request Content-Security-Policy nonce.
	CSPNonceKey contextKey = "csp_nonce"

	// AccessingOUIDKey is the context key for the organization unit a request is made on behalf of,
	// as opposed to the one the caller belongs to.
	AccessingOUIDKey contextKey = "accessing_ou_id"
)

// ============================================================================
// Trace ID Functions
// ============================================================================

// generateUUID generates a UUID v4 string.
// This is a copy of utils.GenerateUUID to avoid import cycles.
func generateUUID() string {
	var uuid [16]byte
	_, err := rand.Read(uuid[:])
	if err != nil {
		panic(fmt.Errorf("failed to generate random bytes: %w", err))
	}

	uuid[6] = (uuid[6] & 0x0f) | 0x40 // Version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // Variant is 10

	return fmt.Sprintf("%x-%x-%x-%x-%x",
		uuid[0:4],
		uuid[4:6],
		uuid[6:8],
		uuid[8:10],
		uuid[10:],
	)
}

// GetTraceID retrieves the trace ID (correlation ID) from the context.
// If no trace ID exists, it generates a new UUID.
// This trace ID can be used to correlate logs, events, and operations across a request flow.
func GetTraceID(ctx context.Context) string {
	if ctx == nil {
		return generateUUID()
	}

	if traceID, ok := ctx.Value(TraceIDKey).(string); ok && traceID != "" {
		return traceID
	}

	return generateUUID()
}

// WithTraceID adds a trace ID (correlation ID) to the context.
// Use this to propagate trace IDs through your application.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, TraceIDKey, traceID)
}

// EnsureTraceID ensures a trace ID (correlation ID) exists in the context,
// generating one if needed. This is useful at entry points where you want to
// guarantee a trace ID is present for downstream operations.
func EnsureTraceID(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	if traceID, ok := ctx.Value(TraceIDKey).(string); !ok || traceID == "" {
		ctx = WithTraceID(ctx, generateUUID())
	}

	return ctx
}

// ============================================================================
// CSP Nonce Functions
// ============================================================================

// GetCSPNonce retrieves the Content-Security-Policy nonce from the context. Returns "" if absent; it
// does not generate one, since the nonce must be generated once per request by the middleware that
// also emits it in the CSP header.
func GetCSPNonce(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	nonce, _ := ctx.Value(CSPNonceKey).(string)
	return nonce
}

// WithCSPNonce adds the Content-Security-Policy nonce to the context.
func WithCSPNonce(ctx context.Context, nonce string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, CSPNonceKey, nonce)
}

// ============================================================================
// Accessing Organization Unit Functions
// ============================================================================

// GetAccessingOUID retrieves the organization unit a request is made on behalf of, or "" when the
// request named none.
//
// This is not the caller's own organization unit, which lives on the security context: an M2M
// application authenticates as itself and then asks for a token for one of its customers. Reading it
// back from one place is what keeps admission, scope filtering and claim resolution agreeing on which
// organization that is.
func GetAccessingOUID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	ouID, _ := ctx.Value(AccessingOUIDKey).(string)
	return ouID
}

// WithAccessingOUID records the organization unit a request is made on behalf of.
func WithAccessingOUID(ctx context.Context, ouID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, AccessingOUIDKey, ouID)
}
