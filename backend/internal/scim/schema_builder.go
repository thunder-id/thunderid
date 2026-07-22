/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package scim

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/entitytype"
)

// mapEntityTypeToSCIMSchema converts a ThunderID EntityType into a SCIM Schema resource
// per RFC 7643 §7.
func mapEntityTypeToSCIMSchema(et entitytype.EntityType, baseURL string) (SCIMSchema, error) {
	schemaURN := buildSchemaURN(et.Name)
	location := fmt.Sprintf("%s%s/Schemas/%s", baseURL, SCIMBasePath, schemaURN)
	description := fmt.Sprintf("%s user type", et.Name)

	// Parse the raw schema JSON into our property def map.
	var rawProps map[string]rawPropertyDef
	if err := json.Unmarshal(et.Schema, &rawProps); err != nil {
		return SCIMSchema{}, fmt.Errorf(
			"mapEntityTypeToSCIMSchema: failed to parse schema JSON for %q: %w",
			et.Name, err,
		)
	}

	// Convert every property dynamically — no hardcoding, no length limit.
	attributes := make([]SCIMSchemaAttribute, 0, len(rawProps))
	for propName, propDef := range rawProps {
		attributes = append(attributes, mapRawPropertyToSCIMAttribute(propName, propDef))
	}

	return SCIMSchema{
		Schemas:     []string{SCIMSchemaSchemaURN},
		ID:          schemaURN,
		Name:        et.Name,
		Description: description,
		Attributes:  attributes,
		Meta: SCIMMeta{
			ResourceType: "Schema",
			Location:     location,
			Version:      computeSchemaVersion(et),
		},
	}, nil
}

// mapRawPropertyToSCIMAttribute recursively converts a single rawPropertyDef into a
// SCIMSchemaAttribute. Called for every top-level attribute and for each sub-attribute
// of object and array-of-object properties.
func mapRawPropertyToSCIMAttribute(name string, def rawPropertyDef) SCIMSchemaAttribute {
	attr := SCIMSchemaAttribute{
		Name:        name,
		Description: def.DisplayName,
		Required:    def.Required,
		CaseExact:   true,
		MultiValued: false,
		Mutability:  scimMutabilityReadWrite,
		Returned:    scimReturnedDefault,
		Uniqueness:  scimUniquenessNone,
	}

	// Credential fields must never be returned per RFC 7643 §7 and the proposal security constraints.
	if def.Credential {
		attr.Returned = scimReturnedNever
		attr.Mutability = scimMutabilityWriteOnly
		attr.CaseExact = true
	}

	if def.Unique {
		attr.Uniqueness = scimUniquenessServer
	}

	// Derive SCIM type and populate type-specific extras.
	switch strings.ToLower(def.Type) {
	case "string":
		attr.Type = scimAttrTypeString
		// Map enum constraint → canonicalValues (RFC 7643 §7, advisory list).
		if len(def.Enum) > 0 {
			attr.CanonicalValues = rawEnumToStrings(def.Enum)
		}

	case "number":
		attr.Type = scimAttrTypeDecimal
		// Number enum values are stringified to fit the []string canonicalValues field.
		if len(def.Enum) > 0 {
			attr.CanonicalValues = rawEnumToStrings(def.Enum)
		}

	case "boolean":
		attr.Type = scimAttrTypeBoolean
		// boolean has no enum / regex — nothing extra to map.

	case rawPropertyTypeObject:
		// Complex type: recursively map every nested property as a sub-attribute.
		attr.Type = scimAttrTypeComplex
		if len(def.Properties) > 0 {
			subs := make([]SCIMSchemaAttribute, 0, len(def.Properties))
			for subName, subDef := range def.Properties {
				subs = append(subs, mapRawPropertyToSCIMAttribute(subName, subDef))
			}
			attr.SubAttributes = subs
		}

	case rawPropertyTypeArray:
		// Multi-valued type: the SCIM type is derived from the items definition.
		attr.MultiValued = true
		if def.Items != nil {
			itemAttr := mapRawPropertyToSCIMAttribute(name, *def.Items)
			attr.Type = itemAttr.Type
			// Propagate all extras from the items attribute.
			if len(itemAttr.SubAttributes) > 0 {
				attr.SubAttributes = itemAttr.SubAttributes
			}
			if len(itemAttr.CanonicalValues) > 0 {
				attr.CanonicalValues = itemAttr.CanonicalValues
			}
		} else {
			// Array without an items definition — default to string per RFC 7643 §2.3.
			attr.Type = scimAttrTypeString
		}

	default:
		// Unknown type: fall back to string. CompileSchema rejects unknown types at
		// write time, so this branch is a defensive guard for future type additions.
		attr.Type = scimAttrTypeString
	}

	return attr
}

