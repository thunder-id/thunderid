// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/user"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// categoryTraits is what the executors need to know about an entity category.
type categoryTraits struct {
	// groupMemberType represents an entity of the category in a group.
	groupMemberType group.MemberType
	// roleAssigneeType represents an entity of the category in a role assignment.
	roleAssigneeType role.AssigneeType
	// attributeConflictCode is what the category's management service raises when a unique
	// attribute already belongs to another entity.
	attributeConflictCode string
	// autoProvisionedAtAuthentication marks a category that federated authentication may create
	// on the fly, gated on the eligibility flag that authentication writes.
	autoProvisionedAtAuthentication bool
	// applicationAllowedTypes returns the entity types the application admits for the category.
	applicationAllowedTypes func(ctx *providers.NodeContext) []string
	// recordFields are carried on the entity's own record rather than in its schema.
	recordFields []string
	// missingRecordFields reports which record fields the flow still has to collect.
	missingRecordFields func(ctx *providers.NodeContext) []providers.Input
}

// categoryTraitsByCategory holds the traits of every supported category. Supporting a new one
// means adding an entry.
var categoryTraitsByCategory = map[entitytype.TypeCategory]categoryTraits{
	entitytype.TypeCategoryUser: {
		groupMemberType:                 group.MemberTypeUser,
		roleAssigneeType:                role.AssigneeTypeUser,
		attributeConflictCode:           user.ErrorAttributeConflict.Code,
		autoProvisionedAtAuthentication: true,
		applicationAllowedTypes: func(ctx *providers.NodeContext) []string {
			return ctx.Application.AllowedUserTypes
		},
	},
	entitytype.TypeCategoryAgent: {
		groupMemberType:       group.MemberTypeAgent,
		roleAssigneeType:      role.AssigneeTypeAgent,
		attributeConflictCode: agentAttributeConflictCode,
		applicationAllowedTypes: func(ctx *providers.NodeContext) []string {
			return ctx.Application.AllowedAgentTypes
		},
		recordFields:        []string{nameKey, ownerIDKey, delegatedKey, redirectURIsKey},
		missingRecordFields: missingAgentRecordFields,
	},
}

// traitsFor returns the traits of the category, falling back to the user category so an
// unrecognized one behaves as it did before any other was supported.
func traitsFor(category entitytype.TypeCategory) categoryTraits {
	if traits, ok := categoryTraitsByCategory[category]; ok {
		return traits
	}

	return categoryTraitsByCategory[entitytype.TypeCategoryUser]
}

// missingAgentRecordFields returns the record fields an agent still has to collect.
func missingAgentRecordFields(ctx *providers.NodeContext) []providers.Input {
	missing := make([]providers.Input, 0, 2)

	if collectedValue(ctx, nameKey) == "" {
		missing = append(missing, requiredTextInput(nameKey))
	}
	// Delegation is what makes a redirect URI necessary. A value that cannot be read is reported
	// when the entity is created, so it is treated as absent here.
	delegated, _ := collectedFlag(ctx, delegatedKey)
	if delegated && collectedValue(ctx, redirectURIsKey) == "" {
		missing = append(missing, requiredTextInput(redirectURIsKey))
	}

	return missing
}

// requiredTextInput describes a text field the flow must collect.
func requiredTextInput(identifier string) providers.Input {
	return providers.Input{
		Identifier: identifier,
		Type:       providers.InputTypeText,
		Required:   true,
	}
}

// hasRecordValues reports whether the flow collected any of the category's record fields.
func hasRecordValues(ctx *providers.NodeContext, category entitytype.TypeCategory) bool {
	for _, key := range traitsFor(category).recordFields {
		if collectedValue(ctx, key) != "" {
			return true
		}
	}

	return false
}

// missingNonSchemaAttributes returns the record fields the category still has to collect.
func missingNonSchemaAttributes(ctx *providers.NodeContext,
	category entitytype.TypeCategory) []providers.Input {
	missing := traitsFor(category).missingRecordFields
	if missing == nil {
		return nil
	}

	return missing(ctx)
}
