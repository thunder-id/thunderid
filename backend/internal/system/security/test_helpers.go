// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

import (
	"context"
	"testing"
)

// NewSecurityContextForTest creates a new immutable SecurityContext.
// Used for testing purposes.
func NewSecurityContextForTest(userID, ouID, token string,
	permissions []string, attributes map[string]interface{}) *SecurityContext {
	if !testing.Testing() {
		panic("only for tests!")
	}
	return &SecurityContext{
		subject:     userID,
		ouID:        ouID,
		token:       token,
		permissions: permissions,
		attributes:  attributes,
	}
}

// WithSecurityContextTest adds security context to the request context.
// Used for testing purposes.
func WithSecurityContextTest(ctx context.Context, authCtx *SecurityContext) context.Context {
	if !testing.Testing() {
		panic("only for tests!")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, securityContextKey, authCtx)
}

// ForEachPermissionPlaneForTest activates each plane's permission set in turn, the control plane's and
// then the data plane's, and calls run with the plane's name while it is active. It restores the control
// plane's set when it returns.
func ForEachPermissionPlaneForTest(run func(plane string)) {
	if !testing.Testing() {
		panic("only for tests!")
	}
	defer InitControlPlanePermissions("")
	for _, plane := range []struct {
		name string
		init func(handle string)
	}{
		{"ControlPlane", InitControlPlanePermissions},
		{"DataPlane", InitDataPlanePermissions},
	} {
		plane.init("")
		run(plane.name)
	}
}
