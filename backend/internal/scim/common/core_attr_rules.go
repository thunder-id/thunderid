// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import "strings"

// CoreField is a top-level SCIM core User schema attribute name.
type CoreField string

// Core User schema top-level field names (RFC 7643 §4.1).
const (
	fieldUserName          CoreField = "userName"
	fieldEmails            CoreField = "emails"
	fieldPhoneNumbers      CoreField = "phoneNumbers"
	fieldDisplayName       CoreField = "displayName"
	fieldName              CoreField = "name"
	fieldTitle             CoreField = "title"
	fieldPreferredLanguage CoreField = "preferredLanguage"
	fieldTimezone          CoreField = "timezone"
	fieldAddresses         CoreField = "addresses"
	fieldNickName          CoreField = "nickName"
	fieldPhotos            CoreField = "photos"
	fieldLocale            CoreField = "locale"
	fieldProfileURL        CoreField = "profileUrl"
)

// AttrKind classifies how a CoreAttrRule's candidate attribute rolls into its SCIM core field.
type AttrKind string

// CoreAttrRule.Kind values.
const (
	KindSimpleString     AttrKind = "simpleString"     // userName, displayName, title, etc.
	KindMultiComplex     AttrKind = "multiComplex"     // emails, phoneNumbers, photos, addresses
	KindSubAttr          AttrKind = "subAttr"          // sub-attribute of a complex parent object, e.g. name.givenName
	KindMultiComplexPart AttrKind = "multiComplexPart" // discrete attribute rolling into a KindMultiComplex
	// field's single entry, e.g. street_address/locality/etc into addresses[0]. Not specific to
	// addresses: any KindMultiComplex field can gain part rules by setting ParentField to it.
)

// CoreAttrRule maps a single ThunderID library attribute candidate to its SCIM core User schema
// representation. Shared between users (runtime attribute mapping) and discovery (deriving
// declared core schema characteristics from a designated ThunderID user type).
type CoreAttrRule struct {
	Candidate   string    // ThunderID attr name, matched case-insensitively
	SCIMField   CoreField // target top-level SCIM field (KindSimpleString/KindMultiComplex only)
	Kind        AttrKind
	ParentField CoreField // for KindSubAttr/KindMultiComplexPart; complex object/entry this rolls into
	SubAttr     string    // only for KindSubAttr/KindMultiComplexPart; key within the parent object/entry
	ValueKey    string    // e.g. "value" (default)
}

