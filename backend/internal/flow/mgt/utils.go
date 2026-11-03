// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowmgt

import (
	"regexp"
	"strconv"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// handleFormatRegex matches valid handle format:
// - starts with lowercase letter or digit
// - contains only lowercase letters, digits, underscores, or dashes
// - ends with lowercase letter or digit
var handleFormatRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*[a-z0-9]$|^[a-z0-9]$`)

// isValidHandleFormat validates that the handle follows the required format.
func isValidHandleFormat(handle string) bool {
	return handleFormatRegex.MatchString(handle)
}

// isValidDefaultForType checks that a default value can be read as the given input type.
// Types without a value format accept any default.
func isValidDefaultForType(inputType, value string) bool {
	switch inputType {
	case providers.InputTypeBoolean:
		_, err := strconv.ParseBool(value)
		return err == nil
	case providers.InputTypeNumber:
		_, err := strconv.ParseFloat(value, 64)
		return err == nil
	default:
		return true
	}
}

// isValidFlowType checks if the provided flow type is valid.
func isValidFlowType(flowType providers.FlowType) bool {
	for _, valid := range providers.ValidFlowTypes {
		if flowType == valid {
			return true
		}
	}
	return false
}
