// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"context"
	"fmt"
	"sort"

	"github.com/thunder-id/thunderid/internal/idp"

	"github.com/thunder-id/thunderid/internal/entitytype"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Runtime deletion is an optional capability of an import adapter. The create/update adapter
// contracts intentionally stay unchanged: the importer type-asserts these narrow interfaces at
// deletion time and reports an "unsupported" outcome for resource types whose service cannot delete
// (translations and server configuration have no delete operation). deleters_assert_test.go pins each
// interface against the concrete domain service so a signature drift is a compile error rather than a
// silent fallback to unsupported.
type (
	applicationDeleter interface {
		DeleteApplication(ctx context.Context, appID string) *tidcommon.ServiceError
	}
	idpDeleter interface {
		DeleteIdentityProvider(ctx context.Context, idpID string) *tidcommon.ServiceError
	}
	senderDeleter interface {
		DeleteSender(ctx context.Context, id string) *tidcommon.ServiceError
	}
	flowDeleter interface {
		DeleteFlow(ctx context.Context, flowID string) *tidcommon.ServiceError
	}
	ouDeleter interface {
		DeleteOrganizationUnit(ctx context.Context, id string) *tidcommon.ServiceError
	}
	entityTypeDeleter interface {
		DeleteEntityType(ctx context.Context, category entitytype.TypeCategory,
			schemaID string) *tidcommon.ServiceError
	}
	roleDeleter interface {
		DeleteRole(ctx context.Context, id string) *tidcommon.ServiceError
	}
	groupDeleter interface {
		DeleteGroup(ctx context.Context, groupID string) *tidcommon.ServiceError
	}
	resourceServerDeleter interface {
		DeleteResourceServer(ctx context.Context, id string) *tidcommon.ServiceError
	}
	themeDeleter interface {
		DeleteTheme(ctx context.Context, id string) *tidcommon.ServiceError
	}
	layoutDeleter interface {
		DeleteLayout(ctx context.Context, id string) *tidcommon.ServiceError
	}
	agentDeleter interface {
		DeleteAgent(ctx context.Context, agentID string) *tidcommon.ServiceError
	}
	presentationDefinitionDeleter interface {
		DeletePresentationDefinition(ctx context.Context, id string) *tidcommon.ServiceError
	}
	credentialConfigurationDeleter interface {
		DeleteCredentialConfiguration(ctx context.Context, id string) *tidcommon.ServiceError
	}
)

// deleteResources removes the requested resources and returns one outcome per deletion. Deletions run
// in reverse dependency order (dependents before their dependencies) so that referential guards, such
// as a flow still referencing a connection, do not reject an otherwise valid prune.
func (s *importService) deleteResources(
	ctx context.Context, deletions []ResourceDeletion, options *ImportOptions, dryRun bool,
) ([]ImportItemOutcome, int, int) {
	ordered := orderDeletionsByDependencies(deletions)

	outcomes := make([]ImportItemOutcome, 0, len(ordered))
	deleted := 0
	failed := 0
	for _, deletion := range ordered {
		outcome, removed := s.deleteRuntimeResource(ctx, deletion, dryRun)
		outcomes = append(outcomes, outcome)
		if outcome.Status == statusSuccess {
			// Counted only when the deletion completed against the store. A dry run calls no
			// service, and a service that answers not-found had nothing to remove, so neither is
			// counted even though both are a success. Most services return success for an id that
			// was never there, so this is what completed rather than what was verified removed.
			if removed {
				deleted++
			}
			continue
		}
		failed++
		if !options.IsContinueOnErrorEnabled() {
			break
		}
	}
	return outcomes, deleted, failed
}

// orderDeletionsByDependencies sorts deletions into the reverse of the import dependency order.
func orderDeletionsByDependencies(deletions []ResourceDeletion) []ResourceDeletion {
	priority := make(map[string]int, len(resourceDependencyOrder))
	for i, resourceType := range resourceDependencyOrder {
		priority[resourceType] = i
	}

	ordered := make([]ResourceDeletion, len(deletions))
	copy(ordered, deletions)

	sort.SliceStable(ordered, func(i, j int) bool {
		pi, ok := priority[ordered[i].ResourceType]
		if !ok {
			pi = -1
		}
		pj, ok := priority[ordered[j].ResourceType]
		if !ok {
			pj = -1
		}
		return pi > pj
	})

	return ordered
}

