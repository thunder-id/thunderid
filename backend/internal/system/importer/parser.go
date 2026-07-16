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

		resourceType := resourceTypeFromField(root)
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

func resourceTypeFromField(node *yaml.Node) string {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == "resource_type" {
			value := node.Content[i+1].Value
			if isKnownResourceType(value) {
				return value
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
