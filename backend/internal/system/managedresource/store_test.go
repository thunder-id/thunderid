// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package managedresource

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

const testDeploymentID = "test-deployment"

var errDatabase = errors.New("database unavailable")

type StoreTestSuite struct {
	suite.Suite
	ctx      context.Context
	provider *providermock.DBProviderInterfaceMock
	client   *providermock.DBClientInterfaceMock
	store    *store
}

func TestStoreTestSuite(t *testing.T) {
	suite.Run(t, new(StoreTestSuite))
}

func (s *StoreTestSuite) SetupTest() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime("", &config.Config{
		Server: engineconfig.ServerConfig{Identifier: testDeploymentID},
	}))
	s.ctx = context.Background()
	s.provider = providermock.NewDBProviderInterfaceMock(s.T())
	s.client = providermock.NewDBClientInterfaceMock(s.T())
	s.provider.On("GetConfigDBClient").Return(s.client, nil).Maybe()
	s.store = &store{dbProvider: s.provider}
}

func (s *StoreTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *StoreTestSuite) TestMarkWritesTheResourceRow() {
	s.client.On("ExecuteContext", s.ctx, queryMark, "group", "g-1", "", "", testDeploymentID).
		Return(int64(1), nil).Once()
	s.NoError(s.store.Mark(s.ctx, "group", "g-1"))
}

func (s *StoreTestSuite) TestUnmarkForgetsTheResourceAndItsEntries() {
	s.client.On("ExecuteContext", s.ctx, queryUnmark, "group", "g-1", testDeploymentID).
		Return(int64(3), nil).Once()
	s.NoError(s.store.Unmark(s.ctx, "group", "g-1"))
}

func (s *StoreTestSuite) TestAWriteThatFailsIsReported() {
	s.client.On("ExecuteContext", s.ctx, queryMark, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything).Return(int64(0), errDatabase)
	s.ErrorIs(s.store.Mark(s.ctx, "group", "g-1"), errDatabase)

	s.client.On("ExecuteContext", s.ctx, mock.Anything, "group", "g-1", PartMember, "user/u-1", testDeploymentID).
		Return(int64(0), errDatabase)
	s.ErrorIs(s.store.MarkParts(s.ctx, "group", "g-1", PartMember, []string{"user/u-1"}), errDatabase)
}

func (s *StoreTestSuite) TestNoDatabaseClientIsReported() {
	provider := providermock.NewDBProviderInterfaceMock(s.T())
	provider.On("GetConfigDBClient").Return(nil, errDatabase)
	unavailable := &store{dbProvider: provider}

	s.ErrorIs(unavailable.Mark(s.ctx, "group", "g-1"), errDatabase)
	_, err := unavailable.IsManaged(s.ctx, "g-1", "group")
	s.ErrorIs(err, errDatabase)
}

func (s *StoreTestSuite) TestIsManagedAnswersOnlyForTheTypesAsked() {
	s.client.On("QueryContext", s.ctx, queryGetTypes, "id-1", testDeploymentID).
		Return([]map[string]interface{}{{"resource_type": "application"}}, nil)

	managed, err := s.store.IsManaged(s.ctx, "id-1", "user", "application")
	s.NoError(err)
	s.True(managed)

	managed, err = s.store.IsManaged(s.ctx, "id-1", "group")
	s.NoError(err)
	s.False(managed)
}

func (s *StoreTestSuite) TestIsManagedReportsAFailedRead() {
	s.client.On("QueryContext", s.ctx, queryGetTypes, "id-1", testDeploymentID).Return(nil, errDatabase)
	_, err := s.store.IsManaged(s.ctx, "id-1", "group")
	s.ErrorIs(err, errDatabase)
}

func (s *StoreTestSuite) TestManagedIDsReadsAListInOneQuery() {
	s.client.On("QueryContext", s.ctx, mock.MatchedBy(func(q dbmodel.DBQuery) bool {
		return q.ID == "MRQ-MANAGED_MGT-07" &&
			q.Query == `SELECT RESOURCE_ID, RESOURCE_TYPE FROM "MANAGED_RESOURCE" `+
				`WHERE RESOURCE_ID IN ($1,$2,$3) AND PART = '' AND DEPLOYMENT_ID = $4`
	}), "a", "b", "c", testDeploymentID).Return([]map[string]interface{}{
		{"resource_id": "a", "resource_type": "group"},
		{"resource_id": "b", "resource_type": "role"},
	}, nil).Once()

	managed, err := s.store.ManagedIDs(s.ctx, []string{"a", "b", "c"}, "group")
	s.NoError(err)
	s.Equal(map[string]bool{"a": true}, managed)
}

func (s *StoreTestSuite) TestManagedIDsOfNothingReadsNothing() {
	managed, err := s.store.ManagedIDs(s.ctx, nil, "group")
	s.NoError(err)
	s.Empty(managed)
	s.client.AssertNotCalled(s.T(), "QueryContext")
}

func (s *StoreTestSuite) TestManagedIDsReportsAFailedRead() {
	s.client.On("QueryContext", s.ctx, mock.Anything, "a", testDeploymentID).Return(nil, errDatabase)
	_, err := s.store.ManagedIDs(s.ctx, []string{"a"}, "group")
	s.ErrorIs(err, errDatabase)
}

