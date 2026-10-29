// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"reflect"
	"slices"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	entitytypemodel "github.com/thunder-id/thunderid/internal/entitytype/model"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/log"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// federatedSync is what a federated sign-in writes to the user holding its link. A nil Groups or Roles
// leaves that part unchanged.
type federatedSync struct {
	// IdpID and Sub are the connection and subject the sign-in asserted. Nothing is written unless the
	// user holds this link.
	IdpID string `json:"idpId"`
	Sub   string `json:"sub"`
	// AttributesByType holds the provider's values for the attributes to override, keyed by user type
	// and then by local attribute. Only the entry for the user's own type is written.
	AttributesByType map[string]map[string]interface{} `json:"attributesByType,omitempty"`
	Groups           *membershipSync                   `json:"groups,omitempty"`
	Roles            *membershipSync                   `json:"roles,omitempty"`
	// MappingIncomplete is set when groups and roles are overridden from a mapping that could not be
	// fully resolved, which fails the sign-in before the sync is kept.
	MappingIncomplete bool `json:"-"`
}

// membershipSync replaces a user's direct groups or roles.
type membershipSync struct {
	// IDs are the direct memberships after the sync.
	IDs []string `json:"ids"`
	// RetainIDs are kept when the user already has them, even when not in IDs.
	RetainIDs []string `json:"retainIds,omitempty"`
}

// federatedSyncer writes the sync a federated sign-in kept in runtime data to the user holding its link.
type federatedSyncer struct {
	entityProvider        entityprovider.EntityProviderInterface
	entityTypeService     entitytype.EntityTypeServiceInterface
	groupService          group.GroupServiceInterface
	roleAssignmentService role.RoleAssignmentServiceInterface
	authnProvider         providers.AuthnProviderManager
}

// newFederatedSyncer creates the syncer the executors that settle a link holder share.
func newFederatedSyncer(deps ExecutorDependencies) *federatedSyncer {
	return &federatedSyncer{
		entityProvider:        deps.EntityProvider,
		entityTypeService:     deps.EntityTypeService,
		groupService:          deps.GroupService,
		roleAssignmentService: deps.RoleAssignmentService,
		authnProvider:         deps.AuthnProvider,
	}
}

// buildFederatedSync decides what a returning user's sign-in writes, as the connection's sync modes
// configure, or returns nil when neither mode overrides the account. received holds the provider's
// claims, which each attribute mapping reads by its external attribute, and sub the subject the
// sign-in asserted.
func buildFederatedSync(idpDTO *providers.IDPDTO, resolved *idp.ResolvedAuthorizationMapping,
	received map[string]interface{}, sub string) *federatedSync {
	modes := syncConfiguration(idpDTO)
	overrideAttributes := modes.AttributesMode() == providers.SyncModeOverrideReceived
	overrideGroupsAndRoles := modes.GroupsAndRolesMode() == providers.SyncModeOverrideAll
	if !overrideAttributes && !overrideGroupsAndRoles {
		return nil
	}

	sync := &federatedSync{IdpID: idpDTO.ID, Sub: sub}

	// The account's user type isn't known here, and may differ from the one the claims resolve to, so
	// every user type's mapped values are computed and the sync writes its own type's.
	if overrideAttributes && idpDTO.AttributeConfiguration != nil {
		for _, entry := range idpDTO.AttributeConfiguration.UserTypeAttributeMappings {
			for _, m := range entry.Attributes {
				value, ok := systemutils.GetNestedValue(received, m.ExternalAttribute)
				if !ok || m.LocalAttribute == userAttributeSub {
					continue
				}

				if sync.AttributesByType == nil {
					sync.AttributesByType = make(map[string]map[string]interface{})
				}
				if sync.AttributesByType[entry.UserType] == nil {
					sync.AttributesByType[entry.UserType] = make(map[string]interface{})
				}

				sync.AttributesByType[entry.UserType][m.LocalAttribute] = value
			}
		}
	}

	if !overrideGroupsAndRoles || (!resolved.MapsGroups && !resolved.MapsRoles) {
		return sync
	}

	if !resolved.Complete {
		sync.MappingIncomplete = true
		return sync
	}

	if resolved.MapsGroups {
		sync.Groups = &membershipSync{}
	}

	if resolved.MapsRoles {
		sync.Roles = &membershipSync{}
	}

	for _, t := range resolved.Targets {
		switch {
		case t.Type == providers.AuthorizationTargetGroup && resolved.MapsGroups:
			sync.Groups.IDs = append(sync.Groups.IDs, t.ID)
		case t.Type == providers.AuthorizationTargetRole && resolved.MapsRoles:
			sync.Roles.IDs = append(sync.Roles.IDs, t.ID)
		}
	}

	for _, t := range resolved.Ambiguous {
		switch {
		case t.Type == providers.AuthorizationTargetGroup && resolved.MapsGroups:
			sync.Groups.RetainIDs = append(sync.Groups.RetainIDs, t.ID)
		case t.Type == providers.AuthorizationTargetRole && resolved.MapsRoles:
			sync.Roles.RetainIDs = append(sync.Roles.RetainIDs, t.ID)
		}
	}

	return sync
}

