// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package users

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	entitytypemodel "github.com/thunder-id/thunderid/internal/entitytype/model"
	scim "github.com/thunder-id/thunderid/internal/scim/common"
)

const (
	scimValueKey     = "value"
	scimTypeKey      = "type"
	scimPrimaryKey   = "primary"
	scimFormattedKey = "formatted"
)

// ============================================================================
// Section 1: SCIM Filter and Query Path Rules
// Handles translating and validating attribute paths for ?filter= and ?attributes=
// ============================================================================

// scimToThunderAttrIndex is a pre-built, lowercase-keyed map from SCIM filter
// attribute paths to internal ThunderID attribute names.
var scimToThunderAttrIndex = buildSCIMToThunderAttrIndex()

// buildSCIMToThunderAttrIndex builds the lookup map from SCIM attribute paths to ThunderID attribute names.
func buildSCIMToThunderAttrIndex() map[string]string {
	index := make(map[string]string)
	for _, rule := range scim.CoreAttrRules {
		switch rule.Kind {
		case scim.KindSimpleString:
			index[strings.ToLower(string(rule.SCIMField))] = rule.Candidate
		case scim.KindMultiComplex:
			index[strings.ToLower(string(rule.SCIMField))] = rule.Candidate
			valueKey := rule.ValueKey
			if valueKey == "" {
				valueKey = scimValueKey
			}
			index[strings.ToLower(string(rule.SCIMField)+"."+valueKey)] = rule.Candidate
		case scim.KindSubAttr, scim.KindMultiComplexPart:
			index[strings.ToLower(string(rule.ParentField)+"."+rule.SubAttr)] = rule.Candidate
		}
	}
	for _, rule := range scim.EnterpriseAttrRules {
		if rule.SCIMField == scim.EnterpriseFieldManager {
			index[strings.ToLower(string(rule.SCIMField)+".value")] = rule.Candidate
		} else {
			index[strings.ToLower(string(rule.SCIMField))] = rule.Candidate
		}
	}
	return index
}

// translateSCIMFilterAttr translates a SCIM filter attribute path to a ThunderID attribute name.
func translateSCIMFilterAttr(attr string) string {
	if thunderAttr, ok := scimToThunderAttrIndex[strings.ToLower(attr)]; ok {
		return thunderAttr
	}
	return attr
}

// scimCoreFilterAttrs is the set of recognized SCIM core User schema attribute paths (lowercase).
var scimCoreFilterAttrs = buildSCIMCoreFilterAttrs()

// buildSCIMCoreFilterAttrs builds the recognized-core-attribute set used by isCoreSCIMFilterAttr.
func buildSCIMCoreFilterAttrs() map[string]struct{} {
	set := make(map[string]struct{})
	for _, rule := range scim.CoreAttrRules {
		switch rule.Kind {
		case scim.KindSimpleString:
			set[strings.ToLower(string(rule.SCIMField))] = struct{}{}
		case scim.KindMultiComplex:
			set[strings.ToLower(string(rule.SCIMField))] = struct{}{}
			valueKey := rule.ValueKey
			if valueKey == "" {
				valueKey = scimValueKey
			}
			set[strings.ToLower(string(rule.SCIMField)+"."+valueKey)] = struct{}{}
		case scim.KindSubAttr, scim.KindMultiComplexPart:
			set[strings.ToLower(string(rule.ParentField)+"."+rule.SubAttr)] = struct{}{}
		}
	}
	set["id"] = struct{}{}
	return set
}

// isCoreSCIMFilterAttr reports whether attr is a recognized core User schema attribute
// path, as opposed to a custom/extension attribute that requires a schema URN prefix.
func isCoreSCIMFilterAttr(attr string) bool {
	_, ok := scimCoreFilterAttrs[strings.ToLower(attr)]
	return ok
}

// scimUnsupportedMultiComplexSubAttrs lists the multi-valued complex sub-attributes unsupported as filter targets.
var scimUnsupportedMultiComplexSubAttrs = []string{scimTypeKey, scimPrimaryKey}

// scimUnsupportedFilterAttrs is the set of SCIM filter paths that are recognized but not supported for comparison.
var scimUnsupportedFilterAttrs = buildSCIMUnsupportedFilterAttrs()

