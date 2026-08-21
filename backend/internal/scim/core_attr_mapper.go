// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package scim

import (
	"encoding/json"
	"strings"
)

type scimCoreField string

const (
	scimFieldUserName      scimCoreField = "userName"
	scimFieldEmails        scimCoreField = "emails"
	scimFieldPhoneNumbers  scimCoreField = "phoneNumbers"
	scimFieldDisplayName   scimCoreField = "displayName"
	scimFieldName          scimCoreField = "name"
	scimFieldTitle         scimCoreField = "title"
	scimFieldPreferredLang scimCoreField = "preferredLanguage"
	scimFieldTimezone      scimCoreField = "timezone"
	scimFieldAddresses     scimCoreField = "addresses"
	scimFieldNickName      scimCoreField = "nickName"
	scimFieldPhotos        scimCoreField = "photos"
	scimFieldLocale        scimCoreField = "locale"
	scimFieldProfileURL    scimCoreField = "profileUrl"
)

const (
	scimValueKey     = "value"
	scimTypeKey      = "type"
	scimPrimaryKey   = "primary"
	scimFormattedKey = "formatted"
)

type attrKind string

const (
	kindSimpleString     attrKind = "simpleString"     // userName, displayName, title, etc.
	kindMultiComplex     attrKind = "multiComplex"     // emails, phoneNumbers, photos, addresses
	kindSubAttr          attrKind = "subAttr"          // sub-attribute of a complex parent object, e.g. name.givenName
	kindMultiComplexPart attrKind = "multiComplexPart" // discrete attribute rolling into a kindMultiComplex
	// field's single entry, e.g. street_address/locality/etc into addresses[0]. Not specific to
	// addresses: any kindMultiComplex field can gain part rules by setting parentField to it.
)

type coreAttrRule struct {
	candidate   string        // ThunderID attr name, matched case-insensitively
	scimField   scimCoreField // target top-level SCIM field (simpleString/multiComplex only)
	kind        attrKind
	parentField scimCoreField // for kindSubAttr/kindMultiComplexPart; complex object/entry this rolls into
	subAttr     string        // only for kindSubAttr/kindMultiComplexPart; key within the parent object/entry
	valueKey    string        // e.g. "value" (default)
}

// coreAttrRules is the pre-configured mapping table, one candidate per ThunderID library attribute.
var coreAttrRules = []coreAttrRule{
	{
		candidate: "username",
		scimField: scimFieldUserName,
		kind:      kindSimpleString,
	},
	{
		candidate: "email",
		scimField: scimFieldEmails,
		kind:      kindMultiComplex,
		valueKey:  scimValueKey,
	},
	{
		candidate:   "given_name",
		kind:        kindSubAttr,
		parentField: scimFieldName,
		subAttr:     "givenName",
	},
	{
		candidate:   "family_name",
		kind:        kindSubAttr,
		parentField: scimFieldName,
		subAttr:     "familyName",
	},
	{
		candidate: "phone_number",
		scimField: scimFieldPhoneNumbers,
		kind:      kindMultiComplex,
		valueKey:  scimValueKey,
	},
	{
		candidate: "display_name",
		scimField: scimFieldDisplayName,
		kind:      kindSimpleString,
	},
	{
		candidate:   "name",
		kind:        kindSubAttr,
		parentField: scimFieldName,
		subAttr:     "formatted",
	},
	{
		candidate:   "middle_name",
		kind:        kindSubAttr,
		parentField: scimFieldName,
		subAttr:     "middleName",
	},
	{
		candidate: "nickname",
		scimField: scimFieldNickName,
		kind:      kindSimpleString,
	},
	{
		candidate: "picture",
		scimField: scimFieldPhotos,
		kind:      kindMultiComplex,
		valueKey:  scimValueKey,
	},
	{
		candidate: "locale",
		scimField: scimFieldLocale,
		kind:      kindSimpleString,
	},
	{
		candidate: "preferred_language",
		scimField: scimFieldPreferredLang,
		kind:      kindSimpleString,
	},
	{
		candidate: "zoneinfo",
		scimField: scimFieldTimezone,
		kind:      kindSimpleString,
	},
	{
		candidate: "profile",
		scimField: scimFieldProfileURL,
		kind:      kindSimpleString,
	},
	{
		candidate: "title",
		scimField: scimFieldTitle,
		kind:      kindSimpleString,
	},
	{
		candidate: "address",
		scimField: scimFieldAddresses,
		kind:      kindMultiComplex,
		valueKey:  scimFormattedKey,
	},
	{
		candidate:   "street_address",
		kind:        kindMultiComplexPart,
		parentField: scimFieldAddresses,
		subAttr:     "streetAddress",
	},
	{
		candidate:   "locality",
		kind:        kindMultiComplexPart,
		parentField: scimFieldAddresses,
		subAttr:     "locality",
	},
	{
		candidate:   "region",
		kind:        kindMultiComplexPart,
		parentField: scimFieldAddresses,
		subAttr:     "region",
	},
	{
		candidate:   "postal_code",
		kind:        kindMultiComplexPart,
		parentField: scimFieldAddresses,
		subAttr:     "postalCode",
	},
	{
		candidate:   "country",
		kind:        kindMultiComplexPart,
		parentField: scimFieldAddresses,
		subAttr:     "country",
	},
}

