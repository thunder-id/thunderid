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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/entitytype"
	scimconfig "github.com/thunder-id/thunderid/internal/scim/config"
	"github.com/thunder-id/thunderid/internal/system/constants"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
)

// testGenericBaseURL is used in tests where the base URL value is irrelevant.
const testGenericBaseURL = "https://example.com"

// newTestSCIMService creates a scimService with nil user and entity type services.
// This is safe for ServiceProviderConfig tests because GetServiceProviderConfig
// does not use either of those dependencies.
func newTestSCIMService() *scimService {
	return newSCIMService(nil, nil, scimconfig.SCIMConfig{})
}

// --- GetServiceProviderConfig ---

func TestGetServiceProviderConfig_SchemasContainServiceProviderConfigURN(t *testing.T) {
	svc := newTestSCIMService()
	result := svc.GetServiceProviderConfig(context.Background(), testGenericBaseURL)

	require.Len(t, result.Schemas, 1)
	require.Equal(t, SCIMServiceProviderConfigSchemaURN, result.Schemas[0])
}

func TestGetServiceProviderConfig_MetaLocation(t *testing.T) {
	baseURL := testBaseURL
	svc := newTestSCIMService()
	result := svc.GetServiceProviderConfig(context.Background(), baseURL)

	require.Equal(t, "ServiceProviderConfig", result.Meta.ResourceType)
	require.Equal(t, baseURL+"/scim/v2/ServiceProviderConfig", result.Meta.Location)
}

func TestGetServiceProviderConfig_MetaCreatedEqualsLastModified(t *testing.T) {
	svc := newTestSCIMService()
	result := svc.GetServiceProviderConfig(context.Background(), testGenericBaseURL)

	require.Equal(t, scimServerStartTime, result.Meta.Created)
	require.Equal(t, scimServerStartTime, result.Meta.LastModified)
}

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

	if scimconfig.ETagSupported {
		require.NotEmpty(t, result.Meta.Version)
		require.True(t, strings.HasPrefix(result.Meta.Version, `W/"`),
			"version must follow RFC 7232 weak ETag format W/\"<value>\"")
	} else {
		require.Empty(t, result.Meta.Version)
	}
}

func TestGetServiceProviderConfig_AuthenticationSchemes(t *testing.T) {
	svc := newTestSCIMService()
	result := svc.GetServiceProviderConfig(context.Background(), testGenericBaseURL)

	require.NotEmpty(t, result.AuthenticationSchemes)
	scheme := result.AuthenticationSchemes[0]
	require.Equal(t, "oauthbearertoken", scheme.Type)
	require.Equal(t, "OAuth Bearer Token", scheme.Name)
	require.NotEmpty(t, scheme.Description)
}

// --- computeSCIMConfigVersion ---

func TestComputeSCIMConfigVersion_IsDeterministic(t *testing.T) {
	cfg := scimconfig.SCIMConfig{PublicURL: "https://example.com"}
	require.Equal(t, computeSCIMConfigVersion(cfg), computeSCIMConfigVersion(cfg),
		"version must be identical across calls for the same config")
}

func TestComputeSCIMConfigVersion_ChangesWhenConfigChanges(t *testing.T) {
	v1 := computeSCIMConfigVersion(scimconfig.SCIMConfig{PublicURL: "https://example.com"})
	v2 := computeSCIMConfigVersion(scimconfig.SCIMConfig{PublicURL: "https://example.org"})
	require.NotEqual(t, v1, v2,
		"version must differ when the config changes so SCIM clients can detect updates")
}

func TestComputeSCIMConfigVersion_FollowsWeakETagFormat(t *testing.T) {
	version := computeSCIMConfigVersion(scimconfig.SCIMConfig{PublicURL: "https://example.com"})
	require.True(t, strings.HasPrefix(version, `W/"`), `must start with W/"`)
	require.True(t, strings.HasSuffix(version, `"`), `must end with "`)
}

