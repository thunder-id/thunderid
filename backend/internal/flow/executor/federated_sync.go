// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/log"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// federatedSync is what a federated sign-in writes to the user holding its link.
type federatedSync struct {
	Entity providers.FederatedEntitySync `json:"entity"`
	// MappingIncomplete is set when groups and roles are overridden from a mapping that could not be
	// fully resolved, which fails the sync.
	MappingIncomplete bool `json:"mappingIncomplete,omitempty"`
}

// buildFederatedSync decides what a returning user's sign-in writes, as the connection's sync modes
// configure, or returns nil when neither mode overrides the account. received holds the provider's
// claims, which each attribute mapping reads by its external attribute, and sub the subject the
// sign-in asserted.
func buildFederatedSync(idpDTO *providers.IDPDTO, resolved *idp.ResolvedAuthorizationMapping,
	received map[string]interface{}, sub string) *federatedSync {
	overrideAttributes := idp.GetAttributesSyncMode(idpDTO) == providers.SyncModeOverrideReceived
	overrideGroupsAndRoles := idp.GetGroupsAndRolesSyncMode(idpDTO) == providers.SyncModeOverrideAll
	if !overrideAttributes && !overrideGroupsAndRoles {
		return nil
	}

	sync := &federatedSync{Entity: providers.FederatedEntitySync{
		Link: providers.LinkedAccount{IdpID: idpDTO.ID, Sub: sub},
	}}

	// The account's user type isn't known here, and may differ from the one the claims resolve to, so
	// every user type's mapped values are computed and the entity service writes its own type's.
	if overrideAttributes && idpDTO.AttributeConfiguration != nil {
		for _, entry := range idpDTO.AttributeConfiguration.UserTypeAttributeMappings {
			for _, m := range entry.Attributes {
				value, ok := systemutils.GetNestedValue(received, m.ExternalAttribute)
				if !ok || m.LocalAttribute == userAttributeSub {
					continue
				}

				if sync.Entity.AttributesByType == nil {
					sync.Entity.AttributesByType = make(map[string]map[string]interface{})
				}
				if sync.Entity.AttributesByType[entry.UserType] == nil {
					sync.Entity.AttributesByType[entry.UserType] = make(map[string]interface{})
				}

				sync.Entity.AttributesByType[entry.UserType][m.LocalAttribute] = value
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
		sync.Entity.Groups = &providers.MembershipSync{}
	}

	if resolved.MapsRoles {
		sync.Entity.Roles = &providers.MembershipSync{}
	}

	for _, t := range resolved.Targets {
		switch {
		case t.Type == providers.AuthorizationTargetGroup && resolved.MapsGroups:
			sync.Entity.Groups.IDs = append(sync.Entity.Groups.IDs, t.ID)
		case t.Type == providers.AuthorizationTargetRole && resolved.MapsRoles:
			sync.Entity.Roles.IDs = append(sync.Entity.Roles.IDs, t.ID)
		}
	}

	for _, t := range resolved.Ambiguous {
		switch {
		case t.Type == providers.AuthorizationTargetGroup && resolved.MapsGroups:
			sync.Entity.Groups.RetainIDs = append(sync.Entity.Groups.RetainIDs, t.ID)
		case t.Type == providers.AuthorizationTargetRole && resolved.MapsRoles:
			sync.Entity.Roles.RetainIDs = append(sync.Entity.Roles.RetainIDs, t.ID)
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

// syncPendingFederatedUser writes the sync a federated sign-in kept in runtime data to userID's
// account, which writes nothing unless the user holds the sign-in's link. Nothing is synced when no
// user is settled. A rejected write fails the sign-in on execResp and returns false. Server failures
// are returned as errors.
func syncPendingFederatedUser(
	ctx *providers.NodeContext, execResp *providers.ExecutorResponse,
	entityProvider entityprovider.EntityProviderInterface, userID string, logger *log.Logger,
) (bool, error) {
	pending, ok := execResp.RuntimeData[common.RuntimeKeyPendingFederatedSync]
	if !ok {
		pending = ctx.RuntimeData[common.RuntimeKeyPendingFederatedSync]
	}
	if pending == "" || userID == "" {
		return true, nil
	}

	var sync federatedSync
	if err := json.Unmarshal([]byte(pending), &sync); err != nil {
		return false, fmt.Errorf("failed to decode the pending federated sync: %w", err)
	}

	if sync.MappingIncomplete {
		return false, errors.New("failed to sync federated user: authorization mapping could not be fully resolved")
	}

	if epErr := entityProvider.SyncFederatedEntity(userID, sync.Entity); epErr != nil {
		if epErr.Code == entityprovider.ErrorCodeNotImplemented {
			logger.Debug(ctx.Context, "Entity provider does not support federated sync, skipping",
				log.MaskedString(log.LoggerKeyUserID, userID))
			return true, nil
		}
		if epErr.Code == entityprovider.ErrorCodeSystemError {
			return false, fmt.Errorf("failed to sync federated user: %w", epErr)
		}

		logger.Debug(ctx.Context, "Federated user sync was rejected", log.MaskedString(log.LoggerKeyUserID, userID),
			log.String("errorCode", string(epErr.Code)))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrFederatedUserSyncFailed
		return false, nil
	}

	// The sync is written once, by the first node that settles the user.
	execResp.RuntimeData[common.RuntimeKeyPendingFederatedSync] = ""
	if logger.IsDebugEnabled() {
		logger.Debug(ctx.Context, "Synced federated user", log.MaskedString(log.LoggerKeyUserID, userID),
			log.Bool("attributesSynced", sync.Entity.AttributesByType != nil),
			log.Bool("groupsSynced", sync.Entity.Groups != nil), log.Bool("rolesSynced", sync.Entity.Roles != nil))
	}

	return true, nil
}
