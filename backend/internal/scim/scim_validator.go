// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package scim

import (
	"encoding/json"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// SCIMUserPayload is the parsed, validated result of a SCIM User POST/PUT request body.
type SCIMUserPayload struct {
	// ExtensionURN is the full ThunderID extension URN exactly as sent by the client.
	ExtensionURN string
	// UserTypeName is the user type name extracted from the extension URN (e.g. "employee").
	UserTypeName string
	// CoreAttrs holds top-level request fields that are NOT "schemas" and NOT the extension URN object.
	CoreAttrs map[string]json.RawMessage
	// ExtensionAttrs holds the key/value pairs from inside the extension URN object.
	ExtensionAttrs map[string]json.RawMessage
}

// parseAndValidateSCIMUserRequest parses, extracts, and validates a SCIM user payload.
func parseAndValidateSCIMUserRequest(body []byte) (*SCIMUserPayload, *tidcommon.ServiceError) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, &ErrorInvalidRequestBody
	}

	schemas, svcErr := parseSCIMSchemas(raw)
	if svcErr != nil {
		return nil, svcErr
	}

	extensionURN, userTypeName, svcErr := resolveThunderIDExtensionURN(raw, schemas)
	if svcErr != nil {
		return nil, svcErr
	}

	extAttrs, svcErr := extractExtensionAttrs(raw, extensionURN)
	if svcErr != nil {
		return nil, svcErr
	}

	coreAttrs := extractCoreAttrs(raw, extensionURN)
	if extensionURN == "" && len(coreAttrs) == 0 {
		return nil, &ErrorMissingCustomSchema
	}

	if svcErr := validateCoreUserSchemaDeclared(schemas, coreAttrs); svcErr != nil {
		return nil, svcErr
	}

	return &SCIMUserPayload{
		ExtensionURN:   extensionURN,
		UserTypeName:   userTypeName,
		CoreAttrs:      coreAttrs,
		ExtensionAttrs: extAttrs,
	}, nil
}

// parseSCIMSchemas validates the presence of the "schemas" array, unmarshals it,
// and rejects duplicate URNs within it.
func parseSCIMSchemas(raw map[string]json.RawMessage) ([]string, *tidcommon.ServiceError) {
	schemasRaw, ok := raw["schemas"]
	if !ok {
		return nil, &ErrorMissingSchemas
	}
	var schemas []string
	if err := json.Unmarshal(schemasRaw, &schemas); err != nil || len(schemas) == 0 {
		return nil, &ErrorMissingSchemas
	}

	seen := make(map[string]struct{}, len(schemas))
	for _, urn := range schemas {
		lower := strings.ToLower(strings.TrimSpace(urn))
		if _, exists := seen[lower]; exists {
			return nil, &ErrorDuplicateSchemas
		}
		seen[lower] = struct{}{}
	}
	return schemas, nil
}

// resolveThunderIDExtensionURN finds the single ThunderID extension URN declared in
// "schemas", if any. At most one is allowed; zero is allowed only when the payload
// carries SCIM core attributes only (no custom type declared), in which case the
// service layer falls back to the sole configured user type. When zero URNs are
// declared, any body key that is itself a ThunderID extension URN is rejected,
// since SCIM requires extension objects to be declared in "schemas".
func resolveThunderIDExtensionURN(
	raw map[string]json.RawMessage, schemas []string,
) (extensionURN, userTypeName string, svcErr *tidcommon.ServiceError) {
	thunderIDPrefix := strings.ToLower(ThunderIDURNPrefix)
	var thunderIDURNs []string
	for _, urn := range schemas {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(urn)), thunderIDPrefix) {
			thunderIDURNs = append(thunderIDURNs, urn)
		}
	}
	if len(thunderIDURNs) > 1 {
		return "", "", &ErrorMultipleCustomSchemas
	}
	if len(thunderIDURNs) == 0 {
		for k := range raw {
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(k)), thunderIDPrefix) {
				return "", "", &ErrorUndeclaredCustomSchemaObject
			}
		}
		return "", "", nil
	}

	extensionURN = thunderIDURNs[0]
	userTypeName, ok := parseUserTypeFromSchemaURN(extensionURN)
	if !ok || strings.TrimSpace(userTypeName) == "" {
		return "", "", &ErrorInvalidCustomSchemaURN
	}
	return extensionURN, userTypeName, nil
}

