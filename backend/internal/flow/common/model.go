// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"fmt"
	"regexp"
	"strings"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// PrepareValidationRules compiles the regex pattern of every regex rule in place.
// An empty or non-string regex value is treated as a no-op.
func PrepareValidationRules(rules []providers.ValidationRule) error {
	for i := range rules {
		if rules[i].Type != providers.ValidationTypeRegex {
			continue
		}
		pattern, ok := rules[i].Value.(string)
		if !ok || pattern == "" {
			continue
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid validation regex %q: %w", pattern, err)
		}
		rules[i].CompiledRegex = re
	}
	return nil
}

// ExtensionProperties is an extension step's configuration, read from its node properties.
type ExtensionProperties struct {
	ConnectionID    string
	InputKeys       []string
	RuntimeDataKeys []string
	ResponseKeys    []string
	MaxAttempts     int
}

// ParseExtensionProperties reads an extension step's node properties, applying the defaults, and
// returns the name of the first property whose value cannot be used, or "" when all can. Flow save
// and the executor both apply it, so they share one rule.
func ParseExtensionProperties(properties map[string]interface{}) (ExtensionProperties, string) {
	var props ExtensionProperties
	connectionID, _ := properties[NodePropertyConnectionID].(string)
	if props.ConnectionID = strings.TrimSpace(connectionID); props.ConnectionID == "" {
		return props, NodePropertyConnectionID
	}
	var ok bool
	if props.InputKeys, ok = extensionKeyList(properties[NodePropertyInputKeys], false); !ok {
		return props, NodePropertyInputKeys
	}
	if props.RuntimeDataKeys, ok = extensionRuntimeDataKeys(properties[NodePropertyRuntimeDataKeys]); !ok {
		return props, NodePropertyRuntimeDataKeys
	}
	if props.ResponseKeys, ok = extensionKeyList(properties[NodePropertyResponseKeys], true); !ok {
		return props, NodePropertyResponseKeys
	}
	if props.MaxAttempts, ok = extensionMaxAttemptsValue(properties[NodePropertyMaxAttempts]); !ok {
		return props, NodePropertyMaxAttempts
	}
	return props, ""
}

// extensionKeyList reads a list of names given as a JSON array of strings or, when commaSeparated
// allows it, as a comma-separated string. nil is unset; an empty list or a blank name is unusable.
func extensionKeyList(value interface{}, commaSeparated bool) ([]string, bool) {
	var items []interface{}
	switch v := value.(type) {
	case nil:
		return nil, true
	case []interface{}:
		items = v
	case string:
		if !commaSeparated {
			return nil, false
		}
		for _, item := range strings.Split(v, ",") {
			items = append(items, item)
		}
	default:
		return nil, false
	}
	if len(items) == 0 {
		return nil, false
	}
	keys := make([]string, 0, len(items))
	for _, item := range items {
		key, isString := item.(string)
		if key = strings.TrimSpace(key); !isString || key == "" {
			return nil, false
		}
		keys = append(keys, key)
	}
	return keys, true
}

// extensionRuntimeDataKeys reads runtimeDataKeys, defaulting to the user ID. Only the user ID and
// extension data may be named: an allow list, so a runtime-data key added later is never sent
// without a decision to send it.
func extensionRuntimeDataKeys(value interface{}) ([]string, bool) {
	keys, ok := extensionKeyList(value, false)
	if !ok {
		return nil, false
	}
	if keys == nil {
		return []string{authnprovidercm.UserAttributeUserID}, true
	}
	for _, key := range keys {
		if key != authnprovidercm.UserAttributeUserID && !strings.HasPrefix(key, ExtensionDataKeyPrefix) {
			return nil, false
		}
	}
	return keys, true
}

// extensionMaxAttemptsValue reads maxAttempts as a whole number within its bounds. nil is the default.
func extensionMaxAttemptsValue(value interface{}) (int, bool) {
	if value == nil {
		return extensionDefaultAttempts, true
	}
	n, ok := utils.ToInt64(value)
	if f, isFloat := value.(float64); !ok || (isFloat && f != float64(n)) ||
		n < extensionMinAttempts || n > extensionMaxAttempts {
		return 0, false
	}
	return int(n), true
}

// FieldError represents a single validation rule failure for a specific input field.
type FieldError struct {
	Identifier string `json:"identifier"`
	Message    string `json:"message"`
}

// Action represents an action to be executed in a flow step
type Action struct {
	Ref      string `json:"ref,omitempty"`
	Type     string `json:"type,omitempty"`
	NextNode string `json:"nextNode,omitempty"`
}

// Prompt groups inputs with an action for prompt nodes.
type Prompt struct {
	Inputs []providers.Input `json:"inputs,omitempty"`
	Action *Action           `json:"action,omitempty"`
}

// NodeResponse represents the response from a node execution
type NodeResponse struct {
	Status            NodeStatus              `json:"status"`
	Type              NodeResponseType        `json:"type"`
	Error             *tidcommon.ServiceError `json:"error,omitempty"`
	Inputs            []providers.Input       `json:"inputs,omitempty"`
	AdditionalData    map[string]string       `json:"additionalData,omitempty"`
	RedirectURL       string                  `json:"redirectUrl,omitempty"`
	Actions           []Action                `json:"actions,omitempty"`
	Meta              interface{}             `json:"meta,omitempty"`
	NextNodeID        string                  `json:"nextNodeId,omitempty"`
	RuntimeData       map[string]string       `json:"runtimeData,omitempty"`
	SharedRuntimeData map[string]string       `json:"-"`
	ForwardedData     map[string]interface{}  `json:"forwardedData,omitempty"`
	Assertion         string                  `json:"assertion,omitempty"`
	FieldErrors       []FieldError            `json:"fieldErrors,omitempty"`
	AuthUser          providers.AuthUser      `json:"-"`
	// EngineData carries executor output the engine consumes internally; never serialized to the client.
	EngineData       map[string]string `json:"-"`
	CallTargetFlowID string            `json:"callTargetFlowId,omitempty"`
}

// InterceptorResponse represents the response from an interceptor execution
type InterceptorResponse struct {
	Status InterceptorStatus       `json:"status"`
	Error  *tidcommon.ServiceError `json:"error,omitempty"`

	// EngineOutputs passes data from interceptors back to the engine.
	EngineOutputs map[string]string `json:"engineOutputs,omitempty"`

	// Flow step data returned to the client.
	ChallengeToken string       `json:"challengeToken,omitempty"`
	FieldErrors    []FieldError `json:"fieldErrors,omitempty"`
}
