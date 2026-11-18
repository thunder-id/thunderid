// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package defaultprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"reflect"
	"slices"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	entitytypemodel "github.com/thunder-id/thunderid/internal/entitytype/model"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/idp"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/log"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// tokenMetadataClaims describe the token rather than the user, so they never fill an attribute.
var tokenMetadataClaims = []string{
	oauth2const.ClaimAud, oauth2const.ClaimExp, oauth2const.ClaimIat, oauth2const.ClaimNbf,
	oauth2const.ClaimIss, oauth2const.ClaimJTI, oauth2const.ClaimAtHash, oauth2const.ClaimCHash,
	oauth2const.ClaimAzp, oauth2const.ClaimNonce,
}

// pendingIdentity is a federated sign-in this provider still owes a sync, held until an authentication by
// entity ID signs in the account that holds its link.
type pendingIdentity struct {
	IdpID  string                 `json:"idpId"`
	Sub    string                 `json:"sub"`
	Claims map[string]interface{} `json:"claims,omitempty"`
}

// decodePendingIdentity reads a pending identity, as this provider returned it or after a round trip
// through JSON. It returns nil for one that names no federated identity.
func decodePendingIdentity(identity any) *pendingIdentity {
	if identity == nil {
		return nil
	}
	encoded, err := json.Marshal(identity)
	if err != nil {
		return nil
	}
	var signIn pendingIdentity
	if err := json.Unmarshal(encoded, &signIn); err != nil || signIn.IdpID == "" || signIn.Sub == "" {
		return nil
	}
	return &signIn
}

// syncsAtSignIn reports whether a federated sign-in syncs the one account holding its link right away.
// Registration and onboarding leave it to the existing-user rules, so a registration that rejects an
// existing user never syncs one.
func syncsAtSignIn(metadata *providers.AuthnMetadata) bool {
	if metadata == nil {
		return true
	}
	return metadata.FlowType != providers.FlowTypeRegistration &&
		metadata.FlowType != providers.FlowTypeUserOnboarding
}

// syncLinkedAccount syncs the federated sign-in signIn describes to entityID. It returns false when the entity
// does not hold the sign-in's link, and writes nothing then. created marks the account the flow just
// created: its attributes stay as Provisioning stored them, and its groups and roles are only added to.
func (p *defaultAuthnProvider) syncLinkedAccount(ctx context.Context, entityID string, signIn *pendingIdentity,
	created bool) (*authnprovidercm.SessionValues, bool, *tidcommon.ServiceError) {
	// The account's own links decide, so an account another organization unit's sign-in picks still counts.
	account, err := p.entitySvc.GetEntityProfile(ctx, entityID)
	if err != nil {
		return nil, false, p.logAndReturnServerError(ctx, "Failed to read the account to sync",
			log.Error(err))
	}
	if !holdsLink(account, signIn.IdpID, signIn.Sub) {
		p.logger.Debug(ctx, "Entity does not hold the federated link, skipping sync",
			log.MaskedString(log.LoggerKeyEntityID, entityID))
		return nil, false, nil
	}
	values, svcErr := p.syncAccount(ctx, account, signIn, created)
	return values, true, svcErr
}

// holdsLink reports whether the account's linked identities include the subject at the connection.
func holdsLink(account *providers.Entity, idpID, sub string) bool {
	var system map[string]json.RawMessage
	if len(account.SystemAttributes) == 0 || json.Unmarshal(account.SystemAttributes, &system) != nil {
		return false
	}
	var linked map[string]map[string]json.RawMessage
	if json.Unmarshal(system[authnprovidercm.SystemAttrLinkedIDs], &linked) != nil {
		return false
	}
	_, ok := linked[idpID][sub]
	return ok
}

