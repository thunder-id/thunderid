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

const scimValueKey = "value"

type attrKind string

const (
	kindSimpleString attrKind = "simpleString" // userName, displayName, title, etc.
	kindMultiComplex attrKind = "multiComplex" // emails, phoneNumbers, photos
	kindSubAttr      attrKind = "subAttr"      // sub-attribute of a complex parent object, e.g. name.givenName
	kindAddrPart     attrKind = "addrPart"     // sub-attribute of the single addresses[0] element
)

type coreAttrRule struct {
	candidate   string        // ThunderID attr name, matched case-insensitively
	scimField   scimCoreField // target top-level SCIM field (simpleString/multiComplex only)
	kind        attrKind
	parentField scimCoreField // only for kindSubAttr; top-level complex object this rolls into, e.g. "name"
	subAttr     string        // only for kindSubAttr/kindAddrPart; key within the parent/addresses object
	defaultType string        // only for kindMultiComplex; e.g. "work"
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
		candidate:   "email",
		scimField:   scimFieldEmails,
		kind:        kindMultiComplex,
		defaultType: "work",
		valueKey:    scimValueKey,
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
		candidate:   "phone_number",
		scimField:   scimFieldPhoneNumbers,
		kind:        kindMultiComplex,
		defaultType: "work",
		valueKey:    scimValueKey,
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
		candidate:   "picture",
		scimField:   scimFieldPhotos,
		kind:        kindMultiComplex,
		defaultType: "photo",
		valueKey:    scimValueKey,
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
		candidate: "street_address",
		kind:      kindAddrPart,
		subAttr:   "streetAddress",
	},
	{
		candidate: "locality",
		kind:      kindAddrPart,
		subAttr:   "locality",
	},
	{
		candidate: "region",
		kind:      kindAddrPart,
		subAttr:   "region",
	},
	{
		candidate: "postal_code",
		kind:      kindAddrPart,
		subAttr:   "postalCode",
	},
	{
		candidate: "country",
		kind:      kindAddrPart,
		subAttr:   "country",
	},
}

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
	addrParts := make(map[string]json.RawMessage)
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
			arr := normalizeToMultiComplex(val, rule.defaultType, rule.valueKey)
			if len(arr) > 0 {
				b, _ := json.Marshal(arr)
				result[string(rule.scimField)] = b
			}
		case kindSubAttr:
			if sv := extractStringValue(val, ""); sv != "" {
				if parentObjs[rule.parentField] == nil {
					parentObjs[rule.parentField] = make(map[string]json.RawMessage)
				}
				b, _ := json.Marshal(sv)
				parentObjs[rule.parentField][rule.subAttr] = b
			}
		case kindAddrPart:
			if sv := extractStringValue(val, ""); sv != "" {
				b, _ := json.Marshal(sv)
				addrParts[rule.subAttr] = b
			}
		}
	}
	for parent, obj := range parentObjs {
		b, _ := json.Marshal(obj)
		result[string(parent)] = b
	}
	if len(addrParts) > 0 {
		addrParts["type"], _ = json.Marshal("work")
		addrParts["primary"], _ = json.Marshal(true)
		b, _ := json.Marshal([]map[string]json.RawMessage{addrParts})
		result[string(scimFieldAddresses)] = b
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func reverseMapCoreAttrsForSchema(coreAttrs map[string]json.RawMessage,
	schema json.RawMessage) map[string]json.RawMessage {
	if len(coreAttrs) == 0 || len(schema) == 0 {
		return nil
	}
	var rawProps map[string]rawPropertyDef
	if err := json.Unmarshal(schema, &rawProps); err != nil || len(rawProps) == 0 {
		return nil
	}

	result := make(map[string]json.RawMessage)
	for _, rule := range coreAttrRules {
		coreVal, ok := coreAttrs[string(reverseLookupField(rule))]
		if !ok || len(coreVal) == 0 {
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
		return nil
	}
	return result
}

// reverseLookupField returns the coreAttrs key to read for rule, based on its kind.
func reverseLookupField(rule coreAttrRule) scimCoreField {
	switch rule.kind {
	case kindSubAttr:
		return rule.parentField
	case kindAddrPart:
		return scimFieldAddresses
	default:
		return rule.scimField
	}
}

// findTargetAttrName finds the entity-type schema property matching candidate, case-insensitively.
func findTargetAttrName(rawProps map[string]rawPropertyDef, candidate string) string {
	for propName := range rawProps {
		if strings.EqualFold(propName, candidate) {
			return propName
		}
	}
	return ""
}

// reverseMapRuleValue converts a single core attribute value back into its entity-type
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
	case kindAddrPart:
		return reverseMapAddrPart(rule, coreVal)
	default:
		return nil, false
	}
}

