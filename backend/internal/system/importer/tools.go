// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterMCPTools registers the import as an MCP tool that answers as POST /import does.
func RegisterMCPTools(server *mcp.Server, service ImportServiceInterface) {
	destructive := true
	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_import_configuration",
		Description: `Import declarative YAML into this deployment, as POST /import does: create or update each ` +
			`resource in content, filling its placeholders from variables, and remove the resources in deletions. ` +
			`Run it with dryRun first and show the user what it would change.`,
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ImportRequest) (
		*mcp.CallToolResult, *ImportResponse, error) {
		imported, svcErr := service.ImportResources(ctx, &in)
		if svcErr != nil {
			return nil, nil, fmt.Errorf("failed to import: %s (%s)", svcErr.ErrorDescription.DefaultValue, svcErr.Code)
		}
		return nil, imported, nil
	})
}
