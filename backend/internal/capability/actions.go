// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package capability

import "github.com/thunder-id/thunderid/internal/system/security"

// viewAction is the catalog action every other action of a resource builds on.
const viewAction = "view"

// The catalog resources that no system action covers, answered from the API routes in requiredRoutes.
const (
	resourceImport          security.ResourceType = "import"
	resourceExport          security.ResourceType = "export"
	resourceVariable        security.ResourceType = "variable"
	resourceSecret          security.ResourceType = "secret"
	resourceGatewayVariable security.ResourceType = "gatewayvariable"
	resourceGatewaySecret   security.ResourceType = "gatewaysecret"
)

// requiredActions maps each catalog resource and action to the system actions a caller needs, all of
// them, to do it in the Console. The first is the action itself, where a catalog "view" is the
// resource's read. The rest are what its Console page can't work without: a create wizard that places
// the record in an organization unit lists them, and the application wizard lists flows and
// connections to offer sign-in options and creates a sign-in flow. Every action other than view also
// needs what the resource's view needs, since the Console reaches it from the resource's pages.
var requiredActions = map[security.ResourceType]map[string][]security.Action{
	security.ResourceTypeOU: {
		"view":   {security.ActionReadOU},
		"create": {security.ActionCreateOU},
		"update": {security.ActionUpdateOU},
		"delete": {security.ActionDeleteOU},
	},
	security.ResourceTypeUser: {
		"view":   {security.ActionReadUser, security.ActionReadUserType},
		"create": {security.ActionCreateUser, security.ActionReadOU},
		"update": {security.ActionUpdateUser},
		"delete": {security.ActionDeleteUser},
	},
	security.ResourceTypeGroup: {
		"view":   {security.ActionReadGroup},
		"create": {security.ActionCreateGroup, security.ActionReadOU},
		"update": {security.ActionUpdateGroup},
		"delete": {security.ActionDeleteGroup},
	},
	security.ResourceTypeAgent: {
		"view":   {security.ActionReadAgent},
		"create": {security.ActionCreateAgent, security.ActionReadServerConfig, security.ActionReadFlow},
		"update": {security.ActionUpdateAgent},
		"delete": {security.ActionDeleteAgent},
	},
	security.ResourceTypeUserType: {
		"view":   {security.ActionReadUserType},
		"create": {security.ActionCreateUserType, security.ActionReadOU},
		"update": {security.ActionUpdateUserType},
		"delete": {security.ActionDeleteUserType},
	},
	security.ResourceTypeAgentType: {
		"view":   {security.ActionReadAgentType},
		"create": {security.ActionCreateAgentType},
		"update": {security.ActionUpdateAgentType},
		"delete": {security.ActionDeleteAgentType},
	},
	security.ResourceTypeNotificationTemplate: {
		"view":   {security.ActionReadNotificationTemplate},
		"create": {security.ActionCreateNotificationTemplate, security.ActionReadOU},
		"update": {security.ActionUpdateNotificationTemplate},
		"delete": {security.ActionDeleteNotificationTemplate},
	},
	security.ResourceTypeApplication: {
		"view": {security.ActionReadApplication},
		"create": {
			security.ActionCreateApplication, security.ActionReadOU, security.ActionReadFlow,
			security.ActionCreateFlow, security.ActionReadConnection,
		},
		"update": {security.ActionUpdateApplication},
		"delete": {security.ActionDeleteApplication},
	},
	security.ResourceTypeRole: {
		"view":   {security.ActionReadRole},
		"create": {security.ActionCreateRole, security.ActionReadOU},
		"update": {security.ActionUpdateRole},
		"delete": {security.ActionDeleteRole},
	},
	security.ResourceTypeResourceServer: {
		"view":   {security.ActionReadResourceServer},
		"create": {security.ActionCreateResourceServer, security.ActionReadOU},
		"update": {security.ActionUpdateResourceServer},
		"delete": {security.ActionDeleteResourceServer},
	},
	security.ResourceTypeConnection: {
		"view":   {security.ActionReadConnection},
		"create": {security.ActionCreateConnection, security.ActionReadOU},
		"update": {security.ActionUpdateConnection},
		"delete": {security.ActionDeleteConnection},
	},
	security.ResourceTypeFlow: {
		"view":   {security.ActionReadFlow},
		"create": {security.ActionCreateFlow, security.ActionReadOU},
		"update": {security.ActionUpdateFlow},
		"delete": {security.ActionDeleteFlow},
	},
	security.ResourceTypeVerifiableCredential: {
		"view":   {security.ActionReadVC},
		"create": {security.ActionCreateVC, security.ActionReadOU},
		"update": {security.ActionUpdateVC},
		"delete": {security.ActionDeleteVC},
	},
	security.ResourceTypeProject: {
		"view":   {security.ActionReadProject},
		"create": {security.ActionCreateProject},
		"update": {security.ActionUpdateProject},
		"delete": {security.ActionDeleteProject},
	},
	security.ResourceTypeDesign: {
		"view":   {security.ActionReadDesign},
		"create": {security.ActionCreateDesign},
		"update": {security.ActionUpdateDesign},
		"delete": {security.ActionDeleteDesign},
	},
	security.ResourceTypeI18n: {
		"view": {security.ActionReadI18n},
		// The Console adds a language through POST /i18n/languages/{lang}/translations, which needs update.
		"create": {security.ActionUpdateI18n},
		"update": {security.ActionUpdateI18n},
		"delete": {security.ActionDeleteI18n},
	},
	security.ResourceTypeGateway: {
		"view":   {security.ActionReadGateway},
		"create": {security.ActionCreateGateway},
		"update": {security.ActionUpdateGateway},
		"delete": {security.ActionDeleteGateway},
	},
	security.ResourceTypeAuthorizationDetailType: {
		"view":   {security.ActionReadAuthzDetail},
		"create": {security.ActionCreateAuthzDetail},
		"update": {security.ActionUpdateAuthzDetail},
		"delete": {security.ActionDeleteAuthzDetail},
	},
	security.ResourceTypeServerConfig: {
		"view":   {security.ActionReadServerConfig},
		"update": {security.ActionUpdateServerConfig},
	},
	security.ResourceTypeConfigurationVersion: {
		"view":   {security.ActionReadConfigVersion},
		"create": {security.ActionCreateConfigVersion},
	},
}