// scimToThunderAttrIndex is a pre-built, lowercase-keyed reverse lookup of
// coreAttrRules, mapping a SCIM filter attribute path (e.g. "username",
// "emails.value", "name.givenname", "addresses.streetaddress") to its
// ThunderID attribute name. Built once at package init so filter translation
// is an O(1) map lookup instead of a per-request scan of coreAttrRules.
var scimToThunderAttrIndex = buildSCIMToThunderAttrIndex()

// buildSCIMToThunderAttrIndex constructs the lowercase-keyed lookup map from SCIM attribute paths to internal
// ThunderID attribute names.
func buildSCIMToThunderAttrIndex() map[string]string {
	index := make(map[string]string)
	for _, rule := range coreAttrRules {
		switch rule.kind {
		case kindSimpleString:
			index[strings.ToLower(string(rule.scimField))] = rule.candidate
		case kindMultiComplex:
			index[strings.ToLower(string(rule.scimField))] = rule.candidate
			valueKey := rule.valueKey
			if valueKey == "" {
				valueKey = scimValueKey
			}
			index[strings.ToLower(string(rule.scimField)+"."+valueKey)] = rule.candidate
		case kindSubAttr, kindMultiComplexPart:
			index[strings.ToLower(string(rule.parentField)+"."+rule.subAttr)] = rule.candidate
		}
	}
	return index
}

// translateSCIMFilterAttr translates a SCIM filter attribute path into its
// ThunderID attribute name using scimToThunderAttrIndex. Attributes with no
// core mapping (id, active, custom schema attributes already in ThunderID
// naming, etc.) are passed through unchanged.
func translateSCIMFilterAttr(attr string) string {
	if thunderAttr, ok := scimToThunderAttrIndex[strings.ToLower(attr)]; ok {
		return thunderAttr
	}
	return attr
}

// scimUnsupportedMultiComplexSubAttrs: sub-attrs ThunderID never stores as a flat
// field, regardless of scalar/array schema. "value" excluded — wrong only for
// array schemas, undetectable at filter-parse time.
var scimUnsupportedMultiComplexSubAttrs = []string{scimTypeKey, scimPrimaryKey}

// scimUnsupportedFilterAttrs: filter paths on multi-valued core attrs (emails,
// phoneNumbers, photos) with no flat equivalent to compare against, e.g.
// "emails.type". Filtering these errors instead of silently matching nothing.
var scimUnsupportedFilterAttrs = buildSCIMUnsupportedFilterAttrs()

// buildSCIMUnsupportedFilterAttrs builds a set of filter paths for multi-valued complex attributes that are
// unsupported for direct comparison.
func buildSCIMUnsupportedFilterAttrs() map[string]struct{} {
	unsupported := make(map[string]struct{})
	for _, rule := range coreAttrRules {
		if rule.kind != kindMultiComplex {
			continue
		}
		for _, subAttr := range scimUnsupportedMultiComplexSubAttrs {
			unsupported[strings.ToLower(string(rule.scimField)+"."+subAttr)] = struct{}{}
		}
	}
	return unsupported
}

// isUnsupportedSCIMFilterAttr reports whether attr is a recognized but
// currently unsupported SCIM filter attribute path (see
// scimUnsupportedFilterAttrs).
func isUnsupportedSCIMFilterAttr(attr string) bool {
	_, ok := scimUnsupportedFilterAttrs[strings.ToLower(attr)]
	return ok
}

