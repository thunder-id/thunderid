// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// autoApplyFixture is a version fixture whose only gateway is the default one, with auto apply set.
func autoApplyFixture(t *testing.T, enabled, isDefault bool) *versionFixture {
	t.Helper()
	f := newVersionFixture(t)
	svc := f.svc.(*versionService)
	svc.autoApply = enabled
	svc.gateways.(*fakeStore).gateways[0].IsDefault = isDefault
	f.capture(t, "ou-a")
	return f
}

func TestAutoApplyAppliesTheLatestVersionToTheDefaultGateway(t *testing.T) {
	f := autoApplyFixture(t, true, true)

	f.svc.(*versionService).applyToDefaultGateway(context.Background())

	assert.Len(t, f.client.sent, 1)
}

func TestAutoApplyAppliesAgainWhenTheConfigurationIsTheSame(t *testing.T) {
	f := autoApplyFixture(t, true, true)
	svc := f.svc.(*versionService)

	svc.applyToDefaultGateway(context.Background())
	svc.applyToDefaultGateway(context.Background())

	assert.Len(t, f.client.sent, 2, "a changed secret leaves the version as it was, so it is sent again")
}

func TestAutoApplyDoesNothingWhenItIsOff(t *testing.T) {
	f := autoApplyFixture(t, false, true)

	f.svc.(*versionService).applyToDefaultGateway(context.Background())

	assert.Empty(t, f.client.sent)
}

func TestAutoApplyDoesNothingWithoutADefaultGateway(t *testing.T) {
	f := autoApplyFixture(t, true, false)

	f.svc.(*versionService).applyToDefaultGateway(context.Background())

	assert.Empty(t, f.client.sent)
}

// A gateway that cannot take the version is only logged: the capture it follows stands, and the
// version stays the latest one for an operator to apply.
func TestAutoApplyLeavesTheVersionWhenTheGatewayRefusesIt(t *testing.T) {
	f := autoApplyFixture(t, true, true)
	f.client.err = errors.New("connection refused")

	f.svc.(*versionService).applyToDefaultGateway(context.Background())

	assert.Len(t, f.client.sent, 1, "the apply was tried once")
	held, _ := f.svc.GetApplied(context.Background(), "gw-1")
	assert.Zero(t, held.AppliedVersion, "a refused apply records nothing")
}

// A store that cannot list the gateways ends the apply without sending anything.
func TestAutoApplyDoesNothingWhenTheGatewaysCannotBeListed(t *testing.T) {
	f := autoApplyFixture(t, true, true)
	f.svc.(*versionService).gateways.(*fakeStore).err = errors.New("database is down")

	f.svc.(*versionService).applyToDefaultGateway(context.Background())

	assert.Empty(t, f.client.sent)
}

// The capture that follows a change applies the version it captured, so a write reaches the default
// gateway without anyone applying anything.
func TestAChangeCaptureAppliesTheVersionItCaptured(t *testing.T) {
	f := newVersionFixture(t)
	svc := f.svc.(*versionService)
	svc.autoApply = true
	svc.gateways.(*fakeStore).gateways[0].IsDefault = true
	f.exporter.response = exportOf("", "ou-a")

	capture := NewChangeCapture()
	capture.bind(svc)

	assert.NoError(t, capture.capture(context.Background()))
	assert.Len(t, f.client.sent, 1)
}
