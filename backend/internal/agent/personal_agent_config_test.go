// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
)

type PersonalAgentConfigTestSuite struct {
	suite.Suite
}

func TestPersonalAgentConfigTestSuite(t *testing.T) {
	suite.Run(t, new(PersonalAgentConfigTestSuite))
}

func boolPtr(v bool) *bool {
	return &v
}

func (s *PersonalAgentConfigTestSuite) TestDefaults() {
	cfg := PersonalAgentConfig{}
	s.False(cfg.IsEnabled())
	s.Equal(defaultPersonalAgentMaxAccessTokenValidity, cfg.EffectiveMaxAccessTokenValidity())
	s.Equal(defaultPersonalAgentAuthFlowHandle, cfg.EffectiveAuthFlowHandle())
}

func (s *PersonalAgentConfigTestSuite) TestConfiguredValues() {
	cfg := PersonalAgentConfig{
		Enabled:                boolPtr(true),
		MaxAccessTokenValidity: 900,
		AuthFlow:               PersonalAgentAuthFlowConfig{DefaultHandle: "custom-flow"},
	}
	s.True(cfg.IsEnabled())
	s.Equal(int64(900), cfg.EffectiveMaxAccessTokenValidity())
	s.Equal("custom-flow", cfg.EffectiveAuthFlowHandle())

	s.False(PersonalAgentConfig{Enabled: boolPtr(false)}.IsEnabled())
}

func (s *PersonalAgentConfigTestSuite) TestDecode() {
	h := NewPersonalAgentConfigHandler()

	got, err := h.Decode(nil)
	s.Require().NoError(err)
	s.Equal(PersonalAgentConfig{}, got)

	got, err = h.Decode(json.RawMessage(
		`{"enabled":true,"maxAccessTokenValidity":600,"authFlow":{"defaultHandle":"pa-flow"}}`))
	s.Require().NoError(err)
	s.Equal(PersonalAgentConfig{
		Enabled:                boolPtr(true),
		MaxAccessTokenValidity: 600,
		AuthFlow:               PersonalAgentAuthFlowConfig{DefaultHandle: "pa-flow"},
	}, got)

	_, err = h.Decode(json.RawMessage(`{`))
	s.Error(err)
}

func (s *PersonalAgentConfigTestSuite) TestValidate() {
	validated := ""
	h := NewPersonalAgentConfigHandler()
	h.SetAuthFlowHandleValidator(func(_ context.Context, handle string) bool {
		validated = handle
		return handle == "existing-auth-flow"
	})

	s.NoError(h.Validate(PersonalAgentConfig{}, nil, nil))
	s.NoError(h.Validate(PersonalAgentConfig{
		Enabled:                boolPtr(true),
		MaxAccessTokenValidity: 3600,
		AuthFlow:               PersonalAgentAuthFlowConfig{DefaultHandle: "existing-auth-flow"},
	}, nil, nil))
	s.Equal("existing-auth-flow", validated)

	s.ErrorContains(h.Validate(PersonalAgentConfig{MaxAccessTokenValidity: -1}, nil, nil),
		"maxAccessTokenValidity must be >= 0")
	s.ErrorContains(h.Validate(PersonalAgentConfig{
		AuthFlow: PersonalAgentAuthFlowConfig{DefaultHandle: "missing-flow"},
	}, nil, nil), `authFlow.defaultHandle "missing-flow"`)
	s.ErrorContains(h.Validate("not a config", nil, nil), "unexpected config type")
}

// Declarative files load before the validator is injected, so the handle is not checked then.
func (s *PersonalAgentConfigTestSuite) TestValidate_NoValidator() {
	h := NewPersonalAgentConfigHandler()
	s.NoError(h.Validate(PersonalAgentConfig{
		AuthFlow: PersonalAgentAuthFlowConfig{DefaultHandle: "any-flow"},
	}, nil, nil))
}

func (s *PersonalAgentConfigTestSuite) TestMerge() {
	h := NewPersonalAgentConfigHandler()
	defaults := PersonalAgentConfig{
		Enabled:                boolPtr(false),
		MaxAccessTokenValidity: defaultPersonalAgentMaxAccessTokenValidity,
		AuthFlow:               PersonalAgentAuthFlowConfig{DefaultHandle: defaultPersonalAgentAuthFlowHandle},
	}

	// Nothing configured: the merged value holds the defaults.
	s.Equal(defaults, h.Merge(nil, nil))
	s.Equal(defaults, h.Merge(PersonalAgentConfig{}, PersonalAgentConfig{}))

	readOnly := PersonalAgentConfig{
		Enabled:                boolPtr(true),
		MaxAccessTokenValidity: 1800,
		AuthFlow:               PersonalAgentAuthFlowConfig{DefaultHandle: "ro-flow"},
	}
	// An unset writable layer keeps every read-only value.
	s.Equal(readOnly, h.Merge(readOnly, PersonalAgentConfig{}))

	// A set writable value wins, including a false that overrides a read-only true.
	writable := PersonalAgentConfig{
		Enabled:                boolPtr(false),
		MaxAccessTokenValidity: 600,
		AuthFlow:               PersonalAgentAuthFlowConfig{DefaultHandle: "wr-flow"},
	}
	s.Equal(writable, h.Merge(readOnly, writable))

	// A partial writable layer overrides only what it sets.
	s.Equal(PersonalAgentConfig{
		Enabled:                boolPtr(true),
		MaxAccessTokenValidity: defaultPersonalAgentMaxAccessTokenValidity,
		AuthFlow:               PersonalAgentAuthFlowConfig{DefaultHandle: defaultPersonalAgentAuthFlowHandle},
	}, h.Merge(nil, PersonalAgentConfig{Enabled: boolPtr(true)}))
}
