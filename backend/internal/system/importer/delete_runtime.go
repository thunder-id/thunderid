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
	"context"
	"fmt"
	"sort"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/managedresource"
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
		outcome := s.deleteRuntimeResource(ctx, deletion, dryRun)
		outcomes = append(outcomes, outcome)
		if outcome.Status == statusSuccess {
			deleted++
			// Drop the ownership record too, so a later resource that reuses this id does not inherit
			// it and become uneditable for no reason.
			if !dryRun {
				if err := managedresource.Default().Unmark(ctx, deletion.ResourceType, deletion.ID); err != nil {
					log.GetLogger().Warn(ctx, "Failed to drop a control plane ownership record",
						log.String("resourceType", deletion.ResourceType),
						log.String("resourceId", deletion.ID), log.Error(err))
				}
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
func (s *importService) deleteRuntimeResource(
	ctx context.Context, deletion ResourceDeletion, dryRun bool,
) ImportItemOutcome {
	outcome := ImportItemOutcome{
		ResourceType: deletion.ResourceType,
		ResourceID:   deletion.ID,
		Operation:    operationDelete,
	}

	if deletion.ResourceType == "" || deletion.ID == "" {
		outcome.Status = statusFailed
		outcome.Code = ErrorInvalidImportRequest.Code
		outcome.Message = "resourceType and id are required to delete a resource"
		return outcome
	}

	deleteFn, svcErr := s.resolveDeleter(deletion)
	if svcErr != nil {
		outcome.Status = statusFailed
		outcome.Code = svcErr.Code
		outcome.Message = svcErr.ErrorDescription.DefaultValue
		return outcome
	}

	if dryRun {
		outcome.Status = statusSuccess
		return outcome
	}

	if svcErr := deleteFn(ctx); svcErr != nil {
		if isNotFoundServiceError(svcErr) {
			outcome.Status = statusSuccess
			outcome.Message = "resource already absent"
			return outcome
		}
		outcome.Status = statusFailed
		outcome.Code = svcErr.Code
		outcome.Message = svcErr.Error.DefaultValue
		return outcome
	}

	outcome.Status = statusSuccess
	return outcome
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
		if hasIDP && s.idpService != nil {
			if _, svcErr := s.idpService.GetIdentityProvider(ctx, deletion.ID); svcErr == nil {
				return idpDelete.DeleteIdentityProvider(ctx, deletion.ID)
			}
		}
		if hasSender && s.senderService != nil {
			if _, svcErr := s.senderService.GetSender(ctx, deletion.ID); svcErr == nil {
				return senderDelete.DeleteSender(ctx, deletion.ID)
			}
		}
		// Neither service owns the id: the connection is already gone.
		return nil
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
