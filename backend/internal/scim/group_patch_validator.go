package scim

import (
	"encoding/json"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// scimGroupPatchTarget identifies which attribute a validated PATCH operation targets.
type scimGroupPatchTarget int

const (
	scimGroupPatchTargetMembers scimGroupPatchTarget = iota
	scimGroupPatchTargetDisplayName
)

// SCIM PATCH operation values (RFC 7644 §3.5.2).
const (
	scimPatchOpAdd     = "add"
	scimPatchOpRemove  = "remove"
	scimPatchOpReplace = "replace"
)

// SCIMGroupPatchAction is a single normalized, validated PATCH operation ready to apply.
type SCIMGroupPatchAction struct {
	Op          string // "add", "remove", "replace"
	Target      scimGroupPatchTarget
	Members     []SCIMGroupMember // set when Target == scimGroupPatchTargetMembers
	FilterValue string            // set when path is members[value eq "<id>"]; empty otherwise
	DisplayName string            // set when Target == scimGroupPatchTargetDisplayName
}

// ValidateSCIMGroupPatchRequest parses and validates a SCIM Group PATCH request body,
// returning a normalized list of actions ready to apply (RFC 7644 §3.5.2).
func ValidateSCIMGroupPatchRequest(body []byte) ([]SCIMGroupPatchAction, *tidcommon.ServiceError) {
	var req SCIMGroupPatchRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, &ErrorInvalidRequestBody
	}

	hasPatchOpSchema := false
	for _, urn := range req.Schemas {
		if strings.EqualFold(strings.TrimSpace(urn), SCIMPatchOpSchemaURN) {
			hasPatchOpSchema = true
			break
		}
	}
	if !hasPatchOpSchema {
		return nil, &ErrorMissingSchemas
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

func validateSCIMGroupPatchOp(op SCIMGroupPatchOp) (SCIMGroupPatchAction, *tidcommon.ServiceError) {
	normalizedOp := strings.ToLower(strings.TrimSpace(op.Op))
	if normalizedOp != scimPatchOpAdd && normalizedOp != scimPatchOpRemove && normalizedOp != scimPatchOpReplace {
		return SCIMGroupPatchAction{}, &ErrorInvalidPatchOp
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
		return SCIMGroupPatchAction{}, &ErrorInvalidPatchPath
	}
}

func validateDisplayNamePatchOp(op string, raw json.RawMessage) (SCIMGroupPatchAction, *tidcommon.ServiceError) {
	if op == scimPatchOpRemove {
		// displayName is REQUIRED (RFC 7643 §4.2); removing it is not permitted.
		return SCIMGroupPatchAction{}, &ErrorInvalidPatchPath
	}
	var displayName string
	if err := json.Unmarshal(raw, &displayName); err != nil || strings.TrimSpace(displayName) == "" {
		return SCIMGroupPatchAction{}, &ErrorInvalidPatchValue
	}
	return SCIMGroupPatchAction{Op: op, Target: scimGroupPatchTargetDisplayName, DisplayName: displayName}, nil
}

func validateMembersPatchOp(op string, raw json.RawMessage, filterValue string,
) (SCIMGroupPatchAction, *tidcommon.ServiceError) {
	switch {
	case op == scimPatchOpRemove && filterValue != "":
		// Remove one member selected by filter; no value expected.
		if len(raw) > 0 {
			return SCIMGroupPatchAction{}, &ErrorInvalidPatchValue
		}
		return SCIMGroupPatchAction{Op: op, Target: scimGroupPatchTargetMembers, FilterValue: filterValue}, nil

	case op == scimPatchOpRemove && filterValue == "":
		// Remove the entire members attribute (RFC 7644 §3.5.2.2); no value expected.
		if len(raw) > 0 {
			return SCIMGroupPatchAction{}, &ErrorInvalidPatchValue
		}
		return SCIMGroupPatchAction{Op: op, Target: scimGroupPatchTargetMembers}, nil

	case filterValue != "":
		// add/replace do not support a filtered path.
		return SCIMGroupPatchAction{}, &ErrorInvalidPatchPath

	default:
		var members []SCIMGroupMember
		if err := json.Unmarshal(raw, &members); err != nil {
			return SCIMGroupPatchAction{}, &ErrorInvalidPatchValue
		}
		if op == scimPatchOpAdd && len(members) == 0 {
			return SCIMGroupPatchAction{}, &ErrorInvalidPatchValue
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
		return "", &ErrorInvalidPatchPath
	}
	inner := strings.TrimSuffix(path[len(prefix):], "]")

	fields := strings.Fields(inner)
	if len(fields) != 3 || !strings.EqualFold(fields[0], "value") || !strings.EqualFold(fields[1], "eq") {
		return "", &ErrorInvalidPatchPath
	}
	value := strings.Trim(fields[2], `"`)
	if value == "" {
		return "", &ErrorInvalidPatchPath
	}
	return value, nil
}