// multiComplexPartRules returns the kindMultiComplexPart rules that roll into
// field's single entry (e.g. streetAddress/locality/etc for scimFieldAddresses).
// Not addresses-specific: any kindMultiComplex field gains part-merging behavior
// simply by having rules declare it as their parentField.
func multiComplexPartRules(field scimCoreField) []coreAttrRule {
	var out []coreAttrRule
	for _, rule := range coreAttrRules {
		if rule.kind == kindMultiComplexPart && rule.parentField == field {
			out = append(out, rule)
		}
	}
	return out
}

// canonicalMultiComplexSubAttrs returns the set of keys allowed to survive
// normalizeAndTranslateMultiComplexOutbound's filtering: the meta
// keys (type/primary), the field's own value key (e.g. "formatted" for
// addresses), and every part rule's subAttr/candidate name.
func canonicalMultiComplexSubAttrs(valueKey string, parts []coreAttrRule) map[string]struct{} {
	m := map[string]struct{}{scimTypeKey: {}, scimPrimaryKey: {}}
	if valueKey == "" {
		valueKey = scimValueKey
	}
	m[strings.ToLower(valueKey)] = struct{}{}
	for _, rule := range parts {
		m[strings.ToLower(rule.subAttr)] = struct{}{}
		m[strings.ToLower(rule.candidate)] = struct{}{}
	}
	return m
}

// normalizeAndTranslateMultiComplexOutbound filters and renames each entry of
// arr (already produced by normalizeToMultiComplex for field) down to
// field's canonical sub-attributes, translating part-rule candidate names
// (e.g. "street_address") to their SCIM subAttr names (e.g. "streetAddress").
// A no-op passthrough for any field with no declared part rules (emails,
// phoneNumbers, photos, ...).
func normalizeAndTranslateMultiComplexOutbound(
	field scimCoreField, valueKey string, arr []map[string]interface{},
) []map[string]interface{} {
	parts := multiComplexPartRules(field)
	if len(parts) == 0 {
		return arr
	}
	if len(arr) == 0 {
		return nil
	}
	canonical := canonicalMultiComplexSubAttrs(valueKey, parts)
	var out []map[string]interface{}
	for _, obj := range arr {
		newObj := make(map[string]interface{})
		for k, v := range obj {
			if _, ok := canonical[strings.ToLower(k)]; ok {
				newObj[k] = v
			}
		}
		for _, rule := range parts {
			if v, ok := newObj[rule.candidate]; ok {
				newObj[rule.subAttr] = v
				if rule.candidate != rule.subAttr {
					delete(newObj, rule.candidate)
				}
			}
		}
		nonMetaCount := 0
		for k := range newObj {
			if k != scimTypeKey && k != scimPrimaryKey {
				nonMetaCount++
			}
		}
		if nonMetaCount > 0 {
			out = append(out, newObj)
		}
	}
	return out
}

// translateMultiComplexPartsInboundForSchema renames obj's SCIM subAttr keys
// (e.g. "streetAddress") back to whatever name field's part rules' candidate
// matches in the target usertype schema (e.g. "street_address"), so a schema
// declaring its own attribute names round-trips correctly. A no-op passthrough
// for any field with no declared part rules.
func translateMultiComplexPartsInboundForSchema(
	field scimCoreField, obj map[string]interface{}, propDef rawPropertyDef,
) map[string]interface{} {
	parts := multiComplexPartRules(field)
	if len(parts) == 0 {
		return obj
	}
	targetProps := propDef.Properties
	if targetProps == nil && propDef.Items != nil {
		targetProps = propDef.Items.Properties
	}
	out := make(map[string]interface{})
	for k, v := range obj {
		out[k] = v
	}
	for _, rule := range parts {
		if scimVal, ok := out[rule.subAttr]; ok {
			for targetProp := range targetProps {
				if strings.EqualFold(targetProp, rule.candidate) {
					out[targetProp] = scimVal
					if targetProp != rule.subAttr {
						delete(out, rule.subAttr)
					}
					break
				}
			}
		}
	}
	return out
}

