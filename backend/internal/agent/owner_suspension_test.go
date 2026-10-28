// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"errors"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/identitygovernance"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/identitygovernancemock"
)

// recordingRevoker records every revocation batch handed to it, and the order of calls in calls.
type recordingRevoker struct {
	batches [][]revocation.CriteriaRevocation
	calls   *[]string
	err     *tidcommon.ServiceError
}

func (r *recordingRevoker) RevokeCriteriaBatch(_ context.Context,
	revocations []revocation.CriteriaRevocation) *tidcommon.ServiceError {
	r.batches = append(r.batches, revocations)
	*r.calls = append(*r.calls, "revoke")
	return r.err
}

// ownerSuspensionFixture returns a service whose governance and revoker record the calls, with alice's personal
// agent stored. ownerSuspended says whether alice has already suspended it.
func (suite *AgentServiceTestSuite) ownerSuspensionFixture(ownerSuspended bool) (*agentService,
	*entitymock.EntityServiceInterfaceMock, *identitygovernancemock.IdentityGovernanceServiceInterfaceMock,
	*recordingRevoker, *[]string) {
	suite.Require().NoError(config.InitializeServerRuntime("", &config.Config{JWT: engineconfig.JWTConfig{Leeway: 30}}))
	suite.T().Cleanup(config.ResetServerRuntime)

	svc, mockEntity, _, _, _ := suite.setupService()
	owned := withKind(buildAgentEntityFixture(testAgentName, "desc", "alice", "cid-123"),
		providers.AgentKindPersonalAssistant)
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).Maybe().Return(owned, nil)
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).Maybe().Return(owned, nil)

	calls := []string{}
	governance := identitygovernancemock.NewIdentityGovernanceServiceInterfaceMock(suite.T())
	governance.EXPECT().ReadAccess(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, entities []providers.Entity) []identitygovernance.AccessRead {
			reads := activeReads(context.Background(), entities)
			if ownerSuspended {
				for i := range reads {
					reads[i].Status = ownerSuspendedStatus()
				}
			}
			return reads
		}).Maybe()
	svc.governance = governance
	revoker := &recordingRevoker{calls: &calls}
	svc.revoker = revoker
	return svc, mockEntity, governance, revoker, &calls
}

func ownerSuspendedStatus() providers.AccessStatusReport {
	return providers.AccessStatusReport{Value: providers.AccessStatusSuspended,
		Details: &providers.AccessStatusDetails{Reason: providers.AccessHoldReasonOwnerSuspension,
			AppliedBy: []providers.SuspensionAppliedBy{providers.SuspensionAppliedByOwner}}}
}

// runtimeContext matches a governance call made with the runtime context, as the owner's calls are.
var runtimeContext = mock.MatchedBy(security.IsRuntimeContext)

// The owner's suspension revokes the tokens issued to the agent's client, bounded at now plus the leeway, and
// only then places the owner's hold.
func (suite *AgentServiceTestSuite) TestSuspendOwnedAgent_RevokesThenSuspends() {
	svc, _, governance, revoker, calls := suite.ownerSuspensionFixture(false)
	governance.EXPECT().Suspend(runtimeContext, testAgentID, "").
		RunAndReturn(func(context.Context, string, string) *tidcommon.ServiceError {
			*calls = append(*calls, "suspend")
			return nil
		}).Once()
	before := time.Now()

	suite.Require().Nil(svc.SuspendOwnedAgent(context.Background(), "alice", testAgentID,
		providers.AgentKindPersonalAssistant))

	suite.Equal([]string{"revoke", "suspend"}, *calls)
	suite.Require().Len(revoker.batches, 1)
	suite.Require().Len(revoker.batches[0], 1)
	revoked := revoker.batches[0][0]
	suite.Equal(revocation.Criterion{Type: revocation.CriterionTypeApplicationKey, Value: "cid-123"},
		revoked.Criterion)
	suite.Equal(revocation.ModeBeforeAction, revoked.Mode)
	suite.Equal(revocation.ReasonIdentitySuspended, revoked.Reason)
	suite.WithinDuration(before.Add(30*time.Second), revoked.Cutoff, 5*time.Second)
	suite.Equal(suspensionRevocationTTL, revoked.TTL, "kept as long as the suspend flow keeps its revocation")
}

