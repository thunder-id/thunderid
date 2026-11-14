// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outype

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
)

const (
	testOUTypeID = "550e8400-e29b-41d4-a716-446655440000"
	testOUID     = "26eec421-f1bb-4deb-a5d3-9ab6554c2ae6"
)

var testSchema = json.RawMessage(`{"registrationNo":{"type":"string","required":true,"unique":true}}`)

type OUTypeServiceTestSuite struct {
	suite.Suite
	entityTypes *entitytypemock.EntityTypeServiceInterfaceMock
	service     OUTypeServiceInterface
}

func TestOUTypeServiceTestSuite(t *testing.T) {
	suite.Run(t, new(OUTypeServiceTestSuite))
}

func (s *OUTypeServiceTestSuite) SetupTest() {
	s.entityTypes = entitytypemock.NewEntityTypeServiceInterfaceMock(s.T())
	s.service = newOUTypeService(s.entityTypes)
}

func storedEntityType() *entitytype.EntityType {
	return &entitytype.EntityType{
		ID:                    testOUTypeID,
		Category:              entitytype.TypeCategoryOU,
		Handle:                "partner",
		DisplayName:           "Partner",
		OUID:                  testOUID,
		OUHandle:              "root",
		AllowSelfRegistration: true,
		SystemAttributes:      &entitytype.SystemAttributes{Display: "registrationNo"},
		Schema:                testSchema,
		IsReadOnly:            true,
	}
}

func expectedOUType() *OUType {
	return &OUType{
		ID:               testOUTypeID,
		Handle:           "partner",
		DisplayName:      "Partner",
		OUID:             testOUID,
		OUHandle:         "root",
		SystemAttributes: &SystemAttributes{Display: "registrationNo"},
		Schema:           testSchema,
		IsReadOnly:       true,
	}
}

func (s *OUTypeServiceTestSuite) TestGetOUTypeList() {
	s.entityTypes.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryOU, 10, 5, true).
		Return(&entitytype.EntityTypeListResponse{
			TotalResults: 6,
			StartIndex:   6,
			Count:        1,
			Types: []entitytype.EntityTypeListItem{{
				ID: testOUTypeID, Handle: "partner", DisplayName: "Partner", OUID: testOUID, OUHandle: "root",
				AllowSelfRegistration: true, SystemAttributes: &entitytype.SystemAttributes{}, IsReadOnly: true,
			}},
			Links: []entitytype.Link{{Href: "/organization-unit-types?offset=0&limit=10", Rel: "first"}},
		}, nil).Once()

	list, svcErr := s.service.GetOUTypeList(context.Background(), 10, 5, true)

	s.Require().Nil(svcErr)
	s.Equal(&OUTypeListResponse{
		TotalResults: 6,
		StartIndex:   6,
		Count:        1,
		Types: []OUTypeListItem{{
			ID: testOUTypeID, Handle: "partner", DisplayName: "Partner", OUID: testOUID, OUHandle: "root",
			IsReadOnly: true,
		}},
		Links: []entitytype.Link{{Href: "/organization-unit-types?offset=0&limit=10", Rel: "first"}},
	}, list)
}

func (s *OUTypeServiceTestSuite) TestGetOUTypeList_MapsError() {
	s.entityTypes.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryOU, 0, 0, false).
		Return(nil, &entitytype.ErrorInvalidLimit).Once()

	list, svcErr := s.service.GetOUTypeList(context.Background(), 0, 0, false)

	s.Nil(list)
	s.Equal(&ErrorInvalidLimit, svcErr)
}

func (s *OUTypeServiceTestSuite) TestCreateOUType() {
	s.entityTypes.On("CreateEntityType", mock.Anything, entitytype.TypeCategoryOU,
		entitytype.CreateEntityTypeRequestWithID{
			Handle:           "partner",
			DisplayName:      "Partner",
			OUID:             testOUID,
			SystemAttributes: &entitytype.SystemAttributes{Display: "registrationNo"},
			Schema:           testSchema,
		}).Return(storedEntityType(), nil).Once()

	created, svcErr := s.service.CreateOUType(context.Background(), CreateOUTypeRequest{
		Handle:           "partner",
		DisplayName:      "Partner",
		OUID:             testOUID,
		SystemAttributes: &SystemAttributes{Display: "registrationNo"},
		Schema:           testSchema,
	})

	s.Require().Nil(svcErr)
	s.Equal(expectedOUType(), created)
}

