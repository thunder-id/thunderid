// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/entitytype"
	scim "github.com/thunder-id/thunderid/internal/scim/common"
	scimconfig "github.com/thunder-id/thunderid/internal/scim/config"
	"github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
)

// testGenericBaseURL is used in tests where the base URL value is irrelevant.
const testGenericBaseURL = "https://example.com"

// testGenericLoggerComponentName is used in tests where the logger component name is irrelevant.
const testGenericLoggerComponentName = "SCIMTest"

// newTestSCIMService creates a scimDiscoveryService with a nil user type service.
// This is safe for ServiceProviderConfig tests because GetServiceProviderConfig
// does not use that dependency.
// newTestSCIMService handles new test scim service.
func newTestSCIMService() *scimDiscoveryService {
	return newSCIMDiscoveryService(nil, scimconfig.SCIMConfig{}, testServerStartTime)
}

// --- GetServiceProviderConfig ---

// TestGetServiceProviderConfig_SchemasContainServiceProviderConfigURN tests Get Service Provider Config for
// Schemas Contain Service Provider Config URN.
func TestGetServiceProviderConfig_SchemasContainServiceProviderConfigURN(t *testing.T) {
	svc := newTestSCIMService()
	result := svc.GetServiceProviderConfig(context.Background(), testGenericBaseURL)

	require.Len(t, result.Schemas, 1)
	require.Equal(t, scimServiceProviderConfigSchemaURN, result.Schemas[0])
}

// TestGetServiceProviderConfig_MetaLocation tests Get Service Provider Config for Meta Location.
func TestGetServiceProviderConfig_MetaLocation(t *testing.T) {
	baseURL := testBaseURL
	svc := newTestSCIMService()
	result := svc.GetServiceProviderConfig(context.Background(), baseURL)

	require.Equal(t, "ServiceProviderConfig", result.Meta.ResourceType)
	require.Equal(t, baseURL+"/scim/v2/ServiceProviderConfig", result.Meta.Location)
}

// TestGetServiceProviderConfig_MetaCreatedEqualsLastModified tests Get Service Provider Config for Meta
// Created Equals Last Modified.
func TestGetServiceProviderConfig_MetaCreatedEqualsLastModified(t *testing.T) {
	svc := newTestSCIMService()
	result := svc.GetServiceProviderConfig(context.Background(), testGenericBaseURL)

	require.Equal(t, testServerStartTime, result.Meta.Created)
	require.Equal(t, testServerStartTime, result.Meta.LastModified)
}

// TestGetServiceProviderConfig_CapabilitiesMatchConstants tests Get Service Provider Config for Capabilities
// Match Constants.
func TestGetServiceProviderConfig_CapabilitiesMatchConstants(t *testing.T) {
	svc := newTestSCIMService()
	result := svc.GetServiceProviderConfig(context.Background(), testGenericBaseURL)

	require.Equal(t, scimconfig.PatchSupported, result.Patch.Supported)
	require.Equal(t, scimconfig.BulkSupported, result.Bulk.Supported)
	require.Equal(t, scimconfig.BulkMaxOperations, result.Bulk.MaxOperations)
	require.Equal(t, scimconfig.BulkMaxPayloadSize, result.Bulk.MaxPayloadSize)
	require.Equal(t, scimconfig.FilterSupported, result.Filter.Supported)
	require.Equal(t, scimconfig.FilterMaxResults, result.Filter.MaxResults)
	require.Equal(t, scimconfig.ChangePasswordSupported, result.ChangePassword.Supported)
	require.Equal(t, scimconfig.SortSupported, result.Sort.Supported)
	require.Equal(t, scimconfig.ETagSupported, result.ETag.Supported)
	require.Equal(t, scimconfig.PaginationCursorSupported, result.Pagination.Cursor)
	require.Equal(t, scimconfig.PaginationIndexSupported, result.Pagination.Index)
	require.Equal(t, scimconfig.PaginationDefaultMethod, result.Pagination.DefaultPaginationMethod)
	require.Equal(t, scimconfig.PaginationDefaultPageSize, result.Pagination.DefaultPageSize)
	require.Equal(t, scimconfig.PaginationMaxPageSize, result.Pagination.MaxPageSize)
}

// TestGetServiceProviderConfig_AuthenticationSchemes tests Get Service Provider Config for Authentication Schemes.
func TestGetServiceProviderConfig_AuthenticationSchemes(t *testing.T) {
	svc := newTestSCIMService()
	result := svc.GetServiceProviderConfig(context.Background(), testGenericBaseURL)

	require.NotEmpty(t, result.AuthenticationSchemes)
	scheme := result.AuthenticationSchemes[0]
	require.Equal(t, "oauthbearertoken", scheme.Type)
	require.Equal(t, "OAuth Bearer Token", scheme.Name)
	require.NotEmpty(t, scheme.Description)
}

