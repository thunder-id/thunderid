// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/testhelpers"
)

type SchemaValidateTestSuite struct {
	suite.Suite
	logger *log.Logger
}

func TestSchemaValidateTestSuite(t *testing.T) {
	suite.Run(t, new(SchemaValidateTestSuite))
}

func (s *SchemaValidateTestSuite) SetupTest() {
	s.logger = log.GetLogger()
}

func (s *SchemaValidateTestSuite) TestValidationLogs_ExcludeAttributeValues() {
	type validationLogCase struct {
		name       string
		definition string
		payload    string
		reason     string
		property   string
		attribute  string
	}
	testCases := []validationLogCase{
		{
			name: "default email", definition: `{"contact":{"type":"email"}}`,
			payload: `{"contact":"privacy-canary@example"}`,
			reason:  "Regex pattern mismatch", property: "contact",
		},
		{
			name: "default mobile", definition: `{"contact":{"type":"mobile"}}`,
			payload: `{"contact":"+1555123456789012"}`,
			reason:  "Regex pattern mismatch", property: "contact",
		},
		{
			name:       "custom credential regex",
			definition: `{"secret":{"type":"string","credential":true,"regex":"^[A-Z]+$"}}`,
			payload:    `{"secret":"credential-canary"}`,
			reason:     "Regex pattern mismatch", property: "secret",
		},
		{
			name: "string enum", definition: `{"value":{"type":"string","enum":["allowed"]}}`,
			payload: `{"value":"privacy-canary@example.test"}`,
			reason:  "Value not in enum", property: "value",
		},
		{
			name: "number enum", definition: `{"value":{"type":"number","enum":[1]}}`,
			payload: `{"value":86753}`,
			reason:  "Value not in enum", property: "value",
		},
		{
			name:       "unknown top-level attribute",
			definition: `{"known":{"type":"string"}}`,
			payload:    `{"unexpectedContact":"privacy-canary@example.test credential-canary"}`,
			reason:     "Attribute not defined in schema",
			attribute:  "unexpectedContact",
		},
		{
			name:       "unknown nested attribute",
			definition: `{"profile":{"type":"object","properties":{"known":{"type":"string"}}}}`,
			payload:    `{"profile":{"unexpectedContact":"privacy-canary@example.test credential-canary"}}`,
			reason:     "Attribute not defined in schema", attribute: "profile.unexpectedContact",
		},
		{
			name: "unknown array object attribute",
			definition: `{"profiles":{"type":"array","items":{"type":"object",` +
				`"properties":{"known":{"type":"string"}}}}}`,
			payload: `{"profiles":[{"unexpectedContact":"privacy-canary@example.test credential-canary"}]}`,
			reason:  "Attribute not defined in schema", attribute: "profiles[0].unexpectedContact",
		},
	}
	for _, kind := range []string{TypeEmail, TypeMobile} {
		for _, nested := range []bool{false, true} {
			definition := fmt.Sprintf(`{"contacts":{"type":"array","items":{"type":%q}}}`, kind)
			payload := `{"contacts":["privacy-canary@example"]}`
			property := "contacts[0]"
			if nested {
				definition = fmt.Sprintf(`{"profile":{"type":"object","properties":{"contact":{"type":%q}}}}`, kind)
				payload = `{"profile":{"contact":"privacy-canary@example"}}`
				property = "profile.contact"
			}
			testCases = append(testCases, validationLogCase{
				fmt.Sprintf("%s at %s", kind, property), definition, payload, "Regex pattern mismatch", property, "",
			})
		}
	}
	for _, kind := range []string{TypeString, TypeEmail, TypeMobile, TypeNumber, TypeBoolean, TypeArray, TypeObject} {
		definition := fmt.Sprintf(`{"value":{"type":%q}}`, kind)
		switch kind {
		case TypeArray:
			definition = `{"value":{"type":"array","items":{"type":"string"}}}`
		case TypeObject:
			definition = `{"value":{"type":"object","properties":{"known":{"type":"string"}}}}`
		}
		reasonType := kind
		if IsStringBackedType(kind) {
			reasonType = TypeString
		}
		for _, wrongValue := range []struct {
			name string
			json string
		}{
			{"object", `{"email":"privacy-canary@example.test","password":"credential-canary"}`},
			{"array", `["privacy-canary@example.test","credential-canary"]`},
			{"string", `"privacy-canary@example.test credential-canary"`},
		} {
			if wrongValue.name == kind || (IsStringBackedType(kind) && wrongValue.name == TypeString) {
				continue
			}
			testCases = append(testCases, validationLogCase{
				fmt.Sprintf("%s rejects %s", kind, wrongValue.name), definition,
				`{"value":` + wrongValue.json + `}`,
				"Expected " + reasonType + " but got different type", "value", "",
			})
		}
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			schema, err := CompileSchema(json.RawMessage(tc.definition))
			s.Require().NoError(err)
			out := testhelpers.CaptureLogs(s.T(), "DEBUG", func() {
				valid, err := schema.Validate(context.Background(), json.RawMessage(tc.payload), s.logger, false)
				s.Require().NoError(err)
				s.False(valid)
			})
			s.Contains(out, tc.reason)
			if tc.property != "" {
				s.Contains(out, "property="+tc.property)
			}
			if tc.attribute != "" {
				s.Contains(out, "attribute="+tc.attribute)
			}
			for _, secret := range []string{"privacy-canary", "credential-canary", "1555123456789012", "86753"} {
				s.NotContains(out, secret)
			}
		})
	}
}

