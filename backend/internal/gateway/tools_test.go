// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/variablestore"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// The version and store services the tools call are generated mocks, each test setting what it expects.
var (
	appliedAt  = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	devApplied = &AppliedVersion{GatewayID: "dev", Applied: "bbb", Previous: "aaa", AppliedAt: &appliedAt,
		AppliedBy: &Actor{ID: "u1", Name: "Alice"}}
)

type GatewayToolsTestSuite struct {
	suite.Suite
	versions *VersionServiceInterfaceMock
	stores   *StoreServiceInterfaceMock
	applied  *AppliedConfigurationServiceInterfaceMock
	gateways []Gateway
	session  *mcp.ClientSession
}

func TestGatewayToolsTestSuite(t *testing.T) {
	suite.Run(t, new(GatewayToolsTestSuite))
}

// SetupTest connects an in-memory MCP client to the tools, so every call crosses the SDK's schema
// validation and output encoding as an agent's would.
func (s *GatewayToolsTestSuite) SetupTest() {
	s.versions = NewVersionServiceInterfaceMock(s.T())
	s.stores = NewStoreServiceInterfaceMock(s.T())
	s.applied = NewAppliedConfigurationServiceInterfaceMock(s.T())
	s.gateways = []Gateway{
		{ID: "dev", Name: "Development", BaseURL: "https://dev.test", IsDefault: true},
		{ID: "prod", Name: "Production", BaseURL: "https://prod.test"},
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)
	registerMCPTools(server, &gatewayTools{
		gateways: &stubService{listed: s.gateways}, versions: s.versions, applied: s.applied, stores: s.stores,
	})
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

// call invokes a tool and returns its structured answer as JSON.
func (s *GatewayToolsTestSuite) call(name string, args map[string]any) string {
	result := s.result(name, args)
	s.Require().False(result.IsError, "%s failed: %v", name, result.Content)
	raw, err := json.Marshal(result.StructuredContent)
	s.Require().NoError(err)
	return string(raw)
}

func (s *GatewayToolsTestSuite) result(name string, args map[string]any) *mcp.CallToolResult {
	result, err := s.session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	s.Require().NoError(err)
	return result
}

// restJSON is what a REST endpoint answers with for the value.
func (s *GatewayToolsTestSuite) restJSON(value any) string {
	raw, err := json.Marshal(value)
	s.Require().NoError(err)
	return string(raw)
}

func (s *GatewayToolsTestSuite) TestToolsAreListedWithTheirHints() {
	listed, err := s.session.ListTools(context.Background(), nil)
	s.Require().NoError(err)
	byName := map[string]*mcp.Tool{}
	for _, tool := range listed.Tools {
		byName[tool.Name] = tool
	}
	for _, name := range []string{"list_gateways", "get_gateway_applied_version",
		"get_gateway_configuration", "list_configuration_versions",
		"get_configuration_version", "diff_version", "list_gateway_variables",
		"list_gateway_secrets"} {
		s.Require().Contains(byName, name)
		s.True(byName[name].Annotations.ReadOnlyHint, name)
	}
	for _, name := range []string{"deploy_version", "promote_gateway",
		"rollback_gateway", "delete_gateway_variable", "delete_gateway_secret"} {
		s.Require().Contains(byName, name)
		s.Require().NotNil(byName[name].Annotations.DestructiveHint, name)
		s.True(*byName[name].Annotations.DestructiveHint, name)
	}
	for _, name := range []string{"capture_configuration_version", "set_gateway_variable",
		"set_gateway_secret"} {
		s.Contains(byName, name)
	}
}

// Each tool answers with its REST endpoint's body; a list is given under a key.
func (s *GatewayToolsTestSuite) TestAnswersAreTheRESTBodies() {
	versions := []Version{{Seq: 2, Hash: "bbb", Name: "release", Current: true}, {Seq: 1, Hash: "aaa"}}
	version := &Version{Seq: 2, Hash: "bbb", Resources: "resource_type: application\nid: app-a\n",
		References: []ValueReference{{Name: "APPLICATION_A_CLIENT_ID", Kind: ReferenceVariable}}}
	captured := &Version{Seq: 3, Hash: "ccc", Name: "rc1"}
	diff := &Diff{ToVersion: "bbb", Summary: DiffSummary{Added: 1}, Changes: []Change{
		{Key: "application/app-a", ResourceType: "application", ID: "app-a", Change: ChangeAdded,
			Lines: []LineOp{{Kind: "+", Text: "id: app-a"}}},
	}}
	s.versions.EXPECT().GetApplied(mock.Anything, "dev").Return(devApplied, nil)
	s.versions.EXPECT().ListVersions(mock.Anything).Return(versions, nil)
	s.versions.EXPECT().GetVersion(mock.Anything, "bbb").Return(version, nil)
	s.versions.EXPECT().Capture(mock.Anything, CaptureRequest{Name: "rc1"}).Return(captured, nil)
	s.versions.EXPECT().Diff(mock.Anything, "dev", "bbb").Return(diff, nil)
	configuration := &AppliedConfiguration{GatewayID: "dev", Version: "bbb", Resources: []AppliedResource{
		{ResourceType: "application", ID: "app-a", Resource: map[string]any{"name": "App A"}},
	}}
	s.applied.EXPECT().GetAppliedConfiguration(mock.Anything, "dev").Return(configuration, nil)

	s.JSONEq(s.restJSON(map[string]any{"gateways": s.gateways}), s.call("list_gateways", nil))
	s.JSONEq(s.restJSON(devApplied),
		s.call("get_gateway_applied_version", map[string]any{"gatewayId": "dev"}))

	s.JSONEq(s.restJSON(configuration),
		s.call("get_gateway_configuration", map[string]any{"gatewayId": "dev"}))

	s.JSONEq(s.restJSON(map[string]any{"versions": versions}), s.call("list_configuration_versions", nil))
	s.JSONEq(s.restJSON(version), s.call("get_configuration_version", map[string]any{"version": "bbb"}))
	s.JSONEq(s.restJSON(captured), s.call("capture_configuration_version", map[string]any{"name": "rc1"}))
	s.JSONEq(s.restJSON(diff),
		s.call("diff_version", map[string]any{"gatewayId": "dev", "version": "bbb"}))
}

// A deploy passes its options on, and answers with the apply's result as REST does: the gateway's
// import included, which names what it refused.
func (s *GatewayToolsTestSuite) TestDeployAnswersWithTheApplyResult() {
	result := &ApplyResult{
		GatewayID: "prod", Recorded: true,
		Diff: Diff{ToVersion: "bbb", Summary: DiffSummary{Added: 1}, Changes: []Change{
			{Key: "agent/a1", ResourceType: "agent", ID: "a1", Name: "Bot", Change: ChangeAdded},
		}},
		Import: json.RawMessage(`{"summary":{"failed":1},"results":[{"resourceType":"agent","resourceId":"a1",` +
			`"operation":"create","status":"failed","code":"AGT-1039","message":"Owner not found"}]}`),
		Missing: &MissingValues{Variables: []string{"AGENT_BOT_OWNER"}},
	}
	selection := []string{"agent/a1"}
	s.versions.EXPECT().Apply(mock.Anything, "prod", ApplyRequest{Version: "bbb", Selection: &selection,
		ConfirmRemovals: true}).Return(result, nil).Once()

	answer := s.call("deploy_version", map[string]any{
		"gatewayId": "prod", "version": "bbb", "selection": []string{"agent/a1"}, "confirmRemovals": true,
	})

	s.JSONEq(s.restJSON(result), answer)
}

// A capture can keep the configuration as the current version, and a list of a gateway's values can be
// filtered, as the REST endpoints allow.
func (s *GatewayToolsTestSuite) TestCurrentCaptureAndFilteredValuesPassThrough() {
	captured := &Version{Seq: 3, Hash: "ccc", Current: true}
	s.versions.EXPECT().Capture(mock.Anything, CaptureRequest{Current: true}).Return(captured, nil).Once()
	query := StoreListQuery{Limit: "5", Filter: `name sw "APP_"`}
	s.stores.EXPECT().ListVariables(mock.Anything, "dev", query).Return(
		&StoreAnswer[variablestore.VariableListResponse]{Value: &variablestore.VariableListResponse{}}, nil).Once()
	s.stores.EXPECT().ListSecrets(mock.Anything, "dev", query).Return(
		&StoreAnswer[variablestore.SecretListResponse]{Value: &variablestore.SecretListResponse{}}, nil).Once()

	s.JSONEq(s.restJSON(captured), s.call("capture_configuration_version", map[string]any{"current": true}))
	args := map[string]any{"gatewayId": "dev", "limit": 5, "filter": `name sw "APP_"`}
	s.call("list_gateway_variables", args)
	s.call("list_gateway_secrets", args)
}

func (s *GatewayToolsTestSuite) TestAServiceErrorIsReportedWithItsCode() {
	s.versions.EXPECT().Apply(mock.Anything, "prod", ApplyRequest{}).Return(nil, &tidcommon.ServiceError{
		Code: "GTW-1010", Error: tidcommon.I18nMessage{DefaultValue: "Values missing"}})

	result := s.result("deploy_version", map[string]any{"gatewayId": "prod"})

	s.True(result.IsError)
	raw, err := json.Marshal(result.Content)
	s.Require().NoError(err)
	s.Contains(string(raw), "GTW-1010")
}

// A promotion is the version service's, asked for as the tool was called.
func (s *GatewayToolsTestSuite) TestPromote() {
	s.versions.EXPECT().Promote(mock.Anything, PromoteRequest{SourceGatewayID: "dev", TargetGatewayID: "prod",
		DryRun: true, ConfirmRemovals: true}).Return(&ApplyResult{GatewayID: "prod", DryRun: true}, nil).Once()
	s.versions.EXPECT().Promote(mock.Anything, PromoteRequest{SourceGatewayID: "dev", TargetGatewayID: "dev"}).
		Return(nil, &ErrorPromoteToItself).Once()

	answer := s.call("promote_gateway", map[string]any{"sourceGatewayId": "dev", "targetGatewayId": "prod",
		"dryRun": true, "confirmRemovals": true})

	s.JSONEq(s.restJSON(&ApplyResult{GatewayID: "prod", DryRun: true}), answer)
	result := s.result("promote_gateway", map[string]any{"sourceGatewayId": "dev", "targetGatewayId": "dev"})
	s.True(result.IsError)
	raw, err := json.Marshal(result.Content)
	s.Require().NoError(err)
	s.Contains(string(raw), ErrorPromoteToItself.Code)
}

func (s *GatewayToolsTestSuite) TestRollback() {
	s.versions.EXPECT().Revert(mock.Anything, "dev", RevertRequest{}).Return(
		&ApplyResult{GatewayID: "dev", Recorded: true}, nil).Once()

	answer := s.call("rollback_gateway", map[string]any{"gatewayId": "dev"})

	s.JSONEq(s.restJSON(&ApplyResult{GatewayID: "dev", Recorded: true}), answer)
}

// A gateway's values are listed, set and removed as REST does, and a refusal by the gateway's store is
// reported with its status and body.
func (s *GatewayToolsTestSuite) TestGatewayValues() {
	s.stores.EXPECT().ListVariables(mock.Anything, "dev", mock.Anything).Return(
		&StoreAnswer[variablestore.VariableListResponse]{Value: &variablestore.VariableListResponse{
			TotalResults: 1, Count: 1,
			Variables: []variablestore.Variable{{Name: "APP_URL", Value: "https://app.test"}},
		}}, nil)
	s.stores.EXPECT().SetVariable(mock.Anything, "dev", "APP_OWNER",
		variablestore.VariableUpdateRequest{Value: "user-1"}).Return(
		&StoreAnswer[variablestore.Variable]{Value: &variablestore.Variable{Name: "APP_OWNER", Value: "user-1"}},
		nil).Once()
	s.stores.EXPECT().DeleteSecret(mock.Anything, "dev", "S").Return(&StoreAnswer[struct{}]{}, nil).Once()
	s.stores.EXPECT().SetVariable(mock.Anything, "dev", "BAD NAME", mock.Anything).Return(
		&StoreAnswer[variablestore.Variable]{Refusal: &StoreRefusal{Status: 400,
			Body: json.RawMessage(`{"code":"VAR-1001"}`)}}, nil).Once()

	s.JSONEq(`{"totalResults":1,"startIndex":0,"count":1,"variables":[{"name":"APP_URL","value":"https://app.test"}],`+
		`"links":null}`, s.call("list_gateway_variables", map[string]any{"gatewayId": "dev"}))
	s.JSONEq(`{"name":"APP_OWNER","value":"user-1"}`, s.call("set_gateway_variable",
		map[string]any{"gatewayId": "dev", "name": "APP_OWNER", "value": "user-1"}))
	s.JSONEq(`{}`, s.call("delete_gateway_secret", map[string]any{"gatewayId": "dev", "name": "S"}))

	result := s.result("set_gateway_variable",
		map[string]any{"gatewayId": "dev", "name": "BAD NAME", "value": "x"})
	s.True(result.IsError)
	raw, _ := json.Marshal(result.Content)
	s.Contains(string(raw), "VAR-1001")
}
