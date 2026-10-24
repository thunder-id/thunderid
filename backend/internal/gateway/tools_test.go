// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// scriptedVersions answers what each gateway runs and what an apply to it reports, and records the
// applies it was asked for.
type scriptedVersions struct {
	VersionServiceInterface
	applied  map[string]*AppliedVersion
	results  map[string]*ApplyResult
	applies  []appliedCall
	reverted []string
	err      *tidcommon.ServiceError
}

type appliedCall struct {
	gatewayID string
	req       ApplyRequest
}

func (s *scriptedVersions) GetApplied(_ context.Context, id string) (*AppliedVersion, *tidcommon.ServiceError) {
	if applied, ok := s.applied[id]; ok {
		return applied, nil
	}
	return &AppliedVersion{GatewayID: id}, nil
}

func (s *scriptedVersions) Apply(_ context.Context, id string, req ApplyRequest) (*ApplyResult,
	*tidcommon.ServiceError) {
	s.applies = append(s.applies, appliedCall{gatewayID: id, req: req})
	if s.err != nil {
		return nil, s.err
	}
	if result, ok := s.results[id]; ok {
		return result, nil
	}
	return &ApplyResult{GatewayID: id, DryRun: req.DryRun}, nil
}

func (s *scriptedVersions) Revert(_ context.Context, id string, req RevertRequest) (*ApplyResult,
	*tidcommon.ServiceError) {
	s.reverted = append(s.reverted, id)
	return &ApplyResult{GatewayID: id, DryRun: req.DryRun, Recorded: !req.DryRun}, nil
}

func (s *scriptedVersions) ListVersions(context.Context) ([]Version, *tidcommon.ServiceError) {
	return []Version{{Seq: 2, Hash: "bbb", Name: "release", CreatedBy: &Actor{ID: "u1", Name: "Alice"},
		Current: true}, {Seq: 1, Hash: "aaa"}}, nil
}

func (s *scriptedVersions) Capture(_ context.Context, req CaptureRequest) (*Version, *tidcommon.ServiceError) {
	return &Version{Seq: 3, Hash: "ccc", Name: req.Name, Note: req.Note}, nil
}

type GatewayToolsTestSuite struct {
	suite.Suite
	versions *scriptedVersions
	session  *mcp.ClientSession
}

func TestGatewayToolsTestSuite(t *testing.T) {
	suite.Run(t, new(GatewayToolsTestSuite))
}

// SetupTest connects an in-memory MCP client to the tools, so every call crosses the SDK's schema
// validation and output encoding as an agent's would.
func (s *GatewayToolsTestSuite) SetupTest() {
	appliedAt := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	s.versions = &scriptedVersions{
		applied: map[string]*AppliedVersion{
			"dev": {GatewayID: "dev", Applied: "bbb", Previous: "aaa", AppliedAt: &appliedAt,
				AppliedBy: &Actor{ID: "u1", Name: "Alice"}},
		},
		results: map[string]*ApplyResult{},
	}
	gateways := &stubService{listed: []Gateway{
		{ID: "dev", Name: "Development", BaseURL: "https://dev.test", IsDefault: true},
		{ID: "prod", Name: "Production", BaseURL: "https://prod.test"},
	}}
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)
	registerMCPTools(server, gateways, s.versions)
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

// call invokes a tool and decodes its structured output into out.
func (s *GatewayToolsTestSuite) call(name string, args map[string]any, out any) *mcp.CallToolResult {
	result, err := s.session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	s.Require().NoError(err)
	if out != nil && !result.IsError {
		raw, err := json.Marshal(result.StructuredContent)
		s.Require().NoError(err)
		s.Require().NoError(json.Unmarshal(raw, out))
	}
	return result
}

func (s *GatewayToolsTestSuite) TestToolsAreListedWithTheirHints() {
	listed, err := s.session.ListTools(context.Background(), nil)
	s.Require().NoError(err)
	byName := map[string]*mcp.Tool{}
	for _, tool := range listed.Tools {
		byName[tool.Name] = tool
	}
	for _, name := range []string{"thunderid_list_gateways", "thunderid_list_configuration_versions"} {
		s.Require().Contains(byName, name)
		s.True(byName[name].Annotations.ReadOnlyHint, name)
	}
	for _, name := range []string{"thunderid_deploy_version", "thunderid_promote_gateway",
		"thunderid_rollback_gateway"} {
		s.Require().Contains(byName, name)
		s.Require().NotNil(byName[name].Annotations.DestructiveHint, name)
		s.True(*byName[name].Annotations.DestructiveHint, name)
	}
	s.Contains(byName, "thunderid_capture_configuration_version")
}

func (s *GatewayToolsTestSuite) TestListGatewaysShowsWhatEachRuns() {
	var out gatewayListOutput
	s.call("thunderid_list_gateways", nil, &out)

	s.Require().Len(out.Gateways, 2)
	s.Equal("bbb", out.Gateways[0].AppliedVersion)
	s.Equal("aaa", out.Gateways[0].PreviousVersion)
	s.Equal("Alice", out.Gateways[0].AppliedBy)
	s.Empty(out.Gateways[1].AppliedVersion)
}

