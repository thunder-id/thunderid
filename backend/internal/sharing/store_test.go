// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/internal/system/deployment"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

const storeDeploymentID = "test-deployment-id"

// StoreTestSuite covers the SQL layer: that each method sends the query and arguments it should,
// that a row set comes back as a hydrated policy, and that every failure is wrapped rather than
// swallowed.
type StoreTestSuite struct {
	suite.Suite
	provider *providermock.DBProviderInterfaceMock
	client   *providermock.DBClientInterfaceMock
	store    *sharingStore
}

func TestStoreTestSuite(t *testing.T) {
	suite.Run(t, new(StoreTestSuite))
}

func (s *StoreTestSuite) SetupTest() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("", &config.Config{
		Server: engineconfig.ServerConfig{Identifier: storeDeploymentID},
	})
	s.provider = providermock.NewDBProviderInterfaceMock(s.T())
	s.client = providermock.NewDBClientInterfaceMock(s.T())
	s.store = &sharingStore{dbProvider: s.provider}
}

// clientAvailable wires the provider to hand back the mock client.
func (s *StoreTestSuite) clientAvailable() {
	s.provider.On("GetConfigDBClient").Return(s.client, nil)
}

// clientUnavailable wires the provider to fail, which every method has to surface.
func (s *StoreTestSuite) clientUnavailable() error {
	failure := errors.New("db client error")
	s.provider.On("GetConfigDBClient").Return(nil, failure)
	return failure
}

// expectExec expects one write of query with argc bound arguments. The mock unrolls variadic
// arguments, so each one has to be matched in its own right.
func (s *StoreTestSuite) expectExec(query dbmodel.DBQuery, argc int) {
	s.client.On("ExecuteContext", s.anyArgs(query, argc)...).Return(int64(1), nil).Once()
}

// expectExecError is expectExec for the failure of one particular write.
func (s *StoreTestSuite) expectExecError(query dbmodel.DBQuery, argc int, err error) {
	s.client.On("ExecuteContext", s.anyArgs(query, argc)...).Return(int64(0), err).Once()
}

// anyArgs builds the matcher list for a context, a query and argc bound arguments.
func (s *StoreTestSuite) anyArgs(query dbmodel.DBQuery, argc int) []interface{} {
	out := []interface{}{mock.Anything, query}
	for range argc {
		out = append(out, mock.Anything)
	}
	return out
}

// policyRow is what a select on the policy table gives back.
func policyRow(id string) map[string]interface{} {
	return map[string]interface{}{
		"id":               id,
		"resource_type":    string(testType),
		"resource_id":      testResource,
		"owning_ou_id":     ownerOU,
		"initiating_ou_id": rootOU,
		"policy_stage":     string(stageShare),
		"parent_policy_id": "",
		"version":          int64(2),
	}
}

// A numeric column comes back as whatever width the driver decoded, and a driver that decoded it as
// JSON hands back a float. The version has to survive every one of those, because an edit is
// accepted or refused by comparing it.
func (s *StoreTestSuite) TestTheVersionIsReadWhateverWidthTheDriverReturns() {
	for _, tc := range []struct {
		name  string
		value interface{}
		want  int
	}{
		{"int64, as most drivers return it", int64(2), 2},
		{"int", 2, 2},
		{"int32", int32(2), 2},
		{"float64, from a driver that decoded json", float64(2), 2},
		{"a column carrying nothing is the zero a row with no version means", nil, 0},
		{"a string is not parsed into one", "2", 0},
	} {
		s.Run(tc.name, func() {
			row := policyRow("p1")
			row["version"] = tc.value

			s.Equal(tc.want, policyFromRow(row).Version)
		})
	}
}

// expectHydrate answers the three follow-up queries hydrate makes for one policy.
func (s *StoreTestSuite) expectHydrate(policyID string, targets, exclusions, rules []map[string]interface{}) {
	s.client.On("QueryContext", mock.Anything, queryListTargets, policyID, storeDeploymentID).
		Return(targets, nil).Once()
	s.client.On("QueryContext", mock.Anything, queryListExclusions, policyID, storeDeploymentID).
		Return(exclusions, nil).Once()
	s.client.On("QueryContext", mock.Anything, queryListRules, policyID, storeDeploymentID).
		Return(rules, nil).Once()
}

// A policy write is the row plus its targets, exclusions, rules and members, all in one call.
func (s *StoreTestSuite) TestCreatePolicyWritesEveryPart() {
	s.clientAvailable()
	p := Policy{
		ID: "p1", ResourceType: testType, ResourceID: testResource, OwningOUID: ownerOU,
		InitiatingOUID: rootOU, Stage: stageShare, Version: 1,
		Targets: []Target{{
			ID: "t1", Scope: ScopeRoot, OUID: rootOU, ExcludedOUIDs: []string{childOU, childOU},
		}},
		Rules: []StoredRule{{
			FieldKey: "assignments", TargetID: "t1",
			Resolved:  OverlayRule{Editable: true, AllowedValues: members("a", "b")},
			Requested: OverlayRule{Editable: true, AllowedValues: members("a", "b")},
		}},
	}

	s.expectExec(queryCreatePolicy, 9)
	s.expectExec(queryInsertTarget, 5)
	// The duplicate exclusion is deduplicated before it reaches the database.
	s.expectExec(queryInsertExclusion, 4)
	// One insert per rule: both forms of it travel as documents.
	s.expectExec(queryInsertRule, 7)

	s.Require().NoError(s.store.CreatePolicy(context.Background(), p))
}

