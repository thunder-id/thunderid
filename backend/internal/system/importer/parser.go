// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/system/managedresource"
)

const (
	resourceTypeOrganizationUnit        = managedresource.TypeOrganizationUnit
	resourceTypeProject                 = managedresource.TypeProject
	resourceTypeEntityType              = managedresource.TypeUserType
	resourceTypeAgentType               = managedresource.TypeAgentType
	resourceTypeResourceServer          = managedresource.TypeResourceServer
	resourceTypeAuthorizationDetailType = managedresource.TypeAuthorizationDetailType
	resourceTypeRole                    = managedresource.TypeRole
	resourceTypeGroup                   = managedresource.TypeGroup
	resourceTypeConnection              = managedresource.TypeConnection
	resourceTypeFlow                    = managedresource.TypeFlow
	resourceTypeTheme                   = managedresource.TypeTheme
	resourceTypeLayout                  = managedresource.TypeLayout
	resourceTypeApplication             = managedresource.TypeApplication
	resourceTypeUser                    = managedresource.TypeUser
	resourceTypeTranslation             = managedresource.TypeTranslation
	resourceTypeAgent                   = managedresource.TypeAgent
	resourceTypePresentationDefinition  = managedresource.TypePresentationDefinition
	resourceTypeCredentialConfiguration = managedresource.TypeCredentialConfiguration
	resourceTypeServerConfig            = managedresource.TypeServerConfig
	resourceTypeGateway                 = managedresource.TypeGateway
	resourceTypeNotificationTemplate    = managedresource.TypeNotificationTemplate
	resourceTypeUnknown                 = "unknown"
)

// Removal types that take assignees off a role or members off a group, leaving the role or group.
const (
	removalTypeRoleAssignment = "role_assignment"
	removalTypeGroupMember    = "group_member"
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
		resourceTypeProject:                 {},
		resourceTypeEntityType:              {},
		resourceTypeAgentType:               {},
		resourceTypeResourceServer:          {},
		resourceTypeAuthorizationDetailType: {},
		resourceTypeRole:                    {},
		resourceTypeConnection:              {},
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
		resourceTypeGateway:                 {},
		resourceTypeNotificationTemplate:    {},
	}

	_, exists := knownTypes[resourceType]
	return exists
}