func (s *GatewayToolsTestSuite) TestListAndCaptureVersions() {
	var listed versionListOutput
	s.call("thunderid_list_configuration_versions", nil, &listed)
	s.Require().Len(listed.Versions, 2)
	s.Equal("bbb", listed.Versions[0].Version)
	s.Equal("Alice", listed.Versions[0].CreatedBy)
	s.True(listed.Versions[0].Current)

	var captured versionOutput
	s.call("thunderid_capture_configuration_version", map[string]any{"name": "rc1", "note": "for QA"}, &captured)
	s.Equal("ccc", captured.Version)
	s.Equal("rc1", captured.Name)
}

// A deploy reports what the gateway refused, which the rest of the version was applied without.
func (s *GatewayToolsTestSuite) TestDeployReportsRefusedResourcesAndMissingValues() {
	s.versions.results["prod"] = &ApplyResult{
		GatewayID: "prod", Recorded: true,
		Diff: Diff{ToVersion: "bbb", Summary: DiffSummary{Added: 1, Unchanged: 1}, Changes: []Change{
			{Key: "agent/a1", ResourceType: "agent", ID: "a1", Name: "Bot", Change: ChangeAdded},
			{Key: "application/x", ResourceType: "application", ID: "x", Change: ChangeUnchanged},
		}},
		Import: json.RawMessage(`{"summary":{"failed":1},"results":[` +
			`{"resourceType":"agent","resourceId":"a1","operation":"create","status":"failed",` +
			`"code":"AGT-1039","message":"Owner not found"},` +
			`{"resourceType":"application","resourceId":"x","status":"success"}]}`),
		Missing: &MissingValues{Variables: []string{"AGENT_BOT_OWNER"}},
	}

	var out deployOutput
	s.call("thunderid_deploy_version", map[string]any{"gatewayId": "prod", "version": "bbb"}, &out)

	s.Require().Len(s.versions.applies, 1)
	s.Equal(ApplyRequest{Version: "bbb"}, s.versions.applies[0].req)
	s.True(out.Recorded)
	s.Require().Len(out.Changes, 1, "an unchanged resource is left out")
	s.Equal("Bot", out.Changes[0].Name)
	s.Require().Len(out.Refused, 1)
	s.Equal("AGT-1039", out.Refused[0].Code)
	s.Equal([]string{"AGENT_BOT_OWNER"}, out.MissingVariables)
}

func (s *GatewayToolsTestSuite) TestDeployReportsAServiceError() {
	s.versions.err = &tidcommon.ServiceError{Code: "GTW-1010",
		Error: tidcommon.I18nMessage{DefaultValue: "Values missing"}}

	result := s.call("thunderid_deploy_version", map[string]any{"gatewayId": "prod"}, nil)

	s.True(result.IsError)
	raw, err := json.Marshal(result.Content)
	s.Require().NoError(err)
	s.Contains(string(raw), "GTW-1010")
}

// A promotion applies what the source runs, leaving out what the source holds back.
func (s *GatewayToolsTestSuite) TestPromoteSkipsWhatTheSourceHoldsBack() {
	s.versions.results["dev"] = &ApplyResult{GatewayID: "dev", DryRun: true, Diff: Diff{Changes: []Change{
		{Key: "role/r1", Change: ChangeAdded, Excluded: true},
		{Key: "role/r2", Change: ChangeUnchanged, Excluded: true},
		{Key: "role/r3", Change: ChangeUpdated},
	}}}

	var out deployOutput
	s.call("thunderid_promote_gateway",
		map[string]any{"sourceGatewayId": "dev", "targetGatewayId": "prod", "dryRun": true}, &out)

	s.Require().Len(s.versions.applies, 2)
	s.Equal(appliedCall{gatewayID: "dev", req: ApplyRequest{Version: "bbb", DryRun: true}}, s.versions.applies[0])
	s.Equal(appliedCall{gatewayID: "prod", req: ApplyRequest{Version: "bbb", DryRun: true,
		Skip: []string{"role/r1"}}}, s.versions.applies[1])
	s.True(out.DryRun)
}

func (s *GatewayToolsTestSuite) TestPromoteRefusesASourceThatRunsNothing() {
	result := s.call("thunderid_promote_gateway",
		map[string]any{"sourceGatewayId": "prod", "targetGatewayId": "dev"}, nil)

	s.True(result.IsError)
	s.Empty(s.versions.applies)
}

func (s *GatewayToolsTestSuite) TestPromoteRefusesTheSameGateway() {
	result := s.call("thunderid_promote_gateway",
		map[string]any{"sourceGatewayId": "dev", "targetGatewayId": "dev"}, nil)

	s.True(result.IsError)
	s.Empty(s.versions.applies)
}

func (s *GatewayToolsTestSuite) TestRollback() {
	var out deployOutput
	s.call("thunderid_rollback_gateway", map[string]any{"gatewayId": "dev"}, &out)

	s.Equal([]string{"dev"}, s.versions.reverted)
	s.True(out.Recorded)
}

func TestRefusedOfIgnoresAnAnswerItCannotRead(t *testing.T) {
	require.Nil(t, refusedOf(nil))
	require.Nil(t, refusedOf(json.RawMessage(`not json`)))
}
