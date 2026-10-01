// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"context"
	"fmt"
	"sort"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
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
			if removed {
				deleted++
			}
			continue
		}
		failed++
		s.logDeletionFailure(ctx, deletion, outcome)
		if !options.IsContinueOnErrorEnabled() {
			break
		}
	}
	return outcomes, deleted, failed
}

// logDeletionFailure records why a deletion did not happen.
//
// The reason already reaches the caller on the outcome, which is what a client reads. This is for
// the operator of the deployment, who has the logs and not the response: an import that reports two
// failures out of forty is otherwise something they can see happened but not diagnose.
func (s *importService) logDeletionFailure(
	ctx context.Context, deletion ResourceDeletion, outcome ImportItemOutcome) {
	fields := []log.Field{
		log.String("resourceType", deletion.ResourceType),
		log.String("resourceId", deletion.ID),
		log.String("code", outcome.Code),
	}
	if outcome.Message != "" {
		fields = append(fields, log.String("error", outcome.Message))
	}
	log.GetLogger().Error(ctx, "Import failed to delete a resource", fields...)
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
// success: a dry run and an already-absent resource both succeed without removing anything. That is
// what the caller counts, so the count is of deletions that completed rather than of resources
// verified gone, which most services cannot tell it.
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

	deleteFn := s.deleterFor(deletion.ResourceType)
	if deleteFn == nil {
		svcErr := deleteUnsupportedError(deletion.ResourceType)
		outcome.Status = statusFailed
		outcome.Code = svcErr.Code
		outcome.Message = svcErr.ErrorDescription.DefaultValue
		return outcome, false
	}

	if dryRun {
		outcome.Status = statusSuccess
		return outcome, false
	}

	if svcErr := deleteFn(ctx, deletion.ID); svcErr != nil {
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

// deleteFunc removes one resource by id.
type deleteFunc func(ctx context.Context, id string) *tidcommon.ServiceError

// deleterFor returns the delete an import uses for a resource type, or nil when this deployment
// cannot remove that type: translations and server configuration have no delete, and an adapter the
// deployment was not configured with cannot be asked. It mirrors importDocument, which dispatches the
// same resource types to the same adapters for create and update.
func (s *importService) deleterFor(resourceType string) deleteFunc {
	switch resourceType {
	case resourceTypeApplication:
		if s.applicationService != nil {
			return s.applicationService.DeleteApplication
		}
	case resourceTypeConnection:
		if s.hasConnectionAdapter() {
			return s.deleteConnection
		}
	case resourceTypeFlow:
		if s.flowService != nil {
			return s.flowService.DeleteFlow
		}
	case resourceTypeOrganizationUnit:
		if s.ouService != nil {
			return s.ouService.DeleteOrganizationUnit
		}
	case resourceTypeEntityType, resourceTypeAgentType:
		if s.entityTypeService != nil {
			return s.entityTypeDeleter(resourceType)
		}
	case resourceTypeRole:
		if s.roleService != nil {
			return s.roleService.DeleteRole
		}
	case resourceTypeGroup:
		if s.groupService != nil {
			return s.groupService.DeleteGroup
		}
	case resourceTypeResourceServer:
		if s.resourceService != nil {
			return s.resourceService.DeleteResourceServer
		}
	case resourceTypeTheme:
		if s.themeService != nil {
			return s.themeService.DeleteTheme
		}
	case resourceTypeLayout:
		if s.layoutService != nil {
			return s.layoutService.DeleteLayout
		}
	case resourceTypeUser:
		if s.userService != nil {
			return s.userService.DeleteUser
		}
	case resourceTypeAgent:
		if s.agentService != nil {
			return s.agentService.DeleteAgent
		}
	case resourceTypePresentationDefinition:
		if s.presentationDefinitionService != nil {
			return s.presentationDefinitionService.DeletePresentationDefinition
		}
	case resourceTypeCredentialConfiguration:
		if s.credentialConfigurationService != nil {
			return s.credentialConfigurationService.DeleteCredentialConfiguration
		}
	}
	return nil
}

// entityTypeDeleter deletes an entity type within the category its resource type names, which is how
// user_type and agent_type, imported as separate resource types, are removed as separate ones too.
func (s *importService) entityTypeDeleter(resourceType string) deleteFunc {
	category := entitytype.TypeCategoryUser
	if resourceType == resourceTypeAgentType {
		category = entitytype.TypeCategoryAgent
	}
	return func(ctx context.Context, id string) *tidcommon.ServiceError {
		return s.entityTypeService.DeleteEntityType(ctx, category, id)
	}
}

// hasConnectionAdapter reports whether any of the services a connection can be held by is configured.
func (s *importService) hasConnectionAdapter() bool {
	return s.idpService != nil || s.senderService != nil || s.authZENPDPService != nil
}

// deleteConnection removes a connection from whichever service holds it, as importConnection writes
// one to whichever service its document resolves to.
//
// The owner is found with a read before the delete, because the identity provider delete succeeds
// for an id it does not hold, so deleting blindly would report a removal that never happened. Only a
// not-found answer moves on to the next service: any other failure is the lookup itself failing, and
// carrying on past it would report the connection as already gone. When no service holds the id, the
// last not-found is returned, which deleteRuntimeResource reports as already absent.
func (s *importService) deleteConnection(ctx context.Context, id string) *tidcommon.ServiceError {
	absent := &idp.ErrorIDPNotFound

	if s.idpService != nil {
		_, svcErr := s.idpService.GetIdentityProvider(ctx, id)
		if svcErr == nil {
			return s.idpService.DeleteIdentityProvider(ctx, id)
		}
		if !isNotFoundServiceError(svcErr) {
			return svcErr
		}
	}

	if s.senderService != nil {
		_, svcErr := s.senderService.GetSender(ctx, id)
		if svcErr == nil {
			return s.senderService.DeleteSender(ctx, id)
		}
		if !isNotFoundServiceError(svcErr) {
			return svcErr
		}
		absent = svcErr
	}

	if s.authZENPDPService != nil {
		pdpConnection, svcErr := s.authZENPDPService.GetAuthZENPDP(ctx, id)
		if svcErr != nil {
			return svcErr
		}
		if pdpConnection != nil {
			return s.authZENPDPService.DeleteAuthZENPDPConnection(ctx, id)
		}
	}

	return absent
}

// deleteUnsupportedError builds the client error reported for a deletion that cannot be performed.
func deleteUnsupportedError(resourceType string) *tidcommon.ServiceError {
	return &tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: ErrorDeleteNotSupported.Code,
		Error: tidcommon.I18nMessage{
			Key:          "error.import.deleteNotSupported",
			DefaultValue: fmt.Sprintf("Deletion is not supported for %q", resourceType),
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.import.deleteNotSupported.description",
			DefaultValue: fmt.Sprintf("resource type %q cannot be deleted", resourceType),
		},
	}
}
