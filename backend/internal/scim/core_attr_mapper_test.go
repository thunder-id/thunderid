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
	"testing"

	"github.com/stretchr/testify/require"
)

// --- mapToCoreAttrs ---

func TestMapToCoreAttrs_EmptyInput(t *testing.T) {
	result := mapToCoreAttrs(nil)
	require.Nil(t, result)
}

func TestMapToCoreAttrs_InvalidJSON(t *testing.T) {
	result := mapToCoreAttrs(json.RawMessage(`not json`))
	require.Nil(t, result)
}

func TestMapToCoreAttrs_NoMatchingAttrs(t *testing.T) {
	result := mapToCoreAttrs(json.RawMessage(`{"foo":"bar"}`))
	require.Nil(t, result)
}

func TestMapToCoreAttrs_SimpleString(t *testing.T) {
	result := mapToCoreAttrs(json.RawMessage(`{"username":"jdoe"}`))
	require.NotNil(t, result)
	require.JSONEq(t, `"jdoe"`, string(result["userName"]))
}

func TestMapToCoreAttrs_SimpleString_CaseInsensitiveCandidate(t *testing.T) {
	result := mapToCoreAttrs(json.RawMessage(`{"USERNAME":"jdoe"}`))
	require.NotNil(t, result)
	require.JSONEq(t, `"jdoe"`, string(result["userName"]))
}

func TestMapToCoreAttrs_SimpleString_EmptyValueSkipped(t *testing.T) {
	result := mapToCoreAttrs(json.RawMessage(`{"username":""}`))
	require.Nil(t, result)
}

func TestMapToCoreAttrs_EmailPlainString(t *testing.T) {
	result := mapToCoreAttrs(json.RawMessage(`{"email":"a@example.com"}`))
	require.NotNil(t, result)
	var emails []map[string]interface{}
	require.NoError(t, json.Unmarshal(result["emails"], &emails))
	require.Len(t, emails, 1)
	require.Equal(t, "a@example.com", emails[0]["value"])
	require.Equal(t, "work", emails[0]["type"])
	require.Equal(t, true, emails[0]["primary"])
}

func TestMapToCoreAttrs_EmailArrayOfStrings(t *testing.T) {
	result := mapToCoreAttrs(json.RawMessage(`{"email":["a@example.com","b@example.com"]}`))
	var emails []map[string]interface{}
	require.NoError(t, json.Unmarshal(result["emails"], &emails))
	require.Len(t, emails, 2)
	require.Equal(t, true, emails[0]["primary"])
	require.Equal(t, false, emails[1]["primary"])
}

func TestMapToCoreAttrs_EmailArrayOfObjects(t *testing.T) {
	result := mapToCoreAttrs(json.RawMessage(`{"email":[{"value":"a@example.com","type":"home"}]}`))
	var emails []map[string]interface{}
	require.NoError(t, json.Unmarshal(result["emails"], &emails))
	require.Equal(t, "home", emails[0]["type"])
	require.Equal(t, true, emails[0]["primary"])
}

func TestMapToCoreAttrs_PhoneNumber(t *testing.T) {
	result := mapToCoreAttrs(json.RawMessage(`{"phone_number":"123456"}`))
	var phones []map[string]interface{}
	require.NoError(t, json.Unmarshal(result["phoneNumbers"], &phones))
	require.Equal(t, "123456", phones[0]["value"])
	require.Equal(t, "work", phones[0]["type"])
}

func TestMapToCoreAttrs_Picture(t *testing.T) {
	result := mapToCoreAttrs(json.RawMessage(`{"picture":"http://x/y.png"}`))
	var photos []map[string]interface{}
	require.NoError(t, json.Unmarshal(result["photos"], &photos))
	require.Equal(t, "photo", photos[0]["type"])
}

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
	require.Equal(t, "work", addrs[0]["type"])
	require.Equal(t, true, addrs[0]["primary"])
}

func TestMapToCoreAttrs_PartialAddress(t *testing.T) {
	result := mapToCoreAttrs(json.RawMessage(`{"country":"US"}`))
	var addrs []map[string]interface{}
	require.NoError(t, json.Unmarshal(result["addresses"], &addrs))
	require.Len(t, addrs, 1)
	require.Equal(t, "US", addrs[0]["country"])
	_, hasStreet := addrs[0]["streetAddress"]
	require.False(t, hasStreet)
}

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