// deleteRuntimeResource deletes a single resource. Deleting an absent resource is reported as success
// so that re-applying the same configuration is idempotent.
//
// It reports whether the resource was actually removed, which is not the same as the outcome being a
// success: a dry run and an already-absent resource both succeed without removing anything.
func (s *importService) deleteRuntimeResource(
	ctx context.Context, deletion ResourceDeletion, dryRun bool,
) (ImportItemOutcome, bool) {
	outcome := ImportItemOutcome{
		ResourceType: deletion.ResourceType,
		ResourceID:   deletion.ID,
		Operation:    operationDelete,
	}

	if deletion.ResourceType == "" || deletion.ID == "" {
		outcome.Status = statusFailed
		outcome.Code = ErrorInvalidImportRequest.Code
		outcome.Message = "resourceType and id are required to delete a resource"
		return outcome, false
	}

	deleteFn, svcErr := s.resolveDeleter(deletion)
	if svcErr != nil {
		outcome.Status = statusFailed
		outcome.Code = svcErr.Code
		outcome.Message = svcErr.ErrorDescription.DefaultValue
		return outcome, false
	}

	if dryRun {
		outcome.Status = statusSuccess
		return outcome, false
	}

	if svcErr := deleteFn(ctx); svcErr != nil {
		if isNotFoundServiceError(svcErr) {
			outcome.Status = statusSuccess
			outcome.Message = "resource already absent"
			return outcome, false
		}
		outcome.Status = statusFailed
		outcome.Code = svcErr.Code
		outcome.Message = svcErr.Error.DefaultValue
		return outcome, false
	}

	outcome.Status = statusSuccess
	return outcome, true
}

// resolveDeleter returns the delete operation for a resource type, or a service error describing why
// the resource type cannot be deleted at runtime.
func (s *importService) resolveDeleter(
	deletion ResourceDeletion,
) (func(context.Context) *tidcommon.ServiceError, *tidcommon.ServiceError) {
	switch deletion.ResourceType {
	case resourceTypeApplication:
		if d, ok := s.applicationService.(applicationDeleter); ok {
			return func(ctx context.Context) *tidcommon.ServiceError {
				return d.DeleteApplication(ctx, deletion.ID)
			}, nil
		}
	case resourceTypeConnection:
		return s.resolveConnectionDeleter(deletion)
	case resourceTypeFlow:
		if d, ok := s.flowService.(flowDeleter); ok {
			return func(ctx context.Context) *tidcommon.ServiceError {
				return d.DeleteFlow(ctx, deletion.ID)
			}, nil
		}
	case resourceTypeOrganizationUnit:
		if d, ok := s.ouService.(ouDeleter); ok {
			return func(ctx context.Context) *tidcommon.ServiceError {
				return d.DeleteOrganizationUnit(ctx, deletion.ID)
			}, nil
		}
	case resourceTypeEntityType:
		return s.resolveEntityTypeDeleter(deletion)
	case resourceTypeRole:
		if d, ok := s.roleService.(roleDeleter); ok {
			return func(ctx context.Context) *tidcommon.ServiceError {
				return d.DeleteRole(ctx, deletion.ID)
			}, nil
		}
	case resourceTypeGroup:
		if d, ok := s.groupService.(groupDeleter); ok {
			return func(ctx context.Context) *tidcommon.ServiceError {
				return d.DeleteGroup(ctx, deletion.ID)
			}, nil
		}
	case resourceTypeResourceServer:
		if d, ok := s.resourceService.(resourceServerDeleter); ok {
			return func(ctx context.Context) *tidcommon.ServiceError {
				return d.DeleteResourceServer(ctx, deletion.ID)
			}, nil
		}
	case resourceTypeTheme:
		if d, ok := s.themeService.(themeDeleter); ok {
			return func(ctx context.Context) *tidcommon.ServiceError {
				return d.DeleteTheme(ctx, deletion.ID)
			}, nil
		}
	case resourceTypeLayout:
		if d, ok := s.layoutService.(layoutDeleter); ok {
			return func(ctx context.Context) *tidcommon.ServiceError {
				return d.DeleteLayout(ctx, deletion.ID)
			}, nil
		}
	case resourceTypeUser:
		if s.userService != nil {
			return func(ctx context.Context) *tidcommon.ServiceError {
				return s.userService.DeleteUser(ctx, deletion.ID)
			}, nil
		}
	case resourceTypeAgent:
		if d, ok := s.agentService.(agentDeleter); ok {
			return func(ctx context.Context) *tidcommon.ServiceError {
				return d.DeleteAgent(ctx, deletion.ID)
			}, nil
		}
	case resourceTypePresentationDefinition:
		if d, ok := s.presentationDefinitionService.(presentationDefinitionDeleter); ok {
			return func(ctx context.Context) *tidcommon.ServiceError {
				return d.DeletePresentationDefinition(ctx, deletion.ID)
			}, nil
		}
	case resourceTypeCredentialConfiguration:
		if d, ok := s.credentialConfigurationService.(credentialConfigurationDeleter); ok {
			return func(ctx context.Context) *tidcommon.ServiceError {
				return d.DeleteCredentialConfiguration(ctx, deletion.ID)
			}, nil
		}
	default:
		return nil, deleteUnsupportedError(deletion.ResourceType,
			fmt.Sprintf("resource type %q cannot be deleted", deletion.ResourceType))
	}

	return nil, deleteUnsupportedError(deletion.ResourceType,
		fmt.Sprintf("the configured %q adapter does not support deletion", deletion.ResourceType))
}