func (s *SchemaValidateTestSuite) TestUniquenessLogs_ExcludeInvalidObjectValue() {
	schema, err := CompileSchema(json.RawMessage(`{
		"profile":{"type":"object","properties":{"email":{"type":"email","unique":true}}}
	}`))
	s.Require().NoError(err)
	out := testhelpers.CaptureLogs(s.T(), "DEBUG", func() {
		valid, err := schema.ValidateUniqueness(context.Background(),
			map[string]interface{}{"profile": "privacy-canary@example.test credential-canary"},
			func(map[string]interface{}) (bool, error) {
				s.Fail("uniqueness lookup must not run for an invalid object")
				return false, nil
			}, s.logger)
		s.Require().NoError(err)
		s.False(valid)
	})
	s.Contains(out, "Expected object but got different type")
	s.Contains(out, "property=profile")
	s.NotContains(out, "privacy-canary")
	s.NotContains(out, "credential-canary")
}

func (s *SchemaValidateTestSuite) TestValidAttributes_Pass() {
	schema, err := CompileSchema(json.RawMessage(`{
		"email": {"type": "string", "required": true},
		"age": {"type": "number"}
	}`))
	s.Require().NoError(err)

	ok, err := schema.Validate(
		context.Background(),
		json.RawMessage(`{"email":"user@example.com","age":30}`),
		s.logger,
		false)
	s.Require().NoError(err)
	s.Require().True(ok)
}

func (s *SchemaValidateTestSuite) TestExtraTopLevelAttribute_Rejected() {
	schema, err := CompileSchema(json.RawMessage(`{
		"email": {"type": "string", "required": true}
	}`))
	s.Require().NoError(err)

	ok, err := schema.Validate(
		context.Background(),
		json.RawMessage(`{"email":"user@example.com","address":"123 Main St"}`),
		s.logger,
		false)
	s.Require().NoError(err)
	s.Require().False(ok)
}

func (s *SchemaValidateTestSuite) TestExtraNestedObjectAttribute_Rejected() {
	schema, err := CompileSchema(json.RawMessage(`{
		"address": {
			"type": "object",
			"properties": {
				"city": {"type": "string"}
			}
		}
	}`))
	s.Require().NoError(err)

	ok, err := schema.Validate(
		context.Background(),
		json.RawMessage(`{"address":{"city":"NYC","zip":"10001"}}`),
		s.logger,
		false)
	s.Require().NoError(err)
	s.Require().False(ok)
}

func (s *SchemaValidateTestSuite) TestValidOnlyDeclaredAttributes_Pass() {
	schema, err := CompileSchema(json.RawMessage(`{
		"email": {"type": "string"},
		"age": {"type": "number"},
		"active": {"type": "boolean"}
	}`))
	s.Require().NoError(err)

	ok, err := schema.Validate(
		context.Background(),
		json.RawMessage(`{"email":"a@b.com","age":25,"active":true}`),
		s.logger,
		false)
	s.Require().NoError(err)
	s.Require().True(ok)
}

func (s *SchemaValidateTestSuite) TestSubsetOfDeclaredAttributes_Pass() {
	schema, err := CompileSchema(json.RawMessage(`{
		"email": {"type": "string"},
		"age": {"type": "number"},
		"active": {"type": "boolean"}
	}`))
	s.Require().NoError(err)

	ok, err := schema.Validate(context.Background(), json.RawMessage(`{"email":"a@b.com"}`), s.logger, false)
	s.Require().NoError(err)
	s.Require().True(ok)
}