// syncAccount applies the connection's sync modes to account and returns what the sign-in keeps to the
// session. An Override all mapping that could not be fully resolved fails before anything is written.
func (p *defaultAuthnProvider) syncAccount(ctx context.Context, account *providers.Entity, signIn *pendingIdentity,
	created bool) (*authnprovidercm.SessionValues, *tidcommon.ServiceError) {
	idpDTO, svcErr := p.idpService.GetIdentityProvider(ctx, signIn.IdpID)
	if svcErr != nil || idpDTO == nil {
		return nil, p.logAndReturnServerError(ctx, "Failed to load the connection of a federated sign-in",
			log.String(log.LoggerKeyIDPID, signIn.IdpID))
	}
	var modes *providers.SyncConfiguration
	if idpDTO.AttributeConfiguration != nil {
		modes = idpDTO.AttributeConfiguration.Sync
	}
	// The connection's user type resolution picks the mapping profile from the claims, as it does for
	// the sign-in, whatever type the account has.
	typeMappings := idp.GetAttributeMappings(idpDTO, signIn.Claims)
	received := idp.ApplyAttributeMappings(signIn.Claims, typeMappings)

	resolved, svcErr := idp.ResolveAuthorizationMapping(ctx, idpDTO, received,
		p.idpService.ResolveDirectAuthorizationTargets)
	groupsAndRolesMode := modes.GroupsAndRolesMode()
	if svcErr != nil {
		if groupsAndRolesMode == providers.SyncModeOverrideAll && (resolved.MapsGroups || resolved.MapsRoles) {
			p.logger.Debug(ctx, "Override all mapping could not be fully resolved, failing the sync",
				log.String(log.LoggerKeyIDPID, idpDTO.ID))
			return nil, newClientError(authnprovidercm.ErrorCodeFederatedSyncFailed, "Federated sync failed",
				"The connection's authorization mapping could not be fully resolved")
		}
		p.logger.Warn(ctx, "Failed to resolve direct authorization targets, continuing with rule-based targets only",
			log.String(log.LoggerKeyIDPID, idpDTO.ID), log.String("error", svcErr.Error.DefaultValue))
	}
	roleIDs, groupIDs, permissions := idp.SplitAuthorizationTargets(resolved.Targets)
	ambiguousRoleIDs, ambiguousGroupIDs, _ := idp.SplitAuthorizationTargets(resolved.Ambiguous)

	values := &authnprovidercm.SessionValues{Permissions: permissions}
	sessionAttributes, svcErr := p.syncAttributes(ctx, account, modes.AttributesMode(),
		resolvedMappedAttributes(signIn.Claims, typeMappings), received, created)
	if svcErr != nil {
		return nil, svcErr
	}
	values.Attributes = sessionAttributes

	groupsToSession := groupsAndRolesMode == providers.SyncModeSessionOnly || group.IsDeclarativeOnly()
	rolesToSession := groupsAndRolesMode == providers.SyncModeSessionOnly || role.IsDeclarativeOnly()
	replace := !created && groupsAndRolesMode == providers.SyncModeOverrideAll
	addOnly := created && groupsAndRolesMode != providers.SyncModeSessionOnly

	if groupsToSession {
		values.GroupIDs = groupIDs
		values.GroupIDsWithAncestors = p.withAncestorGroups(ctx, groupIDs)
	} else if addOnly || replace && resolved.MapsGroups {
		if svcErr := p.syncGroups(ctx, account, groupIDs, ambiguousGroupIDs, replace); svcErr != nil {
			return nil, svcErr
		}
	}
	if rolesToSession {
		values.RoleIDs = roleIDs
	} else if addOnly || replace && resolved.MapsRoles {
		if svcErr := p.syncRoles(ctx, account, roleIDs, ambiguousRoleIDs, replace); svcErr != nil {
			return nil, svcErr
		}
	}

	if p.logger.IsDebugEnabled() {
		p.logger.Debug(ctx, "Synced federated sign-in to the account holding its link",
			log.MaskedString(log.LoggerKeyEntityID, account.ID), log.Bool("created", created),
			log.String("attributesMode", string(modes.AttributesMode())),
			log.String("groupsAndRolesMode", string(groupsAndRolesMode)))
	}
	return values, nil
}