// resolveConnectionDeleter picks the service that owns a connection id. A connection is stored as
// either an identity provider or a notification sender, and the identity provider delete succeeds
// even when the id is absent, so ownership is probed with a read before choosing the delete.
func (s *importService) resolveConnectionDeleter(
	deletion ResourceDeletion,
) (func(context.Context) *tidcommon.ServiceError, *tidcommon.ServiceError) {
	idpDelete, hasIDP := s.idpService.(idpDeleter)
	senderDelete, hasSender := s.senderService.(senderDeleter)
	if !hasIDP && !hasSender {
		return nil, deleteUnsupportedError(resourceTypeConnection,
			"the configured connection adapters do not support deletion")
	}

	return func(ctx context.Context) *tidcommon.ServiceError {
		// Only a not-found answer means the next service should be asked. Any other lookup failure is
		// the probe itself failing, and carrying on past it would report a deletion that never
		// happened: the id would fall through both services and come back as already gone.
		//
		// The last not-found is kept rather than discarded. Neither service owning the id is the
		// absent case, and returning it lets the one place that already knows what to do with an
		// absent resource do it: report the outcome as a success that removed nothing. Returning nil
		// here would say a connection had been removed when the probes had just confirmed there was
		// none, and it would be counted as one.
		var absent *tidcommon.ServiceError
		if hasIDP && s.idpService != nil {
			switch _, svcErr := s.idpService.GetIdentityProvider(ctx, deletion.ID); {
			case svcErr == nil:
				return idpDelete.DeleteIdentityProvider(ctx, deletion.ID)
			case !isNotFoundServiceError(svcErr):
				return svcErr
			default:
				absent = svcErr
			}
		}
		if hasSender && s.senderService != nil {
			switch _, svcErr := s.senderService.GetSender(ctx, deletion.ID); {
			case svcErr == nil:
				return senderDelete.DeleteSender(ctx, deletion.ID)
			case !isNotFoundServiceError(svcErr):
				return svcErr
			default:
				absent = svcErr
			}
		}
		if absent != nil {
			return absent
		}
		// Neither service is configured to be asked at all, so there is nothing here to remove.
		return &idp.ErrorIDPNotFound
	}, nil
}

// resolveEntityTypeDeleter builds the entity type delete, which is scoped by category. The category
// defaults to user, matching the import path's default for a document that omits it.
func (s *importService) resolveEntityTypeDeleter(
	deletion ResourceDeletion,
) (func(context.Context) *tidcommon.ServiceError, *tidcommon.ServiceError) {
	d, ok := s.entityTypeService.(entityTypeDeleter)
	if !ok {
		return nil, deleteUnsupportedError(resourceTypeEntityType,
			"the configured user type adapter does not support deletion")
	}

	category := entitytype.TypeCategory(deletion.Category)
	if deletion.Category == "" {
		category = entitytype.TypeCategoryUser
	}
	if !category.IsValid() {
		return nil, deleteUnsupportedError(resourceTypeEntityType,
			fmt.Sprintf("invalid user type category %q", deletion.Category))
	}

	return func(ctx context.Context) *tidcommon.ServiceError {
		return d.DeleteEntityType(ctx, category, deletion.ID)
	}, nil
}

// deleteUnsupportedError builds the client error reported for a deletion that cannot be performed.
func deleteUnsupportedError(resourceType, description string) *tidcommon.ServiceError {
	return &tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: ErrorDeleteNotSupported.Code,
		Error: tidcommon.I18nMessage{
			Key:          "error.import.deleteNotSupported",
			DefaultValue: fmt.Sprintf("Deletion is not supported for %q", resourceType),
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.import.deleteNotSupported.description",
			DefaultValue: description,
		},
	}
}