func (s *SchemaValidateTestSuite) TestMultipleExtraAttributes_Rejected() {
	schema, err := CompileSchema(json.RawMessage(`{
		"email": {"type": "string"}
	}`))
	s.Require().NoError(err)

	ok, err := schema.Validate(
		context.Background(),
		json.RawMessage(`{"email":"a@b.com","foo":"bar","baz":123}`),
		s.logger,
		false)
	s.Require().NoError(err)
	s.Require().False(ok)
}

func (s *SchemaValidateTestSuite) TestDeeplyNestedExtraAttribute_Rejected() {
	schema, err := CompileSchema(json.RawMessage(`{
		"profile": {
			"type": "object",
			"properties": {
				"address": {
					"type": "object",
					"properties": {
						"city": {"type": "string"}
					}
				}
			}
		}
	}`))
	s.Require().NoError(err)

	ok, err := schema.Validate(context.Background(), json.RawMessage(`{
		"profile": {
			"address": {
				"city": "NYC",
				"country": "US"
			}
		}
	}`), s.logger, false)
	s.Require().NoError(err)
	s.Require().False(ok)
}

func (s *SchemaValidateTestSuite) TestEmptyAttributes_Pass() {
	schema, err := CompileSchema(json.RawMessage(`{
		"email": {"type": "string"}
	}`))
	s.Require().NoError(err)

	ok, err := schema.Validate(context.Background(), json.RawMessage(`{}`), s.logger, false)
	s.Require().NoError(err)
	s.Require().True(ok)
}

func (s *SchemaValidateTestSuite) TestNilAttributes_Pass() {
	schema, err := CompileSchema(json.RawMessage(`{
		"email": {"type": "string"}
	}`))
	s.Require().NoError(err)

	ok, err := schema.Validate(context.Background(), nil, s.logger, false)
	s.Require().NoError(err)
	s.Require().True(ok)
}

func (s *SchemaValidateTestSuite) TestValidNestedObjectAttributes_Pass() {
	schema, err := CompileSchema(json.RawMessage(`{
		"address": {
			"type": "object",
			"properties": {
				"street": {"type": "string"},
				"city": {"type": "string"}
			}
		}
	}`))
	s.Require().NoError(err)

	ok, err := schema.Validate(
		context.Background(),
		json.RawMessage(`{"address":{"street":"123 Main","city":"NYC"}}`),
		s.logger,
		false)
	s.Require().NoError(err)
	s.Require().True(ok)
}

func (s *SchemaValidateTestSuite) TestDisplayNameOnAllPropertyTypes_CompileSuccess() {
	schema, err := CompileSchema(json.RawMessage(`{
		"given_name": {"type": "string", "required": true, "displayName": "First Name"},
		"age": {"type": "number", "displayName": "Age"},
		"active": {"type": "boolean", "displayName": "Is Active"},
		"address": {
			"type": "object",
			"displayName": "Home Address",
			"properties": {
				"city": {"type": "string", "displayName": "City"}
			}
		},
		"tags": {
			"type": "array",
			"displayName": "Tags",
			"items": {"type": "string"}
		}
	}`))
	s.Require().NoError(err)
	s.Require().NotNil(schema)

	ok, err := schema.Validate(context.Background(), json.RawMessage(`{
		"given_name": "John",
		"age": 30,
		"active": true,
		"address": {"city": "NYC"},
		"tags": ["admin"]
	}`), s.logger, false)
	s.Require().NoError(err)
	s.Require().True(ok)
}

func (s *SchemaValidateTestSuite) TestDisplayNameWithI18nPattern_CompileSuccess() {
	schema, err := CompileSchema(json.RawMessage(`{
		"family_name": {"type": "string", "displayName": "{{t(custom:user.familyName)}}"}
	}`))
	s.Require().NoError(err)
	s.Require().NotNil(schema)
}

func (s *SchemaValidateTestSuite) TestDisplayNameInvalidType_CompileError() {
	_, err := CompileSchema(json.RawMessage(`{
		"email": {"type": "string", "displayName": 123}
	}`))
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "'displayName' field must be a string")
}

func (s *SchemaValidateTestSuite) TestSchemaWithoutDisplayName_CompileSuccess() {
	schema, err := CompileSchema(json.RawMessage(`{
		"email": {"type": "string", "required": true}
	}`))
	s.Require().NoError(err)
	s.Require().NotNil(schema)
}

