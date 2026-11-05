// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package thememgt

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/security"
)

// TestRegisteredToolsRequireDesignPermissions verifies that every theme MCP tool is mapped to a design
// catalog permission, so none falls back to root by accident.
func TestRegisteredToolsRequireDesignPermissions(t *testing.T) {
	security.InitSystemPermissions("")
	p := security.GetSystemPermissions()
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)
	registerMCPTools(server, NewThemeMgtServiceInterfaceMock(t))

	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	_, err := server.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil).
		Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	defer func() { _ = session.Close() }()
	tools, err := session.ListTools(ctx, nil)
	require.NoError(t, err)

	require.NotEmpty(t, tools.Tools)
	for _, registered := range tools.Tools {
		assert.Contains(t, []string{p.DesignView, p.DesignCreate, p.DesignUpdate, p.DesignDelete},
			security.ResolveMCPToolPermission(registered.Name),
			"tool %q must require a design permission", registered.Name)
	}
}
