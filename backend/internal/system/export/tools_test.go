// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package export

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// fixedExport answers every export with the same files, and records the request.
type fixedExport struct {
	ExportServiceInterface
	request *ExportRequest
	err     *tidcommon.ServiceError
}

func (f *fixedExport) ExportResources(_ context.Context, request *ExportRequest) (*ExportResponse,
	*tidcommon.ServiceError) {
	f.request = request
	if f.err != nil {
		return nil, f.err
	}
	return &ExportResponse{
		Files:   []ExportFile{{FileName: "app.yaml", Content: "resource_type: application\nid: app-a\n"}},
		EnvFile: &EnvironmentFile{Content: "APPLICATION_APP_A_CLIENT_ID=a\n"},
	}, nil
}

func exportSession(t *testing.T, service ExportServiceInterface) *mcp.ClientSession {
	t.Helper()
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
	return session
}

// The export tool answers as POST /export does: the documents together and the values beside them.
func TestExportToolAnswersAsTheRESTEndpoint(t *testing.T) {
	service := &fixedExport{}
	session := exportSession(t, service)

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "thunderid_export_configuration", Arguments: map[string]any{"applications": []string{"*"}},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Equal(t, []string{"*"}, service.request.Applications)

	exported, _ := service.ExportResources(context.Background(), &ExportRequest{})
	raw, err := json.Marshal(result.StructuredContent)
	require.NoError(t, err)
	assert.JSONEq(t, `{"resources":"`+jsonEscape(t, buildCombinedResources(exported.Files))+
		`","environment_variables":"APPLICATION_APP_A_CLIENT_ID=a\n"}`, string(raw))
}

func TestExportToolReportsAServiceError(t *testing.T) {
	session := exportSession(t, &fixedExport{err: &tidcommon.ServiceError{Code: "EXP-1003"}})

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "thunderid_export_configuration", Arguments: map[string]any{},
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func jsonEscape(t *testing.T, s string) string {
	t.Helper()
	raw, err := json.Marshal(s)
	require.NoError(t, err)
	return string(raw[1 : len(raw)-1])
}
