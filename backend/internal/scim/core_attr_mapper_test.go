// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package scim

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// --- mapToCoreAttrs ---

// TestMapToCoreAttrs_EmptyInput tests Map To Core Attrs for Empty Input.
func TestMapToCoreAttrs_EmptyInput(t *testing.T) {
	result := mapToCoreAttrs(nil)
	require.Nil(t, result)
}

// TestMapToCoreAttrs_InvalidJSON tests Map To Core Attrs for Invalid JSON.
func TestMapToCoreAttrs_InvalidJSON(t *testing.T) {
	result := mapToCoreAttrs(json.RawMessage(`not json`))
	require.Nil(t, result)
}

// TestMapToCoreAttrs_NoMatchingAttrs tests Map To Core Attrs for No Matching Attrs.
func TestMapToCoreAttrs_NoMatchingAttrs(t *testing.T) {
	result := mapToCoreAttrs(json.RawMessage(`{"foo":"bar"}`))
	require.Nil(t, result)
}

// TestMapToCoreAttrs_SimpleString tests Map To Core Attrs for Simple String.
func TestMapToCoreAttrs_SimpleString(t *testing.T) {
	result := mapToCoreAttrs(json.RawMessage(`{"username":"jdoe"}`))
	require.NotNil(t, result)
	require.JSONEq(t, `"jdoe"`, string(result["userName"]))
}

// TestMapToCoreAttrs_SimpleString_CaseInsensitiveCandidate tests Map To Core Attrs for Simple String Case
// Insensitive Candidate.
func TestMapToCoreAttrs_SimpleString_CaseInsensitiveCandidate(t *testing.T) {
	result := mapToCoreAttrs(json.RawMessage(`{"USERNAME":"jdoe"}`))
	require.NotNil(t, result)
	require.JSONEq(t, `"jdoe"`, string(result["userName"]))
}

// TestMapToCoreAttrs_SimpleString_EmptyValueSkipped tests Map To Core Attrs for Simple String Empty Value Skipped.
func TestMapToCoreAttrs_SimpleString_EmptyValueSkipped(t *testing.T) {
	result := mapToCoreAttrs(json.RawMessage(`{"username":""}`))
	require.Nil(t, result)
}

// TestMapToCoreAttrs_EmailPlainString tests Map To Core Attrs for Email Plain String.
func TestMapToCoreAttrs_EmailPlainString(t *testing.T) {
	result := mapToCoreAttrs(json.RawMessage(`{"email":"a@example.com"}`))
	require.NotNil(t, result)
	var emails []map[string]interface{}
	require.NoError(t, json.Unmarshal(result["emails"], &emails))
	require.Len(t, emails, 1)
	require.Equal(t, "a@example.com", emails[0]["value"])
	_, hasType := emails[0]["type"]
	require.False(t, hasType)
	require.Equal(t, true, emails[0]["primary"])
}

// TestMapToCoreAttrs_EmailArrayOfStrings tests Map To Core Attrs for Email Array Of Strings.
func TestMapToCoreAttrs_EmailArrayOfStrings(t *testing.T) {
	result := mapToCoreAttrs(json.RawMessage(`{"email":["a@example.com","b@example.com"]}`))
	var emails []map[string]interface{}
	require.NoError(t, json.Unmarshal(result["emails"], &emails))
	require.Len(t, emails, 2)
	require.Equal(t, true, emails[0]["primary"])
	require.Equal(t, false, emails[1]["primary"])
}

// TestMapToCoreAttrs_EmailArrayOfObjects tests Map To Core Attrs for Email Array Of Objects.
func TestMapToCoreAttrs_EmailArrayOfObjects(t *testing.T) {
	result := mapToCoreAttrs(json.RawMessage(`{"email":[{"value":"a@example.com","type":"home"}]}`))
	var emails []map[string]interface{}
	require.NoError(t, json.Unmarshal(result["emails"], &emails))
	require.Equal(t, "home", emails[0]["type"])
	require.Equal(t, true, emails[0]["primary"])
}

// TestMapToCoreAttrs_PhoneNumber tests Map To Core Attrs for Phone Number.
func TestMapToCoreAttrs_PhoneNumber(t *testing.T) {
	result := mapToCoreAttrs(json.RawMessage(`{"phone_number":"123456"}`))
	var phones []map[string]interface{}
	require.NoError(t, json.Unmarshal(result["phoneNumbers"], &phones))
	require.Equal(t, "123456", phones[0]["value"])
	_, hasType := phones[0]["type"]
	require.False(t, hasType)
}

