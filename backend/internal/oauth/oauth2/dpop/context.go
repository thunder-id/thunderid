// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package dpop

import "context"

// contextKey is a private type for DPoP context value keys to avoid collisions.
type contextKey string

// Context keys for DPoP values propagated across the request pipeline.
const (
	proofKey       contextKey = "dpop_proof"
	jktKey         contextKey = "dpop_jkt"
	requestPathKey contextKey = "dpop_request_path"
)

// WithProof attaches a raw DPoP proof JWT to the context for downstream verification.
func WithProof(ctx context.Context, proof string) context.Context {
	return context.WithValue(ctx, proofKey, proof)
}

// GetProof returns the raw DPoP proof JWT previously attached via WithProof, or "".
func GetProof(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(proofKey).(string); ok {
		return v
	}
	return ""
}

// WithJkt attaches the verified DPoP proof's JWK thumbprint to the context so grant
// handlers can sender-constrain the issued tokens.
func WithJkt(ctx context.Context, jkt string) context.Context {
	return context.WithValue(ctx, jktKey, jkt)
}

// GetJkt returns the verified DPoP jkt previously attached via WithJkt, or "".
func GetJkt(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(jktKey).(string); ok {
		return v
	}
	return ""
}

// WithRequestPath records the path of the request a proof arrived on, so the expected htu can be
// built from the request rather than from a configured endpoint.
//
// RFC 9449 section 4.3 binds a proof to the target URI of the request it accompanies, and one
// handler may serve more than one path, as the token endpoint does. Only the path travels this
// way: scheme and host come from the configured public URL, not from headers an attacker can set.
func WithRequestPath(ctx context.Context, path string) context.Context {
	return context.WithValue(ctx, requestPathKey, path)
}

// GetRequestPath returns the path previously attached via WithRequestPath, or "".
func GetRequestPath(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(requestPathKey).(string); ok {
		return v
	}
	return ""
}