// TestGetSchema_ResolvesUserTypeNameCaseInsensitively tests Get Schema for Resolves User Type Name Case Insensitively.
func TestGetSchema_ResolvesUserTypeNameCaseInsensitively(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	et := &entitytype.EntityType{
		Name:   "Person",
		Schema: json.RawMessage(`{"userName":{"type":"string","displayName":"User name"}}`),
	}
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(
			&entitytype.EntityTypeListResponse{
				TotalResults: 1,
				Types:        []entitytype.EntityTypeListItem{{Name: "Person"}},
			},
			(*tidcommon.ServiceError)(nil),
		).Once()
	mockET.On("GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, "Person").
		Return(et, (*tidcommon.ServiceError)(nil)).Once()

	svc := newSCIMDiscoveryService(mockET, scimconfig.SCIMConfig{}, testServerStartTime)

	result, svcErr := svc.GetSchema(
		context.Background(),
		"urn:thunderid:params:scim:schemas:person:2.0:User",
		testGenericBaseURL,
	)

	require.Nil(t, svcErr)
	require.NotNil(t, result)
	require.Equal(t, "urn:thunderid:params:scim:schemas:person:2.0:User", result.ID)
}

// --- buildCoreUserSchema ---

// testCoreUserType is a designated core user type whose schema defines userName and emails,
// used by buildCoreUserSchema tests.
var testCoreUserType = entitytype.EntityType{
	Name:   "Employee",
	Schema: json.RawMessage(`{"username":{"type":"string"},"email":{"type":"string","required":true}}`),
}

// TestBuildCoreUserSchema_IDIsCoreURN tests Build Core User Schema for ID Is Core URN.
func TestBuildCoreUserSchema_IDIsCoreURN(t *testing.T) {
	schema, err := buildCoreUserSchema(testGenericBaseURL, testCoreUserType)
	require.NoError(t, err)
	require.Equal(t, scim.SCIMCoreUserSchemaURN, schema.ID)
}

// TestBuildCoreUserSchema_MetaLocation tests Build Core User Schema for Meta Location.
func TestBuildCoreUserSchema_MetaLocation(t *testing.T) {
	baseURL := testBaseURL
	schema, err := buildCoreUserSchema(baseURL, testCoreUserType)
	require.NoError(t, err)
	require.Equal(t, baseURL+"/scim/v2/Schemas/"+scim.SCIMCoreUserSchemaURN, schema.Meta.Location)
	require.Equal(t, "Schema", schema.Meta.ResourceType)
}

// TestBuildCoreUserSchema_ContainsIDAlwaysAndMatchedAttributes tests that "id" is always present
// and that attributes matching the designated type's schema are included, correctly required.
func TestBuildCoreUserSchema_ContainsIDAlwaysAndMatchedAttributes(t *testing.T) {
	schema, err := buildCoreUserSchema(testGenericBaseURL, testCoreUserType)
	require.NoError(t, err)

	byName := make(map[string]scimSchemaAttribute, len(schema.Attributes))
	for _, a := range schema.Attributes {
		byName[a.Name] = a
	}

	require.Contains(t, byName, "id")
	require.Contains(t, byName, "userName")
	require.False(t, byName["userName"].Required)
	require.Contains(t, byName, "emails")
	require.True(t, byName["emails"].Required)
}

// TestBuildCoreUserSchema_OmitsUnmatchedAttributes tests that fields with no matching attribute
// in the designated type's schema are omitted rather than falling back to the RFC default shape.
func TestBuildCoreUserSchema_OmitsUnmatchedAttributes(t *testing.T) {
	schema, err := buildCoreUserSchema(testGenericBaseURL, testCoreUserType)
	require.NoError(t, err)

	names := make([]string, 0, len(schema.Attributes))
	for _, a := range schema.Attributes {
		names = append(names, a.Name)
	}
	require.NotContains(t, names, "phoneNumbers")
	require.NotContains(t, names, "title")
}

// TestBuildCoreUserSchema_FiltersSubAttributesToMatchedOnly tests that a complex attribute's
// sub-attributes are individually filtered to what the designated type's schema actually
// defines, rather than advertising the full RFC sub-attribute set unconditionally.
func TestBuildCoreUserSchema_FiltersSubAttributesToMatchedOnly(t *testing.T) {
	coreType := entitytype.EntityType{
		Name: "Employee",
		Schema: json.RawMessage(
			`{"given_name":{"type":"string"},"street_address":{"type":"string"}}`,
		),
	}
	schema, err := buildCoreUserSchema(testGenericBaseURL, coreType)
	require.NoError(t, err)

	byName := make(map[string]scimSchemaAttribute, len(schema.Attributes))
	for _, a := range schema.Attributes {
		byName[a.Name] = a
	}

	require.Contains(t, byName, "name")
	nameSubs := make([]string, 0, len(byName["name"].SubAttributes))
	for _, s := range byName["name"].SubAttributes {
		nameSubs = append(nameSubs, s.Name)
	}
	require.Equal(t, []string{"givenName"}, nameSubs)

	require.Contains(t, byName, "addresses")
	addrSubs := make(map[string]struct{}, len(byName["addresses"].SubAttributes))
	for _, s := range byName["addresses"].SubAttributes {
		addrSubs[s.Name] = struct{}{}
	}
	// streetAddress is individually matched; formatted/type/primary are protocol-only
	// (no dedicated candidate) and always ride along with the parent match.
	require.Contains(t, addrSubs, "streetAddress")
	require.Contains(t, addrSubs, "formatted")
	require.Contains(t, addrSubs, "type")
	require.Contains(t, addrSubs, "primary")
	require.NotContains(t, addrSubs, "locality")
	require.NotContains(t, addrSubs, "region")
	require.NotContains(t, addrSubs, "postalCode")
	require.NotContains(t, addrSubs, "country")
}

// TestBuildCoreUserSchema_InvalidSchemaJSON_ReturnsError tests that malformed schema JSON on the
// designated core user type surfaces an error instead of silently producing a partial schema.
func TestBuildCoreUserSchema_InvalidSchemaJSON_ReturnsError(t *testing.T) {
	broken := entitytype.EntityType{Name: "Broken", Schema: json.RawMessage(`{INVALID`)}
	_, err := buildCoreUserSchema(testGenericBaseURL, broken)
	require.Error(t, err)
}

// --- scim.ParseUserTypeFromSchemaURN ---

// TestParseUserTypeFromSchemaURN_ValidURN tests Parse User Type From Schema URN for Valid URN.
func TestParseUserTypeFromSchemaURN_ValidURN(t *testing.T) {
	name, ok := scim.ParseUserTypeFromSchemaURN("urn:thunderid:params:scim:schemas:person:2.0:User")
	require.True(t, ok)
	require.Equal(t, "person", name)
}

// TestParseUserTypeFromSchemaURN_UppercaseInput tests Parse User Type From Schema URN for Uppercase Input.
func TestParseUserTypeFromSchemaURN_UppercaseInput(t *testing.T) {
	name, ok := scim.ParseUserTypeFromSchemaURN("URN:THUNDERID:PARAMS:SCIM:SCHEMAS:EMPLOYEE:2.0:USER")
	require.True(t, ok)
	require.Equal(t, "employee", name)
}

// TestParseUserTypeFromSchemaURN_WrongPrefix tests Parse User Type From Schema URN for Wrong Prefix.
func TestParseUserTypeFromSchemaURN_WrongPrefix(t *testing.T) {
	_, ok := scim.ParseUserTypeFromSchemaURN("urn:ietf:params:scim:schemas:core:2.0:User")
	require.False(t, ok)
}

// TestParseUserTypeFromSchemaURN_WrongSuffix tests Parse User Type From Schema URN for Wrong Suffix.
func TestParseUserTypeFromSchemaURN_WrongSuffix(t *testing.T) {
	_, ok := scim.ParseUserTypeFromSchemaURN("urn:thunderid:params:scim:schemas:person:2.0:Group")
	require.False(t, ok)
}

// TestParseUserTypeFromSchemaURN_EmptyName tests Parse User Type From Schema URN for Empty Name.
func TestParseUserTypeFromSchemaURN_EmptyName(t *testing.T) {
	// Construct a URN where prefix and suffix are adjacent (no name in between).
	urn := scim.ThunderIDURNPrefix + scim.ThunderIDURNSuffix
	_, ok := scim.ParseUserTypeFromSchemaURN(urn)
	require.False(t, ok)
}

// TestParseUserTypeFromSchemaURN_EmptyString tests Parse User Type From Schema URN for Empty String.
func TestParseUserTypeFromSchemaURN_EmptyString(t *testing.T) {
	_, ok := scim.ParseUserTypeFromSchemaURN("")
	require.False(t, ok)
}

// --- mapRawPropertyToSCIMAttribute type branches ---

// TestMapRawProperty_StringType tests Map Raw Property for String Type.
func TestMapRawProperty_StringType(t *testing.T) {
	attr := mapRawPropertyToSCIMAttribute("email", scim.RawPropertyDef{Type: "string"})
	require.Equal(t, scimAttrTypeString, attr.Type)
	require.False(t, attr.MultiValued)
}

// TestMapRawProperty_NumberType tests Map Raw Property for Number Type.
func TestMapRawProperty_NumberType(t *testing.T) {
	attr := mapRawPropertyToSCIMAttribute("age", scim.RawPropertyDef{Type: "number"})
	require.Equal(t, scimAttrTypeDecimal, attr.Type)
}

// TestMapRawProperty_BooleanType tests Map Raw Property for Boolean Type.
func TestMapRawProperty_BooleanType(t *testing.T) {
	attr := mapRawPropertyToSCIMAttribute("active", scim.RawPropertyDef{Type: "boolean"})
	require.Equal(t, scimAttrTypeBoolean, attr.Type)
}

// TestMapRawProperty_ObjectType_WithSubAttributes tests Map Raw Property for Object Type With Sub Attributes.
func TestMapRawProperty_ObjectType_WithSubAttributes(t *testing.T) {
	def := scim.RawPropertyDef{
		Type: "object",
		Properties: map[string]scim.RawPropertyDef{
			"street": {Type: "string"},
		},
	}
	attr := mapRawPropertyToSCIMAttribute("address", def)
	require.Equal(t, scimAttrTypeComplex, attr.Type)
	require.Len(t, attr.SubAttributes, 1)
	require.Equal(t, "street", attr.SubAttributes[0].Name)
}

// TestMapRawProperty_ObjectType_NoSubAttributes tests Map Raw Property for Object Type No Sub Attributes.
func TestMapRawProperty_ObjectType_NoSubAttributes(t *testing.T) {
	attr := mapRawPropertyToSCIMAttribute("meta", scim.RawPropertyDef{Type: "object"})
	require.Equal(t, scimAttrTypeComplex, attr.Type)
	require.Empty(t, attr.SubAttributes)
}

// TestMapRawProperty_ArrayType_WithStringItems tests Map Raw Property for Array Type With String Items.
func TestMapRawProperty_ArrayType_WithStringItems(t *testing.T) {
	items := scim.RawPropertyDef{Type: "string"}
	attr := mapRawPropertyToSCIMAttribute("emails", scim.RawPropertyDef{Type: "array", Items: &items})
	require.True(t, attr.MultiValued)
	require.Equal(t, scimAttrTypeString, attr.Type)
}

// TestMapRawProperty_ArrayType_WithObjectItems tests Map Raw Property for Array Type With Object Items.
func TestMapRawProperty_ArrayType_WithObjectItems(t *testing.T) {
	items := scim.RawPropertyDef{
		Type: "object",
		Properties: map[string]scim.RawPropertyDef{
			"value": {Type: "string"},
		},
	}
	attr := mapRawPropertyToSCIMAttribute("addresses", scim.RawPropertyDef{Type: "array", Items: &items})
	require.True(t, attr.MultiValued)
	require.Equal(t, scimAttrTypeComplex, attr.Type)
	require.NotEmpty(t, attr.SubAttributes)
}

// TestMapRawProperty_ArrayType_NilItems_DefaultsToString tests Map Raw Property for Array Type Nil Items
// Defaults To String.
func TestMapRawProperty_ArrayType_NilItems_DefaultsToString(t *testing.T) {
	attr := mapRawPropertyToSCIMAttribute("tags", scim.RawPropertyDef{Type: "array", Items: nil})
	require.True(t, attr.MultiValued)
	require.Equal(t, scimAttrTypeString, attr.Type)
}

// TestMapRawProperty_UnknownType_DefaultsToString tests Map Raw Property for Unknown Type Defaults To String.
func TestMapRawProperty_UnknownType_DefaultsToString(t *testing.T) {
	attr := mapRawPropertyToSCIMAttribute("custom", scim.RawPropertyDef{Type: "uuid"})
	require.Equal(t, scimAttrTypeString, attr.Type)
}

// TestMapRawProperty_CredentialField tests Map Raw Property for Credential Field.
func TestMapRawProperty_CredentialField(t *testing.T) {
	attr := mapRawPropertyToSCIMAttribute("password", scim.RawPropertyDef{Type: "string", Credential: true})
	require.Equal(t, scimReturnedNever, attr.Returned)
	require.Equal(t, scimMutabilityWriteOnly, attr.Mutability)
	require.True(t, attr.CaseExact)
}

// TestMapRawProperty_UniqueField tests Map Raw Property for Unique Field.
func TestMapRawProperty_UniqueField(t *testing.T) {
	attr := mapRawPropertyToSCIMAttribute("username", scim.RawPropertyDef{Type: "string", Unique: true})
	require.Equal(t, scimUniquenessServer, attr.Uniqueness)
}

// --- mapUserTypeToSCIMSchema ---

// TestMapUserTypeToSCIMSchema_InvalidJSON_ReturnsError tests Map User Type To SCIM Schema for Invalid JSON
// Returns Error.
func TestMapUserTypeToSCIMSchema_InvalidJSON_ReturnsError(t *testing.T) {
	et := entitytype.EntityType{
		Name:   "Broken",
		Schema: json.RawMessage(`{INVALID`),
	}
	_, err := mapUserTypeToSCIMSchema(et, testGenericBaseURL)
	require.Error(t, err)
}

// TestMapUserTypeToSCIMSchema_ValidSchema tests Map User Type To SCIM Schema for Valid Schema.
func TestMapUserTypeToSCIMSchema_ValidSchema(t *testing.T) {
	et := entitytype.EntityType{
		Name:   "Employee",
		Schema: json.RawMessage(`{"userName":{"type":"string","displayName":"User Name"}}`),
	}
	schema, err := mapUserTypeToSCIMSchema(et, testGenericBaseURL)
	require.NoError(t, err)
	require.Equal(t, "urn:thunderid:params:scim:schemas:employee:2.0:User", schema.ID)
	require.Len(t, schema.Attributes, 1)
	require.Equal(t, "userName", schema.Attributes[0].Name)
}

// --- GetSchema additional branches ---

// TestGetSchema_CoreUserURN_SingleUserType_DerivesSchema tests that with exactly one
// configured user type, GetSchema for the core User URN derives its attributes from that type.
func TestGetSchema_CoreUserURN_SingleUserType_DerivesSchema(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(
			&entitytype.EntityTypeListResponse{
				TotalResults: 1,
				Types:        []entitytype.EntityTypeListItem{{Name: "Employee"}},
			},
			(*tidcommon.ServiceError)(nil),
		)
	mockET.On("GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, "Employee").
		Return(&testCoreUserType, (*tidcommon.ServiceError)(nil))

	svc := newSCIMDiscoveryService(mockET, scimconfig.SCIMConfig{}, testServerStartTime)
	schema, svcErr := svc.GetSchema(context.Background(), scim.SCIMCoreUserSchemaURN, testGenericBaseURL)
	require.Nil(t, svcErr)
	require.NotNil(t, schema)
	require.Equal(t, scim.SCIMCoreUserSchemaURN, schema.ID)
	require.Equal(t, "User", schema.Name)
}

// TestGetSchema_CoreUserURN_NoUserTypes_Returns404 tests that with no configured user types,
// and no CoreUserTypeID configured, the core schema is unavailable rather than falling back
// to a static, potentially inaccurate declaration.
func TestGetSchema_CoreUserURN_NoUserTypes_Returns404(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(
			&entitytype.EntityTypeListResponse{TotalResults: 0, Types: nil},
			(*tidcommon.ServiceError)(nil),
		)

	svc := newSCIMDiscoveryService(mockET, scimconfig.SCIMConfig{}, testServerStartTime)
	schema, svcErr := svc.GetSchema(context.Background(), scim.SCIMCoreUserSchemaURN, testGenericBaseURL)
	require.Nil(t, schema)
	require.NotNil(t, svcErr)
	require.Equal(t, scim.ErrorSchemaNotFound.Code, svcErr.Code)
}

// TestGetSchema_UnknownURN_Returns404 tests Get Schema for Unknown URN Returns 404.
func TestGetSchema_UnknownURN_Returns404(t *testing.T) {
	svc := newSCIMDiscoveryService(nil, scimconfig.SCIMConfig{}, testServerStartTime)
	schema, svcErr := svc.GetSchema(context.Background(), "urn:unknown:schema", testGenericBaseURL)
	require.Nil(t, schema)
	require.NotNil(t, svcErr)
	require.Equal(t, scim.ErrorSchemaNotFound.Code, svcErr.Code)
}

// TestGetSchema_UserTypeNotFound_Returns404 tests Get Schema for User Type Not Found Returns 404.
func TestGetSchema_UserTypeNotFound_Returns404(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(
			&entitytype.EntityTypeListResponse{
				TotalResults: 1,
				Types:        []entitytype.EntityTypeListItem{{Name: "Ghost"}},
			},
			(*tidcommon.ServiceError)(nil),
		)
	mockET.On("GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, "Ghost").
		Return((*entitytype.EntityType)(nil), &tidcommon.ServiceError{Code: "ET-404"})

	svc := newSCIMDiscoveryService(mockET, scimconfig.SCIMConfig{}, testServerStartTime)

	schema, svcErr := svc.GetSchema(
		context.Background(),
		"urn:thunderid:params:scim:schemas:ghost:2.0:User",
		testGenericBaseURL,
	)
	require.Nil(t, schema)
	require.NotNil(t, svcErr)
	require.Equal(t, scim.ErrorSchemaNotFound.Code, svcErr.Code)
}

// --- ListSchemas ---

// TestListSchemas_NoUserTypes_OmitsCoreUserSchema tests that with no configured user types
// (and no CoreUserTypeID configured), ListSchemas omits the core User schema rather than
// falling back to a static, potentially inaccurate declaration — only the Group schema remains.
func TestListSchemas_NoUserTypes_OmitsCoreUserSchema(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(
			&entitytype.EntityTypeListResponse{TotalResults: 0, Types: nil},
			(*tidcommon.ServiceError)(nil),
		)

	svc := newSCIMDiscoveryService(mockET, scimconfig.SCIMConfig{}, testServerStartTime)

	resp, svcErr := svc.ListSchemas(context.Background(), testGenericBaseURL)
	require.Nil(t, svcErr)

	schemas := resp.Resources
	require.Len(t, schemas, 1)
	require.Equal(t, scim.SCIMCoreGroupSchemaURN, schemas[0].ID)
}

// TestListSchemas_IncludesExtensionSchemasForEachUserType tests List Schemas for Includes Extension Schemas
// For Each User Type.
func TestListSchemas_IncludesExtensionSchemasForEachUserType(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(
			&entitytype.EntityTypeListResponse{
				TotalResults: 1,
				Types:        []entitytype.EntityTypeListItem{{Name: "Customer"}},
			},
			(*tidcommon.ServiceError)(nil),
		)
	mockET.On("GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, "Customer").
		Return(
			&entitytype.EntityType{Name: "Customer", Schema: json.RawMessage(`{"email":{"type":"string"}}`)},
			(*tidcommon.ServiceError)(nil),
		)

	svc := newSCIMDiscoveryService(mockET, scimconfig.SCIMConfig{}, testServerStartTime)

	resp, svcErr := svc.ListSchemas(context.Background(), testGenericBaseURL)
	require.Nil(t, svcErr)

	schemas := resp.Resources
	// User schema + Group schema + 1 user-type extension = 3
	require.Equal(t, 3, resp.TotalResults)
	require.Len(t, schemas, 3)

	urns := make([]string, 0, len(schemas))
	for _, s := range schemas {
		urns = append(urns, s.ID)
	}
	require.Contains(t, urns, scim.SCIMCoreUserSchemaURN)
	require.Contains(t, urns, scim.SCIMCoreGroupSchemaURN)
	require.Contains(t, urns, "urn:thunderid:params:scim:schemas:customer:2.0:User")
}

// TestListSchemas_IncludesCoreGroupSchema tests List Schemas for Includes Core Group Schema.
func TestListSchemas_IncludesCoreGroupSchema(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(
			&entitytype.EntityTypeListResponse{TotalResults: 0, Types: nil},
			(*tidcommon.ServiceError)(nil),
		)

	svc := newSCIMDiscoveryService(mockET, scimconfig.SCIMConfig{}, testServerStartTime)

	resp, svcErr := svc.ListSchemas(context.Background(), testGenericBaseURL)
	require.Nil(t, svcErr)

	urns := make([]string, 0, len(resp.Resources))
	for _, s := range resp.Resources {
		urns = append(urns, s.ID)
	}
	require.Contains(t, urns, scim.SCIMCoreGroupSchemaURN)
}

// TestListSchemas_SchemasField tests List Schemas for Schemas Field.
func TestListSchemas_SchemasField(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(
			&entitytype.EntityTypeListResponse{TotalResults: 0, Types: nil},
			(*tidcommon.ServiceError)(nil),
		)

	svc := newSCIMDiscoveryService(mockET, scimconfig.SCIMConfig{}, testServerStartTime)

	resp, svcErr := svc.ListSchemas(context.Background(), testGenericBaseURL)
	require.Nil(t, svcErr)
	require.Equal(t, []string{scim.SCIMListResponseSchemaURN}, resp.Schemas)
}

// TestListSchemas_TotalResultsMatchesResourceCount tests List Schemas for Total Results Matches Resource Count.
func TestListSchemas_TotalResultsMatchesResourceCount(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(
			&entitytype.EntityTypeListResponse{TotalResults: 0, Types: nil},
			(*tidcommon.ServiceError)(nil),
		)

	svc := newSCIMDiscoveryService(mockET, scimconfig.SCIMConfig{}, testServerStartTime)

	resp, svcErr := svc.ListSchemas(context.Background(), testGenericBaseURL)
	require.Nil(t, svcErr)

	schemas := resp.Resources // ← direct access, no type assertion
	require.Equal(t, resp.TotalResults, len(schemas))
	require.Equal(t, 1, resp.StartIndex)
}

// =====================================================================
// GetSchema — additional branch coverage
// =====================================================================

// TestGetSchema_EmptyURN_Returns404 tests Get Schema for Empty URN Returns 404.
func TestGetSchema_EmptyURN_Returns404(t *testing.T) {
	svc := newSCIMDiscoveryService(nil, scimconfig.SCIMConfig{}, testServerStartTime)
	schema, svcErr := svc.GetSchema(context.Background(), "   ", testGenericBaseURL)
	require.Nil(t, schema)
	require.NotNil(t, svcErr)
	require.Equal(t, scim.ErrorSchemaNotFound.Code, svcErr.Code)
}

// TestGetSchema_AuthErrorFromResolve_Returns404 tests Get Schema for Auth Error From Resolve Returns 404.
func TestGetSchema_AuthErrorFromResolve_Returns404(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	authErr := tidcommon.ErrorUnauthorized
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return((*entitytype.EntityTypeListResponse)(nil), &authErr)

	svc := newSCIMDiscoveryService(mockET, scimconfig.SCIMConfig{}, testServerStartTime)

	schema, svcErr := svc.GetSchema(
		context.Background(),
		"urn:thunderid:params:scim:schemas:employee:2.0:User",
		testGenericBaseURL,
	)
	require.Nil(t, schema)
	require.NotNil(t, svcErr)
	require.Equal(t, scim.ErrorSchemaNotFound.Code, svcErr.Code)
}

// TestGetSchema_UserTypeNameNotFoundAfterList_Returns404 tests Get Schema for User Type Name Not Found After
// List Returns 404.
func TestGetSchema_UserTypeNameNotFoundAfterList_Returns404(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(
			&entitytype.EntityTypeListResponse{
				TotalResults: 1,
				Types:        []entitytype.EntityTypeListItem{{Name: "OtherType"}},
			},
			(*tidcommon.ServiceError)(nil),
		)

	svc := newSCIMDiscoveryService(mockET, scimconfig.SCIMConfig{}, testServerStartTime)

	schema, svcErr := svc.GetSchema(
		context.Background(),
		"urn:thunderid:params:scim:schemas:ghost:2.0:User",
		testGenericBaseURL,
	)
	require.Nil(t, schema)
	require.NotNil(t, svcErr)
	require.Equal(t, scim.ErrorSchemaNotFound.Code, svcErr.Code)
}

// TestGetSchema_AuthErrorFromGetEntityTypeByName_Returns404 tests Get Schema for Auth Error From Get Entity
// Type By Name Returns 404.
func TestGetSchema_AuthErrorFromGetEntityTypeByName_Returns404(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(
			&entitytype.EntityTypeListResponse{
				TotalResults: 1,
				Types:        []entitytype.EntityTypeListItem{{Name: "Employee"}},
			},
			(*tidcommon.ServiceError)(nil),
		)

	authErr := tidcommon.ErrorUnauthorized
	mockET.On("GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, "Employee").
		Return((*entitytype.EntityType)(nil), &authErr)

	svc := newSCIMDiscoveryService(mockET, scimconfig.SCIMConfig{}, testServerStartTime)

	schema, svcErr := svc.GetSchema(
		context.Background(),
		"urn:thunderid:params:scim:schemas:employee:2.0:User",
		testGenericBaseURL,
	)
	require.Nil(t, schema)
	require.NotNil(t, svcErr)
	require.Equal(t, scim.ErrorSchemaNotFound.Code, svcErr.Code)
}

// TestGetSchema_MalformedUserTypeSchema_Returns500 tests Get Schema for Malformed User Type Schema Returns 500.
func TestGetSchema_MalformedUserTypeSchema_Returns500(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(
			&entitytype.EntityTypeListResponse{
				TotalResults: 1,
				Types:        []entitytype.EntityTypeListItem{{Name: "Broken"}},
			},
			(*tidcommon.ServiceError)(nil),
		)
	mockET.On("GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, "Broken").
		Return(
			&entitytype.EntityType{Name: "Broken", Schema: json.RawMessage(`{INVALID JSON`)},
			(*tidcommon.ServiceError)(nil),
		)

	svc := newSCIMDiscoveryService(mockET, scimconfig.SCIMConfig{}, testServerStartTime)

	schema, svcErr := svc.GetSchema(
		context.Background(),
		"urn:thunderid:params:scim:schemas:broken:2.0:User",
		testGenericBaseURL,
	)
	require.Nil(t, schema)
	require.NotNil(t, svcErr)
	require.Equal(t, scim.ErrorInternalServer.Code, svcErr.Code)
}

// =====================================================================
// ListSchemas — error and pagination branch coverage
// =====================================================================

// TestListSchemas_GetEntityTypeListError_ReturnsError tests List Schemas for Get Entity Type List Error Returns Error.
func TestListSchemas_GetEntityTypeListError_ReturnsError(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return((*entitytype.EntityTypeListResponse)(nil), &tidcommon.ServiceError{Code: "ET-500"})

	svc := newSCIMDiscoveryService(mockET, scimconfig.SCIMConfig{}, testServerStartTime)

	resp, svcErr := svc.ListSchemas(context.Background(), testGenericBaseURL)
	require.NotNil(t, svcErr)
	require.Empty(t, resp.Resources)
}

// TestListSchemas_GetEntityTypeByNameError_SkipsItem tests that a GetEntityTypeByName failure
// for the sole (and therefore auto-designated core) user type omits both the core User schema
// and that type's extension schema — only the Group schema remains.
func TestListSchemas_GetEntityTypeByNameError_SkipsItem(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(
			&entitytype.EntityTypeListResponse{
				TotalResults: 1,
				Types:        []entitytype.EntityTypeListItem{{Name: "Broken"}},
			},
			(*tidcommon.ServiceError)(nil),
		)
	mockET.On("GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, "Broken").
		Return((*entitytype.EntityType)(nil), &tidcommon.ServiceError{Code: "ET-404"})

	svc := newSCIMDiscoveryService(mockET, scimconfig.SCIMConfig{}, testServerStartTime)

	resp, svcErr := svc.ListSchemas(context.Background(), testGenericBaseURL)
	require.Nil(t, svcErr)
	require.Equal(t, 1, resp.TotalResults)
	require.Equal(t, scim.SCIMCoreGroupSchemaURN, resp.Resources[0].ID)
}

// TestListSchemas_MalformedUserTypeSchema_SkipsItem tests that a malformed schema on the sole
// (and therefore auto-designated core) user type omits both the core User schema and that
// type's extension schema — only the Group schema remains.
func TestListSchemas_MalformedUserTypeSchema_SkipsItem(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(
			&entitytype.EntityTypeListResponse{
				TotalResults: 1,
				Types:        []entitytype.EntityTypeListItem{{Name: "Bad"}},
			},
			(*tidcommon.ServiceError)(nil),
		)
	mockET.On("GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, "Bad").
		Return(
			&entitytype.EntityType{Name: "Bad", Schema: json.RawMessage(`{BAD`)},
			(*tidcommon.ServiceError)(nil),
		)

	svc := newSCIMDiscoveryService(mockET, scimconfig.SCIMConfig{}, testServerStartTime)

	resp, svcErr := svc.ListSchemas(context.Background(), testGenericBaseURL)
	require.Nil(t, svcErr)
	require.Equal(t, 1, resp.TotalResults)
	require.Equal(t, scim.SCIMCoreGroupSchemaURN, resp.Resources[0].ID)
}

// TestListSchemas_PaginationFetchesSecondPage tests List Schemas for Pagination Fetches Second Page.
func TestListSchemas_PaginationFetchesSecondPage(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, 0, false).
		Return(
			&entitytype.EntityTypeListResponse{
				TotalResults: 2,
				Types:        []entitytype.EntityTypeListItem{{Name: "TypeA"}},
			},
			(*tidcommon.ServiceError)(nil),
		).Once()
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, 1, false).
		Return(
			&entitytype.EntityTypeListResponse{
				TotalResults: 2,
				Types:        []entitytype.EntityTypeListItem{{Name: "TypeB"}},
			},
			(*tidcommon.ServiceError)(nil),
		).Once()

	mockET.On("GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, "TypeA").
		Return(
			&entitytype.EntityType{Name: "TypeA", Schema: json.RawMessage(`{"field":{"type":"string"}}`)},
			(*tidcommon.ServiceError)(nil),
		).Once()
	mockET.On("GetEntityTypeByName", mock.Anything, entitytype.TypeCategoryUser, "TypeB").
		Return(
			&entitytype.EntityType{Name: "TypeB", Schema: json.RawMessage(`{"field":{"type":"string"}}`)},
			(*tidcommon.ServiceError)(nil),
		).Once()

	svc := newSCIMDiscoveryService(mockET, scimconfig.SCIMConfig{}, testServerStartTime)

	resp, svcErr := svc.ListSchemas(context.Background(), testGenericBaseURL)
	require.Nil(t, svcErr)
	// Group schema + 2 user-type extensions = 3. Core User schema is omitted: with 2
	// configured user types and no CoreUserTypeID set, which one backs it is ambiguous.
	require.Equal(t, 3, resp.TotalResults)
}

// =====================================================================
// ResolveUserTypeNameForSchemaURN — branch coverage
// =====================================================================

// TestResolveUserTypeName_AuthError_Returns404 tests Resolve User Type Name for Auth Error Returns 404.
func TestResolveUserTypeName_AuthError_Returns404(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	authErr := tidcommon.ErrorUnauthorized
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return((*entitytype.EntityTypeListResponse)(nil), &authErr)

	svc := newSCIMDiscoveryService(mockET, scimconfig.SCIMConfig{}, testServerStartTime)

	_, svcErr := svc.GetSchema(
		context.Background(),
		"urn:thunderid:params:scim:schemas:anytype:2.0:User",
		testGenericBaseURL,
	)
	require.NotNil(t, svcErr)
	require.Equal(t, scim.ErrorSchemaNotFound.Code, svcErr.Code)
}

// TestResolveUserTypeName_NonAuthListError_Returns404 tests Resolve User Type Name for Non Auth List Error Returns 404.
func TestResolveUserTypeName_NonAuthListError_Returns404(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return((*entitytype.EntityTypeListResponse)(nil), &tidcommon.ServiceError{Code: "ET-DB-ERR"})

	svc := newSCIMDiscoveryService(mockET, scimconfig.SCIMConfig{}, testServerStartTime)

	_, svcErr := svc.GetSchema(
		context.Background(),
		"urn:thunderid:params:scim:schemas:anytype:2.0:User",
		testGenericBaseURL,
	)
	require.NotNil(t, svcErr)
	require.Equal(t, scim.ErrorSchemaNotFound.Code, svcErr.Code)
}

// =====================================================================
// ListResourceTypes — service-layer tests
// =====================================================================

// TestListResourceTypes_ReturnsUserAndGroupResourceType tests List Resource Types for Returns User And Group
// Resource Type.
func TestListResourceTypes_ReturnsUserAndGroupResourceType(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(
			&entitytype.EntityTypeListResponse{TotalResults: 0, Types: nil},
			(*tidcommon.ServiceError)(nil),
		)

	svc := newSCIMDiscoveryService(mockET, scimconfig.SCIMConfig{}, testServerStartTime)

	resp, svcErr := svc.ListResourceTypes(context.Background(), testGenericBaseURL)
	require.Nil(t, svcErr)
	require.Equal(t, 2, resp.TotalResults)
	require.Len(t, resp.Resources, 2)
	ids := []string{resp.Resources[0].ID, resp.Resources[1].ID}
	require.Contains(t, ids, scimResourceTypeUserID)
	require.Contains(t, ids, scimResourceTypeGroupID)
}

// TestListResourceTypes_SchemasField tests List Resource Types for Schemas Field.
func TestListResourceTypes_SchemasField(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(
			&entitytype.EntityTypeListResponse{TotalResults: 0, Types: nil},
			(*tidcommon.ServiceError)(nil),
		)

	svc := newSCIMDiscoveryService(mockET, scimconfig.SCIMConfig{}, testServerStartTime)

	resp, svcErr := svc.ListResourceTypes(context.Background(), testGenericBaseURL)
	require.Nil(t, svcErr)
	require.Equal(t, []string{scim.SCIMListResponseSchemaURN}, resp.Schemas)
	require.Equal(t, 1, resp.StartIndex)
	require.Equal(t, 2, resp.ItemsPerPage)
}

// TestListResourceTypes_IncludesExtensionPerUserType tests List Resource Types for Includes Extension Per User Type.
func TestListResourceTypes_IncludesExtensionPerUserType(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(
			&entitytype.EntityTypeListResponse{
				TotalResults: 1,
				Types:        []entitytype.EntityTypeListItem{{Name: "Employee"}},
			},
			(*tidcommon.ServiceError)(nil),
		)

	svc := newSCIMDiscoveryService(mockET, scimconfig.SCIMConfig{}, testServerStartTime)

	resp, svcErr := svc.ListResourceTypes(context.Background(), testGenericBaseURL)
	require.Nil(t, svcErr)
	require.Len(t, resp.Resources[0].SchemaExtensions, 1)
	require.Equal(t, scim.BuildSchemaURN("Employee"), resp.Resources[0].SchemaExtensions[0].Schema)
	require.False(t, resp.Resources[0].SchemaExtensions[0].Required)
}

// TestListResourceTypes_EntityTypeListError_ReturnsError tests List Resource Types for Entity Type List Error
// Returns Error.
func TestListResourceTypes_EntityTypeListError_ReturnsError(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return((*entitytype.EntityTypeListResponse)(nil), &tidcommon.ServiceError{Code: "ET-500"})

	svc := newSCIMDiscoveryService(mockET, scimconfig.SCIMConfig{}, testServerStartTime)

	resp, svcErr := svc.ListResourceTypes(context.Background(), testGenericBaseURL)
	require.NotNil(t, svcErr)
	require.Empty(t, resp.Resources)
}

// TestListResourceTypes_MetaLocationContainsBaseURL tests List Resource Types for Meta Location Contains Base URL.
func TestListResourceTypes_MetaLocationContainsBaseURL(t *testing.T) {
	baseURL := testBaseURL
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(
			&entitytype.EntityTypeListResponse{TotalResults: 0, Types: nil},
			(*tidcommon.ServiceError)(nil),
		)

	svc := newSCIMDiscoveryService(mockET, scimconfig.SCIMConfig{}, testServerStartTime)

	resp, svcErr := svc.ListResourceTypes(context.Background(), baseURL)
	require.Nil(t, svcErr)
	rt := resp.Resources[0]
	require.Contains(t, rt.Meta.Location, baseURL)
	require.Contains(t, rt.Meta.Location, scimResourceTypeUserID)
}

// =====================================================================
// GetResourceType — service-layer tests
// =====================================================================

// TestGetResourceType_UserID_ReturnsUserResourceType tests Get Resource Type for User ID Returns User Resource Type.
func TestGetResourceType_UserID_ReturnsUserResourceType(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(
			&entitytype.EntityTypeListResponse{TotalResults: 0, Types: nil},
			(*tidcommon.ServiceError)(nil),
		)

	svc := newSCIMDiscoveryService(mockET, scimconfig.SCIMConfig{}, testServerStartTime)

	rt, svcErr := svc.GetResourceType(context.Background(), "User", testGenericBaseURL)
	require.Nil(t, svcErr)
	require.NotNil(t, rt)
	require.Equal(t, scimResourceTypeUserID, rt.ID)
	require.Equal(t, scimResourceTypeUserName, rt.Name)
}

// TestGetResourceType_CaseInsensitiveID tests Get Resource Type for Case Insensitive ID.
func TestGetResourceType_CaseInsensitiveID(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(
			&entitytype.EntityTypeListResponse{TotalResults: 0, Types: nil},
			(*tidcommon.ServiceError)(nil),
		)

	svc := newSCIMDiscoveryService(mockET, scimconfig.SCIMConfig{}, testServerStartTime)

	rt, svcErr := svc.GetResourceType(context.Background(), "user", testGenericBaseURL)
	require.Nil(t, svcErr)
	require.NotNil(t, rt)
}

// TestGetResourceType_UnknownID_Returns404 tests Get Resource Type for Unknown ID Returns 404.
func TestGetResourceType_UnknownID_Returns404(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)

	svc := newSCIMDiscoveryService(mockET, scimconfig.SCIMConfig{}, testServerStartTime)

	rt, svcErr := svc.GetResourceType(context.Background(), "Unknown", testGenericBaseURL)
	require.Nil(t, rt)
	require.NotNil(t, svcErr)
	require.Equal(t, scim.ErrorResourceTypeNotFound.Code, svcErr.Code)
}

// TestGetResourceType_EntityTypeListError_Propagates tests Get Resource Type for Entity Type List Error Propagates.
func TestGetResourceType_EntityTypeListError_Propagates(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return((*entitytype.EntityTypeListResponse)(nil), &tidcommon.ServiceError{Code: "ET-500"})

	svc := newSCIMDiscoveryService(mockET, scimconfig.SCIMConfig{}, testServerStartTime)

	rt, svcErr := svc.GetResourceType(context.Background(), "User", testGenericBaseURL)
	require.Nil(t, rt)
	require.NotNil(t, svcErr)
}

// =====================================================================
// Handler — ResourceType routes
// =====================================================================

// TestHandleResourceTypeListRequest_Success tests Handle Resource Type List Request for Success.
func TestHandleResourceTypeListRequest_Success(t *testing.T) {
	expectedResp := SCIMResourceTypeListResponse{
		Schemas:      []string{scim.SCIMListResponseSchemaURN},
		TotalResults: 2,
		StartIndex:   1,
		ItemsPerPage: 2,
		Resources: []SCIMResourceType{
			{
				ID:     scimResourceTypeUserID,
				Name:   scimResourceTypeUserName,
				Schema: scim.SCIMCoreUserSchemaURN,
			},
			{
				ID:     scimResourceTypeGroupID,
				Name:   scimResourceTypeGroupName,
				Schema: scim.SCIMCoreGroupSchemaURN,
			},
		},
	}

	mockSvc := NewSCIMDiscoveryServiceInterfaceMock(t)
	mockSvc.On("ListResourceTypes", mock.Anything, testBaseURL).
		Return(expectedResp, (*tidcommon.ServiceError)(nil))

	h := &Handler{svc: mockSvc, baseURL: testBaseURL, logger: *log.GetLogger()}
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/ResourceTypes", nil)
	rr := httptest.NewRecorder()

	h.HandleResourceTypeListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, constants.SCIMContentType, rr.Header().Get("Content-Type"))

	var got SCIMResourceTypeListResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, 2, got.TotalResults)
	ids := []string{got.Resources[0].ID, got.Resources[1].ID}
	require.Contains(t, ids, scimResourceTypeUserID)
	require.Contains(t, ids, scimResourceTypeGroupID)
}

// TestHandleResourceTypeListRequest_ServiceError tests Handle Resource Type List Request for Service Error.
func TestHandleResourceTypeListRequest_ServiceError(t *testing.T) {
	mockSvc := NewSCIMDiscoveryServiceInterfaceMock(t)
	mockSvc.On("ListResourceTypes", mock.Anything, testBaseURL).
		Return(SCIMResourceTypeListResponse{}, &scim.ErrorResourceTypeNotFound)

	h := &Handler{svc: mockSvc, baseURL: testBaseURL, logger: *log.GetLogger()}
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/ResourceTypes", nil)
	rr := httptest.NewRecorder()

	h.HandleResourceTypeListRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

// TestHandleResourceTypeGetRequest_Success tests Handle Resource Type Get Request for Success.
func TestHandleResourceTypeGetRequest_Success(t *testing.T) {
	expectedRT := &SCIMResourceType{
		Schemas: []string{scimResourceTypeSchemaURN},
		ID:      scimResourceTypeUserID,
		Name:    scimResourceTypeUserName,
	}

	mockSvc := NewSCIMDiscoveryServiceInterfaceMock(t)
	mockSvc.On("GetResourceType", mock.Anything, scimResourceTypeUserID, testBaseURL).
		Return(expectedRT, (*tidcommon.ServiceError)(nil))

	h := &Handler{svc: mockSvc, baseURL: testBaseURL, logger: *log.GetLogger()}
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/ResourceTypes/User", nil)
	req.SetPathValue("id", scimResourceTypeUserID)
	rr := httptest.NewRecorder()

	h.HandleResourceTypeGetRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, constants.SCIMContentType, rr.Header().Get("Content-Type"))

	var got SCIMResourceType
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, scimResourceTypeUserID, got.ID)
}

// TestHandleResourceTypeGetRequest_NotFound tests Handle Resource Type Get Request for Not Found.
func TestHandleResourceTypeGetRequest_NotFound(t *testing.T) {
	mockSvc := NewSCIMDiscoveryServiceInterfaceMock(t)
	mockSvc.On("GetResourceType", mock.Anything, "Group", testBaseURL).
		Return((*SCIMResourceType)(nil), &scim.ErrorResourceTypeNotFound)

	h := &Handler{svc: mockSvc, baseURL: testBaseURL, logger: *log.GetLogger()}
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/ResourceTypes/Group", nil)
	req.SetPathValue("id", "Group")
	rr := httptest.NewRecorder()

	h.HandleResourceTypeGetRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

// TestHandleResourceTypeGetRequest_MissingID tests Handle Resource Type Get Request for Missing ID.
func TestHandleResourceTypeGetRequest_MissingID(t *testing.T) {
	mockSvc := NewSCIMDiscoveryServiceInterfaceMock(t)

	h := &Handler{svc: mockSvc, baseURL: testBaseURL, logger: *log.GetLogger()}
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/ResourceTypes/", nil)
	// Intentionally do NOT set path value.
	rr := httptest.NewRecorder()

	h.HandleResourceTypeGetRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

// =====================================================================
// scim.HandleSCIMError — remaining branch coverage
// =====================================================================

// TestHandleSCIMError_ServerErrorType_Returns500 tests Handle SCIM Error for Server Error Type Returns 500.
func TestHandleSCIMError_ServerErrorType_Returns500(t *testing.T) {
	svcErr := &tidcommon.InternalServerError
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/test", nil)
	rr := httptest.NewRecorder()

	scim.HandleSCIMError(rr, req, svcErr, testGenericLoggerComponentName)

	require.Equal(t, http.StatusInternalServerError, rr.Code)

	var errResp scim.SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, "500", errResp.Status)
	require.Equal(t, []string{scim.SCIMErrorSchemaURN}, errResp.Schemas)
}

// TestHandleSCIMError_AuthError_Returns403 tests Handle SCIM Error for Auth Error Returns 403.
func TestHandleSCIMError_AuthError_Returns403(t *testing.T) {
	authErr := tidcommon.ErrorUnauthorized
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/test", nil)
	rr := httptest.NewRecorder()

	scim.HandleSCIMError(rr, req, &authErr, testGenericLoggerComponentName)

	require.Equal(t, http.StatusForbidden, rr.Code)

	var errResp scim.SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, "403", errResp.Status)
	require.Empty(t, errResp.ScimType)
}

// TestHandleSCIMError_DefaultFallback_Returns400InvalidValue tests Handle SCIM Error for Default Fallback
// Returns 400 Invalid Value.
func TestHandleSCIMError_DefaultFallback_Returns400InvalidValue(t *testing.T) {
	unknownErr := &tidcommon.ServiceError{Code: "SCIM-UNKNOWN-9999"}
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/test", nil)
	rr := httptest.NewRecorder()

	scim.HandleSCIMError(rr, req, unknownErr, testGenericLoggerComponentName)

	require.Equal(t, http.StatusBadRequest, rr.Code)

	var errResp scim.SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, scim.ScimErrorTypeInvalidValue, errResp.ScimType)
}

// =====================================================================
// rawEnumToStrings
// =====================================================================

// TestRawEnumToStrings_StringValues tests Raw Enum To Strings for String Values.
func TestRawEnumToStrings_StringValues(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`"active"`),
		json.RawMessage(`"inactive"`),
	}
	out := rawEnumToStrings(raw)
	require.Equal(t, []string{"active", "inactive"}, out)
}