// TestMapToCoreAttrs_Picture tests Map To Core Attrs for Picture.
func TestMapToCoreAttrs_Picture(t *testing.T) {
	result := mapToCoreAttrs(json.RawMessage(`{"picture":"http://x/y.png"}`))
	var photos []map[string]interface{}
	require.NoError(t, json.Unmarshal(result["photos"], &photos))
	_, hasType := photos[0]["type"]
	require.False(t, hasType)
}

// TestMapToCoreAttrs_NameSubAttrsMerged tests Map To Core Attrs for Name Sub Attrs Merged.
func TestMapToCoreAttrs_NameSubAttrsMerged(t *testing.T) {
	result := mapToCoreAttrs(json.RawMessage(`{
		"given_name":"John",
		"family_name":"Doe",
		"middle_name":"Q",
		"name":"John Q Doe"
	}`))
	require.NotNil(t, result)
	var name map[string]string
	require.NoError(t, json.Unmarshal(result["name"], &name))
	require.Equal(t, "John", name["givenName"])
	require.Equal(t, "Doe", name["familyName"])
	require.Equal(t, "Q", name["middleName"])
	require.Equal(t, "John Q Doe", name["formatted"])
}

// TestMapToCoreAttrs_AddressParts tests Map To Core Attrs for Address Parts.
func TestMapToCoreAttrs_AddressParts(t *testing.T) {
	result := mapToCoreAttrs(json.RawMessage(`{
		"street_address":"123 Main St",
		"locality":"Metropolis",
		"region":"NY",
		"postal_code":"10001",
		"country":"US"
	}`))
	var addrs []map[string]interface{}
	require.NoError(t, json.Unmarshal(result["addresses"], &addrs))
	require.Len(t, addrs, 1)
	require.Equal(t, "123 Main St", addrs[0]["streetAddress"])
	require.Equal(t, "Metropolis", addrs[0]["locality"])
	require.Equal(t, "NY", addrs[0]["region"])
	require.Equal(t, "10001", addrs[0]["postalCode"])
	require.Equal(t, "US", addrs[0]["country"])
	_, hasType := addrs[0]["type"]
	require.False(t, hasType)
	require.Equal(t, true, addrs[0]["primary"])
}

// TestMapToCoreAttrs_FormattedAddressAndPartsMerge guards against the
// kindMultiComplex "address" write and the kindMultiComplexPart merge both
// targeting result["addresses"]: when a usertype declares both a formatted
// "address" attribute and discrete street_address/locality/etc attributes,
// neither source should silently clobber the other.
// TestMapToCoreAttrs_FormattedAddressAndPartsMerge tests Map To Core Attrs for Formatted Address And Parts Merge.
func TestMapToCoreAttrs_FormattedAddressAndPartsMerge(t *testing.T) {
	result := mapToCoreAttrs(json.RawMessage(`{
		"address":"123 Main St, Metropolis, NY 10001, US",
		"street_address":"123 Main St",
		"locality":"Metropolis",
		"country":"US"
	}`))
	var addrs []map[string]interface{}
	require.NoError(t, json.Unmarshal(result["addresses"], &addrs))
	require.Len(t, addrs, 1)
	require.Equal(t, "123 Main St, Metropolis, NY 10001, US", addrs[0]["formatted"],
		"formatted address must survive alongside the discrete address parts")
	require.Equal(t, "123 Main St", addrs[0]["streetAddress"])
	require.Equal(t, "Metropolis", addrs[0]["locality"])
	require.Equal(t, "US", addrs[0]["country"])
}

// TestMapToCoreAttrs_PartialAddress tests Map To Core Attrs for Partial Address.
func TestMapToCoreAttrs_PartialAddress(t *testing.T) {
	result := mapToCoreAttrs(json.RawMessage(`{"country":"US"}`))
	var addrs []map[string]interface{}
	require.NoError(t, json.Unmarshal(result["addresses"], &addrs))
	require.Len(t, addrs, 1)
	require.Equal(t, "US", addrs[0]["country"])
	_, hasStreet := addrs[0]["streetAddress"]
	require.False(t, hasStreet)
}

// TestMapToCoreAttrs_AllSimpleFields tests Map To Core Attrs for All Simple Fields.
func TestMapToCoreAttrs_AllSimpleFields(t *testing.T) {
	result := mapToCoreAttrs(json.RawMessage(`{
		"title":"Engineer",
		"nickname":"Johnny",
		"locale":"en-US",
		"preferred_language":"en",
		"zoneinfo":"UTC",
		"profile":"http://profile",
		"display_name":"John"
	}`))
	require.JSONEq(t, `"Engineer"`, string(result["title"]))
	require.JSONEq(t, `"Johnny"`, string(result["nickName"]))
	require.JSONEq(t, `"en-US"`, string(result["locale"]))
	require.JSONEq(t, `"en"`, string(result["preferredLanguage"]))
	require.JSONEq(t, `"UTC"`, string(result["timezone"]))
	require.JSONEq(t, `"http://profile"`, string(result["profileUrl"]))
	require.JSONEq(t, `"John"`, string(result["displayName"]))
}

