// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// fixedImport answers every import with one outcome, and records the request.
type fixedImport struct {
	ImportServiceInterface
	request *ImportRequest
}

func (f *fixedImport) ImportResources(_ context.Context, request *ImportRequest) (*ImportResponse,
	*tidcommon.ServiceError) {
	f.request = request
	return &ImportResponse{
		Summary: &ImportSummary{TotalDocuments: 1, Failed: 1},
		Results: []ImportItemOutcome{{ResourceType: "agent", ResourceID: "a1", Status: "failed", Code: "AGT-1039"}},
	}, nil
}

// The import tool passes the request on as POST /import takes it, and answers as it does.
func TestImportToolAnswersAsTheRESTEndpoint(t *testing.T) {
	service := &fixedImport{}
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)
	RegisterMCPTools(server, service)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })
	session, err := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "1"}, nil).
		Connect(context.Background(), clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "thunderid_import_configuration", Arguments: map[string]any{
			"content": "resource_type: agent\nid: a1\n", "variables": map[string]any{"OWNER": "u1"},
			"dryRun": true, "deletions": []map[string]any{{"resourceType": "role", "id": "r1"}},
		},
	})
	require.NoError(t, err)
	require.False(t, result.IsError, "%v", result.Content)

	assert.Equal(t, "resource_type: agent\nid: a1\n", service.request.Content)
	assert.Equal(t, map[string]any{"OWNER": "u1"}, service.request.Variables)
	assert.True(t, service.request.DryRun)
	assert.Equal(t, []ResourceDeletion{{ResourceType: "role", ID: "r1"}}, service.request.Deletions)
	answered, _ := service.ImportResources(context.Background(), &ImportRequest{})
	want, _ := json.Marshal(answered)
	raw, _ := json.Marshal(result.StructuredContent)
	assert.JSONEq(t, string(want), string(raw))
}
