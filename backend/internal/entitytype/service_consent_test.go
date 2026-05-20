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

package entitytype

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/consent"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/consentmock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
)

type EntityTypeServiceConsentTestSuite struct {
	suite.Suite
}

func TestEntityTypeServiceConsentTestSuite(t *testing.T) {
	suite.Run(t, new(EntityTypeServiceConsentTestSuite))
}

// newTestSchemaServiceWithConsent creates a entityTypeService with only the consentService field set.
func newTestSchemaServiceWithConsent(consentSvc consent.ConsentServiceInterface) *entityTypeService {
	return &entityTypeService{consentService: consentSvc}
}

// ----- extractAttributeNames -----

func (s *EntityTypeServiceConsentTestSuite) TestExtractAttributeNames_EmptySchema() {
	names, svcErr := extractAttributeNames(TypeCategoryUser, json.RawMessage{})

	s.Nil(svcErr)
	s.Nil(names)
}

func (s *EntityTypeServiceConsentTestSuite) TestExtractAttributeNames_ValidSchema() {
	schema := json.RawMessage(`{"email":{},"phone":{}}`)

	names, svcErr := extractAttributeNames(TypeCategoryUser, schema)

	s.Nil(svcErr)
	s.Len(names, 2)
	s.ElementsMatch([]string{"email", "phone"}, names)
}

func (s *EntityTypeServiceConsentTestSuite) TestExtractAttributeNames_InvalidJSON() {
	schema := json.RawMessage(`not-valid-json`)

	names, svcErr := extractAttributeNames(TypeCategoryUser, schema)

	s.Nil(names)
	s.NotNil(svcErr)
}

// ----- extractAttributeNamesAsMap -----

func (s *EntityTypeServiceConsentTestSuite) TestExtractAttributeNamesAsMap() {
	result, svcErr := extractAttributeNamesAsMap(TypeCategoryUser, json.RawMessage{})

	s.Nil(svcErr)
	s.Empty(result)
}

func (s *EntityTypeServiceConsentTestSuite) TestExtractAttributeNamesAsMap_ValidSchema() {
	schema := json.RawMessage(`{"email":{},"phone":{}}`)

	result, svcErr := extractAttributeNamesAsMap(TypeCategoryUser, schema)

	s.Nil(svcErr)
	s.Len(result, 2)
	s.True(result["email"])
	s.True(result["phone"])
}

func (s *EntityTypeServiceConsentTestSuite) TestExtractAttributeNamesAsMap_InvalidJSON() {
	result, svcErr := extractAttributeNamesAsMap(TypeCategoryUser, json.RawMessage(`{bad json}`))

	s.Nil(result)
	s.NotNil(svcErr)
}

// ----- wrapConsentServiceError (entitytype) -----

func (s *EntityTypeServiceConsentTestSuite) TestWrapConsentServiceError_Nil() {
	result := wrapConsentServiceError(nil, nil)

	s.Nil(result)
}

func (s *EntityTypeServiceConsentTestSuite) TestWrapConsentServiceError_ClientError() {
	clientErr := &serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "CSE-1007",
	}

	result := wrapConsentServiceError(clientErr, log.GetLogger())

	s.NotNil(result)
	s.Equal(serviceerror.ClientErrorType, result.Type)
	s.Equal(ErrorConsentSyncFailed.Code, result.Code)
}

func (s *EntityTypeServiceConsentTestSuite) TestWrapConsentServiceError_ServerError() {
	serverErr := &serviceerror.ServiceError{
		Type: serviceerror.ServerErrorType,
		Code: "CSE-500",
	}

	result := wrapConsentServiceError(serverErr, log.GetLogger())

	s.NotNil(result)
	s.Equal(serviceerror.ServerErrorType, result.Type)
}

// ----- createMissingConsentElements -----

func (s *EntityTypeServiceConsentTestSuite) TestCreateMissingConsentElements_EmptyNames() {
	cMock := consentmock.NewConsentServiceInterfaceMock(s.T())
	svc := newTestSchemaServiceWithConsent(cMock)

	result := svc.createMissingConsentElements(context.Background(), "default", []string{}, log.GetLogger())

	s.Nil(result)
}