// Every method needs the client before it can do anything, so a provider failure surfaces from all
// of them rather than from whichever happens to be called first.
func (s *StoreTestSuite) TestEveryMethodSurfacesAClientFailure() {
	failure := s.clientUnavailable()
	ctx := context.Background()

	calls := map[string]func() error{
		"CreatePolicy": func() error { return s.store.CreatePolicy(ctx, Policy{ID: "p1"}) },
		"GetPolicy":    func() error { _, err := s.store.GetPolicy(ctx, "p1"); return err },
		"ListPoliciesForResource": func() error {
			_, err := s.store.ListPoliciesForResource(ctx, testType, testResource, 100, 0)
			return err
		},
		"ListAllPoliciesForResource": func() error {
			_, err := s.store.ListAllPoliciesForResource(ctx, testType, testResource)
			return err
		},
		"CountPoliciesForResource": func() error {
			_, err := s.store.CountPoliciesForResource(ctx, testType, testResource)
			return err
		},

		"ReplacePolicyContents": func() error { return s.store.ReplacePolicyContents(ctx, Policy{ID: "p1"}, 1) },
		"DeletePolicy":          func() error { return s.store.DeletePolicy(ctx, "p1") },
		"GetOverlayValues": func() error {
			_, err := s.store.GetOverlayValues(ctx, testType, testResource, rootOU)
			return err
		},
		"DeleteOverlayValuesForOU": func() error {
			return s.store.DeleteOverlayValuesForOU(ctx, testType, testResource, rootOU)
		},
		"DeleteOverlayValue": func() error {
			return s.store.DeleteOverlayValue(ctx, testType, testResource, rootOU, "f")
		},
		"SetOverlayValue": func() error {
			return s.store.SetOverlayValue(ctx, testType, testResource, rootOU, "f", []string{"a"})
		},
		"GetPolicyByInitiator": func() error {
			_, err := s.store.GetPolicyByInitiator(ctx, testType, testResource, rootOU)
			return err
		},
		"ListPoliciesRelevantToChain": func() error {
			_, err := s.store.ListPoliciesRelevantToChain(ctx, testType, testResource, []string{rootOU})
			return err
		},
	}

	for name, call := range calls {
		s.Run(name, func() {
			err := call()
			s.Require().Error(err)
			s.ErrorIs(err, failure, "the provider's error must reach the caller")
		})
	}
}

// A row set comes back as a policy with its targets, exclusions and rules attached.
func (s *StoreTestSuite) TestGetPolicyHydratesEveryPart() {
	s.clientAvailable()
	s.client.On("QueryContext", mock.Anything, queryGetPolicyByID, "p1", storeDeploymentID).
		Return([]map[string]interface{}{policyRow("p1")}, nil).Once()
	s.expectHydrate("p1",
		[]map[string]interface{}{{"id": "t1", "target_scope": string(ScopeRoot), "target_ou_id": rootOU}},
		[]map[string]interface{}{{"target_id": "t1", "excluded_ou_id": childOU}},
		[]map[string]interface{}{{
			"id": "r1", "field_key": "assignments", "target_id": "t1",
			"resolved":  `{"editable":true,"allowedValues":["a"]}`,
			"requested": `{"editable":true,"allowedValues":["a"]}`,
		}})

	p, err := s.store.GetPolicy(context.Background(), "p1")
	s.Require().NoError(err)

	s.Equal("p1", p.ID)
	s.Equal(stageShare, p.Stage)
	s.Equal(2, p.Version)
	s.Equal([]Target{{
		ID: "t1", Scope: ScopeRoot, OUID: rootOU, ExcludedOUIDs: []string{childOU},
	}}, p.Targets)
	s.Require().Len(p.Rules, 1)
	s.Equal("assignments", p.Rules[0].FieldKey)
	s.Equal([]string{"a"}, *p.Rules[0].Resolved.AllowedValues)
	s.Nil(p.Rules[0].Resolved.Value, "a set flag of false has to come back absent, not empty")
}