func TestReverseMapCoreAttrsForSchema_EmptyCoreAttrs(t *testing.T) {
	result := reverseMapCoreAttrsForSchema(nil, json.RawMessage(`{}`))
	require.Nil(t, result)
}

func TestReverseMapCoreAttrsForSchema_EmptySchema(t *testing.T) {
	coreAttrs := map[string]json.RawMessage{"userName": json.RawMessage(`"jdoe"`)}
	result := reverseMapCoreAttrsForSchema(coreAttrs, nil)
	require.Nil(t, result)
}

func TestReverseMapCoreAttrsForSchema_InvalidSchemaJSON(t *testing.T) {
	coreAttrs := map[string]json.RawMessage{"userName": json.RawMessage(`"jdoe"`)}
	result := reverseMapCoreAttrsForSchema(coreAttrs, json.RawMessage(`not json`))
	require.Nil(t, result)
}

func TestReverseMapCoreAttrsForSchema_SimpleString(t *testing.T) {
	schema := json.RawMessage(`{"username":{"type":"string"}}`)
	coreAttrs := map[string]json.RawMessage{"userName": json.RawMessage(`"jdoe"`)}
	result := reverseMapCoreAttrsForSchema(coreAttrs, schema)
	require.JSONEq(t, `"jdoe"`, string(result["username"]))
}

func TestReverseMapCoreAttrsForSchema_CaseInsensitivePropName(t *testing.T) {
	schema := json.RawMessage(`{"UserName":{"type":"string"}}`)
	coreAttrs := map[string]json.RawMessage{"userName": json.RawMessage(`"jdoe"`)}
	result := reverseMapCoreAttrsForSchema(coreAttrs, schema)
	require.JSONEq(t, `"jdoe"`, string(result["UserName"]))
}

func TestReverseMapCoreAttrsForSchema_NoMatchingSchemaProp(t *testing.T) {
	schema := json.RawMessage(`{"foo":{"type":"string"}}`)
	coreAttrs := map[string]json.RawMessage{"userName": json.RawMessage(`"jdoe"`)}
	result := reverseMapCoreAttrsForSchema(coreAttrs, schema)
	require.Nil(t, result)
}

func TestReverseMapCoreAttrsForSchema_MultiComplex_ArrayType(t *testing.T) {
	schema := json.RawMessage(`{"email":{"type":"array","items":{"type":"string"}}}`)
	coreAttrs := map[string]json.RawMessage{
		"emails": json.RawMessage(`[{"value":"a@example.com","type":"work","primary":true}]`),
	}
	result := reverseMapCoreAttrsForSchema(coreAttrs, schema)
	var got []string
	require.NoError(t, json.Unmarshal(result["email"], &got))
	require.Equal(t, []string{"a@example.com"}, got)
}

func TestReverseMapCoreAttrsForSchema_MultiComplex_StringType(t *testing.T) {
	schema := json.RawMessage(`{"email":{"type":"string"}}`)
	coreAttrs := map[string]json.RawMessage{
		"emails": json.RawMessage(`[{"value":"a@example.com","type":"work","primary":true}]`),
	}
	result := reverseMapCoreAttrsForSchema(coreAttrs, schema)
	require.JSONEq(t, `"a@example.com"`, string(result["email"]))
}

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
	result := reverseMapCoreAttrsForSchema(coreAttrs, schema)
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

func TestReverseMapCoreAttrsForSchema_MultiComplex_SingleObject(t *testing.T) {
	schema := json.RawMessage(`{
		"email":{"type":"object","properties":{
			"value":{"type":"string"},"type":{"type":"string"},"primary":{"type":"boolean"}
		}}
	}`)
	coreAttrs := map[string]json.RawMessage{
		"emails": json.RawMessage(`[{"value":"a.work@example.com","type":"work","primary":true}]`),
	}
	result := reverseMapCoreAttrsForSchema(coreAttrs, schema)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(result["email"], &got))
	require.Equal(t, "a.work@example.com", got["value"])
	require.Equal(t, "work", got["type"])
	require.Equal(t, true, got["primary"])
}

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
	result := reverseMapCoreAttrsForSchema(coreAttrs, schema)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(result["email"], &got))
	require.Equal(t, "a.work@example.com", got["value"])
}

