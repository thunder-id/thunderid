// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzdetailmgt

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/mcp/tool"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const toolsTestName = "Flight"

const toolsTestSchema = `{"type":"object","properties":{"route":{"type":"string"},` +
	`"maxFare":{"type":"number","policy":{"modification":"decrease","modifiableBy":["user"]}}}}`

type TypeToolsTestSuite struct {
	suite.Suite
	service *AuthorizationDetailMgtServiceInterfaceMock
	tools   *typeTools
}

func TestTypeToolsTestSuite(t *testing.T) {
	suite.Run(t, new(TypeToolsTestSuite))
}

func (s *TypeToolsTestSuite) SetupTest() {
	s.service = NewAuthorizationDetailMgtServiceInterfaceMock(s.T())
	s.tools = &typeTools{service: s.service}
}

func toolsTestType() providers.AuthorizationDetailType {
	return providers.AuthorizationDetailType{ID: "type-1", Type: "flight_booking", Name: toolsTestName, Version: 2,
		Schema: json.RawMessage(toolsTestSchema), ConsentTemplate: "Book {{route}}", Reuse: true}
}

// The tool lists every type, however many there are, not only a first page.
func (s *TypeToolsTestSuite) TestListTypes() {
	s.service.EXPECT().ListAllAuthorizationDetailTypes(mock.Anything).
		Return([]providers.AuthorizationDetailType{toolsTestType()}, nil)

	_, out, err := s.tools.listTypes(context.Background(), &mcp.CallToolRequest{}, nil)

	s.Require().NoError(err)
	s.Equal(1, out.TotalResults)
	s.Equal([]typeMCPItem{{ID: "type-1", Type: "flight_booking", Name: toolsTestName, Version: 2, Reuse: true}},
		out.Types)
}

func (s *TypeToolsTestSuite) TestListTypesFails() {
	s.service.EXPECT().ListAllAuthorizationDetailTypes(mock.Anything).
		Return(nil, &tidcommon.InternalServerError)

	_, _, err := s.tools.listTypes(context.Background(), &mcp.CallToolRequest{}, nil)

	s.Error(err)
}

func (s *TypeToolsTestSuite) TestGetType() {
	typ := toolsTestType()
	s.service.EXPECT().GetAuthorizationDetailType(mock.Anything, "type-1").Return(&typ, nil)

	out, err := s.tools.getType(context.Background(), tool.IDInput{ID: "type-1"})

	s.Require().NoError(err)
	s.Equal(toTypeResponse(typ), *out)
}

func (s *TypeToolsTestSuite) TestGetTypeNotFound() {
	s.service.EXPECT().GetAuthorizationDetailType(mock.Anything, "missing").Return(nil, &ErrorTypeNotFound)

	_, err := s.tools.getType(context.Background(), tool.IDInput{ID: "missing"})

	s.ErrorContains(err, "failed to get authorization details type")
}

func (s *TypeToolsTestSuite) TestCreateTypeGoesThroughTheService() {
	created := toolsTestType()
	s.service.EXPECT().CreateAuthorizationDetailType(mock.Anything, mock.MatchedBy(
		func(t providers.AuthorizationDetailType) bool {
			return t.Type == "flight_booking" && t.Name == toolsTestName && !t.Reuse &&
				string(t.Schema) == toolsTestSchema
		})).Return(&created, nil)

	out, err := s.tools.createType(context.Background(),
		AuthorizationDetailTypeRequest{Type: "flight_booking", Name: toolsTestName,
			Schema: json.RawMessage(toolsTestSchema)})

	s.Require().NoError(err)
	s.Equal("type-1", out.ID)
}

func (s *TypeToolsTestSuite) TestCreateTypeRefused() {
	s.service.EXPECT().CreateAuthorizationDetailType(mock.Anything, mock.Anything).Return(nil, &ErrorInvalidSchema)

	_, err := s.tools.createType(context.Background(),
		AuthorizationDetailTypeRequest{Type: "x", Name: "X", Schema: json.RawMessage(`{"type":"string"}`)})

	s.ErrorContains(err, "failed to create authorization details type")
}

func (s *TypeToolsTestSuite) TestUpdateType() {
	updated := toolsTestType()
	s.service.EXPECT().UpdateAuthorizationDetailType(mock.Anything, "type-1", mock.MatchedBy(
		func(t providers.AuthorizationDetailType) bool { return t.ConsentTemplate == "Book {{route}}" })).
		Return(&updated, nil)

	out, err := s.tools.updateType(context.Background(), updateTypeInput{ID: "type-1",
		AuthorizationDetailTypeRequest: AuthorizationDetailTypeRequest{Type: "flight_booking", Name: toolsTestName,
			Schema: json.RawMessage(toolsTestSchema), ConsentTemplate: "Book {{route}}"}})

	s.Require().NoError(err)
	s.Equal(2, out.Version)
}