// setPendingFederatedSync keeps sync in runtime data for the node that settles the user holding the
// link. A nil sync clears one an earlier sign-in in the flow kept.
func setPendingFederatedSync(execResp *providers.ExecutorResponse, sync *federatedSync) error {
	if sync == nil {
		execResp.RuntimeData[common.RuntimeKeyPendingFederatedSync] = ""
		return nil
	}

	encoded, err := json.Marshal(sync)
	if err != nil {
		return fmt.Errorf("failed to encode the pending federated sync: %w", err)
	}

	execResp.RuntimeData[common.RuntimeKeyPendingFederatedSync] = string(encoded)
	return nil
}

// syncPendingUser writes the sync a federated sign-in kept in runtime data to userID's account, and
// signs the user in again when attributes were written, so the rest of the flow reads the stored
// values. Nothing is written unless the user holds the sign-in's link, and nothing is synced when no
// user is settled yet. A rejected write fails the sign-in on execResp and returns false. Server
// failures are returned as errors. Each write commits on its own, so a sync that fails partway is
// completed by the next sign-in.
func (f *federatedSyncer) syncPendingUser(ctx *providers.NodeContext, execResp *providers.ExecutorResponse,
	userID string, logger *log.Logger) (bool, error) {
	pending, ok := execResp.RuntimeData[common.RuntimeKeyPendingFederatedSync]
	if !ok {
		pending = ctx.RuntimeData[common.RuntimeKeyPendingFederatedSync]
	}
	if pending == "" {
		return true, nil
	}

	var sync federatedSync
	if err := json.Unmarshal([]byte(pending), &sync); err != nil {
		return false, fmt.Errorf("failed to decode the pending federated sync: %w", err)
	}

	if userID == "" {
		return true, nil
	}

	holders, epErr := f.entityProvider.GetLinkedEntities(ctx.Context, sync.IdpID, sync.Sub)
	if epErr != nil {
		if epErr.Code == entityprovider.ErrorCodeNotImplemented {
			logger.Debug(ctx.Context, "Entity provider does not support linked accounts, skipping federated sync",
				log.MaskedString(log.LoggerKeyUserID, userID))
			return true, nil
		}
		return false, fmt.Errorf("failed to read the federated link's holders: %w", epErr)
	}
	holder := slices.IndexFunc(holders, func(e *providers.Entity) bool { return e.ID == userID })
	if holder < 0 {
		execResp.RuntimeData[common.RuntimeKeyPendingFederatedSync] = ""
		logger.Debug(ctx.Context, "User does not hold the federated link, skipping sync",
			log.MaskedString(log.LoggerKeyUserID, userID))
		return true, nil
	}
	user := holders[holder]

	attributesWritten, err := f.syncAttributes(ctx.Context, user, sync.AttributesByType[user.Type], logger)
	if err != nil {
		return false, err
	}
	svcErr := f.syncGroups(ctx.Context, user, sync.Groups, logger)
	if svcErr == nil {
		svcErr = f.syncRoles(ctx.Context, user, sync.Roles, logger)
	}
	if svcErr == nil && attributesWritten {
		// The AuthUser loaded the account's attributes when it authenticated, before this write.
		svcErr = authenticateEntityByID(ctx, f.authnProvider, userID, execResp.AuthUser, execResp)
	}
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return false, fmt.Errorf("failed to sync federated user: %s", svcErr.Code)
		}
		logger.Debug(ctx.Context, "Federated user sync was rejected", log.MaskedString(log.LoggerKeyUserID, userID),
			log.String("errorCode", svcErr.Code))
		execResp.Status = providers.ExecFailure
		execResp.Error = cmp.Or(statedHold(svcErr), &ErrFederatedUserSyncFailed)
		return false, nil
	}

	// The sync is written once, by the first node that settles the user.
	execResp.RuntimeData[common.RuntimeKeyPendingFederatedSync] = ""
	if logger.IsDebugEnabled() {
		logger.Debug(ctx.Context, "Synced federated user", log.MaskedString(log.LoggerKeyUserID, userID),
			log.Bool("attributesSynced", attributesWritten),
			log.Bool("groupsSynced", sync.Groups != nil), log.Bool("rolesSynced", sync.Roles != nil))
	}

	return true, nil
}

