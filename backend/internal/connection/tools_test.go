// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/connection/httpconnectionmock"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
	"github.com/thunder-id/thunderid/tests/mocks/notification/notificationmock"
)

const toolsStoredURL = "https://risk.example.com/evaluate"

type HTTPConnectionToolsTestSuite struct {
	suite.Suite
	mockHTTP *httpconnectionmock.HTTPConnectionServiceInterfaceMock
	session  *mcp.ClientSession
}

func TestHTTPConnectionToolsTestSuite(t *testing.T) {
	suite.Run(t, new(HTTPConnectionToolsTestSuite))
}

// SetupTest registers the tools over the real connection facade, backed by a mocked HTTP connection
// service, and connects an in-memory MCP client, so every call crosses the SDK's schema validation
// and output encoding as an agent's would.
func (s *HTTPConnectionToolsTestSuite) SetupTest() {
	initConfigWithTestCryptoKey(s.T())
	s.T().Cleanup(config.ResetServerRuntime)
	s.mockHTTP = httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	svc := newService(idpmock.NewIDPServiceInterfaceMock(s.T()),
		notificationmock.NewNotificationSenderMgtSvcInterfaceMock(s.T()), &testResourceServerLister{},
		newAuthZENPDPServiceMock(s.T()), s.mockHTTP)

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)
	registerMCPTools(server, svc)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	s.session, err = client.Connect(ctx, clientTransport, nil)
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = s.session.Close() })
}

// storedConnection is the connection the HTTP connection service returns: the password is a real
// encrypted secret property, as it is after a store read.
func (s *HTTPConnectionToolsTestSuite) storedConnection(url string) *httpconnection.HTTPConnection {
	auth, err := outboundauth.ToProperties(outboundauth.Config{Type: outboundauth.TypeBasic,
		Properties: map[string]string{outboundauth.FieldBasicUsername: "svc-user",
			outboundauth.FieldBasicPassword: testHTTPConnectionPassword}})
	s.Require().NoError(err)
	return &httpconnection.HTTPConnection{
		ID: "http-1", Name: "Risk", Description: "Risk engine", URL: url, TimeoutMS: 5000,
		Headers: []httpconnection.Header{{Name: "X-Tenant", Value: "acme"}}, AuthProperties: auth,
	}
}

// call invokes a tool through the client session and returns the raw JSON a client receives, with
// the structured content and the text content both included.
func (s *HTTPConnectionToolsTestSuite) call(name string, args map[string]any) (*mcp.CallToolResult, string) {
	result, err := s.session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	s.Require().NoError(err)
	raw, err := json.Marshal(result)
	s.Require().NoError(err)
	return result, string(raw)
}

func (s *HTTPConnectionToolsTestSuite) errorText(result *mcp.CallToolResult) string {
	s.Require().True(result.IsError)
	s.Require().NotEmpty(result.Content)
	text, ok := result.Content[0].(*mcp.TextContent)
	s.Require().True(ok)
	return text.Text
}

func (s *HTTPConnectionToolsTestSuite) structured(result *mcp.CallToolResult, out any) {
	s.Require().False(result.IsError, "tool returned an error: %v", result.Content)
	raw, err := json.Marshal(result.StructuredContent)
	s.Require().NoError(err)
	s.Require().NoError(json.Unmarshal(raw, out))
}

// captureUpdate makes the HTTP connection service hold the stored connection at storedURL and records
// the connection the facade hands to UpdateHTTPConnection.
func (s *HTTPConnectionToolsTestSuite) captureUpdate(storedURL string) *httpconnection.HTTPConnection {
	stored := s.storedConnection(storedURL)
	s.mockHTTP.On("GetHTTPConnection", mock.Anything, "http-1").Return(stored, (*tidcommon.ServiceError)(nil))
	received := &httpconnection.HTTPConnection{}
	s.mockHTTP.On("UpdateHTTPConnection", mock.Anything, "http-1", mock.Anything).
		Run(func(args mock.Arguments) { *received = args.Get(2).(httpconnection.HTTPConnection) }).
		Return(stored, (*tidcommon.ServiceError)(nil))
	return received
}

func (s *HTTPConnectionToolsTestSuite) receivedPassword(received *httpconnection.HTTPConnection) string {
	authConfig, err := outboundauth.FromProperties(received.AuthProperties)
	s.Require().NoError(err)
	return authConfig.Get(outboundauth.FieldBasicPassword)
}

