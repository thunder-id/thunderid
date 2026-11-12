// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"encoding/json"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// IsPersonalAgent reports whether the entity is an agent of the personal assistant kind.
func IsPersonalAgent(e providers.Entity) bool {
	if len(e.SystemAttributes) == 0 {
		return false
	}
	var sysAttrs map[string]interface{}
	if err := json.Unmarshal(e.SystemAttributes, &sysAttrs); err != nil {
		return false
	}
	kind, _ := sysAttrs[SystemAttributeKind].(string)
	return providers.AgentKind(kind) == providers.AgentKindPersonalAssistant
}
