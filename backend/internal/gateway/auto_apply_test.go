// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixedAutoApply is an auto apply setting that does not change.
type fixedAutoApply bool

func (f fixedAutoApply) enabled(context.Context) bool { return bool(f) }

func (fixedAutoApply) projectSettings(context.Context) map[string]ProjectSyncSetting { return nil }

// autoApplyFixture is a version fixture whose only gateway is the default one, with auto apply set.
func autoApplyFixture(t *testing.T, enabled, isDefault bool) *versionFixture {
	t.Helper()
	f := newVersionFixture(t)
	svc := f.svc.(*versionService)
	svc.autoApply = fixedAutoApply(enabled)
	svc.gateways.(*fakeStore).gateways[0].IsDefault = isDefault
	f.capture(t, "ou-a")
	return f
}

func TestAutoApplyAppliesTheLatestVersionToTheDefaultGateway(t *testing.T) {
	f := autoApplyFixture(t, true, true)

	f.svc.(*versionService).applyToFirstGateway(context.Background(), nil)

	assert.Len(t, f.client.sent, 1)
}

func TestAutoApplyAppliesAgainWhenTheConfigurationIsTheSame(t *testing.T) {
	f := autoApplyFixture(t, true, true)
	svc := f.svc.(*versionService)
	f.captureReferring(t)
	f.client.held = map[string]bool{"APP_CLIENT_ID": true, "APP_SECRET": true}
	set := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	f.client.heldSince = map[string]time.Time{"APP_CLIENT_ID": set, "APP_SECRET": set}

	svc.applyToFirstGateway(context.Background(), nil)
	svc.applyToFirstGateway(context.Background(), nil)
	require.Len(t, f.client.sent, 1, "a version the gateway holds as it is was sent again")

	// A captured secret leaves the version as it was and sets the value on the gateway again, so the
	// resource that refers to it is sent again to read it.
	f.client.heldSince["APP_SECRET"] = set.Add(time.Minute)
	svc.applyToFirstGateway(context.Background(), nil)

	require.Len(t, f.client.sent, 2, "a changed secret did not send the resource again")
	assert.Contains(t, f.client.last().Content, "app-1")
}

func TestAutoApplyDoesNothingWhenItIsOff(t *testing.T) {
	f := autoApplyFixture(t, false, true)

	f.svc.(*versionService).applyToFirstGateway(context.Background(), nil)

	assert.Empty(t, f.client.sent)
}

func TestAutoApplyDoesNothingWithoutADefaultGateway(t *testing.T) {
	f := autoApplyFixture(t, true, false)

	f.svc.(*versionService).applyToFirstGateway(context.Background(), nil)

	assert.Empty(t, f.client.sent)
}

// A gateway that cannot take the version is only logged: the capture it follows stands, and the
// version stays the latest one for an operator to apply.
func TestAutoApplyLeavesTheVersionWhenTheGatewayRefusesIt(t *testing.T) {
	f := autoApplyFixture(t, true, true)
	f.client.err = errors.New("connection refused")

	f.svc.(*versionService).applyToFirstGateway(context.Background(), nil)

	assert.Len(t, f.client.sent, 1, "the apply was tried once")
	held, _ := f.svc.GetApplied(context.Background(), "gw-1")
	assert.Zero(t, held.AppliedVersion, "a refused apply records nothing")
}

// A store that cannot list the gateways ends the apply without sending anything.
func TestAutoApplyDoesNothingWhenTheGatewaysCannotBeListed(t *testing.T) {
	f := autoApplyFixture(t, true, true)
	f.svc.(*versionService).gateways.(*fakeStore).err = errors.New("database is down")

	f.svc.(*versionService).applyToFirstGateway(context.Background(), nil)

	assert.Empty(t, f.client.sent)
}

// The capture that follows a change applies the version it captured, so a write reaches the default
// gateway without anyone applying anything.
func TestAChangeCaptureAppliesTheVersionItCaptured(t *testing.T) {
	f := newVersionFixture(t)
	svc := f.svc.(*versionService)
	svc.autoApply = fixedAutoApply(true)
	svc.gateways.(*fakeStore).gateways[0].IsDefault = true
	f.exporter.response = exportOf("", "ou-a")

	capture := NewChangeCapture()
	capture.bind(svc)

	assert.NoError(t, capture.capture(context.Background()))
	assert.Len(t, f.client.sent, 1)
}

// Going back to what an older version holds, as a revert does, captures a new version, the latest, and
// the default gateway gets it.
func TestAutoApplyAppliesTheVersionTheConfigurationWentBackTo(t *testing.T) {
	f := autoApplyFixture(t, true, true)
	svc := f.svc.(*versionService)
	f.capture(t, "ou-b")
	back := f.capture(t, "ou-a")
	require.False(t, back.Unchanged)
	require.Equal(t, 3, back.Seq)

	svc.applyToFirstGateway(context.Background(), back)

	held, _ := f.svc.GetApplied(context.Background(), "gw-1")
	assert.Equal(t, 3, held.AppliedVersion)
}

// When another capture came in after it, the captured version is applied by its hash, not as "latest".
func TestAutoApplyAppliesTheCapturedVersionWhenItIsNotTheLatest(t *testing.T) {
	f := autoApplyFixture(t, true, true)
	svc := f.svc.(*versionService)
	captured := f.capture(t, "ou-b")
	f.capture(t, "ou-c")

	svc.applyToFirstGateway(context.Background(), captured)

	held, _ := f.svc.GetApplied(context.Background(), "gw-1")
	assert.Equal(t, captured.Seq, held.AppliedVersion)
}