func (s *EntityTypeServiceConsentTestSuite) TestCreateMissingConsentElements_AllExist() {
	cMock := consentmock.NewConsentServiceInterfaceMock(s.T())
	svc := newTestSchemaServiceWithConsent(cMock)

	names := []string{"email", "phone"}
	cMock.EXPECT().ValidateConsentElements(mock.Anything, "default", names).
		Return([]string{"email", "phone"}, nil)

	result := svc.createMissingConsentElements(context.Background(), "default", names, log.GetLogger())

	s.Nil(result)
}

func (s *EntityTypeServiceConsentTestSuite) TestCreateMissingConsentElements_SomeMissing() {
	cMock := consentmock.NewConsentServiceInterfaceMock(s.T())
	svc := newTestSchemaServiceWithConsent(cMock)

	names := []string{"email", "phone"}
	cMock.EXPECT().ValidateConsentElements(mock.Anything, "default", names).
		Return([]string{"email"}, nil)

	expectedInput := []consent.ConsentElementInput{
		{Name: "phone", Namespace: consent.NamespaceAttribute},
	}
	cMock.EXPECT().CreateConsentElements(mock.Anything, "default", expectedInput).
		Return([]consent.ConsentElement{{ID: "e1", Name: "phone"}}, nil)

	result := svc.createMissingConsentElements(context.Background(), "default", names, log.GetLogger())

	s.Nil(result)
}

func (s *EntityTypeServiceConsentTestSuite) TestCreateMissingConsentElements_ValidateError() {
	cMock := consentmock.NewConsentServiceInterfaceMock(s.T())
	svc := newTestSchemaServiceWithConsent(cMock)

	names := []string{"email"}
	cMock.EXPECT().ValidateConsentElements(mock.Anything, "default", names).
		Return(nil, &serviceerror.InternalServerError)

	result := svc.createMissingConsentElements(context.Background(), "default", names, log.GetLogger())

	s.NotNil(result)
	s.Equal(serviceerror.ServerErrorType, result.Type)
}

func (s *EntityTypeServiceConsentTestSuite) TestCreateMissingConsentElements_CreateError() {
	cMock := consentmock.NewConsentServiceInterfaceMock(s.T())
	svc := newTestSchemaServiceWithConsent(cMock)

	names := []string{"email"}
	cMock.EXPECT().ValidateConsentElements(mock.Anything, "default", names).
		Return([]string{}, nil)
	cMock.EXPECT().CreateConsentElements(mock.Anything, "default", mock.Anything).
		Return(nil, &serviceerror.InternalServerError)

	result := svc.createMissingConsentElements(context.Background(), "default", names, log.GetLogger())

	s.NotNil(result)
}

// ----- deleteConsentElements -----

func (s *EntityTypeServiceConsentTestSuite) TestDeleteConsentElements_EmptyList() {
	cMock := consentmock.NewConsentServiceInterfaceMock(s.T())
	svc := newTestSchemaServiceWithConsent(cMock)

	result := svc.deleteConsentElements(context.Background(), []string{}, log.GetLogger())

	s.Nil(result)
}

func (s *EntityTypeServiceConsentTestSuite) TestDeleteConsentElements() {
	cMock := consentmock.NewConsentServiceInterfaceMock(s.T())
	svc := newTestSchemaServiceWithConsent(cMock)

	cMock.EXPECT().ListConsentElements(mock.Anything, "default", consent.NamespaceAttribute, "email").
		Return([]consent.ConsentElement{{ID: "e1", Name: "email"}}, nil)
	cMock.EXPECT().DeleteConsentElement(mock.Anything, "default", "e1").
		Return((*serviceerror.ServiceError)(nil))

	result := svc.deleteConsentElements(context.Background(), []string{"email"}, log.GetLogger())

	s.Nil(result)
}

func (s *EntityTypeServiceConsentTestSuite) TestDeleteConsentElements_NoExistingElements() {
	cMock := consentmock.NewConsentServiceInterfaceMock(s.T())
	svc := newTestSchemaServiceWithConsent(cMock)

	cMock.EXPECT().ListConsentElements(mock.Anything, "default", consent.NamespaceAttribute, "email").
		Return([]consent.ConsentElement{}, nil)

	result := svc.deleteConsentElements(context.Background(), []string{"email"}, log.GetLogger())

	s.Nil(result)
}

