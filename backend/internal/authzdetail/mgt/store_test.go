// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzdetailmgt

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

const testDeploymentID = "test-deployment"

type StoreTestSuite struct {
	suite.Suite
	ctx        context.Context
	dbProvider *providermock.DBProviderInterfaceMock
	dbClient   *providermock.DBClientInterfaceMock
	types      *typeStore
}

func TestStoreTestSuite(t *testing.T) {
	suite.Run(t, new(StoreTestSuite))
}

func (s *StoreTestSuite) SetupTest() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("", &config.Config{
		Server: engineconfig.ServerConfig{Identifier: testDeploymentID},
	})
	s.ctx = context.Background()
	s.dbProvider = providermock.NewDBProviderInterfaceMock(s.T())
	s.dbClient = providermock.NewDBClientInterfaceMock(s.T())
	s.types = &typeStore{dbProvider: s.dbProvider}
}

func typeRow() map[string]interface{} {
	return map[string]interface{}{
		"id": "type-1", "type": testFlightType, "name": "Flight",
		"description": "Books a flight", "version": int64(2), "schema_definition": flightSchema,
		"properties": []byte(`{"consentTemplate":"Book {{route}}","reuse":false,` +
			`"processor":{"endpoint":"https://x"}}`),
	}
}

func (s *StoreTestSuite) TestCreateType() {
	s.dbProvider.EXPECT().GetConfigDBClient().Return(s.dbClient, nil)
	s.dbClient.EXPECT().ExecuteContext(mock.Anything, queryCreateType, "type-1", testFlightType, "Flight",
		nil, 1, []byte(flightSchema), mock.Anything, testDeploymentID).Return(int64(1), nil).Once()

	t := flightType()
	t.Name, t.Description, t.Schema = "Flight", "", json.RawMessage(flightSchema)
	s.NoError(s.types.CreateType(s.ctx, t))

	s.dbClient.EXPECT().ExecuteContext(mock.Anything, queryCreateType, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(int64(0), errors.New("insert failed")).Once()
	s.Error(s.types.CreateType(s.ctx, t))
}

func (s *StoreTestSuite) TestGetType() {
	s.dbProvider.EXPECT().GetConfigDBClient().Return(s.dbClient, nil)
	s.dbClient.EXPECT().QueryContext(mock.Anything, queryGetTypeByID, "type-1", testDeploymentID).
		Return([]map[string]interface{}{typeRow()}, nil).Once()

	t, err := s.types.GetTypeByID(s.ctx, "type-1")
	s.Require().NoError(err)
	s.Equal(2, t.Version)
	s.Equal("Book {{route}}", t.ConsentTemplate)
	s.False(t.Reuse)
	s.Equal("https://x", t.Processor.Endpoint)
	s.False(buildTypeFromRowWith(s, `{"consentTemplate":"Book {{route}}"}`).Reuse,
		"a stored type without reuse asks every time")
	s.True(buildTypeFromRowWith(s, `{"consentTemplate":"Book {{route}}","reuse":true}`).Reuse)
	s.Equal(flightSchema, string(t.Schema), "the schema is read back as it was written")
	for _, column := range []interface{}{flightSchema, []byte(flightSchema)} {
		row := typeRow()
		row["schema_definition"] = column
		read, err := buildTypeFromRow(row)
		s.Require().NoError(err)
		s.Equal(flightSchema, string(read.Schema), "whichever form the driver returns the text column in")
	}

	s.dbClient.EXPECT().QueryContext(mock.Anything, queryGetTypeByID, "missing", testDeploymentID).
		Return([]map[string]interface{}{}, nil).Once()
	_, err = s.types.GetTypeByID(s.ctx, "missing")
	s.ErrorIs(err, errTypeNotFound)

	s.dbClient.EXPECT().QueryContext(mock.Anything, queryGetTypeByID, "err", testDeploymentID).
		Return(nil, errors.New("db down")).Once()
	_, err = s.types.GetTypeByID(s.ctx, "err")
	s.Error(err)

	bad := typeRow()
	bad["properties"] = "not json"
	s.dbClient.EXPECT().QueryContext(mock.Anything, queryGetTypeByID, "bad", testDeploymentID).
		Return([]map[string]interface{}{bad}, nil).Once()
	_, err = s.types.GetTypeByID(s.ctx, "bad")
	s.Error(err)
}

func (s *StoreTestSuite) TestGetTypeByValueAndVersionForms() {
	s.dbProvider.EXPECT().GetConfigDBClient().Return(s.dbClient, nil)
	for _, version := range []interface{}{7, int64(7), float64(7)} {
		row := typeRow()
		row["version"] = version
		row["properties"] = nil
		s.dbClient.EXPECT().QueryContext(mock.Anything, queryGetTypeByValue, testFlightType, testDeploymentID).
			Return([]map[string]interface{}{row}, nil).Once()
		t, err := s.types.GetTypeByValue(s.ctx, testFlightType)
		s.Require().NoError(err)
		s.Equal(7, t.Version)
		s.Nil(t.Processor)
	}
}

func (s *StoreTestSuite) TestCountTypesAndListTypePage() {
	s.dbProvider.EXPECT().GetConfigDBClient().Return(s.dbClient, nil)
	s.dbClient.EXPECT().QueryContext(mock.Anything, queryCountTypes, testDeploymentID).
		Return([]map[string]interface{}{{"total": int64(3)}}, nil).Once()
	count, err := s.types.CountTypes(s.ctx)
	s.NoError(err)
	s.Equal(3, count)

	s.dbClient.EXPECT().QueryContext(mock.Anything, queryCountTypes, testDeploymentID).
		Return([]map[string]interface{}{}, nil).Once()
	_, err = s.types.CountTypes(s.ctx)
	s.Error(err, "a count query that returns no row")

	s.dbClient.EXPECT().QueryContext(mock.Anything, queryCountTypes, testDeploymentID).
		Return(nil, errors.New("db down")).Once()
	_, err = s.types.CountTypes(s.ctx)
	s.Error(err)

	s.dbClient.EXPECT().QueryContext(mock.Anything, queryListTypePage, 2, 1, testDeploymentID).
		Return([]map[string]interface{}{typeRow()}, nil).Once()
	types, err := s.types.ListTypePage(s.ctx, 2, 1)
	s.NoError(err)
	s.Len(types, 1)

	s.dbClient.EXPECT().QueryContext(mock.Anything, queryListTypePage, 2, 1, testDeploymentID).
		Return(nil, errors.New("db down")).Once()
	_, err = s.types.ListTypePage(s.ctx, 2, 1)
	s.Error(err)
}

func (s *StoreTestSuite) TestListTypesAndValues() {
	s.dbProvider.EXPECT().GetConfigDBClient().Return(s.dbClient, nil)
	s.dbClient.EXPECT().QueryContext(mock.Anything, queryListTypes, testDeploymentID).
		Return([]map[string]interface{}{typeRow(), typeRow()}, nil).Once()
	types, err := s.types.ListTypes(s.ctx)
	s.NoError(err)
	s.Len(types, 2)

	s.dbClient.EXPECT().QueryContext(mock.Anything, queryListTypes, testDeploymentID).
		Return(nil, errors.New("db down")).Once()
	_, err = s.types.ListTypes(s.ctx)
	s.Error(err)

	bad := typeRow()
	bad["properties"] = "{"
	s.dbClient.EXPECT().QueryContext(mock.Anything, queryListTypes, testDeploymentID).
		Return([]map[string]interface{}{bad}, nil).Once()
	_, err = s.types.ListTypes(s.ctx)
	s.Error(err)
}

func (s *StoreTestSuite) TestGetTypeByIDAndIsTypeDeclarative() {
	s.dbProvider.EXPECT().GetConfigDBClient().Return(s.dbClient, nil)
	s.dbClient.EXPECT().QueryContext(mock.Anything, queryGetTypeByID, "type-1", testDeploymentID).
		Return([]map[string]interface{}{typeRow()}, nil).Once()
	t, err := s.types.GetTypeByID(s.ctx, "type-1")
	s.Require().NoError(err)
	s.Equal(testFlightType, t.Type)

	s.dbClient.EXPECT().QueryContext(mock.Anything, queryGetTypeByID, "missing", testDeploymentID).
		Return([]map[string]interface{}{}, nil).Once()
	_, err = s.types.GetTypeByID(s.ctx, "missing")
	s.ErrorIs(err, errTypeNotFound)

	isDeclarative, err := s.types.IsTypeDeclarative(s.ctx, "type-1")
	s.NoError(err)
	s.False(isDeclarative, "a type in the database is never declarative")
}

func (s *StoreTestSuite) TestUpdateAndDeleteType() {
	s.dbProvider.EXPECT().GetConfigDBClient().Return(s.dbClient, nil)
	anyUpdate := []interface{}{mock.Anything, queryUpdateType, "type-1", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, 1, 1, testDeploymentID}

	s.dbClient.EXPECT().ExecuteContext(anyUpdate[0], anyUpdate[1], anyUpdate[2:]...).Return(int64(1), nil).Once()
	s.NoError(s.types.UpdateType(s.ctx, flightType(), true))

	s.dbClient.EXPECT().ExecuteContext(anyUpdate[0], anyUpdate[1], anyUpdate[2:]...).Return(int64(0), nil).Once()
	s.ErrorIs(s.types.UpdateType(s.ctx, flightType(), true), errTypeNotFound)

	s.dbClient.EXPECT().ExecuteContext(anyUpdate[0], anyUpdate[1], anyUpdate[2:]...).
		Return(int64(0), errors.New("db down")).Once()
	s.Error(s.types.UpdateType(s.ctx, flightType(), true))

	s.dbClient.EXPECT().ExecuteContext(mock.Anything, queryDeleteType, "type-1", testDeploymentID).
		Return(int64(1), nil).Once()
	s.NoError(s.types.DeleteType(s.ctx, "type-1"))

	s.dbClient.EXPECT().ExecuteContext(mock.Anything, queryDeleteType, "type-1", testDeploymentID).
		Return(int64(0), errors.New("db down")).Once()
	s.Error(s.types.DeleteType(s.ctx, "type-1"))
}

func (s *StoreTestSuite) TestConfigDBUnavailable() {
	s.dbProvider.EXPECT().GetConfigDBClient().Return(nil, errors.New("no db"))
	s.Error(s.types.CreateType(s.ctx, flightType()))
	_, err := s.types.ListTypes(s.ctx)
	s.Error(err)
}

func (s *StoreTestSuite) TestColumnBytes() {
	s.Equal([]byte("a"), columnBytes("a"))
	s.Equal([]byte("b"), columnBytes([]byte("b")))
	s.Nil(columnBytes(nil))
	s.Nil(columnBytes(5))
	s.Nil(resolveNullableString(""))
	s.Equal("x", resolveNullableString("x"))
}

// buildTypeFromRowWith builds a type from a row whose properties are the given JSON.
func buildTypeFromRowWith(s *StoreTestSuite, properties string) providers.AuthorizationDetailType {
	row := typeRow()
	row["properties"] = []byte(properties)
	t, err := buildTypeFromRow(row)
	s.Require().NoError(err)
	return t
}