// buildCoreUserSchema returns the static SCIM Core User schema (RFC 7643 §4.1).
func buildCoreUserSchema(baseURL string) SCIMSchema {
	location := fmt.Sprintf("%s%s/Schemas/%s", baseURL, SCIMBasePath, SCIMCoreUserSchemaURN)
	return SCIMSchema{
		Schemas:     []string{SCIMSchemaSchemaURN},
		ID:          SCIMCoreUserSchemaURN,
		Name:        "User",
		Description: "User Account",
		Attributes:  coreUserAttributes(),
		Meta: SCIMMeta{
			ResourceType: "Schema",
			Location:     location,
		},
	}
}

// coreUserAttributes returns the minimal set of SCIM core User attributes per RFC 7643 §4.1.
// Kept separate from buildCoreUserSchema for readability and unit-testability.
func coreUserAttributes() []SCIMSchemaAttribute {
	return []SCIMSchemaAttribute{
		{
			Name:        "id",
			Type:        scimAttrTypeString,
			Description: "Unique identifier for the SCIM resource.",
			Required:    false,
			CaseExact:   true,
			Mutability:  scimMutabilityReadOnly,
			Returned:    scimReturnedAlways,
			Uniqueness:  scimUniquenessServer,
		},
		{
			Name: "userName",
			Type: scimAttrTypeString,
			Description: "Unique identifier for the User, typically used by the user to directly " +
				"authenticate to the service provider.",
			Required:   false,
			CaseExact:  false,
			Mutability: scimMutabilityReadWrite,
			Returned:   scimReturnedDefault,
			Uniqueness: scimUniquenessServer,
		},
		{
			Name:        "displayName",
			Type:        scimAttrTypeString,
			Description: "The name of the User, suitable for display to end-users.",
			Required:    false,
			CaseExact:   false,
			Mutability:  scimMutabilityReadWrite,
			Returned:    scimReturnedDefault,
			Uniqueness:  scimUniquenessNone,
		},
		{
			Name:        "name",
			Type:        scimAttrTypeComplex,
			Description: "The components of the user's real name.",
			Required:    false,
			CaseExact:   false,
			Mutability:  scimMutabilityReadWrite,
			Returned:    scimReturnedDefault,
			Uniqueness:  scimUniquenessNone,
			SubAttributes: []SCIMSchemaAttribute{
				{
					Name:        "formatted",
					Type:        scimAttrTypeString,
					Description: "The full name, including all middle names, titles, and suffixes.",
					Required:    false,
					CaseExact:   false,
					Mutability:  scimMutabilityReadWrite,
					Returned:    scimReturnedDefault,
					Uniqueness:  scimUniquenessNone,
				},
				{
					Name:        "givenName",
					Type:        scimAttrTypeString,
					Description: "The given name of the User, or first name.",
					Required:    false,
					CaseExact:   false,
					Mutability:  scimMutabilityReadWrite,
					Returned:    scimReturnedDefault,
					Uniqueness:  scimUniquenessNone,
				},
				{
					Name:        "familyName",
					Type:        scimAttrTypeString,
					Description: "The family name of the User, or last name.",
					Required:    false,
					CaseExact:   false,
					Mutability:  scimMutabilityReadWrite,
					Returned:    scimReturnedDefault,
					Uniqueness:  scimUniquenessNone,
				},
				{
					Name:        "middleName",
					Type:        scimAttrTypeString,
					Description: "The middle name(s) of the User.",
					Required:    false,
					CaseExact:   false,
					Mutability:  scimMutabilityReadWrite,
					Returned:    scimReturnedDefault,
					Uniqueness:  scimUniquenessNone,
				},
			},
		},
		{
			Name:        "emails",
			Type:        scimAttrTypeComplex,
			MultiValued: true,
			Description: "Email addresses for the user.",
			Required:    false,
			CaseExact:   false,
			Mutability:  scimMutabilityReadWrite,
			Returned:    scimReturnedDefault,
			Uniqueness:  scimUniquenessNone,
			SubAttributes: []SCIMSchemaAttribute{
				{
					Name:        "value",
					Type:        scimAttrTypeString,
					Description: "Email address.",
					Required:    false,
					CaseExact:   false,
					Mutability:  scimMutabilityReadWrite,
					Returned:    scimReturnedDefault,
					Uniqueness:  scimUniquenessNone,
				},
				{
					Name:        "type",
					Type:        scimAttrTypeString,
					Description: "A label indicating the attribute's function, e.g., 'work' or 'home'.",
					Required:    false,
					CaseExact:   false,
					Mutability:  scimMutabilityReadWrite,
					Returned:    scimReturnedDefault,
					Uniqueness:  scimUniquenessNone,
				},
				{
					Name: "primary",
					Type: scimAttrTypeBoolean,
					Description: "A Boolean value indicating the 'primary' or preferred attribute " +
						"value for this attribute.",
					Required:   false,
					CaseExact:  false,
					Mutability: scimMutabilityReadWrite,
					Returned:   scimReturnedDefault,
					Uniqueness: scimUniquenessNone,
				},
			},
		},
		{
			Name:        "phoneNumbers",
			Type:        scimAttrTypeComplex,
			MultiValued: true,
			Description: "Phone numbers for the user.",
			Required:    false,
			CaseExact:   false,
			Mutability:  scimMutabilityReadWrite,
			Returned:    scimReturnedDefault,
			Uniqueness:  scimUniquenessNone,
			SubAttributes: []SCIMSchemaAttribute{
				{
					Name:        "value",
					Type:        scimAttrTypeString,
					Description: "Phone number.",
					Required:    false,
					CaseExact:   false,
					Mutability:  scimMutabilityReadWrite,
					Returned:    scimReturnedDefault,
					Uniqueness:  scimUniquenessNone,
				},
				{
					Name:        "type",
					Type:        scimAttrTypeString,
					Description: "A label indicating the attribute's function, e.g., 'work', 'home', 'mobile'.",
					Required:    false,
					CaseExact:   false,
					Mutability:  scimMutabilityReadWrite,
					Returned:    scimReturnedDefault,
					Uniqueness:  scimUniquenessNone,
				},
				{
					Name: "primary",
					Type: scimAttrTypeBoolean,
					Description: "A Boolean value indicating the 'primary' or preferred attribute " +
						"value for this attribute.",
					Required:   false,
					CaseExact:  false,
					Mutability: scimMutabilityReadWrite,
					Returned:   scimReturnedDefault,
					Uniqueness: scimUniquenessNone,
				},
			},
		},
		{
			Name:        "photos",
			Type:        scimAttrTypeComplex,
			MultiValued: true,
			Description: "URLs of photos of the User.",
			Required:    false,
			CaseExact:   false,
			Mutability:  scimMutabilityReadWrite,
			Returned:    scimReturnedDefault,
			Uniqueness:  scimUniquenessNone,
			SubAttributes: []SCIMSchemaAttribute{
				{
					Name:        "value",
					Type:        scimAttrTypeString,
					Description: "URL of a photo of the User.",
					Required:    false,
					CaseExact:   false,
					Mutability:  scimMutabilityReadWrite,
					Returned:    scimReturnedDefault,
					Uniqueness:  scimUniquenessNone,
				},
				{
					Name:        "type",
					Type:        scimAttrTypeString,
					Description: "A label indicating the attribute's function, e.g., 'photo' or 'thumbnail'.",
					Required:    false,
					CaseExact:   false,
					Mutability:  scimMutabilityReadWrite,
					Returned:    scimReturnedDefault,
					Uniqueness:  scimUniquenessNone,
				},
				{
					Name: "primary",
					Type: scimAttrTypeBoolean,
					Description: "A Boolean value indicating the 'primary' or preferred attribute " +
						"value for this attribute.",
					Required:   false,
					CaseExact:  false,
					Mutability: scimMutabilityReadWrite,
					Returned:   scimReturnedDefault,
					Uniqueness: scimUniquenessNone,
				},
			},
		},
		{
			Name:        "nickName",
			Type:        scimAttrTypeString,
			Description: "The casual way to address the user in real life.",
			Required:    false,
			CaseExact:   false,
			Mutability:  scimMutabilityReadWrite,
			Returned:    scimReturnedDefault,
			Uniqueness:  scimUniquenessNone,
		},
		{
			Name:        "profileUrl",
			Type:        scimAttrTypeString,
			Description: "A fully qualified URL pointing to a page representing the User's online profile.",
			Required:    false,
			CaseExact:   false,
			Mutability:  scimMutabilityReadWrite,
			Returned:    scimReturnedDefault,
			Uniqueness:  scimUniquenessNone,
		},
		{
			Name:        "title",
			Type:        scimAttrTypeString,
			Description: "The user's title, such as \"Vice President.\"",
			Required:    false,
			CaseExact:   false,
			Mutability:  scimMutabilityReadWrite,
			Returned:    scimReturnedDefault,
			Uniqueness:  scimUniquenessNone,
		},
		{
			Name:        "preferredLanguage",
			Type:        scimAttrTypeString,
			Description: "Indicates the User's preferred written or spoken language.",
			Required:    false,
			CaseExact:   false,
			Mutability:  scimMutabilityReadWrite,
			Returned:    scimReturnedDefault,
			Uniqueness:  scimUniquenessNone,
		},
		{
			Name: "locale",
			Type: scimAttrTypeString,
			Description: "Used to indicate the User's default location, for purposes of localizing " +
				"items such as currency, date time format, or numerical representations.",
			Required:   false,
			CaseExact:  false,
			Mutability: scimMutabilityReadWrite,
			Returned:   scimReturnedDefault,
			Uniqueness: scimUniquenessNone,
		},
		{
			Name:        "timezone",
			Type:        scimAttrTypeString,
			Description: "The User's time zone in the 'Olson' time zone database format.",
			Required:    false,
			CaseExact:   false,
			Mutability:  scimMutabilityReadWrite,
			Returned:    scimReturnedDefault,
			Uniqueness:  scimUniquenessNone,
		},
		{
			Name:        "addresses",
			Type:        scimAttrTypeComplex,
			MultiValued: true,
			Description: "A physical mailing address for this User.",
			Required:    false,
			CaseExact:   false,
			Mutability:  scimMutabilityReadWrite,
			Returned:    scimReturnedDefault,
			Uniqueness:  scimUniquenessNone,
			SubAttributes: []SCIMSchemaAttribute{
				{
					Name:        "formatted",
					Type:        scimAttrTypeString,
					Description: "The full mailing address, formatted for display or use with a mailing label.",
					Required:    false,
					CaseExact:   false,
					Mutability:  scimMutabilityReadWrite,
					Returned:    scimReturnedDefault,
					Uniqueness:  scimUniquenessNone,
				},
				{
					Name:        "streetAddress",
					Type:        scimAttrTypeString,
					Description: "The full street address component.",
					Required:    false,
					CaseExact:   false,
					Mutability:  scimMutabilityReadWrite,
					Returned:    scimReturnedDefault,
					Uniqueness:  scimUniquenessNone,
				},
				{
					Name:        "locality",
					Type:        scimAttrTypeString,
					Description: "The city or locality component.",
					Required:    false,
					CaseExact:   false,
					Mutability:  scimMutabilityReadWrite,
					Returned:    scimReturnedDefault,
					Uniqueness:  scimUniquenessNone,
				},
				{
					Name:        "region",
					Type:        scimAttrTypeString,
					Description: "The state or region component.",
					Required:    false,
					CaseExact:   false,
					Mutability:  scimMutabilityReadWrite,
					Returned:    scimReturnedDefault,
					Uniqueness:  scimUniquenessNone,
				},
				{
					Name:        "postalCode",
					Type:        scimAttrTypeString,
					Description: "The zip code or postal code component.",
					Required:    false,
					CaseExact:   false,
					Mutability:  scimMutabilityReadWrite,
					Returned:    scimReturnedDefault,
					Uniqueness:  scimUniquenessNone,
				},
				{
					Name:        "country",
					Type:        scimAttrTypeString,
					Description: "The country name component.",
					Required:    false,
					CaseExact:   false,
					Mutability:  scimMutabilityReadWrite,
					Returned:    scimReturnedDefault,
					Uniqueness:  scimUniquenessNone,
				},
				{
					Name:        "type",
					Type:        scimAttrTypeString,
					Description: "A label indicating the attribute's function, e.g., 'work' or 'home'.",
					Required:    false,
					CaseExact:   false,
					Mutability:  scimMutabilityReadWrite,
					Returned:    scimReturnedDefault,
					Uniqueness:  scimUniquenessNone,
				},
				{
					Name: "primary",
					Type: scimAttrTypeBoolean,
					Description: "A Boolean value indicating the 'primary' or preferred attribute " +
						"value for this attribute.",
					Required:   false,
					CaseExact:  false,
					Mutability: scimMutabilityReadWrite,
					Returned:   scimReturnedDefault,
					Uniqueness: scimUniquenessNone,
				},
			},
		},
	}
}

