// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/entitytype"
	scim "github.com/thunder-id/thunderid/internal/scim/common"
)

// mapUserTypeToSCIMSchema converts a ThunderID user type (EntityType) into a SCIM Schema resource
// per RFC 7643 §7.
func mapUserTypeToSCIMSchema(et entitytype.EntityType, baseURL string) (SCIMSchema, error) {
	schemaURN := scim.BuildSchemaURN(et.Name)
	location := fmt.Sprintf("%s%s/Schemas/%s", baseURL, scim.SCIMBasePath, schemaURN)
	description := fmt.Sprintf("%s user type", et.Name)

	// Parse the raw schema JSON into our property def map.
	var rawProps map[string]scim.RawPropertyDef
	if err := json.Unmarshal(et.Schema, &rawProps); err != nil {
		return SCIMSchema{}, fmt.Errorf(
			"mapUserTypeToSCIMSchema: failed to parse schema JSON for %q: %w",
			et.Name, err,
		)
	}

	// Convert every property dynamically — no hardcoding, no length limit.
	attributes := make([]scimSchemaAttribute, 0, len(rawProps))
	for propName, propDef := range rawProps {
		attributes = append(attributes, mapRawPropertyToSCIMAttribute(propName, propDef))
	}

	return SCIMSchema{
		Schemas:     []string{scimSchemaSchemaURN},
		ID:          schemaURN,
		Name:        et.Name,
		Description: description,
		Attributes:  attributes,
		Meta: scim.SCIMMeta{
			ResourceType: "Schema",
			Location:     location,
		},
	}, nil
}