func (s *TypeToolsTestSuite) TestUpdateTypeRefused() {
	s.service.EXPECT().UpdateAuthorizationDetailType(mock.Anything, "type-1", mock.Anything).
		Return(nil, &ErrorTypeValueImmutable)

	_, err := s.tools.updateType(context.Background(), updateTypeInput{ID: "type-1"})

	s.ErrorContains(err, "failed to update authorization details type")
}

// Calls the tools through an MCP client to check the schema keeps its property order.
func (s *TypeToolsTestSuite) TestToolsOverMCP() {
	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "test"}, nil)
	registerMCPTools(server, s.service)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	s.Require().NoError(err)
	defer func() { _ = serverSession.Close() }()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "client"}, nil).Connect(ctx, clientTransport, nil)
	s.Require().NoError(err)
	defer func() { _ = session.Close() }()

	listed, err := session.ListTools(ctx, nil)
	s.Require().NoError(err)
	names := make([]string, 0, len(listed.Tools))
	for _, t := range listed.Tools {
		names = append(names, t.Name)
	}
	s.ElementsMatch([]string{"list_authorization_detail_types",
		"get_authorization_detail_type_by_id", "create_authorization_detail_type",
		"update_authorization_detail_type"}, names)

	created := toolsTestType()
	var received json.RawMessage
	s.service.EXPECT().CreateAuthorizationDetailType(mock.Anything, mock.Anything).
		Run(func(_ context.Context, t providers.AuthorizationDetailType) { received = t.Schema }).
		Return(&created, nil)
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "create_authorization_detail_type",
		Arguments: map[string]any{"type": "flight_booking", "name": toolsTestName,
			"schema": json.RawMessage(toolsTestSchema)}})
	s.Require().NoError(err)
	s.Require().False(result.IsError, "%v", result.Content)
	structured, err := json.Marshal(result.StructuredContent)
	s.Require().NoError(err)
	var out AuthorizationDetailTypeResponse
	s.Require().NoError(json.Unmarshal(structured, &out))
	s.Equal("type-1", out.ID)
	s.JSONEq(toolsTestSchema, string(out.Schema))
	s.Equal(toolsTestSchema, string(received), "the schema reaches the service as written")

	got := toolsTestType()
	s.service.EXPECT().GetAuthorizationDetailType(mock.Anything, "type-1").Return(&got, nil)
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "get_authorization_detail_type_by_id",
		Arguments: map[string]any{"id": "type-1"}})
	s.Require().NoError(err)
	s.Require().False(result.IsError)
	text := result.Content[0].(*mcp.TextContent).Text
	s.Contains(text, `"schema":`+toolsTestSchema, "the schema is returned as stored, in its declared order")

	s.service.EXPECT().UpdateAuthorizationDetailType(mock.Anything, "type-1", mock.Anything).
		Return(nil, &ErrorTypeValueImmutable)
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "update_authorization_detail_type",
		Arguments: map[string]any{"id": "type-1", "type": "other", "name": toolsTestName,
			"schema": json.RawMessage(toolsTestSchema)}})
	s.Require().NoError(err)
	s.True(result.IsError, "a refusal is a tool error")

	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "create_authorization_detail_type",
		Arguments: map[string]any{"type": 5}})
	s.Require().NoError(err)
	s.True(result.IsError, "malformed arguments are a tool error")

	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "create_authorization_detail_type",
		Arguments: map[string]any{"type": "flight_booking", "name": toolsTestName,
			"schema": json.RawMessage(toolsTestSchema), "procesor": map[string]any{"endpoint": "https://bank"}}})
	s.Require().NoError(err)
	s.True(result.IsError, "a misspelled key is refused rather than dropped")

	// A definition the get tool returned is sent back as it is.
	fetched := toolsTestType()
	fetched.ID = "type-fetched"
	var arguments map[string]any
	encoded, err := json.Marshal(toTypeResponse(fetched))
	s.Require().NoError(err)
	s.Require().NoError(json.Unmarshal(encoded, &arguments))
	s.Require().Contains(arguments, "version")
	s.Require().Contains(arguments, "isReadOnly")
	s.service.EXPECT().UpdateAuthorizationDetailType(mock.Anything, fetched.ID, mock.Anything).
		Return(&fetched, nil).Once()
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "update_authorization_detail_type",
		Arguments: arguments})
	s.Require().NoError(err)
	s.False(result.IsError, "%s", result.Content[0].(*mcp.TextContent).Text)

	// The create expectation set above answers this call.
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "create_authorization_detail_type",
		Arguments: arguments})
	s.Require().NoError(err)
	s.False(result.IsError, "a fetched definition can be created again: %v", result.Content)
}
