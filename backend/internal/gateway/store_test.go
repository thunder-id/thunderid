// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

const storeDeploymentID = "test-deployment-id"

type StoreTestSuite struct {
	suite.Suite
	providerMock *providermock.DBProviderInterfaceMock
	dbClientMock *providermock.DBClientInterfaceMock
	store        *store
}

func TestStoreTestSuite(t *testing.T) {
	suite.Run(t, new(StoreTestSuite))
}

func (s *StoreTestSuite) SetupTest() {
	s.providerMock = providermock.NewDBProviderInterfaceMock(s.T())
	s.dbClientMock = providermock.NewDBClientInterfaceMock(s.T())
	s.store = &store{dbProvider: s.providerMock, deploymentID: storeDeploymentID}
}

func (s *StoreTestSuite) expectDBClient() {
	s.providerMock.On("GetConfigDBClient").Return(s.dbClientMock, nil)
}

// Every read and write is scoped to the deployment, which is what keeps one organization's gateways
// out of another's. The deployment id is the last parameter of each query by convention.
func (s *StoreTestSuite) TestListIsDeploymentScoped() {
	s.expectDBClient()
	s.dbClientMock.On("QueryContext", mock.Anything, queryListGateways, storeDeploymentID).
		Return([]map[string]interface{}{
			{"id": "gw-1", "name": "production",
				"base_url": "https://dp.test", "management_key": "sealed"},
		}, nil).Once()

	gateways, err := s.store.List(context.Background())

	s.Require().NoError(err)
	s.Require().Len(gateways, 1)
	s.Equal("production", gateways[0].Name)
}

func (s *StoreTestSuite) TestGetByIDReturnsNilWhenAbsent() {
	s.expectDBClient()
	s.dbClientMock.On("QueryContext", mock.Anything, queryGetGatewayByID, "absent", storeDeploymentID).
		Return([]map[string]interface{}{}, nil).Once()

	gw, err := s.store.GetByID(context.Background(), "absent")

	s.Require().NoError(err)
	s.Nil(gw)
}

func (s *StoreTestSuite) TestGetByIDReadsTheWholeRow() {
	s.expectDBClient()
	s.dbClientMock.On("QueryContext", mock.Anything, queryGetGatewayByID, "gw-1", storeDeploymentID).
		Return([]map[string]interface{}{
			{"id": "gw-1", "name": "production", "base_url": "https://dp.test", "management_key": "sealed",
				"ca_certificate": "-----BEGIN CERTIFICATE-----"},
		}, nil).Once()

	gw, err := s.store.GetByID(context.Background(), "gw-1")

	s.Require().NoError(err)
	s.Require().NotNil(gw)
	s.Equal("production", gw.Name)
	s.Equal("sealed", gw.Key)
	s.Equal("-----BEGIN CERTIFICATE-----", gw.CACertificate)
}

// The name and address lookups exist to enforce uniqueness, and a declared gateway is written
// back from what they return. A partial row here would carry empty values into that write, so both
// must read every column.
func (s *StoreTestSuite) TestTheUniquenessLookupsReadTheWholeRow() {
	for name, tc := range map[string]struct {
		query dbmodel.DBQuery
		call  func() (*Gateway, error)
	}{
		"by name": {queryGetGatewayByName, func() (*Gateway, error) {
			return s.store.GetByName(context.Background(), "production")
		}},
		"by base url": {queryGetGatewayByBaseURL, func() (*Gateway, error) {
			return s.store.GetByBaseURL(context.Background(), "https://dp.test")
		}},
	} {
		s.Run(name, func() {
			s.SetupTest()
			s.expectDBClient()
			s.dbClientMock.On("QueryContext", mock.Anything, tc.query, mock.Anything, storeDeploymentID).
				Return([]map[string]interface{}{
					{"id": "gw-1", "name": "production", "base_url": "https://dp.test", "management_key": "sealed"},
				}, nil).Once()

			gw, err := tc.call()

			s.Require().NoError(err)
			s.Require().NotNil(gw)
			s.Equal("production", gw.Name, "the lookup returned a row without its name")
			s.Equal("https://dp.test", gw.BaseURL)
		})
	}
}