// --- reverseMapCoreAttrsForSchema ---

// TestReverseMapCoreAttrsForSchema_EmptyCoreAttrs tests Reverse Map Core Attrs For Schema for Empty Core Attrs.
func TestReverseMapCoreAttrsForSchema_EmptyCoreAttrs(t *testing.T) {
	result, err := reverseMapCoreAttrsForSchema(nil, json.RawMessage(`{}`))
	require.NoError(t, err)
	require.Nil(t, result)
}

// TestReverseMapCoreAttrsForSchema_EmptySchema tests Reverse Map Core Attrs For Schema for Empty Schema.
func TestReverseMapCoreAttrsForSchema_EmptySchema(t *testing.T) {
	coreAttrs := map[string]json.RawMessage{"userName": json.RawMessage(`"jdoe"`)}
	result, err := reverseMapCoreAttrsForSchema(coreAttrs, nil)
	require.NoError(t, err)
	require.Nil(t, result)
}

// TestReverseMapCoreAttrsForSchema_InvalidSchemaJSON tests Reverse Map Core Attrs For Schema for Invalid Schema JSON.
func TestReverseMapCoreAttrsForSchema_InvalidSchemaJSON(t *testing.T) {
	coreAttrs := map[string]json.RawMessage{"userName": json.RawMessage(`"jdoe"`)}
	result, err := reverseMapCoreAttrsForSchema(coreAttrs, json.RawMessage(`not json`))
	require.Error(t, err)
	require.Nil(t, result)
}

// TestReverseMapCoreAttrsForSchema_SimpleString tests Reverse Map Core Attrs For Schema for Simple String.
func TestReverseMapCoreAttrsForSchema_SimpleString(t *testing.T) {
	schema := json.RawMessage(`{"username":{"type":"string"}}`)
	coreAttrs := map[string]json.RawMessage{"userName": json.RawMessage(`"jdoe"`)}
	result, err := reverseMapCoreAttrsForSchema(coreAttrs, schema)
	require.NoError(t, err)
	require.JSONEq(t, `"jdoe"`, string(result["username"]))
}

// TestReverseMapCoreAttrsForSchema_CaseInsensitivePropName tests Reverse Map Core Attrs For Schema for Case
// Insensitive Prop Name.
func TestReverseMapCoreAttrsForSchema_CaseInsensitivePropName(t *testing.T) {
	schema := json.RawMessage(`{"UserName":{"type":"string"}}`)
	coreAttrs := map[string]json.RawMessage{"userName": json.RawMessage(`"jdoe"`)}
	result, err := reverseMapCoreAttrsForSchema(coreAttrs, schema)
	require.NoError(t, err)
	require.JSONEq(t, `"jdoe"`, string(result["UserName"]))
}

// TestReverseMapCoreAttrsForSchema_NoMatchingSchemaProp tests Reverse Map Core Attrs For Schema for No
// Matching Schema Prop.
func TestReverseMapCoreAttrsForSchema_NoMatchingSchemaProp(t *testing.T) {
	schema := json.RawMessage(`{"foo":{"type":"string"}}`)
	coreAttrs := map[string]json.RawMessage{"userName": json.RawMessage(`"jdoe"`)}
	result, err := reverseMapCoreAttrsForSchema(coreAttrs, schema)
	require.NoError(t, err)
	require.Nil(t, result)
}

// TestReverseMapCoreAttrsForSchema_MultiComplex_ArrayType tests Reverse Map Core Attrs For Schema for Multi
// Complex Array Type.
func TestReverseMapCoreAttrsForSchema_MultiComplex_ArrayType(t *testing.T) {
	schema := json.RawMessage(`{"email":{"type":"array","items":{"type":"string"}}}`)
	coreAttrs := map[string]json.RawMessage{
		"emails": json.RawMessage(`[{"value":"a@example.com","type":"work","primary":true}]`),
	}
	result, err := reverseMapCoreAttrsForSchema(coreAttrs, schema)
	require.NoError(t, err)
	var got []string
	require.NoError(t, json.Unmarshal(result["email"], &got))
	require.Equal(t, []string{"a@example.com"}, got)
}

