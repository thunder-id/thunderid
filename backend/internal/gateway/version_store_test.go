// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

type VersionStoreTestSuite struct {
	suite.Suite
	providerMock *providermock.DBProviderInterfaceMock
	dbClientMock *providermock.DBClientInterfaceMock
	store        *versionStore
}

func TestVersionStoreTestSuite(t *testing.T) {
	suite.Run(t, new(VersionStoreTestSuite))
}

func (s *VersionStoreTestSuite) SetupTest() {
	s.providerMock = providermock.NewDBProviderInterfaceMock(s.T())
	s.dbClientMock = providermock.NewDBClientInterfaceMock(s.T())
	s.store = &versionStore{dbProvider: s.providerMock, deploymentID: storeDeploymentID}
	s.providerMock.On("GetConfigDBClient").Return(s.dbClientMock, nil).Maybe()
}

// A capture is numbered by the statement itself, scoped to the deployment.
func (s *VersionStoreTestSuite) TestAddNumbersTheVersionWithinTheDeployment() {
	s.dbClientMock.On("QueryContext", mock.Anything, queryInsertVersion, mock.Anything, "docs", "sealed",
		"note", storeDeploymentID, storeDeploymentID).
		Return([]map[string]interface{}{{"seq": int64(3), "note": "note"}}, nil).Once()

	v, err := s.store.Add(context.Background(), "docs", "sealed", "note")

	s.Require().NoError(err)
	s.Equal(3, v.Seq)
	s.Equal("docs", v.Resources)
}

func (s *VersionStoreTestSuite) TestReadsAreDeploymentScoped() {
	s.dbClientMock.On("QueryContext", mock.Anything, queryListVersions, storeDeploymentID).
		Return([]map[string]interface{}{{"seq": int64(2)}, {"seq": int64(1)}}, nil).Once()
	s.dbClientMock.On("QueryContext", mock.Anything, queryGetVersion, 2, storeDeploymentID).
		Return([]map[string]interface{}{{"seq": int64(2), "resources": "docs", "variables": "sealed"}}, nil).Once()
	s.dbClientMock.On("QueryContext", mock.Anything, queryGetVersion, 9, storeDeploymentID).
		Return([]map[string]interface{}{}, nil).Once()
	s.dbClientMock.On("QueryContext", mock.Anything, queryLatestVersion, storeDeploymentID).
		Return([]map[string]interface{}{{"latest": int64(2)}}, nil).Once()

	listed, err := s.store.List(context.Background())
	s.Require().NoError(err)
	s.Len(listed, 2)

	v, err := s.store.Get(context.Background(), 2)
	s.Require().NoError(err)
	s.Equal("sealed", v.variables)

	absent, err := s.store.Get(context.Background(), 9)
	s.Require().NoError(err)
	s.Nil(absent)

	latest, err := s.store.Latest(context.Background())
	s.Require().NoError(err)
	s.Equal(2, latest)
}

// Pruning spares a version a gateway holds or could revert to, which the statement decides.
func (s *VersionStoreTestSuite) TestPruneNamesTheBoundAndTheDeployment() {
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryPruneVersions, 4,
		storeDeploymentID, storeDeploymentID, storeDeploymentID).Return(int64(1), nil).Once()

	s.Require().NoError(s.store.Prune(context.Background(), 4))
	s.Require().NoError(s.store.Prune(context.Background(), 0), "nothing to prune below the bound")
}