// The set flags carry absent-versus-empty, so a flag set with no member rows is an empty list and
// not a missing one.
func (s *StoreTestSuite) TestHydrateRuleSeparatesAbsentFromEmpty() {
	// hydrateRule reads the row it is handed; nothing else is consulted.
	rule, err := hydrateRule(map[string]interface{}{
		"id": "r1", "field_key": "assignments",
		"resolved":  `{"editable":false,"value":[]}`,
		"requested": `{"editable":false}`,
	})
	s.Require().NoError(err)

	s.Require().NotNil(rule.Resolved.Value, "an explicitly empty list was present")
	s.Empty(*rule.Resolved.Value, "present with no members is the empty list")
	s.Nil(rule.Resolved.AllowedValues, "an omitted list comes back absent")
	s.Nil(rule.Requested.Value, "and absence survives on the requested form too")
}

// A missing row is the not-found sentinel, which the service turns into its own error rather than a
// database failure.
func (s *StoreTestSuite) TestGetPolicyReportsNotFound() {
	s.clientAvailable()
	s.client.On("QueryContext", mock.Anything, queryGetPolicyByID, "missing", storeDeploymentID).
		Return([]map[string]interface{}{}, nil).Once()

	_, err := s.store.GetPolicy(context.Background(), "missing")
	s.ErrorIs(err, errPolicyNotFound)
}

// The same sentinel for the by-initiator lookup, which is how the service detects a policy already
// exists for an organization unit.
func (s *StoreTestSuite) TestGetPolicyByInitiatorReportsNotFound() {
	s.clientAvailable()
	s.client.On("QueryContext", mock.Anything, queryGetPolicyByInitiator,
		string(testType), testResource, rootOU, storeDeploymentID).
		Return([]map[string]interface{}{}, nil).Once()

	_, err := s.store.GetPolicyByInitiator(context.Background(), testType, testResource, rootOU)
	s.ErrorIs(err, errPolicyNotFound)
}

// The version check is the optimistic concurrency guard: no row updated means somebody else moved
// the policy, and nothing further may be written.
func (s *StoreTestSuite) TestReplacePolicyContentsRefusesAStaleVersion() {
	s.clientAvailable()
	s.client.On("ExecuteContext", mock.Anything, queryBumpPolicyVersion, "p1", 1, storeDeploymentID).
		Return(int64(0), nil).Once()

	err := s.store.ReplacePolicyContents(context.Background(), Policy{ID: "p1"}, 1)
	s.ErrorIs(err, errPolicyNotFound)
}

// A successful version bump clears the old contents before writing the new ones, or a replace would
// accumulate rather than replace.
func (s *StoreTestSuite) TestReplacePolicyContentsClearsBeforeWriting() {
	s.clientAvailable()
	s.client.On("ExecuteContext", mock.Anything, queryBumpPolicyVersion, "p1", 2, storeDeploymentID).
		Return(int64(1), nil).Once()
	s.expectExec(queryDeleteRules, 2)
	s.expectExec(queryDeleteExclusions, 2)
	s.expectExec(queryDeleteTargets, 2)
	s.expectExec(queryInsertTarget, 5)

	err := s.store.ReplacePolicyContents(context.Background(), Policy{
		ID: "p1", Targets: []Target{{ID: "t1", Scope: ScopeChild, OUID: childOU}},
	}, 2)
	s.Require().NoError(err)
}

// A chain lookup builds its query from the chain length, and the organization unit ids must arrive
// as bound arguments rather than interpolated text.
func (s *StoreTestSuite) TestListPoliciesRelevantToChainBindsTheChain() {
	s.clientAvailable()
	chain := []string{rootOU, childOU, grandOU}
	query, args := buildRelevantPoliciesQuery(testType, testResource, chain, storeDeploymentID)
	s.client.On("QueryContext", append([]interface{}{mock.Anything, query}, args...)...).
		Return([]map[string]interface{}{}, nil).Once()

	got, err := s.store.ListPoliciesRelevantToChain(context.Background(), testType, testResource, chain)
	s.Require().NoError(err)
	s.Empty(got)
	for _, ouID := range chain {
		s.Contains(args, ouID, "each chain member has to be a bound argument")
	}
}

// An overlay value is stored as json, so it has to come back decoded and keyed by field.
func (s *StoreTestSuite) TestOverlayValuesRoundTripThroughJSON() {
	s.clientAvailable()
	s.expectExec(queryUpsertOverlayValue, 6)
	s.client.On("QueryContext", mock.Anything, queryGetOverlayValue,
		string(testType), testResource, rootOU, storeDeploymentID).
		Return([]map[string]interface{}{
			{"field_key": "assignments", "value": `["a","b"]`},
		}, nil).Once()

	ctx := context.Background()
	s.Require().NoError(s.store.SetOverlayValue(ctx, testType, testResource, rootOU, "assignments", []string{"a", "b"}))

	got, err := s.store.GetOverlayValues(ctx, testType, testResource, rootOU)
	s.Require().NoError(err)
	s.Equal(map[string][]string{"assignments": {"a", "b"}}, got)
}