// buildSCIMUnsupportedFilterAttrs builds a set of filter paths for multi-valued complex attributes
// and unmapped core attributes that are unsupported for direct comparison.
func buildSCIMUnsupportedFilterAttrs() map[string]struct{} {
	unsupported := make(map[string]struct{})
	for _, rule := range scim.CoreAttrRules {
		if rule.Kind != scim.KindMultiComplex {
			continue
		}
		for _, subAttr := range scimUnsupportedMultiComplexSubAttrs {
			unsupported[strings.ToLower(string(rule.SCIMField)+"."+subAttr)] = struct{}{}
		}
	}
	unsupported["active"] = struct{}{}
	unsupported["externalid"] = struct{}{}
	unsupported["usertype"] = struct{}{}
	unsupported["manager.$ref"] = struct{}{}
	return unsupported
}

// isUnsupportedSCIMFilterAttr reports whether attr is a recognized but unsupported SCIM filter attribute path.
func isUnsupportedSCIMFilterAttr(attr string) bool {
	_, ok := scimUnsupportedFilterAttrs[strings.ToLower(attr)]
	return ok
}

// usersFilterAttrRules wires the Users schema attribute checks into the shared SCIM filter parser.
var usersFilterAttrRules = scim.FilterAttrRules{
	IsUnsupported: isUnsupportedSCIMFilterAttr,
	IsCore:        isCoreSCIMFilterAttr,
	Translate:     translateSCIMFilterAttr,
}

// scimCoreTopLevelAttrs is the set of top-level SCIM core User schema attribute names (lowercase).
var scimCoreTopLevelAttrs = buildSCIMCoreTopLevelAttrs()

// buildSCIMCoreTopLevelAttrs builds scimCoreTopLevelAttrs from scim.CoreAttrRules plus the
// envelope attributes that have no entry there (id, meta, schemas).
func buildSCIMCoreTopLevelAttrs() map[string]struct{} {
	set := map[string]struct{}{
		"id": {}, "meta": {}, "schemas": {},
	}
	for _, rule := range scim.CoreAttrRules {
		switch {
		case rule.SCIMField != "":
			set[strings.ToLower(string(rule.SCIMField))] = struct{}{}
		case rule.ParentField != "":
			set[strings.ToLower(string(rule.ParentField))] = struct{}{}
		}
	}
	return set
}

// isCoreSCIMAttrPath reports whether attr's root segment belongs to the core User schema.
func isCoreSCIMAttrPath(attr string) bool {
	root := attr
	if idx := strings.IndexByte(attr, '.'); idx >= 0 {
		root = attr[:idx]
	}
	_, ok := scimCoreTopLevelAttrs[strings.ToLower(root)]
	return ok
}

// scimEnterpriseTopLevelAttrs is the set of top-level SCIM Enterprise User schema attribute names (lowercase).
var scimEnterpriseTopLevelAttrs = buildSCIMEnterpriseTopLevelAttrs()

// buildSCIMEnterpriseTopLevelAttrs builds scimEnterpriseTopLevelAttrs from scim.EnterpriseAttrRules.
func buildSCIMEnterpriseTopLevelAttrs() map[string]struct{} {
	set := make(map[string]struct{}, len(scim.EnterpriseAttrRules))
	for _, rule := range scim.EnterpriseAttrRules {
		set[strings.ToLower(string(rule.SCIMField))] = struct{}{}
	}
	return set
}

// isEnterpriseSCIMAttrPath reports whether attr's root segment belongs to the Enterprise User schema.
func isEnterpriseSCIMAttrPath(attr string) bool {
	root := attr
	if idx := strings.IndexByte(attr, '.'); idx >= 0 {
		root = attr[:idx]
	}
	_, ok := scimEnterpriseTopLevelAttrs[strings.ToLower(root)]
	return ok
}

