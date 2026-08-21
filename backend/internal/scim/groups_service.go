// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package scim

import (
	"context"
	"strings"

	"github.com/thunder-id/thunderid/internal/group"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

const groupServiceLoggerComponentName = "SCIMGroupsService"

// SCIMGroupsServiceInterface defines the Groups CRUD operations exposed to the handler.
type SCIMGroupsServiceInterface interface {
	ListGroups(ctx context.Context, startIndex, count int, baseURL string,
	) (SCIMGroupListResponse, *tidcommon.ServiceError)
	CreateGroup(ctx context.Context, displayName string, members []SCIMGroupMember,
		baseURL string) (*SCIMGroup, *tidcommon.ServiceError)
	GetGroup(ctx context.Context, groupID, baseURL string,
	) (*SCIMGroup, *tidcommon.ServiceError)
	ReplaceGroup(ctx context.Context, groupID, displayName string,
		members []SCIMGroupMember, ifMatch, baseURL string) (*SCIMGroup, *tidcommon.ServiceError)
	PatchGroup(ctx context.Context, groupID string, actions []SCIMGroupPatchAction,
		ifMatch, baseURL string) (*SCIMGroup, *tidcommon.ServiceError)
	DeleteGroup(ctx context.Context, groupID string, ifMatch string) *tidcommon.ServiceError
}

// scimGroupsService implements SCIMGroupsServiceInterface.
type scimGroupsService struct {
	groupService group.GroupServiceInterface
}

// newSCIMGroupsService creates a new scimGroupsService instance.
func newSCIMGroupsService(groupService group.GroupServiceInterface) SCIMGroupsServiceInterface {
	return &scimGroupsService{
		groupService: groupService,
	}
}

// ListGroups retrieves a paginated list of SCIM Group resources.
func (s *scimGroupsService) ListGroups(ctx context.Context, startIndex, count int,
	baseURL string) (SCIMGroupListResponse, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, groupServiceLoggerComponentName))

	if startIndex < 1 {
		startIndex = 1
	}
	if count < 0 {
		count = 0
	}

	// GetGroupList rejects a limit below 1, so a count of 0 (client wants only
	// totalResults, no resources per RFC 7644 §3.4.2.4) fetches a single row
	// and discards it below.
	fetchLimit := count
	if fetchLimit == 0 {
		fetchLimit = 1
	}

	offset := startIndex - 1
	listResp, svcErr := s.groupService.GetGroupList(ctx, fetchLimit, offset, true)
	if svcErr != nil {
		logger.Error(ctx, "SCIM ListGroups: failed to get group list", log.Any("error", svcErr))
		return SCIMGroupListResponse{}, mapGroupServiceErrorToSCIM(svcErr)
	}
	if count == 0 {
		return buildSCIMGroupListResponse(nil, listResp.TotalResults, startIndex, 0), nil
	}
	scimGroups := make([]SCIMGroup, 0, len(listResp.Groups))
	for _, g := range listResp.Groups {
		// GroupBasic carries no Members; fetch separately. GetGroup would not populate them
		// either for database-backed groups, so calling it here would be redundant.
		members, svcErr := s.fetchAllGroupMembers(ctx, g.ID, true)
		if svcErr != nil {
			logger.Error(ctx, "SCIM ListGroups: failed to fetch group members",
				log.String("groupID", g.ID), log.Any("error", svcErr))
			return SCIMGroupListResponse{}, mapGroupServiceErrorToSCIM(svcErr)
		}
		full := group.Group{
			ID:          g.ID,
			Name:        g.Name,
			Description: g.Description,
			OUID:        g.OUID,
			OUHandle:    g.OUHandle,
			IsReadOnly:  g.IsReadOnly,
			Members:     members,
		}
		scimGroups = append(scimGroups, buildSCIMGroupResource(full, baseURL))
	}
	return buildSCIMGroupListResponse(scimGroups, listResp.TotalResults, startIndex, len(scimGroups)), nil
}