// TestRawEnumToStrings_NumberValues tests Raw Enum To Strings for Number Values.
func TestRawEnumToStrings_NumberValues(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`1`),
		json.RawMessage(`3.14`),
	}
	out := rawEnumToStrings(raw)
	require.Equal(t, []string{"1", "3.14"}, out)
}

// TestRawEnumToStrings_EmptySlice tests Raw Enum To Strings for Empty Slice.
func TestRawEnumToStrings_EmptySlice(t *testing.T) {
	out := rawEnumToStrings(nil)
	require.Empty(t, out)
}

// =====================================================================
// mapRawPropertyToSCIMAttribute — enum/canonical-values branches
// =====================================================================

// TestMapRawProperty_StringWithEnum_PopulatesCanonicalValues tests Map Raw Property for String With Enum
// Populates Canonical Values.
func TestMapRawProperty_StringWithEnum_PopulatesCanonicalValues(t *testing.T) {
	def := scim.RawPropertyDef{
		Type: "string",
		Enum: []json.RawMessage{json.RawMessage(`"a"`), json.RawMessage(`"b"`)},
	}
	attr := mapRawPropertyToSCIMAttribute("status", def)
	require.Equal(t, scimAttrTypeString, attr.Type)
	require.Equal(t, []string{"a", "b"}, attr.CanonicalValues)
}

