// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/security"
)

func TestToolPermissionMiddleware(t *testing.T) {
	security.InitSystemPermissions("")

	tests := []struct {
		name        string
		tool        string
		permissions []string
		wantCalled  bool
	}{
		{name: "RootCallsAnyTool", tool: "create_flow", permissions: []string{"system"}, wantCalled: true},
		{name: "ExactPermission", tool: "create_flow",
			permissions: []string{"system:flow:create"}, wantCalled: true},
		{name: "UpdateCannotCreate", tool: "create_flow",
			permissions: []string{"system:flow:update"}, wantCalled: false},
		{name: "CreateCannotUpdate", tool: "update_flow",
			permissions: []string{"system:flow:create"}, wantCalled: false},
		{name: "ParentPermission", tool: "update_flow",
			permissions: []string{"system:flow"}, wantCalled: true},
		{name: "ViewCannotWrite", tool: "create_flow",
			permissions: []string{"system:flow:view"}, wantCalled: false},
		{name: "ViewCanRead", tool: "list_flows",
			permissions: []string{"system:flow:view"}, wantCalled: true},
		{name: "UnmappedToolNeedsRoot", tool: "list_gateway_variables",
			permissions: []string{"system:gateway"}, wantCalled: false},
		{name: "NoPermissions", tool: "list_flows", permissions: nil, wantCalled: false},
		{name: "AppRootCallsAnyTool", tool: "create_application", permissions: []string{"system"}, wantCalled: true},
		{name: "AppExactPermission", tool: "create_application",
			permissions: []string{"system:app:create"}, wantCalled: true},
		{name: "AppUpdateCannotCreate", tool: "create_application",
			permissions: []string{"system:app:update"}, wantCalled: false},
		{name: "AppCreateCannotUpdate", tool: "update_application",
			permissions: []string{"system:app:create"}, wantCalled: false},
		{name: "AppParentPermission", tool: "update_application",
			permissions: []string{"system:app"}, wantCalled: true},
		{name: "AppViewCannotWrite", tool: "create_application",
			permissions: []string{"system:app:view"}, wantCalled: false},
		{name: "AppViewCanRead", tool: "list_applications",
			permissions: []string{"system:app:view"}, wantCalled: true},
		{name: "AppUnmappedToolNeedsRoot", tool: "list_flows",
			permissions: []string{"system:app"}, wantCalled: false},
		{name: "AppNoPermissions", tool: "list_applications", permissions: nil, wantCalled: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			next := func(context.Context, string, mcp.Request) (mcp.Result, error) {
				called = true
				return &mcp.CallToolResult{}, nil
			}
			ctx := security.WithSecurityContextTest(context.Background(),
				security.NewSecurityContextForTest("user-1", "ou-1", "", tt.permissions, nil))

			result, err := toolPermissionMiddleware(next)(ctx, "tools/call",
				&mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: tt.tool}})

			require.NoError(t, err)
			assert.Equal(t, tt.wantCalled, called)
			if !tt.wantCalled {
				callResult, ok := result.(*mcp.CallToolResult)
				require.True(t, ok)
				assert.True(t, callResult.IsError)
			}
		})
	}
}

func TestToolPermissionMiddleware_OtherMethodsPassThrough(t *testing.T) {
	security.InitSystemPermissions("")

	called := false
	next := func(context.Context, string, mcp.Request) (mcp.Result, error) {
		called = true
		return &mcp.ListToolsResult{}, nil
	}

	_, err := toolPermissionMiddleware(next)(context.Background(), "tools/list", &mcp.ListToolsRequest{})

	require.NoError(t, err)
	assert.True(t, called)
}
