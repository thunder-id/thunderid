// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"github.com/thunder-id/thunderid/internal/system/config"
)

// getSelfServiceMaxAge returns the maximum age in seconds of a sign-in for a self-service change that proves no
// current credential.
func getSelfServiceMaxAge() int64 {
	return int64(config.GetServerRuntime().Config.User.SelfService.MaxAuthenticationAge())
}