// TestMapRawProperty_NumberWithEnum_PopulatesCanonicalValues tests Map Raw Property for Number With Enum
// Populates Canonical Values.
func TestMapRawProperty_NumberWithEnum_PopulatesCanonicalValues(t *testing.T) {
	def := scim.RawPropertyDef{
		Type: "number",
		Enum: []json.RawMessage{json.RawMessage(`1`), json.RawMessage(`2`)},
	}
	attr := mapRawPropertyToSCIMAttribute("level", def)
	require.Equal(t, scimAttrTypeDecimal, attr.Type)
	require.Equal(t, []string{"1", "2"}, attr.CanonicalValues)
}

// TestMapRawProperty_ArrayWithNumberItems tests Map Raw Property for Array With Number Items.
func TestMapRawProperty_ArrayWithNumberItems(t *testing.T) {
	items := scim.RawPropertyDef{Type: "number"}
	attr := mapRawPropertyToSCIMAttribute("scores", scim.RawPropertyDef{Type: "array", Items: &items})
	require.True(t, attr.MultiValued)
	require.Equal(t, scimAttrTypeDecimal, attr.Type)
}

// TestMapRawProperty_ArrayWithEnumItems_PropagatesCanonicalValues tests Map Raw Property for Array With Enum
// Items Propagates Canonical Values.
func TestMapRawProperty_ArrayWithEnumItems_PropagatesCanonicalValues(t *testing.T) {
	items := scim.RawPropertyDef{
		Type: "string",
		Enum: []json.RawMessage{json.RawMessage(`"x"`)},
	}
	attr := mapRawPropertyToSCIMAttribute("tags", scim.RawPropertyDef{Type: "array", Items: &items})
	require.True(t, attr.MultiValued)
	require.Equal(t, []string{"x"}, attr.CanonicalValues)
}

