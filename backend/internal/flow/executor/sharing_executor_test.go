// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// recordedShareTarget is what validate records for a user shared into ou-a.
const recordedShareTarget = "user:ou-a"

type SharingExecutorTestSuite struct {
	suite.Suite
	provider *sharingAdminProviderMock
	executor *sharingExecutor
}

func TestSharingExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(SharingExecutorTestSuite))
}

func (s *SharingExecutorTestSuite) SetupTest() {
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))
	s.T().Cleanup(config.ResetServerRuntime)
	factory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	s.provider = newSharingAdminProviderMock(s.T())
	s.executor = newSharingExecutor(factory, s.provider)
}

// adminContext is the security context of the admin who started the flow.
func adminContext() context.Context {
	return security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest("admin-1", "ou-a", "token", nil, map[string]interface{}{}))
}

func (s *SharingExecutorTestSuite) nodeContext(ctx context.Context, mode string,
	properties map[string]interface{}) *providers.NodeContext {
	return &providers.NodeContext{
		Context:        ctx,
		FlowType:       providers.FlowTypeInvitation,
		ExecutorMode:   mode,
		NodeProperties: properties,
		UserInputs:     map[string]string{},
		RuntimeData: map[string]string{
			common.RuntimeKeyOUID:    "ou-a",
			common.RuntimeKeyInviter: "admin-1",
			userAttributeUserID:      "user-1",
		},
	}
}

// signUpContext is a sign-up that provisioned user-new and shares it into ou-new.
func (s *SharingExecutorTestSuite) signUpContext(flowType providers.FlowType) *providers.NodeContext {
	return &providers.NodeContext{
		Context:        context.Background(),
		FlowType:       flowType,
		ExecutorMode:   ExecutorModeShare,
		NodeProperties: existingUserProperties(),
		UserInputs:     map[string]string{},
		RuntimeData: map[string]string{
			common.RuntimeKeyOUID: "ou-new",
			userAttributeUserID:   "user-new",
		},
	}
}

func existingUserProperties() map[string]interface{} {
	return map[string]interface{}{
		propertyKeyResourceType: "user",
		propertyKeyResourceID:   "{{ctx(userID)}}",
		propertyKeyTargetOUID:   "{{ctx(ouId)}}",
	}
}

func (s *SharingExecutorTestSuite) TestValidate_RecordsTheAuthorizedTarget() {
	ctx := s.nodeContext(adminContext(), ExecutorModeValidate, existingUserProperties())
	s.provider.EXPECT().ValidateShareIntoChildOU(ctx.Context, sharing.ResourceType("user"), "user-1", "", "ou-a").
		Return(nil).Once()

	resp, err := s.executor.Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	s.Equal(recordedShareTarget, resp.RuntimeData[common.RuntimeKeyAuthorizedShare])
}

func (s *SharingExecutorTestSuite) TestValidate_ANewUserNamesItsOwningUnit() {
	ctx := s.nodeContext(adminContext(), ExecutorModeValidate, map[string]interface{}{
		propertyKeyResourceType: "user",
		propertyKeyOwningOUID:   "{{ctx(defaultOUID)}}",
		propertyKeyTargetOUID:   "{{ctx(ouId)}}",
	})
	ctx.RuntimeData["defaultOUID"] = "root"
	s.provider.EXPECT().ValidateShareIntoChildOU(ctx.Context, sharing.ResourceType("user"), "", "root", "ou-a").
		Return(nil).Once()

	resp, err := s.executor.Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	s.Equal(recordedShareTarget, resp.RuntimeData[common.RuntimeKeyAuthorizedShare])
}

func (s *SharingExecutorTestSuite) TestValidate_RefusalCarriesTheProvidersErrorAndRecordsNothing() {
	ctx := s.nodeContext(adminContext(), ExecutorModeValidate, existingUserProperties())
	refusal := tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "SHR-1029"}
	s.provider.EXPECT().ValidateShareIntoChildOU(mock.Anything, sharing.ResourceType("user"), "user-1", "", "ou-a").
		Return(&refusal)

	resp, err := s.executor.Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Require().NotNil(resp.Error)
	s.Equal("SHR-1029", resp.Error.Code)
	s.NotContains(resp.RuntimeData, common.RuntimeKeyAuthorizedShare)
}