func TestGetSchema_ResolvesEntityTypeNameCaseInsensitively(t *testing.T) {
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

	svc := newSCIMService(nil, mockET, scimconfig.SCIMConfig{})

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

func TestBuildCoreUserSchema_IDIsCorURN(t *testing.T) {
	schema := buildCoreUserSchema(testGenericBaseURL)
	require.Equal(t, SCIMCoreUserSchemaURN, schema.ID)
}

func TestBuildCoreUserSchema_MetaLocation(t *testing.T) {
	baseURL := testBaseURL
	schema := buildCoreUserSchema(baseURL)
	require.Equal(t, baseURL+"/scim/v2/Schemas/"+SCIMCoreUserSchemaURN, schema.Meta.Location)
	require.Equal(t, "Schema", schema.Meta.ResourceType)
}

func TestBuildCoreUserSchema_ContainsIDAndMetaAttributes(t *testing.T) {
	schema := buildCoreUserSchema(testGenericBaseURL)
	names := make([]string, 0, len(schema.Attributes))
	for _, a := range schema.Attributes {
		names = append(names, a.Name)
	}
	require.Contains(t, names, "id")
}

// --- parseUserTypeFromSchemaURN ---

func TestParseUserTypeFromSchemaURN_ValidURN(t *testing.T) {
	name, ok := parseUserTypeFromSchemaURN("urn:thunderid:params:scim:schemas:person:2.0:User")
	require.True(t, ok)
	require.Equal(t, "person", name)
}

func TestParseUserTypeFromSchemaURN_UppercaseInput(t *testing.T) {
	name, ok := parseUserTypeFromSchemaURN("URN:THUNDERID:PARAMS:SCIM:SCHEMAS:EMPLOYEE:2.0:USER")
	require.True(t, ok)
	require.Equal(t, "employee", name)
}

func TestParseUserTypeFromSchemaURN_WrongPrefix(t *testing.T) {
	_, ok := parseUserTypeFromSchemaURN("urn:ietf:params:scim:schemas:core:2.0:User")
	require.False(t, ok)
}

func TestParseUserTypeFromSchemaURN_WrongSuffix(t *testing.T) {
	_, ok := parseUserTypeFromSchemaURN("urn:thunderid:params:scim:schemas:person:2.0:Group")
	require.False(t, ok)
}

func TestParseUserTypeFromSchemaURN_EmptyName(t *testing.T) {
	// Construct a URN where prefix and suffix are adjacent (no name in between).
	urn := ThunderIDURNPrefix + ThunderIDURNSuffix
	_, ok := parseUserTypeFromSchemaURN(urn)
	require.False(t, ok)
}

func TestParseUserTypeFromSchemaURN_EmptyString(t *testing.T) {
	_, ok := parseUserTypeFromSchemaURN("")
	require.False(t, ok)
}

// --- mapRawPropertyToSCIMAttribute type branches ---

func TestMapRawProperty_StringType(t *testing.T) {
	attr := mapRawPropertyToSCIMAttribute("email", rawPropertyDef{Type: "string"})
	require.Equal(t, scimAttrTypeString, attr.Type)
	require.False(t, attr.MultiValued)
}

func TestMapRawProperty_NumberType(t *testing.T) {
	attr := mapRawPropertyToSCIMAttribute("age", rawPropertyDef{Type: "number"})
	require.Equal(t, scimAttrTypeDecimal, attr.Type)
}

func TestMapRawProperty_BooleanType(t *testing.T) {
	attr := mapRawPropertyToSCIMAttribute("active", rawPropertyDef{Type: "boolean"})
	require.Equal(t, scimAttrTypeBoolean, attr.Type)
}

func TestMapRawProperty_ObjectType_WithSubAttributes(t *testing.T) {
	def := rawPropertyDef{
		Type: "object",
		Properties: map[string]rawPropertyDef{
			"street": {Type: "string"},
		},
	}
	attr := mapRawPropertyToSCIMAttribute("address", def)
	require.Equal(t, scimAttrTypeComplex, attr.Type)
	require.Len(t, attr.SubAttributes, 1)
	require.Equal(t, "street", attr.SubAttributes[0].Name)
}

func TestMapRawProperty_ObjectType_NoSubAttributes(t *testing.T) {
	attr := mapRawPropertyToSCIMAttribute("meta", rawPropertyDef{Type: "object"})
	require.Equal(t, scimAttrTypeComplex, attr.Type)
	require.Empty(t, attr.SubAttributes)
}

func TestMapRawProperty_ArrayType_WithStringItems(t *testing.T) {
	items := rawPropertyDef{Type: "string"}
	attr := mapRawPropertyToSCIMAttribute("emails", rawPropertyDef{Type: "array", Items: &items})
	require.True(t, attr.MultiValued)
	require.Equal(t, scimAttrTypeString, attr.Type)
}

func TestMapRawProperty_ArrayType_WithObjectItems(t *testing.T) {
	items := rawPropertyDef{
		Type: "object",
		Properties: map[string]rawPropertyDef{
			"value": {Type: "string"},
		},
	}
	attr := mapRawPropertyToSCIMAttribute("addresses", rawPropertyDef{Type: "array", Items: &items})
	require.True(t, attr.MultiValued)
	require.Equal(t, scimAttrTypeComplex, attr.Type)
	require.NotEmpty(t, attr.SubAttributes)
}

func TestMapRawProperty_ArrayType_NilItems_DefaultsToString(t *testing.T) {
	attr := mapRawPropertyToSCIMAttribute("tags", rawPropertyDef{Type: "array", Items: nil})
	require.True(t, attr.MultiValued)
	require.Equal(t, scimAttrTypeString, attr.Type)
}

func TestMapRawProperty_UnknownType_DefaultsToString(t *testing.T) {
	attr := mapRawPropertyToSCIMAttribute("custom", rawPropertyDef{Type: "uuid"})
	require.Equal(t, scimAttrTypeString, attr.Type)
}

func TestMapRawProperty_CredentialField(t *testing.T) {
	attr := mapRawPropertyToSCIMAttribute("password", rawPropertyDef{Type: "string", Credential: true})
	require.Equal(t, scimReturnedNever, attr.Returned)
	require.Equal(t, scimMutabilityWriteOnly, attr.Mutability)
	require.True(t, attr.CaseExact)
}

func TestMapRawProperty_UniqueField(t *testing.T) {
	attr := mapRawPropertyToSCIMAttribute("username", rawPropertyDef{Type: "string", Unique: true})
	require.Equal(t, scimUniquenessServer, attr.Uniqueness)
}

// --- mapEntityTypeToSCIMSchema ---

func TestMapEntityTypeToSCIMSchema_InvalidJSON_ReturnsError(t *testing.T) {
	et := entitytype.EntityType{
		Name:   "Broken",
		Schema: json.RawMessage(`{INVALID`),
	}
	_, err := mapEntityTypeToSCIMSchema(et, testGenericBaseURL)
	require.Error(t, err)
}

func TestMapEntityTypeToSCIMSchema_ValidSchema(t *testing.T) {
	et := entitytype.EntityType{
		Name:   "Employee",
		Schema: json.RawMessage(`{"userName":{"type":"string","displayName":"User Name"}}`),
	}
	schema, err := mapEntityTypeToSCIMSchema(et, testGenericBaseURL)
	require.NoError(t, err)
	require.Equal(t, "urn:thunderid:params:scim:schemas:employee:2.0:User", schema.ID)
	require.Len(t, schema.Attributes, 1)
	require.Equal(t, "userName", schema.Attributes[0].Name)
}

// --- GetSchema additional branches ---

func TestGetSchema_CoreUserURN_ReturnsStaticSchema(t *testing.T) {
	svc := newSCIMService(nil, nil, scimconfig.SCIMConfig{})
	schema, svcErr := svc.GetSchema(context.Background(), SCIMCoreUserSchemaURN, testGenericBaseURL)
	require.Nil(t, svcErr)
	require.NotNil(t, schema)
	require.Equal(t, SCIMCoreUserSchemaURN, schema.ID)
	require.Equal(t, "User", schema.Name)
}

func TestGetSchema_UnknownURN_Returns404(t *testing.T) {
	svc := newSCIMService(nil, nil, scimconfig.SCIMConfig{})
	schema, svcErr := svc.GetSchema(context.Background(), "urn:unknown:schema", testGenericBaseURL)
	require.Nil(t, schema)
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorSchemaNotFound.Code, svcErr.Code)
}