// A value the database cannot decode is reported rather than returned as an empty list, which would
// read as an organization unit holding nothing.
func (s *StoreTestSuite) TestGetOverlayValuesReportsUndecodableJSON() {
	s.clientAvailable()
	s.client.On("QueryContext", mock.Anything, queryGetOverlayValue,
		string(testType), testResource, rootOU, storeDeploymentID).
		Return([]map[string]interface{}{{"field_key": "assignments", "value": "not json"}}, nil).Once()

	_, err := s.store.GetOverlayValues(context.Background(), testType, testResource, rootOU)
	s.Require().Error(err)
	s.Contains(err.Error(), "failed to decode overlay value")
}

// Each write wraps its own failure, so a partly written policy names the part that failed.
func (s *StoreTestSuite) TestWriteFailuresAreWrappedWithTheirPart() {
	failure := errors.New("constraint violation")
	cases := []struct {
		name    string
		arrange func()
		policy  Policy
		wantMsg string
	}{
		{
			name:    "the policy row",
			arrange: func() { s.expectExecError(queryCreatePolicy, 9, failure) },
			policy:  Policy{ID: "p1"},
			wantMsg: "failed to create sharing policy",
		},
		{
			name: "a target",
			arrange: func() {
				s.expectExec(queryCreatePolicy, 9)
				s.expectExecError(queryInsertTarget, 5, failure)
			},
			policy:  Policy{ID: "p1", Targets: []Target{{ID: "t1", Scope: ScopeChild, OUID: childOU}}},
			wantMsg: "failed to create sharing policy target",
		},
		{
			name: "an exclusion",
			arrange: func() {
				s.expectExec(queryCreatePolicy, 9)
				s.expectExec(queryInsertTarget, 5)
				s.expectExecError(queryInsertExclusion, 4, failure)
			},
			policy: Policy{ID: "p1", Targets: []Target{{
				ID: "t1", Scope: ScopeChild, OUID: childOU, ExcludedOUIDs: []string{childOU},
			}}},
			wantMsg: "failed to create sharing policy exclusion",
		},
		{
			name: "an overlay rule",
			arrange: func() {
				s.expectExec(queryCreatePolicy, 9)
				s.expectExecError(queryInsertRule, 7, failure)
			},
			policy:  Policy{ID: "p1", Rules: []StoredRule{{FieldKey: "assignments"}}},
			wantMsg: "failed to create overlay rule",
		},
	}

	for _, tt := range cases {
		s.Run(tt.name, func() {
			s.SetupTest()
			s.clientAvailable()
			tt.arrange()

			err := s.store.CreatePolicy(context.Background(), tt.policy)
			s.Require().Error(err)
			s.Contains(err.Error(), tt.wantMsg)
			s.ErrorIs(err, failure, "the database error has to stay wrapped inside")
		})
	}
}

// A read failure is wrapped too, for each query a read makes.
func (s *StoreTestSuite) TestReadFailuresAreWrapped() {
	failure := errors.New("query error")
	cases := []struct {
		name    string
		arrange func()
		call    func() error
		wantMsg string
		// noWrap marks a failure that is not a database error being passed through.
		noWrap bool
	}{
		{
			name: "the policy row",
			arrange: func() {
				s.client.On("QueryContext", mock.Anything, queryGetPolicyByID, "p1", storeDeploymentID).
					Return(nil, failure).Once()
			},
			call:    func() error { _, err := s.store.GetPolicy(context.Background(), "p1"); return err },
			wantMsg: "failed to get sharing policy",
		},
		{
			name: "its targets",
			arrange: func() {
				s.client.On("QueryContext", mock.Anything, queryGetPolicyByID, "p1", storeDeploymentID).
					Return([]map[string]interface{}{policyRow("p1")}, nil).Once()
				s.client.On("QueryContext", mock.Anything, queryListTargets, "p1", storeDeploymentID).
					Return(nil, failure).Once()
			},
			call:    func() error { _, err := s.store.GetPolicy(context.Background(), "p1"); return err },
			wantMsg: "failed to list sharing policy targets",
		},
		{
			name: "its exclusions",
			arrange: func() {
				s.client.On("QueryContext", mock.Anything, queryGetPolicyByID, "p1", storeDeploymentID).
					Return([]map[string]interface{}{policyRow("p1")}, nil).Once()
				s.client.On("QueryContext", mock.Anything, queryListTargets, "p1", storeDeploymentID).
					Return([]map[string]interface{}{}, nil).Once()
				s.client.On("QueryContext", mock.Anything, queryListExclusions, "p1", storeDeploymentID).
					Return(nil, failure).Once()
			},
			call:    func() error { _, err := s.store.GetPolicy(context.Background(), "p1"); return err },
			wantMsg: "failed to list sharing policy exclusions",
		},
		{
			name: "the resource listing",
			arrange: func() {
				s.client.On("QueryContext", mock.Anything, queryListPoliciesForResource,
					string(testType), testResource, storeDeploymentID, 100, 0).Return(nil, failure).Once()
			},
			call: func() error {
				_, err := s.store.ListPoliciesForResource(context.Background(), testType, testResource, 100, 0)
				return err
			},
			wantMsg: "failed to list sharing policies",
		},
		{
			name: "the whole-set read",
			arrange: func() {
				s.client.On("QueryContext", mock.Anything, queryListAllPoliciesForResource,
					string(testType), testResource, storeDeploymentID).Return(nil, failure).Once()
			},
			call: func() error {
				_, err := s.store.ListAllPoliciesForResource(context.Background(), testType, testResource)
				return err
			},
			wantMsg: "failed to list sharing policies",
		},
		{
			name: "the count behind the listing",
			arrange: func() {
				s.client.On("QueryContext", mock.Anything, queryCountPoliciesForResource,
					string(testType), testResource, storeDeploymentID).Return(nil, failure).Once()
			},
			call: func() error {
				_, err := s.store.CountPoliciesForResource(context.Background(), testType, testResource)
				return err
			},
			wantMsg: "failed to count sharing policies",
		},
	}

	for _, tt := range cases {
		s.Run(tt.name, func() {
			s.SetupTest()
			s.clientAvailable()
			tt.arrange()

			err := tt.call()
			s.Require().Error(err)
			s.Contains(err.Error(), tt.wantMsg)
			if !tt.noWrap {
				s.ErrorIs(err, failure)
			}
		})
	}
}