// TestReverseMapCoreAttrsForSchema_MultiComplex_StringType tests Reverse Map Core Attrs For Schema for Multi
// Complex String Type.
func TestReverseMapCoreAttrsForSchema_MultiComplex_StringType(t *testing.T) {
	schema := json.RawMessage(`{"email":{"type":"string"}}`)
	coreAttrs := map[string]json.RawMessage{
		"emails": json.RawMessage(`[{"value":"a@example.com","type":"work","primary":true}]`),
	}
	result, err := reverseMapCoreAttrsForSchema(coreAttrs, schema)
	require.NoError(t, err)
	require.JSONEq(t, `"a@example.com"`, string(result["email"]))
}

// TestReverseMapCoreAttrsForSchema_MultiComplex_ArrayOfObjects_AllEntriesPreserved tests Reverse Map Core
// Attrs For Schema for Multi Complex Array Of Objects All Entries Preserved.
func TestReverseMapCoreAttrsForSchema_MultiComplex_ArrayOfObjects_AllEntriesPreserved(t *testing.T) {
	schema := json.RawMessage(`{
		"email":{"type":"array","items":{"type":"object","properties":{
			"value":{"type":"string"},"type":{"type":"string"},"primary":{"type":"boolean"}
		}}}
	}`)
	coreAttrs := map[string]json.RawMessage{
		"emails": json.RawMessage(`[
			{"value":"a.work@example.com","type":"work","primary":true},
			{"value":"a.home@example.com","type":"home","primary":false}
		]`),
	}
	result, err := reverseMapCoreAttrsForSchema(coreAttrs, schema)
	require.NoError(t, err)
	var got []map[string]interface{}
	require.NoError(t, json.Unmarshal(result["email"], &got))
	require.Len(t, got, 2)
	require.Equal(t, "a.work@example.com", got[0]["value"])
	require.Equal(t, "work", got[0]["type"])
	require.Equal(t, true, got[0]["primary"])
	require.Equal(t, "a.home@example.com", got[1]["value"])
	require.Equal(t, "home", got[1]["type"])
	require.Equal(t, false, got[1]["primary"])
}

// TestReverseMapCoreAttrsForSchema_MultiComplex_SingleObject tests Reverse Map Core Attrs For Schema for
// Multi Complex Single Object.
func TestReverseMapCoreAttrsForSchema_MultiComplex_SingleObject(t *testing.T) {
	schema := json.RawMessage(`{
		"email":{"type":"object","properties":{
			"value":{"type":"string"},"type":{"type":"string"},"primary":{"type":"boolean"}
		}}
	}`)
	coreAttrs := map[string]json.RawMessage{
		"emails": json.RawMessage(`[{"value":"a.work@example.com","type":"work","primary":true}]`),
	}
	result, err := reverseMapCoreAttrsForSchema(coreAttrs, schema)
	require.NoError(t, err)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(result["email"], &got))
	require.Equal(t, "a.work@example.com", got["value"])
	require.Equal(t, "work", got["type"])
	require.Equal(t, true, got["primary"])
}

// TestReverseMapCoreAttrsForSchema_MultiComplex_SingleObject_TakesFirstEntry tests Reverse Map Core Attrs For
// Schema for Multi Complex Single Object Takes First Entry.
func TestReverseMapCoreAttrsForSchema_MultiComplex_SingleObject_TakesFirstEntry(t *testing.T) {
	schema := json.RawMessage(`{
		"email":{"type":"object","properties":{
			"value":{"type":"string"},"type":{"type":"string"},"primary":{"type":"boolean"}
		}}
	}`)
	coreAttrs := map[string]json.RawMessage{
		"emails": json.RawMessage(`[
			{"value":"a.work@example.com","type":"work","primary":true},
			{"value":"a.home@example.com","type":"home","primary":false}
		]`),
	}
	result, err := reverseMapCoreAttrsForSchema(coreAttrs, schema)
	require.NoError(t, err)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(result["email"], &got))
	require.Equal(t, "a.work@example.com", got["value"])
}

// TestReverseMapCoreAttrsForSchema_MultiComplex_ArrayOfStrings_AllEntriesPreserved tests Reverse Map Core
// Attrs For Schema for Multi Complex Array Of Strings All Entries Preserved.
func TestReverseMapCoreAttrsForSchema_MultiComplex_ArrayOfStrings_AllEntriesPreserved(t *testing.T) {
	schema := json.RawMessage(`{"email":{"type":"array","items":{"type":"string"}}}`)
	coreAttrs := map[string]json.RawMessage{
		"emails": json.RawMessage(`[
			{"value":"a.work@example.com","type":"work","primary":true},
			{"value":"a.home@example.com","type":"home","primary":false}
		]`),
	}
	result, err := reverseMapCoreAttrsForSchema(coreAttrs, schema)
	require.NoError(t, err)
	var got []string
	require.NoError(t, json.Unmarshal(result["email"], &got))
	require.Equal(t, []string{"a.work@example.com", "a.home@example.com"}, got)
}