// buildCoreGroupSchema returns the static SCIM Core Group schema (RFC 7643 §4.2).
func buildCoreGroupSchema(baseURL string) SCIMSchema {
	location := fmt.Sprintf("%s%s/Schemas/%s", baseURL, SCIMBasePath, SCIMCoreGroupSchemaURN)
	return SCIMSchema{
		Schemas:     []string{SCIMSchemaSchemaURN},
		ID:          SCIMCoreGroupSchemaURN,
		Name:        "Group",
		Description: "Group",
		Attributes:  coreGroupAttributes(),
		Meta: SCIMMeta{
			ResourceType: "Schema",
			Location:     location,
		},
	}
}

// coreGroupAttributes returns the SCIM core Group attributes per RFC 7643 §4.2.
func coreGroupAttributes() []SCIMSchemaAttribute {
	return []SCIMSchemaAttribute{
		{
			Name:        "id",
			Type:        scimAttrTypeString,
			Description: "Unique identifier for the SCIM resource.",
			Required:    false,
			CaseExact:   true,
			Mutability:  scimMutabilityReadOnly,
			Returned:    scimReturnedAlways,
			Uniqueness:  scimUniquenessServer,
		},
		{
			Name:        "displayName",
			Type:        scimAttrTypeString,
			Description: "A human-readable name for the Group.",
			Required:    false,
			Mutability:  scimMutabilityReadWrite,
			Returned:    scimReturnedDefault,
			Uniqueness:  scimUniquenessNone,
		},
		{
			Name:        "members",
			Type:        scimAttrTypeComplex,
			MultiValued: true,
			Description: "A list of members of the Group.",
			Required:    false,
			Mutability:  scimMutabilityReadWrite,
			Returned:    scimReturnedDefault,
			Uniqueness:  scimUniquenessNone,
			SubAttributes: []SCIMSchemaAttribute{
				{
					Name:        "value",
					Type:        scimAttrTypeString,
					Description: "Identifier of the member resource.",
					Mutability:  scimMutabilityImmutable,
					Returned:    scimReturnedDefault,
					Uniqueness:  scimUniquenessNone,
				},
				{
					Name:        "$ref",
					Type:        scimAttrTypeString,
					Description: "The URI of the SCIM resource.",
					Mutability:  scimMutabilityImmutable,
					Returned:    scimReturnedDefault,
					Uniqueness:  scimUniquenessNone,
				},
				{
					Name:            "type",
					Type:            scimAttrTypeString,
					Description:     "A label indicating the attribute's resource type.",
					CanonicalValues: []string{"User", "Group"},
					Mutability:      scimMutabilityImmutable,
					Returned:        scimReturnedDefault,
					Uniqueness:      scimUniquenessNone,
				},
				{
					Name:        "display",
					Type:        scimAttrTypeString,
					Description: "A human-readable name for the member.",
					Mutability:  scimMutabilityImmutable,
					Returned:    scimReturnedDefault,
					Uniqueness:  scimUniquenessNone,
				},
			},
		},
	}
}