// Deleting relies on the database cascading a policy's contents, so the store issues one statement.
func (s *StoreTestSuite) TestDeletePolicyIssuesOneStatement() {
	s.clientAvailable()
	s.client.On("ExecuteContext", mock.Anything, queryDeletePolicy, "p1", storeDeploymentID).
		Return(int64(1), nil).Once()

	s.Require().NoError(s.store.DeletePolicy(context.Background(), "p1"))
}

// Clearing an organization unit's values is scoped to that unit and resource, never wider.
func (s *StoreTestSuite) TestOverlayDeletesAreScoped() {
	s.clientAvailable()
	s.client.On("ExecuteContext", mock.Anything, queryDeleteOverlayValue,
		string(testType), testResource, rootOU, "assignments", storeDeploymentID).
		Return(int64(1), nil).Once()
	s.client.On("ExecuteContext", mock.Anything, queryDeleteOverlayValuesForOU,
		string(testType), testResource, rootOU, storeDeploymentID).
		Return(int64(2), nil).Once()

	ctx := context.Background()
	s.Require().NoError(s.store.DeleteOverlayValue(ctx, testType, testResource, rootOU, "assignments"))
	s.Require().NoError(s.store.DeleteOverlayValuesForOU(ctx, testType, testResource, rootOU))
}

// A listing hydrates every row, and one bad row fails the whole listing rather than returning a
// partial set a caller would read as the complete one.
func (s *StoreTestSuite) TestHydrateAllFailsRatherThanReturningPartialResults() {
	s.clientAvailable()
	failure := errors.New("query error")
	s.client.On("QueryContext", mock.Anything, queryListPoliciesForResource,
		string(testType), testResource, storeDeploymentID, 100, 0).
		Return([]map[string]interface{}{policyRow("p1"), policyRow("p2")}, nil).Once()
	// The first row hydrates, the second fails on its targets.
	s.expectHydrate("p1", nil, nil, nil)
	s.client.On("QueryContext", mock.Anything, queryListTargets, "p2", storeDeploymentID).
		Return(nil, failure).Once()

	got, err := s.store.ListPoliciesForResource(context.Background(), testType, testResource, 100, 0)

	s.Require().Error(err)
	s.ErrorIs(err, failure)
	s.Nil(got, "a failed hydration must yield no policies at all")
}

// Two rows both hydrate, in the order the database returned them.
func (s *StoreTestSuite) TestHydrateAllReturnsEveryPolicy() {
	s.clientAvailable()
	s.client.On("QueryContext", mock.Anything, queryListPoliciesForResource,
		string(testType), testResource, storeDeploymentID, 100, 0).
		Return([]map[string]interface{}{policyRow("p1"), policyRow("p2")}, nil).Once()
	s.expectHydrate("p1", nil, nil, nil)
	s.expectHydrate("p2", nil, nil, nil)

	got, err := s.store.ListPoliciesForResource(context.Background(), testType, testResource, 100, 0)

	s.Require().NoError(err)
	s.Require().Len(got, 2)
	s.Equal([]string{"p1", "p2"}, []string{got[0].ID, got[1].ID})
}

