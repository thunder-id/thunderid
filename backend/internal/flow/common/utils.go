// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"strings"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
)

// ExtensionAuthResolverProperties is an extension auth resolver's configuration, read from its node
// properties.
type ExtensionAuthResolverProperties struct {
	SourceKey        string
	MatchAttribute   string
	AttributeMapping map[string]string
}

// ParseExtensionAuthResolverProperties reads an extension auth resolver's node properties, applying the
// defaults, and returns the name of the first property whose value cannot be used, or "" when all can.
func ParseExtensionAuthResolverProperties(properties map[string]interface{}) (
	ExtensionAuthResolverProperties, string) {
	props := ExtensionAuthResolverProperties{MatchAttribute: UserAttributeUsername}
	sourceKey, _ := properties[NodePropertySourceKey].(string)
	if props.SourceKey = strings.TrimSpace(sourceKey); !isExtensionDataKey(props.SourceKey) {
		return props, NodePropertySourceKey
	}

	if value := properties[NodePropertyMatchAttribute]; value != nil {
		matchAttribute, ok := value.(string)
		if !ok {
			return props, NodePropertyMatchAttribute
		}
		if matchAttribute = strings.TrimSpace(matchAttribute); matchAttribute != "" {
			props.MatchAttribute = matchAttribute
		}
		if props.MatchAttribute != UserAttributeUsername &&
			props.MatchAttribute != authnprovidercm.UserAttributeUserID {
			return props, NodePropertyMatchAttribute
		}
	}

	// A user ID that matches no one cannot be provisioned, so the provisioning properties do nothing with it.
	allowWithoutLocalUser, _ := properties[NodePropertyAllowAuthenticationWithoutLocalUser].(bool)
	if allowWithoutLocalUser && props.MatchAttribute == authnprovidercm.UserAttributeUserID {
		return props, NodePropertyAllowAuthenticationWithoutLocalUser
	}

	value := properties[NodePropertyAttributeMapping]
	if value == nil {
		return props, ""
	}
	mapping, ok := value.(map[string]interface{})
	if !ok {
		return props, NodePropertyAttributeMapping
	}
	if len(mapping) > 0 && props.MatchAttribute == authnprovidercm.UserAttributeUserID {
		return props, NodePropertyAttributeMapping
	}
	props.AttributeMapping = make(map[string]string, len(mapping))
	for attribute, key := range mapping {
		keyText, isString := key.(string)
		attribute, keyText = strings.TrimSpace(attribute), strings.TrimSpace(keyText)
		if !isString || attribute == "" || !isExtensionDataKey(keyText) {
			return props, NodePropertyAttributeMapping
		}
		props.AttributeMapping[attribute] = keyText
	}
	return props, ""
}

// ExtensionNamespace returns the prefix of the runtime-data keys the extension step nodeID stores.
func ExtensionNamespace(nodeID string) string {
	return ExtensionDataKeyPrefix + nodeID + "."
}

// isExtensionDataKey reports whether key names a field an extension step stored, as
// extension.<nodeId>.<field>.
func isExtensionDataKey(key string) bool {
	rest, found := strings.CutPrefix(key, ExtensionDataKeyPrefix)
	nodeID, field, hasField := strings.Cut(rest, ".")
	return found && hasField && nodeID != "" && field != ""
}