// TestReverseMapCoreAttrsForSchema_SubAttr tests Reverse Map Core Attrs For Schema for Sub Attr.
func TestReverseMapCoreAttrsForSchema_SubAttr(t *testing.T) {
	schema := json.RawMessage(`{"given_name":{"type":"string"}}`)
	coreAttrs := map[string]json.RawMessage{
		"name": json.RawMessage(`{"givenName":"John","familyName":"Doe"}`),
	}
	result, err := reverseMapCoreAttrsForSchema(coreAttrs, schema)
	require.NoError(t, err)
	require.JSONEq(t, `"John"`, string(result["given_name"]))
}

// TestReverseMapCoreAttrsForSchema_SubAttr_MissingKey tests Reverse Map Core Attrs For Schema for Sub Attr Missing Key.
func TestReverseMapCoreAttrsForSchema_SubAttr_MissingKey(t *testing.T) {
	schema := json.RawMessage(`{"given_name":{"type":"string"}}`)
	coreAttrs := map[string]json.RawMessage{
		"name": json.RawMessage(`{"familyName":"Doe"}`),
	}
	result, err := reverseMapCoreAttrsForSchema(coreAttrs, schema)
	require.NoError(t, err)
	require.Nil(t, result)
}

// TestReverseMapCoreAttrsForSchema_AddrPart tests Reverse Map Core Attrs For Schema for Addr Part.
func TestReverseMapCoreAttrsForSchema_AddrPart(t *testing.T) {
	schema := json.RawMessage(`{"country":{"type":"string"}}`)
	coreAttrs := map[string]json.RawMessage{
		"addresses": json.RawMessage(`[{"country":"US","type":"work","primary":true}]`),
	}
	result, err := reverseMapCoreAttrsForSchema(coreAttrs, schema)
	require.NoError(t, err)
	require.JSONEq(t, `"US"`, string(result["country"]))
}

// TestMapToCoreAttrs_AddressObject tests Map To Core Attrs for Address Object.
func TestMapToCoreAttrs_AddressObject(t *testing.T) {
	result := mapToCoreAttrs(json.RawMessage(`{"address":{
		"street_address":"456 Tech Park","locality":"Colombo","postal_code":"00100"
	}}`))
	require.NotNil(t, result)
	var addrs []map[string]interface{}
	require.NoError(t, json.Unmarshal(result["addresses"], &addrs))
	require.Len(t, addrs, 1)
	require.Equal(t, "456 Tech Park", addrs[0]["streetAddress"])
	require.Equal(t, "Colombo", addrs[0]["locality"])
	require.Equal(t, "00100", addrs[0]["postalCode"])
	_, hasType := addrs[0]["type"]
	require.False(t, hasType)
	require.Equal(t, true, addrs[0]["primary"])
}

// TestMapToCoreAttrs_AddressObject_NonCanonicalSubAttrsDropped tests Map To Core Attrs for Address Object Non
// Canonical Sub Attrs Dropped.
func TestMapToCoreAttrs_AddressObject_NonCanonicalSubAttrsDropped(t *testing.T) {
	result := mapToCoreAttrs(json.RawMessage(`{"address":{"unknown_field":"456 Tech Park"}}`))
	require.Nil(t, result["addresses"])
}

// TestMapToCoreAttrs_AddressArray_AllEntriesMetaOnly_NoAddresses tests Map To Core Attrs for Address Array
// All Entries Meta Only No Addresses.
func TestMapToCoreAttrs_AddressArray_AllEntriesMetaOnly_NoAddresses(t *testing.T) {
	result := mapToCoreAttrs(json.RawMessage(`{"address":[{"type":"work"}]}`))
	require.Nil(t, result)
}

// TestReverseMapCoreAttrsForSchema_AddrPart_EmptyArray tests Reverse Map Core Attrs For Schema for Addr Part
// Empty Array.
func TestReverseMapCoreAttrsForSchema_AddrPart_EmptyArray(t *testing.T) {
	schema := json.RawMessage(`{"country":{"type":"string"}}`)
	coreAttrs := map[string]json.RawMessage{
		"addresses": json.RawMessage(`[]`),
	}
	result, err := reverseMapCoreAttrsForSchema(coreAttrs, schema)
	require.NoError(t, err)
	require.Nil(t, result)
}

