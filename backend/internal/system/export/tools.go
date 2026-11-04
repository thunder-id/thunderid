// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package export

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterMCPTools registers the export as an MCP tool that answers as POST /export does.
func RegisterMCPTools(server *mcp.Server, service ExportServiceInterface) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "export_configuration",
		Description: `Export resources as declarative YAML, as POST /export does: the resources named by ID, or ` +
			`"*" for every resource of a type. On a control plane values are written as references; elsewhere ` +
			`environment_variables carries the values the YAML refers to, secrets included, so do not repeat it ` +
			`back to the user.`,
		// It changes nothing, but it is not marked read-only: a client runs read-only tools without asking,
		// and an export outside a control plane returns secret values.
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ExportRequest) (
		*mcp.CallToolResult, *JSONExportResponse, error) {
		exported, svcErr := service.ExportResources(ctx, &in)
		if svcErr != nil {
			reason := svcErr.ErrorDescription.DefaultValue
			if reason == "" {
				reason = svcErr.Error.DefaultValue
			}
			return nil, nil, fmt.Errorf("failed to export: %s (%s)", reason, svcErr.Code)
		}
		answer := &JSONExportResponse{Resources: buildCombinedResources(exported.Files)}
		if exported.EnvFile != nil {
			answer.EnvironmentVariables = exported.EnvFile.Content
		}
		return nil, answer, nil
	})
}
