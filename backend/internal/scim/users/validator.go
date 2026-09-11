// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package users

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	scim "github.com/thunder-id/thunderid/internal/scim/common"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// SCIMUserPayload is the parsed, validated result of a SCIM User POST/PUT request body.
type SCIMUserPayload struct {
	// ExtensionURN is the full ThunderID extension URN exactly as sent by the client.
	ExtensionURN string
	// UserTypeName is the user type name extracted from the extension URN (e.g. "employee").
	UserTypeName string
	// CoreAttrs holds top-level request fields that are NOT "schemas", NOT the extension URN object,
	// and NOT the Enterprise schema URN object.
	CoreAttrs map[string]json.RawMessage
	// ExtensionAttrs holds the key/value pairs from inside the extension URN object.
	ExtensionAttrs map[string]json.RawMessage
	// EnterpriseAttrs holds the key/value pairs from inside the Enterprise extension URN object.
	EnterpriseAttrs map[string]json.RawMessage
	// HasEnterpriseSchema is true if SCIMEnterpriseUserSchemaURN was declared in "schemas".
	HasEnterpriseSchema bool
}

// parseAndValidateSCIMUserRequest parses, extracts, and validates a SCIM user payload.
func parseAndValidateSCIMUserRequest(body []byte) (*SCIMUserPayload, *tidcommon.ServiceError) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, &scim.ErrorInvalidRequestBody
	}

	schemas, svcErr := parseSCIMSchemas(raw)
	if svcErr != nil {
		return nil, svcErr
	}

	hasEnterpriseSchema := scim.HasSchemaURN(schemas, scim.SCIMEnterpriseUserSchemaURN)
	enterpriseAttrs, svcErr := extractEnterpriseAttrs(raw, hasEnterpriseSchema)
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
	if extensionURN == "" && len(coreAttrs) == 0 && len(enterpriseAttrs) == 0 {
		return nil, &scim.ErrorMissingCustomSchema
	}

	if svcErr := validateCoreUserSchemaDeclared(schemas, coreAttrs); svcErr != nil {
		return nil, svcErr
	}

	return &SCIMUserPayload{
		ExtensionURN:        extensionURN,
		UserTypeName:        userTypeName,
		CoreAttrs:           coreAttrs,
		ExtensionAttrs:      extAttrs,
		EnterpriseAttrs:     enterpriseAttrs,
		HasEnterpriseSchema: hasEnterpriseSchema,
	}, nil
}

// parseSCIMSchemas validates the presence of the "schemas" array, unmarshals it,
// and rejects duplicate URNs within it.
func parseSCIMSchemas(raw map[string]json.RawMessage) ([]string, *tidcommon.ServiceError) {
	schemasRaw, ok := raw["schemas"]
	if !ok {
		return nil, &scim.ErrorMissingSchemas
	}
	var schemas []string
	if err := json.Unmarshal(schemasRaw, &schemas); err != nil || len(schemas) == 0 {
		return nil, &scim.ErrorMissingSchemas
	}

	seen := make(map[string]struct{}, len(schemas))
	for _, urn := range schemas {
		lower := strings.ToLower(strings.TrimSpace(urn))
		if _, exists := seen[lower]; exists {
			return nil, &scim.ErrorDuplicateSchemas
		}
		seen[lower] = struct{}{}
	}
	return schemas, nil
}

// resolveThunderIDExtensionURN finds the single ThunderID extension URN declared in schemas, if any.
func resolveThunderIDExtensionURN(
	raw map[string]json.RawMessage, schemas []string,
) (extensionURN, userTypeName string, svcErr *tidcommon.ServiceError) {
	thunderIDPrefix := strings.ToLower(scim.ThunderIDURNPrefix)
	var thunderIDURNs []string
	for _, urn := range schemas {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(urn)), thunderIDPrefix) {
			thunderIDURNs = append(thunderIDURNs, urn)
		}
	}
	if len(thunderIDURNs) > 1 {
		return "", "", &scim.ErrorMultipleCustomSchemas
	}
	if len(thunderIDURNs) == 0 {
		for k := range raw {
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(k)), thunderIDPrefix) {
				return "", "", &scim.ErrorUndeclaredCustomSchemaObject
			}
		}
		return "", "", nil
	}

	extensionURN = thunderIDURNs[0]
	userTypeName, ok := scim.ParseUserTypeFromSchemaURN(extensionURN)
	if !ok || strings.TrimSpace(userTypeName) == "" {
		return "", "", &scim.ErrorInvalidCustomSchemaURN
	}
	return extensionURN, userTypeName, nil
}