// syncAttributes applies the attributes sync mode. Session only returns the received values of the
// account's attributes and stores nothing. Under Override received a returning account takes the mapped
// values that differ. A created account is never written, since Provisioning stored the received values
// with it. mapped names the attributes a connection mapping filled from the claims. A value the account
// can't take is skipped and logged.
func (p *defaultAuthnProvider) syncAttributes(ctx context.Context, account *providers.Entity,
	mode providers.SyncMode, mapped map[string]bool, received map[string]interface{},
	created bool) (map[string]interface{}, *tidcommon.ServiceError) {
	if account.Category != providers.EntityCategoryUser {
		return nil, nil
	}
	if mode != providers.SyncModeSessionOnly && (created || mode != providers.SyncModeOverrideReceived) {
		return nil, nil
	}
	schemaAttrs, svcErr := p.entityTypeService.GetAttributes(ctx, entitytype.TypeCategoryUser, account.Type,
		entitytype.AttributeFilter{AllowNonCredential: true})
	if svcErr != nil {
		return nil, p.logAndReturnServerError(ctx, "Failed to get the user type's attributes",
			log.String("errorCode", svcErr.Code))
	}
	current := make(map[string]interface{})
	if len(account.Attributes) > 0 {
		// Numbers stay json.Number, so the stored values the sync doesn't change are written back as they are.
		decoder := json.NewDecoder(bytes.NewReader(account.Attributes))
		decoder.UseNumber()
		if err := decoder.Decode(&current); err != nil {
			return nil, p.logAndReturnServerError(ctx, "Failed to read the account's attributes", log.Error(err))
		}
		if current == nil {
			current = make(map[string]interface{})
		}
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
			logSkippedSyncAttribute(ctx, p.logger, account.ID, a.Attribute,
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
	if account.IsReadOnly {
		return nil, nil
	}

	changes := make(map[string]interface{})
	for name, value := range candidates {
		stored := current[name]
		if jsonEqual(value, stored) {
			continue
		}
		changes[name] = value
	}
	return nil, p.writeAttributes(ctx, account.ID, current, changes)
}

// writeAttributes stores changes on top of current. When the account refuses the whole set, each change
// is tried on its own, so only the values the account can't take are left out.
func (p *defaultAuthnProvider) writeAttributes(ctx context.Context, entityID string,
	current, changes map[string]interface{}) *tidcommon.ServiceError {
	if len(changes) == 0 {
		return nil
	}
	merged := maps.Clone(current)
	maps.Copy(merged, changes)
	if taken, svcErr := p.updateAttributes(ctx, entityID, merged); taken || svcErr != nil {
		return svcErr
	}

	merged = current
	for _, name := range slices.Sorted(maps.Keys(changes)) {
		candidate := maps.Clone(merged)
		candidate[name] = changes[name]
		taken, svcErr := p.updateAttributes(ctx, entityID, candidate)
		if svcErr != nil {
			return svcErr
		}
		if !taken {
			logSkippedSyncAttribute(ctx, p.logger, entityID, name, "received value fails validation")
			continue
		}
		merged = candidate
	}
	return nil
}

// updateAttributes stores attributes as the account's profile, and returns false when the account can't
// take them: a value fails the schema or is held by another user.
func (p *defaultAuthnProvider) updateAttributes(ctx context.Context, entityID string,
	attributes map[string]interface{}) (bool, *tidcommon.ServiceError) {
	encoded, err := json.Marshal(attributes)
	if err != nil {
		return false, p.logAndReturnServerError(ctx, "Failed to encode the account's attributes", log.Error(err))
	}
	err = p.entitySvc.UpdateAttributes(ctx, entityID, encoded)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, entity.ErrSchemaValidationFailed), errors.Is(err, entity.ErrAttributeConflict):
		return false, nil
	default:
		return false, p.logAndReturnServerError(ctx, "Failed to update the account's attributes", log.Error(err))
	}
}

