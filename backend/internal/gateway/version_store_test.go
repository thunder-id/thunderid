// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/deployment"
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
	s.store = &versionStore{dbProvider: s.providerMock}
	s.providerMock.On("GetConfigDBClient").Return(s.dbClientMock, nil).Maybe()
}

// A capture is placed in order by the statement itself, scoped to the deployment, and keeps its hash
// and name.
func (s *VersionStoreTestSuite) TestAddPlacesTheVersionWithinTheDeployment() {
	s.dbClientMock.On("QueryContext", mock.Anything, queryInsertVersion, mock.Anything, "abc123", "Spring",
		"docs", "sealed", "note", storeDeploymentID, storeDeploymentID).
		Return([]map[string]interface{}{{"seq": int64(3)}}, nil).Once()

	v, err := s.store.Add(scoped(),
		Version{Hash: "abc123", Name: "Spring", Note: "note", Resources: "docs", variables: "sealed"})

	s.Require().NoError(err)
	s.Equal(3, v.Seq)
	s.Equal("abc123", v.Hash)
	s.Equal("Spring", v.Name)
	s.Equal("docs", v.Resources)
	s.Empty(v.variables, "a capture's answer carried its sealed values")
}

// A prefix is looked up within the deployment, and at most two versions are read back.
func (s *VersionStoreTestSuite) TestFindLooksUpAHashPrefix() {
	s.dbClientMock.On("QueryContext", mock.Anything, queryFindVersions, "abc1234%", storeDeploymentID).
		Return([]map[string]interface{}{{"seq": int64(5)}, {"seq": int64(2)}}, nil).Once()

	found, err := s.store.Find(scoped(), "abc1234")

	s.Require().NoError(err)
	s.Equal([]int{5, 2}, found)
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

	listed, err := s.store.List(scoped())
	s.Require().NoError(err)
	s.Len(listed, 2)

	v, err := s.store.Get(scoped(), 2)
	s.Require().NoError(err)
	s.Equal("sealed", v.variables)

	absent, err := s.store.Get(scoped(), 9)
	s.Require().NoError(err)
	s.Nil(absent)

	latest, err := s.store.Latest(scoped())
	s.Require().NoError(err)
	s.Equal(2, latest)
}

// Pruning spares a version a gateway holds or could revert to, which the statement decides.
func (s *VersionStoreTestSuite) TestPruneNamesTheBoundAndTheDeployment() {
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryPruneVersions, 4,
		storeDeploymentID, storeDeploymentID, storeDeploymentID).Return(int64(1), nil).Once()

	s.Require().NoError(s.store.Prune(scoped(), 4))
	s.Require().NoError(s.store.Prune(scoped(), 0), "nothing to prune below the bound")
}

func (s *VersionStoreTestSuite) TestAppliedStateIsReadWrittenAndForgotten() {
	s.dbClientMock.On("QueryContext", mock.Anything, queryGetAppliedVersion, "gw-1", storeDeploymentID).
		Return([]map[string]interface{}{{"applied_seq": int64(2), "previous_seq": nil, "applied_hash": "abc123",
			"previous_hash": nil}}, nil).Once()
	s.dbClientMock.On("QueryContext", mock.Anything, queryGetAppliedVersion, "gw-2", storeDeploymentID).
		Return([]map[string]interface{}{}, nil).Once()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryUpsertAppliedVersion, "gw-1", 3, 2,
		storeDeploymentID, 3, storeDeploymentID, 2).Return(int64(1), nil).Once()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryUpsertAppliedVersion, "gw-1", 1, nil,
		storeDeploymentID, 1, storeDeploymentID, 0).Return(int64(1), nil).Once()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryDeleteAppliedVersion, "gw-1",
		storeDeploymentID).Return(int64(1), nil).Once()

	held, err := s.store.GetApplied(scoped(), "gw-1")
	s.Require().NoError(err)
	s.Equal(2, held.AppliedVersion)
	s.Equal("abc123", held.Applied)
	s.Zero(held.PreviousVersion, "a NULL previous version reads as none")
	s.Empty(held.Previous)

	nothing, err := s.store.GetApplied(scoped(), "gw-2")
	s.Require().NoError(err)
	s.Nil(nothing)

	s.Require().NoError(s.store.SetApplied(scoped(), "gw-1", 3, 2, 2))
	s.Require().NoError(s.store.SetApplied(scoped(), "gw-1", 1, 0, 0), "no previous is stored as NULL")
	s.Require().NoError(s.store.DeleteApplied(scoped(), "gw-1"))
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

	s.ErrorIs(s.store.SetApplied(scoped(), "gw-1", 3, 2, 2), errVersionRemoved)
	s.ErrorIs(s.store.SetApplied(scoped(), "gw-1", 3, 2, 2), errAppliedChanged)
}