func (s *StoreTestSuite) TestCountReadsTheTotal() {
	s.expectDBClient()
	s.dbClientMock.On("QueryContext", mock.Anything, queryCountGateways, storeDeploymentID).
		Return([]map[string]interface{}{{"total": int64(3)}}, nil).Once()

	total, err := s.store.Count(context.Background())

	s.Require().NoError(err)
	s.Equal(3, total)
}

// The insert carries the capacity check and returns the row it wrote, so a caller never reads back
// what it just stored.
func (s *StoreTestSuite) TestCreateReturnsTheStoredRow() {
	s.expectDBClient()
	s.dbClientMock.On("QueryContext", mock.Anything, queryInsertGateway,
		"gw-1", "production", "https://dp.test", "sealed", "",
		storeDeploymentID, storeDeploymentID, 5).
		Return([]map[string]interface{}{
			{"id": "gw-1", "name": "production",
				"base_url": "https://dp.test", "management_key": "sealed"},
		}, nil).Once()

	created, err := s.store.Create(context.Background(), &Gateway{
		ID: "gw-1", Name: "production",
		BaseURL: "https://dp.test", Key: "sealed",
	}, 5)

	s.Require().NoError(err)
	s.Require().NotNil(created)
	s.Equal("production", created.Name)
}

// No row back means the capacity check refused the write, which is not an error: the service turns
// it into the limit-reached response.
func (s *StoreTestSuite) TestCreateReturnsNilWhenTheLimitRefusesIt() {
	s.expectDBClient()
	s.dbClientMock.On("QueryContext", mock.Anything, queryInsertGateway,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		storeDeploymentID, storeDeploymentID, 1).
		Return([]map[string]interface{}{}, nil).Once()

	created, err := s.store.Create(context.Background(), &Gateway{ID: "gw-1"}, 1)

	s.Require().NoError(err)
	s.Nil(created)
}

func (s *StoreTestSuite) TestUpdateIsDeploymentScoped() {
	s.expectDBClient()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryUpdateGateway,
		"gw-1", "production", "https://dp.test", "sealed", "", storeDeploymentID).
		Return(int64(1), nil).Once()

	err := s.store.Update(context.Background(), &Gateway{
		ID: "gw-1", Name: "production",
		BaseURL: "https://dp.test", Key: "sealed",
	})

	s.Require().NoError(err)
}

func (s *StoreTestSuite) TestDeleteIsDeploymentScoped() {
	s.expectDBClient()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryDeleteGateway, "gw-1", storeDeploymentID).
		Return(int64(1), nil).Once()

	s.Require().NoError(s.store.Delete(context.Background(), "gw-1"))
}

// A database failure is reported rather than returned as an empty result, which would read as
// "no gateways registered" and is the wrong answer to "the database is down".
func (s *StoreTestSuite) TestAQueryFailureIsReported() {
	s.expectDBClient()
	s.dbClientMock.On("QueryContext", mock.Anything, queryListGateways, storeDeploymentID).
		Return(nil, errors.New("connection reset")).Once()

	_, err := s.store.List(context.Background())

	s.Require().Error(err)
}

func (s *StoreTestSuite) TestAProviderFailureIsReported() {
	s.providerMock.On("GetConfigDBClient").Return(nil, errors.New("no pool")).Once()

	_, err := s.store.List(context.Background())

	s.Require().Error(err)
	s.Contains(err.Error(), "failed to get database client")
}