func TestGetSchema_EntityTypeNotFound_Returns404(t *testing.T) {
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

	svc := newSCIMService(nil, mockET, scimconfig.SCIMConfig{})

	schema, svcErr := svc.GetSchema(
		context.Background(),
		"urn:thunderid:params:scim:schemas:ghost:2.0:User",
		testGenericBaseURL,
	)
	require.Nil(t, schema)
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorSchemaNotFound.Code, svcErr.Code)
}

// --- ListSchemas ---

func TestListSchemas_IncludesCoreUserSchema(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(
			&entitytype.EntityTypeListResponse{TotalResults: 0, Types: nil},
			(*tidcommon.ServiceError)(nil),
		)

	svc := newSCIMService(nil, mockET, scimconfig.SCIMConfig{})

	resp, svcErr := svc.ListSchemas(context.Background(), testGenericBaseURL)
	require.Nil(t, svcErr)

	schemas := resp.Resources // ← direct access, no type assertion
	require.GreaterOrEqual(t, len(schemas), 1)
	require.Equal(t, SCIMCoreUserSchemaURN, schemas[0].ID)
}

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

	svc := newSCIMService(nil, mockET, scimconfig.SCIMConfig{})

	resp, svcErr := svc.ListSchemas(context.Background(), testGenericBaseURL)
	require.Nil(t, svcErr)

	schemas := resp.Resources // ← direct access, no type assertion
	require.Equal(t, 2, resp.TotalResults)
	require.Len(t, schemas, 2)

	urns := []string{schemas[0].ID, schemas[1].ID}
	require.Contains(t, urns, SCIMCoreUserSchemaURN)
	require.Contains(t, urns, "urn:thunderid:params:scim:schemas:customer:2.0:User")
}