func (s *SchemaValidateTestSuite) TestValidateAsDisplayAttribute_RejectsCredential() {
	schema, err := CompileSchema(json.RawMessage(`{
		"email": {"type": "string", "required": true},
		"password": {"type": "string", "required": true, "credential": true}
	}`))
	s.Require().NoError(err)

	s.Require().Equal(DisplayAttributeIsCredential, schema.ValidateAsDisplayAttribute("password"))
	s.Require().Equal(DisplayAttributeValid, schema.ValidateAsDisplayAttribute("email"))
}

func (s *SchemaValidateTestSuite) TestValidate_SkipCredentialRequired() {
	emailAndPasswordSchema := json.RawMessage(`{
		"email": {"type": "string", "required": true},
		"password": {"type": "string", "required": true, "credential": true}
	}`)

	schema, err := CompileSchema(emailAndPasswordSchema)
	s.Require().NoError(err)

	ok, err := schema.Validate(context.Background(), json.RawMessage(`{"email":"user@example.com"}`), s.logger, true)
	s.Require().NoError(err)
	s.Require().True(ok, "missing credential should pass when skipCredentialRequired=true")

	ok, err = schema.Validate(context.Background(), json.RawMessage(`{}`), s.logger, true)
	s.Require().NoError(err)
	s.Require().False(ok, "missing required non-credential should still fail when skipCredentialRequired=true")

	ok, err = schema.Validate(context.Background(), json.RawMessage(`{"email":"user@example.com"}`), s.logger, false)
	s.Require().NoError(err)
	s.Require().False(ok, "missing required credential should fail when skipCredentialRequired=false")
}

func (s *SchemaValidateTestSuite) TestGetAttributes_NonCredentialRequiredOnly_ReturnsOnlyRequiredNonCredential() {
	schema, err := CompileSchema(json.RawMessage(`{
		"email":     {"type": "string", "required": true},
		"firstName": {"type": "string", "required": true, "displayName": "First Name"},
		"password":  {"type": "string", "required": true, "credential": true},
		"age":       {"type": "number"}
	}`))
	s.Require().NoError(err)

	attrs := schema.GetAttributes(AttributeFilter{AllowNonCredential: true, RequiredOnly: true})

	// Only email and firstName should be returned — password is credential, age is not required.
	s.Len(attrs, 2)

	attrMap := make(map[string]AttributeInfo, len(attrs))
	for _, a := range attrs {
		attrMap[a.Attribute] = a
	}

	email, ok := attrMap["email"]
	s.Require().True(ok, "email should be in results")
	s.Equal("", email.DisplayName, "email has no displayName, should be empty")

	firstName, ok := attrMap["firstName"]
	s.Require().True(ok, "firstName should be in results")
	s.Equal("First Name", firstName.DisplayName)

	_, hasPassword := attrMap["password"]
	s.False(hasPassword, "password is credential and must be excluded")

	_, hasAge := attrMap["age"]
	s.False(hasAge, "age is not required and must be excluded")
}

// A caller offering an enum attribute as a choice presents the values the way the schema author
// wrote them, so the declared order has to survive compilation. The enum is also held as a set for
// validation, and a set does not preserve order on its own.
func (s *SchemaValidateTestSuite) TestGetAttributes_EnumKeepsDeclaredOrder() {
	schema, err := CompileSchema(json.RawMessage(`{
		"modelProvider": {"type": "string", "enum": ["openai", "anthropic", "gemini", "mistral"]},
		"model":         {"type": "string"},
		"active":        {"type": "boolean"}
	}`))
	s.Require().NoError(err)

	attrMap := make(map[string]AttributeInfo)
	for _, a := range schema.GetAttributes(AttributeFilter{AllowNonCredential: true}) {
		attrMap[a.Attribute] = a
	}

	s.Equal([]string{"openai", "anthropic", "gemini", "mistral"}, attrMap["modelProvider"].Enum)
	s.Empty(attrMap["model"].Enum, "an unconstrained string carries no permitted values")
	s.Empty(attrMap["active"].Enum, "a non-string property carries no permitted values")
}

func (s *SchemaValidateTestSuite) TestGetAttributes_NonCredentialRequiredOnly_EmptySchema() {
	schema := &Schema{properties: map[string]property{}}

	attrs := schema.GetAttributes(AttributeFilter{AllowNonCredential: true, RequiredOnly: true})

	s.Empty(attrs, "empty schema should return no required attributes")
}