// syncAttributes writes the received values of the user type's non-credential attributes that differ
// from the stored ones, and reports whether any were written. A received value the account can't take,
// one of the wrong shape or one that fails validation, is skipped rather than failing the sync, so a
// provider sending one bad value doesn't block the user's sign-ins. A read-only user is left unchanged.
func (f *federatedSyncer) syncAttributes(ctx context.Context, user *providers.Entity,
	received map[string]interface{}, logger *log.Logger) (bool, error) {
	if len(received) == 0 || user.IsReadOnly || user.Category != providers.EntityCategoryUser {
		return false, nil
	}

	schemaAttrs, svcErr := f.entityTypeService.GetAttributes(ctx, entitytype.TypeCategoryUser, user.Type,
		entitytype.AttributeFilter{AllowNonCredential: true})
	if svcErr != nil {
		return false, fmt.Errorf("failed to get the user type's attributes: %s", svcErr.Code)
	}

	current := make(map[string]interface{})
	if len(user.Attributes) > 0 {
		if err := json.Unmarshal(user.Attributes, &current); err != nil {
			return false, fmt.Errorf("failed to read user attributes: %w", err)
		}
	}

	changes := make(map[string]interface{})
	for _, a := range schemaAttrs {
		raw, ok := received[a.Attribute]
		if !ok {
			continue
		}
		value, ok := syncValue(raw, a.Type)
		if !ok {
			logSkippedSyncAttribute(ctx, logger, user.ID, a.Attribute,
				"received value doesn't match the attribute type")
			continue
		}
		if value == nil || jsonEqual(value, current[a.Attribute]) {
			continue
		}
		changes[a.Attribute] = value
	}
	if len(changes) == 0 {
		return false, nil
	}

	merged := maps.Clone(current)
	maps.Copy(merged, changes)
	if taken, err := f.updateAttributes(ctx, user.ID, merged); taken || err != nil {
		return taken, err
	}

	// Some value is refused. Each change is written on its own, on top of the ones already taken, so
	// only the values the account can't take are left out.
	written := false
	merged = current
	for _, name := range slices.Sorted(maps.Keys(changes)) {
		candidate := maps.Clone(merged)
		candidate[name] = changes[name]
		taken, err := f.updateAttributes(ctx, user.ID, candidate)
		if err != nil {
			return written, err
		}
		if !taken {
			logSkippedSyncAttribute(ctx, logger, user.ID, name, "received value fails validation")
			continue
		}
		merged, written = candidate, true
	}
	return written, nil
}

// updateAttributes stores attributes as the user's profile, and returns false when the account can't
// take them: a value fails the schema or is held by another user.
func (f *federatedSyncer) updateAttributes(ctx context.Context, userID string,
	attributes map[string]interface{}) (bool, error) {
	encoded, err := json.Marshal(attributes)
	if err != nil {
		return false, fmt.Errorf("failed to encode user attributes: %w", err)
	}
	epErr := f.entityProvider.UpdateAttributes(ctx, userID, encoded)
	if epErr == nil {
		return true, nil
	}
	if epErr.Code == entityprovider.ErrorCodeSchemaValidationFailed ||
		epErr.Code == entityprovider.ErrorCodeAttributeConflict {
		return false, nil
	}
	return false, fmt.Errorf("failed to update user attributes: %w", epErr)
}

// syncGroups sets the user's direct group memberships to the mapped groups. Groups that no longer exist
// or lie outside the user's organization units are skipped, and a membership a control plane deployed
// or a group's declaration lists is kept. Nothing is written when groups are declarative-only.
func (f *federatedSyncer) syncGroups(ctx context.Context, user *providers.Entity, groups *membershipSync,
	logger *log.Logger) *tidcommon.ServiceError {
	if groups == nil || group.IsDeclarativeOnly() {
		return nil
	}
	member := group.Member{ID: user.ID, Type: traitsFor(entitytype.TypeCategory(user.Category)).groupMemberType}
	current, svcErr := f.groupService.GetMemberGroupIDs(ctx, member)
	if svcErr != nil {
		return svcErr
	}

	toAdd := slices.DeleteFunc(systemutils.UniqueStrings(groups.IDs),
		func(id string) bool { return slices.Contains(current, id) })
	if len(toAdd) > 0 {
		if svcErr := f.groupService.AddMemberToExistingGroups(ctx, member, toAdd); svcErr != nil {
			return svcErr
		}
	}
	for _, id := range current {
		if slices.Contains(groups.IDs, id) || slices.Contains(groups.RetainIDs, id) {
			continue
		}
		if _, svcErr := f.groupService.RemoveGroupMembers(ctx, id, []group.Member{member}); svcErr != nil {
			if svcErr.Code != group.ErrorGroupNotFound.Code && svcErr.Code != group.ErrorManagedGroupMember.Code &&
				svcErr.Code != group.ErrorImmutableGroup.Code {
				return svcErr
			}
			logger.Debug(ctx, "Keeping a group membership the sync can't remove",
				log.String("groupID", id), log.String("errorCode", svcErr.Code))
		}
	}
	return nil
}