func (s *EntityTypeServiceConsentTestSuite) TestDeleteConsentElements_AssociatedPurposeError() {
	cMock := consentmock.NewConsentServiceInterfaceMock(s.T())
	svc := newTestSchemaServiceWithConsent(cMock)

	// "email" can't be deleted due to associated purpose — should warn and continue
	cMock.EXPECT().ListConsentElements(mock.Anything, "default", consent.NamespaceAttribute, "email").
		Return([]consent.ConsentElement{{ID: "e1", Name: "email"}}, nil)
	cMock.EXPECT().DeleteConsentElement(mock.Anything, "default", "e1").
		Return(&consent.ErrorDeletingConsentElementWithAssociatedPurpose)

	result := svc.deleteConsentElements(context.Background(), []string{"email"}, log.GetLogger())

	s.Nil(result)
}

func (s *EntityTypeServiceConsentTestSuite) TestDeleteConsentElements_OtherDeleteError() {
	cMock := consentmock.NewConsentServiceInterfaceMock(s.T())
	svc := newTestSchemaServiceWithConsent(cMock)

	cMock.EXPECT().ListConsentElements(mock.Anything, "default", consent.NamespaceAttribute, "email").
		Return([]consent.ConsentElement{{ID: "e1", Name: "email"}}, nil)
	cMock.EXPECT().DeleteConsentElement(mock.Anything, "default", "e1").
		Return(&serviceerror.InternalServerError)

	result := svc.deleteConsentElements(context.Background(), []string{"email"}, log.GetLogger())

	s.NotNil(result)
}

func (s *EntityTypeServiceConsentTestSuite) TestDeleteConsentElements_ListError() {
	cMock := consentmock.NewConsentServiceInterfaceMock(s.T())
	svc := newTestSchemaServiceWithConsent(cMock)

	cMock.EXPECT().ListConsentElements(mock.Anything, "default", consent.NamespaceAttribute, "email").
		Return(nil, &serviceerror.InternalServerError)

	result := svc.deleteConsentElements(context.Background(), []string{"email"}, log.GetLogger())

	s.NotNil(result)
}

func (s *EntityTypeServiceConsentTestSuite) TestDeleteConsentElements_MultipleAttrs() {
	cMock := consentmock.NewConsentServiceInterfaceMock(s.T())
	svc := newTestSchemaServiceWithConsent(cMock)

	cMock.EXPECT().ListConsentElements(mock.Anything, "default", consent.NamespaceAttribute, "email").
		Return([]consent.ConsentElement{{ID: "e1", Name: "email"}}, nil)
	cMock.EXPECT().DeleteConsentElement(mock.Anything, "default", "e1").
		Return((*serviceerror.ServiceError)(nil))

	cMock.EXPECT().ListConsentElements(mock.Anything, "default", consent.NamespaceAttribute, "phone").
		Return([]consent.ConsentElement{{ID: "e2", Name: "phone"}}, nil)
	cMock.EXPECT().DeleteConsentElement(mock.Anything, "default", "e2").
		Return((*serviceerror.ServiceError)(nil))

	result := svc.deleteConsentElements(context.Background(), []string{"email", "phone"}, log.GetLogger())

	s.Nil(result)
}

// ----- syncConsentElementsOnCreate -----

func (s *EntityTypeServiceConsentTestSuite) TestSyncConsentElementsOnCreate_EmptySchema() {
	cMock := consentmock.NewConsentServiceInterfaceMock(s.T())
	svc := newTestSchemaServiceWithConsent(cMock)

	result := svc.syncConsentElementsOnCreate(
		context.Background(), TypeCategoryUser, json.RawMessage{}, log.GetLogger())

	s.Nil(result)
}

func (s *EntityTypeServiceConsentTestSuite) TestSyncConsentElementsOnCreate_WithAttributes() {
	cMock := consentmock.NewConsentServiceInterfaceMock(s.T())
	svc := newTestSchemaServiceWithConsent(cMock)

	schema := json.RawMessage(`{"email":{},"phone":{}}`)
	cMock.EXPECT().ValidateConsentElements(mock.Anything, "default", mock.Anything).
		Return([]string{}, nil)
	cMock.EXPECT().CreateConsentElements(mock.Anything, "default", mock.Anything).
		Return([]consent.ConsentElement{{ID: "e1"}, {ID: "e2"}}, nil)

	result := svc.syncConsentElementsOnCreate(context.Background(), TypeCategoryUser, schema, log.GetLogger())

	s.Nil(result)
}

