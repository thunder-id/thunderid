// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/entitytype"
	entitytypemodel "github.com/thunder-id/thunderid/internal/entitytype/model"
	scim "github.com/thunder-id/thunderid/internal/scim/common"
)

// mapUserTypeToSCIMSchema converts a ThunderID user type into a SCIM Schema resource.
func mapUserTypeToSCIMSchema(et entitytype.EntityType, baseURL string) (SCIMSchema, error) {
	schemaURN := scim.BuildSchemaURN(et.Name)
	location := fmt.Sprintf(scimSchemaLocationFmt, baseURL, scim.SCIMBasePath, schemaURN)
	description := fmt.Sprintf("%s user type", et.Name)

	// Parse the raw schema JSON into our property def map.
	rawProps, err := scim.ParseRawProperties(et.Schema)
	if err != nil {
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
			ResourceType: scimMetaResourceTypeSchema,
			Location:     location,
		},
	}, nil
}

// mapRawPropertyToSCIMAttribute recursively converts a RawPropertyDef into a scimSchemaAttribute.
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
	returned, mutability := scim.CredentialCharacteristics(def.Credential)
	attr.Returned, attr.Mutability = scimReturned(returned), scimMutability(mutability)

	if def.Unique {
		attr.Uniqueness = scimUniquenessServer
	}

	// Derive SCIM type and populate type-specific extras.
	switch strings.ToLower(def.Type) {
	case entitytypemodel.TypeString:
		attr.Type = scimAttrTypeString
		// Map enum constraint → canonicalValues (RFC 7643 §7, advisory list).
		if len(def.Enum) > 0 {
			attr.CanonicalValues = rawEnumToStrings(def.Enum)
		}

	case entitytypemodel.TypeNumber:
		attr.Type = scimAttrTypeDecimal
		// Number enum values are stringified to fit the []string canonicalValues field.
		if len(def.Enum) > 0 {
			attr.CanonicalValues = rawEnumToStrings(def.Enum)
		}

	case entitytypemodel.TypeBoolean:
		attr.Type = scimAttrTypeBoolean
		// boolean has no enum / regex — nothing extra to map.

	case entitytypemodel.TypeObject:
		// Complex type: recursively map every nested property as a sub-attribute.
		attr.Type = scimAttrTypeComplex
		if len(def.Properties) > 0 {
			subs := make([]scimSchemaAttribute, 0, len(def.Properties))
			for subName, subDef := range def.Properties {
				subs = append(subs, mapRawPropertyToSCIMAttribute(subName, subDef))
			}
			attr.SubAttributes = subs
		}

	case entitytypemodel.TypeArray:
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

// buildCoreUserSchema builds the SCIM Core User schema from coreType.
func buildCoreUserSchema(baseURL string, coreType entitytype.EntityType) (SCIMSchema, error) {
	location := fmt.Sprintf(scimSchemaLocationFmt, baseURL, scim.SCIMBasePath, scim.SCIMCoreUserSchemaURN)
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
			ResourceType: scimMetaResourceTypeSchema,
			Location:     location,
		},
	}, nil
}

// buildEnterpriseUserSchema builds the SCIM Enterprise User schema extension from coreType.
func buildEnterpriseUserSchema(baseURL string, coreType entitytype.EntityType) (SCIMSchema, error) {
	location := fmt.Sprintf(scimSchemaLocationFmt, baseURL, scim.SCIMBasePath, scim.SCIMEnterpriseUserSchemaURN)
	attrs, err := enterpriseUserAttributes(coreType)
	if err != nil {
		return SCIMSchema{}, err
	}
	return SCIMSchema{
		Schemas:     []string{scimSchemaSchemaURN},
		ID:          scim.SCIMEnterpriseUserSchemaURN,
		Name:        "EnterpriseUser",
		Description: "Enterprise User",
		Attributes:  attrs,
		Meta: scim.SCIMMeta{
			ResourceType: scimMetaResourceTypeSchema,
			Location:     location,
		},
	}, nil
}

// enterpriseUserAttributes derives the declared SCIM Enterprise User schema attributes from coreType.
func enterpriseUserAttributes(coreType entitytype.EntityType) ([]scimSchemaAttribute, error) {
	rawProps, err := scim.ParseRawProperties(coreType.Schema)
	if err != nil {
		return nil, fmt.Errorf(
			"enterpriseUserAttributes: failed to parse schema JSON for %q: %w", coreType.Name, err,
		)
	}

	template := rfcEnterpriseUserAttributeTemplate()
	attrs := make([]scimSchemaAttribute, 0, len(template))
	for _, attr := range template {
		candidates := scim.CandidatesForEnterpriseField(scim.EnterpriseField(attr.Name))
		matched, required, credential := scim.HasSchemaMatch(rawProps, candidates)
		if !matched {
			continue
		}
		attr.Required = required
		returned, mutability := scim.CredentialCharacteristics(credential)
		attr.Returned, attr.Mutability = scimReturned(returned), scimMutability(mutability)
		attrs = append(attrs, attr)
	}
	return attrs, nil
}