// extractExtensionAttrs pulls the extension object keyed by extensionURN from the request body.
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
		return nil, &scim.ErrorMissingCustomSchemaObject
	}
	return extAttrs, nil
}

// extractEnterpriseAttrs pulls the enterprise user extension object from the request body.
func extractEnterpriseAttrs(
	raw map[string]json.RawMessage, hasEnterpriseSchema bool,
) (map[string]json.RawMessage, *tidcommon.ServiceError) {
	if !hasEnterpriseSchema {
		for k := range raw {
			if strings.EqualFold(strings.TrimSpace(k), scim.SCIMEnterpriseUserSchemaURN) {
				return nil, &scim.ErrorUndeclaredEnterpriseSchemaObject
			}
		}
		return nil, nil
	}

	var entRaw json.RawMessage
	for k, v := range raw {
		if strings.EqualFold(strings.TrimSpace(k), scim.SCIMEnterpriseUserSchemaURN) {
			entRaw = v
			break
		}
	}
	if entRaw == nil {
		return make(map[string]json.RawMessage), nil
	}
	entAttrs := make(map[string]json.RawMessage)
	if err := json.Unmarshal(entRaw, &entAttrs); err != nil || entAttrs == nil {
		return nil, &scim.ErrorInvalidRequestBody
	}
	return entAttrs, nil
}

// extractCoreAttrs collects top-level request fields that are not schemas, the extension URN, or the enterprise URN.
func extractCoreAttrs(raw map[string]json.RawMessage, extensionURN string) map[string]json.RawMessage {
	coreAttrs := make(map[string]json.RawMessage)
	for k, v := range raw {
		if strings.EqualFold(k, "schemas") {
			continue
		}
		if extensionURN != "" && strings.EqualFold(k, extensionURN) {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(k), scim.SCIMEnterpriseUserSchemaURN) {
			continue
		}
		coreAttrs[k] = v
	}
	return coreAttrs
}

// validateCoreUserSchemaDeclared ensures the core User schema URN is declared in schemas when core attributes are present.
func validateCoreUserSchemaDeclared(schemas []string, coreAttrs map[string]json.RawMessage) *tidcommon.ServiceError {
	if len(coreAttrs) == 0 {
		return nil
	}
	for _, urn := range schemas {
		if strings.EqualFold(urn, scim.SCIMCoreUserSchemaURN) {
			return nil
		}
	}
	return &scim.ErrorMissingCoreUserSchema
}

// parseSchemaRawProps unmarshals a user-type JSON schema into a property definition map.
func parseSchemaRawProps(schema json.RawMessage) (rawProps map[string]scim.RawPropertyDef, hasSchema bool, err error) {
	if len(schema) == 0 {
		return nil, false, nil
	}
	if err := json.Unmarshal(schema, &rawProps); err != nil {
		return nil, true, fmt.Errorf("parseSchemaRawProps: failed to parse schema JSON: %w", err)
	}
	return rawProps, true, nil
}

// missingRequiredAttrs returns the names of required schema properties absent from attrs.
func missingRequiredAttrs(
	attrs map[string]json.RawMessage, schema json.RawMessage, skipCredential bool,
) ([]string, error) {
	rawProps, hasSchema, err := parseSchemaRawProps(schema)
	if err != nil || !hasSchema {
		return nil, err
	}

	var missing []string
	for name, def := range rawProps {
		if !def.Required {
			continue
		}
		if skipCredential && def.Credential {
			continue
		}
		if !hasAttrCaseInsensitive(attrs, name) {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing, nil
}

// undeclaredAttrs returns the names of extension attributes not declared in the user-type schema.
func undeclaredAttrs(extensionAttrs map[string]json.RawMessage, schema json.RawMessage) ([]string, error) {
	rawProps, hasSchema, err := parseSchemaRawProps(schema)
	if err != nil || !hasSchema {
		return nil, err
	}

	var undeclared []string
	for name := range extensionAttrs {
		if !hasPropCaseInsensitive(rawProps, name) {
			undeclared = append(undeclared, name)
		}
	}

	sort.Strings(undeclared)
	return undeclared, nil
}

// hasAttrCaseInsensitive reports whether a SCIM schema attribute with the given name exists in attrs,
// case-insensitively.
func hasAttrCaseInsensitive(attrs map[string]json.RawMessage, target string) bool {
	for k := range attrs {
		if strings.EqualFold(k, target) {
			return true
		}
	}
	return false
}

// hasPropCaseInsensitive reports whether a property with the given name exists in rawProps, case-insensitively.
func hasPropCaseInsensitive(rawProps map[string]scim.RawPropertyDef, target string) bool {
	for propName := range rawProps {
		if strings.EqualFold(propName, target) {
			return true
		}
	}
	return false
}
