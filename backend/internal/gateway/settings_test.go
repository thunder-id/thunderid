// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/serverconfig"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/serverconfigmock"
)

func boolPtr(b bool) *bool { return &b }

// The section decodes what it is given, an empty layer as unset, and refuses a misspelled field.
func TestSettingsDecode(t *testing.T) {
	h := NewSettingsConfigHandler()

	value, err := h.Decode(json.RawMessage(`{"autoApply": true}`))
	require.NoError(t, err)
	assert.Equal(t, settingsValue{AutoApply: boolPtr(true)}, value)

	value, err = h.Decode(nil)
	require.NoError(t, err)
	assert.Equal(t, settingsValue{}, value)

	_, err = h.Decode(json.RawMessage(`{"autoApplied": true}`))
	assert.Error(t, err)
}

// A write is refused while a declarative document sets the section, and accepted otherwise.
func TestSettingsValidate(t *testing.T) {
	h := NewSettingsConfigHandler()
	incoming := settingsValue{AutoApply: boolPtr(true)}

	assert.NoError(t, h.Validate(context.Background(), incoming, settingsValue{}, settingsValue{}))
	assert.ErrorIs(t, h.Validate(context.Background(), incoming, settingsValue{AutoApply: boolPtr(false)}, nil),
		errDeclarativeSettingsLocked)
	assert.Error(t, h.Validate(context.Background(), "not a section", nil, nil))
}

// The effective value is the declarative document when there is one, else the stored one, each over
// gateway.auto_apply.
func TestSettingsMerge(t *testing.T) {
	h := settingsHandler{base: settingsValue{AutoApply: boolPtr(false)}}

	assert.Equal(t, settingsValue{AutoApply: boolPtr(false)}, h.Merge(settingsValue{}, settingsValue{}))
	assert.Equal(t, settingsValue{AutoApply: boolPtr(true)},
		h.Merge(settingsValue{}, settingsValue{AutoApply: boolPtr(true)}))
	assert.Equal(t, settingsValue{AutoApply: boolPtr(true)},
		h.Merge(settingsValue{AutoApply: boolPtr(true)}, settingsValue{AutoApply: boolPtr(false)}))
}

// Auto apply follows the section, and falls back to gateway.auto_apply when there is no reader or the
// section cannot be read.
func TestAutoApplySourceReadsTheSection(t *testing.T) {
	ctx := context.Background()
	reader := serverconfigmock.NewServerConfigServiceMock(t)
	reader.EXPECT().GetMergedConfig(mock.Anything, string(serverconfig.ConfigNameGateway)).
		Return(settingsValue{AutoApply: boolPtr(true)}, nil).Once()
	reader.EXPECT().GetMergedConfig(mock.Anything, string(serverconfig.ConfigNameGateway)).
		Return(nil, &tidcommon.InternalServerError).Once()
	reader.EXPECT().GetMergedConfig(mock.Anything, string(serverconfig.ConfigNameGateway)).
		Return(settingsValue{}, nil).Once()
	source := autoApplySource{base: false, reader: reader, logger: log.GetLogger()}

	assert.True(t, source.enabled(ctx))
	assert.False(t, source.enabled(ctx), "an unreadable section uses gateway.auto_apply")
	assert.False(t, source.enabled(ctx), "an unset section uses gateway.auto_apply")
	assert.True(t, autoApplySource{base: true}.enabled(ctx))
	assert.False(t, newAutoApplySource(nil).enabled(ctx))
}