func (s *SchemaValidateTestSuite) TestGetAttributes_NonCredentialRequiredOnly_AllCredential() {
	schema, err := CompileSchema(json.RawMessage(`{
		"password": {"type": "string", "required": true, "credential": true},
		"pin":      {"type": "string", "required": true, "credential": true}
	}`))
	s.Require().NoError(err)

	attrs := schema.GetAttributes(AttributeFilter{AllowNonCredential: true, RequiredOnly: true})

	s.Empty(attrs, "all required properties are credentials, result should be empty")
}

func (s *SchemaValidateTestSuite) TestGetAttributes_NonCredentialAllAttrs_IncludesOptional() {
	schema, err := CompileSchema(json.RawMessage(`{
		"email":     {"type": "string", "required": true},
		"firstName": {"type": "string", "required": true, "displayName": "First Name"},
		"password":  {"type": "string", "required": true, "credential": true},
		"age":       {"type": "number"}
	}`))
	s.Require().NoError(err)

	attrs := schema.GetAttributes(AttributeFilter{AllowNonCredential: true})

	// email, firstName, age — password excluded (credential).
	s.Len(attrs, 3, "all non-credential attributes should be returned regardless of required flag")

	attrMap := make(map[string]AttributeInfo, len(attrs))
	for _, a := range attrs {
		attrMap[a.Attribute] = a
	}

	s.True(attrMap["email"].Required)
	s.True(attrMap["firstName"].Required)
	s.Equal("First Name", attrMap["firstName"].DisplayName)
	s.False(attrMap["age"].Required, "optional attribute should be present with Required=false")
	_, hasPassword := attrMap["password"]
	s.False(hasPassword, "credential must always be excluded")
}

func (s *SchemaValidateTestSuite) TestGetAttributes_NonCredentialDisplayNameOnNonStringTypes() {
	schema, err := CompileSchema(json.RawMessage(`{
		"active":  {"type": "boolean", "displayName": "Active Status"},
		"score":   {"type": "number",  "displayName": "Score"},
		"address": {"type": "object",  "displayName": "Address", "properties": {"city": {"type": "string"}}},
		"tags":    {"type": "array",   "displayName": "Tags",    "items": {"type": "string"}}
	}`))
	s.Require().NoError(err)

	attrs := schema.GetAttributes(AttributeFilter{AllowNonCredential: true})
	s.Len(attrs, 4)

	attrMap := make(map[string]AttributeInfo, len(attrs))
	for _, a := range attrs {
		attrMap[a.Attribute] = a
	}
	s.Equal("Active Status", attrMap["active"].DisplayName)
	s.Equal("Score", attrMap["score"].DisplayName)
	s.Equal("Address", attrMap["address"].DisplayName)
	s.Equal("Tags", attrMap["tags"].DisplayName)
}

func (s *SchemaValidateTestSuite) TestGetAttributes_CredentialRequiredOnly_ReturnsOnlyRequiredCredential() {
	schema, err := CompileSchema(json.RawMessage(`{
		"password": {"type": "string", "required": true, "credential": true, "displayName": "Password"},
		"pin":      {"type": "string", "credential": true, "displayName": "PIN"},
		"email":    {"type": "string", "required": true}
	}`))
	s.Require().NoError(err)

	attrs := schema.GetAttributes(AttributeFilter{AllowCredential: true, RequiredOnly: true})

	s.Len(attrs, 1)
	s.Equal("password", attrs[0].Attribute)
	s.Equal("Password", attrs[0].DisplayName)
	s.True(attrs[0].Required)
	s.True(attrs[0].Credential)
}

func (s *SchemaValidateTestSuite) TestGetAttributes_CredentialAllAttrs_IncludesOptional() {
	schema, err := CompileSchema(json.RawMessage(`{
		"password": {"type": "string", "required": true, "credential": true, "displayName": "Password"},
		"pin":      {"type": "string", "credential": true, "displayName": "PIN"},
		"email":    {"type": "string", "required": true}
	}`))
	s.Require().NoError(err)

	attrs := schema.GetAttributes(AttributeFilter{AllowCredential: true})

	s.Len(attrs, 2)
	attrMap := make(map[string]AttributeInfo, len(attrs))
	for _, a := range attrs {
		attrMap[a.Attribute] = a
	}

	s.True(attrMap["password"].Required)
	s.True(attrMap["password"].Credential)
	s.Equal("Password", attrMap["password"].DisplayName)
	s.False(attrMap["pin"].Required)
	s.True(attrMap["pin"].Credential)
	s.Equal("PIN", attrMap["pin"].DisplayName)
	_, hasEmail := attrMap["email"]
	s.False(hasEmail, "non-credential attribute must be excluded")
}