// CoreAttrRules is the pre-configured mapping table, one candidate per ThunderID library attribute.
var CoreAttrRules = []CoreAttrRule{
	{
		Candidate: "username",
		SCIMField: fieldUserName,
		Kind:      KindSimpleString,
	},
	{
		Candidate: "email",
		SCIMField: fieldEmails,
		Kind:      KindMultiComplex,
		ValueKey:  "value",
	},
	{
		Candidate:   "given_name",
		Kind:        KindSubAttr,
		ParentField: fieldName,
		SubAttr:     "givenName",
	},
	{
		Candidate:   "family_name",
		Kind:        KindSubAttr,
		ParentField: fieldName,
		SubAttr:     "familyName",
	},
	{
		Candidate: "phone_number",
		SCIMField: fieldPhoneNumbers,
		Kind:      KindMultiComplex,
		ValueKey:  "value",
	},
	{
		Candidate: "display_name",
		SCIMField: fieldDisplayName,
		Kind:      KindSimpleString,
	},
	{
		Candidate:   "name",
		Kind:        KindSubAttr,
		ParentField: fieldName,
		SubAttr:     "formatted",
	},
	{
		Candidate:   "middle_name",
		Kind:        KindSubAttr,
		ParentField: fieldName,
		SubAttr:     "middleName",
	},
	{
		Candidate: "nickname",
		SCIMField: fieldNickName,
		Kind:      KindSimpleString,
	},
	{
		Candidate: "picture",
		SCIMField: fieldPhotos,
		Kind:      KindMultiComplex,
		ValueKey:  "value",
	},
	{
		Candidate: "locale",
		SCIMField: fieldLocale,
		Kind:      KindSimpleString,
	},
	{
		Candidate: "preferred_language",
		SCIMField: fieldPreferredLanguage,
		Kind:      KindSimpleString,
	},
	{
		Candidate: "zoneinfo",
		SCIMField: fieldTimezone,
		Kind:      KindSimpleString,
	},
	{
		Candidate: "profile",
		SCIMField: fieldProfileURL,
		Kind:      KindSimpleString,
	},
	{
		Candidate: "title",
		SCIMField: fieldTitle,
		Kind:      KindSimpleString,
	},
	{
		Candidate: "address",
		SCIMField: fieldAddresses,
		Kind:      KindMultiComplex,
		ValueKey:  "formatted",
	},
	{
		Candidate:   "street_address",
		Kind:        KindMultiComplexPart,
		ParentField: fieldAddresses,
		SubAttr:     "streetAddress",
	},
	{
		Candidate:   "locality",
		Kind:        KindMultiComplexPart,
		ParentField: fieldAddresses,
		SubAttr:     "locality",
	},
	{
		Candidate:   "region",
		Kind:        KindMultiComplexPart,
		ParentField: fieldAddresses,
		SubAttr:     "region",
	},
	{
		Candidate:   "postal_code",
		Kind:        KindMultiComplexPart,
		ParentField: fieldAddresses,
		SubAttr:     "postalCode",
	},
	{
		Candidate:   "country",
		Kind:        KindMultiComplexPart,
		ParentField: fieldAddresses,
		SubAttr:     "country",
	},
}

// CandidatesForField returns the ThunderID candidate attribute names (CoreAttrRules.Candidate)
// that contribute to field, whether as its own value (KindSimpleString/KindMultiComplex) or as
// a nested piece of it (KindSubAttr/KindMultiComplexPart). Used by discovery to determine
// whether a designated user type's schema defines any attribute backing a given core SCIM field.
func CandidatesForField(field CoreField) []string {
	var candidates []string
	for _, rule := range CoreAttrRules {
		switch rule.Kind {
		case KindSimpleString, KindMultiComplex:
			if rule.SCIMField == field {
				candidates = append(candidates, rule.Candidate)
			}
		case KindSubAttr, KindMultiComplexPart:
			if rule.ParentField == field {
				candidates = append(candidates, rule.Candidate)
			}
		}
	}
	return candidates
}

// CandidatesForSubAttr returns the ThunderID candidate attribute names that back a specific
// sub-attribute (subAttr) of field's complex value — e.g. CandidatesForSubAttr(fieldName,
// "givenName") returns ["given_name"]. Returns nil when subAttr has no ThunderID-mapped
// candidate of its own (protocol structure such as "value"/"type"/"primary", or a value key
// like "formatted" that is really the parent KindMultiComplex rule's own ValueKey) — such
// sub-attributes ride along with the parent field's own match rather than being individually
// gated.
func CandidatesForSubAttr(field CoreField, subAttr string) []string {
	var candidates []string
	for _, rule := range CoreAttrRules {
		if (rule.Kind == KindSubAttr || rule.Kind == KindMultiComplexPart) &&
			rule.ParentField == field && rule.SubAttr == subAttr {
			candidates = append(candidates, rule.Candidate)
		}
	}
	return candidates
}

// HasSchemaMatch reports whether any of candidates matches a property name in rawProps
// (case-insensitive), and whether that property (or any matching property, if more than
// one candidate matches) is marked required.
func HasSchemaMatch(rawProps map[string]RawPropertyDef, candidates []string) (matched, required bool) {
	for _, candidate := range candidates {
		for propName, propDef := range rawProps {
			if strings.EqualFold(propName, candidate) {
				matched = true
				if propDef.Required {
					required = true
				}
			}
		}
	}
	return matched, required
}