func (s *StoreTestSuite) TestPartsAreMarkedInOneStatement() {
	s.client.On("ExecuteContext", s.ctx, mock.MatchedBy(func(q dbmodel.DBQuery) bool {
		return q.ID == "MRQ-MANAGED_MGT-08" &&
			q.Query == `INSERT INTO "MANAGED_RESOURCE" (RESOURCE_TYPE, RESOURCE_ID, PART, PART_KEY, DEPLOYMENT_ID) `+
				`VALUES ($1, $2, $3, $4, $5), ($6, $7, $8, $9, $10) ON CONFLICT DO NOTHING`
	}), "group", "g-1", PartMember, "user/u-1", testDeploymentID,
		"group", "g-1", PartMember, "group/g-2", testDeploymentID).Return(int64(2), nil).Once()
	s.NoError(s.store.MarkParts(s.ctx, "group", "g-1", PartMember, []string{"user/u-1", "group/g-2"}))
}

func (s *StoreTestSuite) TestPartsAreForgottenInOneStatement() {
	s.client.On("ExecuteContext", s.ctx, mock.MatchedBy(func(q dbmodel.DBQuery) bool {
		return q.ID == "MRQ-MANAGED_MGT-09" &&
			q.Query == `DELETE FROM "MANAGED_RESOURCE" WHERE RESOURCE_TYPE = $1 AND RESOURCE_ID = $2 AND PART = $3 `+
				`AND DEPLOYMENT_ID = $4 AND PART_KEY IN ($5,$6)`
	}), "group", "g-1", PartMember, testDeploymentID, "user/u-1", "group/g-2").Return(int64(2), nil).Once()
	s.NoError(s.store.UnmarkParts(s.ctx, "group", "g-1", PartMember, []string{"user/u-1", "group/g-2"}))
}

// A group with more members than one statement takes is marked in as many statements as that needs, each
// within the parameters a database takes.
func (s *StoreTestSuite) TestManyPartsAreMarkedInChunks() {
	keys := make([]string, 0, partsChunk+1)
	for i := 0; i <= partsChunk; i++ {
		keys = append(keys, fmt.Sprintf("user/u-%d", i))
	}
	s.Len(slices.Collect(slices.Chunk(keys, partsChunk)), 2)
	query, args := buildMarkPartsQuery("group", "g-1", PartMember, keys[:partsChunk], testDeploymentID)
	s.Len(args, partsChunk*5)
	s.Equal(partsChunk*5, strings.Count(query.Query, "$"))
}

func (s *StoreTestSuite) TestForgettingPartsThatFailsIsReported() {
	s.client.On("ExecuteContext", s.ctx, mock.Anything, "group", "g-1", PartMember, testDeploymentID, "user/u-1").
		Return(int64(0), errDatabase)
	s.ErrorIs(s.store.UnmarkParts(s.ctx, "group", "g-1", PartMember, []string{"user/u-1"}), errDatabase)
}

func (s *StoreTestSuite) TestPartKeysReadsTheMarkedEntries() {
	s.client.On("QueryContext", s.ctx, queryGetPartKeys, "role", "r-1", PartAssignment, testDeploymentID).
		Return([]map[string]interface{}{{"part_key": "ou-1/user/u-1"}, {"part_key": "ou-1/group/g-1"}}, nil)
	keys, err := s.store.PartKeys(s.ctx, "role", "r-1", PartAssignment)
	s.NoError(err)
	s.Equal([]string{"ou-1/user/u-1", "ou-1/group/g-1"}, keys)
}

func (s *StoreTestSuite) TestPartKeysReportsAFailedRead() {
	s.client.On("QueryContext", s.ctx, queryGetPartKeys, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything).Return(nil, errDatabase)
	_, err := s.store.PartKeys(s.ctx, "role", "r-1", PartAssignment)
	s.ErrorIs(err, errDatabase)
}

func (s *StoreTestSuite) TestIsPartManaged() {
	s.client.On("QueryContext", s.ctx, queryIsPartMarked, "group", "g-1", PartMember, "user/u-1",
		testDeploymentID).Return([]map[string]interface{}{{"marked": int64(1)}}, nil).Once()
	s.client.On("QueryContext", s.ctx, queryIsPartMarked, "group", "g-1", PartMember, "user/u-2",
		testDeploymentID).Return([]map[string]interface{}{}, nil).Once()
	s.client.On("QueryContext", s.ctx, queryIsPartMarked, "group", "g-1", PartMember, "user/u-3",
		testDeploymentID).Return(nil, errDatabase).Once()

	managed, err := s.store.IsPartManaged(s.ctx, "group", "g-1", PartMember, "user/u-1")
	s.NoError(err)
	s.True(managed)
	managed, err = s.store.IsPartManaged(s.ctx, "group", "g-1", PartMember, "user/u-2")
	s.NoError(err)
	s.False(managed)
	_, err = s.store.IsPartManaged(s.ctx, "group", "g-1", PartMember, "user/u-3")
	s.ErrorIs(err, errDatabase)
}

func (s *StoreTestSuite) TestInitializeMakesTheStoreEveryServiceReads() {
	defer func() { defaultStore = nil }()
	s.Nil(GetStore())
	created := Initialize()
	s.NotNil(created)
	s.Same(created, GetStore())
}