func (s *VersionStoreTestSuite) TestAppliedStateIsReadWrittenAndForgotten() {
	s.dbClientMock.On("QueryContext", mock.Anything, queryGetAppliedVersion, "gw-1", storeDeploymentID).
		Return([]map[string]interface{}{{"applied_seq": int64(2), "previous_seq": nil}}, nil).Once()
	s.dbClientMock.On("QueryContext", mock.Anything, queryGetAppliedVersion, "gw-2", storeDeploymentID).
		Return([]map[string]interface{}{}, nil).Once()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryUpsertAppliedVersion, "gw-1", 3, 2,
		storeDeploymentID, 3, storeDeploymentID, 2).Return(int64(1), nil).Once()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryUpsertAppliedVersion, "gw-1", 1, nil,
		storeDeploymentID, 1, storeDeploymentID, 0).Return(int64(1), nil).Once()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryDeleteAppliedVersion, "gw-1",
		storeDeploymentID).Return(int64(1), nil).Once()

	held, err := s.store.GetApplied(context.Background(), "gw-1")
	s.Require().NoError(err)
	s.Equal(2, held.AppliedVersion)
	s.Zero(held.PreviousVersion, "a NULL previous version reads as none")

	nothing, err := s.store.GetApplied(context.Background(), "gw-2")
	s.Require().NoError(err)
	s.Nil(nothing)

	s.Require().NoError(s.store.SetApplied(context.Background(), "gw-1", 3, 2, 2))
	s.Require().NoError(s.store.SetApplied(context.Background(), "gw-1", 1, 0, 0), "no previous is stored as NULL")
	s.Require().NoError(s.store.DeleteApplied(context.Background(), "gw-1"))
}

// Nothing recorded means either the version was removed while it was applied, or another apply to the
// gateway finished first, and the store says which.
func (s *VersionStoreTestSuite) TestAnUnrecordedApplySaysWhy() {
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryUpsertAppliedVersion, "gw-1", 3, 2,
		storeDeploymentID, 3, storeDeploymentID, 2).Return(int64(0), nil).Twice()
	s.dbClientMock.On("QueryContext", mock.Anything, queryGetVersion, 3, storeDeploymentID).
		Return([]map[string]interface{}{}, nil).Once()
	s.dbClientMock.On("QueryContext", mock.Anything, queryGetVersion, 3, storeDeploymentID).
		Return([]map[string]interface{}{{"seq": int64(3), "resources": "", "note": ""}}, nil).Once()

	s.ErrorIs(s.store.SetApplied(context.Background(), "gw-1", 3, 2, 2), errVersionRemoved)
	s.ErrorIs(s.store.SetApplied(context.Background(), "gw-1", 3, 2, 2), errAppliedChanged)
}

func (s *VersionStoreTestSuite) TestEveryCallReportsAFailure() {
	failure := errors.New("database is down")
	s.dbClientMock.On("QueryContext", mock.Anything, mock.Anything, mock.Anything).Return(nil, failure).Maybe()
	s.dbClientMock.On("QueryContext", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, failure).Maybe()
	s.dbClientMock.On("QueryContext", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, failure).Maybe()
	s.dbClientMock.On("ExecuteContext", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(int64(0), failure).Maybe()
	s.dbClientMock.On("ExecuteContext", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything).Return(int64(0), failure).Maybe()
	s.dbClientMock.On("ExecuteContext", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything).Return(int64(0), failure).Maybe()
	s.dbClientMock.On("ExecuteContext", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(int64(0), failure).Maybe()

	ctx := context.Background()
	_, err := s.store.Add(ctx, "", "", "")
	s.Error(err)
	_, err = s.store.List(ctx)
	s.Error(err)
	_, err = s.store.Get(ctx, 1)
	s.Error(err)
	_, err = s.store.Latest(ctx)
	s.Error(err)
	_, err = s.store.GetApplied(ctx, "gw-1")
	s.Error(err)
	s.Error(s.store.Prune(ctx, 1))
	s.Error(s.store.SetApplied(ctx, "gw-1", 1, 0, 0))
	s.Error(s.store.DeleteApplied(ctx, "gw-1"))
}

func (s *VersionStoreTestSuite) TestAProviderFailureIsReported() {
	provider := providermock.NewDBProviderInterfaceMock(s.T())
	provider.On("GetConfigDBClient").Return(nil, errors.New("no database"))
	store := &versionStore{dbProvider: provider, deploymentID: storeDeploymentID}

	_, err := store.List(context.Background())
	s.Error(err)
	s.Error(store.DeleteApplied(context.Background(), "gw-1"))
}