// rawPropertyDef is the internal representation of a single property from a
// ThunderID EntityType JSON schema. It is used only for unmarshalling during
// SCIM schema mapping and is not exposed outside this package.
type rawPropertyDef struct {
	Type        string                    `json:"type"`
	Required    bool                      `json:"required"`
	Unique      bool                      `json:"unique"`
	Credential  bool                      `json:"credential"`
	DisplayName string                    `json:"displayName"`
	Enum        []json.RawMessage         `json:"enum"`       // string: ["a","b"] / number: [1,2]
	Regex       string                    `json:"regex"`      // string type only; mutually exclusive with Pattern
	Pattern     string                    `json:"pattern"`    // alias for Regex; model rejects both being set
	Properties  map[string]rawPropertyDef `json:"properties"` // for type=object
	Items       *rawPropertyDef           `json:"items"`      // for type=array
}

// buildSchemaURN returns the canonical lowercase SCIM extension URN for a ThunderID user type.
// Format: urn:thunderid:params:scim:schemas:<userTypeName>:2.0:User
func buildSchemaURN(userTypeName string) string {
	return ThunderIDURNPrefix + strings.ToLower(userTypeName) + ThunderIDURNSuffix
}

// parseUserTypeFromSchemaURN extracts the user type name from a ThunderID extension URN.
// Matching is case-insensitive per the proposal decision.
// Returns the name and true on success; empty string and false if the URN is not a
// well-formed ThunderID extension URN.
func parseUserTypeFromSchemaURN(schemaURN string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(schemaURN))

	lowerPrefix := strings.ToLower(ThunderIDURNPrefix)
	lowerSuffix := strings.ToLower(ThunderIDURNSuffix)

	if !strings.HasPrefix(lower, lowerPrefix) {
		return "", false
	}

	withoutPrefix := lower[len(lowerPrefix):]

	if !strings.HasSuffix(withoutPrefix, lowerSuffix) {
		return "", false
	}

	name := strings.TrimSuffix(withoutPrefix, lowerSuffix)

	if name == "" {
		return "", false
	}

	return name, true
}

// rawEnumToStrings converts a []json.RawMessage enum array into []string.
// Both string enum values ("active") and number enum values (42, 3.14) are
// converted to their JSON text representation so they fit the SCIM
// canonicalValues field (RFC 7643 §7), which is always []string.
func rawEnumToStrings(raw []json.RawMessage) []string {
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		// Try to unmarshal as a plain string first.
		var s string
		if err := json.Unmarshal(item, &s); err == nil {
			out = append(out, s)
			continue
		}
		// Fall back: use the raw JSON token (e.g. "42" or "3.14") as the string value.
		out = append(out, strings.TrimSpace(string(item)))
	}
	return out
}

// computeSchemaVersion produces a stable weak ETag for a dynamic SCIM extension
// schema derived from a ThunderID EntityType. The hash covers the schema content
// and the entity type name (which determines the URN), so any attribute change or
// rename produces a new ETag. Format follows RFC 7232 weak validator convention.
func computeSchemaVersion(et entitytype.EntityType) string {
	state := struct {
		Name   string
		Schema json.RawMessage
	}{
		Name:   et.Name,
		Schema: et.Schema,
	}
	return generateVersion(state)
}