func (s *EntityTypeServiceConsentTestSuite) TestSyncConsentElementsOnCreate_InvalidSchema() {
	cMock := consentmock.NewConsentServiceInterfaceMock(s.T())
	svc := newTestSchemaServiceWithConsent(cMock)

	result := svc.syncConsentElementsOnCreate(context.Background(),
		TypeCategoryUser, json.RawMessage(`{bad`), log.GetLogger())

	s.NotNil(result)
}

func (s *EntityTypeServiceConsentTestSuite) TestSyncConsentElementsOnCreate_CreateError() {
	cMock := consentmock.NewConsentServiceInterfaceMock(s.T())
	svc := newTestSchemaServiceWithConsent(cMock)

	schema := json.RawMessage(`{"email":{}}`)
	cMock.EXPECT().ValidateConsentElements(mock.Anything, "default", mock.Anything).
		Return(nil, &serviceerror.InternalServerError)

	result := svc.syncConsentElementsOnCreate(context.Background(), TypeCategoryUser, schema, log.GetLogger())

	s.NotNil(result)
}

// ----- syncConsentElementsOnUpdate -----

func (s *EntityTypeServiceConsentTestSuite) TestSyncConsentElementsOnUpdate_NoChanges() {
	cMock := consentmock.NewConsentServiceInterfaceMock(s.T())
	svc := newTestSchemaServiceWithConsent(cMock)

	schema := json.RawMessage(`{"email":{}}`)
	cMock.EXPECT().ValidateConsentElements(mock.Anything, "default", mock.Anything).
		Return([]string{"email"}, nil)

	result := svc.syncConsentElementsOnUpdate(context.Background(), TypeCategoryUser, schema, schema, log.GetLogger())

	s.Nil(result)
}

func (s *EntityTypeServiceConsentTestSuite) TestSyncConsentElementsOnUpdate_NewAttrAdded() {
	cMock := consentmock.NewConsentServiceInterfaceMock(s.T())
	svc := newTestSchemaServiceWithConsent(cMock)

	oldSchema := json.RawMessage(`{"email":{}}`)
	newSchema := json.RawMessage(`{"email":{},"phone":{}}`)

	cMock.EXPECT().ValidateConsentElements(mock.Anything, "default", mock.Anything).
		Return([]string{"email"}, nil)
	cMock.EXPECT().CreateConsentElements(mock.Anything, "default",
		[]consent.ConsentElementInput{{Name: "phone", Namespace: consent.NamespaceAttribute}}).
		Return([]consent.ConsentElement{{ID: "e2", Name: "phone"}}, nil)

	result := svc.syncConsentElementsOnUpdate(
		context.Background(), TypeCategoryUser, oldSchema, newSchema, log.GetLogger())

	s.Nil(result)
}

func (s *EntityTypeServiceConsentTestSuite) TestSyncConsentElementsOnUpdate_AttrRemoved() {
	cMock := consentmock.NewConsentServiceInterfaceMock(s.T())
	svc := newTestSchemaServiceWithConsent(cMock)

	oldSchema := json.RawMessage(`{"email":{},"phone":{}}`)
	newSchema := json.RawMessage(`{"email":{}}`)

	// Validate "email" exists, no new elements to create
	cMock.EXPECT().ValidateConsentElements(mock.Anything, "default", mock.Anything).
		Return([]string{"email"}, nil)

	// Delete "phone" which was removed
	cMock.EXPECT().ListConsentElements(mock.Anything, "default", consent.NamespaceAttribute, "phone").
		Return([]consent.ConsentElement{{ID: "e2", Name: "phone"}}, nil)
	cMock.EXPECT().DeleteConsentElement(mock.Anything, "default", "e2").
		Return((*serviceerror.ServiceError)(nil))

	result := svc.syncConsentElementsOnUpdate(
		context.Background(), TypeCategoryUser, oldSchema, newSchema, log.GetLogger())

	s.Nil(result)
}

func (s *EntityTypeServiceConsentTestSuite) TestSyncConsentElementsOnUpdate_InvalidOldSchema() {
	cMock := consentmock.NewConsentServiceInterfaceMock(s.T())
	svc := newTestSchemaServiceWithConsent(cMock)

	newSchema := json.RawMessage(`{"email":{}}`)

	result := svc.syncConsentElementsOnUpdate(context.Background(),
		TypeCategoryUser, json.RawMessage(`{bad`), newSchema, log.GetLogger())

	s.NotNil(result)
}

