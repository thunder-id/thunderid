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

// everyDeleter satisfies every optional deletion interface, and records which one was called. It
// stands in for each domain service in turn so that one test covers the whole resolution table:
// what matters per resource type is that the right service is asked, and a fake that can answer for
// all of them isolates that from what any single service does.
type everyDeleter struct {
	// The adapter interfaces are embedded rather than implemented: each field on an import service
	// is typed as its full adapter, and only the optional delete method is ever called here. An
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
	// category records the user type category a deletion resolved to, which is the one branch that
	// derives a value rather than passing the id straight through.
	category entitytype.TypeCategory
}

func (e *everyDeleter) record(name, id string) *tidcommon.ServiceError {
	e.called, e.id = name, id
	return nil
}

func (e *everyDeleter) DeleteApplication(_ context.Context, id string) *tidcommon.ServiceError {
	return e.record(resourceTypeApplication, id)
}
func (e *everyDeleter) DeleteIdentityProvider(_ context.Context, id string) *tidcommon.ServiceError {
	return e.record("idp", id)
}
func (e *everyDeleter) DeleteSender(_ context.Context, id string) *tidcommon.ServiceError {
	return e.record("sender", id)
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
// can walk the whole resolution switch.
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

// A resource type with no delete operation reports that plainly rather than silently doing nothing,
// so a configuration that drops one is not quietly left in place.
func TestResolveDeleterRefusesTypesThatCannotBeRemoved(t *testing.T) {
	svc := &importService{}

	for _, resourceType := range []string{
		resourceTypeTranslation,
		resourceTypeServerConfig,
		resourceTypeAgentType,
		"nonsense",
	} {
		deleter, svcErr := svc.resolveDeleter(ResourceDeletion{ResourceType: resourceType, ID: "x"})

		require.NotNil(t, svcErr, "%s should report that it cannot be deleted", resourceType)
		assert.Nil(t, deleter)
		assert.Equal(t, ErrorDeleteNotSupported.Code, svcErr.Code)
	}
}

// An adapter that cannot delete is refused for the same reason, even though the resource type
// itself is deletable: what matters is whether the configured service can carry it out.
func TestResolveDeleterRefusesAnAdapterThatCannotDelete(t *testing.T) {
	svc := &importService{}

	for _, resourceType := range []string{
		resourceTypeApplication, resourceTypeFlow, resourceTypeRole, resourceTypeGroup,
		resourceTypeTheme, resourceTypeLayout, resourceTypeAgent, resourceTypeOrganizationUnit,
		resourceTypeResourceServer, resourceTypeEntityType, resourceTypeConnection,
		resourceTypePresentationDefinition, resourceTypeCredentialConfiguration,
	} {
		_, svcErr := svc.resolveDeleter(ResourceDeletion{ResourceType: resourceType, ID: "x"})

		require.NotNil(t, svcErr, "%s with no adapter should be refused", resourceType)
		assert.Equal(t, ErrorDeleteNotSupported.Code, svcErr.Code)
	}
}

// Each resource type resolves to its own service. A table rather than a test apiece, because the
// thing worth pinning is that the mapping has no crossed wires.
func TestResolveDeleterAsksTheRightService(t *testing.T) {
	for _, tc := range []struct{ resourceType, expect string }{
		{resourceTypeApplication, resourceTypeApplication},
		{resourceTypeFlow, resourceTypeFlow},
		{resourceTypeOrganizationUnit, resourceTypeOrganizationUnit},
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
			deleter, svcErr := serviceWithDeleters(fake).resolveDeleter(
				ResourceDeletion{ResourceType: tc.resourceType, ID: "the-id"})

			require.Nil(t, svcErr)
			require.NotNil(t, deleter)
			require.Nil(t, deleter(context.Background()))
			assert.Equal(t, tc.expect, fake.called, "the wrong service was asked to delete")
			assert.Equal(t, "the-id", fake.id)
		})
	}
}

// A user type deletion carries a category, and an omitted one means the user category rather than
// an empty one the domain would reject.
func TestResolveDeleterDefaultsTheUserTypeCategory(t *testing.T) {
	fake := &everyDeleter{}
	deleter, svcErr := serviceWithDeleters(fake).resolveDeleter(
		ResourceDeletion{ResourceType: resourceTypeEntityType, ID: "ut-1"})

	require.Nil(t, svcErr)
	require.Nil(t, deleter(context.Background()))
	assert.Equal(t, entitytype.TypeCategoryUser, fake.category)
}

// An unrecognized category is refused rather than passed down to fail somewhere less legible.
func TestResolveDeleterRefusesAnInvalidUserTypeCategory(t *testing.T) {
	deleter, svcErr := serviceWithDeleters(&everyDeleter{}).resolveDeleter(
		ResourceDeletion{ResourceType: resourceTypeEntityType, ID: "ut-1", Category: "nonsense"})

	require.NotNil(t, svcErr)
	assert.Nil(t, deleter)
	assert.Equal(t, ErrorDeleteNotSupported.Code, svcErr.Code)
}