// =====================================================================
// buildCoreGroupSchema
// =====================================================================

// TestBuildCoreGroupSchema_IDIsGroupURN tests Build Core Group Schema for ID Is Group URN.
func TestBuildCoreGroupSchema_IDIsGroupURN(t *testing.T) {
	schema := buildCoreGroupSchema(testGenericBaseURL)
	require.Equal(t, scim.SCIMCoreGroupSchemaURN, schema.ID)
}

// TestBuildCoreGroupSchema_MetaLocation tests Build Core Group Schema for Meta Location.
func TestBuildCoreGroupSchema_MetaLocation(t *testing.T) {
	baseURL := testBaseURL
	schema := buildCoreGroupSchema(baseURL)
	require.Equal(t, baseURL+"/scim/v2/Schemas/"+scim.SCIMCoreGroupSchemaURN, schema.Meta.Location)
	require.Equal(t, "Schema", schema.Meta.ResourceType)
}

// TestBuildCoreGroupSchema_ContainsRequiredAttributes tests Build Core Group Schema for Contains Required Attributes.
func TestBuildCoreGroupSchema_ContainsRequiredAttributes(t *testing.T) {
	schema := buildCoreGroupSchema(testGenericBaseURL)
	names := make([]string, 0, len(schema.Attributes))
	for _, a := range schema.Attributes {
		names = append(names, a.Name)
	}
	require.Contains(t, names, "id")
	require.Contains(t, names, "displayName")
	require.Contains(t, names, "members")
}

