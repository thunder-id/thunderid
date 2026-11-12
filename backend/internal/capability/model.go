// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package capability

import (
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
)

// capabilitiesResponse is the body of a capabilities response.
type capabilitiesResponse struct {
	Scope     capabilityScope                           `json:"scope"`
	Resources map[security.ResourceType]map[string]bool `json:"resources"`
}

// capabilityScope says where the reported actions apply.
type capabilityScope struct {
	Type sysauthz.BoundaryType `json:"type"`
	OUID string                `json:"ouId,omitempty"`
}
