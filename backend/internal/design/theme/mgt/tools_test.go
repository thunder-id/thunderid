// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package thememgt

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/mcp/tool"
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

// list_themes with a projectId lists the themes of that project, and get_theme_by_id carries the project.
func TestThemeToolsWithProject(t *testing.T) {
	svc := NewThemeMgtServiceInterfaceMock(t)
	tools := &themeTools{themeMgtService: svc}
	ctx := context.Background()

	svc.EXPECT().GetThemeListByProject(ctx, "project-1", serverconst.MaxPageSize, 0).Return(&ThemeList{
		TotalResults: 1, Count: 1, Themes: []Theme{{ID: "theme-1", ProjectID: "project-1"}},
	}, nil)
	_, listed, err := tools.listThemes(ctx, nil, themeListMCPInput{ProjectID: "project-1"})
	require.NoError(t, err)
	assert.Equal(t, 1, listed.TotalResults)
	require.Len(t, listed.Themes, 1)
	assert.Equal(t, "project-1", listed.Themes[0].ProjectID)

	svc.EXPECT().GetThemeList(ctx, serverconst.MaxPageSize, 0).Return(&ThemeList{
		TotalResults: 2, Count: 2, Themes: []Theme{{ID: "theme-1", ProjectID: "project-1"}, {ID: "theme-2"}},
	}, nil)
	_, all, err := tools.listThemes(ctx, nil, themeListMCPInput{})
	require.NoError(t, err)
	assert.Equal(t, 2, all.TotalResults)

	svc.EXPECT().GetTheme(ctx, "theme-1").Return(&Theme{ID: "theme-1", ProjectID: "project-1"}, nil)
	_, detail, err := tools.getThemeByID(ctx, nil, tool.IDInput{ID: "theme-1"})
	require.NoError(t, err)
	assert.Equal(t, "project-1", detail.ProjectID)
}
