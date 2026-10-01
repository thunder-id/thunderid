// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package groups

import (
	"encoding/json"
	"strings"

	scim "github.com/thunder-id/thunderid/internal/scim/common"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// ---------------------------------------------------------------------------
// Groups — POST / PUT validation
// ---------------------------------------------------------------------------

// parseAndValidateSCIMGroupWriteRequest parses and validates a SCIM Group POST/PUT request body.
func parseAndValidateSCIMGroupWriteRequest(body []byte) (*scimGroupPayload, *tidcommon.ServiceError) {
	var raw struct {
		Schemas     []string          `json:"schemas"`
		DisplayName string            `json:"displayName"`
		Members     []SCIMGroupMember `json:"members"`
	}
	if err := json.Unmarshal(body, &raw); err != nil || raw.DisplayName == "" {
		return nil, &scim.ErrorInvalidRequestBody
	}
	if !scim.HasSchemaURN(raw.Schemas, scim.SCIMCoreGroupSchemaURN) {
		return nil, &scim.ErrorMissingCoreGroupSchema
	}
	return &scimGroupPayload{
		DisplayName: raw.DisplayName,
		Members:     raw.Members,
	}, nil
}

// ---------------------------------------------------------------------------
// Groups — PATCH validation
// ---------------------------------------------------------------------------

// parseAndValidateSCIMGroupPatchRequest parses and validates a SCIM Group PATCH request body.
func parseAndValidateSCIMGroupPatchRequest(body []byte) ([]SCIMGroupPatchAction, *tidcommon.ServiceError) {
	var req scimGroupPatchRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, &scim.ErrorInvalidRequestBody
	}

	if !scim.HasSchemaURN(req.Schemas, scim.SCIMPatchOpSchemaURN) {
		return nil, &scim.ErrorMissingSchemas
	}
	actions := make([]SCIMGroupPatchAction, 0, len(req.Operations))
	for _, op := range req.Operations {
		opActions, svcErr := validateSCIMGroupPatchOp(op)
		if svcErr != nil {
			return nil, svcErr
		}
		actions = append(actions, opActions...)
	}
	return actions, nil
}

// validateSCIMGroupPatchOp validates a single SCIM PATCH operation. A pathless add/replace
// (RFC 7644 §3.5.2.1, §3.5.2.3) can carry several attributes, so it may yield more than one action.
func validateSCIMGroupPatchOp(op scimGroupPatchOp) ([]SCIMGroupPatchAction, *tidcommon.ServiceError) {
	normalizedOp := strings.ToLower(strings.TrimSpace(op.Op))
	if normalizedOp != scimPatchOpAdd && normalizedOp != scimPatchOpRemove && normalizedOp != scimPatchOpReplace {
		return nil, &scim.ErrorInvalidPatchOp
	}

	path := strings.TrimSpace(op.Path)
	if path == "" {
		return validatePathlessPatchOp(normalizedOp, op.Value)
	}

	var action SCIMGroupPatchAction
	var svcErr *tidcommon.ServiceError
	switch {
	case strings.EqualFold(path, "displayName"):
		action, svcErr = validateDisplayNamePatchOp(normalizedOp, op.Value)
	case strings.EqualFold(path, "members"):
		action, svcErr = validateMembersPatchOp(normalizedOp, op.Value, "")
	case strings.HasPrefix(strings.ToLower(path), "members["):
		var filterValue string
		if filterValue, svcErr = parseMembersFilterPath(path); svcErr == nil {
			action, svcErr = validateMembersPatchOp(normalizedOp, op.Value, filterValue)
		}
	default:
		svcErr = &scim.ErrorInvalidPatchPath
	}
	if svcErr != nil {
		return nil, svcErr
	}
	return []SCIMGroupPatchAction{action}, nil
}