func (s *OUTypeServiceTestSuite) TestCreateOUType_RejectsCredentialAttribute() {
	created, svcErr := s.service.CreateOUType(context.Background(), CreateOUTypeRequest{
		Handle:      "partner",
		DisplayName: "Partner",
		OUID:        testOUID,
		Schema:      json.RawMessage(`{"name":{"type":"string"},"apiKey":{"type":"string","credential":true}}`),
	})

	s.Nil(created)
	s.Equal(&ErrorCredentialAttributeNotAllowed, svcErr)
	s.entityTypes.AssertNotCalled(s.T(), "CreateEntityType", mock.Anything, mock.Anything, mock.Anything)
}

func (s *OUTypeServiceTestSuite) TestCreateOUType_RejectsNestedCredentialAttribute() {
	cases := map[string]string{
		"in an object": `{"contact":{"type":"object","properties":{"apiKey":{"type":"string","credential":true}}}}`,
		"in array items": `{"keys":{"type":"array","items":{"type":"object","properties":` +
			`{"secret":{"type":"string","credential":true}}}}}`,
	}
	for name, schema := range cases {
		s.Run(name, func() {
			created, svcErr := s.service.CreateOUType(context.Background(), CreateOUTypeRequest{
				Handle: "partner", DisplayName: "Partner", OUID: testOUID, Schema: json.RawMessage(schema),
			})

			s.Nil(created)
			s.Equal(&ErrorCredentialAttributeNotAllowed, svcErr)
		})
	}
	s.entityTypes.AssertNotCalled(s.T(), "CreateEntityType", mock.Anything, mock.Anything, mock.Anything)
}

func (s *OUTypeServiceTestSuite) TestCreateOUType_LeavesAnInvalidSchemaToTheEntityTypeService() {
	invalid := json.RawMessage(`{"name":{"type":"unknown"}}`)
	s.entityTypes.On("CreateEntityType", mock.Anything, entitytype.TypeCategoryOU,
		mock.MatchedBy(func(r entitytype.CreateEntityTypeRequestWithID) bool {
			return string(r.Schema) == string(invalid)
		})).Return(nil, &tidcommon.ServiceError{
		Type:             tidcommon.ClientErrorType,
		Code:             entitytype.ErrorInvalidEntityTypeRequest.Code,
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "The request is invalid: unsupported type: unknown"},
	}).Once()

	_, svcErr := s.service.CreateOUType(context.Background(), CreateOUTypeRequest{
		Handle: "partner", DisplayName: "Partner", OUID: testOUID, Schema: invalid,
	})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorInvalidOUTypeRequest.Code, svcErr.Code)
	s.Equal(ErrorInvalidOUTypeRequest.ErrorDescription.DefaultValue+": unsupported type: unknown",
		svcErr.ErrorDescription.DefaultValue)
}

func (s *OUTypeServiceTestSuite) TestGetOUType() {
	s.entityTypes.On("GetEntityType", mock.Anything, entitytype.TypeCategoryOU, testOUTypeID, true).
		Return(storedEntityType(), nil).Once()

	ouType, svcErr := s.service.GetOUType(context.Background(), testOUTypeID, true)

	s.Require().Nil(svcErr)
	s.Equal(expectedOUType(), ouType)
}

func (s *OUTypeServiceTestSuite) TestGetOUType_NotFound() {
	s.entityTypes.On("GetEntityType", mock.Anything, entitytype.TypeCategoryOU, testOUTypeID, false).
		Return(nil, &entitytype.ErrorEntityTypeNotFound).Once()

	ouType, svcErr := s.service.GetOUType(context.Background(), testOUTypeID, false)

	s.Nil(ouType)
	s.Equal(&ErrorOUTypeNotFound, svcErr)
}

func (s *OUTypeServiceTestSuite) TestUpdateOUType() {
	s.entityTypes.On("UpdateEntityType", mock.Anything, entitytype.TypeCategoryOU, testOUTypeID,
		entitytype.UpdateEntityTypeRequest{
			Handle:      "partner",
			DisplayName: "Partner",
			OUID:        testOUID,
			Schema:      testSchema,
		}).Return(storedEntityType(), nil).Once()

	updated, svcErr := s.service.UpdateOUType(context.Background(), testOUTypeID, UpdateOUTypeRequest{
		Handle: "partner", DisplayName: "Partner", OUID: testOUID, Schema: testSchema,
	})

	s.Require().Nil(svcErr)
	s.Equal(expectedOUType(), updated)
}