// Each clearing statement in a replace wraps its own failure, so a half-cleared policy says which
// statement stopped it.
func (s *StoreTestSuite) TestReplacePolicyContentsWrapsEachClearingFailure() {
	failure := errors.New("constraint violation")
	cases := []struct {
		name    string
		arrange func()
		wantMsg string
	}{
		{
			name:    "the version bump",
			arrange: func() { s.expectExecError(queryBumpPolicyVersion, 3, failure) },
			wantMsg: "failed to update sharing policy version",
		},
		{
			name: "clearing rules",
			arrange: func() {
				s.expectExec(queryBumpPolicyVersion, 3)
				s.expectExecError(queryDeleteRules, 2, failure)
			},
			wantMsg: "failed to clear overlay rules",
		},
		{
			name: "clearing exclusions",
			arrange: func() {
				s.expectExec(queryBumpPolicyVersion, 3)
				s.expectExec(queryDeleteRules, 2)
				s.expectExecError(queryDeleteExclusions, 2, failure)
			},
			wantMsg: "failed to clear sharing policy exclusions",
		},
		{
			name: "clearing targets",
			arrange: func() {
				s.expectExec(queryBumpPolicyVersion, 3)
				s.expectExec(queryDeleteRules, 2)
				s.expectExec(queryDeleteExclusions, 2)
				s.expectExecError(queryDeleteTargets, 2, failure)
			},
			wantMsg: "failed to clear sharing policy targets",
		},
	}

	for _, tt := range cases {
		s.Run(tt.name, func() {
			s.SetupTest()
			s.clientAvailable()
			tt.arrange()

			err := s.store.ReplacePolicyContents(context.Background(), Policy{ID: "p1"}, 2)
			s.Require().Error(err)
			s.Contains(err.Error(), tt.wantMsg)
			s.ErrorIs(err, failure)
		})
	}
}

// Every remaining statement wraps its failure too, each with its own message.
func (s *StoreTestSuite) TestRemainingStatementFailuresAreWrapped() {
	failure := errors.New("statement failed")
	ctx := context.Background()
	cases := []struct {
		name    string
		arrange func()
		call    func() error
		wantMsg string
		// noWrap marks a failure that is not a database error being passed through.
		noWrap bool
	}{
		{
			name:    "deleting a policy",
			arrange: func() { s.expectExecError(queryDeletePolicy, 2, failure) },
			call:    func() error { return s.store.DeletePolicy(ctx, "p1") },
			wantMsg: "failed to delete sharing policy",
		},
		{
			name:    "setting an overlay value",
			arrange: func() { s.expectExecError(queryUpsertOverlayValue, 6, failure) },
			call: func() error {
				return s.store.SetOverlayValue(ctx, testType, testResource, rootOU, "f", []string{"a"})
			},
			wantMsg: "failed to set overlay value",
		},
		{
			name:    "deleting one overlay value",
			arrange: func() { s.expectExecError(queryDeleteOverlayValue, 5, failure) },
			call:    func() error { return s.store.DeleteOverlayValue(ctx, testType, testResource, rootOU, "f") },
			wantMsg: "failed to delete overlay value",
		},
		{
			name:    "clearing an organization unit's values",
			arrange: func() { s.expectExecError(queryDeleteOverlayValuesForOU, 4, failure) },
			call:    func() error { return s.store.DeleteOverlayValuesForOU(ctx, testType, testResource, rootOU) },
			wantMsg: "failed to delete overlay values",
		},
		{
			name: "the by-initiator lookup",
			arrange: func() {
				s.client.On("QueryContext", mock.Anything, queryGetPolicyByInitiator,
					string(testType), testResource, rootOU, storeDeploymentID).Return(nil, failure).Once()
			},
			call: func() error {
				_, err := s.store.GetPolicyByInitiator(ctx, testType, testResource, rootOU)
				return err
			},
			wantMsg: "failed to get sharing policy by initiator",
		},
		{
			name: "the chain lookup",
			arrange: func() {
				query, args := buildRelevantPoliciesQuery(testType, testResource, []string{rootOU}, storeDeploymentID)
				s.client.On("QueryContext", append([]interface{}{mock.Anything, query}, args...)...).
					Return(nil, failure).Once()
			},
			call: func() error {
				_, err := s.store.ListPoliciesRelevantToChain(ctx, testType, testResource, []string{rootOU})
				return err
			},
			wantMsg: "failed to list policies relevant to chain",
		},
		{
			name: "listing a policy's rules",
			arrange: func() {
				s.client.On("QueryContext", mock.Anything, queryGetPolicyByID, "p1", storeDeploymentID).
					Return([]map[string]interface{}{policyRow("p1")}, nil).Once()
				s.client.On("QueryContext", mock.Anything, queryListTargets, "p1", storeDeploymentID).
					Return([]map[string]interface{}{}, nil).Once()
				s.client.On("QueryContext", mock.Anything, queryListExclusions, "p1", storeDeploymentID).
					Return([]map[string]interface{}{}, nil).Once()
				s.client.On("QueryContext", mock.Anything, queryListRules, "p1", storeDeploymentID).
					Return(nil, failure).Once()
			},
			call:    func() error { _, err := s.store.GetPolicy(ctx, "p1"); return err },
			wantMsg: "failed to list overlay rules",
		},
		{
			// A rule whose document the database cannot return as valid json is reported rather
			// than decoded into an empty rule, which would read as terms nobody set.
			name: "decoding a rule",
			arrange: func() {
				s.client.On("QueryContext", mock.Anything, queryGetPolicyByID, "p1", storeDeploymentID).
					Return([]map[string]interface{}{policyRow("p1")}, nil).Once()
				s.client.On("QueryContext", mock.Anything, queryListTargets, "p1", storeDeploymentID).
					Return([]map[string]interface{}{}, nil).Once()
				s.client.On("QueryContext", mock.Anything, queryListExclusions, "p1", storeDeploymentID).
					Return([]map[string]interface{}{}, nil).Once()
				s.client.On("QueryContext", mock.Anything, queryListRules, "p1", storeDeploymentID).
					Return([]map[string]interface{}{{
						"id": "r1", "field_key": "assignments", "resolved": "not json",
					}}, nil).Once()
			},
			call:    func() error { _, err := s.store.GetPolicy(ctx, "p1"); return err },
			wantMsg: "failed to decode overlay rule",
			noWrap:  true,
		},
	}

	for _, tt := range cases {
		s.Run(tt.name, func() {
			s.SetupTest()
			s.clientAvailable()
			tt.arrange()

			err := tt.call()
			s.Require().Error(err)
			s.Contains(err.Error(), tt.wantMsg)
			if !tt.noWrap {
				s.ErrorIs(err, failure)
			}
		})
	}
}