func basicUpdateArgs(url string, properties map[string]any) map[string]any {
	return map[string]any{
		"id": "http-1", "name": "Risk", "url": url,
		"authentication": map[string]any{"type": "basic", "properties": properties},
	}
}

func (s *HTTPConnectionToolsTestSuite) TestRegisterMCPTools() {
	result, err := s.session.ListTools(context.Background(), nil)
	s.Require().NoError(err)

	tools := map[string]*mcp.Tool{}
	for _, tool := range result.Tools {
		tools[tool.Name] = tool
	}
	s.Len(tools, 4)
	for _, name := range []string{"list_http_connections", "get_http_connection_by_id"} {
		s.Require().Contains(tools, name)
		s.True(tools[name].Annotations.ReadOnlyHint, name)
	}
	s.Require().Contains(tools, "create_http_connection")
	s.False(tools["create_http_connection"].Annotations.ReadOnlyHint)
	s.False(tools["create_http_connection"].Annotations.IdempotentHint)
	s.Require().Contains(tools, "update_http_connection")
	s.False(tools["update_http_connection"].Annotations.ReadOnlyHint)
	s.True(tools["update_http_connection"].Annotations.IdempotentHint)

	var createSchema, updateSchema struct {
		Required   []string                   `json:"required"`
		Properties map[string]json.RawMessage `json:"properties"`
	}
	s.decodeSchema(tools["create_http_connection"].InputSchema, &createSchema)
	s.decodeSchema(tools["update_http_connection"].InputSchema, &updateSchema)
	s.NotContains(createSchema.Properties, "id")
	s.Contains(updateSchema.Required, "id")
	for _, schema := range []string{"create_http_connection", "update_http_connection"} {
		var decoded struct {
			Properties map[string]struct {
				Properties map[string]struct {
					Enum []string `json:"enum"`
				} `json:"properties"`
			} `json:"properties"`
		}
		s.decodeSchema(tools[schema].InputSchema, &decoded)
		s.Equal([]string{"none", "bearer", "basic", "api_key"},
			decoded.Properties["authentication"].Properties["type"].Enum, schema)
	}

	// An agent learns the retention rule and the mask refusal from the description alone.
	update := tools["update_http_connection"].Description
	s.Contains(update, "Omit the password, token or API key value to keep the stored one")
	s.Contains(update, "Sending ****** as the password or token is refused")
	for _, tool := range tools {
		s.NotContains(tool.Description, "—", tool.Name)
		s.NotContains(tool.Description, "--", tool.Name)
	}
}

func (s *HTTPConnectionToolsTestSuite) decodeSchema(schema any, out any) {
	raw, err := json.Marshal(schema)
	s.Require().NoError(err)
	s.Require().NoError(json.Unmarshal(raw, out))
}

func (s *HTTPConnectionToolsTestSuite) TestListReturnsSummariesWithoutSecrets() {
	s.mockHTTP.On("ListHTTPConnections", mock.Anything).
		Return([]httpconnection.HTTPConnection{*s.storedConnection(toolsStoredURL)}, (*tidcommon.ServiceError)(nil))

	result, raw := s.call("list_http_connections", nil)

	var out httpConnectionListMCPResponse
	s.structured(result, &out)
	s.Equal(httpConnectionListMCPResponse{TotalResults: 1, Connections: []httpConnectionMCPItem{
		{ID: "http-1", Name: "Risk", Description: "Risk engine"},
	}}, out)
	s.NotContains(raw, testHTTPConnectionPassword)
}

func (s *HTTPConnectionToolsTestSuite) TestListError() {
	s.mockHTTP.On("ListHTTPConnections", mock.Anything).
		Return([]httpconnection.HTTPConnection(nil), &tidcommon.InternalServerError)

	result, _ := s.call("list_http_connections", nil)

	s.Contains(s.errorText(result), "failed to list HTTP connections")
}

func (s *HTTPConnectionToolsTestSuite) TestGetMasksThePassword() {
	s.mockHTTP.On("GetHTTPConnection", mock.Anything, "http-1").
		Return(s.storedConnection(toolsStoredURL), (*tidcommon.ServiceError)(nil))

	result, raw := s.call("get_http_connection_by_id", map[string]any{"id": "http-1"})

	var out httpConnectionMCPOutput
	s.structured(result, &out)
	s.Equal(httpConnectionMCPOutput{
		ID: "http-1", Name: "Risk", Description: "Risk engine", Type: httpVendorName, URL: toolsStoredURL,
		TimeoutMS: 5000, Headers: []httpConnectionMCPHeader{{Name: "X-Tenant", Value: "acme"}},
		Authentication: httpConnectionMCPAuthentication{Type: string(outboundauth.TypeBasic),
			Properties: map[string]string{outboundauth.FieldBasicUsername: "svc-user",
				outboundauth.FieldBasicPassword: maskedSecretValue}},
	}, out)
	s.NotContains(raw, testHTTPConnectionPassword)
}

