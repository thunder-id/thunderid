// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/log"
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

func (s *SchemaValidateTestSuite) TestPropertyName_AcceptsSCIMCharset() {
	// RFC 7643 section 2.1 names: a letter, then letters, digits, '$', '-' and '_'. The hyphen is
	// the case reported in issue #1696.
	schema, err := CompileSchema(json.RawMessage(`{
		"silver-mail": {"type": "string", "unique": true},
		"given_name": {"type": "string"},
		"empNo2": {"type": "number"},
		"a$ref": {"type": "string"},
		"address": {"type": "object", "properties": {
			"post-code": {"type": "string"}
		}}
	}`))
	s.Require().NoError(err)
	s.Len(schema.properties, 5)
}

func (s *SchemaValidateTestSuite) TestPropertyName_RejectsOutOfCharset() {
	cases := map[string]string{
		"dot":            `{"a.b": {"type": "string"}}`,
		"at sign":        `{"x@y": {"type": "string"}}`,
		"space":          `{"work phone": {"type": "string"}}`,
		"apostrophe":     `{"o'k": {"type": "string"}}`,
		"double quote":   `{"a\"b": {"type": "string"}}`,
		"backslash":      `{"a\\b": {"type": "string"}}`,
		"non-ascii":      `{"héllo": {"type": "string"}}`,
		"leading digit":  `{"1email": {"type": "string"}}`,
		"leading hyphen": `{"-email": {"type": "string"}}`,
		"underscore":     `{"_email": {"type": "string"}}`,
		"empty":          `{"": {"type": "string"}}`,
		"sql":            `{"drop') = 1 OR 1=1 --": {"type": "string"}}`,
	}
	for name, schemaJSON := range cases {
		_, err := CompileSchema(json.RawMessage(schemaJSON))
		s.Require().Error(err, "%s must be rejected", name)
		s.Contains(err.Error(), "invalid property name", "%s", name)
	}
}

func (s *SchemaValidateTestSuite) TestPropertyName_RejectsOutOfCharsetWhenNested() {
	_, err := CompileSchema(json.RawMessage(`{
		"address": {"type": "object", "properties": {
			"post.code": {"type": "string"}
		}}
	}`))
	s.Require().Error(err)
	s.Contains(err.Error(), "invalid property name 'post.code'")
}