// syncGroups adds the account to the mapped groups that exist in its organization units. With replace it
// also removes the direct memberships the mapping no longer lists, keeping the groups a mapping name
// matched ambiguously and the memberships the sync can't remove: a control plane's, or a declarative
// group's.
func (p *defaultAuthnProvider) syncGroups(ctx context.Context, account *providers.Entity,
	groupIDs, ambiguousIDs []string, replace bool) *tidcommon.ServiceError {
	member := group.Member{ID: account.ID, Type: groupMemberType(account.Category)}
	var current []string
	if replace {
		var svcErr *tidcommon.ServiceError
		if current, svcErr = p.groupService.GetMemberGroupIDs(ctx, member); svcErr != nil {
			return p.syncFailure(ctx, "Failed to read the account's groups", svcErr)
		}
	}
	toAdd := slices.DeleteFunc(systemutils.UniqueStrings(groupIDs),
		func(id string) bool { return slices.Contains(current, id) })
	if len(toAdd) > 0 {
		if svcErr := p.groupService.AddMemberToExistingGroups(ctx, member, toAdd); svcErr != nil {
			return p.syncFailure(ctx, "Failed to add the account to its mapped groups", svcErr)
		}
	}
	for _, id := range current {
		if slices.Contains(groupIDs, id) || slices.Contains(ambiguousIDs, id) {
			continue
		}
		if _, svcErr := p.groupService.RemoveGroupMembers(ctx, id, []group.Member{member}); svcErr != nil {
			if svcErr.Code != group.ErrorGroupNotFound.Code && svcErr.Code != group.ErrorManagedGroupMember.Code &&
				svcErr.Code != group.ErrorImmutableGroup.Code {
				return p.syncFailure(ctx, "Failed to remove the account from an unmapped group", svcErr)
			}
			p.logger.Debug(ctx, "Keeping a group membership the sync can't remove",
				log.String("groupID", id), log.String("errorCode", svcErr.Code))
		}
	}
	return nil
}

// syncRoles assigns the account the mapped roles that can be assigned in its organization unit. With
// replace it also removes the direct assignments in that unit the mapping no longer lists, keeping the
// roles a mapping name matched ambiguously and the assignments the sync can't remove: a mandated one, a
// control plane's, or one of a role no longer shared with the unit.
func (p *defaultAuthnProvider) syncRoles(ctx context.Context, account *providers.Entity,
	roleIDs, ambiguousIDs []string, replace bool) *tidcommon.ServiceError {
	assignee := role.RoleAssignment{ID: account.ID, Type: roleAssigneeType(account.Category)}
	var current []string
	if replace {
		var svcErr *tidcommon.ServiceError
		if current, svcErr = p.roleAssignmentService.GetDirectRoleIDs(ctx, account.ID, account.OUID); svcErr != nil {
			return p.syncFailure(ctx, "Failed to read the account's roles", svcErr)
		}
	}
	toAdd := slices.DeleteFunc(systemutils.UniqueStrings(roleIDs),
		func(id string) bool { return slices.Contains(current, id) })
	if len(toAdd) > 0 {
		if svcErr := p.roleAssignmentService.AddAssigneeToExistingRoles(ctx, assignee, toAdd); svcErr != nil {
			return p.syncFailure(ctx, "Failed to assign the account its mapped roles", svcErr)
		}
	}
	for _, id := range current {
		if slices.Contains(roleIDs, id) || slices.Contains(ambiguousIDs, id) {
			continue
		}
		if svcErr := p.roleAssignmentService.RemoveAssignments(
			ctx, id, account.OUID, []role.RoleAssignment{assignee}); svcErr != nil {
			if svcErr.Code != role.ErrorRoleNotFound.Code && svcErr.Code != role.ErrorRoleNotSharedToOU.Code &&
				svcErr.Code != role.ErrorAssignmentMandated.Code &&
				svcErr.Code != role.ErrorManagedRoleAssignment.Code {
				return p.syncFailure(ctx, "Failed to remove an unmapped role from the account", svcErr)
			}
			p.logger.Debug(ctx, "Keeping a role assignment the sync can't remove",
				log.String("roleID", id), log.String("errorCode", svcErr.Code))
		}
	}
	return nil
}

