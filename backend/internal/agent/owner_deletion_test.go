// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"errors"

	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/revocation"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
)

// ownedAgents returns the agents of user-2 the deletion tests use: a personal agent, a declarative personal
// agent and an ordinary agent.
func ownedAgents() []providers.Entity {
	personal := withKind(buildAgentEntityFixture("personal", "", "user-2", "cid-personal"),
		providers.AgentKindPersonalAssistant)
	personal.ID = "agent-personal"
	declarative := withKind(buildAgentEntityFixture("declarative", "", "user-2", "cid-declarative"),
		providers.AgentKindPersonalAssistant)
	declarative.ID = "agent-declarative"
	declarative.IsReadOnly = true
	ordinary := buildAgentEntityFixture("ordinary", "", "user-2", "cid-ordinary")
	ordinary.ID = "agent-ordinary"
	return []providers.Entity{*personal, *declarative, *ordinary}
}

// ownerDeletionService returns a service that lists user-2's agents and records its revocations.
func (suite *AgentServiceTestSuite) ownerDeletionService() (*agentService, *entitymock.EntityServiceInterfaceMock,
	*recordingRevoker) {
	svc, mockEntity, _, _, _ := suite.setupService()
	mockEntity.On("GetEntityListBySystemAttributes", mock.Anything, providers.EntityCategoryAgent,
		ownerQuery("user-2"), serverconst.MaxCompositeStoreRecords, 0).Return(ownedAgents(), nil).Maybe()
	revoker := &recordingRevoker{calls: &[]string{}}
	svc.revoker = revoker
	return svc, mockEntity, revoker
}

// A personal agent is deleted with its owner, so it does not block the owner's deletion. A declarative one cannot
// be deleted at runtime and an ordinary one outlives its owner, so both still block it.
func (suite *AgentServiceTestSuite) TestGetResourceDependencies_ByOwner_PersonalAgentsCascade() {
	svc, _, _ := suite.ownerDeletionService()

	usages, err := svc.GetResourceDependencies(context.Background(), resourcedependency.ResourceTypeUser, "user-2")

	suite.Require().NoError(err)
	behaviors := map[string]string{}
	for _, usage := range usages {
		behaviors[usage.ID] = usage.BehaviorOnDelete
	}
	suite.Equal(map[string]string{
		"agent-personal":    resourcedependency.BehaviorCascade,
		"agent-declarative": resourcedependency.BehaviorRestrict,
		"agent-ordinary":    resourcedependency.BehaviorRestrict,
	}, behaviors)
}

// Without a revoker, which is the control plane, a personal agent's tokens could not be revoked, so it keeps
// blocking its owner's deletion and the cascade deletes nothing.
func (suite *AgentServiceTestSuite) TestOwnerDeletion_WithoutARevokerPersonalAgentsBlock() {
	svc, mockEntity, _ := suite.ownerDeletionService()
	svc.revoker = nil

	usages, err := svc.GetResourceDependencies(context.Background(), resourcedependency.ResourceTypeUser, "user-2")
	suite.Require().NoError(err)
	for _, usage := range usages {
		suite.Equal(resourcedependency.BehaviorRestrict, usage.BehaviorOnDelete, usage.ID)
	}

	deleted, err := svc.CascadeDeleteDependencies(context.Background(), resourcedependency.ResourceTypeUser, "user-2")
	suite.Require().NoError(err)
	suite.Zero(deleted)
	mockEntity.AssertNotCalled(suite.T(), "DeleteEntity", mock.Anything, mock.Anything)
}

// The owner's deletion revokes every token issued to their personal agents' clients, outright, then deletes those
// agents. Declarative and ordinary agents are left alone.
func (suite *AgentServiceTestSuite) TestCascadeDeleteDependencies_DeletesTheOwnersPersonalAgents() {
	svc, mockEntity, revoker := suite.ownerDeletionService()
	clearMockCalls(mockEntity, "DeleteEntity")
	mockEntity.On("DeleteEntity", mock.Anything, "agent-personal").Return(nil).Once()

	deleted, err := svc.CascadeDeleteDependencies(context.Background(), resourcedependency.ResourceTypeUser, "user-2")

	suite.Require().NoError(err)
	suite.Equal(1, deleted)
	suite.Require().Len(revoker.batches, 1)
	suite.Equal([]revocation.CriteriaRevocation{{
		Criterion: revocation.Criterion{Type: revocation.CriterionTypeApplicationKey, Value: "cid-personal"},
		Mode:      revocation.ModeAll,
		Reason:    revocation.ReasonUserDeleted,
	}}, revoker.batches[0])
	mockEntity.AssertNotCalled(suite.T(), "DeleteEntity", mock.Anything, "agent-declarative")
	mockEntity.AssertNotCalled(suite.T(), "DeleteEntity", mock.Anything, "agent-ordinary")
}

// Any failure aborts the owner's deletion: listing the agents, revoking their tokens or deleting one. A failed
// revocation deletes nothing.
func (suite *AgentServiceTestSuite) TestCascadeDeleteDependencies_FailuresAbort() {
	suite.Run("listing", func() {
		svc, mockEntity, _, _, _ := suite.setupService()
		svc.revoker = &recordingRevoker{calls: &[]string{}}
		mockEntity.On("GetEntityListBySystemAttributes", mock.Anything, providers.EntityCategoryAgent,
			ownerQuery("user-2"), serverconst.MaxCompositeStoreRecords, 0).Return(nil, errors.New("store down"))

		_, err := svc.CascadeDeleteDependencies(context.Background(), resourcedependency.ResourceTypeUser, "user-2")
		suite.Error(err)
	})
	suite.Run("revocation", func() {
		svc, mockEntity, revoker := suite.ownerDeletionService()
		revoker.err = &tidcommon.InternalServerError

		_, err := svc.CascadeDeleteDependencies(context.Background(), resourcedependency.ResourceTypeUser, "user-2")
		suite.Error(err)
		mockEntity.AssertNotCalled(suite.T(), "DeleteEntity", mock.Anything, mock.Anything)
	})
	suite.Run("deletion", func() {
		svc, mockEntity, _ := suite.ownerDeletionService()
		clearMockCalls(mockEntity, "DeleteEntity")
		mockEntity.On("DeleteEntity", mock.Anything, "agent-personal").Return(errors.New("store down")).Once()

		_, err := svc.CascadeDeleteDependencies(context.Background(), resourcedependency.ResourceTypeUser, "user-2")
		suite.Error(err)
	})
}

// Only a user's deletion removes agents; the cascade of any other resource type deletes nothing.
func (suite *AgentServiceTestSuite) TestCascadeDeleteDependencies_OtherResourceTypes() {
	svc, mockEntity, revoker := suite.ownerDeletionService()

	deleted, err := svc.CascadeDeleteDependencies(context.Background(), resourcedependency.ResourceTypeAgent,
		"agent-personal")

	suite.Require().NoError(err)
	suite.Zero(deleted)
	suite.Empty(revoker.batches)
	mockEntity.AssertNotCalled(suite.T(), "GetEntityListBySystemAttributes",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