// coreUserAttributes derives the declared SCIM core User schema attributes from coreType.
func coreUserAttributes(coreType entitytype.EntityType) ([]scimSchemaAttribute, error) {
	rawProps, err := scim.ParseRawProperties(coreType.Schema)
	if err != nil {
		return nil, fmt.Errorf(
			"coreUserAttributes: failed to parse schema JSON for %q: %w", coreType.Name, err,
		)
	}

	template := rfcCoreUserAttributeTemplate()
	attrs := make([]scimSchemaAttribute, 0, len(template))
	for _, attr := range template {
		if attr.Name == "id" {
			attrs = append(attrs, attr)
			continue
		}
		candidates := scim.CandidatesForField(scim.CoreField(attr.Name))
		matched, required, credential := scim.HasSchemaMatch(rawProps, candidates)
		if !matched {
			continue
		}
		attr.Required = required
		returned, mutability := scim.CredentialCharacteristics(credential)
		attr.Returned, attr.Mutability = scimReturned(returned), scimMutability(mutability)
		if len(attr.SubAttributes) > 0 {
			attr.SubAttributes = filterMatchedSubAttributes(scim.CoreField(attr.Name), attr.SubAttributes, rawProps)
		}
		attrs = append(attrs, attr)
	}
	return attrs, nil
}

// filterMatchedSubAttributes filters sub-attributes to those matching properties in rawProps.
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
		if matched, _, _ := scim.HasSchemaMatch(rawProps, candidates); matched {
			filtered = append(filtered, sub)
		}
	}
	return filtered
}

// rfcCoreUserAttributeTemplate returns the full SCIM core User attribute definitions.
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

// buildCoreGroupSchema returns the static SCIM Core Group schema.
func buildCoreGroupSchema(baseURL string) SCIMSchema {
	location := fmt.Sprintf(scimSchemaLocationFmt, baseURL, scim.SCIMBasePath, scim.SCIMCoreGroupSchemaURN)
	return SCIMSchema{
		Schemas:     []string{scimSchemaSchemaURN},
		ID:          scim.SCIMCoreGroupSchemaURN,
		Name:        "Group",
		Description: "Group",
		Attributes:  coreGroupAttributes(),
		Meta: scim.SCIMMeta{
			ResourceType: scimMetaResourceTypeSchema,
			Location:     location,
		},
	}
}

// coreGroupAttributes returns the SCIM core Group attributes.
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

// rawEnumToStrings converts a raw JSON enum array into a string slice.
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

// rfcEnterpriseUserAttributeTemplate returns the full SCIM Enterprise User attribute definitions.
func rfcEnterpriseUserAttributeTemplate() []scimSchemaAttribute {
	return []scimSchemaAttribute{
		{
			Name: "employeeNumber",
			Type: scimAttrTypeString,
			Description: "Numeric or alphanumeric identifier assigned to a person, typically based on " +
				"order of hire or association with an organization.",
			Required:   false,
			CaseExact:  false,
			Mutability: scimMutabilityReadWrite,
			Returned:   scimReturnedDefault,
			Uniqueness: scimUniquenessNone,
		},
		{
			Name:        "costCenter",
			Type:        scimAttrTypeString,
			Description: "Identifies the name of a cost center.",
			Required:    false,
			CaseExact:   false,
			Mutability:  scimMutabilityReadWrite,
			Returned:    scimReturnedDefault,
			Uniqueness:  scimUniquenessNone,
		},
		{
			Name:        "organization",
			Type:        scimAttrTypeString,
			Description: "Identifies the name of an organization.",
			Required:    false,
			CaseExact:   false,
			Mutability:  scimMutabilityReadWrite,
			Returned:    scimReturnedDefault,
			Uniqueness:  scimUniquenessNone,
		},
		{
			Name:        "division",
			Type:        scimAttrTypeString,
			Description: "Identifies the name of a division.",
			Required:    false,
			CaseExact:   false,
			Mutability:  scimMutabilityReadWrite,
			Returned:    scimReturnedDefault,
			Uniqueness:  scimUniquenessNone,
		},
		{
			Name:        "department",
			Type:        scimAttrTypeString,
			Description: "Identifies the name of a department.",
			Required:    false,
			CaseExact:   false,
			Mutability:  scimMutabilityReadWrite,
			Returned:    scimReturnedDefault,
			Uniqueness:  scimUniquenessNone,
		},
		{
			Name:        "manager",
			Type:        scimAttrTypeComplex,
			Description: "The User's manager.",
			Required:    false,
			MultiValued: false,
			Mutability:  scimMutabilityReadWrite,
			Returned:    scimReturnedDefault,
			Uniqueness:  scimUniquenessNone,
			SubAttributes: []scimSchemaAttribute{
				{
					Name:        "value",
					Type:        scimAttrTypeString,
					Description: "The id of the SCIM resource representing the User's manager.",
					Required:    false,
					CaseExact:   false,
					Mutability:  scimMutabilityReadWrite,
					Returned:    scimReturnedDefault,
					Uniqueness:  scimUniquenessNone,
				},
				{
					Name:           "$ref",
					Type:           scimAttrTypeReference,
					Description:    "The URI of the SCIM resource representing the User's manager.",
					Required:       false,
					CaseExact:      false,
					Mutability:     scimMutabilityReadOnly,
					Returned:       scimReturnedDefault,
					Uniqueness:     scimUniquenessNone,
					ReferenceTypes: []string{"User"},
				},
			},
		},
	}
}
