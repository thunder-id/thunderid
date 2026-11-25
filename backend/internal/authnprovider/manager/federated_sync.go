// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package manager

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
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// settleFederatedSync syncs the federated sign-in the provider just verified to the one account holding its
// link, unless the caller defers it, or runs the deferred sync of the identity pending on any provider in
// authUser when an authentication by entity ID signs in the account holding its link. A synced identity is
// cleared from the provider state that held it. It returns what the sign-in keeps to the session, or nil
// when nothing was synced.
func (m *authnProviderManager) settleFederatedSync(ctx context.Context, credentials map[string]interface{},
	metadata *providers.AuthnMetadata, authResult *providers.AuthnResult, authUser *providers.AuthUser,
	providerName string) (*authnprovidercm.SessionValues, *tidcommon.ServiceError) {
	owner, identity := providerName, authResult.FederatedIdentity
	created := false
	if identity != nil {
		if metadata != nil && metadata.DeferSync {
			return nil, nil
		}
	} else if _, provisioned := credentials[authnprovidercm.CredentialTypeProvisionedEntityID]; provisioned {
		owner, identity = findPendingIdentity(*authUser)
		created = metadata != nil && metadata.Created
	}
	if identity == nil || authResult.EntityReference == nil || m.sync.EntityProvider == nil {
		return nil, nil
	}

	values, synced, svcErr := m.syncAccount(ctx, *authResult.EntityReference, identity, created)
	if svcErr != nil || !synced {
		return nil, svcErr
	}
	state, _ := authUser.StateFor(owner)
	state.PendingIdentity = nil
	authUser.SetStateFor(owner, state)
	return values, nil
}