// The by-initiator lookup is how the service finds the one policy an organization unit holds, so it
// has to hydrate like any other read.
func (s *StoreTestSuite) TestGetPolicyByInitiatorHydrates() {
	s.clientAvailable()
	s.client.On("QueryContext", mock.Anything, queryGetPolicyByInitiator,
		string(testType), testResource, rootOU, storeDeploymentID).
		Return([]map[string]interface{}{policyRow("p1")}, nil).Once()
	s.expectHydrate("p1",
		[]map[string]interface{}{{"id": "t1", "target_scope": string(ScopeChild), "target_ou_id": childOU}},
		nil, nil)

	p, err := s.store.GetPolicyByInitiator(context.Background(), testType, testResource, rootOU)

	s.Require().NoError(err)
	s.Equal(rootOU, p.InitiatingOUID)
	s.Equal([]Target{{ID: "t1", Scope: ScopeChild, OUID: childOU}}, p.Targets)
}

// A failed read of an organization unit's values is reported, not reported as no values, which
// cleanup would read as nothing to clear.
func (s *StoreTestSuite) TestGetOverlayValuesWrapsAQueryFailure() {
	s.clientAvailable()
	failure := errors.New("query error")
	s.client.On("QueryContext", mock.Anything, queryGetOverlayValue,
		string(testType), testResource, rootOU, storeDeploymentID).
		Return(nil, failure).Once()

	got, err := s.store.GetOverlayValues(context.Background(), testType, testResource, rootOU)

	s.Require().Error(err)
	s.Contains(err.Error(), "failed to get overlay values")
	s.ErrorIs(err, failure)
	s.Nil(got)
}

// The deployment a query is scoped by comes from the request, not from whichever one was configured
// when the process started. A context carrying an id has to override the configured fallback, or a
// request acting for one deployment would read and write another's rows.
func (s *StoreTestSuite) TestQueriesAreScopedByTheRequestsDeployment() {
	const otherDeployment = "other-deployment-id"
	s.Require().NotEqual(storeDeploymentID, otherDeployment)
	s.clientAvailable()

	ctx := deployment.WithID(context.Background(), otherDeployment)
	s.client.On("QueryContext", mock.Anything, queryGetPolicyByID, "p1", otherDeployment).
		Return([]map[string]interface{}{}, nil).Once()

	_, err := s.store.GetPolicy(ctx, "p1")
	s.ErrorIs(err, errPolicyNotFound, "the query ran, scoped by the request's deployment")
}

// A context that never passed through the edge, such as a start-up task, falls back to the
// configured identifier rather than scoping by nothing.
func (s *StoreTestSuite) TestQueriesFallBackToTheConfiguredDeployment() {
	s.clientAvailable()
	s.client.On("QueryContext", mock.Anything, queryGetPolicyByID, "p1", storeDeploymentID).
		Return([]map[string]interface{}{}, nil).Once()

	_, err := s.store.GetPolicy(context.Background(), "p1")
	s.ErrorIs(err, errPolicyNotFound)
}