func TestListSchemas_SchemasField(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(
			&entitytype.EntityTypeListResponse{TotalResults: 0, Types: nil},
			(*tidcommon.ServiceError)(nil),
		)

	svc := newSCIMService(nil, mockET, scimconfig.SCIMConfig{})

	resp, svcErr := svc.ListSchemas(context.Background(), testGenericBaseURL)
	require.Nil(t, svcErr)
	require.Equal(t, []string{SCIMListResponseSchemaURN}, resp.Schemas)
}

func TestListSchemas_TotalResultsMatchesResourceCount(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(
			&entitytype.EntityTypeListResponse{TotalResults: 0, Types: nil},
			(*tidcommon.ServiceError)(nil),
		)

	svc := newSCIMService(nil, mockET, scimconfig.SCIMConfig{})

	resp, svcErr := svc.ListSchemas(context.Background(), testGenericBaseURL)
	require.Nil(t, svcErr)

	schemas := resp.Resources // ← direct access, no type assertion
	require.Equal(t, resp.TotalResults, len(schemas))
	require.Equal(t, 1, resp.StartIndex)
}

// =====================================================================
// GetSchema — additional branch coverage
// =====================================================================

func TestGetSchema_EmptyURN_Returns404(t *testing.T) {
	svc := newSCIMService(nil, nil, scimconfig.SCIMConfig{})
	schema, svcErr := svc.GetSchema(context.Background(), "   ", testGenericBaseURL)
	require.Nil(t, schema)
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorSchemaNotFound.Code, svcErr.Code)
}

func TestGetSchema_AuthErrorFromResolve_Returns404(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	authErr := tidcommon.ErrorUnauthorized
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return((*entitytype.EntityTypeListResponse)(nil), &authErr)

	svc := newSCIMService(nil, mockET, scimconfig.SCIMConfig{})

	schema, svcErr := svc.GetSchema(
		context.Background(),
		"urn:thunderid:params:scim:schemas:employee:2.0:User",
		testGenericBaseURL,
	)
	require.Nil(t, schema)
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorSchemaNotFound.Code, svcErr.Code)
}