// requiredRoutes maps each catalog resource and action that no system action covers to the API routes a
// caller needs, all of them, to do it in the Console. Each route is a "METHOD /path" pattern as the API
// permission map lists it, so the answer follows whatever permission that map requires for it. As with
// requiredActions, every action other than view also needs what the resource's view needs.
var requiredRoutes = map[security.ResourceType]map[string][]string{
	resourceImport: {
		"create": {"POST /import"},
		"delete": {"POST /import/delete"},
	},
	resourceExport: {
		"create": {"POST /export"},
	},
	resourceVariable: {
		"view":   {"GET /variables", "GET /variables/**"},
		"create": {"POST /variables"},
		"update": {"PUT /variables/**"},
		"delete": {"DELETE /variables/**"},
	},
	resourceSecret: {
		"view":   {"GET /secrets", "GET /secrets/**"},
		"create": {"POST /secrets"},
		"update": {"PUT /secrets/**"},
		"delete": {"DELETE /secrets/**"},
	},
	resourceGatewayVariable: {
		"view":   {"GET /gateways/*/variables/**"},
		"create": {"POST /gateways/*/variables/**"},
		"update": {"PUT /gateways/*/variables/**"},
		"delete": {"DELETE /gateways/*/variables/**"},
	},
	resourceGatewaySecret: {
		"view":   {"GET /gateways/*/secrets/**"},
		"create": {"POST /gateways/*/secrets/**"},
		"update": {"PUT /gateways/*/secrets/**"},
		"delete": {"DELETE /gateways/*/secrets/**"},
	},
}