// ============================================================================
// Section 2: Outbound Mapping (ThunderID Attributes -> SCIM Core Attributes)
// Used when building SCIM User resource representations for GET responses.
// ============================================================================

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
	parentObjs := make(map[scim.CoreField]map[string]json.RawMessage)
	// multiComplexObjs accumulates every scim.KindMultiComplex field's entries, and
	// multiPartAdds accumulates the scim.KindMultiComplexPart values that roll into
	// entry 0, so both sources merge instead of overwriting each other.
	multiComplexObjs := make(map[scim.CoreField][]map[string]interface{})
	multiPartAdds := make(map[scim.CoreField]map[string]interface{})
	for _, rule := range scim.CoreAttrRules {
		rawVal := findCandidateValue(attrMap, rule.Candidate)
		if rawVal == nil {
			continue
		}
		switch rule.Kind {
		case scim.KindSimpleString:
			if sv := extractStringValue(rawVal, ""); sv != "" {
				b, _ := json.Marshal(sv)
				result[string(rule.SCIMField)] = b
			}
		case scim.KindMultiComplex:
			arr := normalizeToMultiComplex(rawVal, rule.ValueKey)
			if parts := multiComplexPartRules(rule.SCIMField); len(parts) > 0 {
				arr = filterAndRenameMultiComplexParts(rule.ValueKey, parts, arr)
			}
			if len(arr) > 0 {
				multiComplexObjs[rule.SCIMField] = arr
			}
		case scim.KindSubAttr:
			if sv := extractStringValue(rawVal, ""); sv != "" {
				if parentObjs[rule.ParentField] == nil {
					parentObjs[rule.ParentField] = make(map[string]json.RawMessage)
				}
				b, _ := json.Marshal(sv)
				parentObjs[rule.ParentField][rule.SubAttr] = b
			}
		case scim.KindMultiComplexPart:
			if sv := extractStringValue(rawVal, ""); sv != "" {
				if multiPartAdds[rule.ParentField] == nil {
					multiPartAdds[rule.ParentField] = make(map[string]interface{})
				}
				multiPartAdds[rule.ParentField][rule.SubAttr] = sv
			}
		}
	}
	for parent, obj := range parentObjs {
		b, _ := json.Marshal(obj)
		result[string(parent)] = b
	}
	for field, adds := range multiPartAdds {
		if len(multiComplexObjs[field]) == 0 {
			multiComplexObjs[field] = []map[string]interface{}{{}}
		}
		for k, v := range adds {
			multiComplexObjs[field][0][k] = v
		}
		multiComplexObjs[field][0][scimPrimaryKey] = true
	}
	for field, arr := range multiComplexObjs {
		b, _ := json.Marshal(arr)
		result[string(field)] = b
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// mapToEnterpriseAttrs builds the SCIM Enterprise User extension object from stored ThunderID attributes.
func mapToEnterpriseAttrs(rawAttrs json.RawMessage, baseURL string) json.RawMessage {
	if len(rawAttrs) == 0 {
		return nil
	}
	var attrMap map[string]json.RawMessage
	if err := json.Unmarshal(rawAttrs, &attrMap); err != nil {
		return nil
	}

	result := make(map[string]interface{})

	for _, rule := range scim.EnterpriseAttrRules {
		rawVal := findCandidateValue(attrMap, rule.Candidate)
		if rawVal == nil {
			continue
		}
		if rule.SCIMField == scim.EnterpriseFieldManager {
			managerID := extractStringValue(rawVal, "")
			if managerID != "" {
				managerObj := map[string]interface{}{
					"value": managerID,
				}
				if baseURL != "" {
					managerObj["$ref"] = fmt.Sprintf("%s%s/Users/%s", baseURL, scim.SCIMBasePath, managerID)
				}
				result[string(scim.EnterpriseFieldManager)] = managerObj
			}
		} else {
			strVal := extractStringValue(rawVal, "")
			if strVal != "" {
				result[string(rule.SCIMField)] = strVal
			}
		}
	}

	if len(result) == 0 {
		return nil
	}
	b, err := json.Marshal(result)
	if err != nil {
		return nil
	}
	return b
}

// multiComplexPartRules returns all KindMultiComplexPart rules declared for the given field.
func multiComplexPartRules(field scim.CoreField) []scim.CoreAttrRule {
	var out []scim.CoreAttrRule
	for _, rule := range scim.CoreAttrRules {
		if rule.Kind == scim.KindMultiComplexPart && rule.ParentField == field {
			out = append(out, rule)
		}
	}
	return out
}

// filterAndRenameMultiComplexParts filters each entry to the allowed sub-attributes and renames
// ThunderID candidate keys to their SCIM sub-attribute names.
func filterAndRenameMultiComplexParts(
	valueKey string, parts []scim.CoreAttrRule, arr []map[string]interface{},
) []map[string]interface{} {
	if valueKey == "" {
		valueKey = scimValueKey
	}
	allowedKeys := map[string]struct{}{scimTypeKey: {}, scimPrimaryKey: {}, strings.ToLower(valueKey): {}}
	rename := make(map[string]string, len(parts))
	for _, rule := range parts {
		allowedKeys[strings.ToLower(rule.SubAttr)] = struct{}{}
		allowedKeys[strings.ToLower(rule.Candidate)] = struct{}{}
		if rule.Candidate != rule.SubAttr {
			rename[rule.Candidate] = rule.SubAttr
		}
	}

	var out []map[string]interface{}
	for _, obj := range arr {
		newObj := make(map[string]interface{}, len(obj))
		for k, v := range obj {
			if newKey, ok := rename[k]; ok {
				newObj[newKey] = v
			} else if _, ok := allowedKeys[strings.ToLower(k)]; ok {
				newObj[k] = v
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

// ============================================================================
// Section 3: Inbound Reverse Mapping (SCIM Core Attributes -> ThunderID User Type)
// Used when saving SCIM User resource payloads for POST and PUT requests.
// ============================================================================

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
	var rawProps map[string]scim.RawPropertyDef
	if err := json.Unmarshal(schema, &rawProps); err != nil {
		return nil, err
	}
	if len(rawProps) == 0 {
		return nil, nil
	}

	result := make(map[string]json.RawMessage)

	for _, rule := range scim.CoreAttrRules {
		lookupField := reverseLookupField(rule)
		rawVal, _ := findCoreAttrValue(coreAttrs, lookupField)
		if len(rawVal) == 0 {
			continue
		}

		targetAttrName := findTargetAttrName(rawProps, rule.Candidate)
		if targetAttrName == "" {
			continue
		}

		if b, ok := reverseMapRuleValue(rule, rawVal, rawProps[targetAttrName]); ok {
			result[targetAttrName] = b
		}
	}

	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

// reverseMapEnterpriseAttrsForSchema maps incoming SCIM Enterprise attributes to user-type attributes.
// Unrecognized or undeclared attributes are returned separately in undeclared.
func reverseMapEnterpriseAttrsForSchema(
	enterpriseAttrs map[string]json.RawMessage, schema json.RawMessage,
) (mapped map[string]json.RawMessage, undeclared []string, err error) {
	if len(enterpriseAttrs) == 0 {
		return nil, nil, nil
	}
	rawProps, hasSchema, err := parseSchemaRawProps(schema)
	if err != nil || !hasSchema {
		return nil, nil, err
	}

	mapped = make(map[string]json.RawMessage)
	var undeclaredAttrsList []string

	for entKey, rawVal := range enterpriseAttrs {
		var matchedRule *scim.EnterpriseAttrRule
		for _, rule := range scim.EnterpriseAttrRules {
			if strings.EqualFold(string(rule.SCIMField), entKey) {
				ruleCopy := rule
				matchedRule = &ruleCopy
				break
			}
		}
		if matchedRule == nil {
			undeclaredAttrsList = append(undeclaredAttrsList, entKey)
			continue
		}

		targetAttrName := findTargetAttrName(rawProps, matchedRule.Candidate)
		if targetAttrName == "" {
			undeclaredAttrsList = append(undeclaredAttrsList, entKey)
			continue
		}

		var strVal string
		if matchedRule.SCIMField == scim.EnterpriseFieldManager {
			strVal = extractStringValue(rawVal, scimValueKey)
		} else {
			strVal = extractStringValue(rawVal, "")
		}

		if strVal != "" {
			b, _ := json.Marshal(strVal)
			mapped[targetAttrName] = b
		}
	}

	if len(undeclaredAttrsList) > 0 {
		sort.Strings(undeclaredAttrsList)
		return nil, undeclaredAttrsList, nil
	}

	if len(mapped) == 0 {
		return nil, nil, nil
	}
	return mapped, nil, nil
}

// findCoreAttrValue performs a case-insensitive search for targetField within the coreAttrs map.
func findCoreAttrValue(coreAttrs map[string]json.RawMessage, targetField scim.CoreField) (json.RawMessage, string) {
	target := string(targetField)
	for k, v := range coreAttrs {
		if strings.EqualFold(k, target) {
			return v, k
		}
	}
	return nil, ""
}

// reverseLookupField returns the coreAttrs key to read for rule, based on its kind.
func reverseLookupField(rule scim.CoreAttrRule) scim.CoreField {
	switch rule.Kind {
	case scim.KindSubAttr, scim.KindMultiComplexPart:
		return rule.ParentField
	default:
		return rule.SCIMField
	}
}

// findTargetAttrName finds the user-type schema property matching candidate, case-insensitively.
func findTargetAttrName(rawProps map[string]scim.RawPropertyDef, candidate string) string {
	for propName := range rawProps {
		if strings.EqualFold(propName, candidate) {
			return propName
		}
	}
	return ""
}

// reverseMapRuleValue converts a single core attribute value to its user-type schema representation per rule.Kind.
func reverseMapRuleValue(rule scim.CoreAttrRule, rawVal json.RawMessage, propDef scim.RawPropertyDef,
) (b json.RawMessage, ok bool) {
	switch rule.Kind {
	case scim.KindSimpleString:
		return reverseMapSimpleString(rawVal)
	case scim.KindMultiComplex:
		return reverseMapMultiComplex(rule, rawVal, propDef)
	case scim.KindSubAttr:
		return reverseMapSubAttr(rule, rawVal)
	case scim.KindMultiComplexPart:
		return reverseMapMultiComplexPart(rule, rawVal)
	default:
		return nil, false
	}
}

// reverseMapSimpleString extracts and JSON-encodes a scalar string value from core attribute input.
func reverseMapSimpleString(rawVal json.RawMessage) (json.RawMessage, bool) {
	sv := extractStringValue(rawVal, "")
	if sv == "" {
		return nil, false
	}
	b, _ := json.Marshal(sv)
	return b, true
}

// reverseMapMultiComplex converts a multi-valued complex SCIM attribute to the target schema format.
func reverseMapMultiComplex(rule scim.CoreAttrRule, rawVal json.RawMessage, propDef scim.RawPropertyDef,
) (json.RawMessage, bool) {
	normalized := normalizeToMultiComplex(rawVal, rule.ValueKey)
	if len(normalized) == 0 {
		return nil, false
	}
	if parts := multiComplexPartRules(rule.SCIMField); len(parts) > 0 {
		for i, obj := range normalized {
			normalized[i] = renameMultiComplexPartsInbound(parts, obj, propDef)
		}
	}
	propType := strings.ToLower(propDef.Type)
	switch {
	case propType == entitytypemodel.TypeArray && propDef.Items != nil &&
		strings.ToLower(propDef.Items.Type) == entitytypemodel.TypeObject:
		// Schema wants full objects — keep every entry, value/type/primary included.
		b, _ := json.Marshal(normalized)
		return b, true
	case propType == entitytypemodel.TypeArray:
		// Schema wants a plain array (string/number items) — project every entry's value.
		vals := make([]string, 0, len(normalized))
		for _, obj := range normalized {
			if v, ok := obj[rule.ValueKey].(string); ok && v != "" {
				vals = append(vals, v)
			}
		}
		if len(vals) == 0 {
			return nil, false
		}
		b, _ := json.Marshal(vals)
		return b, true
	case propType == entitytypemodel.TypeObject:
		// Schema wants a single object — first entry, value/type/primary included, no array wrapper.
		b, _ := json.Marshal(normalized[0])
		return b, true
	default:
		// Scalar schema — first entry's value only.
		v, ok := normalized[0][rule.ValueKey].(string)
		if !ok || v == "" {
			return nil, false
		}
		b, _ := json.Marshal(v)
		return b, true
	}
}

// renameMultiComplexPartsInbound renames SCIM sub-attribute keys in obj to match the target user-type schema names.
func renameMultiComplexPartsInbound(
	parts []scim.CoreAttrRule, obj map[string]interface{}, propDef scim.RawPropertyDef,
) map[string]interface{} {
	targetProps := propDef.Properties
	if targetProps == nil && propDef.Items != nil {
		targetProps = propDef.Items.Properties
	}
	for _, rule := range parts {
		if scimVal, ok := obj[rule.SubAttr]; ok {
			for targetProp := range targetProps {
				if strings.EqualFold(targetProp, rule.Candidate) {
					obj[targetProp] = scimVal
					if targetProp != rule.SubAttr {
						delete(obj, rule.SubAttr)
					}
					break
				}
			}
		}
	}
	return obj
}

// reverseMapSubAttr extracts a specific sub-attribute from a complex parent object in core attribute input.
func reverseMapSubAttr(rule scim.CoreAttrRule, rawVal json.RawMessage) (json.RawMessage, bool) {
	var subObjMap map[string]json.RawMessage
	if err := json.Unmarshal(rawVal, &subObjMap); err != nil {
		return nil, false
	}
	subVal, exists := subObjMap[rule.SubAttr]
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
func reverseMapMultiComplexPart(rule scim.CoreAttrRule, rawVal json.RawMessage) (json.RawMessage, bool) {
	var entryArr []map[string]json.RawMessage
	if err := json.Unmarshal(rawVal, &entryArr); err != nil || len(entryArr) == 0 {
		return nil, false
	}
	subVal, exists := entryArr[0][rule.SubAttr]
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

// ============================================================================
// Section 4: Shared JSON Extraction and Normalization Utilities
// Low-level helper routines used across mapping directions.
// ============================================================================

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
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return strconv.FormatBool(b)
	}
	var num json.Number
	if err := json.Unmarshal(raw, &num); err == nil {
		return num.String()
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

// multiComplexValue returns the string value and the count of non-meta keys in a multi-valued complex object.
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

// normalizeMultiComplexEntry copies obj, sets valueKey when valStr is non-empty, and drops an empty "type" key.
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