// mapRawPropertyToSCIMAttribute recursively converts a single scim.RawPropertyDef into a
// scimSchemaAttribute. Called for every top-level attribute and for each sub-attribute
// of object and array-of-object properties.
func mapRawPropertyToSCIMAttribute(name string, def scim.RawPropertyDef) scimSchemaAttribute {
	attr := scimSchemaAttribute{
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
	case scim.RawPropertyTypeString:
		attr.Type = scimAttrTypeString
		// Map enum constraint → canonicalValues (RFC 7643 §7, advisory list).
		if len(def.Enum) > 0 {
			attr.CanonicalValues = rawEnumToStrings(def.Enum)
		}

	case scim.RawPropertyTypeNumber:
		attr.Type = scimAttrTypeDecimal
		// Number enum values are stringified to fit the []string canonicalValues field.
		if len(def.Enum) > 0 {
			attr.CanonicalValues = rawEnumToStrings(def.Enum)
		}

	case scim.RawPropertyTypeBoolean:
		attr.Type = scimAttrTypeBoolean
		// boolean has no enum / regex — nothing extra to map.

	case scim.RawPropertyTypeObject:
		// Complex type: recursively map every nested property as a sub-attribute.
		attr.Type = scimAttrTypeComplex
		if len(def.Properties) > 0 {
			subs := make([]scimSchemaAttribute, 0, len(def.Properties))
			for subName, subDef := range def.Properties {
				subs = append(subs, mapRawPropertyToSCIMAttribute(subName, subDef))
			}
			attr.SubAttributes = subs
		}

	case scim.RawPropertyTypeArray:
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
			if itemAttr.Returned == scimReturnedNever {
				attr.Returned = scimReturnedNever
				attr.Mutability = scimMutabilityWriteOnly
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

// buildCoreUserSchema returns the SCIM Core User schema (RFC 7643 §4.1), with each attribute
// beyond "id" included only if coreType's own schema defines a matching ThunderID attribute
// (see scim.CoreAttrRules), and marked required accordingly. coreType is the ThunderID user
// type designated (via SCIMConfig.CoreUserTypeID, or the sole-user-type fallback) as the
// source of truth for what the core schema actually accepts and returns.
func buildCoreUserSchema(baseURL string, coreType entitytype.EntityType) (SCIMSchema, error) {
	location := fmt.Sprintf("%s%s/Schemas/%s", baseURL, scim.SCIMBasePath, scim.SCIMCoreUserSchemaURN)
	attrs, err := coreUserAttributes(coreType)
	if err != nil {
		return SCIMSchema{}, err
	}
	return SCIMSchema{
		Schemas:     []string{scimSchemaSchemaURN},
		ID:          scim.SCIMCoreUserSchemaURN,
		Name:        "User",
		Description: "User Account",
		Attributes:  attrs,
		Meta: scim.SCIMMeta{
			ResourceType: "Schema",
			Location:     location,
		},
	}, nil
}

// coreUserAttributes derives the declared SCIM core User schema attributes (RFC 7643 §4.1)
// from coreType's schema: "id" is always present (server-managed, RFC-mandated); every other
// field is included only if coreType defines a matching ThunderID attribute for it (per
// scim.CoreAttrRules), with Required set from that attribute's own required flag.
func coreUserAttributes(coreType entitytype.EntityType) ([]scimSchemaAttribute, error) {
	var rawProps map[string]scim.RawPropertyDef
	if len(coreType.Schema) > 0 {
		if err := json.Unmarshal(coreType.Schema, &rawProps); err != nil {
			return nil, fmt.Errorf(
				"coreUserAttributes: failed to parse schema JSON for %q: %w", coreType.Name, err,
			)
		}
	}

	template := rfcCoreUserAttributeTemplate()
	attrs := make([]scimSchemaAttribute, 0, len(template))
	for _, attr := range template {
		if attr.Name == "id" {
			attrs = append(attrs, attr)
			continue
		}
		candidates := scim.CandidatesForField(scim.CoreField(attr.Name))
		matched, required := scim.HasSchemaMatch(rawProps, candidates)
		if !matched {
			continue
		}
		attr.Required = required
		if len(attr.SubAttributes) > 0 {
			attr.SubAttributes = filterMatchedSubAttributes(scim.CoreField(attr.Name), attr.SubAttributes, rawProps)
		}
		attrs = append(attrs, attr)
	}
	return attrs, nil
}

// filterMatchedSubAttributes keeps only the sub-attributes of field's complex value that either
// have no ThunderID-mapped candidate of their own (protocol structure like "value"/"type"/
// "primary", which rides along with the parent field's own match) or whose own candidate
// matches rawProps — e.g. name.middleName is dropped when the type defines no middle_name
// attribute, even though name itself is included via given_name.
func filterMatchedSubAttributes(
	field scim.CoreField, subs []scimSchemaAttribute, rawProps map[string]scim.RawPropertyDef,
) []scimSchemaAttribute {
	filtered := make([]scimSchemaAttribute, 0, len(subs))
	for _, sub := range subs {
		candidates := scim.CandidatesForSubAttr(field, sub.Name)
		if len(candidates) == 0 {
			filtered = append(filtered, sub)
			continue
		}
		if matched, _ := scim.HasSchemaMatch(rawProps, candidates); matched {
			filtered = append(filtered, sub)
		}
	}
	return filtered
}

// rfcCoreUserAttributeTemplate returns the full SCIM core User attribute set per RFC 7643 §4.1,
// with each attribute's SCIM-mandated shape (type, sub-attributes, mutability, etc.) — the
// characteristics that don't vary by ThunderID user type. coreUserAttributes filters this
// template down to what the designated user type actually defines.
func rfcCoreUserAttributeTemplate() []scimSchemaAttribute {
	return []scimSchemaAttribute{
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
			Required:   true,
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
			SubAttributes: []scimSchemaAttribute{
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
			SubAttributes: []scimSchemaAttribute{
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
			SubAttributes: []scimSchemaAttribute{
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
			SubAttributes: []scimSchemaAttribute{
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
			SubAttributes: []scimSchemaAttribute{
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
	location := fmt.Sprintf("%s%s/Schemas/%s", baseURL, scim.SCIMBasePath, scim.SCIMCoreGroupSchemaURN)
	return SCIMSchema{
		Schemas:     []string{scimSchemaSchemaURN},
		ID:          scim.SCIMCoreGroupSchemaURN,
		Name:        "Group",
		Description: "Group",
		Attributes:  coreGroupAttributes(),
		Meta: scim.SCIMMeta{
			ResourceType: "Schema",
			Location:     location,
		},
	}
}

// coreGroupAttributes returns the SCIM core Group attributes per RFC 7643 §4.2.
func coreGroupAttributes() []scimSchemaAttribute {
	return []scimSchemaAttribute{
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
			Required:    true,
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
			SubAttributes: []scimSchemaAttribute{
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
