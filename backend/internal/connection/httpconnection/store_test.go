// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package httpconnection

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/lib/pq"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

type HTTPConnectionStoreTestSuite struct {
	suite.Suite
	ctx        context.Context
	mockClient *providermock.DBClientInterfaceMock
	store      *httpConnectionStore
}

func TestHTTPConnectionStoreTestSuite(t *testing.T) {
	suite.Run(t, new(HTTPConnectionStoreTestSuite))
}

func (s *HTTPConnectionStoreTestSuite) SetupTest() {
	initTestRuntime(s.T(), false)
	s.ctx = context.Background()
	s.mockClient = providermock.NewDBClientInterfaceMock(s.T())
	mockProvider := providermock.NewDBProviderInterfaceMock(s.T())
	mockProvider.On("GetConfigDBClient").Return(s.mockClient, nil).Maybe()
	s.store = &httpConnectionStore{dbProvider: mockProvider}
}

func (s *HTTPConnectionStoreTestSuite) connection() HTTPConnection {
	return HTTPConnection{
		ID: "conn-1", Name: "Risk", Description: "Risk engine", OUID: "ou-1", URL: testURL, TimeoutMS: 5000,
		Headers:        []Header{{Name: "X-Tenant", Value: "acme"}},
		AuthProperties: basicAuthProperties(s.T()),
	}
}

func (s *HTTPConnectionStoreTestSuite) TestCreateStoresEncryptedPropertiesAndRoundTrips() {
	var stored string
	s.mockClient.On("ExecuteContext", mock.Anything, queryCreateHTTPConnection,
		"conn-1", "Risk", "Risk engine", "ou-1", mock.Anything, testDeploymentID).
		Run(func(args mock.Arguments) { stored = args.String(6) }).Return(int64(1), nil)

	connection := s.connection()
	s.Require().NoError(s.store.CreateHTTPConnection(s.ctx, connection))

	s.NotContains(stored, testPassword)
	s.True(json.Valid([]byte(stored)))
	built, err := buildHTTPConnectionFromRow(map[string]interface{}{
		"id": "conn-1", "name": []byte("Risk"), "description": "Risk engine", "ou_id": "ou-1",
		"properties": []byte(stored),
	})
	s.Require().NoError(err)
	s.Equal(connection, built)
	config, err := outboundauth.FromProperties(built.AuthProperties)
	s.Require().NoError(err)
	s.Equal(testPassword, config.Get(outboundauth.FieldBasicPassword))
}