func (s *HTTPConnectionToolsTestSuite) TestGetNotFound() {
	s.mockHTTP.On("GetHTTPConnection", mock.Anything, "missing").
		Return((*httpconnection.HTTPConnection)(nil), &httpconnection.ErrorNotFound)

	result, _ := s.call("get_http_connection_by_id", map[string]any{"id": "missing"})

	text := s.errorText(result)
	s.Contains(text, "failed to get HTTP connection")
	s.Contains(text, httpconnection.ErrorNotFound.ErrorDescription.DefaultValue)
}

func (s *HTTPConnectionToolsTestSuite) TestCreatePassesThePasswordAndMasksTheOutput() {
	var received httpconnection.HTTPConnection
	s.mockHTTP.On("CreateHTTPConnection", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { received = args.Get(1).(httpconnection.HTTPConnection) }).
		Return(s.storedConnection(toolsStoredURL), (*tidcommon.ServiceError)(nil))

	result, raw := s.call("create_http_connection", map[string]any{
		"name": "Risk", "description": "Risk engine", "ouId": "ou-1", "url": toolsStoredURL, "timeoutMs": 5000,
		"headers": []any{map[string]any{"name": "X-Tenant", "value": "acme"}},
		"authentication": map[string]any{"type": "basic", "properties": map[string]any{
			"username": "svc-user", "password": testHTTPConnectionPassword}},
	})

	var out httpConnectionMCPOutput
	s.structured(result, &out)
	s.Equal("http-1", out.ID)
	s.Equal(maskedSecretValue, out.Authentication.Properties[outboundauth.FieldBasicPassword])
	s.NotContains(raw, testHTTPConnectionPassword)
	s.Equal(toolsStoredURL, received.URL)
	s.Equal(5000, received.TimeoutMS)
	s.Equal([]httpconnection.Header{{Name: "X-Tenant", Value: "acme"}}, received.Headers)
	s.Equal(testHTTPConnectionPassword, s.receivedPassword(&received))
}

func (s *HTTPConnectionToolsTestSuite) TestCreateRejectsAnUnsupportedAuthenticationType() {
	result, _ := s.call("create_http_connection", map[string]any{
		"name": "Risk", "ouId": "ou-1", "url": toolsStoredURL, "authentication": map[string]any{"type": "digest"},
	})

	s.True(result.IsError)
	s.mockHTTP.AssertNotCalled(s.T(), "CreateHTTPConnection", mock.Anything, mock.Anything)
}

func (s *HTTPConnectionToolsTestSuite) TestCreateMasksBearerAndAPIKeySecrets() {
	for name, auth := range map[string]map[string]any{
		"bearer":  {"type": "bearer", "properties": map[string]any{"token": testHTTPConnectionPassword}},
		"api_key": {"type": "api_key", "properties": map[string]any{"X-Tenant-Key": testHTTPConnectionPassword}},
	} {
		s.Run(name, func() {
			s.SetupTest()
			s.mockHTTP.On("CreateHTTPConnection", mock.Anything, mock.Anything).
				Return(func(_ context.Context, connection httpconnection.HTTPConnection) (
					*httpconnection.HTTPConnection, *tidcommon.ServiceError) {
					connection.ID = "http-1"
					return &connection, nil
				})

			result, raw := s.call("create_http_connection", map[string]any{
				"name": "Risk", "ouId": "ou-1", "url": toolsStoredURL, "authentication": auth})

			var out httpConnectionMCPOutput
			s.structured(result, &out)
			s.Equal(name, out.Authentication.Type)
			s.NotContains(raw, testHTTPConnectionPassword)
			for _, value := range out.Authentication.Properties {
				s.Equal(maskedSecretValue, value)
			}
		})
	}
}

func (s *HTTPConnectionToolsTestSuite) TestCreateReturnsTheValidationError() {
	s.mockHTTP.On("CreateHTTPConnection", mock.Anything, mock.Anything).
		Return((*httpconnection.HTTPConnection)(nil), &httpconnection.ErrorInvalidURL)

	result, _ := s.call("create_http_connection",
		map[string]any{"name": "Risk", "ouId": "ou-1", "url": "ftp://x"})

	text := s.errorText(result)
	s.Contains(text, "failed to create HTTP connection")
	s.Contains(text, httpconnection.ErrorInvalidURL.ErrorDescription.DefaultValue)
}