func (s *EntityTypeServiceConsentTestSuite) TestSyncConsentElementsOnUpdate_InvalidNewSchema() {
	cMock := consentmock.NewConsentServiceInterfaceMock(s.T())
	svc := newTestSchemaServiceWithConsent(cMock)

	oldSchema := json.RawMessage(`{"email":{}}`)

	result := svc.syncConsentElementsOnUpdate(context.Background(),
		TypeCategoryUser, oldSchema, json.RawMessage(`{bad`), log.GetLogger())

	s.NotNil(result)
}

func (s *EntityTypeServiceConsentTestSuite) TestSyncConsentElementsOnUpdate_CreateError() {
	cMock := consentmock.NewConsentServiceInterfaceMock(s.T())
	svc := newTestSchemaServiceWithConsent(cMock)

	oldSchema := json.RawMessage(`{"email":{}}`)
	newSchema := json.RawMessage(`{"email":{},"phone":{}}`)

	cMock.EXPECT().ValidateConsentElements(mock.Anything, "default", mock.Anything).
		Return(nil, &serviceerror.InternalServerError)

	result := svc.syncConsentElementsOnUpdate(
		context.Background(), TypeCategoryUser, oldSchema, newSchema, log.GetLogger())

	s.NotNil(result)
}

func (s *EntityTypeServiceConsentTestSuite) TestSyncConsentElementsOnUpdate_DeleteError() {
	cMock := consentmock.NewConsentServiceInterfaceMock(s.T())
	svc := newTestSchemaServiceWithConsent(cMock)

	oldSchema := json.RawMessage(`{"email":{},"phone":{}}`)
	newSchema := json.RawMessage(`{"email":{}}`)

	cMock.EXPECT().ValidateConsentElements(mock.Anything, "default", mock.Anything).
		Return([]string{"email"}, nil)
	cMock.EXPECT().ListConsentElements(mock.Anything, "default", consent.NamespaceAttribute, "phone").
		Return(nil, &serviceerror.InternalServerError)

	result := svc.syncConsentElementsOnUpdate(
		context.Background(), TypeCategoryUser, oldSchema, newSchema, log.GetLogger())

	s.NotNil(result)
}

// ----- CreateEntityType compensation tests -----

// TestCreateEntityType_ConsentSyncFails_CompensatesWithSchemaDeletion verifies that when
// consent element sync fails after schema creation, the schema is deleted as compensation.
func (s *EntityTypeServiceConsentTestSuite) TestCreateEntityType_ConsentSyncFails_CompensatesWithSchemaDeletion() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: false},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(s.T(), err)
	defer config.ResetServerRuntime()

	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	ouMock := oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	cMock := consentmock.NewConsentServiceInterfaceMock(s.T())

	svc := &entityTypeService{
		entityTypeStore: storeMock,
		ouService:       ouMock,
		transactioner:   &mockTransactioner{},
		consentService:  cMock,
	}

	ouMock.On("IsOrganizationUnitExists", mock.Anything, testOUID1).Return(true, (*serviceerror.ServiceError)(nil))
	storeMock.On("GetEntityTypeByName", mock.Anything, mock.Anything, "test-schema").
		Return(EntityType{}, ErrEntityTypeNotFound)
	storeMock.On("CreateEntityType", mock.Anything, mock.Anything).Return(nil)
	// Consent sync fails.
	cMock.On("IsEnabled").Return(true)
	cMock.On("ValidateConsentElements", mock.Anything, "default", mock.Anything).
		Return(nil, &serviceerror.InternalServerError)
	// Compensation: schema must be deleted.
	storeMock.On("DeleteEntityTypeByID", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	request := CreateEntityTypeRequestWithID{
		Name:   "test-schema",
		OUID:   testOUID1,
		Schema: json.RawMessage(`{"email":{"type":"string"}}`),
	}

	result, svcErr := svc.CreateEntityType(context.Background(), TypeCategoryUser, request)

	s.Nil(result)
	s.NotNil(svcErr)
	storeMock.AssertCalled(s.T(), "DeleteEntityTypeByID", mock.Anything, mock.Anything, mock.Anything)
}

// ----- UpdateEntityType compensation tests -----