// mapToCoreAttrs converts stored ThunderID user attributes into standard SCIM core attribute representations.
func mapToCoreAttrs(rawAttrs json.RawMessage) map[string]json.RawMessage {
	if len(rawAttrs) == 0 {
		return nil
	}
	var attrMap map[string]json.RawMessage
	if err := json.Unmarshal(rawAttrs, &attrMap); err != nil {
		return nil
	}
	result := make(map[string]json.RawMessage)
	parentObjs := make(map[scimCoreField]map[string]json.RawMessage)
	// multiComplexObjs accumulates every kindMultiComplex field's entries, and
	// multiPartAdds accumulates the kindMultiComplexPart values that roll into
	// entry 0, so both sources merge instead of overwriting each other.
	multiComplexObjs := make(map[scimCoreField][]map[string]interface{})
	multiPartAdds := make(map[scimCoreField]map[string]interface{})
	for _, rule := range coreAttrRules {
		val := findCandidateValue(attrMap, rule.candidate)
		if val == nil {
			continue
		}
		switch rule.kind {
		case kindSimpleString:
			if sv := extractStringValue(val, ""); sv != "" {
				b, _ := json.Marshal(sv)
				result[string(rule.scimField)] = b
			}
		case kindMultiComplex:
			arr := normalizeToMultiComplex(val, rule.valueKey)
			arr = normalizeAndTranslateMultiComplexOutbound(rule.scimField, rule.valueKey, arr)
			if len(arr) > 0 {
				multiComplexObjs[rule.scimField] = arr
			}
		case kindSubAttr:
			if sv := extractStringValue(val, ""); sv != "" {
				if parentObjs[rule.parentField] == nil {
					parentObjs[rule.parentField] = make(map[string]json.RawMessage)
				}
				b, _ := json.Marshal(sv)
				parentObjs[rule.parentField][rule.subAttr] = b
			}
		case kindMultiComplexPart:
			if sv := extractStringValue(val, ""); sv != "" {
				if multiPartAdds[rule.parentField] == nil {
					multiPartAdds[rule.parentField] = make(map[string]interface{})
				}
				multiPartAdds[rule.parentField][rule.subAttr] = sv
			}
		}
	}
	for parent, obj := range parentObjs {
		b, _ := json.Marshal(obj)
		result[string(parent)] = b
	}
	for field, adds := range multiPartAdds {
		if len(adds) == 0 {
			continue
		}
		if len(multiComplexObjs[field]) == 0 {
			multiComplexObjs[field] = []map[string]interface{}{{}}
		}
		for k, v := range adds {
			multiComplexObjs[field][0][k] = v
		}
		multiComplexObjs[field][0][scimPrimaryKey] = true
	}
	for field, arr := range multiComplexObjs {
		if len(arr) == 0 {
			continue
		}
		b, _ := json.Marshal(arr)
		result[string(field)] = b
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// reverseMapCoreAttrsForSchema converts incoming SCIM core attributes into internal user-type attributes
// based on the target schema.
func reverseMapCoreAttrsForSchema(coreAttrs map[string]json.RawMessage,
	schema json.RawMessage) (map[string]json.RawMessage, error) {
	if len(coreAttrs) == 0 {
		return nil, nil
	}
	if len(schema) == 0 {
		return nil, nil
	}
	var rawProps map[string]rawPropertyDef
	if err := json.Unmarshal(schema, &rawProps); err != nil {
		return nil, err
	}
	if len(rawProps) == 0 {
		return nil, nil
	}

	result := make(map[string]json.RawMessage)

	for _, rule := range coreAttrRules {
		lookupField := reverseLookupField(rule)
		coreVal, _ := findCoreAttrValue(coreAttrs, lookupField)
		if len(coreVal) == 0 {
			continue
		}

		targetAttrName := findTargetAttrName(rawProps, rule.candidate)
		if targetAttrName == "" {
			continue
		}

		if b, ok := reverseMapRuleValue(rule, coreVal, rawProps[targetAttrName]); ok {
			result[targetAttrName] = b
		}
	}

	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

// findCoreAttrValue performs a case-insensitive search for targetField within the coreAttrs map.
func findCoreAttrValue(coreAttrs map[string]json.RawMessage, targetField scimCoreField) (json.RawMessage, string) {
	target := string(targetField)
	for k, v := range coreAttrs {
		if strings.EqualFold(k, target) {
			return v, k
		}
	}
	return nil, ""
}

// reverseLookupField returns the coreAttrs key to read for rule, based on its kind.
func reverseLookupField(rule coreAttrRule) scimCoreField {
	switch rule.kind {
	case kindSubAttr, kindMultiComplexPart:
		return rule.parentField
	default:
		return rule.scimField
	}
}

// findTargetAttrName finds the user-type schema property matching candidate, case-insensitively.
func findTargetAttrName(rawProps map[string]rawPropertyDef, candidate string) string {
	for propName := range rawProps {
		if strings.EqualFold(propName, candidate) {
			return propName
		}
	}
	return ""
}

// reverseMapRuleValue converts a single core attribute value back into its user-type
// schema representation, per rule.kind. ok is false when there is nothing to map.
func reverseMapRuleValue(rule coreAttrRule, coreVal json.RawMessage, propDef rawPropertyDef,
) (b json.RawMessage, ok bool) {
	switch rule.kind {
	case kindSimpleString:
		return reverseMapSimpleString(coreVal)
	case kindMultiComplex:
		return reverseMapMultiComplex(rule, coreVal, propDef)
	case kindSubAttr:
		return reverseMapSubAttr(rule, coreVal)
	case kindMultiComplexPart:
		return reverseMapMultiComplexPart(rule, coreVal)
	default:
		return nil, false
	}
}

// reverseMapSimpleString extracts and JSON-encodes a scalar string value from core attribute input.
func reverseMapSimpleString(coreVal json.RawMessage) (json.RawMessage, bool) {
	sv := extractStringValue(coreVal, "")
	if sv == "" {
		return nil, false
	}
	b, _ := json.Marshal(sv)
	return b, true
}

// reverseMapMultiComplex transforms a multi-valued complex SCIM attribute into the format required by the
// target schema property.
func reverseMapMultiComplex(rule coreAttrRule, coreVal json.RawMessage, propDef rawPropertyDef,
) (json.RawMessage, bool) {
	normalized := normalizeToMultiComplex(coreVal, rule.valueKey)
	if len(normalized) == 0 {
		return nil, false
	}
	for i, obj := range normalized {
		normalized[i] = translateMultiComplexPartsInboundForSchema(rule.scimField, obj, propDef)
	}
	propType := strings.ToLower(propDef.Type)
	switch {
	case propType == rawPropertyTypeArray && propDef.Items != nil &&
		strings.ToLower(propDef.Items.Type) == rawPropertyTypeObject:
		// Schema wants full objects — keep every entry, value/type/primary included.
		b, _ := json.Marshal(normalized)
		return b, true
	case propType == rawPropertyTypeArray:
		// Schema wants a plain array (string/number items) — project every entry's value.
		vals := make([]string, 0, len(normalized))
		for _, obj := range normalized {
			if v, ok := obj[rule.valueKey].(string); ok && v != "" {
				vals = append(vals, v)
			}
		}
		if len(vals) == 0 {
			return nil, false
		}
		b, _ := json.Marshal(vals)
		return b, true
	case propType == rawPropertyTypeObject:
		// Schema wants a single object — first entry, value/type/primary included, no array wrapper.
		b, _ := json.Marshal(normalized[0])
		return b, true
	default:
		// Scalar schema — first entry's value only.
		v, ok := normalized[0][rule.valueKey].(string)
		if !ok || v == "" {
			return nil, false
		}
		b, _ := json.Marshal(v)
		return b, true
	}
}

// reverseMapSubAttr extracts a specific sub-attribute from a complex parent object in core attribute input.
func reverseMapSubAttr(rule coreAttrRule, coreVal json.RawMessage) (json.RawMessage, bool) {
	var subObjMap map[string]json.RawMessage
	if err := json.Unmarshal(coreVal, &subObjMap); err != nil {
		return nil, false
	}
	subVal, exists := subObjMap[rule.subAttr]
	if !exists {
		return nil, false
	}
	sv := extractStringValue(subVal, "")
	if sv == "" {
		return nil, false
	}
	b, _ := json.Marshal(sv)
	return b, true
}

// reverseMapMultiComplexPart extracts a specific address part from a multi-complex entry.
func reverseMapMultiComplexPart(rule coreAttrRule, coreVal json.RawMessage) (json.RawMessage, bool) {
	var entryArr []map[string]json.RawMessage
	if err := json.Unmarshal(coreVal, &entryArr); err != nil || len(entryArr) == 0 {
		return nil, false
	}
	subVal, exists := entryArr[0][rule.subAttr]
	if !exists {
		return nil, false
	}
	sv := extractStringValue(subVal, "")
	if sv == "" {
		return nil, false
	}
	b, _ := json.Marshal(sv)
	return b, true
}

// findCandidateValue finds the attribute value in m matching candidate, case-insensitively.
func findCandidateValue(m map[string]json.RawMessage, candidate string) json.RawMessage {
	for k, v := range m {
		if strings.EqualFold(k, candidate) {
			return v
		}
	}
	return nil
}

// extractStringValue extracts a string from raw JSON, handling scalars, objects, and single-element arrays.
func extractStringValue(raw json.RawMessage, targetKey string) string {
	if len(raw) == 0 {
		return ""
	}
	if targetKey == "" {
		targetKey = scimValueKey
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err == nil {
		if v, ok := obj[targetKey]; ok {
			return extractStringValue(v, targetKey)
		}
		// Fallback for address mapping robustness
		if targetKey != scimValueKey {
			if v, ok := obj[scimValueKey]; ok {
				return extractStringValue(v, targetKey)
			}
		}
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		return extractStringValue(arr[0], targetKey)
	}
	return ""
}

// normalizeToMultiComplex normalizes various client input shapes into a standard array of complex maps.
func normalizeToMultiComplex(raw json.RawMessage, valueKey string) []map[string]interface{} {
	if len(raw) == 0 {
		return nil
	}
	if valueKey == "" {
		valueKey = scimValueKey
	}

	// Case 1: Plain string -> wrap in array
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return []map[string]interface{}{{valueKey: s, "primary": true}}
	}

	// Case 2: Array of strings
	var strArr []string
	if err := json.Unmarshal(raw, &strArr); err == nil {
		var out []map[string]interface{}
		for i, val := range strArr {
			out = append(out, map[string]interface{}{valueKey: val, "primary": i == 0})
		}
		return out
	}

	// Case 3: Array of objects
	var objArr []map[string]interface{}
	if err := json.Unmarshal(raw, &objArr); err == nil {
		return normalizeMultiComplexObjectArray(objArr, valueKey)
	}

	// Case 4: Single object -> wrap in array
	var singleObj map[string]interface{}
	if err := json.Unmarshal(raw, &singleObj); err == nil {
		return normalizeMultiComplexSingleObject(singleObj, valueKey)
	}
	return nil
}

// multiComplexValue extracts the value (falling back to the default "value" key) and
// non-meta-attribute count (keys other than type/primary) shared by a multi-valued
// complex object's normalization, whether it came from an array entry or a lone object.
func multiComplexValue(obj map[string]interface{}, valueKey string) (valStr string, nonMetaCount int) {
	valStr, _ = obj[valueKey].(string)
	if valStr == "" && valueKey != scimValueKey {
		valStr, _ = obj[scimValueKey].(string)
	}
	for k := range obj {
		if k != scimTypeKey && k != scimPrimaryKey {
			nonMetaCount++
		}
	}
	return valStr, nonMetaCount
}

// normalizeMultiComplexEntry copies obj, fills in valueKey when valStr is present, and
// drops an empty "type" rather than fabricating one.
func normalizeMultiComplexEntry(obj map[string]interface{}, valueKey, valStr string) map[string]interface{} {
	newObj := make(map[string]interface{})
	for k, v := range obj {
		newObj[k] = v
	}
	if valStr != "" {
		newObj[valueKey] = valStr
	}
	if typStr, _ := obj[scimTypeKey].(string); typStr == "" {
		delete(newObj, scimTypeKey)
	}
	return newObj
}

// normalizeMultiComplexObjectArray normalizes an array of object maps, ensuring valid content and a primary element.
func normalizeMultiComplexObjectArray(objArr []map[string]interface{}, valueKey string) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(objArr))
	for _, obj := range objArr {
		if len(obj) == 0 {
			continue
		}
		valStr, nonMetaCount := multiComplexValue(obj, valueKey)
		if valStr == "" && nonMetaCount == 0 {
			// Object only has a "type" or "primary" field with no content keys — skip it.
			continue
		}
		out = append(out, normalizeMultiComplexEntry(obj, valueKey, valStr))
	}
	if len(out) > 0 && !hasPrimary(out) {
		out[0]["primary"] = true
	}
	return out
}

// normalizeMultiComplexSingleObject wraps a single object map into a one-element slice with primary set to true.
func normalizeMultiComplexSingleObject(singleObj map[string]interface{}, valueKey string) []map[string]interface{} {
	if len(singleObj) == 0 {
		return nil
	}
	valStr, nonMetaCount := multiComplexValue(singleObj, valueKey)
	if valStr == "" && nonMetaCount == 0 {
		return nil
	}
	newObj := normalizeMultiComplexEntry(singleObj, valueKey, valStr)
	newObj["primary"] = true
	return []map[string]interface{}{newObj}
}

// hasPrimary reports whether any map in arr has a primary boolean property set to true.
func hasPrimary(arr []map[string]interface{}) bool {
	for _, obj := range arr {
		if p, ok := obj["primary"].(bool); ok && p {
			return true
		}
	}
	return false
}
