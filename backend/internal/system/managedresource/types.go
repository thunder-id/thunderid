// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package managedresource

// The resource types a resource is marked under: the types of the documents the import takes, which
// is what the managed import marks each resource it writes with.
const (
	TypeOrganizationUnit        = "organization_unit"
	TypeProject                 = "project"
	TypeUserType                = "user_type"
	TypeAgentType               = "agent_type"
	TypeOUType                  = "ou_type"
	TypeResourceServer          = "resource_server"
	TypeAuthorizationDetailType = "authorization_detail_type"
	TypeRole                    = "role"
	TypeGroup                   = "group"
	TypeConnection              = "connection"
	TypeFlow                    = "flow"
	TypeTheme                   = "theme"
	TypeLayout                  = "layout"
	TypeApplication             = "application"
	TypeUser                    = "user"
	TypeTranslation             = "translation"
	TypeAgent                   = "agent"
	TypePresentationDefinition  = "presentation_definition"
	TypeCredentialConfiguration = "credential_configuration" //nolint:gosec // G101: a type name, not a credential
	TypeServerConfig            = "server_config"
	TypeGateway                 = "gateway"
	TypeNotificationTemplate    = "notification_template"
)