// =====================================================================
// GetSchema — Group URN
// =====================================================================

// TestGetSchema_CoreGroupURN_ReturnsStaticSchema tests Get Schema for Core Group URN Returns Static Schema.
func TestGetSchema_CoreGroupURN_ReturnsStaticSchema(t *testing.T) {
	svc := newSCIMDiscoveryService(nil, scimconfig.SCIMConfig{}, testServerStartTime)
	schema, svcErr := svc.GetSchema(context.Background(), scim.SCIMCoreGroupSchemaURN, testGenericBaseURL)
	require.Nil(t, svcErr)
	require.NotNil(t, schema)
	require.Equal(t, scim.SCIMCoreGroupSchemaURN, schema.ID)
	require.Equal(t, "Group", schema.Name)
}

// TestGetSchema_CoreGroupURN_CaseInsensitive tests Get Schema for Core Group URN Case Insensitive.
func TestGetSchema_CoreGroupURN_CaseInsensitive(t *testing.T) {
	svc := newSCIMDiscoveryService(nil, scimconfig.SCIMConfig{}, testServerStartTime)
	schema, svcErr := svc.GetSchema(
		context.Background(),
		"URN:IETF:PARAMS:SCIM:SCHEMAS:CORE:2.0:GROUP",
		testGenericBaseURL,
	)
	require.Nil(t, svcErr)
	require.NotNil(t, schema)
	require.Equal(t, scim.SCIMCoreGroupSchemaURN, schema.ID)
}