// TestReverseMapCoreAttrsForSchema_AddressObject tests Reverse Map Core Attrs For Schema for Address Object.
func TestReverseMapCoreAttrsForSchema_AddressObject(t *testing.T) {
	schema := json.RawMessage(`{
		"address":{"type":"object","properties":{
			"street_address":{"type":"string"},"locality":{"type":"string"},"postal_code":{"type":"string"}
		}}
	}`)
	coreAttrs := map[string]json.RawMessage{
		"addresses": json.RawMessage(`[{
			"streetAddress":"456 Tech Park","locality":"Colombo","postalCode":"00100",
			"type":"work","primary":true
		}]`),
	}
	result, err := reverseMapCoreAttrsForSchema(coreAttrs, schema)
	require.NoError(t, err)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(result["address"], &got))
	require.Equal(t, "456 Tech Park", got["street_address"])
	require.Equal(t, "Colombo", got["locality"])
	require.Equal(t, "00100", got["postal_code"])
}

// TestReverseMapCoreAttrsForSchema_AddressArrayOfObjects tests Reverse Map Core Attrs For Schema for Address
// Array Of Objects.
func TestReverseMapCoreAttrsForSchema_AddressArrayOfObjects(t *testing.T) {
	schema := json.RawMessage(`{
		"address":{"type":"array","items":{"type":"object"}}
	}`)
	coreAttrs := map[string]json.RawMessage{
		"addresses": json.RawMessage(`[{"street":"456 Tech Park","city":"Colombo","type":"work","primary":true}]`),
	}
	result, err := reverseMapCoreAttrsForSchema(coreAttrs, schema)
	require.NoError(t, err)
	var got []map[string]interface{}
	require.NoError(t, json.Unmarshal(result["address"], &got))
	require.Len(t, got, 1)
	require.Equal(t, "456 Tech Park", got[0]["street"])
	require.Equal(t, "Colombo", got[0]["city"])
}

// --- findCandidateValue ---

// TestFindCandidateValue_Found tests Find Candidate Value for Found.
func TestFindCandidateValue_Found(t *testing.T) {
	m := map[string]json.RawMessage{"UserName": json.RawMessage(`"jdoe"`)}
	v := findCandidateValue(m, "username")
	require.Equal(t, json.RawMessage(`"jdoe"`), v)
}

// TestFindCandidateValue_NotFound tests Find Candidate Value for Not Found.
func TestFindCandidateValue_NotFound(t *testing.T) {
	m := map[string]json.RawMessage{"foo": json.RawMessage(`"bar"`)}
	v := findCandidateValue(m, "username")
	require.Nil(t, v)
}

// --- extractStringValue ---

// TestExtractStringValue_Empty tests Extract String Value for Empty.
func TestExtractStringValue_Empty(t *testing.T) {
	require.Equal(t, "", extractStringValue(nil, ""))
}

// TestExtractStringValue_PlainString tests Extract String Value for Plain String.
func TestExtractStringValue_PlainString(t *testing.T) {
	require.Equal(t, "jdoe", extractStringValue(json.RawMessage(`"jdoe"`), ""))
}

// TestExtractStringValue_ObjectWithTargetKey tests Extract String Value for Object With Target Key.
func TestExtractStringValue_ObjectWithTargetKey(t *testing.T) {
	require.Equal(t, "a@example.com", extractStringValue(json.RawMessage(`{"value":"a@example.com"}`), ""))
}

// TestExtractStringValue_ObjectWithCustomTargetKey tests Extract String Value for Object With Custom Target Key.
func TestExtractStringValue_ObjectWithCustomTargetKey(t *testing.T) {
	require.Equal(t, "x", extractStringValue(json.RawMessage(`{"custom":"x"}`), "custom"))
}

// TestExtractStringValue_ObjectFallbackToValue tests Extract String Value for Object Fallback To Value.
func TestExtractStringValue_ObjectFallbackToValue(t *testing.T) {
	require.Equal(t, "y", extractStringValue(json.RawMessage(`{"value":"y"}`), "custom"))
}

// TestExtractStringValue_ArrayRecursesFirstElem tests Extract String Value for Array Recurses First Elem.
func TestExtractStringValue_ArrayRecursesFirstElem(t *testing.T) {
	require.Equal(t, "first", extractStringValue(json.RawMessage(`["first","second"]`), ""))
}

// TestExtractStringValue_NoMatch tests Extract String Value for No Match.
func TestExtractStringValue_NoMatch(t *testing.T) {
	require.Equal(t, "", extractStringValue(json.RawMessage(`{"other":"x"}`), ""))
}

