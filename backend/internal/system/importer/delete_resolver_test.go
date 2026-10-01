// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/entitytype"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// everyDeleter stands in for every adapter's delete, and records which one was called. It
// stands in for each domain service in turn so that one test covers the whole resolution table:
// what matters per resource type is that the right service is asked, and a fake that can answer for
// all of them isolates that from what any single service does.
type everyDeleter struct {
	// The adapter interfaces are embedded rather than implemented: each field on an import service
	// is typed as its full adapter, and only the delete method is ever called here. An
	// embedded nil interface satisfies the type and panics loudly if anything else is reached for,
	// which is the right outcome for a method this test has no business calling.
	applicationAdapter
	flowAdapter
	ouAdapter
	entityTypeAdapter
	roleAdapter
	groupAdapter
	resourceServerAdapter
	themeAdapter
	layoutAdapter
	userAdapter
	agentAdapter
	presentationDefinitionAdapter
	credentialConfigurationAdapter

	called string
	id     string
	// category records the entity type category a deletion resolved to, which is the one branch
	// that derives a value rather than passing the id straight through.
	category entitytype.TypeCategory
}

func (e *everyDeleter) record(name, id string) *tidcommon.ServiceError {
	e.called, e.id = name, id
	return nil
}

func (e *everyDeleter) DeleteApplication(_ context.Context, id string) *tidcommon.ServiceError {
	return e.record(resourceTypeApplication, id)
}
func (e *everyDeleter) DeleteFlow(_ context.Context, id string) *tidcommon.ServiceError {
	return e.record(resourceTypeFlow, id)
}
func (e *everyDeleter) DeleteOrganizationUnit(_ context.Context, id string) *tidcommon.ServiceError {
	return e.record(resourceTypeOrganizationUnit, id)
}
func (e *everyDeleter) DeleteEntityType(_ context.Context, category entitytype.TypeCategory,
	id string) *tidcommon.ServiceError {
	e.category = category
	return e.record(resourceTypeEntityType, id)
}
func (e *everyDeleter) DeleteRole(_ context.Context, id string) *tidcommon.ServiceError {
	return e.record(resourceTypeRole, id)
}
func (e *everyDeleter) DeleteGroup(_ context.Context, id string) *tidcommon.ServiceError {
	return e.record(resourceTypeGroup, id)
}
func (e *everyDeleter) DeleteResourceServer(_ context.Context, id string) *tidcommon.ServiceError {
	return e.record(resourceTypeResourceServer, id)
}
func (e *everyDeleter) DeleteTheme(_ context.Context, id string) *tidcommon.ServiceError {
	return e.record(resourceTypeTheme, id)
}
func (e *everyDeleter) DeleteLayout(_ context.Context, id string) *tidcommon.ServiceError {
	return e.record(resourceTypeLayout, id)
}
func (e *everyDeleter) DeleteUser(_ context.Context, id string) *tidcommon.ServiceError {
	return e.record(resourceTypeUser, id)
}
func (e *everyDeleter) DeleteAgent(_ context.Context, id string) *tidcommon.ServiceError {
	return e.record(resourceTypeAgent, id)
}
func (e *everyDeleter) DeletePresentationDefinition(_ context.Context, id string) *tidcommon.ServiceError {
	return e.record(resourceTypePresentationDefinition, id)
}
func (e *everyDeleter) DeleteCredentialConfiguration(_ context.Context, id string) *tidcommon.ServiceError {
	return e.record(resourceTypeCredentialConfiguration, id)
}

// serviceWithDeleters builds an import service whose every adapter is the same fake, so one table
// can walk the whole deleterFor switch.
func serviceWithDeleters(fake *everyDeleter) *importService {
	return &importService{
		applicationService:             fake,
		flowService:                    fake,
		ouService:                      fake,
		entityTypeService:              fake,
		roleService:                    fake,
		groupService:                   fake,
		resourceService:                fake,
		themeService:                   fake,
		layoutService:                  fake,
		userService:                    fake,
		agentService:                   fake,
		presentationDefinitionService:  fake,
		credentialConfigurationService: fake,
	}
}

// A resource type with no delete operation resolves to none, so it is reported rather than
// silently doing nothing and a configuration that drops one is not quietly left in place.
func TestDeleterForRefusesTypesThatCannotBeRemoved(t *testing.T) {
	svc := serviceWithDeleters(&everyDeleter{})

	for _, resourceType := range []string{resourceTypeTranslation, resourceTypeServerConfig, "nonsense"} {
		assert.Nil(t, svc.deleterFor(resourceType), "%s should have no delete", resourceType)
	}
}

// A deployment not configured with an adapter cannot be asked to delete through it, even though the
// resource type itself is deletable.
func TestDeleterForRefusesAnAdapterThatIsNotConfigured(t *testing.T) {
	svc := &importService{}

	for _, resourceType := range []string{
		resourceTypeApplication, resourceTypeConnection, resourceTypeFlow, resourceTypeOrganizationUnit,
		resourceTypeEntityType, resourceTypeAgentType, resourceTypeRole, resourceTypeGroup,
		resourceTypeResourceServer, resourceTypeTheme, resourceTypeLayout, resourceTypeUser, resourceTypeAgent,
		resourceTypePresentationDefinition, resourceTypeCredentialConfiguration,
	} {
		assert.Nil(t, svc.deleterFor(resourceType), "%s with no adapter should have no delete", resourceType)
	}
}

// Each resource type resolves to its own service. A table rather than a test apiece, because the
// thing worth pinning is that the mapping has no crossed wires.
func TestDeleterForAsksTheRightService(t *testing.T) {
	for _, tc := range []struct{ resourceType, expect string }{
		{resourceTypeApplication, resourceTypeApplication},
		{resourceTypeFlow, resourceTypeFlow},
		{resourceTypeOrganizationUnit, resourceTypeOrganizationUnit},
		{resourceTypeEntityType, resourceTypeEntityType},
		{resourceTypeAgentType, resourceTypeEntityType},
		{resourceTypeRole, resourceTypeRole},
		{resourceTypeGroup, resourceTypeGroup},
		{resourceTypeResourceServer, resourceTypeResourceServer},
		{resourceTypeTheme, resourceTypeTheme},
		{resourceTypeLayout, resourceTypeLayout},
		{resourceTypeUser, resourceTypeUser},
		{resourceTypeAgent, resourceTypeAgent},
		{resourceTypePresentationDefinition, resourceTypePresentationDefinition},
		{resourceTypeCredentialConfiguration, resourceTypeCredentialConfiguration},
	} {
		t.Run(tc.resourceType, func(t *testing.T) {
			fake := &everyDeleter{}
			deleteFn := serviceWithDeleters(fake).deleterFor(tc.resourceType)

			require.NotNil(t, deleteFn)
			require.Nil(t, deleteFn(context.Background(), "the-id"))
			assert.Equal(t, tc.expect, fake.called, "the wrong service was asked to delete")
			assert.Equal(t, "the-id", fake.id)
		})
	}
}

// user_type and agent_type are separate resource types, and each deletes within its own category.
func TestDeleterForScopesEntityTypesByCategory(t *testing.T) {
	for resourceType, category := range map[string]entitytype.TypeCategory{
		resourceTypeEntityType: entitytype.TypeCategoryUser,
		resourceTypeAgentType:  entitytype.TypeCategoryAgent,
	} {
		fake := &everyDeleter{}
		require.Nil(t, serviceWithDeleters(fake).deleterFor(resourceType)(context.Background(), "the-id"))
		assert.Equal(t, category, fake.category, resourceType)
	}
}