// A gateway's choice is read, written in one transaction, and forgotten, all within the deployment.
func (s *VersionStoreTestSuite) TestExclusionsAreReadWrittenInOneTransactionAndForgotten() {
	tx := &recordingTransactioner{}
	s.providerMock.On("GetConfigDBTransactioner").Return(tx, nil).Once()
	s.dbClientMock.On("QueryContext", mock.Anything, queryGetExcluded, "gw-1", storeDeploymentID).
		Return([]map[string]interface{}{{"resource_key": "organization_unit/ou-a",
			"resource_type": "organization_unit", "resource_id": "ou-a", "resource_name": nil}}, nil).Once()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryExclude, "gw-1", "organization_unit/ou-b",
		"organization_unit", "ou-b", nil, storeDeploymentID).Return(int64(1), nil).Once()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryInclude, "gw-1", "organization_unit/ou-a",
		storeDeploymentID).Return(int64(1), nil).Once()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryDeleteExcluded, "gw-1", storeDeploymentID).
		Return(int64(1), nil).Once()

	excluded, err := s.store.GetExcluded(scoped(), "gw-1")
	s.Require().NoError(err)
	s.Equal([]excludedResource{{Key: "organization_unit/ou-a", Type: "organization_unit", ID: "ou-a"}}, excluded)
	s.Require().NoError(s.store.SetExcluded(scoped(), "gw-1",
		[]excludedResource{{Key: "organization_unit/ou-b", Type: "organization_unit", ID: "ou-b"}},
		[]string{"organization_unit/ou-a"}))
	s.True(tx.ran)
	s.False(tx.rolledBack)
	s.Require().NoError(s.store.DeleteExcluded(scoped(), "gw-1"))
}

// A write cut short rolls the whole choice back, so it is never left half made.
func (s *VersionStoreTestSuite) TestAnExclusionWriteCutShortRollsBack() {
	tx := &recordingTransactioner{}
	s.providerMock.On("GetConfigDBTransactioner").Return(tx, nil).Once()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryExclude, "gw-1", "organization_unit/ou-b",
		"organization_unit", "ou-b", "b", storeDeploymentID).Return(int64(1), nil).Once()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryInclude, "gw-1", "organization_unit/ou-a",
		storeDeploymentID).Return(int64(0), errors.New("database is down")).Once()

	err := s.store.SetExcluded(scoped(), "gw-1",
		[]excludedResource{{Key: "organization_unit/ou-b", Type: "organization_unit", ID: "ou-b", Name: "b"}},
		[]string{"organization_unit/ou-a"})

	s.Error(err)
	s.True(tx.rolledBack)

	provider := providermock.NewDBProviderInterfaceMock(s.T())
	provider.On("GetConfigDBTransactioner").Return(nil, errors.New("no database")).Once()
	store := &versionStore{dbProvider: provider}
	s.Error(store.SetExcluded(scoped(), "gw-1", nil, []string{"organization_unit/ou-a"}))
}

func (s *VersionStoreTestSuite) TestEveryCallReportsAFailure() {
	failure := errors.New("database is down")
	s.dbClientMock.On("QueryContext", mock.Anything, mock.Anything, mock.Anything).Return(nil, failure).Maybe()
	s.dbClientMock.On("QueryContext", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, failure).Maybe()
	s.dbClientMock.On("QueryContext", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, failure).Maybe()
	s.dbClientMock.On("QueryContext", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, failure).Maybe()
	s.dbClientMock.On("ExecuteContext", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(int64(0), failure).Maybe()
	s.dbClientMock.On("ExecuteContext", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything).Return(int64(0), failure).Maybe()
	s.dbClientMock.On("ExecuteContext", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything).Return(int64(0), failure).Maybe()
	s.dbClientMock.On("ExecuteContext", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(int64(0), failure).Maybe()

	ctx := scoped()
	_, err := s.store.Add(ctx, Version{})
	s.Error(err)
	_, err = s.store.Find(ctx, "abc1234")
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
	_, err = s.store.GetExcluded(ctx, "gw-1")
	s.Error(err)
	s.Error(s.store.DeleteExcluded(ctx, "gw-1"))
}

func (s *VersionStoreTestSuite) TestAProviderFailureIsReported() {
	provider := providermock.NewDBProviderInterfaceMock(s.T())
	provider.On("GetConfigDBClient").Return(nil, errors.New("no database"))
	store := &versionStore{dbProvider: provider}

	_, err := store.List(scoped())
	s.Error(err)
	s.Error(store.DeleteApplied(scoped(), "gw-1"))
}

// Versions, what a gateway runs and what it leaves alone are read in each request's own deployment,
// so one organization never reads another's.
func (s *VersionStoreTestSuite) TestEachRequestIsScopedToItsOwnDeployment() {
	s.dbClientMock.On("QueryContext", mock.Anything, queryListVersions, "org-a").
		Return([]map[string]interface{}{{"seq": int64(1)}}, nil).Once()
	s.dbClientMock.On("QueryContext", mock.Anything, queryListVersions, "org-b").
		Return([]map[string]interface{}{}, nil).Once()
	s.dbClientMock.On("QueryContext", mock.Anything, queryGetAppliedVersion, "gw-1", "org-b").
		Return([]map[string]interface{}{}, nil).Once()
	s.dbClientMock.On("QueryContext", mock.Anything, queryGetExcluded, "gw-1", "org-b").
		Return([]map[string]interface{}{}, nil).Once()
	orgA := deployment.WithID(context.Background(), "org-a")
	orgB := deployment.WithID(context.Background(), "org-b")

	a, err := s.store.List(orgA)
	s.Require().NoError(err)
	b, err := s.store.List(orgB)
	s.Require().NoError(err)
	_, err = s.store.GetApplied(orgB, "gw-1")
	s.Require().NoError(err)
	_, err = s.store.GetExcluded(orgB, "gw-1")
	s.Require().NoError(err)

	s.Len(a, 1)
	s.Empty(b)
}