func reverseMapSimpleString(coreVal json.RawMessage) (json.RawMessage, bool) {
	sv := extractStringValue(coreVal, "")
	if sv == "" {
		return nil, false
	}
	b, _ := json.Marshal(sv)
	return b, true
}

func reverseMapMultiComplex(rule coreAttrRule, coreVal json.RawMessage, propDef rawPropertyDef,
) (json.RawMessage, bool) {
	normalized := normalizeToMultiComplex(coreVal, rule.defaultType, rule.valueKey)
	if len(normalized) == 0 {
		return nil, false
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

func reverseMapAddrPart(rule coreAttrRule, coreVal json.RawMessage) (json.RawMessage, bool) {
	var addrArr []map[string]json.RawMessage
	if err := json.Unmarshal(coreVal, &addrArr); err != nil || len(addrArr) == 0 {
		return nil, false
	}
	subVal, exists := addrArr[0][rule.subAttr]
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

func findCandidateValue(m map[string]json.RawMessage, candidate string) json.RawMessage {
	for k, v := range m {
		if strings.EqualFold(k, candidate) {
			return v
		}
	}
	return nil
}

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

func normalizeToMultiComplex(raw json.RawMessage, defaultType string, valueKey string) []map[string]interface{} {
	if len(raw) == 0 {
		return nil
	}
	if valueKey == "" {
		valueKey = scimValueKey
	}

	// Case 1: Plain string -> wrap in array
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return []map[string]interface{}{{valueKey: s, "type": defaultType, "primary": true}}
	}

	// Case 2: Array of strings
	var strArr []string
	if err := json.Unmarshal(raw, &strArr); err == nil {
		var out []map[string]interface{}
		for i, val := range strArr {
			out = append(out, map[string]interface{}{valueKey: val, "type": defaultType, "primary": i == 0})
		}
		return out
	}

	// Case 3: Array of objects
	var objArr []map[string]interface{}
	if err := json.Unmarshal(raw, &objArr); err == nil {
		var out []map[string]interface{}
		for _, obj := range objArr {
			valStr, _ := obj[valueKey].(string)
			if valStr == "" && valueKey != scimValueKey {
				valStr, _ = obj[scimValueKey].(string)
			}
			if valStr == "" {
				continue
			}

			newObj := make(map[string]interface{})
			for k, v := range obj {
				newObj[k] = v
			}
			newObj[valueKey] = valStr

			typStr, _ := obj["type"].(string)
			if typStr == "" {
				newObj["type"] = defaultType
			}
			out = append(out, newObj)
		}
		if len(out) > 0 && !hasPrimary(out) {
			out[0]["primary"] = true
		}
		return out
	}

	// Case 4: Single object -> wrap in array
	var singleObj map[string]interface{}
	if err := json.Unmarshal(raw, &singleObj); err == nil {
		valStr, _ := singleObj[valueKey].(string)
		if valStr == "" && valueKey != scimValueKey {
			valStr, _ = singleObj[scimValueKey].(string)
		}
		if valStr == "" {
			return nil
		}
		newObj := make(map[string]interface{})
		for k, v := range singleObj {
			newObj[k] = v
		}
		newObj[valueKey] = valStr

		typStr, _ := singleObj["type"].(string)
		if typStr == "" {
			newObj["type"] = defaultType
		}
		newObj["primary"] = true
		return []map[string]interface{}{newObj}
	}
	return nil
}

func hasPrimary(arr []map[string]interface{}) bool {
	for _, obj := range arr {
		if p, ok := obj["primary"].(bool); ok && p {
			return true
		}
	}
	return false
}