// syncAccount maps the federated sign-in with the connection's mappings and applies its sync modes to the
// account ref names. It returns false when that account doesn't hold the sign-in's link, and writes nothing
// then. created marks the account the flow just created: its attributes stay as Provisioning stored them,
// and its groups and roles are only added to. An Override all mapping that could not be fully resolved
// fails before anything is read or written. Without an entity store, nothing is read or written, and the
// session values are still returned.
func (m *authnProviderManager) syncAccount(ctx context.Context, ref providers.EntityReference,
	identity *providers.FederatedIdentity, created bool) (*authnprovidercm.SessionValues, bool,
	*tidcommon.ServiceError) {
	idpDTO, svcErr := m.sync.IDPProvider.GetIdentityProvider(ctx, identity.IdpID)
	if svcErr != nil || idpDTO == nil {
		m.logger.Error(ctx, "Failed to load the connection of a federated sign-in",
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

	resolved, svcErr := idp.ResolveAuthorizationMapping(ctx, idpDTO, received,
		m.sync.IDPProvider.ResolveDirectAuthorizationTargets)
	groupsAndRolesMode := modes.GroupsAndRolesMode()
	if svcErr != nil {
		if groupsAndRolesMode == providers.SyncModeOverrideAll && (resolved.MapsGroups || resolved.MapsRoles) {
			m.logger.Debug(ctx, "Override all mapping could not be fully resolved, failing the sync",
				log.String(log.LoggerKeyIDPID, idpDTO.ID))
			return nil, false, &ErrorFederatedSyncFailed
		}
		m.logger.Warn(ctx, "Failed to resolve direct authorization targets, continuing with rule-based targets only",
			log.String(log.LoggerKeyIDPID, idpDTO.ID), log.String("error", svcErr.Error.DefaultValue))
	}
	roleIDs, groupIDs, permissions := idp.SplitAuthorizationTargets(resolved.Targets)
	ambiguousRoleIDs, ambiguousGroupIDs, _ := idp.SplitAuthorizationTargets(resolved.Ambiguous)

	account, held, svcErr := m.readLinkedAccount(ctx, ref.EntityID, identity)
	if svcErr != nil || !held {
		return nil, false, svcErr
	}
	if account != nil {
		ref = providers.EntityReference{EntityID: account.ID, EntityCategory: string(account.Category),
			EntityType: account.Type, OUID: account.OUID}
	}

	values := &authnprovidercm.SessionValues{Permissions: permissions}
	sessionAttributes, svcErr := m.syncAttributes(ctx, ref, account, modes.AttributesMode(),
		resolvedMappedAttributes(identity.Claims, typeMappings), received, created)
	if svcErr != nil {
		return nil, false, svcErr
	}
	values.Attributes = sessionAttributes

	groupsToSession := groupsAndRolesMode == providers.SyncModeSessionOnly || group.IsDeclarativeOnly()
	rolesToSession := groupsAndRolesMode == providers.SyncModeSessionOnly || role.IsDeclarativeOnly()
	replace := !created && groupsAndRolesMode == providers.SyncModeOverrideAll
	addOnly := created && groupsAndRolesMode != providers.SyncModeSessionOnly

	if groupsToSession {
		values.GroupIDs = groupIDs
		values.GroupIDsWithAncestors = slices.Clone(groupIDs)
		if account != nil {
			values.GroupIDsWithAncestors = m.withAncestorGroups(ctx, groupIDs)
		}
	} else if account != nil && (addOnly || replace && resolved.MapsGroups) {
		if svcErr := m.syncGroups(ctx, ref, groupIDs, ambiguousGroupIDs, replace); svcErr != nil {
			return nil, false, svcErr
		}
	}
	if rolesToSession {
		values.RoleIDs = roleIDs
	} else if account != nil && (addOnly || replace && resolved.MapsRoles) {
		if svcErr := m.syncRoles(ctx, ref, roleIDs, ambiguousRoleIDs, replace); svcErr != nil {
			return nil, false, svcErr
		}
	}

	if m.logger.IsDebugEnabled() {
		m.logger.Debug(ctx, "Synced federated sign-in to the account holding its link",
			log.MaskedString(log.LoggerKeyEntityID, ref.EntityID), log.Bool("created", created),
			log.Bool("stored", account != nil),
			log.String("attributesMode", string(modes.AttributesMode())),
			log.String("groupsAndRolesMode", string(groupsAndRolesMode)))
	}
	return values, true, nil
}

// readLinkedAccount returns the account entityID names when it holds the sign-in's link, and false when it
// doesn't. With the entity provider disabled it returns no account and true, so the sync reads and writes
// nothing. Any other entity provider failure is a server error.
func (m *authnProviderManager) readLinkedAccount(ctx context.Context, entityID string,
	identity *providers.FederatedIdentity) (*providers.Entity, bool, *tidcommon.ServiceError) {
	linked, epErr := m.sync.EntityProvider.GetLinkedEntities(ctx, identity.IdpID, identity.Sub)
	if epErr != nil {
		if epErr.Code == entityprovider.ErrorCodeNotImplemented {
			m.logger.Debug(ctx, "Entity provider is disabled, syncing the federated sign-in to the session only")
			return nil, true, nil
		}
		m.logger.Error(ctx, "Failed to read the accounts holding the federated link",
			log.String("errorCode", string(epErr.Code)))
		return nil, false, &tidcommon.InternalServerError
	}
	for _, account := range linked {
		if account != nil && account.ID == entityID {
			return account, true, nil
		}
	}
	m.logger.Debug(ctx, "Entity does not hold the federated link, skipping sync",
		log.MaskedString(log.LoggerKeyEntityID, entityID))
	return nil, false, nil
}

// syncAttributes applies the attributes sync mode. Session only returns the received values of the
// account's attributes and stores nothing. Under Override received a returning account takes the mapped
// values that differ. A created account is never written, since Provisioning stored the received values
// with it, and neither is an account without a store. mapped names the attributes a connection mapping
// filled from the claims. A value the account can't take is skipped and logged.
func (m *authnProviderManager) syncAttributes(ctx context.Context, ref providers.EntityReference,
	account *providers.Entity, mode providers.SyncMode, mapped map[string]bool, received map[string]interface{},
	created bool) (map[string]interface{}, *tidcommon.ServiceError) {
	if providers.EntityCategory(ref.EntityCategory) != providers.EntityCategoryUser {
		return nil, nil
	}
	if mode != providers.SyncModeSessionOnly &&
		(created || mode != providers.SyncModeOverrideReceived || account == nil || account.IsReadOnly) {
		return nil, nil
	}
	schemaAttrs, svcErr := m.sync.EntityTypeService.GetAttributes(ctx, entitytype.TypeCategoryUser, ref.EntityType,
		entitytype.AttributeFilter{AllowNonCredential: true})
	if svcErr != nil {
		m.logger.Error(ctx, "Failed to get the user type's attributes", log.String("errorCode", svcErr.Code))
		return nil, &tidcommon.InternalServerError
	}

	// The connection's attributes for this account: the mapped claims, kept to the account type's
	// non-credential attributes, without the subject and the claims that describe the token. Override
	// received narrows them to the attributes the connection maps explicitly.
	candidates := make(map[string]interface{})
	for _, a := range schemaAttrs {
		if a.Attribute == authnprovidercm.UserAttributeSub || slices.Contains(tokenMetadataClaims, a.Attribute) {
			continue
		}
		if mode == providers.SyncModeOverrideReceived && !mapped[a.Attribute] {
			continue
		}
		raw, ok := received[a.Attribute]
		if !ok {
			continue
		}
		value, ok := entitytypemodel.ConvertReceivedValue(raw, a.Type)
		if !ok {
			m.logSkippedSyncAttribute(ctx, ref.EntityID, a.Attribute,
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
			m.logger.Error(ctx, "Failed to read the account's attributes", log.Error(err))
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
	return nil, m.writeAttributes(ctx, account.ID, current, changes)
}

// writeAttributes stores changes on top of current. When the account refuses the whole set, each change
// is tried on its own, so only the values the account can't take are left out.
func (m *authnProviderManager) writeAttributes(ctx context.Context, entityID string,
	current, changes map[string]interface{}) *tidcommon.ServiceError {
	if len(changes) == 0 {
		return nil
	}
	merged := maps.Clone(current)
	maps.Copy(merged, changes)
	if taken, svcErr := m.updateAttributes(ctx, entityID, merged); taken || svcErr != nil {
		return svcErr
	}

	merged = current
	for _, name := range slices.Sorted(maps.Keys(changes)) {
		candidate := maps.Clone(merged)
		candidate[name] = changes[name]
		taken, svcErr := m.updateAttributes(ctx, entityID, candidate)
		if svcErr != nil {
			return svcErr
		}
		if !taken {
			m.logSkippedSyncAttribute(ctx, entityID, name, "received value fails validation")
			continue
		}
		merged = candidate
	}
	return nil
}

// updateAttributes stores attributes as the account's profile, and returns false when the account can't
// take them: a value fails the schema or is held by another user. A sign-in has no caller allowed to write
// the account, so it writes with the runtime's authority.
func (m *authnProviderManager) updateAttributes(ctx context.Context, entityID string,
	attributes map[string]interface{}) (bool, *tidcommon.ServiceError) {
	encoded, err := json.Marshal(attributes)
	if err != nil {
		m.logger.Error(ctx, "Failed to encode the account's attributes", log.Error(err))
		return false, &tidcommon.InternalServerError
	}
	epErr := m.sync.EntityProvider.UpdateAttributes(security.WithRuntimeContext(ctx), entityID, encoded)
	switch {
	case epErr == nil:
		return true, nil
	case epErr.Code == entityprovider.ErrorCodeSchemaValidationFailed,
		epErr.Code == entityprovider.ErrorCodeAttributeConflict:
		return false, nil
	default:
		m.logger.Error(ctx, "Failed to update the account's attributes", log.String("errorCode", string(epErr.Code)))
		return false, &tidcommon.InternalServerError
	}
}

// syncGroups adds the account to the mapped groups that exist in its organization units. With replace it
// also removes the direct memberships the mapping no longer lists, keeping the groups a mapping name
// matched ambiguously and the memberships the sync can't remove: a control plane's, or a declarative
// group's. Writes use the runtime's authority, since a sign-in has no caller allowed to make them.
func (m *authnProviderManager) syncGroups(ctx context.Context, ref providers.EntityReference,
	groupIDs, ambiguousIDs []string, replace bool) *tidcommon.ServiceError {
	member := group.Member{ID: ref.EntityID, Type: groupMemberType(providers.EntityCategory(ref.EntityCategory))}
	var current []string
	if replace {
		var svcErr *tidcommon.ServiceError
		if current, svcErr = m.sync.GroupService.GetMemberGroupIDs(ctx, member); svcErr != nil {
			return m.syncFailure(ctx, "Failed to read the account's groups", svcErr)
		}
	}
	writeCtx := security.WithRuntimeContext(ctx)
	toAdd := slices.DeleteFunc(systemutils.UniqueStrings(groupIDs),
		func(id string) bool { return slices.Contains(current, id) })
	if len(toAdd) > 0 {
		if svcErr := m.sync.GroupService.AddMemberToExistingGroups(writeCtx, member, toAdd); svcErr != nil {
			return m.syncFailure(ctx, "Failed to add the account to its mapped groups", svcErr)
		}
	}
	for _, id := range current {
		if slices.Contains(groupIDs, id) || slices.Contains(ambiguousIDs, id) {
			continue
		}
		if _, svcErr := m.sync.GroupService.RemoveGroupMembers(writeCtx, id, []group.Member{member}); svcErr != nil {
			if svcErr.Code != group.ErrorGroupNotFound.Code && svcErr.Code != group.ErrorManagedGroupMember.Code &&
				svcErr.Code != group.ErrorImmutableGroup.Code {
				return m.syncFailure(ctx, "Failed to remove the account from an unmapped group", svcErr)
			}
			m.logger.Debug(ctx, "Keeping a group membership the sync can't remove",
				log.String("groupID", id), log.String("errorCode", svcErr.Code))
		}
	}
	return nil
}

// syncRoles assigns the account the mapped roles that can be assigned in its organization unit. With
// replace it also removes the direct assignments in that unit the mapping no longer lists, keeping the
// roles a mapping name matched ambiguously and the assignments the sync can't remove: a mandated one, a
// control plane's, or one of a role no longer shared with the unit. Writes use the runtime's authority.
func (m *authnProviderManager) syncRoles(ctx context.Context, ref providers.EntityReference,
	roleIDs, ambiguousIDs []string, replace bool) *tidcommon.ServiceError {
	assignee := role.RoleAssignment{ID: ref.EntityID,
		Type: roleAssigneeType(providers.EntityCategory(ref.EntityCategory))}
	var current []string
	if replace {
		var svcErr *tidcommon.ServiceError
		if current, svcErr = m.sync.RoleAssignmentService.GetDirectRoleIDs(ctx, ref.EntityID, ref.OUID); svcErr != nil {
			return m.syncFailure(ctx, "Failed to read the account's roles", svcErr)
		}
	}
	writeCtx := security.WithRuntimeContext(ctx)
	toAdd := slices.DeleteFunc(systemutils.UniqueStrings(roleIDs),
		func(id string) bool { return slices.Contains(current, id) })
	if len(toAdd) > 0 {
		if svcErr := m.sync.RoleAssignmentService.AddAssigneeToExistingRoles(writeCtx, assignee,
			toAdd); svcErr != nil {
			return m.syncFailure(ctx, "Failed to assign the account its mapped roles", svcErr)
		}
	}
	for _, id := range current {
		if slices.Contains(roleIDs, id) || slices.Contains(ambiguousIDs, id) {
			continue
		}
		if svcErr := m.sync.RoleAssignmentService.RemoveAssignments(writeCtx, id, ref.OUID,
			[]role.RoleAssignment{assignee}); svcErr != nil {
			if svcErr.Code != role.ErrorRoleNotFound.Code && svcErr.Code != role.ErrorRoleNotSharedToOU.Code &&
				svcErr.Code != role.ErrorAssignmentMandated.Code &&
				svcErr.Code != role.ErrorManagedRoleAssignment.Code {
				return m.syncFailure(ctx, "Failed to remove an unmapped role from the account", svcErr)
			}
			m.logger.Debug(ctx, "Keeping a role assignment the sync can't remove",
				log.String("roleID", id), log.String("errorCode", svcErr.Code))
		}
	}
	return nil
}

// syncFailure maps a rejected group or role write to the sync failure, and any other failure to a
// server error.
func (m *authnProviderManager) syncFailure(ctx context.Context, msg string,
	svcErr *tidcommon.ServiceError) *tidcommon.ServiceError {
	if svcErr.Type == tidcommon.ServerErrorType {
		m.logger.Error(ctx, msg, log.String("errorCode", svcErr.Code))
		return &tidcommon.InternalServerError
	}
	m.logger.Debug(ctx, msg, log.String("errorCode", svcErr.Code))
	return &ErrorFederatedSyncFailed
}

// withAncestorGroups returns groupIDs with each group's ancestors. A group whose ancestors can't be read
// is kept without them, like the rest of authorization mapping at sign-in.
func (m *authnProviderManager) withAncestorGroups(ctx context.Context, groupIDs []string) []string {
	expanded := slices.Clone(groupIDs)
	for _, groupID := range groupIDs {
		ancestors, epErr := m.sync.EntityProvider.GetTransitiveGroupAncestors(ctx, groupID)
		if epErr != nil {
			m.logger.Warn(ctx, "Failed to resolve ancestors of a mapped group, continuing without them",
				log.String("groupID", groupID), log.String("errorCode", string(epErr.Code)))
			continue
		}
		expanded = systemutils.MergeUniqueStrings(expanded, ancestors)
	}
	return expanded
}

// logSkippedSyncAttribute logs a received attribute a sync leaves out. The value isn't logged, since it
// may hold personal data.
func (m *authnProviderManager) logSkippedSyncAttribute(ctx context.Context, entityID, attribute, reason string) {
	m.logger.Debug(ctx, "Skipping a federated attribute the account can't take",
		log.MaskedString(log.LoggerKeyEntityID, entityID), log.String("attribute", attribute),
		log.String("reason", reason))
}