func TestGetSchema_EntityTypeNameNotFoundAfterList_Returns404(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(
			&entitytype.EntityTypeListResponse{
				TotalResults: 1,
				Types:        []entitytype.EntityTypeListItem{{Name: "OtherType"}},
			},
			(*tidcommon.ServiceError)(nil),
		)

	svc := newSCIMService(nil, mockET, scimconfig.SCIMConfig{})

	schema, svcErr := svc.GetSchema(
		context.Background(),
		"urn:thunderid:params:scim:schemas:ghost:2.0:User",
		testGenericBaseURL,
	)
	require.Nil(t, schema)
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorSchemaNotFound.Code, svcErr.Code)
}

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

	svc := newSCIMService(nil, mockET, scimconfig.SCIMConfig{})

	schema, svcErr := svc.GetSchema(
		context.Background(),
		"urn:thunderid:params:scim:schemas:employee:2.0:User",
		testGenericBaseURL,
	)
	require.Nil(t, schema)
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorSchemaNotFound.Code, svcErr.Code)
}

func TestGetSchema_MalformedEntityTypeSchema_Returns500(t *testing.T) {
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

	svc := newSCIMService(nil, mockET, scimconfig.SCIMConfig{})

	schema, svcErr := svc.GetSchema(
		context.Background(),
		"urn:thunderid:params:scim:schemas:broken:2.0:User",
		testGenericBaseURL,
	)
	require.Nil(t, schema)
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorInternalServer.Code, svcErr.Code)
}

// =====================================================================
// ListSchemas — error and pagination branch coverage
// =====================================================================

func TestListSchemas_GetEntityTypeListError_ReturnsError(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return((*entitytype.EntityTypeListResponse)(nil), &tidcommon.ServiceError{Code: "ET-500"})

	svc := newSCIMService(nil, mockET, scimconfig.SCIMConfig{})

	resp, svcErr := svc.ListSchemas(context.Background(), testGenericBaseURL)
	require.NotNil(t, svcErr)
	require.Empty(t, resp.Resources)
}

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

	svc := newSCIMService(nil, mockET, scimconfig.SCIMConfig{})

	resp, svcErr := svc.ListSchemas(context.Background(), testGenericBaseURL)
	require.Nil(t, svcErr)
	require.Equal(t, 1, resp.TotalResults)
	require.Equal(t, SCIMCoreUserSchemaURN, resp.Resources[0].ID)
}

func TestListSchemas_MalformedEntityTypeSchema_SkipsItem(t *testing.T) {
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

	svc := newSCIMService(nil, mockET, scimconfig.SCIMConfig{})

	resp, svcErr := svc.ListSchemas(context.Background(), testGenericBaseURL)
	require.Nil(t, svcErr)
	require.Equal(t, 1, resp.TotalResults)
	require.Equal(t, SCIMCoreUserSchemaURN, resp.Resources[0].ID)
}

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

	svc := newSCIMService(nil, mockET, scimconfig.SCIMConfig{})

	resp, svcErr := svc.ListSchemas(context.Background(), testGenericBaseURL)
	require.Nil(t, svcErr)
	require.Equal(t, 3, resp.TotalResults)
}

// =====================================================================
// resolveEntityTypeNameForSchemaURN — branch coverage
// =====================================================================

func TestResolveEntityTypeName_AuthError_Returns404(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	authErr := tidcommon.ErrorUnauthorized
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return((*entitytype.EntityTypeListResponse)(nil), &authErr)

	svc := newSCIMService(nil, mockET, scimconfig.SCIMConfig{})

	_, svcErr := svc.GetSchema(
		context.Background(),
		"urn:thunderid:params:scim:schemas:anytype:2.0:User",
		testGenericBaseURL,
	)
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorSchemaNotFound.Code, svcErr.Code)
}

func TestResolveEntityTypeName_NonAuthListError_Returns404(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return((*entitytype.EntityTypeListResponse)(nil), &tidcommon.ServiceError{Code: "ET-DB-ERR"})

	svc := newSCIMService(nil, mockET, scimconfig.SCIMConfig{})

	_, svcErr := svc.GetSchema(
		context.Background(),
		"urn:thunderid:params:scim:schemas:anytype:2.0:User",
		testGenericBaseURL,
	)
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorSchemaNotFound.Code, svcErr.Code)
}

// =====================================================================
// ListResourceTypes — service-layer tests
// =====================================================================