func (s *OUTypeServiceTestSuite) TestUpdateOUType_RejectsCredentialAttribute() {
	updated, svcErr := s.service.UpdateOUType(context.Background(), testOUTypeID, UpdateOUTypeRequest{
		DisplayName: "Partner",
		OUID:        testOUID,
		Schema:      json.RawMessage(`{"apiKey":{"type":"string","credential":true}}`),
	})

	s.Nil(updated)
	s.Equal(&ErrorCredentialAttributeNotAllowed, svcErr)
	s.entityTypes.AssertNotCalled(s.T(), "UpdateEntityType", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything)
}

func (s *OUTypeServiceTestSuite) TestUpdateOUType_MapsError() {
	s.entityTypes.On("UpdateEntityType", mock.Anything, entitytype.TypeCategoryOU, testOUTypeID, mock.Anything).
		Return(nil, &entitytype.ErrorEntityTypeHandleUpdateNotAllowed).Once()

	updated, svcErr := s.service.UpdateOUType(context.Background(), testOUTypeID, UpdateOUTypeRequest{
		Handle: "other", DisplayName: "Partner", OUID: testOUID, Schema: testSchema,
	})

	s.Nil(updated)
	s.Equal(&ErrorOUTypeHandleUpdateNotAllowed, svcErr)
}

func (s *OUTypeServiceTestSuite) TestDeleteOUType() {
	s.entityTypes.On("DeleteEntityType", mock.Anything, entitytype.TypeCategoryOU, testOUTypeID).
		Return(nil).Once()

	s.Nil(s.service.DeleteOUType(context.Background(), testOUTypeID))
}

func (s *OUTypeServiceTestSuite) TestDeleteOUType_MapsError() {
	s.entityTypes.On("DeleteEntityType", mock.Anything, entitytype.TypeCategoryOU, testOUTypeID).
		Return(&entitytype.ErrorCannotModifyDeclarativeResource).Once()

	s.Equal(&ErrorCannotModifyDeclarativeOUType, s.service.DeleteOUType(context.Background(), testOUTypeID))
}

func (s *OUTypeServiceTestSuite) TestToOUTypeError() {
	cases := []struct {
		name string
		in   *tidcommon.ServiceError
		want *tidcommon.ServiceError
	}{
		{"server error", &tidcommon.InternalServerError, &tidcommon.InternalServerError},
		{"unauthorized", &tidcommon.ErrorUnauthorized, &tidcommon.ErrorUnauthorized},
		{"not found", &entitytype.ErrorEntityTypeNotFound, &ErrorOUTypeNotFound},
		{"handle conflict", &entitytype.ErrorEntityTypeHandleConflict, &ErrorOUTypeHandleConflict},
		{"invalid request without a cause", &entitytype.ErrorInvalidEntityTypeRequest, &ErrorInvalidOUTypeRequest},
		{"invalid limit", &entitytype.ErrorInvalidLimit, &ErrorInvalidLimit},
		{"invalid offset", &entitytype.ErrorInvalidOffset, &ErrorInvalidOffset},
		{"invalid handle", &entitytype.ErrorInvalidEntityTypeHandle, &ErrorInvalidOUTypeHandle},
		{"handle update", &entitytype.ErrorEntityTypeHandleUpdateNotAllowed, &ErrorOUTypeHandleUpdateNotAllowed},
		{"declarative", &entitytype.ErrorCannotModifyDeclarativeResource, &ErrorCannotModifyDeclarativeOUType},
		{"managed", &entitytype.ErrorManagedEntityType, &ErrorCannotModifyManagedOUType},
		{"display attribute", &entitytype.ErrorInvalidDisplayAttribute,
			invalidOUTypeRequestErr(entitytype.ErrorInvalidDisplayAttribute.ErrorDescription.DefaultValue)},
	}
	svc := s.service.(*ouTypeService)
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.Equal(tc.want, svc.toOUTypeError(context.Background(), tc.in))
		})
	}
}