// Every lookup reports a database failure rather than answering "not found", which would read as
// "no such gateway" and is the wrong answer to "the database is unreachable".
func (s *StoreTestSuite) TestEveryLookupReportsAFailure() {
	failure := errors.New("connection reset")

	for name, tc := range map[string]struct {
		query dbmodel.DBQuery
		call  func() (*Gateway, error)
	}{
		"by id": {queryGetGatewayByID, func() (*Gateway, error) {
			return s.store.GetByID(context.Background(), "gw-1")
		}},
		"by name": {queryGetGatewayByName, func() (*Gateway, error) {
			return s.store.GetByName(context.Background(), "production")
		}},
		"by base url": {queryGetGatewayByBaseURL, func() (*Gateway, error) {
			return s.store.GetByBaseURL(context.Background(), "https://dp.test")
		}},
	} {
		s.Run(name, func() {
			s.SetupTest()
			s.expectDBClient()
			s.dbClientMock.On("QueryContext", mock.Anything, tc.query, mock.Anything, storeDeploymentID).
				Return(nil, failure).Once()

			gw, err := tc.call()

			s.Require().Error(err)
			s.Nil(gw)
		})
	}
}

// A lookup that matches nothing is not an error: the caller asked whether it exists.
func (s *StoreTestSuite) TestEveryLookupReportsAbsenceAsNil() {
	for name, tc := range map[string]struct {
		query dbmodel.DBQuery
		call  func() (*Gateway, error)
	}{
		"by name": {queryGetGatewayByName, func() (*Gateway, error) {
			return s.store.GetByName(context.Background(), "absent")
		}},
		"by base url": {queryGetGatewayByBaseURL, func() (*Gateway, error) {
			return s.store.GetByBaseURL(context.Background(), "https://absent")
		}},
	} {
		s.Run(name, func() {
			s.SetupTest()
			s.expectDBClient()
			s.dbClientMock.On("QueryContext", mock.Anything, tc.query, mock.Anything, storeDeploymentID).
				Return([]map[string]interface{}{}, nil).Once()

			gw, err := tc.call()

			s.Require().NoError(err)
			s.Nil(gw)
		})
	}
}

// Writes report their failures too.
func (s *StoreTestSuite) TestWritesReportFailures() {
	failure := errors.New("deadlock detected")

	s.Run("create", func() {
		s.SetupTest()
		s.expectDBClient()
		s.dbClientMock.On("QueryContext", mock.Anything, queryInsertGateway,
			mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
			storeDeploymentID, storeDeploymentID, 5).Return(nil, failure).Once()

		_, err := s.store.Create(context.Background(), &Gateway{ID: "gw-1"}, 5)

		s.Require().Error(err)
	})

	s.Run("update", func() {
		s.SetupTest()
		s.expectDBClient()
		s.dbClientMock.On("ExecuteContext", mock.Anything, queryUpdateGateway,
			mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
			storeDeploymentID).Return(int64(0), failure).Once()

		s.Require().Error(s.store.Update(context.Background(), &Gateway{ID: "gw-1"}))
	})

	s.Run("delete", func() {
		s.SetupTest()
		s.expectDBClient()
		s.dbClientMock.On("ExecuteContext", mock.Anything, queryDeleteGateway, "gw-1", storeDeploymentID).
			Return(int64(0), failure).Once()

		s.Require().Error(s.store.Delete(context.Background(), "gw-1"))
	})

	s.Run("count", func() {
		s.SetupTest()
		s.expectDBClient()
		s.dbClientMock.On("QueryContext", mock.Anything, queryCountGateways, storeDeploymentID).
			Return(nil, failure).Once()

		_, err := s.store.Count(context.Background())

		s.Require().Error(err)
	})
}

// A row without an id is a row this store cannot use, and saying so beats returning a gateway whose
// id is empty.
func (s *StoreTestSuite) TestARowWithoutAnIDIsRejected() {
	s.expectDBClient()
	s.dbClientMock.On("QueryContext", mock.Anything, queryGetGatewayByID, "gw-1", storeDeploymentID).
		Return([]map[string]interface{}{{"name": "production"}}, nil).Once()

	_, err := s.store.GetByID(context.Background(), "gw-1")

	s.Require().Error(err)
}
