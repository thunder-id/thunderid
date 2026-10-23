// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// fixedImport answers every import with one outcome, or with err, and records the request.
type fixedImport struct {
	ImportServiceInterface
	request *ImportRequest
	err     *tidcommon.ServiceError
}

func (f *fixedImport) ImportResources(_ context.Context, request *ImportRequest) (*ImportResponse,
	*tidcommon.ServiceError) {
	f.request = request
	if f.err != nil {
		return nil, f.err
	}
	return &ImportResponse{
		Summary: &ImportSummary{TotalDocuments: 1, Failed: 1},
		Results: []ImportItemOutcome{{ResourceType: "agent", ResourceID: "a1", Status: "failed", Code: "AGT-1039"}},
	}, nil
}

type ImportToolsTestSuite struct {
	suite.Suite
}

func TestImportToolsTestSuite(t *testing.T) {
	suite.Run(t, new(ImportToolsTestSuite))
}

// session connects a client to a server with the import tool registered over service.
func (s *ImportToolsTestSuite) session(service ImportServiceInterface) *mcp.ClientSession {
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)
	RegisterMCPTools(server, service)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = serverSession.Close() })
	session, err := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "1"}, nil).
		Connect(context.Background(), clientTransport, nil)
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = session.Close() })
	return session
}

// The import tool passes the request on as POST /import takes it, and answers as it does.
func (s *ImportToolsTestSuite) TestImportToolAnswersAsTheRESTEndpoint() {
	service := &fixedImport{}

	result, err := s.session(service).CallTool(context.Background(), &mcp.CallToolParams{
		Name: "import_configuration", Arguments: map[string]any{
			"content": "resource_type: agent\nid: a1\n", "variables": map[string]any{"OWNER": "u1"},
			"dryRun": true, "deletions": []map[string]any{{"resourceType": "role", "id": "r1"}},
		},
	})
	s.Require().NoError(err)
	s.Require().False(result.IsError, "%v", result.Content)

	s.Equal("resource_type: agent\nid: a1\n", service.request.Content)
	s.Equal(map[string]any{"OWNER": "u1"}, service.request.Variables)
	s.True(service.request.DryRun)
	s.Equal([]ResourceDeletion{{ResourceType: "role", ID: "r1"}}, service.request.Deletions)
	answered, _ := service.ImportResources(context.Background(), &ImportRequest{})
	want, _ := json.Marshal(answered)
	raw, _ := json.Marshal(result.StructuredContent)
	s.JSONEq(string(want), string(raw))
}

// A service error is reported with its description, or with its message when it has none.
func (s *ImportToolsTestSuite) TestImportToolReportsAServiceError() {
	for want, svcErr := range map[string]*tidcommon.ServiceError{
		"failed to import: The YAML is not valid (IMP-1001)": {Code: "IMP-1001",
			Error:            tidcommon.I18nMessage{DefaultValue: "Import failed"},
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "The YAML is not valid"}},
		"failed to import: Import failed (IMP-1001)": {Code: "IMP-1001",
			Error: tidcommon.I18nMessage{DefaultValue: "Import failed"}},
	} {
		s.Run(want, func() {
			result, err := s.session(&fixedImport{err: svcErr}).CallTool(context.Background(),
				&mcp.CallToolParams{Name: "import_configuration", Arguments: map[string]any{"content": "x"}})
			s.Require().NoError(err)
			s.True(result.IsError)
			s.Require().Len(result.Content, 1)
			text, ok := result.Content[0].(*mcp.TextContent)
			s.Require().True(ok)
			s.Equal(want, text.Text)
		})
	}
}