func TestReverseMapCoreAttrsForSchema_MultiComplex_ArrayOfStrings_AllEntriesPreserved(t *testing.T) {
	schema := json.RawMessage(`{"email":{"type":"array","items":{"type":"string"}}}`)
	coreAttrs := map[string]json.RawMessage{
		"emails": json.RawMessage(`[
			{"value":"a.work@example.com","type":"work","primary":true},
			{"value":"a.home@example.com","type":"home","primary":false}
		]`),
	}
	result := reverseMapCoreAttrsForSchema(coreAttrs, schema)
	var got []string
	require.NoError(t, json.Unmarshal(result["email"], &got))
	require.Equal(t, []string{"a.work@example.com", "a.home@example.com"}, got)
}

func TestReverseMapCoreAttrsForSchema_SubAttr(t *testing.T) {
	schema := json.RawMessage(`{"given_name":{"type":"string"}}`)
	coreAttrs := map[string]json.RawMessage{
		"name": json.RawMessage(`{"givenName":"John","familyName":"Doe"}`),
	}
	result := reverseMapCoreAttrsForSchema(coreAttrs, schema)
	require.JSONEq(t, `"John"`, string(result["given_name"]))
}

func TestReverseMapCoreAttrsForSchema_SubAttr_MissingKey(t *testing.T) {
	schema := json.RawMessage(`{"given_name":{"type":"string"}}`)
	coreAttrs := map[string]json.RawMessage{
		"name": json.RawMessage(`{"familyName":"Doe"}`),
	}
	result := reverseMapCoreAttrsForSchema(coreAttrs, schema)
	require.Nil(t, result)
}

func TestReverseMapCoreAttrsForSchema_AddrPart(t *testing.T) {
	schema := json.RawMessage(`{"country":{"type":"string"}}`)
	coreAttrs := map[string]json.RawMessage{
		"addresses": json.RawMessage(`[{"country":"US","type":"work","primary":true}]`),
	}
	result := reverseMapCoreAttrsForSchema(coreAttrs, schema)
	require.JSONEq(t, `"US"`, string(result["country"]))
}

func TestReverseMapCoreAttrsForSchema_AddrPart_EmptyArray(t *testing.T) {
	schema := json.RawMessage(`{"country":{"type":"string"}}`)
	coreAttrs := map[string]json.RawMessage{
		"addresses": json.RawMessage(`[]`),
	}
	result := reverseMapCoreAttrsForSchema(coreAttrs, schema)
	require.Nil(t, result)
}

// --- findCandidateValue ---

func TestFindCandidateValue_Found(t *testing.T) {
	m := map[string]json.RawMessage{"UserName": json.RawMessage(`"jdoe"`)}
	v := findCandidateValue(m, "username")
	require.Equal(t, json.RawMessage(`"jdoe"`), v)
}

func TestFindCandidateValue_NotFound(t *testing.T) {
	m := map[string]json.RawMessage{"foo": json.RawMessage(`"bar"`)}
	v := findCandidateValue(m, "username")
	require.Nil(t, v)
}

// --- extractStringValue ---

func TestExtractStringValue_Empty(t *testing.T) {
	require.Equal(t, "", extractStringValue(nil, ""))
}

func TestExtractStringValue_PlainString(t *testing.T) {
	require.Equal(t, "jdoe", extractStringValue(json.RawMessage(`"jdoe"`), ""))
}

func TestExtractStringValue_ObjectWithTargetKey(t *testing.T) {
	require.Equal(t, "a@example.com", extractStringValue(json.RawMessage(`{"value":"a@example.com"}`), ""))
}

func TestExtractStringValue_ObjectWithCustomTargetKey(t *testing.T) {
	require.Equal(t, "x", extractStringValue(json.RawMessage(`{"custom":"x"}`), "custom"))
}

func TestExtractStringValue_ObjectFallbackToValue(t *testing.T) {
	require.Equal(t, "y", extractStringValue(json.RawMessage(`{"value":"y"}`), "custom"))
}

func TestExtractStringValue_ArrayRecursesFirstElem(t *testing.T) {
	require.Equal(t, "first", extractStringValue(json.RawMessage(`["first","second"]`), ""))
}

func TestExtractStringValue_NoMatch(t *testing.T) {
	require.Equal(t, "", extractStringValue(json.RawMessage(`{"other":"x"}`), ""))
}

func TestExtractStringValue_EmptyArray(t *testing.T) {
	require.Equal(t, "", extractStringValue(json.RawMessage(`[]`), ""))
}

// --- normalizeToMultiComplex ---

func TestNormalizeToMultiComplex_Empty(t *testing.T) {
	require.Nil(t, normalizeToMultiComplex(nil, "work", "value"))
}