func TestListResourceTypes_ReturnsUserResourceType(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(
			&entitytype.EntityTypeListResponse{TotalResults: 0, Types: nil},
			(*tidcommon.ServiceError)(nil),
		)

	svc := newSCIMService(nil, mockET, scimconfig.SCIMConfig{})

	resp, svcErr := svc.ListResourceTypes(context.Background(), testGenericBaseURL)
	require.Nil(t, svcErr)
	require.Equal(t, 1, resp.TotalResults)
	require.Len(t, resp.Resources, 1)
	require.Equal(t, scimResourceTypeUserID, resp.Resources[0].ID)
}

func TestListResourceTypes_SchemasField(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(
			&entitytype.EntityTypeListResponse{TotalResults: 0, Types: nil},
			(*tidcommon.ServiceError)(nil),
		)

	svc := newSCIMService(nil, mockET, scimconfig.SCIMConfig{})

	resp, svcErr := svc.ListResourceTypes(context.Background(), testGenericBaseURL)
	require.Nil(t, svcErr)
	require.Equal(t, []string{SCIMListResponseSchemaURN}, resp.Schemas)
	require.Equal(t, 1, resp.StartIndex)
	require.Equal(t, 1, resp.ItemsPerPage)
}

func TestListResourceTypes_IncludesExtensionPerEntityType(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(
			&entitytype.EntityTypeListResponse{
				TotalResults: 1,
				Types:        []entitytype.EntityTypeListItem{{Name: "Employee"}},
			},
			(*tidcommon.ServiceError)(nil),
		)

	svc := newSCIMService(nil, mockET, scimconfig.SCIMConfig{})

	resp, svcErr := svc.ListResourceTypes(context.Background(), testGenericBaseURL)
	require.Nil(t, svcErr)
	require.Len(t, resp.Resources[0].SchemaExtensions, 1)
	require.Equal(t, buildSchemaURN("Employee"), resp.Resources[0].SchemaExtensions[0].Schema)
	require.False(t, resp.Resources[0].SchemaExtensions[0].Required)
}

func TestListResourceTypes_EntityTypeListError_ReturnsError(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return((*entitytype.EntityTypeListResponse)(nil), &tidcommon.ServiceError{Code: "ET-500"})

	svc := newSCIMService(nil, mockET, scimconfig.SCIMConfig{})

	resp, svcErr := svc.ListResourceTypes(context.Background(), testGenericBaseURL)
	require.NotNil(t, svcErr)
	require.Empty(t, resp.Resources)
}

func TestListResourceTypes_MetaLocationContainsBaseURL(t *testing.T) {
	baseURL := testBaseURL
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(
			&entitytype.EntityTypeListResponse{TotalResults: 0, Types: nil},
			(*tidcommon.ServiceError)(nil),
		)

	svc := newSCIMService(nil, mockET, scimconfig.SCIMConfig{})

	resp, svcErr := svc.ListResourceTypes(context.Background(), baseURL)
	require.Nil(t, svcErr)
	rt := resp.Resources[0]
	require.Contains(t, rt.Meta.Location, baseURL)
	require.Contains(t, rt.Meta.Location, scimResourceTypeUserID)
}

// =====================================================================
// GetResourceType — service-layer tests
// =====================================================================

func TestGetResourceType_UserID_ReturnsUserResourceType(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(
			&entitytype.EntityTypeListResponse{TotalResults: 0, Types: nil},
			(*tidcommon.ServiceError)(nil),
		)

	svc := newSCIMService(nil, mockET, scimconfig.SCIMConfig{})

	rt, svcErr := svc.GetResourceType(context.Background(), "User", testGenericBaseURL)
	require.Nil(t, svcErr)
	require.NotNil(t, rt)
	require.Equal(t, scimResourceTypeUserID, rt.ID)
	require.Equal(t, scimResourceTypeUserName, rt.Name)
}

func TestGetResourceType_CaseInsensitiveID(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(
			&entitytype.EntityTypeListResponse{TotalResults: 0, Types: nil},
			(*tidcommon.ServiceError)(nil),
		)

	svc := newSCIMService(nil, mockET, scimconfig.SCIMConfig{})

	rt, svcErr := svc.GetResourceType(context.Background(), "user", testGenericBaseURL)
	require.Nil(t, svcErr)
	require.NotNil(t, rt)
}

