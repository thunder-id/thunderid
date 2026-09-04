// Copyright 2025-2026 The ThunderID Authors
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

// parseAndValidateSCIMGroupWriteRequest parses, extracts, and validates a SCIM Group POST/PUT request body.
// It ensures the core Group schema URN is declared and that displayName is non-empty.
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

// parseAndValidateSCIMGroupPatchRequest parses, extracts, and validates a SCIM Group PATCH request body,
// returning a normalized list of actions ready to apply (RFC 7644 §3.5.2).
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
		action, svcErr := validateSCIMGroupPatchOp(op)
		if svcErr != nil {
			return nil, svcErr
		}
		actions = append(actions, action)
	}
	return actions, nil
}

// validateSCIMGroupPatchOp validates a single SCIM PATCH operation and returns a
// normalized SCIMGroupPatchAction.
func validateSCIMGroupPatchOp(op scimGroupPatchOp) (SCIMGroupPatchAction, *tidcommon.ServiceError) {
	normalizedOp := strings.ToLower(strings.TrimSpace(op.Op))
	if normalizedOp != scimPatchOpAdd && normalizedOp != scimPatchOpRemove && normalizedOp != scimPatchOpReplace {
		return SCIMGroupPatchAction{}, &scim.ErrorInvalidPatchOp
	}

	path := strings.TrimSpace(op.Path)
	switch {
	case strings.EqualFold(path, "displayName"):
		return validateDisplayNamePatchOp(normalizedOp, op.Value)
	case strings.EqualFold(path, "members"):
		return validateMembersPatchOp(normalizedOp, op.Value, "")
	case strings.HasPrefix(strings.ToLower(path), "members["):
		filterValue, svcErr := parseMembersFilterPath(path)
		if svcErr != nil {
			return SCIMGroupPatchAction{}, svcErr
		}
		return validateMembersPatchOp(normalizedOp, op.Value, filterValue)
	default:
		return SCIMGroupPatchAction{}, &scim.ErrorInvalidPatchPath
	}
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

// validateMembersPatchOp validates a PATCH operation targeting the members attribute,
// with an optional filter value extracted from a path like members[value eq "<id>"].
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

// parseMembersFilterPath extracts the member id from a path of the form
// members[value eq "<id>"]. Only this exact filter attribute/operator is supported.
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
