// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"errors"

	"github.com/stretchr/testify/mock"
)

// A master is numbered by the statement itself, per gateway within the deployment, and its manifest
// is stored with each actor's id alone.
func (s *VersionStoreTestSuite) TestAddMasterNumbersItWithinTheGateway() {
	master := deployedMaster("v1", "ou-a")
	master.Entries[0].By.Name = "alice"
	manifest, err := manifestOf(master.Entries)
	s.Require().NoError(err)
	s.dbClientMock.On("QueryContext", mock.Anything, queryInsertMaster, mock.Anything, "gw-1", master.Hash,
		master.Resources, manifest, "u-1", storeDeploymentID, "gw-1", storeDeploymentID).
		Return([]map[string]interface{}{{"seq": int64(3), "created_at": "2026-10-28T09:00:00Z"}}, nil).Once()

	kept, err := s.store.AddMaster(scoped(), *master)

	s.Require().NoError(err)
	s.Equal(3, kept.Seq)
	s.Equal(masterAt, kept.CreatedAt)
	s.Equal(master.Hash, kept.Hash)
	s.Equal(master.Entries, kept.Entries)
}

// The master a gateway holds is the newest row, read with its resources; a gateway without one reads
// as none.
func (s *VersionStoreTestSuite) TestGetMasterReadsTheNewestRow() {
	master := deployedMaster("v1", "ou-a")
	manifest, err := manifestOf(master.Entries)
	s.Require().NoError(err)
	s.dbClientMock.On("QueryContext", mock.Anything, queryGetMaster, "gw-1", storeDeploymentID).
		Return([]map[string]interface{}{{"seq": int64(2), "hash": master.Hash, "resources": master.Resources,
			"manifest": manifest, "created_at": "2026-10-28T09:00:00Z", "created_by": "u-1"}}, nil).Once()
	s.dbClientMock.On("QueryContext", mock.Anything, queryGetMaster, "gw-2", storeDeploymentID).
		Return([]map[string]interface{}{}, nil).Once()

	read, err := s.store.GetMaster(scoped(), "gw-1")

	s.Require().NoError(err)
	s.Equal(2, read.Seq)
	s.Equal("gw-1", read.GatewayID)
	s.Equal(master.Resources, read.Resources)
	s.Equal(master.Entries, read.Entries)
	s.Equal(MasterSummary{Deployed: 1}, read.Summary)
	s.Equal(&Actor{ID: "u-1"}, read.CreatedBy)

	none, err := s.store.GetMaster(scoped(), "gw-2")
	s.Require().NoError(err)
	s.Nil(none)
}

// A listing reads every master of the gateway, newest first as the statement orders them, without
// resources; a manifest that does not read fails the listing.
func (s *VersionStoreTestSuite) TestListMastersReadsEachManifest() {
	s.dbClientMock.On("QueryContext", mock.Anything, queryListMasters, "gw-1", storeDeploymentID).
		Return([]map[string]interface{}{{"seq": int64(2), "hash": "bbb", "manifest": "[]"},
			{"seq": int64(1), "hash": "aaa", "manifest": ""}}, nil).Once()
	s.dbClientMock.On("QueryContext", mock.Anything, queryListMasters, "gw-2", storeDeploymentID).
		Return([]map[string]interface{}{{"seq": int64(1), "hash": "ccc", "manifest": "{"}}, nil).Once()

	masters, err := s.store.ListMasters(scoped(), "gw-1")

	s.Require().NoError(err)
	s.Require().Len(masters, 2)
	s.Equal([]int{2, 1}, []int{masters[0].Seq, masters[1].Seq})
	s.Empty(masters[0].Resources)
	s.Empty(masters[1].Entries)

	_, err = s.store.ListMasters(scoped(), "gw-2")
	s.Error(err)
}

// Pruning and forgetting are scoped to the gateway within the deployment; nothing is pruned below 1.
func (s *VersionStoreTestSuite) TestMastersArePrunedAndForgottenPerGateway() {
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryPruneMasters, "gw-1", 4, storeDeploymentID).
		Return(int64(2), nil).Once()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryDeleteMasters, "gw-1", storeDeploymentID).
		Return(int64(3), nil).Once()

	s.Require().NoError(s.store.PruneMasters(scoped(), "gw-1", 4))
	s.Require().NoError(s.store.PruneMasters(scoped(), "gw-1", 0))
	s.Require().NoError(s.store.DeleteMasters(scoped(), "gw-1"))
}

// A database that cannot answer fails each call.
func (s *VersionStoreTestSuite) TestMasterCallsReportDatabaseFailures() {
	down := errors.New("database is down")
	s.dbClientMock.On("QueryContext", mock.Anything, queryInsertMaster, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, down).Once()
	s.dbClientMock.On("QueryContext", mock.Anything, queryGetMaster, "gw-1", storeDeploymentID).
		Return(nil, down).Once()
	s.dbClientMock.On("QueryContext", mock.Anything, queryListMasters, "gw-1", storeDeploymentID).
		Return(nil, down).Once()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryPruneMasters, "gw-1", 1, storeDeploymentID).
		Return(int64(0), down).Once()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryDeleteMasters, "gw-1", storeDeploymentID).
		Return(int64(0), down).Once()

	_, err := s.store.AddMaster(scoped(), *deployedMaster("v1", "ou-a"))
	s.Error(err)
	_, err = s.store.GetMaster(scoped(), "gw-1")
	s.Error(err)
	_, err = s.store.ListMasters(scoped(), "gw-1")
	s.Error(err)
	s.Error(s.store.PruneMasters(scoped(), "gw-1", 1))
	s.Error(s.store.DeleteMasters(scoped(), "gw-1"))
}
