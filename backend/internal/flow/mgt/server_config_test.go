// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowmgt

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type FlowConfigHandlerTestSuite struct {
	suite.Suite
	handler *FlowConfigHandler
}

func TestFlowConfigHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(FlowConfigHandlerTestSuite))
}

func (s *FlowConfigHandlerTestSuite) SetupTest() {
	s.handler = NewFlowConfigHandler()
}

func (s *FlowConfigHandlerTestSuite) TestDecode_NilInput() {
	result, err := s.handler.Decode(nil)
	s.NoError(err)
	s.Equal(flowconfig.FlowSectionConfig{}, result)
}

func (s *FlowConfigHandlerTestSuite) TestDecode_EmptyBytes() {
	result, err := s.handler.Decode(json.RawMessage{})
	s.NoError(err)
	s.Equal(flowconfig.FlowSectionConfig{}, result)
}

func (s *FlowConfigHandlerTestSuite) TestDecode_ValidJSON() {
	raw := json.RawMessage(`{"authFlow":{"defaultHandle":"my-auth","expirySeconds":900}}`)

	result, err := s.handler.Decode(raw)
	s.Require().NoError(err)

	cfg, ok := result.(flowconfig.FlowSectionConfig)
	s.Require().True(ok)
	s.Equal("my-auth", cfg.AuthFlow.DefaultHandle)
	s.Equal(int64(900), cfg.AuthFlow.ExpirySeconds)
}

func (s *FlowConfigHandlerTestSuite) TestDecode_InvalidJSON() {
	_, err := s.handler.Decode(json.RawMessage(`{invalid`))
	s.Error(err)
}

func (s *FlowConfigHandlerTestSuite) TestValidate_WrongType() {
	err := s.handler.Validate(context.Background(), "not-a-config", nil, nil)
	s.Error(err)
}

func (s *FlowConfigHandlerTestSuite) TestValidate_NegativeExpiry() {
	cfg := flowconfig.FlowSectionConfig{
		AuthFlow: flowconfig.FlowTypeConfig{ExpirySeconds: -1},
	}
	err := s.handler.Validate(context.Background(), cfg, nil, nil)
	s.Error(err)
}

func (s *FlowConfigHandlerTestSuite) TestValidate_ValidConfig() {
	cfg := flowconfig.FlowSectionConfig{
		AuthFlow:         flowconfig.FlowTypeConfig{DefaultHandle: "", ExpirySeconds: 1800},
		RegistrationFlow: flowconfig.FlowTypeConfig{ExpirySeconds: 3600},
		RecoveryFlow:     flowconfig.FlowTypeConfig{ExpirySeconds: 1800},
		SignOutFlow:      flowconfig.FlowTypeConfig{ExpirySeconds: 1800},
	}
	err := s.handler.Validate(context.Background(), cfg, nil, nil)
	s.NoError(err)
}

func (s *FlowConfigHandlerTestSuite) TestValidate_HandleValidatorCalled() {
	called := false
	s.handler.SetHandleValidator(func(_ context.Context, handle string, _ providers.FlowType) bool {
		called = true
		return handle == "valid-handle"
	})

	cfg := flowconfig.FlowSectionConfig{
		AuthFlow: flowconfig.FlowTypeConfig{DefaultHandle: "valid-handle"},
	}
	err := s.handler.Validate(context.Background(), cfg, nil, nil)
	s.NoError(err)
	s.True(called)
}

func (s *FlowConfigHandlerTestSuite) TestValidate_HandleValidatorRejectsUnknown() {
	s.handler.SetHandleValidator(func(_ context.Context, _ string, _ providers.FlowType) bool {
		return false
	})

	cfg := flowconfig.FlowSectionConfig{
		SignOutFlow: flowconfig.FlowTypeConfig{DefaultHandle: "nonexistent"},
	}
	err := s.handler.Validate(context.Background(), cfg, nil, nil)
	s.Error(err)
}

func (s *FlowConfigHandlerTestSuite) TestValidate_NoValidatorSkipsHandleCheck() {
	cfg := flowconfig.FlowSectionConfig{
		AuthFlow: flowconfig.FlowTypeConfig{DefaultHandle: "any-handle"},
	}
	err := s.handler.Validate(context.Background(), cfg, nil, nil)
	s.NoError(err)
}

func (s *FlowConfigHandlerTestSuite) TestMerge_WritableWins() {
	ro := flowconfig.FlowSectionConfig{
		AuthFlow: flowconfig.FlowTypeConfig{DefaultHandle: "ro-handle", ExpirySeconds: 1800},
	}
	wr := flowconfig.FlowSectionConfig{
		AuthFlow: flowconfig.FlowTypeConfig{DefaultHandle: "wr-handle", ExpirySeconds: 900},
	}

	result := s.handler.Merge(ro, wr).(flowconfig.FlowSectionConfig)
	s.Equal("wr-handle", result.AuthFlow.DefaultHandle)
	s.Equal(int64(900), result.AuthFlow.ExpirySeconds)
}