// GetGroup retrieves a single SCIM Group by ID.
func (s *scimGroupsService) GetGroup(ctx context.Context, groupID, baseURL string,
) (*SCIMGroup, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, groupServiceLoggerComponentName))

	g, svcErr := s.groupService.GetGroup(ctx, groupID, true)
	if svcErr != nil {
		logger.Debug(ctx, "SCIM GetGroup: group service error",
			log.String("groupID", groupID), log.Any("error", svcErr))
		return nil, mapGroupServiceErrorToSCIM(svcErr)
	}
	// GetGroup does not populate Members for database-backed groups, so fetch separately.
	members, svcErr := s.fetchAllGroupMembers(ctx, groupID, true)
	if svcErr != nil {
		logger.Error(ctx, "SCIM GetGroup: failed to fetch group members",
			log.String("groupID", groupID), log.Any("error", svcErr))
		return nil, mapGroupServiceErrorToSCIM(svcErr)
	}
	g.Members = members
	scimGroup := buildSCIMGroupResource(*g, baseURL)
	return &scimGroup, nil
}

// CreateGroup creates a new SCIM Group resource.
func (s *scimGroupsService) CreateGroup(ctx context.Context, displayName string,
	members []SCIMGroupMember, baseURL string) (*SCIMGroup, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, groupServiceLoggerComponentName))

	thunderIDMembers, svcErr := scimMembersToThunderID(members)
	if svcErr != nil {
		logger.Debug(ctx, "SCIM CreateGroup: invalid member type", log.Any("error", svcErr))
		return nil, svcErr
	}

	req := group.CreateGroupRequest{
		Name:    displayName,
		OUID:    security.GetOUID(ctx),
		Members: thunderIDMembers,
	}

	created, svcErr := s.groupService.CreateGroup(ctx, req)
	if svcErr != nil {
		logger.Error(ctx, "SCIM CreateGroup: group service error", log.Any("error", svcErr))
		return nil, mapGroupServiceErrorToSCIM(svcErr)
	}
	return s.GetGroup(ctx, created.ID, baseURL)
}

// ReplaceGroup replaces a group's displayName and members (RFC 7644 full replace).
// KNOWN LIMITATION: member removal and addition below are separate, non-transactional
// group service calls. If AddGroupMembers fails after RemoveGroupMembers has already
// succeeded, the group is left with members removed and none added, while the caller
// sees only the failure. group.GroupServiceInterface has no atomic replace-members
// operation; fixing this requires a new transactional method at the group service layer.
func (s *scimGroupsService) ReplaceGroup(ctx context.Context, groupID, displayName string,
	members []SCIMGroupMember, ifMatch, baseURL string) (*SCIMGroup, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, groupServiceLoggerComponentName))

	thunderIDMembers, svcErr := scimMembersToThunderID(members)
	if svcErr != nil {
		logger.Debug(ctx, "SCIM ReplaceGroup: invalid member type", log.Any("error", svcErr))
		return nil, svcErr
	}

	g, svcErr := s.groupService.GetGroup(ctx, groupID, false)
	if svcErr != nil {
		logger.Debug(ctx, "SCIM ReplaceGroup: group service error",
			log.String("groupID", groupID), log.Any("error", svcErr))
		return nil, mapGroupServiceErrorToSCIM(svcErr)
	}
	if g.IsReadOnly {
		logger.Debug(ctx, "SCIM ReplaceGroup: group is read-only", log.String("groupID", groupID))
		return nil, &ErrorMutabilityViolation
	}

	if trimmed := strings.TrimSpace(ifMatch); trimmed != "" {
		existingMembers, svcErr := s.fetchAllGroupMembers(ctx, groupID, true)
		if svcErr != nil {
			logger.Error(ctx, "SCIM ReplaceGroup: failed to fetch group members",
				log.String("groupID", groupID), log.Any("error", svcErr))
			return nil, mapGroupServiceErrorToSCIM(svcErr)
		}
		g.Members = existingMembers
		if svcErr := checkIfMatch(trimmed, generateVersion(groupVersionState(*g))); svcErr != nil {
			return nil, svcErr
		}
	}

	req := group.UpdateGroupRequest{Name: displayName, Description: g.Description, OUID: g.OUID}
	_, svcErr = s.groupService.UpdateGroup(ctx, groupID, req)
	if svcErr != nil {
		logger.Error(ctx, "SCIM ReplaceGroup: failed to update group",
			log.String("groupID", groupID), log.Any("error", svcErr))
		return nil, mapGroupServiceErrorToSCIM(svcErr)
	}

	// Full replace: remove all existing members, add new ones
	existingMembers, svcErr := s.fetchAllGroupMembers(ctx, groupID, false)
	if svcErr != nil {
		logger.Error(ctx, "SCIM ReplaceGroup: failed to fetch existing members",
			log.String("groupID", groupID), log.Any("error", svcErr))
		return nil, mapGroupServiceErrorToSCIM(svcErr)
	}
	if len(existingMembers) > 0 {
		if _, svcErr = s.groupService.RemoveGroupMembers(ctx, groupID, existingMembers); svcErr != nil {
			logger.Error(ctx, "SCIM ReplaceGroup: failed to remove existing members",
				log.String("groupID", groupID), log.Any("error", svcErr))
			return nil, mapGroupServiceErrorToSCIM(svcErr)
		}
	}
	if len(thunderIDMembers) > 0 {
		if _, svcErr = s.groupService.AddGroupMembers(ctx, groupID, thunderIDMembers); svcErr != nil {
			logger.Error(ctx, "SCIM ReplaceGroup: failed to add new members",
				log.String("groupID", groupID), log.Any("error", svcErr))
			return nil, mapGroupServiceErrorToSCIM(svcErr)
		}
	}
	return s.GetGroup(ctx, groupID, baseURL)
}