func (s *HTTPConnectionToolsTestSuite) TestCreateMapsAnUnsupportedTypeToTheAPIError() {
	_, svcErr := httpMCPInputToDTO(httpConnectionMCPInput{
		Name: "Risk", URL: toolsStoredURL,
		Authentication: &httpConnectionMCPAuthentication{Type: "digest"},
	}, httpToDTO)

	s.Require().NotNil(svcErr)
	s.Equal(ErrorInvalidAuthenticationType.Code, svcErr.Code)
}

func (s *HTTPConnectionToolsTestSuite) TestUpdateMasksTheOutput() {
	received := s.captureUpdate(toolsStoredURL)

	result, raw := s.call("update_http_connection", basicUpdateArgs(toolsStoredURL,
		map[string]any{"username": "svc-user", "password": "new-password"}))

	var out httpConnectionMCPOutput
	s.structured(result, &out)
	s.Equal(maskedSecretValue, out.Authentication.Properties[outboundauth.FieldBasicPassword])
	s.NotContains(raw, testHTTPConnectionPassword)
	s.NotContains(raw, "new-password")
	s.Equal("new-password", s.receivedPassword(received))
}

func (s *HTTPConnectionToolsTestSuite) TestUpdateKeepsTheStoredPasswordForTheSameOrigin() {
	received := s.captureUpdate(toolsStoredURL)

	result, _ := s.call("update_http_connection", basicUpdateArgs(
		"https://risk.example.com/v2/evaluate", map[string]any{"username": "svc-user"}))

	s.False(result.IsError)
	s.Equal(testHTTPConnectionPassword, s.receivedPassword(received))
}

func (s *HTTPConnectionToolsTestSuite) TestUpdateRefusesAnOmittedPasswordForAnotherOrigin() {
	s.mockHTTP.On("GetHTTPConnection", mock.Anything, "http-1").
		Return(s.storedConnection(toolsStoredURL), (*tidcommon.ServiceError)(nil))

	result, _ := s.call("update_http_connection", basicUpdateArgs(
		"https://attacker.example.net/evaluate", map[string]any{"username": "svc-user"}))

	s.Contains(s.errorText(result), httpconnection.ErrorInvalidAuthentication.ErrorDescription.DefaultValue)
	s.mockHTTP.AssertNotCalled(s.T(), "UpdateHTTPConnection", mock.Anything, mock.Anything, mock.Anything)
}

// The reviewer's case: an agent reads a connection and sends the object back, mask included, as the
// update workflow tells it to.
// The mask must be refused rather than stored, or read as "keep" (AC11.3).
func (s *HTTPConnectionToolsTestSuite) TestUpdateRefusesTheMaskedPasswordReadBack() {
	s.mockHTTP.On("GetHTTPConnection", mock.Anything, "http-1").
		Return(s.storedConnection(toolsStoredURL), (*tidcommon.ServiceError)(nil))
	got, _ := s.call("get_http_connection_by_id", map[string]any{"id": "http-1"})
	var read map[string]any
	s.structured(got, &read)
	delete(read, "type")

	result, raw := s.call("update_http_connection", read)

	s.Contains(s.errorText(result), httpconnection.ErrorInvalidAuthentication.ErrorDescription.DefaultValue)
	s.NotContains(raw, testHTTPConnectionPassword)
	s.mockHTTP.AssertNotCalled(s.T(), "UpdateHTTPConnection", mock.Anything, mock.Anything, mock.Anything)
}

func (s *HTTPConnectionToolsTestSuite) TestUpdateNotFound() {
	s.mockHTTP.On("GetHTTPConnection", mock.Anything, "missing").
		Return((*httpconnection.HTTPConnection)(nil), &httpconnection.ErrorNotFound)

	args := basicUpdateArgs(toolsStoredURL, map[string]any{"username": "svc-user"})
	args["id"] = "missing"
	result, _ := s.call("update_http_connection", args)

	text := s.errorText(result)
	s.Contains(text, "failed to update HTTP connection")
	s.Contains(text, httpconnection.ErrorNotFound.ErrorDescription.DefaultValue)
	s.mockHTTP.AssertNotCalled(s.T(), "UpdateHTTPConnection", mock.Anything, mock.Anything, mock.Anything)
}