// =====================================================================
// GetResourceType — Group
// =====================================================================

// TestGetResourceType_GroupID_ReturnsGroupResourceType tests Get Resource Type for Group ID Returns Group
// Resource Type.
func TestGetResourceType_GroupID_ReturnsGroupResourceType(t *testing.T) {
	svc := newSCIMDiscoveryService(nil, scimconfig.SCIMConfig{}, testServerStartTime)

	rt, svcErr := svc.GetResourceType(context.Background(), "Group", testGenericBaseURL)
	require.Nil(t, svcErr)
	require.NotNil(t, rt)
	require.Equal(t, scimResourceTypeGroupID, rt.ID)
	require.Equal(t, scimResourceTypeGroupName, rt.Name)
	require.Equal(t, scim.SCIMCoreGroupSchemaURN, rt.Schema)
	require.Empty(t, rt.SchemaExtensions)
}

// TestGetResourceType_GroupID_CaseInsensitive tests Get Resource Type for Group ID Case Insensitive.
func TestGetResourceType_GroupID_CaseInsensitive(t *testing.T) {
	svc := newSCIMDiscoveryService(nil, scimconfig.SCIMConfig{}, testServerStartTime)

	rt, svcErr := svc.GetResourceType(context.Background(), "group", testGenericBaseURL)
	require.Nil(t, svcErr)
	require.NotNil(t, rt)
	require.Equal(t, scimResourceTypeGroupID, rt.ID)
}

// TestGetResourceType_GroupMetaLocation tests Get Resource Type for Group Meta Location.
func TestGetResourceType_GroupMetaLocation(t *testing.T) {
	baseURL := testBaseURL
	svc := newSCIMDiscoveryService(nil, scimconfig.SCIMConfig{}, testServerStartTime)

	rt, svcErr := svc.GetResourceType(context.Background(), "Group", baseURL)
	require.Nil(t, svcErr)
	require.Contains(t, rt.Meta.Location, baseURL)
	require.Contains(t, rt.Meta.Location, scimResourceTypeGroupID)
}
