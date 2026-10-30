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

type stubApplied struct{}

func (stubApplied) GetAppliedConfiguration(_ context.Context, id string) (*AppliedConfiguration,
	*tidcommon.ServiceError) {
	return &AppliedConfiguration{GatewayID: id, Version: "bbb", Resources: []AppliedResource{
		{ResourceType: "application", ID: "app-a", Resource: map[string]any{"name": "App A"}},
	}}, nil
}

type GatewayToolsTestSuite struct {
	suite.Suite
	versions *VersionServiceInterfaceMock
	stores   *StoreServiceInterfaceMock
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
	s.gateways = []Gateway{
		{ID: "dev", Name: "Development", BaseURL: "https://dev.test", IsDefault: true},
		{ID: "prod", Name: "Production", BaseURL: "https://prod.test"},
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)
	registerMCPTools(server, &gatewayTools{
		gateways: &stubService{listed: s.gateways}, versions: s.versions, applied: stubApplied{}, stores: s.stores,
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
	for _, name := range []string{"thunderid_list_gateways", "thunderid_get_gateway_applied_version",
		"thunderid_get_gateway_configuration", "thunderid_list_configuration_versions",
		"thunderid_get_configuration_version", "thunderid_diff_version", "thunderid_list_gateway_variables",
		"thunderid_list_gateway_secrets"} {
		s.Require().Contains(byName, name)
		s.True(byName[name].Annotations.ReadOnlyHint, name)
	}
	for _, name := range []string{"thunderid_deploy_version", "thunderid_promote_gateway",
		"thunderid_rollback_gateway", "thunderid_delete_gateway_variable", "thunderid_delete_gateway_secret"} {
		s.Require().Contains(byName, name)
		s.Require().NotNil(byName[name].Annotations.DestructiveHint, name)
		s.True(*byName[name].Annotations.DestructiveHint, name)
	}
	for _, name := range []string{"thunderid_capture_configuration_version", "thunderid_set_gateway_variable",
		"thunderid_set_gateway_secret"} {
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

	s.JSONEq(s.restJSON(map[string]any{"gateways": s.gateways}), s.call("thunderid_list_gateways", nil))
	s.JSONEq(s.restJSON(devApplied),
		s.call("thunderid_get_gateway_applied_version", map[string]any{"gatewayId": "dev"}))

	configuration, _ := stubApplied{}.GetAppliedConfiguration(context.Background(), "dev")
	s.JSONEq(s.restJSON(configuration),
		s.call("thunderid_get_gateway_configuration", map[string]any{"gatewayId": "dev"}))

	s.JSONEq(s.restJSON(map[string]any{"versions": versions}), s.call("thunderid_list_configuration_versions", nil))
	s.JSONEq(s.restJSON(version), s.call("thunderid_get_configuration_version", map[string]any{"version": "bbb"}))
	s.JSONEq(s.restJSON(captured), s.call("thunderid_capture_configuration_version", map[string]any{"name": "rc1"}))
	s.JSONEq(s.restJSON(diff),
		s.call("thunderid_diff_version", map[string]any{"gatewayId": "dev", "version": "bbb"}))
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
		Skip: []string{"role/r1"}, ConfirmRemovals: true}).Return(result, nil).Once()

	answer := s.call("thunderid_deploy_version", map[string]any{
		"gatewayId": "prod", "version": "bbb", "selection": []string{"agent/a1"}, "skip": []string{"role/r1"},
		"confirmRemovals": true,
	})

	s.JSONEq(s.restJSON(result), answer)
}

func (s *GatewayToolsTestSuite) TestAServiceErrorIsReportedWithItsCode() {
	s.versions.EXPECT().Apply(mock.Anything, "prod", ApplyRequest{}).Return(nil, &tidcommon.ServiceError{
		Code: "GTW-1010", Error: tidcommon.I18nMessage{DefaultValue: "Values missing"}})

	result := s.result("thunderid_deploy_version", map[string]any{"gatewayId": "prod"})

	s.True(result.IsError)
	raw, err := json.Marshal(result.Content)
	s.Require().NoError(err)
	s.Contains(string(raw), "GTW-1010")
}

// A promotion applies what the source runs, leaving out what the source holds back.
func (s *GatewayToolsTestSuite) TestPromoteSkipsWhatTheSourceHoldsBack() {
	s.versions.EXPECT().GetApplied(mock.Anything, "dev").Return(devApplied, nil)
	s.versions.EXPECT().Apply(mock.Anything, "dev", ApplyRequest{Version: "bbb", DryRun: true}).Return(
		&ApplyResult{GatewayID: "dev", DryRun: true, Diff: Diff{Changes: []Change{
			{Key: "role/r1", Change: ChangeAdded, Excluded: true},
			{Key: "role/r2", Change: ChangeUnchanged, Excluded: true},
			{Key: "role/r3", Change: ChangeUpdated},
		}}}, nil).Once()
	s.versions.EXPECT().Apply(mock.Anything, "prod", ApplyRequest{Version: "bbb", DryRun: true,
		Skip: []string{"role/r1"}}).Return(&ApplyResult{GatewayID: "prod", DryRun: true}, nil).Once()

	s.call("thunderid_promote_gateway",
		map[string]any{"sourceGatewayId": "dev", "targetGatewayId": "prod", "dryRun": true})
}

// Nothing is applied for a source that runs no version, or one promoted to itself.
func (s *GatewayToolsTestSuite) TestPromoteRefusesASourceThatRunsNothingOrItself() {
	s.versions.EXPECT().GetApplied(mock.Anything, "prod").Return(&AppliedVersion{GatewayID: "prod"}, nil)

	s.True(s.result("thunderid_promote_gateway",
		map[string]any{"sourceGatewayId": "prod", "targetGatewayId": "dev"}).IsError)
	s.True(s.result("thunderid_promote_gateway",
		map[string]any{"sourceGatewayId": "dev", "targetGatewayId": "dev"}).IsError)
	s.versions.AssertNotCalled(s.T(), "Apply", mock.Anything, mock.Anything, mock.Anything)
}

func (s *GatewayToolsTestSuite) TestRollback() {
	s.versions.EXPECT().Revert(mock.Anything, "dev", RevertRequest{}).Return(
		&ApplyResult{GatewayID: "dev", Recorded: true}, nil).Once()

	answer := s.call("thunderid_rollback_gateway", map[string]any{"gatewayId": "dev"})

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
		`"links":null}`, s.call("thunderid_list_gateway_variables", map[string]any{"gatewayId": "dev"}))
	s.JSONEq(`{"name":"APP_OWNER","value":"user-1"}`, s.call("thunderid_set_gateway_variable",
		map[string]any{"gatewayId": "dev", "name": "APP_OWNER", "value": "user-1"}))
	s.JSONEq(`{}`, s.call("thunderid_delete_gateway_secret", map[string]any{"gatewayId": "dev", "name": "S"}))

	result := s.result("thunderid_set_gateway_variable",
		map[string]any{"gatewayId": "dev", "name": "BAD NAME", "value": "x"})
	s.True(result.IsError)
	raw, _ := json.Marshal(result.Content)
	s.Contains(string(raw), "VAR-1001")
}
