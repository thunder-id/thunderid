// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package export

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/suite"

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

type ExportToolsTestSuite struct {
	suite.Suite
}

func TestExportToolsTestSuite(t *testing.T) {
	suite.Run(t, new(ExportToolsTestSuite))
}

// session connects a client to a server with the export tool registered over service.
func (s *ExportToolsTestSuite) session(service ExportServiceInterface) *mcp.ClientSession {
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

// The export tool answers as POST /export does: the documents together and the values beside them.
func (s *ExportToolsTestSuite) TestExportToolAnswersAsTheRESTEndpoint() {
	service := &fixedExport{}

	result, err := s.session(service).CallTool(context.Background(), &mcp.CallToolParams{
		Name: "export_configuration", Arguments: map[string]any{"applications": []string{"*"}},
	})
	s.Require().NoError(err)
	s.Require().False(result.IsError)
	s.Equal([]string{"*"}, service.request.Applications)

	exported, _ := service.ExportResources(context.Background(), &ExportRequest{})
	raw, err := json.Marshal(result.StructuredContent)
	s.Require().NoError(err)
	s.JSONEq(`{"resources":"`+s.jsonEscape(buildCombinedResources(exported.Files))+
		`","environment_variables":"APPLICATION_APP_A_CLIENT_ID=a\n"}`, string(raw))
}

// The export is not marked read-only, so a client asks before it runs a tool that can return secrets.
func (s *ExportToolsTestSuite) TestExportToolIsNotMarkedReadOnly() {
	listed, err := s.session(&fixedExport{}).ListTools(context.Background(), nil)
	s.Require().NoError(err)
	s.Require().Len(listed.Tools, 1)
	annotations := listed.Tools[0].Annotations
	s.True(annotations == nil || !annotations.ReadOnlyHint)
}

// A service error is reported with its description, or with its message when it has none.
func (s *ExportToolsTestSuite) TestExportToolReportsAServiceError() {
	for want, svcErr := range map[string]*tidcommon.ServiceError{
		"failed to export: No such application (EXP-1003)": {Code: "EXP-1003",
			Error:            tidcommon.I18nMessage{DefaultValue: "Export failed"},
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "No such application"}},
		"failed to export: Export failed (EXP-1003)": {Code: "EXP-1003",
			Error: tidcommon.I18nMessage{DefaultValue: "Export failed"}},
	} {
		s.Run(want, func() {
			result, err := s.session(&fixedExport{err: svcErr}).CallTool(context.Background(),
				&mcp.CallToolParams{Name: "export_configuration", Arguments: map[string]any{}})
			s.Require().NoError(err)
			s.True(result.IsError)
			s.Require().Len(result.Content, 1)
			text, ok := result.Content[0].(*mcp.TextContent)
			s.Require().True(ok)
			s.Equal(want, text.Text)
		})
	}
}

func (s *ExportToolsTestSuite) jsonEscape(value string) string {
	raw, err := json.Marshal(value)
	s.Require().NoError(err)
	return string(raw[1 : len(raw)-1])
}