// syncFailure maps a rejected group or role write to the sync failure, and any other failure to a
// server error.
func (p *defaultAuthnProvider) syncFailure(ctx context.Context, msg string,
	svcErr *tidcommon.ServiceError) *tidcommon.ServiceError {
	if svcErr.Type == tidcommon.ServerErrorType {
		return p.logAndReturnServerError(ctx, msg, log.String("errorCode", svcErr.Code))
	}
	p.logger.Debug(ctx, msg, log.String("errorCode", svcErr.Code))
	return newClientError(authnprovidercm.ErrorCodeFederatedSyncFailed, "Federated sync failed",
		svcErr.ErrorDescription.DefaultValue)
}

// withAncestorGroups returns groupIDs with each group's ancestors. A group whose ancestors can't be read
// is kept without them, like the rest of authorization mapping at sign-in.
func (p *defaultAuthnProvider) withAncestorGroups(ctx context.Context, groupIDs []string) []string {
	expanded := slices.Clone(groupIDs)
	for _, groupID := range groupIDs {
		ancestors, err := p.entitySvc.GetTransitiveGroupAncestors(ctx, groupID)
		if err != nil {
			p.logger.Warn(ctx, "Failed to resolve ancestors of a mapped group, continuing without them",
				log.String("groupID", groupID), log.Error(err))
			continue
		}
		expanded = systemutils.MergeUniqueStrings(expanded, ancestors)
	}
	return expanded
}

// groupMemberType returns how an entity of the category is named as a group member.
func groupMemberType(category providers.EntityCategory) group.MemberType {
	switch category {
	case providers.EntityCategoryApp:
		return group.MemberTypeApp
	case providers.EntityCategoryAgent:
		return group.MemberTypeAgent
	default:
		return group.MemberTypeUser
	}
}

// roleAssigneeType returns how an entity of the category is named as a role assignee.
func roleAssigneeType(category providers.EntityCategory) role.AssigneeType {
	switch category {
	case providers.EntityCategoryApp:
		return role.AssigneeTypeApp
	case providers.EntityCategoryAgent:
		return role.AssigneeTypeAgent
	default:
		return role.AssigneeTypeUser
	}
}

// logSkippedSyncAttribute logs a received attribute a sync leaves out. The value isn't logged, since it
// may hold personal data.
func logSkippedSyncAttribute(ctx context.Context, logger *log.Logger, entityID, attribute, reason string) {
	logger.Debug(ctx, "Skipping a federated attribute the account can't take",
		log.MaskedString(log.LoggerKeyEntityID, entityID), log.String("attribute", attribute),
		log.String("reason", reason))
}

// resolvedMappedAttributes returns the local attributes whose connection mapping finds its external
// attribute in claims.
func resolvedMappedAttributes(claims map[string]interface{},
	mappings []providers.AttributeMapping) map[string]bool {
	mapped := make(map[string]bool, len(mappings))
	for _, m := range mappings {
		if _, ok := systemutils.GetNestedValue(claims, m.ExternalAttribute); ok {
			mapped[m.LocalAttribute] = true
		}
	}
	return mapped
}

// jsonEqual reports whether a and b hold the same value once encoded, so a stored number and a received
// one of a different Go type compare equal. Values that can't be encoded are never equal.
func jsonEqual(a, b interface{}) bool {
	normalize := func(v interface{}) (interface{}, bool) {
		encoded, err := json.Marshal(v)
		if err != nil {
			return nil, false
		}
		var out interface{}
		if err := json.Unmarshal(encoded, &out); err != nil {
			return nil, false
		}
		return out, true
	}
	normalizedA, okA := normalize(a)
	normalizedB, okB := normalize(b)
	return okA && okB && reflect.DeepEqual(normalizedA, normalizedB)
}