func (s *HTTPConnectionStoreTestSuite) TestUniqueNameViolationIsAConflict() {
	for name, dbErr := range map[string]error{
		"sqlite":   errors.New("constraint failed: UNIQUE constraint failed: " + nameIndexColumns + " (2067)"),
		"postgres": &pq.Error{Code: "23505", Constraint: nameIndexName},
	} {
		s.Run(name, func() {
			s.SetupTest()
			s.mockClient.On("ExecuteContext", mock.Anything, queryCreateHTTPConnection, mock.Anything,
				mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(int64(0), dbErr)
			s.mockClient.On("ExecuteContext", mock.Anything, queryUpdateHTTPConnection, mock.Anything,
				mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(int64(0), dbErr)

			s.ErrorIs(s.store.CreateHTTPConnection(s.ctx, s.connection()), errNameConflict)
			s.ErrorIs(s.store.UpdateHTTPConnection(s.ctx, s.connection()), errNameConflict)
		})
	}
}

func (s *HTTPConnectionStoreTestSuite) TestPrimaryKeyViolationIsAnIDConflict() {
	for name, dbErr := range map[string]error{
		"sqlite":   errors.New("constraint failed: UNIQUE constraint failed: " + primaryKeyColumns + " (1555)"),
		"postgres": &pq.Error{Code: "23505", Constraint: primaryKeyName},
	} {
		s.Run(name, func() {
			s.SetupTest()
			s.mockClient.On("ExecuteContext", mock.Anything, queryCreateHTTPConnection, mock.Anything,
				mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(int64(0), dbErr)

			err := s.store.CreateHTTPConnection(s.ctx, s.connection())

			s.ErrorIs(err, errIDConflict)
			s.NotErrorIs(err, errNameConflict)
		})
	}
}

func (s *HTTPConnectionStoreTestSuite) TestOtherWriteErrorsAreNotConflicts() {
	s.mockClient.On("ExecuteContext", mock.Anything, queryCreateHTTPConnection, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(int64(0), errors.New("disk full"))

	err := s.store.CreateHTTPConnection(s.ctx, s.connection())

	s.Require().Error(err)
	s.NotErrorIs(err, errNameConflict)
	s.NotErrorIs(err, errIDConflict)
}

func (s *HTTPConnectionStoreTestSuite) TestUpdateAndDeleteReportMissingRows() {
	s.mockClient.On("ExecuteContext", mock.Anything, queryUpdateHTTPConnection,
		"Risk", "Risk engine", mock.Anything, "conn-1", testDeploymentID).Return(int64(0), nil)
	s.mockClient.On("ExecuteContext", mock.Anything, queryDeleteHTTPConnection,
		"conn-1", testDeploymentID).Return(int64(0), nil)

	s.ErrorIs(s.store.UpdateHTTPConnection(s.ctx, s.connection()), errConnectionNotFound)
	s.ErrorIs(s.store.DeleteHTTPConnection(s.ctx, "conn-1"), errConnectionNotFound)
}

func (s *HTTPConnectionStoreTestSuite) TestUpdateAndDeleteSucceed() {
	s.mockClient.On("ExecuteContext", mock.Anything, queryUpdateHTTPConnection,
		"Risk", "Risk engine", mock.Anything, "conn-1", testDeploymentID).Return(int64(1), nil)
	s.mockClient.On("ExecuteContext", mock.Anything, queryDeleteHTTPConnection,
		"conn-1", testDeploymentID).Return(int64(1), nil)

	s.NoError(s.store.UpdateHTTPConnection(s.ctx, s.connection()))
	s.NoError(s.store.DeleteHTTPConnection(s.ctx, "conn-1"))
}

func (s *HTTPConnectionStoreTestSuite) TestGetAndList() {
	row := map[string]interface{}{"id": "conn-1", "name": "Risk", "description": nil, "ou_id": "ou-1",
		"properties": `{"url":"` + testURL + `","timeoutMs":1000}`}
	s.mockClient.On("QueryContext", mock.Anything, queryGetHTTPConnectionByID, "conn-1", testDeploymentID).
		Return([]map[string]interface{}{row}, nil)
	s.mockClient.On("QueryContext", mock.Anything, queryGetHTTPConnectionByID, "missing", testDeploymentID).
		Return([]map[string]interface{}{}, nil)
	s.mockClient.On("QueryContext", mock.Anything, queryGetHTTPConnectionByName, "Risk", testDeploymentID).
		Return([]map[string]interface{}{row}, nil)
	s.mockClient.On("QueryContext", mock.Anything, queryGetHTTPConnectionByName, "Missing", testDeploymentID).
		Return([]map[string]interface{}{}, nil)
	s.mockClient.On("QueryContext", mock.Anything, queryListHTTPConnections, testDeploymentID).
		Return([]map[string]interface{}{row}, nil)
	want := HTTPConnection{ID: "conn-1", Name: "Risk", OUID: "ou-1", URL: testURL, TimeoutMS: 1000}

	got, err := s.store.GetHTTPConnection(s.ctx, "conn-1")
	s.Require().NoError(err)
	s.Equal(&want, got)

	got, err = s.store.GetHTTPConnection(s.ctx, "missing")
	s.NoError(err)
	s.Nil(got)

	got, err = s.store.GetHTTPConnectionByName(s.ctx, "Risk")
	s.Require().NoError(err)
	s.Equal(&want, got)

	got, err = s.store.GetHTTPConnectionByName(s.ctx, "Missing")
	s.NoError(err)
	s.Nil(got)

	listed, err := s.store.ListHTTPConnections(s.ctx)
	s.Require().NoError(err)
	s.Equal([]HTTPConnection{want}, listed)
}

func (s *HTTPConnectionStoreTestSuite) TestDatabaseErrorsPropagate() {
	failure := errors.New("database unavailable")
	mockProvider := providermock.NewDBProviderInterfaceMock(s.T())
	mockProvider.On("GetConfigDBClient").Return(nil, failure)
	store := &httpConnectionStore{dbProvider: mockProvider}

	s.ErrorIs(store.CreateHTTPConnection(s.ctx, s.connection()), failure)
	_, err := store.ListHTTPConnections(s.ctx)
	s.ErrorIs(err, failure)
	_, err = store.GetHTTPConnection(s.ctx, "conn-1")
	s.ErrorIs(err, failure)
	_, err = store.GetHTTPConnectionByName(s.ctx, "Risk")
	s.ErrorIs(err, failure)
	s.ErrorIs(store.UpdateHTTPConnection(s.ctx, s.connection()), failure)
	s.ErrorIs(store.DeleteHTTPConnection(s.ctx, "conn-1"), failure)
	_, err = store.CountHTTPConnections(s.ctx)
	s.ErrorIs(err, failure)
}

func (s *HTTPConnectionStoreTestSuite) TestMalformedPropertiesAreAnError() {
	for _, raw := range []string{"", "{", `{"timeoutMs":"slow"}`, `{"authentication":{"not":"an array"}}`} {
		_, err := buildHTTPConnectionFromRow(map[string]interface{}{"id": "1", "properties": raw})
		s.Error(err, raw)
	}
}

func (s *HTTPConnectionStoreTestSuite) TestCount() {
	s.mockClient.On("QueryContext", mock.Anything, queryCountHTTPConnections, testDeploymentID).
		Return([]map[string]interface{}{{"count": int64(3)}}, nil).Once()

	count, err := s.store.CountHTTPConnections(s.ctx)
	s.Require().NoError(err)
	s.Equal(3, count)

	s.mockClient.On("QueryContext", mock.Anything, queryCountHTTPConnections, testDeploymentID).
		Return([]map[string]interface{}{{"count": "three"}}, nil).Once()
	_, err = s.store.CountHTTPConnections(s.ctx)
	s.Error(err)
}

func (s *HTTPConnectionStoreTestSuite) TestCountFailures() {
	failure := errors.New("query failed")
	s.mockClient.On("QueryContext", mock.Anything, queryCountHTTPConnections, testDeploymentID).
		Return(nil, failure).Once()
	_, err := s.store.CountHTTPConnections(s.ctx)
	s.ErrorIs(err, failure)

	s.mockClient.On("QueryContext", mock.Anything, queryCountHTTPConnections, testDeploymentID).
		Return([]map[string]interface{}{}, nil).Once()
	_, err = s.store.CountHTTPConnections(s.ctx)
	s.ErrorContains(err, "returned no rows")
}
