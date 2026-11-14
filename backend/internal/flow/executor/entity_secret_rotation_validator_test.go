// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type EntitySecretRotationValidatorTestSuite struct {
	suite.Suite
	factory   core.FlowFactoryInterface
	validator *clientSecretValidatorMock
}

func TestEntitySecretRotationValidatorTestSuite(t *testing.T) {
	suite.Run(t, new(EntitySecretRotationValidatorTestSuite))
}

func (s *EntitySecretRotationValidatorTestSuite) SetupTest() {
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))
	s.T().Cleanup(config.ResetServerRuntime)
	s.factory, _ = core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	s.validator = newClientSecretValidatorMock(s.T())
}

// agentInputs names an agent as the entity whose secret is rotated.
var agentInputs = map[string]string{
	entity.SystemFlowInputEntity:         "agent-1",
	entity.SystemFlowInputEntityCategory: string(providers.EntityCategoryAgent),
}

func (s *EntitySecretRotationValidatorTestSuite) execute(inputs map[string]string) (
	*providers.ExecutorResponse, error) {
	inputCopy := make(map[string]string, len(inputs))
	for k, v := range inputs {
		inputCopy[k] = v
	}
	return newEntitySecretRotationValidator(s.factory, s.validator).Execute(&providers.NodeContext{
		Context:      context.Background(),
		ExecutorMode: ExecutorModeRevokeBeforeAction,
		UserInputs:   inputCopy,
		RuntimeData:  map[string]string{},
	})
}

// An entity whose client issued tokens gets a plan revoking them up to now, for as long as they can live.
func (s *EntitySecretRotationValidatorTestSuite) TestPublishesAPlanForTheClient() {
	s.validator.EXPECT().ValidateClientSecretRegeneration(mock.Anything, providers.EntityCategoryAgent, "agent-1").
		Return(&entity.ClientSecretProfile{ClientID: "client-1", MaxLifetimeSeconds: 600}, nil)

	resp, err := s.execute(agentInputs)

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	plan, err := decodeRevocationPlan(resp.SharedRuntimeData)
	s.Require().NoError(err)
	s.Equal(revocation.ModeBeforeAction, plan.Mode)
	s.Equal(revocation.ReasonApplicationSecretRegenerated, plan.Reason)
	s.Equal("agent-1", plan.TargetID)
	s.Equal([]revocation.Criterion{{Type: revocation.CriterionTypeApplicationKey, Value: "client-1"}},
		plan.Criteria)
	s.Equal(int64(600), plan.TTLSeconds)
	s.False(plan.Cutoff.IsZero())
}

// The owning service's refusal lands on this node, before anything is revoked, and keeps its own code.
func (s *EntitySecretRotationValidatorTestSuite) TestRefusalPublishesNoPlan() {
	s.validator.EXPECT().ValidateClientSecretRegeneration(mock.Anything, providers.EntityCategoryAgent, "agent-1").
		Return(nil, &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "AGT-1050"})

	resp, err := s.execute(agentInputs)

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Equal("AGT-1050", resp.Error.Code)
	s.Empty(resp.SharedRuntimeData[common.RuntimeKeyRevocationPlan])
}

// A refusal with no code of its own falls back to this executor's.
func (s *EntitySecretRotationValidatorTestSuite) TestUncodedRefusalFallsBack() {
	s.validator.EXPECT().ValidateClientSecretRegeneration(mock.Anything, providers.EntityCategoryAgent, "agent-1").
		Return(nil, &tidcommon.ServiceError{Type: tidcommon.ClientErrorType})

	resp, err := s.execute(agentInputs)

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Equal(ErrSecretRegenerationNotAllowed.Code, resp.Error.Code)
}

// A server-side validation failure is an execution error, not a flow failure the caller can act on.
func (s *EntitySecretRotationValidatorTestSuite) TestServerErrorFailsExecution() {
	s.validator.EXPECT().ValidateClientSecretRegeneration(mock.Anything, providers.EntityCategoryAgent, "agent-1").
		Return(nil, &tidcommon.InternalServerError)

	_, err := s.execute(agentInputs)

	s.Require().Error(err)
}

// The entity and its category are required inputs, so omitting either asks for it rather than acting on
// nothing.
func (s *EntitySecretRotationValidatorTestSuite) TestMissingInputsAskForThem() {
	for _, missing := range []string{entity.SystemFlowInputEntity, entity.SystemFlowInputEntityCategory} {
		inputs := map[string]string{}
		for k, v := range agentInputs {
			if k != missing {
				inputs[k] = v
			}
		}
		resp, err := s.execute(inputs)

		s.Require().NoError(err, missing)
		s.Equal(providers.ExecUserInputRequired, resp.Status, missing)
	}
}