// A revocation of an agent's tokens is kept as long as a token issued to its client can live, and at least for
// the given floor. A client that cannot be resolved keeps the floor.
func (suite *AgentServiceTestSuite) TestRevocationTTL() {
	client := &providers.OAuthClient{ClientID: "cid-123"}
	cases := []struct {
		name     string
		lifetime time.Duration
		lookup   error
		floor    time.Duration
		want     time.Duration
	}{
		{"a lifetime beyond the floor", 30 * 24 * time.Hour, nil, suspensionRevocationTTL, 30 * 24 * time.Hour},
		{"a lifetime within the floor", time.Hour, nil, suspensionRevocationTTL, suspensionRevocationTTL},
		{"no floor", 2 * time.Hour, nil, 0, 2 * time.Hour},
		{"an unresolvable client", 30 * 24 * time.Hour, errors.New("store down"), suspensionRevocationTTL,
			suspensionRevocationTTL},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			svc, _, mockInbound, _, _ := suite.setupService()
			svc.resolveLifetime = func(*providers.OAuthClient) time.Duration { return tc.lifetime }
			clearMockCalls(mockInbound, "GetOAuthClientByClientID")
			if tc.lookup != nil {
				mockInbound.On("GetOAuthClientByClientID", mock.Anything, "cid-123").Return(nil, tc.lookup).Once()
			} else {
				mockInbound.On("GetOAuthClientByClientID", mock.Anything, "cid-123").Return(client, nil).Once()
			}

			suite.Equal(tc.want, svc.revocationTTL(context.Background(), "cid-123", tc.floor))
		})
	}

	svc, _, _, _, _ := suite.setupService()
	suite.Equal(suspensionRevocationTTL, svc.revocationTTL(context.Background(), "cid-123", suspensionRevocationTTL),
		"without a resolver, the floor is kept")
}

// An agent the owner already suspended is left as it is: nothing is revoked or written again.
func (suite *AgentServiceTestSuite) TestSuspendOwnedAgent_AlreadySuspendedByTheOwner() {
	svc, _, governance, revoker, _ := suite.ownerSuspensionFixture(true)

	suite.Require().Nil(svc.SuspendOwnedAgent(context.Background(), "alice", testAgentID,
		providers.AgentKindPersonalAssistant))

	suite.Empty(revoker.batches)
	governance.AssertNotCalled(suite.T(), "Suspend", mock.Anything, mock.Anything, mock.Anything)
}

// A failed revocation suspends nothing.
func (suite *AgentServiceTestSuite) TestSuspendOwnedAgent_FailedRevocationSuspendsNothing() {
	svc, _, governance, revoker, _ := suite.ownerSuspensionFixture(false)
	revoker.err = &tidcommon.InternalServerError

	svcErr := svc.SuspendOwnedAgent(context.Background(), "alice", testAgentID, providers.AgentKindPersonalAssistant)

	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	governance.AssertNotCalled(suite.T(), "Suspend", mock.Anything, mock.Anything, mock.Anything)
}

// Another owner's agent is not found, for each owner action, and nothing is touched.
func (suite *AgentServiceTestSuite) TestOwnerSuspension_AnotherOwnersAgentIsNotFound() {
	svc, _, governance, revoker, _ := suite.ownerSuspensionFixture(false)

	for name, action := range map[string]func() *tidcommon.ServiceError{
		"suspend": func() *tidcommon.ServiceError {
			return svc.SuspendOwnedAgent(context.Background(), "bob", testAgentID, providers.AgentKindPersonalAssistant)
		},
		"unsuspend": func() *tidcommon.ServiceError {
			return svc.UnsuspendOwnedAgent(context.Background(), "bob", testAgentID,
				providers.AgentKindPersonalAssistant)
		},
	} {
		svcErr := action()
		suite.Require().NotNil(svcErr, name)
		suite.Equal(ErrorAgentNotFound.Code, svcErr.Code, name)
	}
	suite.Empty(revoker.batches)
	governance.AssertNotCalled(suite.T(), "Suspend", mock.Anything, mock.Anything, mock.Anything)
	governance.AssertNotCalled(suite.T(), "Unsuspend", mock.Anything, mock.Anything)
}

// Lifting the owner's suspension needs no revocation. An agent the owner has not suspended is left as it is, and a
// governance fault is a server error.
func (suite *AgentServiceTestSuite) TestUnsuspendOwnedAgent() {
	for name, tc := range map[string]struct {
		governance *tidcommon.ServiceError
		want       *tidcommon.ServiceError
	}{
		"lifted":              {nil, nil},
		"not suspended by it": {&identitygovernance.ErrorNotApplicable, nil},
		"governance fault":    {&tidcommon.InternalServerError, &tidcommon.InternalServerError},
	} {
		suite.Run(name, func() {
			svc, _, governance, revoker, _ := suite.ownerSuspensionFixture(false)
			governance.EXPECT().Unsuspend(runtimeContext, testAgentID).Return(tc.governance).Once()

			svcErr := svc.UnsuspendOwnedAgent(context.Background(), "alice", testAgentID,
				providers.AgentKindPersonalAssistant)

			if tc.want == nil {
				suite.Nil(svcErr)
			} else {
				suite.Require().NotNil(svcErr)
				suite.Equal(tc.want.Code, svcErr.Code)
			}
			suite.Empty(revoker.batches)
		})
	}
}