// TestExtractStringValue_EmptyArray tests Extract String Value for Empty Array.
func TestExtractStringValue_EmptyArray(t *testing.T) {
	require.Equal(t, "", extractStringValue(json.RawMessage(`[]`), ""))
}

// --- normalizeToMultiComplex ---

// TestNormalizeToMultiComplex_Empty tests Normalize To Multi Complex for Empty.
func TestNormalizeToMultiComplex_Empty(t *testing.T) {
	require.Nil(t, normalizeToMultiComplex(nil, "value"))
}

// TestNormalizeToMultiComplex_PlainString tests Normalize To Multi Complex for Plain String.
func TestNormalizeToMultiComplex_PlainString(t *testing.T) {
	out := normalizeToMultiComplex(json.RawMessage(`"a@example.com"`), "value")
	require.Len(t, out, 1)
	require.Equal(t, "a@example.com", out[0]["value"])
	_, hasType := out[0]["type"]
	require.False(t, hasType)
	require.Equal(t, true, out[0]["primary"])
}

// TestNormalizeToMultiComplex_EmptyValueKeyDefaultsToValue tests Normalize To Multi Complex for Empty Value
// Key Defaults To Value.
func TestNormalizeToMultiComplex_EmptyValueKeyDefaultsToValue(t *testing.T) {
	out := normalizeToMultiComplex(json.RawMessage(`"a@example.com"`), "")
	require.Equal(t, "a@example.com", out[0]["value"])
}

// TestNormalizeToMultiComplex_ArrayOfStrings tests Normalize To Multi Complex for Array Of Strings.
func TestNormalizeToMultiComplex_ArrayOfStrings(t *testing.T) {
	out := normalizeToMultiComplex(json.RawMessage(`["a","b"]`), "value")
	require.Len(t, out, 2)
	require.Equal(t, true, out[0]["primary"])
	require.Equal(t, false, out[1]["primary"])
}

// TestNormalizeToMultiComplex_ArrayOfObjects_NoTypeWhenMissing tests Normalize To Multi Complex for Array Of
// Objects No Type When Missing.
func TestNormalizeToMultiComplex_ArrayOfObjects_NoTypeWhenMissing(t *testing.T) {
	out := normalizeToMultiComplex(json.RawMessage(`[{"value":"a"}]`), "value")
	_, hasType := out[0]["type"]
	require.False(t, hasType)
	require.Equal(t, true, out[0]["primary"])
}

// TestNormalizeToMultiComplex_ArrayOfObjects_ExplicitType tests Normalize To Multi Complex for Array Of
// Objects Explicit Type.
func TestNormalizeToMultiComplex_ArrayOfObjects_ExplicitType(t *testing.T) {
	out := normalizeToMultiComplex(json.RawMessage(`[{"value":"a","type":"home"}]`), "value")
	require.Equal(t, "home", out[0]["type"])
}

// TestNormalizeToMultiComplex_ArrayOfObjects_SkipsMissingValue tests Normalize To Multi Complex for Array Of
// Objects Skips Missing Value.
func TestNormalizeToMultiComplex_ArrayOfObjects_SkipsMissingValue(t *testing.T) {
	out := normalizeToMultiComplex(json.RawMessage(`[{"type":"home"},{"value":"a"}]`), "value")
	require.Len(t, out, 1)
	require.Equal(t, "a", out[0]["value"])
}

// TestNormalizeToMultiComplex_ArrayOfObjects_PreservesExistingPrimary tests Normalize To Multi Complex for
// Array Of Objects Preserves Existing Primary.
func TestNormalizeToMultiComplex_ArrayOfObjects_PreservesExistingPrimary(t *testing.T) {
	out := normalizeToMultiComplex(
		json.RawMessage(`[{"value":"a","primary":false},{"value":"b","primary":true}]`), "value",
	)
	require.Equal(t, false, out[0]["primary"])
	require.Equal(t, true, out[1]["primary"])
}

// TestNormalizeToMultiComplex_SingleObject tests Normalize To Multi Complex for Single Object.
func TestNormalizeToMultiComplex_SingleObject(t *testing.T) {
	out := normalizeToMultiComplex(json.RawMessage(`{"value":"a"}`), "value")
	require.Len(t, out, 1)
	require.Equal(t, true, out[0]["primary"])
	_, hasType := out[0]["type"]
	require.False(t, hasType)
}

// TestNormalizeToMultiComplex_SingleObject_MissingValue tests Normalize To Multi Complex for Single Object
// Missing Value.
func TestNormalizeToMultiComplex_SingleObject_MissingValue(t *testing.T) {
	out := normalizeToMultiComplex(json.RawMessage(`{"type":"home"}`), "value")
	require.Nil(t, out)
}

