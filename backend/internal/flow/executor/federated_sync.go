// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	entitytypemodel "github.com/thunder-id/thunderid/internal/entitytype/model"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/log"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/internal/user"
)

// federatedSyncer keeps a returning federated user's account in line with the provider, as the
// connection's sync modes configure.
type federatedSyncer struct {
	entityProvider    entityprovider.EntityProviderInterface
	entityTypeService entitytype.EntityTypeServiceInterface
	userService       user.UserServiceInterface
}

func newFederatedSyncer(deps ExecutorDependencies) *federatedSyncer {
	return &federatedSyncer{
		entityProvider:    deps.EntityProvider,
		entityTypeService: deps.EntityTypeService,
		userService:       deps.UserService,
	}
}

// federatedSyncPlan is what a returning federated sign-in writes to the account.
type federatedSyncPlan struct {
	// attributes holds the attributes whose values change. Nil leaves the attributes unchanged.
	attributes map[string]interface{}
	// groupIDs and roleIDs set the account's direct groups and roles. Nil leaves them unchanged.
	groupIDs []string
	roleIDs  []string
}

// planFederatedSync decides what a returning federated sign-in writes. schemaAttrs are the user
// type's non-credential attributes, current the account's attributes and received the values the
// provider sent for the local attributes the connection maps. The subject identifier is never
// changed. Groups are synced only when the authorization mapping can resolve groups, and roles only
// when it can resolve roles.
func planFederatedSync(
	attributesMode, groupsAndRolesMode providers.SyncMode,
	schemaAttrs []entitytype.AttributeInfo,
	current, received map[string]interface{},
	mapsGroups, mapsRoles bool, mappedGroupIDs, mappedRoleIDs []string,
) federatedSyncPlan {
	var plan federatedSyncPlan

	if attributesMode == providers.SyncModeOverrideReceived {
		for _, a := range schemaAttrs {
			if a.Attribute == userAttributeSub {
				continue
			}
			value := syncValue(received[a.Attribute], a.Type)
			if value == nil || jsonEqual(value, current[a.Attribute]) {
				continue
			}
			if plan.attributes == nil {
				plan.attributes = make(map[string]interface{})
			}
			plan.attributes[a.Attribute] = value
		}
	}

	if groupsAndRolesMode == providers.SyncModeOverrideAll {
		if mapsGroups {
			plan.groupIDs = append([]string{}, mappedGroupIDs...)
		}
		if mapsRoles {
			plan.roleIDs = append([]string{}, mappedRoleIDs...)
		}
	}
	return plan
}

// syncValue returns the value a received claim is stored as, or nil when the provider sent none.
// Arrays and objects keep their structure. Other values are converted the same way provisioning
// converts them when it creates the account.
func syncValue(received interface{}, schemaType string) interface{} {
	switch schemaType {
	case entitytypemodel.TypeArray, entitytypemodel.TypeObject:
		return received
	}
	value := systemutils.ConvertInterfaceValueToString(received)
	if value == "" {
		return nil
	}
	return convertToSchemaType(value, schemaType)
}

// mappingTargetTypes reports whether the authorization mapping can resolve groups and roles.
func mappingTargetTypes(mapping *providers.AuthorizationMapping) (mapsGroups, mapsRoles bool) {
	if mapping == nil {
		return false, false
	}
	note := func(t providers.AuthorizationTargetType) {
		mapsGroups = mapsGroups || t == providers.AuthorizationTargetGroup
		mapsRoles = mapsRoles || t == providers.AuthorizationTargetRole
	}
	for _, r := range mapping.Rules {
		for _, v := range r.Values {
			for _, t := range v.Targets {
				note(t.Type)
			}
		}
	}
	for _, d := range mapping.Direct {
		note(d.TargetType)
	}
	return mapsGroups, mapsRoles
}

// jsonEqual reports whether a and b hold the same value once encoded, so a stored number and a
// received one of a different Go type compare equal.
func jsonEqual(a, b interface{}) bool {
	normalize := func(v interface{}) interface{} {
		encoded, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		var out interface{}
		_ = json.Unmarshal(encoded, &out)
		return out
	}
	return reflect.DeepEqual(normalize(a), normalize(b))
}