func (s *FlowConfigHandlerTestSuite) TestMerge_ReadOnlyFallsBackWhenWritableZero() {
	ro := flowconfig.FlowSectionConfig{
		AuthFlow: flowconfig.FlowTypeConfig{DefaultHandle: "ro-handle", ExpirySeconds: 1800},
	}
	wr := flowconfig.FlowSectionConfig{
		AuthFlow: flowconfig.FlowTypeConfig{DefaultHandle: "", ExpirySeconds: 0},
	}

	result := s.handler.Merge(ro, wr).(flowconfig.FlowSectionConfig)
	s.Equal("ro-handle", result.AuthFlow.DefaultHandle)
	s.Equal(int64(1800), result.AuthFlow.ExpirySeconds)
}

func (s *FlowConfigHandlerTestSuite) TestMerge_NilInputsReturnZero() {
	result := s.handler.Merge(nil, nil).(flowconfig.FlowSectionConfig)
	s.Equal(flowconfig.FlowSectionConfig{}, result)
}

func (s *FlowConfigHandlerTestSuite) TestMergeFlowTypeConfig_WritableHandleWins() {
	ro := flowconfig.FlowTypeConfig{DefaultHandle: "ro", ExpirySeconds: 500}
	wr := flowconfig.FlowTypeConfig{DefaultHandle: "wr", ExpirySeconds: 0}

	merged := mergeFlowTypeConfig(ro, wr)
	s.Equal("wr", merged.DefaultHandle)
	s.Equal(int64(500), merged.ExpirySeconds)
}

func (s *FlowConfigHandlerTestSuite) TestMergeFlowTypeConfig_WritableExpiryWins() {
	ro := flowconfig.FlowTypeConfig{DefaultHandle: "ro", ExpirySeconds: 500}
	wr := flowconfig.FlowTypeConfig{DefaultHandle: "", ExpirySeconds: 900}

	merged := mergeFlowTypeConfig(ro, wr)
	s.Equal("ro", merged.DefaultHandle)
	s.Equal(int64(900), merged.ExpirySeconds)
}

// ---------------------------------------------------------------------------
// User deletion flow
// ---------------------------------------------------------------------------

// An unset handle is valid: it is how a deployment opts out of flow-based deletion and keeps the
// native endpoint.
func (s *FlowConfigHandlerTestSuite) TestValidate_UserDeletionHandleOptional() {
	s.NoError(s.handler.Validate(context.Background(), flowconfig.FlowSectionConfig{}, nil, nil))
}

// The handle must name an administration flow, not merely exist.
func (s *FlowConfigHandlerTestSuite) TestValidate_UserDeletionHandleCheckedAgainstAdministration() {
	var gotType providers.FlowType
	s.handler.SetHandleValidator(func(_ context.Context, _ string, flowType providers.FlowType) bool {
		gotType = flowType
		return true
	})
	cfg := flowconfig.FlowSectionConfig{
		UserDeletionFlow: flowconfig.FlowTypeConfig{DefaultHandle: "default-user-deletion-flow"},
	}

	s.Require().NoError(s.handler.Validate(context.Background(), cfg, nil, nil))
	s.Equal(providers.FlowTypeAdministration, gotType)
}

func (s *FlowConfigHandlerTestSuite) TestValidate_UserDeletionHandleRejectedWhenNotAdministration() {
	s.handler.SetHandleValidator(func(_ context.Context, _ string, _ providers.FlowType) bool {
		return false
	})
	cfg := flowconfig.FlowSectionConfig{
		UserDeletionFlow: flowconfig.FlowTypeConfig{DefaultHandle: "not-an-admin-flow"},
	}

	err := s.handler.Validate(context.Background(), cfg, nil, nil)

	s.Require().Error(err)
	s.Contains(err.Error(), "userDeletionFlow.defaultHandle")
}

// An operator can repoint deletion at their own administration flow through the writable layer.
func (s *FlowConfigHandlerTestSuite) TestMerge_WritableUserDeletionHandleWins() {
	ro := flowconfig.FlowSectionConfig{
		UserDeletionFlow: flowconfig.FlowTypeConfig{DefaultHandle: "default-user-deletion-flow"},
	}
	wr := flowconfig.FlowSectionConfig{
		UserDeletionFlow: flowconfig.FlowTypeConfig{DefaultHandle: "acme-offboarding"},
	}

	merged, ok := s.handler.Merge(ro, wr).(flowconfig.FlowSectionConfig)

	s.Require().True(ok)
	s.Equal("acme-offboarding", merged.UserDeletionFlow.DefaultHandle)
}

func (s *FlowConfigHandlerTestSuite) TestMerge_EmptyWritableKeepsDeclarativeUserDeletionHandle() {
	ro := flowconfig.FlowSectionConfig{
		UserDeletionFlow: flowconfig.FlowTypeConfig{DefaultHandle: "default-user-deletion-flow"},
	}

	merged, _ := s.handler.Merge(ro, flowconfig.FlowSectionConfig{}).(flowconfig.FlowSectionConfig)

	s.Equal("default-user-deletion-flow", merged.UserDeletionFlow.DefaultHandle)
}