// PatchGroup applies a sequence of validated PATCH actions to a group (RFC 7644 §3.5.2).
// Actions are applied in order so a single request can, e.g., remove one member and
// add another. Mirrors ReplaceGroup's mutability check before applying any changes.
// KNOWN LIMITATION: actions are applied sequentially with no surrounding transaction.
// If an action fails partway through, prior actions in the same request remain
// committed, which is not strictly all-or-nothing per RFC 7644 §3.5.2. See
// ReplaceGroup for the same limitation in the underlying group service calls.
func (s *scimGroupsService) PatchGroup(ctx context.Context, groupID string, actions []SCIMGroupPatchAction,
	ifMatch, baseURL string) (*SCIMGroup, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, groupServiceLoggerComponentName))

	g, svcErr := s.groupService.GetGroup(ctx, groupID, false)
	if svcErr != nil {
		logger.Debug(ctx, "SCIM PatchGroup: group service error",
			log.String("groupID", groupID), log.Any("error", svcErr))
		return nil, mapGroupServiceErrorToSCIM(svcErr)
	}
	if g.IsReadOnly {
		logger.Debug(ctx, "SCIM PatchGroup: group is read-only", log.String("groupID", groupID))
		return nil, &ErrorMutabilityViolation
	}

	if trimmed := strings.TrimSpace(ifMatch); trimmed != "" {
		existingMembers, svcErr := s.fetchAllGroupMembers(ctx, groupID, true)
		if svcErr != nil {
			logger.Error(ctx, "SCIM PatchGroup: failed to fetch group members",
				log.String("groupID", groupID), log.Any("error", svcErr))
			return nil, mapGroupServiceErrorToSCIM(svcErr)
		}
		g.Members = existingMembers
		if svcErr := checkIfMatch(trimmed, generateVersion(groupVersionState(*g))); svcErr != nil {
			return nil, svcErr
		}
	}

	for _, action := range actions {
		var applyErr *tidcommon.ServiceError
		switch action.Target {
		case scimGroupPatchTargetDisplayName:
			applyErr = s.applyDisplayNamePatch(ctx, groupID, g.OUID, g.Description, action)
		case scimGroupPatchTargetMembers:
			applyErr = s.applyMembersPatch(ctx, groupID, action)
		}
		if applyErr != nil {
			return nil, applyErr
		}
	}

	return s.GetGroup(ctx, groupID, baseURL)
}

// DeleteGroup deletes a group by ID.
func (s *scimGroupsService) DeleteGroup(ctx context.Context, groupID string, ifMatch string,
) *tidcommon.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, groupServiceLoggerComponentName))

	g, svcErr := s.groupService.GetGroup(ctx, groupID, false)
	if svcErr != nil {
		logger.Debug(ctx, "SCIM DeleteGroup: group service error",
			log.String("groupID", groupID), log.Any("error", svcErr))
		return mapGroupServiceErrorToSCIM(svcErr)
	}
	if g.IsReadOnly {
		logger.Debug(ctx, "SCIM DeleteGroup: group is read-only", log.String("groupID", groupID))
		return &ErrorMutabilityViolation
	}
	if trimmed := strings.TrimSpace(ifMatch); trimmed != "" {
		existingMembers, svcErr := s.fetchAllGroupMembers(ctx, groupID, true)
		if svcErr != nil {
			logger.Error(ctx, "SCIM DeleteGroup: failed to fetch group members",
				log.String("groupID", groupID), log.Any("error", svcErr))
			return mapGroupServiceErrorToSCIM(svcErr)
		}
		g.Members = existingMembers
		if svcErr := checkIfMatch(trimmed, generateVersion(groupVersionState(*g))); svcErr != nil {
			return svcErr
		}
	}
	svcErr = s.groupService.DeleteGroup(ctx, groupID)
	if svcErr != nil {
		logger.Error(ctx, "SCIM DeleteGroup: failed to delete group",
			log.String("groupID", groupID), log.Any("error", svcErr))
	}
	return mapGroupServiceErrorToSCIM(svcErr)
}