// An authorizer run without the admin's token allows everything, so validate needs the inviter's own context.
func (s *SharingExecutorTestSuite) TestValidate_RefusesAnyCallerButTheInviter() {
	for name, callerCtx := range map[string]context.Context{
		"runtime": security.WithRuntimeContext(context.Background()),
		"other": security.WithSecurityContextTest(context.Background(),
			security.NewSecurityContextForTest("admin-2", "ou-a", "token", nil, map[string]interface{}{})),
		// A step the invitee sends runs as the recorded inviter, who did not send it.
		"recorded": security.WithRecordedCaller(context.Background(), "admin-1", "ou-a", []string{"system"}),
	} {
		s.Run(name, func() {
			resp, err := s.executor.Execute(s.nodeContext(callerCtx, ExecutorModeValidate, existingUserProperties()))

			s.Require().NoError(err)
			s.Equal(providers.ExecFailure, resp.Status)
			s.Equal(ErrNotTheInviter.Code, resp.Error.Code)
		})
	}
	s.provider.AssertNotCalled(s.T(), "ValidateShareIntoChildOU",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *SharingExecutorTestSuite) TestShare_SharesTheRecordedTarget() {
	ctx := s.nodeContext(security.WithRuntimeContext(context.Background()), ExecutorModeShare,
		existingUserProperties())
	ctx.RuntimeData[common.RuntimeKeyAuthorizedShare] = recordedShareTarget
	s.provider.EXPECT().ShareIntoChildOU(ctx.Context, sharing.ResourceType("user"), "user-1", "ou-a").Return(nil).Once()

	resp, err := s.executor.Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
}

func (s *SharingExecutorTestSuite) TestShare_RefusesWithoutAMatchingRecord() {
	for name, record := range map[string]string{"none": "", "another target": "user:ou-b"} {
		s.Run(name, func() {
			ctx := s.nodeContext(security.WithRuntimeContext(context.Background()), ExecutorModeShare,
				existingUserProperties())
			if record != "" {
				ctx.RuntimeData[common.RuntimeKeyAuthorizedShare] = record
			}

			resp, err := s.executor.Execute(ctx)

			s.Require().NoError(err)
			s.Equal(providers.ExecFailure, resp.Status)
			s.Equal(ErrShareNotAuthorized.Code, resp.Error.Code)
		})
	}
	s.provider.AssertNotCalled(s.T(), "ShareIntoChildOU", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything)
}

// The invitee's inputs cannot stand in for what the admin's half recorded or resolved.
func (s *SharingExecutorTestSuite) TestShare_IgnoresInputsNamedLikeRuntimeKeys() {
	ctx := s.nodeContext(security.WithRuntimeContext(context.Background()), ExecutorModeShare,
		existingUserProperties())
	delete(ctx.RuntimeData, userAttributeUserID)
	ctx.UserInputs = map[string]string{
		common.RuntimeKeyAuthorizedShare: recordedShareTarget,
		userAttributeUserID:              "user-9",
	}

	resp, err := s.executor.Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Equal(ErrShareNotAuthorized.Code, resp.Error.Code)
	s.provider.AssertNotCalled(s.T(), "ShareIntoChildOU", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything)
}

func (s *SharingExecutorTestSuite) TestShare_UnresolvedPropertyIsAnError() {
	ctx := s.nodeContext(security.WithRuntimeContext(context.Background()), ExecutorModeShare,
		existingUserProperties())
	ctx.RuntimeData[common.RuntimeKeyAuthorizedShare] = recordedShareTarget
	delete(ctx.RuntimeData, userAttributeUserID)

	_, err := s.executor.Execute(ctx)

	s.Error(err)
}

func (s *SharingExecutorTestSuite) TestShare_ProviderOutcomes() {
	s.Run("ClientErrorIsAFailure", func() {
		ctx := s.nodeContext(security.WithRuntimeContext(context.Background()), ExecutorModeShare,
			existingUserProperties())
		ctx.RuntimeData[common.RuntimeKeyAuthorizedShare] = recordedShareTarget
		s.provider.EXPECT().ShareIntoChildOU(mock.Anything, sharing.ResourceType("user"), "user-1", "ou-a").Return(
			&tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "SHR-1029"}).Once()

		resp, err := s.executor.Execute(ctx)

		s.Require().NoError(err)
		s.Equal(providers.ExecFailure, resp.Status)
		s.Equal("SHR-1029", resp.Error.Code)
	})

	s.Run("ServerErrorIsAnError", func() {
		ctx := s.nodeContext(security.WithRuntimeContext(context.Background()), ExecutorModeShare,
			existingUserProperties())
		ctx.RuntimeData[common.RuntimeKeyAuthorizedShare] = recordedShareTarget
		s.provider.EXPECT().ShareIntoChildOU(mock.Anything, sharing.ResourceType("user"), "user-1", "ou-a").Return(
			&tidcommon.InternalServerError).Once()

		_, err := s.executor.Execute(ctx)

		s.Error(err)
	})
}

func (s *SharingExecutorTestSuite) TestShare_SignUpSharesWhatItNames() {
	for _, flowType := range []providers.FlowType{providers.FlowTypeAuthentication, providers.FlowTypeRegistration} {
		s.Run(string(flowType), func() {
			ctx := s.signUpContext(flowType)
			s.provider.EXPECT().ShareIntoChildOU(ctx.Context, sharing.ResourceType("user"), "user-new", "ou-new").
				Return(nil).Once()

			resp, err := s.executor.Execute(ctx)

			s.Require().NoError(err)
			s.Equal(providers.ExecComplete, resp.Status)
		})
	}
}

func (s *SharingExecutorTestSuite) TestModeOutsideItsFlowTypesIsAnError() {
	for name, ctx := range map[string]*providers.NodeContext{
		"validate in a sign-in flow": func() *providers.NodeContext {
			ctx := s.signUpContext(providers.FlowTypeAuthentication)
			ctx.ExecutorMode = ExecutorModeValidate
			return ctx
		}(),
		"share in an administration flow": s.signUpContext(providers.FlowTypeAdministration),
	} {
		s.Run(name, func() {
			_, err := s.executor.Execute(ctx)

			s.Error(err)
		})
	}
	s.provider.AssertNotCalled(s.T(), "ShareIntoChildOU", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything)
}

func (s *SharingExecutorTestSuite) TestUnsupportedModeIsAnError() {
	_, err := s.executor.Execute(s.nodeContext(adminContext(), "unknown", existingUserProperties()))

	s.Error(err)
}
