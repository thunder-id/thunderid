/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package importer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	resourceTypeOrganizationUnit        = "organization_unit"
	resourceTypeEntityType              = "user_type"
	resourceTypeResourceServer          = "resource_server"
	resourceTypeRole                    = "role"
	resourceTypeGroup                   = "group"
	resourceTypeIdentityProvider        = "identity_provider"
	resourceTypeNotificationSender      = "notification_sender"
	resourceTypeFlow                    = "flow"
	resourceTypeTheme                   = "theme"
	resourceTypeLayout                  = "layout"
	resourceTypeApplication             = "application"
	resourceTypeUser                    = "user"
	resourceTypeTranslation             = "translation"
	resourceTypeAgent                   = "agent"
	resourceTypePresentationDefinition  = "presentation_definition"
	resourceTypeCredentialConfiguration = "credential_configuration" //nolint:gosec
	resourceTypeServerConfig            = "server_config"
	resourceTypeUnknown                 = "unknown"
)

type parsedDocument struct {
	ResourceType string
	Node         *yaml.Node
	Sequence     int
}

func parseDocuments(content string) ([]parsedDocument, error) {
	decoder := yaml.NewDecoder(bytes.NewReader([]byte(content)))

	docs := make([]parsedDocument, 0)
	for seq := 0; ; seq++ {
		var doc yaml.Node
		err := decoder.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to parse YAML document: %w", err)
		}

		if len(doc.Content) == 0 {
			continue
		}

		root := doc.Content[0]
		if root.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("document %d root must be a YAML mapping", seq+1)
		}

		resourceType := classifyResourceTypeWithComments(&doc, root)
		if resourceType == resourceTypeUnknown {
			return nil, fmt.Errorf("unable to determine resource type for document %d", seq+1)
		}

		docs = append(docs, parsedDocument{
			ResourceType: resourceType,
			Node:         root,
			Sequence:     seq,
		})
	}

	return docs, nil
}

func classifyResourceTypeWithComments(doc *yaml.Node, node *yaml.Node) string {
	commentCandidates := []string{
		doc.HeadComment,
		doc.LineComment,
		doc.FootComment,
		node.HeadComment,
		node.LineComment,
		node.FootComment,
	}
	for _, contentNode := range node.Content {
		commentCandidates = append(
			commentCandidates,
			contentNode.HeadComment,
			contentNode.LineComment,
			contentNode.FootComment,
		)
	}

	if resourceType := resourceTypeFromComments(commentCandidates...); resourceType != resourceTypeUnknown {
		return resourceType
	}

	return classifyResourceType(node)
}

func resourceTypeFromComments(comments ...string) string {
	for _, comment := range comments {
		if resourceType := parseResourceTypeFromComment(comment); resourceType != resourceTypeUnknown {
			return resourceType
		}
	}

	return resourceTypeUnknown
}

func parseResourceTypeFromComment(comment string) string {
	if strings.TrimSpace(comment) == "" {
		return resourceTypeUnknown
	}

	for _, line := range strings.Split(comment, "\n") {
		normalized := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "#")))
		if normalized == "" {
			continue
		}

		for _, prefix := range []string{"resource_type:", "resource type:", "resourcetype:"} {
			if !strings.HasPrefix(normalized, prefix) {
				continue
			}

			resourceType := strings.TrimSpace(strings.TrimPrefix(normalized, prefix))
			resourceType = strings.Trim(resourceType, "\"'")
			if isKnownResourceType(resourceType) {
				return resourceType
			}
		}
	}

	return resourceTypeUnknown
}

func isKnownResourceType(resourceType string) bool {
	knownTypes := map[string]struct{}{
		resourceTypeOrganizationUnit:        {},
		resourceTypeEntityType:              {},
		resourceTypeResourceServer:          {},
		resourceTypeRole:                    {},
		resourceTypeIdentityProvider:        {},
		resourceTypeNotificationSender:      {},
		resourceTypeFlow:                    {},
		resourceTypeTheme:                   {},
		resourceTypeLayout:                  {},
		resourceTypeApplication:             {},
		resourceTypeUser:                    {},
		resourceTypeGroup:                   {},
		resourceTypeTranslation:             {},
		resourceTypeAgent:                   {},
		resourceTypePresentationDefinition:  {},
		resourceTypeCredentialConfiguration: {},
		resourceTypeServerConfig:            {},
	}

	_, exists := knownTypes[resourceType]
	return exists
}

func classifyResourceType(node *yaml.Node) string {
	matches := make([]string, 0, 4)

	if hasAllKeys(node, "language", "translations") {
		matches = append(matches, resourceTypeTranslation)
	}

	if hasAllKeys(node, "schema") && hasAnyKey(node, "ouId", "ouHandle") {
		matches = append(matches, resourceTypeEntityType)
	}

	if hasAllKeys(node, "identifier", "resources") {
		matches = append(matches, resourceTypeResourceServer)
	}

	if hasAllKeys(node, "name", "permissions") {
		matches = append(matches, resourceTypeRole)
	}

	if hasAllKeys(node, "displayName", "theme") {
		matches = append(matches, resourceTypeTheme)
	}

	if hasAllKeys(node, "displayName", "layout") {
		matches = append(matches, resourceTypeLayout)
	}

	if hasAllKeys(node, "type", "attributes") && hasAnyKey(node, "ouId", "ouHandle") {
		matches = append(matches, resourceTypeUser)
	}

	if hasAllKeys(node, "flowType", "nodes", "handle", "name") {
		matches = append(matches, resourceTypeFlow)
	}

	if hasAllKeys(node, "type", "properties", "name") {
		matches = append(matches, resourceTypeIdentityProvider)
	}

	if hasAnyKey(node, "owner") ||
		(hasAnyKey(node, "type") &&
			hasAnyKey(node, "authFlowId", "registrationFlowId", "inboundAuthConfig", "allowedUserTypes") &&
			!hasAnyKey(node, "properties")) {
		matches = append(matches, resourceTypeAgent)
	}

	if !hasAnyKey(node, "owner") &&
		!(hasAnyKey(node, "type") &&
			hasAnyKey(node, "authFlowId", "registrationFlowId", "inboundAuthConfig", "allowedUserTypes") &&
			!hasAnyKey(node, "properties")) &&
		hasAnyKey(node, "authFlowId", "registrationFlowId", "inboundAuthConfig", "allowedUserTypes") {
		matches = append(matches, resourceTypeApplication)
	}

	if hasAllKeys(node, "name") && hasAnyKey(node, "ouId", "ouHandle") &&
		!hasAnyKey(node, "handle", "permissions", "identifier", "type", "flowType",
			"displayName", "properties", "schema") {
		matches = append(matches, resourceTypeGroup)
	}

	if hasAllKeys(node, "handle", "name") &&
		!hasAnyKey(node, "flowType", "nodes", "authFlowId", "registrationFlowId", "inboundAuthConfig") {
		matches = append(matches, resourceTypeOrganizationUnit)
	}

	if len(matches) != 1 {
		return resourceTypeUnknown
	}

	return matches[0]
}

func hasAllKeys(node *yaml.Node, keys ...string) bool {
	for _, key := range keys {
		if !hasAnyKey(node, key) {
			return false
		}
	}
	return true
}

func hasAnyKey(node *yaml.Node, keys ...string) bool {
	if node == nil || node.Kind != yaml.MappingNode {
		return false
	}

	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i]
		for _, candidate := range keys {
			if key.Value == candidate {
				return true
			}
		}
	}

	return false
}