func (s *SchemaValidateTestSuite) TestGetAttributes_AllAttrs_CredentialFieldSet() {
	schema, err := CompileSchema(json.RawMessage(`{
		"password": {"type": "string", "required": true, "credential": true},
		"email":    {"type": "string", "required": true}
	}`))
	s.Require().NoError(err)

	attrs := schema.GetAttributes(AttributeFilter{AllowCredential: true, AllowNonCredential: true})

	s.Len(attrs, 2)
	attrMap := make(map[string]AttributeInfo, len(attrs))
	for _, a := range attrs {
		attrMap[a.Attribute] = a
	}

	s.True(attrMap["password"].Credential, "credential attribute must have Credential=true")
	s.False(attrMap["email"].Credential, "non-credential attribute must have Credential=false")
}

func (s *SchemaValidateTestSuite) TestGetAttributes_UniqueOnlyAndType() {
	schema, err := CompileSchema(json.RawMessage(`{
		"email":    {"type": "string", "required": true, "unique": true},
		"empNo":    {"type": "number", "required": true, "unique": true},
		"nickname": {"type": "string", "unique": false},
		"password": {"type": "string", "required": true, "unique": true, "credential": true}
	}`))
	s.Require().NoError(err)

	// UniqueOnly across credential + non-credential returns every unique attribute, with the Unique
	// and Type output fields populated.
	unique := schema.GetAttributes(AttributeFilter{
		AllowCredential: true, AllowNonCredential: true, UniqueOnly: true,
	})
	byName := make(map[string]AttributeInfo, len(unique))
	for _, a := range unique {
		byName[a.Attribute] = a
		s.True(a.Unique, "%s came back from a UniqueOnly filter", a.Attribute)
	}
	s.Len(unique, 3)
	s.Equal(TypeString, byName["email"].Type)
	s.Equal(TypeNumber, byName["empNo"].Type)
	_, hasNickname := byName["nickname"]
	s.False(hasNickname, "non-unique attribute must be excluded")

	// Combining UniqueOnly + RequiredOnly + string Type + non-credential narrows to a single valid
	// subject-attribute candidate (the same filter used by subject-mapping validation).
	subjectCandidates := schema.GetAttributes(AttributeFilter{
		AllowNonCredential: true, RequiredOnly: true, UniqueOnly: true, Type: TypeString,
	})
	s.Require().Len(subjectCandidates, 1)
	s.Equal("email", subjectCandidates[0].Attribute)
	s.True(subjectCandidates[0].Unique)
	s.True(subjectCandidates[0].Required)
	s.Equal(TypeString, subjectCandidates[0].Type)
}

// validatePayload compiles the definition and validates the payload against it.
func (s *SchemaValidateTestSuite) validatePayload(definition, payload string) bool {
	schema, err := CompileSchema(json.RawMessage(definition))
	s.Require().NoError(err)
	ok, err := schema.Validate(context.Background(), json.RawMessage(payload), s.logger, false)
	s.Require().NoError(err)
	return ok
}

func (s *SchemaValidateTestSuite) TestContactValues_DefaultFormat() {
	testCases := []struct {
		kind  string
		value interface{}
		valid bool
	}{
		{TypeEmail, "User+tag@example.com", true},
		{TypeEmail, "", false},
		{TypeEmail, "Name <a@b.com>", false},
		{TypeEmail, " a@b.com", false},
		{TypeEmail, "a@b.com ", false},
		{TypeEmail, "a b@b.com", false},
		{TypeEmail, "person\u00a0@example.com", false},
		{TypeEmail, "person\v@example.com", false},
		{TypeEmail, "person\u2003@example.com", false},
		{TypeEmail, "person\u2028@example.com", false},
		{TypeEmail, "person\u2029@example.com", false},
		{TypeEmail, "person\ufeff@example.com", false},
		{TypeEmail, "person\u0085@example.com", true},
		{TypeEmail, "person\u200b@example.com", true},
		{TypeEmail, "ü@b.com", true},
		{TypeEmail, "invalid", false},
		{TypeEmail, 7, false},
		{TypeMobile, "+12", true},
		{TypeMobile, "+123456789012345", true},
		{TypeMobile, "+1234567890123456", false},
		{TypeMobile, "+01", false},
		{TypeMobile, "+1", false},
		{TypeMobile, "12345", false},
		{TypeMobile, "+12 345", false},
		{TypeMobile, "+12\n", false},
		{TypeMobile, "", false},
		{TypeMobile, nil, false},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("%s/%v", tc.kind, tc.value), func() {
			payload, err := json.Marshal(map[string]interface{}{"contact": tc.value})
			s.Require().NoError(err)
			s.Equal(tc.valid, s.validatePayload(
				fmt.Sprintf(`{"contact":{"type":%q,"required":true}}`, tc.kind), string(payload)))
		})
	}
}

