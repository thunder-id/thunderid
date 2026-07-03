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
	// Step 1: Parse the request body as JSON.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, &ErrorInvalidRequestBody
	}

	// Step 2: Validate the presence of the "schemas" array and extract URNs.
	schemasRaw, ok := raw["schemas"]
	if !ok {
		return nil, &ErrorMissingSchemas
	}
	var schemas []string
	if err := json.Unmarshal(schemasRaw, &schemas); err != nil || len(schemas) == 0 {
		return nil, &ErrorMissingSchemas
	}

	// Step 3: Check for duplicate URNs in the "schemas" array.
	seen := make(map[string]struct{}, len(schemas))
	for _, urn := range schemas {
		lower := strings.ToLower(strings.TrimSpace(urn))
		if _, exists := seen[lower]; exists {
			return nil, &ErrorDuplicateSchemas
		}
		seen[lower] = struct{}{}
	}

	// Step 4: exactly one ThunderID extension URN .
	thunderPrefix := strings.ToLower(ThunderIDURNPrefix)
	var thunderURNs []string
	for _, urn := range schemas {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(urn)), thunderPrefix) {
			thunderURNs = append(thunderURNs, urn)
		}
	}
	if len(thunderURNs) == 0 {
		return nil, &ErrorMissingCustomSchema
	}
	if len(thunderURNs) > 1 {
		return nil, &ErrorMultipleCustomSchemas
	}
	extensionURN := thunderURNs[0]

	// step 5: ThunderID URN well-formed
	userTypeName, ok := parseUserTypeFromSchemaURN(extensionURN)
	if !ok || strings.TrimSpace(userTypeName) == "" {
		return nil, &ErrorInvalidCustomSchemaURN
	}

	// Step 6: extension object must exist and be a JSON object
	var extRaw json.RawMessage
	for k, v := range raw {
		if strings.EqualFold(k, extensionURN) {
			extRaw = v
			break
		}
	}
	if extRaw == nil {
		return nil, &ErrorMissingCustomSchemaObject
	}

	var extAttrs map[string]json.RawMessage
	if err := json.Unmarshal(extRaw, &extAttrs); err != nil || extAttrs == nil {
		return nil, &ErrorMissingCustomSchemaObject
	}

	// Collect core attributes (those that are not "schemas" or the extension URN)
	coreAttrs := make(map[string]json.RawMessage)
	for k, v := range raw {
		if strings.EqualFold(k, "schemas") {
			continue
		}
		if strings.EqualFold(k, extensionURN) {
			continue
		}
		coreAttrs[k] = v
	}
	return &SCIMUserPayload{
		ExtensionURN:   extensionURN,
		UserTypeName:   userTypeName,
		CoreAttrs:      coreAttrs,
		ExtensionAttrs: extAttrs,
	}, nil
}