// The documents are the only record of absent versus empty now that the set flags are gone, so the
// encoding has to keep them apart. Absent means unconstrained, empty permits nothing: collapsing
// them turns the most restrictive rule into the least.
func (s *StoreTestSuite) TestRuleDocumentsKeepAbsentApartFromEmpty() {
	cases := []struct {
		name  string
		rule  OverlayRule
		check func(OverlayRule)
	}{
		{
			name: "omitted stays omitted",
			rule: OverlayRule{Editable: true},
			check: func(got OverlayRule) {
				s.Nil(got.AllowedValues)
				s.Nil(got.Value)
				s.Nil(got.ExcludedValues)
			},
		},
		{
			name: "explicitly empty stays empty",
			rule: OverlayRule{Editable: true, AllowedValues: members()},
			check: func(got OverlayRule) {
				s.Require().NotNil(got.AllowedValues)
				s.Empty(*got.AllowedValues)
			},
		},
		{
			name: "a populated set survives",
			rule: OverlayRule{Editable: false, Value: members("a", "b"), ExcludedValues: members("c")},
			check: func(got OverlayRule) {
				s.False(got.Editable)
				s.Equal([]string{"a", "b"}, *got.Value)
				s.Equal([]string{"c"}, *got.ExcludedValues)
			},
		},
	}

	for _, tt := range cases {
		s.Run(tt.name, func() {
			encoded, err := json.Marshal(tt.rule)
			s.Require().NoError(err)

			got, err := hydrateRule(map[string]interface{}{
				"field_key": "assignments",
				"resolved":  string(encoded),
				"requested": string(encoded),
			})
			s.Require().NoError(err)
			tt.check(got.Resolved)
			tt.check(got.Requested)
		})
	}
}

// A member is opaque to the framework and a document bounds none of them, which is the point of
// storing sets this way rather than one row per member in a bounded column.
func (s *StoreTestSuite) TestARuleDocumentBoundsNoSingleMember() {
	long := strings.Repeat("a", 4096)
	encoded, err := json.Marshal(OverlayRule{Editable: true, AllowedValues: members(long)})
	s.Require().NoError(err)

	got, err := hydrateRule(map[string]interface{}{
		"field_key": "assignments", "resolved": string(encoded), "requested": string(encoded),
	})

	s.Require().NoError(err)
	s.Equal([]string{long}, *got.Resolved.AllowedValues)
}

// The unbounded read carries no page arguments and hydrates every row it finds, which is what makes
// it the one policy evaluation can ask a coverage question of.
func (s *StoreTestSuite) TestTheWholeSetReadHydratesEveryPolicy() {
	s.clientAvailable()
	s.client.On("QueryContext", mock.Anything, queryListAllPoliciesForResource,
		string(testType), testResource, storeDeploymentID).
		Return([]map[string]interface{}{policyRow("p1"), policyRow("p2")}, nil).Once()
	s.expectHydrate("p1", nil, nil, nil)
	s.expectHydrate("p2", nil, nil, nil)

	got, err := s.store.ListAllPoliciesForResource(context.Background(), testType, testResource)

	s.Require().NoError(err)
	s.Equal([]string{"p1", "p2"}, []string{got[0].ID, got[1].ID})
}

// The count backs the paged listing's total.
func (s *StoreTestSuite) TestCountingAResourcesPolicies() {
	s.clientAvailable()
	s.client.On("QueryContext", mock.Anything, queryCountPoliciesForResource,
		string(testType), testResource, storeDeploymentID).
		Return([]map[string]interface{}{{"total": int64(7)}}, nil).Once()

	count, err := s.store.CountPoliciesForResource(context.Background(), testType, testResource)

	s.Require().NoError(err)
	s.Equal(7, count)
}

// A COUNT comes back as whatever width the driver decoded it to, so the value is read through the
// converter that accepts them all rather than asserted to int64.
func (s *StoreTestSuite) TestACountIsReadWhateverWidthTheDriverUsed() {
	widths := map[string]interface{}{
		"int":     3,
		"int32":   int32(3),
		"uint64":  uint64(3),
		"float64": float64(3),
	}
	for name, value := range widths {
		s.Run(name, func() {
			s.SetupTest()
			s.clientAvailable()
			s.client.On("QueryContext", mock.Anything, queryCountPoliciesForResource,
				string(testType), testResource, storeDeploymentID).
				Return([]map[string]interface{}{{"total": value}}, nil).Once()

			count, err := s.store.CountPoliciesForResource(context.Background(), testType, testResource)

			s.Require().NoError(err)
			s.Equal(3, count)
		})
	}
}

// No row at all is a count of zero, not a failure: the resource simply has no policies.
func (s *StoreTestSuite) TestACountWithNoRowIsZero() {
	s.clientAvailable()
	s.client.On("QueryContext", mock.Anything, queryCountPoliciesForResource,
		string(testType), testResource, storeDeploymentID).
		Return([]map[string]interface{}{}, nil).Once()

	count, err := s.store.CountPoliciesForResource(context.Background(), testType, testResource)

	s.Require().NoError(err)
	s.Equal(0, count)
}

// A value that is no kind of number is a failure rather than a silent zero, which would report a
// resource as having no policies while it holds them.
func (s *StoreTestSuite) TestACountThatIsNotANumberIsAFailure() {
	s.clientAvailable()
	s.client.On("QueryContext", mock.Anything, queryCountPoliciesForResource,
		string(testType), testResource, storeDeploymentID).
		Return([]map[string]interface{}{{"total": []byte{0x01}}}, nil).Once()

	_, err := s.store.CountPoliciesForResource(context.Background(), testType, testResource)

	s.Require().Error(err)
	s.Contains(err.Error(), "failed to read sharing policy count")
}