// syncRoles sets the user's direct role assignments in its organization unit to the mapped roles. A
// role that no longer exists, can't be assigned or unassigned in the unit, is mandated by a sharing
// policy, or was deployed by a control plane is left as it is. Nothing is written when roles are
// declarative-only.
func (f *federatedSyncer) syncRoles(ctx context.Context, user *providers.Entity, roles *membershipSync,
	logger *log.Logger) *tidcommon.ServiceError {
	if roles == nil || role.IsDeclarativeOnly() {
		return nil
	}
	assignee := role.RoleAssignment{ID: user.ID,
		Type: traitsFor(entitytype.TypeCategory(user.Category)).roleAssigneeType}
	current, svcErr := f.roleAssignmentService.GetDirectRoleIDs(ctx, user.ID, user.OUID)
	if svcErr != nil {
		return svcErr
	}

	toAdd := slices.DeleteFunc(systemutils.UniqueStrings(roles.IDs),
		func(id string) bool { return slices.Contains(current, id) })
	if len(toAdd) > 0 {
		if svcErr := f.roleAssignmentService.AddAssigneeToExistingRoles(ctx, assignee, toAdd); svcErr != nil {
			return svcErr
		}
	}
	for _, id := range current {
		if slices.Contains(roles.IDs, id) || slices.Contains(roles.RetainIDs, id) {
			continue
		}
		if svcErr := f.roleAssignmentService.RemoveAssignments(
			ctx, id, user.OUID, []role.RoleAssignment{assignee}); svcErr != nil {
			if svcErr.Code != role.ErrorRoleNotFound.Code && svcErr.Code != role.ErrorRoleNotSharedToOU.Code &&
				svcErr.Code != role.ErrorAssignmentMandated.Code &&
				svcErr.Code != role.ErrorManagedRoleAssignment.Code {
				return svcErr
			}
			logger.Debug(ctx, "Keeping a role assignment the sync can't remove",
				log.String("roleID", id), log.String("errorCode", svcErr.Code))
		}
	}
	return nil
}

// logSkippedSyncAttribute logs a received attribute a federated sync leaves out. The value isn't
// logged, since it may hold personal data.
func logSkippedSyncAttribute(ctx context.Context, logger *log.Logger, userID, attribute, reason string) {
	logger.Debug(ctx, "Skipping a federated attribute the account can't take",
		log.MaskedString(log.LoggerKeyUserID, userID), log.String("attribute", attribute),
		log.String("reason", reason))
}

// syncValue returns the value a received attribute is stored as, or nil when it's null or an empty
// string. Arrays and objects, empty ones included, keep their structure, and a single value received
// for an array becomes a one-item array, as providers often send a one-valued claim. Other values are
// converted to the schema type. It returns false when the received value has the wrong shape for the
// type, or converts to a number JSON can't hold.
func syncValue(received interface{}, schemaType string) (interface{}, bool) {
	if text, ok := received.(string); received == nil || ok && text == "" {
		return nil, true
	}
	_, isObject := received.(map[string]interface{})
	isArray := false
	if !isObject {
		kind := reflect.ValueOf(received).Kind()
		isArray = kind == reflect.Slice || kind == reflect.Array
	}

	switch schemaType {
	case entitytypemodel.TypeArray:
		if isObject {
			return nil, false
		}
		if !isArray {
			return []interface{}{received}, true
		}
		return received, true
	case entitytypemodel.TypeObject:
		return received, isObject
	}
	if isObject || isArray {
		return nil, false
	}

	value := systemutils.ConvertInterfaceValueToString(received)
	if value == "" {
		return nil, true
	}
	converted := convertToSchemaType(value, schemaType)
	if number, ok := converted.(float64); ok && (math.IsNaN(number) || math.IsInf(number, 0)) {
		return nil, false
	}
	return converted, true
}

// jsonEqual reports whether a and b hold the same value once encoded, so a stored number and a
// received one of a different Go type compare equal. Values that can't be encoded are never equal.
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