func TestNormalizeToMultiComplex_PlainString(t *testing.T) {
	out := normalizeToMultiComplex(json.RawMessage(`"a@example.com"`), "work", "value")
	require.Len(t, out, 1)
	require.Equal(t, "a@example.com", out[0]["value"])
	require.Equal(t, "work", out[0]["type"])
	require.Equal(t, true, out[0]["primary"])
}

func TestNormalizeToMultiComplex_EmptyValueKeyDefaultsToValue(t *testing.T) {
	out := normalizeToMultiComplex(json.RawMessage(`"a@example.com"`), "work", "")
	require.Equal(t, "a@example.com", out[0]["value"])
}

func TestNormalizeToMultiComplex_ArrayOfStrings(t *testing.T) {
	out := normalizeToMultiComplex(json.RawMessage(`["a","b"]`), "work", "value")
	require.Len(t, out, 2)
	require.Equal(t, true, out[0]["primary"])
	require.Equal(t, false, out[1]["primary"])
}

func TestNormalizeToMultiComplex_ArrayOfObjects_DefaultType(t *testing.T) {
	out := normalizeToMultiComplex(json.RawMessage(`[{"value":"a"}]`), "work", "value")
	require.Equal(t, "work", out[0]["type"])
	require.Equal(t, true, out[0]["primary"])
}

func TestNormalizeToMultiComplex_ArrayOfObjects_ExplicitType(t *testing.T) {
	out := normalizeToMultiComplex(json.RawMessage(`[{"value":"a","type":"home"}]`), "work", "value")
	require.Equal(t, "home", out[0]["type"])
}

func TestNormalizeToMultiComplex_ArrayOfObjects_SkipsMissingValue(t *testing.T) {
	out := normalizeToMultiComplex(json.RawMessage(`[{"type":"home"},{"value":"a"}]`), "work", "value")
	require.Len(t, out, 1)
	require.Equal(t, "a", out[0]["value"])
}

func TestNormalizeToMultiComplex_ArrayOfObjects_PreservesExistingPrimary(t *testing.T) {
	out := normalizeToMultiComplex(
		json.RawMessage(`[{"value":"a","primary":false},{"value":"b","primary":true}]`), "work", "value",
	)
	require.Equal(t, false, out[0]["primary"])
	require.Equal(t, true, out[1]["primary"])
}

func TestNormalizeToMultiComplex_SingleObject(t *testing.T) {
	out := normalizeToMultiComplex(json.RawMessage(`{"value":"a"}`), "work", "value")
	require.Len(t, out, 1)
	require.Equal(t, true, out[0]["primary"])
	require.Equal(t, "work", out[0]["type"])
}

func TestNormalizeToMultiComplex_SingleObject_MissingValue(t *testing.T) {
	out := normalizeToMultiComplex(json.RawMessage(`{"type":"home"}`), "work", "value")
	require.Nil(t, out)
}

func TestNormalizeToMultiComplex_ArrayOfObjects_CustomValueKeyFallback(t *testing.T) {
	out := normalizeToMultiComplex(json.RawMessage(`[{"value":"a"}]`), "work", "custom")
	require.Len(t, out, 1)
	require.Equal(t, "a", out[0]["custom"])
}

func TestNormalizeToMultiComplex_SingleObject_CustomValueKeyFallback(t *testing.T) {
	out := normalizeToMultiComplex(json.RawMessage(`{"value":"a"}`), "work", "custom")
	require.Len(t, out, 1)
	require.Equal(t, "a", out[0]["custom"])
}

func TestNormalizeToMultiComplex_UnmatchedShape(t *testing.T) {
	require.Nil(t, normalizeToMultiComplex(json.RawMessage(`42`), "work", "value"))
	require.Nil(t, normalizeToMultiComplex(json.RawMessage(`true`), "work", "value"))
}

// --- hasPrimary ---

func TestHasPrimary_True(t *testing.T) {
	arr := []map[string]interface{}{{"primary": true}}
	require.True(t, hasPrimary(arr))
}

func TestHasPrimary_False(t *testing.T) {
	arr := []map[string]interface{}{{"primary": false}}
	require.False(t, hasPrimary(arr))
}

func TestHasPrimary_Missing(t *testing.T) {
	arr := []map[string]interface{}{{}}
	require.False(t, hasPrimary(arr))
}

// --- translateSCIMFilterAttr ---

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