// extractExtensionAttrs pulls the extension object keyed by extensionURN out of the
// request body. The extension object is optional if no extension attributes are
// provided; if present, it must be a valid JSON object.
func extractExtensionAttrs(
	raw map[string]json.RawMessage, extensionURN string,
) (map[string]json.RawMessage, *tidcommon.ServiceError) {
	extAttrs := make(map[string]json.RawMessage)
	if extensionURN == "" {
		return extAttrs, nil
	}

	var extRaw json.RawMessage
	for k, v := range raw {
		if strings.EqualFold(k, extensionURN) {
			extRaw = v
			break
		}
	}
	if extRaw == nil {
		return extAttrs, nil
	}
	if err := json.Unmarshal(extRaw, &extAttrs); err != nil || extAttrs == nil {
		return nil, &ErrorMissingCustomSchemaObject
	}
	return extAttrs, nil
}

// extractCoreAttrs collects the top-level request fields that are not "schemas"
// and not the extension URN object.
func extractCoreAttrs(raw map[string]json.RawMessage, extensionURN string) map[string]json.RawMessage {
	coreAttrs := make(map[string]json.RawMessage)
	for k, v := range raw {
		if strings.EqualFold(k, "schemas") {
			continue
		}
		if extensionURN != "" && strings.EqualFold(k, extensionURN) {
			continue
		}
		coreAttrs[k] = v
	}
	return coreAttrs
}

// validateCoreUserSchemaDeclared ensures that, when the request carries core
// attributes, the SCIM Core User schema URN is declared in "schemas". A
// custom-type-only payload (no core attributes at all) does not need to declare it.
func validateCoreUserSchemaDeclared(schemas []string, coreAttrs map[string]json.RawMessage) *tidcommon.ServiceError {
	if len(coreAttrs) == 0 {
		return nil
	}
	for _, urn := range schemas {
		if strings.EqualFold(urn, SCIMCoreUserSchemaURN) {
			return nil
		}
	}
	return &ErrorMissingCoreUserSchema
}

// ---------------------------------------------------------------------------
// Groups — POST / PUT validation
// ---------------------------------------------------------------------------

// parseAndValidateSCIMGroupWriteRequest parses, extracts, and validates a SCIM Group POST/PUT request body.
// It ensures the core Group schema URN is declared and that displayName is non-empty.
func parseAndValidateSCIMGroupWriteRequest(body []byte) (*SCIMGroupPayload, *tidcommon.ServiceError) {
	var raw struct {
		Schemas     []string          `json:"schemas"`
		DisplayName string            `json:"displayName"`
		Members     []SCIMGroupMember `json:"members"`
	}
	if err := json.Unmarshal(body, &raw); err != nil || raw.DisplayName == "" {
		return nil, &ErrorInvalidRequestBody
	}
	if !hasSchemaURN(raw.Schemas, SCIMCoreGroupSchemaURN) {
		return nil, &ErrorMissingCoreGroupSchema
	}
	return &SCIMGroupPayload{
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
	var req SCIMGroupPatchRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, &ErrorInvalidRequestBody
	}

	if !hasSchemaURN(req.Schemas, SCIMPatchOpSchemaURN) {
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

// validateSCIMGroupPatchOp validates a single SCIM PATCH operation and returns a
// normalized SCIMGroupPatchAction.
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

// validateDisplayNamePatchOp validates a PATCH operation targeting the displayName attribute.
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

// validateMembersPatchOp validates a PATCH operation targeting the members attribute,
// with an optional filter value extracted from a path like members[value eq "<id>"].
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

// hasSchemaURN checks if a target schema URN exists in a list of schemas (case-insensitive).
func hasSchemaURN(schemas []string, targetURN string) bool {
	for _, urn := range schemas {
		if strings.EqualFold(strings.TrimSpace(urn), targetURN) {
			return true
		}
	}
	return false
}
