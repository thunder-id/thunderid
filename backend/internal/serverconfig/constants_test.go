// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultResourceServerConfigNameSupported(t *testing.T) {
	assert.Equal(t, ConfigName("defaultResourceServer"), ConfigNameDefaultResourceServer)
	assert.True(t, ConfigNameDefaultResourceServer.IsValid())
	assert.Contains(t, supportedConfigNames, ConfigNameDefaultResourceServer)
}

func TestAccountAccessConfigNameSupported(t *testing.T) {
	assert.Equal(t, ConfigName("accountAccess"), ConfigNameAccountAccess)
	assert.True(t, ConfigNameAccountAccess.IsValid())
	assert.Contains(t, supportedConfigNames, ConfigNameAccountAccess)
}

func TestNotificationConfigNameSupported(t *testing.T) {
	assert.Equal(t, ConfigName("notification"), ConfigNameNotification)
	assert.True(t, ConfigNameNotification.IsValid())
	assert.Contains(t, supportedConfigNames, ConfigNameNotification)
}

func TestPersonalAgentConfigNameSupported(t *testing.T) {
	assert.Equal(t, ConfigName("personalAgent"), ConfigNamePersonalAgent)
	assert.True(t, ConfigNamePersonalAgent.IsValid())
	assert.Contains(t, supportedConfigNames, ConfigNamePersonalAgent)
}
