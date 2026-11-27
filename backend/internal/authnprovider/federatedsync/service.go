// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package federatedsync syncs a federated sign-in to the account holding its link.
package federatedsync

import (
	"bytes"
	"context"
	"encoding/json"
	"maps"
	"slices"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	entitytypemodel "github.com/thunder-id/thunderid/internal/entitytype/model"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/idp"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// FederatedSyncServiceInterface syncs a federated sign-in to the account holding its link.
type FederatedSyncServiceInterface interface {
	SyncAccount(ctx context.Context, ref providers.EntityReference, identity *providers.FederatedIdentity,
		created bool) (*authnprovidercm.SessionValues, bool, *tidcommon.ServiceError)
}

// federatedSyncService syncs a federated sign-in to the account holding its link.
type federatedSyncService struct {
	idpProvider           providers.IDPProvider
	entityProvider        entityprovider.EntityProviderInterface
	entityTypeService     entitytype.EntityTypeServiceInterface
	groupService          group.GroupServiceInterface
	roleAssignmentService role.RoleAssignmentServiceInterface
	logger                *log.Logger
}

// newFederatedSyncService creates a federatedSyncService.
func newFederatedSyncService(idpProvider providers.IDPProvider, entityProvider entityprovider.EntityProviderInterface,
	entityTypeService entitytype.EntityTypeServiceInterface, groupService group.GroupServiceInterface,
	roleAssignmentService role.RoleAssignmentServiceInterface) *federatedSyncService {
	return &federatedSyncService{
		idpProvider:           idpProvider,
		entityProvider:        entityProvider,
		entityTypeService:     entityTypeService,
		groupService:          groupService,
		roleAssignmentService: roleAssignmentService,
		logger:                log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FederatedSyncService")),
	}
}

// SyncAccount maps the federated sign-in with the connection's mappings and applies its sync modes to the
// account ref names. It returns what the sign-in keeps to the session, and false when that account doesn't
// hold the sign-in's link, in which case it writes nothing. created marks the account the flow just
// created: its attributes stay as Provisioning stored them, and its groups and roles are only added to. A
// sync the account's groups, roles or mapping refuse fails with ErrorFederatedSyncFailed. An Override all
// mapping that could not be fully resolved fails before anything is written, when the sync would replace
// the stored groups or roles with it.
// Without a stored account, nothing is read or written, and the mapped groups and roles go to the session.
func (s *federatedSyncService) SyncAccount(ctx context.Context, ref providers.EntityReference,
	identity *providers.FederatedIdentity, created bool) (*authnprovidercm.SessionValues, bool,
	*tidcommon.ServiceError) {
	idpDTO, svcErr := s.idpProvider.GetIdentityProvider(ctx, identity.IdpID)
	if svcErr != nil || idpDTO == nil {
		s.logger.Error(ctx, "Failed to load the connection of a federated sign-in",
			log.String(log.LoggerKeyIDPID, identity.IdpID))
		return nil, false, &tidcommon.InternalServerError
	}
	var modes *providers.SyncConfiguration
	if idpDTO.AttributeConfiguration != nil {
		modes = idpDTO.AttributeConfiguration.Sync
	}
	// The connection's user type resolution picks the mapping profile from the claims, as it does for
	// the sign-in, whatever type the account has.
	typeMappings := idp.GetAttributeMappings(idpDTO, identity.Claims)
	received := idp.ApplyAttributeMappings(identity.Claims, typeMappings)

	resolved, resolveErr := idp.ResolveAuthorizationMapping(ctx, idpDTO, received,
		s.idpProvider.ResolveDirectAuthorizationTargets)
	groupsAndRolesMode := modes.GroupsAndRolesMode()
	roleIDs, groupIDs, permissions := idp.SplitAuthorizationTargets(resolved.Targets)
	ambiguousRoleIDs, ambiguousGroupIDs, _ := idp.SplitAuthorizationTargets(resolved.Ambiguous)

	account, held, svcErr := s.readLinkedAccount(ctx, ref.EntityID, identity)
	if svcErr != nil || !held {
		return nil, false, svcErr
	}
	if account != nil {
		ref = providers.EntityReference{EntityID: account.ID, EntityCategory: string(account.Category),
			EntityType: account.Type, OUID: account.OUID}
	}

	groupsToSession, rolesToSession := s.toSession(account, groupsAndRolesMode)
	replace := !created && groupsAndRolesMode == providers.SyncModeOverrideAll
	addOnly := created && groupsAndRolesMode != providers.SyncModeSessionOnly

	if resolveErr != nil {
		replaces := replace && (resolved.MapsGroups && !groupsToSession || resolved.MapsRoles && !rolesToSession)
		if svcErr := s.partialMappingFailure(ctx, idpDTO.ID, resolveErr, replaces); svcErr != nil {
			return nil, false, svcErr
		}
	}

	values := &authnprovidercm.SessionValues{Permissions: permissions}
	sessionAttributes, svcErr := s.syncAttributes(ctx, ref, account, modes.AttributesMode(),
		resolvedMappedAttributes(identity.Claims, typeMappings), received, created)
	if svcErr != nil {
		return nil, false, svcErr
	}
	values.Attributes = sessionAttributes

	if groupsToSession {
		values.GroupIDs = groupIDs
		values.GroupIDsWithAncestors = slices.Clone(groupIDs)
		if account != nil {
			values.GroupIDsWithAncestors = s.withAncestorGroups(ctx, groupIDs)
		}
	} else if addOnly || replace && resolved.MapsGroups {
		if svcErr := s.syncGroups(ctx, ref, groupIDs, ambiguousGroupIDs, replace); svcErr != nil {
			return nil, false, svcErr
		}
	}
	if rolesToSession {
		values.RoleIDs = roleIDs
	} else if addOnly || replace && resolved.MapsRoles {
		if svcErr := s.syncRoles(ctx, ref, roleIDs, ambiguousRoleIDs, replace); svcErr != nil {
			return nil, false, svcErr
		}
	}

	if s.logger.IsDebugEnabled() {
		s.logger.Debug(ctx, "Synced federated sign-in to the account holding its link",
			log.MaskedString(log.LoggerKeyEntityID, ref.EntityID), log.Bool("created", created),
			log.Bool("stored", account != nil),
			log.String("attributesMode", string(modes.AttributesMode())),
			log.String("groupsAndRolesMode", string(groupsAndRolesMode)))
	}
	return values, true, nil
}

// readLinkedAccount returns the account entityID names when it holds the sign-in's link, and false when it
// doesn't. Without an entity provider, or with it disabled, it returns no account and true, so the sync
// reads and writes nothing. Any other entity provider failure is a server error.
func (s *federatedSyncService) readLinkedAccount(ctx context.Context, entityID string,
	identity *providers.FederatedIdentity) (*providers.Entity, bool, *tidcommon.ServiceError) {
	if s.entityProvider == nil {
		return nil, true, nil
	}
	linked, epErr := s.entityProvider.GetLinkedEntities(ctx, identity.IdpID, identity.Sub)
	if epErr != nil {
		if epErr.Code == entityprovider.ErrorCodeNotImplemented {
			s.logger.Debug(ctx, "Entity provider is disabled, syncing the federated sign-in to the session only")
			return nil, true, nil
		}
		s.logger.Error(ctx, "Failed to read the accounts holding the federated link",
			log.String("errorCode", string(epErr.Code)))
		return nil, false, &tidcommon.InternalServerError
	}
	for _, account := range linked {
		if account != nil && account.ID == entityID {
			return account, true, nil
		}
	}
	s.logger.Debug(ctx, "Entity does not hold the federated link, skipping sync",
		log.MaskedString(log.LoggerKeyEntityID, entityID))
	return nil, false, nil
}

// syncAttributes applies the attributes sync mode to the attributes a connection mapping filled from the
// claims, named in mapped. Session only returns their received values and stores nothing. Under Override
// received a returning account takes the values that differ. A created account is never written, since
// Provisioning stored the received values with it, and neither is an account without a store. Without the
// entity type service no attribute can be typed, so none is synced. A value the account can't take is
// skipped and logged.
func (s *federatedSyncService) syncAttributes(ctx context.Context, ref providers.EntityReference,
	account *providers.Entity, mode providers.SyncMode, mapped map[string]bool, received map[string]interface{},
	created bool) (map[string]interface{}, *tidcommon.ServiceError) {
	if providers.EntityCategory(ref.EntityCategory) != providers.EntityCategoryUser || s.entityTypeService == nil {
		return nil, nil
	}
	if mode != providers.SyncModeSessionOnly &&
		(created || mode != providers.SyncModeOverrideReceived || account == nil || account.IsReadOnly) {
		return nil, nil
	}
	schemaAttrs, svcErr := s.entityTypeService.GetAttributes(ctx, entitytype.TypeCategoryUser, ref.EntityType,
		entitytype.AttributeFilter{AllowNonCredential: true})
	if svcErr != nil {
		s.logger.Error(ctx, "Failed to get the user type's attributes", log.String("errorCode", svcErr.Code))
		return nil, &tidcommon.InternalServerError
	}

	// The mapped attributes, kept to the account type's non-credential attributes, without the subject and
	// the claims that describe the token.
	candidates := make(map[string]interface{})
	for _, a := range schemaAttrs {
		if !mapped[a.Attribute] || a.Attribute == authnprovidercm.UserAttributeSub ||
			slices.Contains(oauth2const.TokenMetadataClaims, a.Attribute) {
			continue
		}
		raw, ok := received[a.Attribute]
		if !ok {
			continue
		}
		value, ok := entitytypemodel.ConvertReceivedValue(raw, a.Type)
		if !ok {
			s.logSkippedSyncAttribute(ctx, ref.EntityID, a.Attribute,
				"received value doesn't match the attribute type")
			continue
		}
		if value == nil {
			continue
		}
		candidates[a.Attribute] = value
	}
	if mode == providers.SyncModeSessionOnly {
		return candidates, nil
	}

	current := make(map[string]interface{})
	if len(account.Attributes) > 0 {
		// Numbers stay json.Number, so the stored values the sync doesn't change are written back as they are.
		decoder := json.NewDecoder(bytes.NewReader(account.Attributes))
		decoder.UseNumber()
		if err := decoder.Decode(&current); err != nil {
			s.logger.Error(ctx, "Failed to read the account's attributes", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		if current == nil {
			current = make(map[string]interface{})
		}
	}
	changes := make(map[string]interface{})
	for name, value := range candidates {
		if !jsonEqual(value, current[name]) {
			changes[name] = value
		}
	}
	return nil, s.writeAttributes(ctx, account.ID, current, changes)
}

// writeAttributes stores changes on top of current. When the account refuses the whole set, each change
// is tried on its own, so only the values the account can't take are left out.
func (s *federatedSyncService) writeAttributes(ctx context.Context, entityID string,
	current, changes map[string]interface{}) *tidcommon.ServiceError {
	if len(changes) == 0 {
		return nil
	}
	merged := maps.Clone(current)
	maps.Copy(merged, changes)
	if taken, svcErr := s.updateAttributes(ctx, entityID, merged); taken || svcErr != nil {
		return svcErr
	}

	merged = current
	for _, name := range slices.Sorted(maps.Keys(changes)) {
		candidate := maps.Clone(merged)
		candidate[name] = changes[name]
		taken, svcErr := s.updateAttributes(ctx, entityID, candidate)
		if svcErr != nil {
			return svcErr
		}
		if !taken {
			s.logSkippedSyncAttribute(ctx, entityID, name, "received value fails validation")
			continue
		}
		merged = candidate
	}
	return nil
}

// updateAttributes stores attributes as the account's profile, and returns false when the account can't
// take them: a value fails the schema or is held by another user. A sign-in has no caller allowed to write
// the account, so it writes with the runtime's authority.
func (s *federatedSyncService) updateAttributes(ctx context.Context, entityID string,
	attributes map[string]interface{}) (bool, *tidcommon.ServiceError) {
	encoded, err := json.Marshal(attributes)
	if err != nil {
		s.logger.Error(ctx, "Failed to encode the account's attributes", log.Error(err))
		return false, &tidcommon.InternalServerError
	}
	epErr := s.entityProvider.UpdateAttributes(security.WithRuntimeContext(ctx), entityID, encoded)
	switch {
	case epErr == nil:
		return true, nil
	case epErr.Code == entityprovider.ErrorCodeSchemaValidationFailed,
		epErr.Code == entityprovider.ErrorCodeAttributeConflict:
		return false, nil
	default:
		s.logger.Error(ctx, "Failed to update the account's attributes", log.String("errorCode", string(epErr.Code)))
		return false, &tidcommon.InternalServerError
	}
}

// syncGroups adds the account to the mapped groups that exist in its organization units. With replace it
// also removes the direct memberships the mapping no longer lists, keeping the groups a mapping name
// matched ambiguously and the memberships the sync can't remove: a control plane's, or a declarative
// group's. Writes use the runtime's authority, since a sign-in has no caller allowed to make them.
func (s *federatedSyncService) syncGroups(ctx context.Context, ref providers.EntityReference,
	groupIDs, ambiguousIDs []string, replace bool) *tidcommon.ServiceError {
	member := group.Member{ID: ref.EntityID, Type: groupMemberType(providers.EntityCategory(ref.EntityCategory))}
	var current []string
	if replace {
		var svcErr *tidcommon.ServiceError
		if current, svcErr = s.groupService.GetMemberGroupIDs(ctx, member); svcErr != nil {
			return s.syncFailure(ctx, "Failed to read the account's groups", svcErr)
		}
	}
	writeCtx := security.WithRuntimeContext(ctx)
	toAdd := slices.DeleteFunc(systemutils.UniqueStrings(groupIDs),
		func(id string) bool { return slices.Contains(current, id) })
	if len(toAdd) > 0 {
		if svcErr := s.groupService.AddMemberToExistingGroups(writeCtx, member, toAdd); svcErr != nil {
			return s.syncFailure(ctx, "Failed to add the account to its mapped groups", svcErr)
		}
	}
	for _, id := range current {
		if slices.Contains(groupIDs, id) || slices.Contains(ambiguousIDs, id) {
			continue
		}
		if _, svcErr := s.groupService.RemoveGroupMembers(writeCtx, id, []group.Member{member}); svcErr != nil {
			if svcErr.Code != group.ErrorGroupNotFound.Code && svcErr.Code != group.ErrorManagedGroupMember.Code &&
				svcErr.Code != group.ErrorImmutableGroup.Code {
				return s.syncFailure(ctx, "Failed to remove the account from an unmapped group", svcErr)
			}
			s.logger.Debug(ctx, "Keeping a group membership the sync can't remove",
				log.String("groupID", id), log.String("errorCode", svcErr.Code))
		}
	}
	return nil
}

// syncRoles assigns the account the mapped roles that can be assigned in its organization unit. With
// replace it also removes the direct assignments in that unit the mapping no longer lists, keeping the
// roles a mapping name matched ambiguously and the assignments the sync can't remove: a mandated one, a
// control plane's, or one of a role no longer shared with the unit. Writes use the runtime's authority.
func (s *federatedSyncService) syncRoles(ctx context.Context, ref providers.EntityReference,
	roleIDs, ambiguousIDs []string, replace bool) *tidcommon.ServiceError {
	assignee := role.RoleAssignment{ID: ref.EntityID,
		Type: roleAssigneeType(providers.EntityCategory(ref.EntityCategory)), OUID: ref.OUID}
	var current []string
	if replace {
		var svcErr *tidcommon.ServiceError
		if current, svcErr = s.roleAssignmentService.GetDirectRoleIDs(ctx, ref.EntityID, ref.OUID); svcErr != nil {
			return s.syncFailure(ctx, "Failed to read the account's roles", svcErr)
		}
	}
	writeCtx := security.WithRuntimeContext(ctx)
	toAdd := slices.DeleteFunc(systemutils.UniqueStrings(roleIDs),
		func(id string) bool { return slices.Contains(current, id) })
	if len(toAdd) > 0 {
		if svcErr := s.roleAssignmentService.AddAssigneeToExistingRoles(writeCtx, assignee,
			toAdd); svcErr != nil {
			return s.syncFailure(ctx, "Failed to assign the account its mapped roles", svcErr)
		}
	}
	for _, id := range current {
		if slices.Contains(roleIDs, id) || slices.Contains(ambiguousIDs, id) {
			continue
		}
		if svcErr := s.roleAssignmentService.RemoveAssignments(writeCtx, id, ref.OUID,
			[]role.RoleAssignment{assignee}); svcErr != nil {
			if svcErr.Code != role.ErrorRoleNotFound.Code && svcErr.Code != role.ErrorRoleNotSharedToOU.Code &&
				svcErr.Code != role.ErrorAssignmentMandated.Code &&
				svcErr.Code != role.ErrorManagedRoleAssignment.Code {
				return s.syncFailure(ctx, "Failed to remove an unmapped role from the account", svcErr)
			}
			s.logger.Debug(ctx, "Keeping a role assignment the sync can't remove",
				log.String("roleID", id), log.String("errorCode", svcErr.Code))
		}
	}
	return nil
}

// toSession reports whether the mapped groups, and the mapped roles, go to the session rather than the
// account: under Session only, without a stored account, or with a declarative-only store.
func (s *federatedSyncService) toSession(account *providers.Entity, mode providers.SyncMode) (bool, bool) {
	if account == nil || mode == providers.SyncModeSessionOnly {
		return true, true
	}
	return s.groupService.IsDeclarativeOnly(), s.roleAssignmentService.IsDeclarativeOnly()
}

// partialMappingFailure fails a sync whose mapping couldn't be fully resolved when it replaces the stored
// groups or roles with it, since it would remove assignments the provider still grants. Otherwise the
// partial mapping only grants less, and the sync continues with it.
func (s *federatedSyncService) partialMappingFailure(ctx context.Context, idpID string,
	resolveErr *tidcommon.ServiceError, replaces bool) *tidcommon.ServiceError {
	if replaces {
		return s.syncFailure(ctx, "Failed to fully resolve an Override all mapping", resolveErr)
	}
	s.logger.Warn(ctx, "Failed to resolve direct authorization targets, continuing with rule-based targets only",
		log.String(log.LoggerKeyIDPID, idpID), log.String("error", resolveErr.Error.DefaultValue))
	return nil
}

// syncFailure maps a refused group, role or mapping lookup to the sync failure, and any other failure to
// a server error.
func (s *federatedSyncService) syncFailure(ctx context.Context, msg string,
	svcErr *tidcommon.ServiceError) *tidcommon.ServiceError {
	if svcErr.Type == tidcommon.ServerErrorType {
		s.logger.Error(ctx, msg, log.String("errorCode", svcErr.Code))
		return &tidcommon.InternalServerError
	}
	s.logger.Debug(ctx, msg, log.String("errorCode", svcErr.Code))
	return &ErrorFederatedSyncFailed
}

// withAncestorGroups returns groupIDs with each group's ancestors. A group whose ancestors can't be read
// is kept without them, like the rest of authorization mapping at sign-in.
func (s *federatedSyncService) withAncestorGroups(ctx context.Context, groupIDs []string) []string {
	expanded := slices.Clone(groupIDs)
	for _, groupID := range groupIDs {
		ancestors, epErr := s.entityProvider.GetTransitiveGroupAncestors(ctx, groupID)
		if epErr != nil {
			s.logger.Warn(ctx, "Failed to resolve ancestors of a mapped group, continuing without them",
				log.String("groupID", groupID), log.String("errorCode", string(epErr.Code)))
			continue
		}
		expanded = systemutils.MergeUniqueStrings(expanded, ancestors)
	}
	return expanded
}

// logSkippedSyncAttribute logs a received attribute a sync leaves out. The value isn't logged, since it
// may hold personal data.
func (s *federatedSyncService) logSkippedSyncAttribute(ctx context.Context, entityID, attribute, reason string) {
	s.logger.Debug(ctx, "Skipping a federated attribute the account can't take",
		log.MaskedString(log.LoggerKeyEntityID, entityID), log.String("attribute", attribute),
		log.String("reason", reason))
}
