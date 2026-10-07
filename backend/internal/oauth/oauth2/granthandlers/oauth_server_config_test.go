// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package granthandlers

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// stubGraceConfigReader serves a fixed oauth section value, or an error, in place of the
// server-config service.
type stubGraceConfigReader struct {
	cfg OAuthServerConfig
	err *common.ServiceError
	// wrongType returns a value of an unexpected type, standing in for a section decoded by a
	// different handler.
	wrongType bool
}

func (s *stubGraceConfigReader) GetMergedConfig(_ context.Context, _ string) (any, *common.ServiceError) {
	if s == nil {
		return OAuthServerConfig{}, nil
	}
	if s.err != nil {
		return nil, s.err
	}
	if s.wrongType {
		return "not an oauth server config", nil
	}
	return s.cfg, nil
}

// gracePolicy builds an oauth section value carrying the given rotation grace policy.
func gracePolicy(enabled bool, ceilingSeconds int64) OAuthServerConfig {
	return OAuthServerConfig{RefreshToken: RefreshTokenServerConfig{
		GraceEnabled:        &enabled,
		GraceCeilingSeconds: ceilingSeconds,
	}}
}

func refreshTokenPolicy(enabled *bool, ceilingSeconds int64) RefreshTokenServerConfig {
	return RefreshTokenServerConfig{GraceEnabled: enabled, GraceCeilingSeconds: ceilingSeconds}
}

type OAuthServerConfigTestSuite struct {
	suite.Suite
}

func TestOAuthServerConfigSuite(t *testing.T) {
	suite.Run(t, new(OAuthServerConfigTestSuite))
}

func (suite *OAuthServerConfigTestSuite) TestValidate() {
	enabled, disabled := true, false
	tests := []struct {
		name      string
		incoming  OAuthServerConfig
		readOnly  any
		expectErr bool
	}{
		{"unset is valid", OAuthServerConfig{}, nil, false},
		{"enabled with a ceiling is valid", gracePolicy(true, 30), nil, false},
		{"disabled with a ceiling is valid", gracePolicy(false, 30), nil, false},
		{"explicitly disabled with no ceiling is valid", gracePolicy(false, 0), nil, false},
		{"negative ceiling is rejected", gracePolicy(false, -1), nil, true},
		{"enabled with no ceiling is rejected", gracePolicy(true, 0), nil, true},
		{"ceiling beyond the maximum is rejected", gracePolicy(true, maxGraceCeilingSeconds+1), nil, true},
		{"ceiling at the maximum is valid", gracePolicy(true, maxGraceCeilingSeconds), nil, false},
		// A write replaces only the writable layer, so coherence is judged on the policy that will
		// take effect: switching the feature on is coherent when the declarative layer has a ceiling.
		{
			"enabling over a declarative ceiling is valid",
			OAuthServerConfig{RefreshToken: refreshTokenPolicy(&enabled, 0)},
			gracePolicy(false, 30),
			false,
		},
		{
			"enabling with no ceiling anywhere is rejected",
			OAuthServerConfig{RefreshToken: refreshTokenPolicy(&enabled, 0)},
			OAuthServerConfig{},
			true,
		},
		{
			"disabling over a declarative enable is valid",
			OAuthServerConfig{RefreshToken: refreshTokenPolicy(&disabled, 0)},
			gracePolicy(true, 30),
			false,
		},
	}
	handler := OAuthServerConfigHandler{}
	for _, tt := range tests {
		suite.Run(tt.name, func() {
			err := handler.Validate(tt.incoming, tt.readOnly, nil)
			if tt.expectErr {
				assert.Error(suite.T(), err)
				return
			}
			assert.NoError(suite.T(), err)
		})
	}
}

// A disabled policy reports no ceiling whatever value it carries, so a stale ceiling cannot be
// honored after the feature is switched off.
func (suite *OAuthServerConfigTestSuite) TestCeiling() {
	tests := []struct {
		name     string
		cfg      RefreshTokenServerConfig
		expected time.Duration
	}{
		{"enabled reports its ceiling", gracePolicy(true, 30).RefreshToken, 30 * time.Second},
		{"disabled reports zero despite a ceiling", gracePolicy(false, 30).RefreshToken, 0},
		{"unset switch reports zero despite a ceiling", refreshTokenPolicy(nil, 30), 0},
		{"enabled with no ceiling reports zero", gracePolicy(true, 0).RefreshToken, 0},
		{"negative ceiling reports zero", gracePolicy(true, -5).RefreshToken, 0},
	}
	for _, tt := range tests {
		suite.Run(tt.name, func() {
			assert.Equal(suite.T(), tt.expected, tt.cfg.Ceiling())
		})
	}
}

func (suite *OAuthServerConfigTestSuite) TestDecode() {
	handler := OAuthServerConfigHandler{}

	empty, err := handler.Decode(nil)
	suite.Require().NoError(err)
	assert.Equal(suite.T(), OAuthServerConfig{}, empty)

	decoded, err := handler.Decode(json.RawMessage(
		`{"refreshToken":{"graceEnabled":true,"graceCeilingSeconds":30}}`))
	suite.Require().NoError(err)
	assert.Equal(suite.T(), gracePolicy(true, 30), decoded)

	// A block that is absent decodes to an unset policy rather than failing, so a section written
	// before a block existed still loads.
	other, err := handler.Decode(json.RawMessage(`{}`))
	suite.Require().NoError(err)
	assert.Equal(suite.T(), OAuthServerConfig{}, other)

	_, err = handler.Decode(json.RawMessage(`not json`))
	assert.Error(suite.T(), err)
}

func (suite *OAuthServerConfigTestSuite) TestMerge() {
	handler := OAuthServerConfigHandler{}
	readOnly := gracePolicy(true, 30)

	unset := handler.Merge(readOnly, OAuthServerConfig{})
	assert.Equal(suite.T(), readOnly, unset, "an unwritten layer must leave the declarative value")

	overridden := handler.Merge(readOnly, gracePolicy(true, 10))
	assert.Equal(suite.T(), gracePolicy(true, 10), overridden)

	// An explicit false must switch the feature off over a declarative layer that enables it, even
	// with no ceiling written alongside it, since false is also the zero value of a plain bool.
	disabled := false
	off := handler.Merge(readOnly, OAuthServerConfig{RefreshToken: refreshTokenPolicy(&disabled, 0)})
	assert.Equal(suite.T(), gracePolicy(false, 30), off)

	// Fields merge independently: writing only a ceiling keeps the declarative switch.
	ceilingOnly := handler.Merge(readOnly, OAuthServerConfig{RefreshToken: refreshTokenPolicy(nil, 5)})
	assert.Equal(suite.T(), gracePolicy(true, 5), ceilingOnly)
}