// applyDisplayNamePatch updates the group's display name.
func (s *scimGroupsService) applyDisplayNamePatch(ctx context.Context, groupID, ouID, description string,
	action SCIMGroupPatchAction) *tidcommon.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, groupServiceLoggerComponentName))

	req := group.UpdateGroupRequest{Name: action.DisplayName, Description: description, OUID: ouID}
	if _, svcErr := s.groupService.UpdateGroup(ctx, groupID, req); svcErr != nil {
		logger.Error(ctx, "SCIM PatchGroup: failed to update displayName",
			log.String("groupID", groupID), log.Any("error", svcErr))
		return mapGroupServiceErrorToSCIM(svcErr)
	}
	return nil
}

// applyMembersPatch applies a member patch action (add, remove, or replace) to a group.
func (s *scimGroupsService) applyMembersPatch(ctx context.Context, groupID string,
	action SCIMGroupPatchAction) *tidcommon.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, groupServiceLoggerComponentName))

	switch action.Op {
	case scimPatchOpAdd:
		thunderIDMembers, svcErr := scimMembersToThunderID(action.Members)
		if svcErr != nil {
			logger.Debug(ctx, "SCIM PatchGroup: invalid member type on add",
				log.String("groupID", groupID), log.Any("error", svcErr))
			return svcErr
		}
		if len(thunderIDMembers) == 0 {
			return nil
		}
		if _, svcErr := s.groupService.AddGroupMembers(ctx, groupID, thunderIDMembers); svcErr != nil {
			logger.Error(ctx, "SCIM PatchGroup: failed to add members",
				log.String("groupID", groupID), log.Any("error", svcErr))
			return mapGroupServiceErrorToSCIM(svcErr)
		}
		return nil

	case scimPatchOpRemove:
		return s.applyMembersRemove(ctx, groupID, action.FilterValue)

	case scimPatchOpReplace:
		// KNOWN LIMITATION: remove-then-add below is non-transactional; see ReplaceGroup.
		thunderIDMembers, svcErr := scimMembersToThunderID(action.Members)
		if svcErr != nil {
			logger.Debug(ctx, "SCIM PatchGroup: invalid member type on replace",
				log.String("groupID", groupID), log.Any("error", svcErr))
			return svcErr
		}
		existing, svcErr := s.fetchAllGroupMembers(ctx, groupID, true)
		if svcErr != nil {
			logger.Error(ctx, "SCIM PatchGroup: failed to fetch existing members for replace",
				log.String("groupID", groupID), log.Any("error", svcErr))
			return mapGroupServiceErrorToSCIM(svcErr)
		}
		if len(existing) > 0 {
			if _, svcErr := s.groupService.RemoveGroupMembers(ctx, groupID, existing); svcErr != nil {
				logger.Error(ctx, "SCIM PatchGroup: failed to remove existing members for replace",
					log.String("groupID", groupID), log.Any("error", svcErr))
				return mapGroupServiceErrorToSCIM(svcErr)
			}
		}
		if len(thunderIDMembers) > 0 {
			if _, svcErr := s.groupService.AddGroupMembers(ctx, groupID, thunderIDMembers); svcErr != nil {
				logger.Error(ctx, "SCIM PatchGroup: failed to add new members for replace",
					log.String("groupID", groupID), log.Any("error", svcErr))
				return mapGroupServiceErrorToSCIM(svcErr)
			}
		}
		return nil
	}
	return nil
}

