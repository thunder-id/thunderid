// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/flow/sessionmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/revocationmock"
)

type PreCredentialUpdateExecutorTestSuite struct {
	suite.Suite
	factory core.FlowFactoryInterface
}

func TestPreCredentialUpdateExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(PreCredentialUpdateExecutorTestSuite))
}

func (s *PreCredentialUpdateExecutorTestSuite) SetupTest() {
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))
	s.T().Cleanup(config.ResetServerRuntime)
	s.factory, _ = core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
}

// plan runs the executor for user-123 and returns what it shared.
func (s *PreCredentialUpdateExecutorTestSuite) plan() map[string]string {
	resp, err := newPreCredentialUpdateExecutor(s.factory).Execute(&providers.NodeContext{
		Context:      context.Background(),
		ExecutorMode: string(revocation.ModeBeforeAction),
		UserInputs:   map[string]string{revocationInputSubject: "user-123"},
		RuntimeData:  map[string]string{},
	})
	s.Require().NoError(err)
	s.Require().Equal(providers.ExecComplete, resp.Status)
	s.Require().NotEmpty(resp.SharedRuntimeData[common.RuntimeKeyRevocationPlan])
	return resp.SharedRuntimeData
}

func (s *PreCredentialUpdateExecutorTestSuite) TestCredentialUpdateFlow() {
	before := time.Now().UTC()
	shared := s.plan()
	after := time.Now().UTC()

	revoker := revocationmock.NewCriteriaRevokerInterfaceMock(s.T())
	revoker.EXPECT().RevokeCriteriaBatch(mock.Anything,
		mock.MatchedBy(func(batch []revocation.CriteriaRevocation) bool {
			return len(batch) == 1 && batch[0].Criterion.Type == revocation.CriterionTypeSubject &&
				batch[0].Criterion.Value == "user-123" && batch[0].Mode == revocation.ModeBeforeAction &&
				batch[0].Reason == revocation.ReasonCredentialUpdated &&
				!batch[0].Cutoff.Before(before) && !batch[0].Cutoff.After(after)
		})).Return(nil).Once()
	revokeResp, err := newCriteriaRevocationExecutor(s.factory, revoker).Execute(&providers.NodeContext{
		Context: context.Background(), ExecutorMode: ExecutorModeRevoke, SharedRuntimeData: shared,
	})
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, revokeResp.Status)

	sessions := sessionmock.NewServiceMock(s.T())
	sessions.EXPECT().TerminateBySubject(mock.Anything, "user-123").Return(nil).Once()
	sessionResp, err := newSessionRevocationExecutor(s.factory, sessions).Execute(&providers.NodeContext{
		Context: context.Background(), SharedRuntimeData: shared,
	})
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, sessionResp.Status)

	// The post flow moves the cutoff to the moment after the write.
	revoker.EXPECT().RevokeCriteriaBatch(mock.Anything,
		mock.MatchedBy(func(batch []revocation.CriteriaRevocation) bool {
			return len(batch) == 1 && batch[0].Reason == revocation.ReasonCredentialUpdated &&
				!batch[0].Cutoff.Before(after)
		})).Return(nil).Once()
	advanceResp, err := newCriteriaRevocationExecutor(s.factory, revoker).Execute(&providers.NodeContext{
		Context: context.Background(), ExecutorMode: ExecutorModeAdvanceCutoff, SharedRuntimeData: shared,
	})
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, advanceResp.Status)
}

func (s *PreCredentialUpdateExecutorTestSuite) TestRequestsTheSubject() {
	resp, err := newPreCredentialUpdateExecutor(s.factory).Execute(
		&providers.NodeContext{Context: context.Background()})

	s.Require().NoError(err)
	s.Equal(providers.ExecUserInputRequired, resp.Status)
	s.Empty(resp.SharedRuntimeData)
}

func (s *PreCredentialUpdateExecutorTestSuite) TestDeclaresModeContract() {
	meta := newPreCredentialUpdateExecutor(s.factory).GetMeta()

	s.Equal(string(revocation.ModeBeforeAction), meta.DefaultMode)
	s.Equal([]string{string(revocation.ModeBeforeAction)}, meta.SupportedModes)
	s.Equal([]providers.FlowType{providers.FlowTypeSystem}, meta.SupportedFlowTypes)
}