func (s *SchemaValidateTestSuite) TestContactValues_NestedAndArray() {
	definition := `{
		"work":   {"type": "email", "regex": "@example\\.com$"},
		"nested": {"type": "object", "properties": {"email": {"type": "email"}}},
		"phones": {"type": "array", "items": {"type": "mobile"}}
	}`

	s.True(s.validatePayload(definition,
		`{"work":"a@example.com","nested":{"email":"b@example.com"},"phones":["+123"]}`))
	for _, payload := range []string{
		`{"work":"b@other.com"}`, `{"nested":{"email":"bad"}}`, `{"phones":["bad"]}`, `{"phones":[""]}`,
	} {
		s.False(s.validatePayload(definition, payload), payload)
	}
}

func (s *SchemaValidateTestSuite) TestContactValues_SuppliedEmptyValuesValidated() {
	definition := `{
		"work":   {"type": "email"},
		"phone":  {"type": "mobile"},
		"nested": {"type": "object", "properties": {"email": {"type": "email"}}}
	}`

	s.False(s.validatePayload(definition, `{"work":"","phone":"","nested":{"email":""}}`))
}

func (s *SchemaValidateTestSuite) TestContactValues_Uniqueness() {
	schema, err := CompileSchema(json.RawMessage(`{"work":{"type":"email","unique":true}}`))
	s.Require().NoError(err)

	for _, value := range []string{"a@example.com", ""} {
		ok, err := schema.ValidateUniqueness(context.Background(), map[string]interface{}{"work": value},
			func(attrs map[string]interface{}) (bool, error) {
				s.Equal(value, attrs["work"])
				return true, nil
			}, s.logger)
		s.Require().NoError(err)
		s.False(ok)
	}
}

func (s *SchemaValidateTestSuite) TestContactAttributes_RejectCredentialAndEnum() {
	for _, kind := range []string{TypeEmail, TypeMobile} {
		for _, definition := range []string{
			`{"contact":{"type":%q,"credential":true}}`,
			`{"contact":{"type":%q,"enum":["a@example.com","+123"]}}`,
			`{"contacts":{"type":"array","items":{"type":%q,"enum":["a@example.com","+123"]}}}`,
		} {
			_, err := CompileSchema(json.RawMessage(fmt.Sprintf(definition, kind)))
			s.Error(err)
		}
	}
}

func (s *SchemaValidateTestSuite) TestStringAttribute_NoContactFormat() {
	s.True(s.validatePayload(`{"email":{"type":"string"}}`, `{"email":"not an email"}`))
}

func (s *SchemaValidateTestSuite) TestGetAttributes_ContactType() {
	schema, err := CompileSchema(json.RawMessage(`{
		"work":   {"type": "email"},
		"phone":  {"type": "mobile"},
		"nested": {"type": "object", "properties": {"email": {"type": "email"}}}
	}`))
	s.Require().NoError(err)

	attrs := schema.GetAttributes(AttributeFilter{AllowNonCredential: true, Type: TypeEmail})
	s.Require().Len(attrs, 1)
	s.Equal("work", attrs[0].Attribute)
	s.Equal(TypeEmail, attrs[0].Type)
	s.Equal(DisplayAttributeValid, schema.ValidateAsDisplayAttribute("work"))
}