// applyMembersRemove implements RFC 7644 §3.5.2.2's remove rules: a filterValue removes
// the single matching member, an empty filterValue removes all members. A filterValue
// that matches no existing member is a no-op, not an error (the "no target" rule).
func (s *scimGroupsService) applyMembersRemove(ctx context.Context, groupID, filterValue string,
) *tidcommon.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, groupServiceLoggerComponentName))

	existing, svcErr := s.fetchAllGroupMembers(ctx, groupID, true)
	if svcErr != nil {
		logger.Error(ctx, "SCIM PatchGroup: failed to fetch existing members for remove",
			log.String("groupID", groupID), log.Any("error", svcErr))
		return mapGroupServiceErrorToSCIM(svcErr)
	}

	toRemove := existing
	if filterValue != "" {
		toRemove = nil
		for _, m := range existing {
			if m.ID == filterValue {
				toRemove = []group.Member{m}
				break
			}
		}
	}
	if len(toRemove) == 0 {
		return nil
	}
	if _, svcErr := s.groupService.RemoveGroupMembers(ctx, groupID, toRemove); svcErr != nil {
		logger.Error(ctx, "SCIM PatchGroup: failed to remove members",
			log.String("groupID", groupID), log.Any("error", svcErr))
		return mapGroupServiceErrorToSCIM(svcErr)
	}
	return nil
}

// fetchAllGroupMembers retrieves all members of a group, paging through GetGroupMembers
// since it rejects any limit above serverconst.MaxPageSize.
// GetGroup does not populate Members for database-backed groups (only file-based/declarative
// groups carry Members through), so callers that need the full member list must fetch it
// separately via GetGroupMembers.
func (s *scimGroupsService) fetchAllGroupMembers(ctx context.Context, groupID string, includeDisplay bool,
) ([]group.Member, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, groupServiceLoggerComponentName))

	var members []group.Member
	offset := 0
	for {
		resp, svcErr := s.groupService.GetGroupMembers(ctx, groupID, serverconst.MaxPageSize, offset, includeDisplay)
		if svcErr != nil {
			logger.Error(ctx, "SCIM: failed to list group members",
				log.String("groupID", groupID), log.Int("offset", offset), log.Any("error", svcErr))
			return nil, svcErr
		}
		members = append(members, resp.Members...)
		if len(resp.Members) == 0 || len(members) >= resp.TotalResults {
			break
		}
		offset += len(resp.Members)
	}
	return members, nil
}

// scimMembersToThunderID converts SCIM members to a ThunderID Member slice.
// SCIM "User" → group.MemberTypeUser, "Group" → group.MemberTypeGroup (case-insensitive);
// an omitted type defaults to User per RFC 7643 §4.2 (type is OPTIONAL); any other
// non-empty type value is rejected rather than silently defaulting to User.
func scimMembersToThunderID(members []SCIMGroupMember) ([]group.Member, *tidcommon.ServiceError) {
	result := make([]group.Member, 0, len(members))
	for _, m := range members {
		var mt group.MemberType
		switch {
		case m.Type == "", strings.EqualFold(m.Type, "User"):
			mt = group.MemberTypeUser
		case strings.EqualFold(m.Type, "Group"):
			mt = group.MemberTypeGroup
		default:
			return nil, &ErrorUnsupportedMemberType
		}
		result = append(result, group.Member{ID: m.Value, Type: mt, Display: m.Display})
	}
	return result, nil
}

// mapGroupServiceErrorToSCIM maps internal group service errors to SCIM package errors.
func mapGroupServiceErrorToSCIM(svcErr *tidcommon.ServiceError) *tidcommon.ServiceError {
	if svcErr == nil {
		return nil
	}
	switch svcErr.Code {
	case group.ErrorGroupNotFound.Code:
		return &ErrorResourceNotFound
	case group.ErrorImmutableGroup.Code:
		return &ErrorMutabilityViolation
	case group.ErrorInvalidMemberID.Code, group.ErrorInvalidGroupMemberID.Code:
		return &ErrorInvalidGroupMember
	case group.ErrorGroupNameConflict.Code:
		return &ErrorUniquenessConflict
	case group.ErrorInvalidMemberType.Code:
		return &ErrorUnsupportedMemberType
	case tidcommon.ErrorUnauthorized.Code:
		return svcErr
	default:
		if svcErr.Type == tidcommon.ServerErrorType {
			return &tidcommon.InternalServerError
		}
		return &ErrorInvalidRequestBody
	}
}