// mappedAttributes returns the values the provider sent for the local attributes the connection's
// attribute mappings name for userType, keyed by local attribute.
func mappedAttributes(
	idpDTO *providers.IDPDTO, userType string, received map[string]interface{},
) map[string]interface{} {
	mapped := make(map[string]interface{})
	if idpDTO.AttributeConfiguration == nil {
		return mapped
	}
	for _, m := range idpDTO.AttributeConfiguration.UserTypeAttributeMappings {
		if m.UserType != userType {
			continue
		}
		for _, a := range m.Attributes {
			if value, ok := systemutils.GetNestedValue(received, a.ExternalAttribute); ok {
				mapped[a.LocalAttribute] = value
			}
		}
	}
	return mapped
}

// syncAccount syncs the returning user's account with what the provider sent, as the
// connection's sync modes configure. mappingComplete is false when part of the authorization
// mapping could not be resolved, in which case groups and roles are not overridden from it.
func (s *federatedSyncer) syncAccount(
	ctx *providers.NodeContext, idpDTO *providers.IDPDTO, userID string,
	received map[string]interface{}, mappedGroupIDs, mappedRoleIDs []string, mappingComplete bool,
	logger *log.Logger,
) error {
	attributesMode := idp.GetAttributesSyncMode(idpDTO)
	groupsAndRolesMode := idp.GetGroupsAndRolesSyncMode(idpDTO)
	var mapping *providers.AuthorizationMapping
	if idpDTO.AttributeConfiguration != nil {
		mapping = idpDTO.AttributeConfiguration.AuthorizationMapping
	}
	mapsGroups, mapsRoles := mappingTargetTypes(mapping)
	syncAttributes := attributesMode == providers.SyncModeOverrideReceived
	syncGroupsAndRoles := groupsAndRolesMode == providers.SyncModeOverrideAll && (mapsGroups || mapsRoles)
	if !syncAttributes && !syncGroupsAndRoles {
		return nil
	}
	if syncGroupsAndRoles && !mappingComplete {
		return errors.New("authorization mapping could not be fully resolved")
	}

	entity, epErr := s.entityProvider.GetEntity(userID)
	if epErr != nil {
		return fmt.Errorf("failed to load user: %s", epErr.Error())
	}
	var current map[string]interface{}
	if len(entity.Attributes) > 0 {
		if err := json.Unmarshal(entity.Attributes, &current); err != nil {
			return fmt.Errorf("failed to read user attributes: %w", err)
		}
	}
	schemaAttrs, svcErr := s.entityTypeService.GetAttributes(ctx.Context, entitytype.TypeCategoryUser,
		entity.Type, entitytype.AttributeFilter{AllowNonCredential: true})
	if svcErr != nil {
		return fmt.Errorf("failed to read user type attributes: %s", svcErr.ErrorDescription.DefaultValue)
	}

	plan := planFederatedSync(attributesMode, groupsAndRolesMode, schemaAttrs, current,
		mappedAttributes(idpDTO, entity.Type, received), mapsGroups, mapsRoles, mappedGroupIDs, mappedRoleIDs)

	if plan.attributes == nil && plan.groupIDs == nil && plan.roleIDs == nil {
		return nil
	}
	if svcErr := s.userService.SyncFederatedUser(
		ctx.Context, userID, plan.attributes, plan.groupIDs, plan.roleIDs); svcErr != nil {
		return fmt.Errorf("failed to write the sync: %s", svcErr.Error.DefaultValue)
	}

	logger.Info(ctx.Context, "Synced federated user", log.MaskedString(log.LoggerKeyUserID, userID),
		log.Bool("attributesChanged", plan.attributes != nil),
		log.Bool("groupsSynced", plan.groupIDs != nil), log.Bool("rolesSynced", plan.roleIDs != nil))
	return nil
}

// syncFederatedUser runs syncAccount with the authorization targets the federated login
// resolved. On failure it fails the sign-in on execResp and returns false.
func (s *federatedSyncer) syncFederatedUser(
	ctx *providers.NodeContext, execResp *providers.ExecutorResponse, idpDTO *providers.IDPDTO,
	userID string, received map[string]interface{}, mappingComplete bool, logger *log.Logger,
) bool {
	mappedGroupIDs := systemutils.ParseStringArray(execResp.RuntimeData[common.RuntimeKeyMappedGroupIDs], " ")
	mappedRoleIDs := systemutils.ParseStringArray(execResp.RuntimeData[common.RuntimeKeyMappedRoleIDs], " ")
	if err := s.syncAccount(ctx, idpDTO, userID, received, mappedGroupIDs, mappedRoleIDs,
		mappingComplete, logger); err != nil {
		logger.Error(ctx.Context, "Failed to sync federated user",
			log.MaskedString(log.LoggerKeyUserID, userID), log.Error(err))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrFederatedUserSyncFailed
		return false
	}
	return true
}