// validatePathlessPatchOp validates a PATCH operation with no "path". A remove has no target
// (RFC 7644 §3.5.2.2); an add/replace carries an object of attributes in "value".
func validatePathlessPatchOp(op string, raw json.RawMessage) ([]SCIMGroupPatchAction, *tidcommon.ServiceError) {
	if op == scimPatchOpRemove {
		return nil, &scim.ErrorNoTarget
	}

	var attrs map[string]json.RawMessage
	if err := json.Unmarshal(raw, &attrs); err != nil || len(attrs) == 0 {
		return nil, &scim.ErrorInvalidPatchValue
	}

	var displayNameRaw, membersRaw json.RawMessage
	for name, value := range attrs {
		switch strings.ToLower(name) {
		case "displayname":
			displayNameRaw = value
		case "members":
			membersRaw = value
		default:
			return nil, &scim.ErrorInvalidPatchPath
		}
	}

	actions := make([]SCIMGroupPatchAction, 0, len(attrs))
	if displayNameRaw != nil {
		action, svcErr := validateDisplayNamePatchOp(op, displayNameRaw)
		if svcErr != nil {
			return nil, svcErr
		}
		actions = append(actions, action)
	}
	if membersRaw != nil {
		action, svcErr := validateMembersPatchOp(op, membersRaw, "")
		if svcErr != nil {
			return nil, svcErr
		}
		actions = append(actions, action)
	}
	return actions, nil
}

// validateDisplayNamePatchOp validates a PATCH operation targeting the displayName attribute.
func validateDisplayNamePatchOp(op string, raw json.RawMessage) (SCIMGroupPatchAction, *tidcommon.ServiceError) {
	if op == scimPatchOpRemove {
		// displayName is REQUIRED (RFC 7643 §4.2); removing it is not permitted.
		return SCIMGroupPatchAction{}, &scim.ErrorInvalidPatchPath
	}
	var displayName string
	if err := json.Unmarshal(raw, &displayName); err != nil || strings.TrimSpace(displayName) == "" {
		return SCIMGroupPatchAction{}, &scim.ErrorInvalidPatchValue
	}
	return SCIMGroupPatchAction{Op: op, Target: scimGroupPatchTargetDisplayName, DisplayName: displayName}, nil
}

// validateMembersPatchOp validates a PATCH operation targeting the members attribute.
func validateMembersPatchOp(op string, raw json.RawMessage, filterValue string,
) (SCIMGroupPatchAction, *tidcommon.ServiceError) {
	switch {
	case op == scimPatchOpRemove && filterValue != "":
		// Remove one member selected by filter; no value expected.
		if len(raw) > 0 {
			return SCIMGroupPatchAction{}, &scim.ErrorInvalidPatchValue
		}
		return SCIMGroupPatchAction{Op: op, Target: scimGroupPatchTargetMembers, FilterValue: filterValue}, nil

	case op == scimPatchOpRemove && filterValue == "":
		// Remove the entire members attribute (RFC 7644 §3.5.2.2); no value expected.
		if len(raw) > 0 {
			return SCIMGroupPatchAction{}, &scim.ErrorInvalidPatchValue
		}
		return SCIMGroupPatchAction{Op: op, Target: scimGroupPatchTargetMembers}, nil

	case filterValue != "":
		// add/replace do not support a filtered path.
		return SCIMGroupPatchAction{}, &scim.ErrorInvalidPatchPath

	default:
		var members []SCIMGroupMember
		if err := json.Unmarshal(raw, &members); err != nil {
			return SCIMGroupPatchAction{}, &scim.ErrorInvalidPatchValue
		}
		if op == scimPatchOpAdd && len(members) == 0 {
			return SCIMGroupPatchAction{}, &scim.ErrorInvalidPatchValue
		}
		return SCIMGroupPatchAction{Op: op, Target: scimGroupPatchTargetMembers, Members: members}, nil
	}
}

// parseMembersFilterPath extracts the member ID from a filter path.
func parseMembersFilterPath(path string) (string, *tidcommon.ServiceError) {
	path = strings.TrimSpace(path)
	const prefix = "members["
	if len(path) < len(prefix) || !strings.EqualFold(path[:len(prefix)], prefix) || !strings.HasSuffix(path, "]") {
		return "", &scim.ErrorInvalidPatchPath
	}
	inner := strings.TrimSuffix(path[len(prefix):], "]")

	fields := strings.Fields(inner)
	if len(fields) != 3 || !strings.EqualFold(fields[0], "value") || !strings.EqualFold(fields[1], "eq") {
		return "", &scim.ErrorInvalidPatchPath
	}
	value := strings.Trim(fields[2], `"`)
	if value == "" {
		return "", &scim.ErrorInvalidPatchPath
	}
	return value, nil
}