// TestNormalizeToMultiComplex_SingleObject_Empty tests Normalize To Multi Complex for Single Object Empty.
func TestNormalizeToMultiComplex_SingleObject_Empty(t *testing.T) {
	require.Nil(t, normalizeToMultiComplex(json.RawMessage(`{}`), "value"))
}

// TestNormalizeToMultiComplex_ArrayOfObjects_SkipsEmptyObject tests Normalize To Multi Complex for Array Of
// Objects Skips Empty Object.
func TestNormalizeToMultiComplex_ArrayOfObjects_SkipsEmptyObject(t *testing.T) {
	out := normalizeToMultiComplex(json.RawMessage(`[{},{"value":"a"}]`), "value")
	require.Len(t, out, 1)
	require.Equal(t, "a", out[0]["value"])
}

// TestNormalizeToMultiComplex_ArrayOfObjects_CustomValueKeyFallback tests Normalize To Multi Complex for
// Array Of Objects Custom Value Key Fallback.
func TestNormalizeToMultiComplex_ArrayOfObjects_CustomValueKeyFallback(t *testing.T) {
	out := normalizeToMultiComplex(json.RawMessage(`[{"value":"a"}]`), "custom")
	require.Len(t, out, 1)
	require.Equal(t, "a", out[0]["custom"])
}

// TestNormalizeToMultiComplex_SingleObject_CustomValueKeyFallback tests Normalize To Multi Complex for Single
// Object Custom Value Key Fallback.
func TestNormalizeToMultiComplex_SingleObject_CustomValueKeyFallback(t *testing.T) {
	out := normalizeToMultiComplex(json.RawMessage(`{"value":"a"}`), "custom")
	require.Len(t, out, 1)
	require.Equal(t, "a", out[0]["custom"])
}

// TestNormalizeToMultiComplex_UnmatchedShape tests Normalize To Multi Complex for Unmatched Shape.
func TestNormalizeToMultiComplex_UnmatchedShape(t *testing.T) {
	require.Nil(t, normalizeToMultiComplex(json.RawMessage(`42`), "value"))
	require.Nil(t, normalizeToMultiComplex(json.RawMessage(`true`), "value"))
}

// --- hasPrimary ---

// TestHasPrimary_True tests Has Primary for True.
func TestHasPrimary_True(t *testing.T) {
	arr := []map[string]interface{}{{"primary": true}}
	require.True(t, hasPrimary(arr))
}

// TestHasPrimary_False tests Has Primary for False.
func TestHasPrimary_False(t *testing.T) {
	arr := []map[string]interface{}{{"primary": false}}
	require.False(t, hasPrimary(arr))
}

// TestHasPrimary_Missing tests Has Primary for Missing.
func TestHasPrimary_Missing(t *testing.T) {
	arr := []map[string]interface{}{{}}
	require.False(t, hasPrimary(arr))
}

// --- translateSCIMFilterAttr ---

// TestTranslateSCIMFilterAttr tests Translate SCIM Filter Attr.
func TestTranslateSCIMFilterAttr(t *testing.T) {
	tests := []struct {
		name string
		attr string
		want string
	}{
		{"simple string", "userName", "username"},
		{"sub-attribute", "name.givenName", "given_name"},
		{"multi-valued bare key", "emails", "email"},
		{"multi-valued value sub-key", "emails.value", "email"},
		{"address sub-attribute", "addresses.streetAddress", "street_address"},
		{"case insensitive", "USERNAME", "username"},
		{"unmapped passthrough", "active", "active"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, translateSCIMFilterAttr(tt.attr))
		})
	}
}

// --- isUnsupportedSCIMFilterAttr ---

// TestIsUnsupportedSCIMFilterAttr tests Is Unsupported SCIM Filter Attr.
func TestIsUnsupportedSCIMFilterAttr(t *testing.T) {
	tests := []struct {
		name string
		attr string
		want bool
	}{
		{"emails type sub-key", "emails.type", true},
		{"phoneNumbers type sub-key", "phoneNumbers.type", true},
		{"photos type sub-key", "photos.type", true},
		{"emails primary sub-key", "emails.primary", true},
		{"phoneNumbers primary sub-key", "phoneNumbers.primary", true},
		{"photos primary sub-key", "photos.primary", true},
		{"case insensitive type", "EMAILS.TYPE", true},
		{"case insensitive primary", "EMAILS.PRIMARY", true},
		{"emails value sub-key is supported", "emails.value", false},
		{"emails bare key is supported", "emails", false},
		{"unrelated attribute", "userName", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isUnsupportedSCIMFilterAttr(tt.attr))
		})
	}
}