func (s *SchemaValidateTestSuite) TestValidateAsPrimaryContact() {
	schema, err := CompileSchema(json.RawMessage(`{
		"work":   {"type": "email"},
		"phone":  {"type": "mobile"},
		"nested": {"type": "object", "properties": {"email": {"type": "email"}}}
	}`))
	s.Require().NoError(err)

	s.Equal(PrimaryContactValid, schema.ValidateAsPrimaryContact("work", TypeEmail))
	s.Equal(PrimaryContactValid, schema.ValidateAsPrimaryContact("phone", TypeMobile))
	s.Equal(PrimaryContactNotFound, schema.ValidateAsPrimaryContact("missing", TypeEmail))
	s.Equal(PrimaryContactNotFound, schema.ValidateAsPrimaryContact("nested.email", TypeEmail))
	s.Equal(PrimaryContactTypeMismatch, schema.ValidateAsPrimaryContact("work", TypeMobile))
	s.Equal(PrimaryContactTypeMismatch, schema.ValidateAsPrimaryContact("nested", TypeEmail))
}

func (s *SchemaValidateTestSuite) TestIsStringBackedType() {
	for _, attrType := range []string{TypeString, TypeEmail, TypeMobile} {
		s.True(IsStringBackedType(attrType), attrType)
	}
	for _, attrType := range []string{TypeNumber, TypeBoolean, TypeObject, TypeArray} {
		s.False(IsStringBackedType(attrType), attrType)
	}
}

func (s *SchemaValidateTestSuite) TestContactRegex_Override() {
	testCases := []struct {
		name, definition, payload string
		valid                     bool
	}{
		{"default email", `{"contact":{"type":"email"}}`, `{"contact":"a@example.com"}`, true},
		{"default mobile", `{"contact":{"type":"mobile"}}`, `{"contact":"0771234567"}`, false},
		{"custom email", `{"contact":{"type":"email","regex":"^.+@localhost$"}}`, `{"contact":"a@localhost"}`, true},
		{
			"custom email restriction",
			`{"contact":{"type":"email","regex":"^.+@localhost$"}}`,
			`{"contact":"a@example.com"}`, false,
		},
		{
			"custom local mobile",
			`{"contact":{"type":"mobile","regex":"^0[0-9]{9}$"}}`,
			`{"contact":"0771234567"}`, true,
		},
		{
			"custom mobile restriction",
			`{"contact":{"type":"mobile","regex":"^0[0-9]{9}$"}}`,
			`{"contact":"+94771234567"}`, false,
		},
		{"pattern alias", `{"contact":{"type":"mobile","pattern":"^0[0-9]{9}$"}}`, `{"contact":"0771234567"}`, true},
		{"empty regex", `{"contact":{"type":"email","regex":""}}`, `{"contact":"admin-defined-value"}`, false},
		{"empty pattern", `{"contact":{"type":"mobile","pattern":""}}`, `{"contact":"admin-defined-value"}`, false},
		{"empty email regex default", `{"contact":{"type":"email","regex":""}}`, `{"contact":"a@example.com"}`, true},
		{"empty mobile regex default", `{"contact":{"type":"mobile","regex":""}}`, `{"contact":"+94771234567"}`, true},
		{"required empty", `{"contact":{"type":"email","required":true,"regex":".*"}}`, `{"contact":""}`, true},
		{"optional omitted", `{"contact":{"type":"mobile"}}`, `{}`, true},
		{"optional empty default", `{"contact":{"type":"email"}}`, `{"contact":""}`, false},
		{"optional empty allowed by regex", `{"contact":{"type":"email","regex":".*"}}`, `{"contact":""}`, true},
		{"optional empty", `{"contact":{"type":"mobile","regex":"^0[0-9]{9}$"}}`, `{"contact":""}`, false},
		{
			"custom array",
			`{"contact":{"type":"array","items":{"type":"mobile","regex":"^0[0-9]{9}$"}}}`,
			`{"contact":["0771234567"]}`, true,
		},
		{
			"empty array item",
			`{"contact":{"type":"array","items":{"type":"email","regex":".*"}}}`,
			`{"contact":[""]}`, true,
		},
		{
			"custom nested",
			`{"contact":{"type":"object","properties":{"email":{"type":"email","regex":"^.+@localhost$"}}}}`,
			`{"contact":{"email":"a@localhost"}}`, true,
		},
		{"custom wrong JSON type", `{"contact":{"type":"mobile","regex":".*"}}`, `{"contact":7}`, false},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.Equal(tc.valid, s.validatePayload(tc.definition, tc.payload))
		})
	}
}

func (s *SchemaValidateTestSuite) TestContactRegex_InvalidDefinitionRejected() {
	for _, definition := range []string{
		`{"contact":{"type":"email","regex":"("}}`,
		`{"contact":{"type":"mobile","regex":".*","pattern":".*"}}`,
	} {
		_, err := CompileSchema(json.RawMessage(definition))
		s.Error(err, definition)
	}
}