func TestGetResourceType_UnknownID_Returns404(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)

	svc := newSCIMService(nil, mockET, scimconfig.SCIMConfig{})

	rt, svcErr := svc.GetResourceType(context.Background(), "Group", testGenericBaseURL)
	require.Nil(t, rt)
	require.NotNil(t, svcErr)
	require.Equal(t, ErrorResourceTypeNotFound.Code, svcErr.Code)
}

func TestGetResourceType_EntityTypeListError_Propagates(t *testing.T) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return((*entitytype.EntityTypeListResponse)(nil), &tidcommon.ServiceError{Code: "ET-500"})

	svc := newSCIMService(nil, mockET, scimconfig.SCIMConfig{})

	rt, svcErr := svc.GetResourceType(context.Background(), "User", testGenericBaseURL)
	require.Nil(t, rt)
	require.NotNil(t, svcErr)
}

// =====================================================================
// Handler — ResourceType routes
// =====================================================================

func TestHandleResourceTypeListRequest_Success(t *testing.T) {
	expectedResp := SCIMResourceTypeListResponse{
		Schemas:      []string{SCIMListResponseSchemaURN},
		TotalResults: 1,
		StartIndex:   1,
		ItemsPerPage: 1,
		Resources: []SCIMResourceType{
			{
				ID:     scimResourceTypeUserID,
				Name:   scimResourceTypeUserName,
				Schema: SCIMCoreUserSchemaURN,
			},
		},
	}

	mockSvc := NewSCIMServiceInterfaceMock(t)
	mockSvc.On("ListResourceTypes", mock.Anything, testBaseURL).
		Return(expectedResp, (*tidcommon.ServiceError)(nil))

	h := newSCIMHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/ResourceTypes", nil)
	rr := httptest.NewRecorder()

	h.HandleResourceTypeListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, constants.SCIMContentType, rr.Header().Get("Content-Type"))

	var got SCIMResourceTypeListResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, 1, got.TotalResults)
	require.Equal(t, scimResourceTypeUserID, got.Resources[0].ID)
}

func TestHandleResourceTypeListRequest_ServiceError(t *testing.T) {
	mockSvc := NewSCIMServiceInterfaceMock(t)
	mockSvc.On("ListResourceTypes", mock.Anything, testBaseURL).
		Return(SCIMResourceTypeListResponse{}, &ErrorResourceTypeNotFound)

	h := newSCIMHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/ResourceTypes", nil)
	rr := httptest.NewRecorder()

	h.HandleResourceTypeListRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleResourceTypeGetRequest_Success(t *testing.T) {
	expectedRT := &SCIMResourceType{
		Schemas: []string{SCIMResourceTypeSchemaURN},
		ID:      scimResourceTypeUserID,
		Name:    scimResourceTypeUserName,
	}

	mockSvc := NewSCIMServiceInterfaceMock(t)
	mockSvc.On("GetResourceType", mock.Anything, scimResourceTypeUserID, testBaseURL).
		Return(expectedRT, (*tidcommon.ServiceError)(nil))

	h := newSCIMHandler(mockSvc, testBaseURL)
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

func TestHandleResourceTypeGetRequest_NotFound(t *testing.T) {
	mockSvc := NewSCIMServiceInterfaceMock(t)
	mockSvc.On("GetResourceType", mock.Anything, "Group", testBaseURL).
		Return((*SCIMResourceType)(nil), &ErrorResourceTypeNotFound)

	h := newSCIMHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/ResourceTypes/Group", nil)
	req.SetPathValue("id", "Group")
	rr := httptest.NewRecorder()

	h.HandleResourceTypeGetRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleResourceTypeGetRequest_MissingID(t *testing.T) {
	mockSvc := NewSCIMServiceInterfaceMock(t)

	h := newSCIMHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/ResourceTypes/", nil)
	// Intentionally do NOT set path value.
	rr := httptest.NewRecorder()

	h.HandleResourceTypeGetRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

// =====================================================================
// handleSCIMError — remaining branch coverage
// =====================================================================

func TestHandleSCIMError_ServerErrorType_Returns500(t *testing.T) {
	svcErr := &tidcommon.InternalServerError
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/test", nil)
	rr := httptest.NewRecorder()

	h := newSCIMHandler(nil, "")
	h.handleSCIMError(rr, req, svcErr)

	require.Equal(t, http.StatusInternalServerError, rr.Code)

	var errResp SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, "500", errResp.Status)
	require.Equal(t, []string{SCIMErrorSchemaURN}, errResp.Schemas)
}

func TestHandleSCIMError_AuthError_Returns403(t *testing.T) {
	authErr := tidcommon.ErrorUnauthorized
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/test", nil)
	rr := httptest.NewRecorder()

	h := newSCIMHandler(nil, "")
	h.handleSCIMError(rr, req, &authErr)

	require.Equal(t, http.StatusForbidden, rr.Code)

	var errResp SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, "403", errResp.Status)
	require.Empty(t, errResp.ScimType)
}

func TestHandleSCIMError_DefaultFallback_Returns400InvalidValue(t *testing.T) {
	unknownErr := &tidcommon.ServiceError{Code: "SCIM-UNKNOWN-9999"}
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/test", nil)
	rr := httptest.NewRecorder()

	h := newSCIMHandler(nil, "")
	h.handleSCIMError(rr, req, unknownErr)

	require.Equal(t, http.StatusBadRequest, rr.Code)

	var errResp SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, "invalidValue", errResp.ScimType)
}

// =====================================================================
// rawEnumToStrings
// =====================================================================

func TestRawEnumToStrings_StringValues(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`"active"`),
		json.RawMessage(`"inactive"`),
	}
	out := rawEnumToStrings(raw)
	require.Equal(t, []string{"active", "inactive"}, out)
}