// Suspending all of the owner's agents skips those the owner already suspended, revokes the rest in one batch,
// and reports how many it suspended.
func (suite *AgentServiceTestSuite) TestSuspendOwnedAgents_SkipsAndCounts() {
	svc, mockEntity, governance, revoker, _ := suite.ownerSuspensionFixture(false)
	first := withKind(buildAgentEntityFixture("first", "", "alice", "cid-1"), providers.AgentKindPersonalAssistant)
	first.ID = "agent-1"
	second := withKind(buildAgentEntityFixture("second", "", "alice", "cid-2"), providers.AgentKindPersonalAssistant)
	second.ID = "agent-2"
	already := withKind(buildAgentEntityFixture("already", "", "alice", "cid-3"), providers.AgentKindPersonalAssistant)
	already.ID = "agent-3"
	mockEntity.On("GetEntityListBySystemAttributes", mock.Anything, providers.EntityCategoryAgent, mock.Anything,
		mock.Anything, 0).Return([]providers.Entity{*first, *second, *already}, nil).Once()
	governance.ExpectedCalls = nil
	governance.EXPECT().ReadAccess(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, entities []providers.Entity) []identitygovernance.AccessRead {
			reads := activeReads(context.Background(), entities)
			reads[2].Status = ownerSuspendedStatus()
			return reads
		}).Once()
	governance.EXPECT().Suspend(runtimeContext, "agent-1", "").Return(nil).Once()
	governance.EXPECT().Suspend(runtimeContext, "agent-2", "").Return(nil).Once()

	suspended, svcErr := svc.SuspendOwnedAgents(context.Background(), "alice", providers.AgentKindPersonalAssistant)

	suite.Require().Nil(svcErr)
	suite.Equal(2, suspended)
	suite.Require().Len(revoker.batches, 1)
	suite.Require().Len(revoker.batches[0], 2)
	suite.Equal("cid-1", revoker.batches[0][0].Criterion.Value)
	suite.Equal("cid-2", revoker.batches[0][1].Criterion.Value)
}

// A failed listing suspends nothing.
func (suite *AgentServiceTestSuite) TestSuspendOwnedAgents_FailedListing() {
	svc, mockEntity, _, revoker, _ := suite.ownerSuspensionFixture(false)
	mockEntity.On("GetEntityListBySystemAttributes", mock.Anything, providers.EntityCategoryAgent, mock.Anything,
		mock.Anything, 0).Return(nil, errors.New("store down")).Once()

	_, svcErr := svc.SuspendOwnedAgents(context.Background(), "alice", providers.AgentKindPersonalAssistant)

	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	suite.Empty(revoker.batches)
}

// A user's suspension dependents are their personal agents, declarative ones included. Other resource types have
// none, and a failed listing is an error.
func (suite *AgentServiceTestSuite) TestSuspensionDependents() {
	svc, mockEntity, _, _, _ := suite.setupService()
	declarative := withKind(buildAgentEntityFixture("file", "", "alice", "cid-2"), providers.AgentKindPersonalAssistant)
	declarative.ID, declarative.IsReadOnly = "declarative-agent", true
	mockEntity.On("GetEntityListBySystemAttributes", mock.Anything, providers.EntityCategoryAgent,
		entity.SystemAttributeQuery{SystemAttributes: map[string]interface{}{
			fieldOwner: "alice", fieldKind: string(providers.AgentKindPersonalAssistant),
		}}, mock.Anything, 0).
		Return([]providers.Entity{{ID: "agent-1"}, *declarative}, nil).Once()

	dependents, err := svc.SuspensionDependents(context.Background(), resourcedependency.ResourceTypeUser, "alice")
	suite.Require().NoError(err)
	suite.Equal([]string{"agent-1", "declarative-agent"}, dependents)

	dependents, err = svc.SuspensionDependents(context.Background(), resourcedependency.ResourceTypeAgent, "agent-1")
	suite.Require().NoError(err)
	suite.Empty(dependents)

	mockEntity.On("GetEntityListBySystemAttributes", mock.Anything, providers.EntityCategoryAgent, mock.Anything,
		mock.Anything, 0).Return(nil, errors.New("store down")).Once()
	_, err = svc.SuspensionDependents(context.Background(), resourcedependency.ResourceTypeUser, "alice")
	suite.Error(err)
}
