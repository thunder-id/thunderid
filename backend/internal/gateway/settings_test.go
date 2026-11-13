// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/serverconfig"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/serverconfigmock"
)

type SettingsTestSuite struct {
	suite.Suite
}

func TestSettingsTestSuite(t *testing.T) {
	suite.Run(t, new(SettingsTestSuite))
}

func boolPtr(b bool) *bool { return &b }

// The section decodes what it is given, an empty layer as unset, and refuses a misspelled field.
func (s *SettingsTestSuite) TestSettingsDecode() {
	h := NewSettingsConfigHandler()

	value, err := h.Decode(json.RawMessage(`{"autoApply": true}`))
	s.Require().NoError(err)
	s.Equal(settingsValue{AutoApply: boolPtr(true)}, value)

	value, err = h.Decode(nil)
	s.Require().NoError(err)
	s.Equal(settingsValue{}, value)

	_, err = h.Decode(json.RawMessage(`{"autoApplied": true}`))
	s.Error(err)
}

// A gateway section is accepted, and anything else is refused.
func (s *SettingsTestSuite) TestSettingsValidate() {
	h := NewSettingsConfigHandler()

	s.NoError(h.Validate(context.Background(), settingsValue{AutoApply: boolPtr(true)}, settingsValue{},
		settingsValue{}))
	s.Error(h.Validate(context.Background(), "not a section", nil, nil))
}

// A declarative document that sets anything locks the section, and Merge lays the section over
// gateway.auto_apply itself, so there is no separate default.
func (s *SettingsTestSuite) TestSettingsIsLocked() {
	h := NewSettingsConfigHandler()

	s.True(h.IsLocked(settingsValue{AutoApply: boolPtr(false)}))
	s.False(h.IsLocked(settingsValue{}))
	s.False(h.IsLocked(nil))
	s.Nil(h.Default())
}

// The effective value is the declarative document when there is one, else the stored one, each over
// gateway.auto_apply.
func (s *SettingsTestSuite) TestSettingsMerge() {
	h := settingsHandler{base: settingsValue{AutoApply: boolPtr(false)}}

	s.Equal(settingsValue{AutoApply: boolPtr(false)}, h.Merge(settingsValue{}, settingsValue{}))
	s.Equal(settingsValue{AutoApply: boolPtr(true)},
		h.Merge(settingsValue{}, settingsValue{AutoApply: boolPtr(true)}))
	s.Equal(settingsValue{AutoApply: boolPtr(true)},
		h.Merge(settingsValue{AutoApply: boolPtr(true)}, settingsValue{AutoApply: boolPtr(false)}))
}

// Auto apply follows the section, and falls back to gateway.auto_apply when there is no reader or the
// section cannot be read.
func (s *SettingsTestSuite) TestAutoApplySourceReadsTheSection() {
	ctx := context.Background()
	reader := serverconfigmock.NewServerConfigServiceMock(s.T())
	reader.EXPECT().GetMergedConfig(mock.Anything, string(serverconfig.ConfigNameGateway)).
		Return(settingsValue{AutoApply: boolPtr(true)}, nil).Once()
	reader.EXPECT().GetMergedConfig(mock.Anything, string(serverconfig.ConfigNameGateway)).
		Return(nil, &tidcommon.InternalServerError).Once()
	reader.EXPECT().GetMergedConfig(mock.Anything, string(serverconfig.ConfigNameGateway)).
		Return(settingsValue{}, nil).Once()
	source := autoApplySource{base: false, reader: reader, logger: log.GetLogger()}

	s.True(source.enabled(ctx))
	s.False(source.enabled(ctx), "an unreadable section uses gateway.auto_apply")
	s.False(source.enabled(ctx), "an unset section uses gateway.auto_apply")
	s.True(autoApplySource{base: true}.enabled(ctx))
	s.False(newAutoApplySource(nil).enabled(ctx))
}