func TestRawEnumToStrings_NumberValues(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`1`),
		json.RawMessage(`3.14`),
	}
	out := rawEnumToStrings(raw)
	require.Equal(t, []string{"1", "3.14"}, out)
}

func TestRawEnumToStrings_EmptySlice(t *testing.T) {
	out := rawEnumToStrings(nil)
	require.Empty(t, out)
}

// =====================================================================
// mapRawPropertyToSCIMAttribute — enum/canonical-values branches
// =====================================================================

func TestMapRawProperty_StringWithEnum_PopulatesCanonicalValues(t *testing.T) {
	def := rawPropertyDef{
		Type: "string",
		Enum: []json.RawMessage{json.RawMessage(`"a"`), json.RawMessage(`"b"`)},
	}
	attr := mapRawPropertyToSCIMAttribute("status", def)
	require.Equal(t, scimAttrTypeString, attr.Type)
	require.Equal(t, []string{"a", "b"}, attr.CanonicalValues)
}

func TestMapRawProperty_NumberWithEnum_PopulatesCanonicalValues(t *testing.T) {
	def := rawPropertyDef{
		Type: "number",
		Enum: []json.RawMessage{json.RawMessage(`1`), json.RawMessage(`2`)},
	}
	attr := mapRawPropertyToSCIMAttribute("level", def)
	require.Equal(t, scimAttrTypeDecimal, attr.Type)
	require.Equal(t, []string{"1", "2"}, attr.CanonicalValues)
}

func TestMapRawProperty_ArrayWithNumberItems(t *testing.T) {
	items := rawPropertyDef{Type: "number"}
	attr := mapRawPropertyToSCIMAttribute("scores", rawPropertyDef{Type: "array", Items: &items})
	require.True(t, attr.MultiValued)
	require.Equal(t, scimAttrTypeDecimal, attr.Type)
}

func TestMapRawProperty_ArrayWithEnumItems_PropagatesCanonicalValues(t *testing.T) {
	items := rawPropertyDef{
		Type: "string",
		Enum: []json.RawMessage{json.RawMessage(`"x"`)},
	}
	attr := mapRawPropertyToSCIMAttribute("tags", rawPropertyDef{Type: "array", Items: &items})
	require.True(t, attr.MultiValued)
	require.Equal(t, []string{"x"}, attr.CanonicalValues)
}