// TestUpdateEntityType_ConsentSyncFails_CompensatesWithSchemaRevert verifies that when
// consent element sync fails after schema update, the schema is reverted as compensation.
func (s *EntityTypeServiceConsentTestSuite) TestUpdateEntityType_ConsentSyncFails_CompensatesWithSchemaRevert() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: false},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(s.T(), err)
	defer config.ResetServerRuntime()

	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	ouMock := oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	cMock := consentmock.NewConsentServiceInterfaceMock(s.T())

	svc := &entityTypeService{
		entityTypeStore: storeMock,
		ouService:       ouMock,
		transactioner:   &mockTransactioner{},
		consentService:  cMock,
	}

	existingSchema := EntityType{
		ID:     "schema-id",
		Name:   "test-schema",
		OUID:   testOUID1,
		Schema: json.RawMessage(`{"email":{"type":"string"}}`),
	}

	storeMock.On("IsEntityTypeDeclarative", TypeCategoryUser, "schema-id").Return(false)
	ouMock.On("IsOrganizationUnitExists", mock.Anything, testOUID1).Return(true, (*serviceerror.ServiceError)(nil))
	storeMock.On("GetEntityTypeByID", mock.Anything, mock.Anything, "schema-id").Return(existingSchema, nil)
	// Both the actual update (in tx) and the compensation revert share the same mock.
	storeMock.On("UpdateEntityTypeByID", mock.Anything, mock.Anything, "schema-id", mock.Anything).Return(nil)
	// Consent sync fails: ValidateConsentElements returns an I18n error.
	cMock.On("IsEnabled").Return(true)
	cMock.On("ValidateConsentElements", mock.Anything, "default", mock.Anything).
		Return(nil, &serviceerror.InternalServerError)

	request := UpdateEntityTypeRequest{
		Name:   "test-schema",
		OUID:   testOUID1,
		Schema: json.RawMessage(`{"email":{"type":"string"}}`),
	}

	result, svcErr := svc.UpdateEntityType(context.Background(), TypeCategoryUser, "schema-id", request)

	s.Nil(result)
	s.NotNil(svcErr)
	// Verify compensation was called: UpdateEntityTypeByID twice (update + revert).
	storeMock.AssertNumberOfCalls(s.T(), "UpdateEntityTypeByID", 2)
}

// ----- DeleteEntityType consent tests -----

// TestDeleteEntityType_ConsentEnabled_DeletesConsentElementsAfterSchemaDeletion verifies
// that when consent is enabled, consent elements are cleaned up after the schema is deleted.
func (s *EntityTypeServiceConsentTestSuite) TestDeleteEntityType_ConsentEnabled_DeletesConsentElements() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: false},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(s.T(), err)
	defer config.ResetServerRuntime()

	storeMock := newEntityTypeStoreInterfaceMock(s.T())
	cMock := consentmock.NewConsentServiceInterfaceMock(s.T())

	svc := &entityTypeService{
		entityTypeStore: storeMock,
		transactioner:   &mockTransactioner{},
		consentService:  cMock,
	}

	existingSchema := EntityType{
		ID:     "schema-id",
		Name:   "test-schema",
		OUID:   testOUID1,
		Schema: json.RawMessage(`{"email":{"type":"string"}}`),
	}

	storeMock.On("GetEntityTypeByID", mock.Anything, mock.Anything, "schema-id").Return(existingSchema, nil)
	storeMock.On("IsEntityTypeDeclarative", TypeCategoryUser, "schema-id").Return(false)
	cMock.On("IsEnabled").Return(true)
	storeMock.On("DeleteEntityTypeByID", mock.Anything, mock.Anything, "schema-id").Return(nil)
	// Consent element cleanup: ListConsentElements → found → DeleteConsentElement
	cMock.On("ListConsentElements", mock.Anything, "default", consent.NamespaceAttribute, "email").
		Return([]consent.ConsentElement{{ID: "elem-1", Name: "email"}}, (*serviceerror.ServiceError)(nil))
	cMock.On("DeleteConsentElement", mock.Anything, "default", "elem-1").
		Return((*serviceerror.ServiceError)(nil))

	svcErr := svc.DeleteEntityType(context.Background(), TypeCategoryUser, "schema-id")

	s.Nil(svcErr)
	storeMock.AssertCalled(s.T(), "DeleteEntityTypeByID", mock.Anything, mock.Anything, "schema-id")
	cMock.AssertCalled(s.T(), "ListConsentElements", mock.Anything, "default", consent.NamespaceAttribute, "email")
}
