package scim

import (
	"encoding/json"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// SCIMUserPayload is the parsed, validated result of a SCIM User POST/PUT request body.
type SCIMUserPayload struct {
	// ExtensionURN is the full ThunderID extension URN exactly as sent by the client.
	ExtensionURN string
	// UserTypeName is the entity type name extracted from the extension URN (e.g. "employee").
	UserTypeName string
	// CoreAttrs holds top-level request fields that are NOT "schemas" and NOT the extension URN object.
	CoreAttrs map[string]json.RawMessage
	// ExtensionAttrs holds the key/value pairs from inside the extension URN object.
	ExtensionAttrs map[string]json.RawMessage
}

// ValidateSCIMUserRequest parses and validates a SCIM user payload.
func ValidateSCIMUserRequest(body []byte) (*SCIMUserPayload, *tidcommon.ServiceError) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, &ErrorInvalidRequestBody
	}

	schemas, svcErr := parseSCIMSchemas(raw)
	if svcErr != nil {
		return nil, svcErr
	}

	extensionURN, userTypeName, svcErr := resolveThunderExtensionURN(raw, schemas)
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

// resolveThunderExtensionURN finds the single ThunderID extension URN declared in
// "schemas", if any. At most one is allowed; zero is allowed only when the payload
// carries SCIM core attributes only (no custom type declared), in which case the
// service layer falls back to the sole configured user type. When zero URNs are
// declared, any body key that is itself a ThunderID extension URN is rejected,
// since SCIM requires extension objects to be declared in "schemas".
func resolveThunderExtensionURN(
	raw map[string]json.RawMessage, schemas []string,
) (extensionURN, userTypeName string, svcErr *tidcommon.ServiceError) {
	thunderPrefix := strings.ToLower(ThunderIDURNPrefix)
	var thunderURNs []string
	for _, urn := range schemas {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(urn)), thunderPrefix) {
			thunderURNs = append(thunderURNs, urn)
		}
	}
	if len(thunderURNs) > 1 {
		return "", "", &ErrorMultipleCustomSchemas
	}
	if len(thunderURNs) == 0 {
		for k := range raw {
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(k)), thunderPrefix) {
				return "", "", &ErrorUndeclaredCustomSchemaObject
			}
		}
		return "", "", nil
	}

	extensionURN = thunderURNs[0]
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
