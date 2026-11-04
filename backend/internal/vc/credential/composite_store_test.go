// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package credential

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type CompositeCredentialStoreTestSuite struct {
	suite.Suite
	composite *compositeCredentialStore
	fileStore *credentialFileBasedStore
	dbStore   *credentialStoreInterfaceMock
	ctx       context.Context
}

func TestCompositeCredentialStoreTestSuite(t *testing.T) {
	suite.Run(t, new(CompositeCredentialStoreTestSuite))
}

func (s *CompositeCredentialStoreTestSuite) SetupTest() {
	fileStore := newCredentialFileBasedStore()
	s.Require().NoError(fileStore.GenericFileBasedStore.ClearByType())
	s.fileStore = fileStore
	s.dbStore = newStatefulCredentialStore(s.T())
	s.composite = newCompositeCredentialStore(fileStore, s.dbStore)
	s.ctx = context.Background()
}

func (s *CompositeCredentialStoreTestSuite) seedFile(id, handle, vct string) {
	storer := &credentialStorer{store: s.fileStore}
	s.Require().NoError(storer.Create(id, &CredentialConfigurationDTO{
		ID: id, Handle: handle, VCT: vct, Format: DefaultCredentialFormat,
	}))
}

func (s *CompositeCredentialStoreTestSuite) seedDB(id, handle, vct string) {
	s.Require().NoError(s.dbStore.CreateCredentialConfiguration(s.ctx, CredentialConfigurationDTO{
		ID: id, Handle: handle, VCT: vct, Format: DefaultCredentialFormat,
	}))
}

func (s *CompositeCredentialStoreTestSuite) TestGetByIDFallsBackToFileStore() {
	s.seedFile("file-1", "file-handle", "v")

	got, err := s.composite.GetCredentialConfigurationByID(s.ctx, "file-1")
	s.Require().NoError(err)
	s.Equal("file-handle", got.Handle)
}

func (s *CompositeCredentialStoreTestSuite) TestGetByIDPrefersDBStore() {
	s.seedDB("dup", "db-handle", "v-db")
	s.seedFile("dup", "file-handle", "v-file")

	got, err := s.composite.GetCredentialConfigurationByID(s.ctx, "dup")
	s.Require().NoError(err)
	s.Equal("db-handle", got.Handle)
}

func (s *CompositeCredentialStoreTestSuite) TestGetByIDNotFound() {
	_, err := s.composite.GetCredentialConfigurationByID(s.ctx, "missing")
	s.ErrorIs(err, ErrNotFound)
}

func (s *CompositeCredentialStoreTestSuite) TestGetByHandleFallsBackToFileStore() {
	s.seedFile("file-1", "file-handle", "v")

	got, err := s.composite.GetCredentialConfigurationByHandle(s.ctx, "file-handle")
	s.Require().NoError(err)
	s.Equal("file-1", got.ID)
}

func (s *CompositeCredentialStoreTestSuite) TestListSummariesMergesAndDeduplicates() {
	s.seedDB("db-1", "db-handle", "v")
	s.seedFile("file-1", "file-handle", "v")
	s.seedDB("dup", "db-dup", "v-db")
	s.seedFile("dup", "file-dup", "v-file")

	summaries, err := s.composite.ListCredentialConfigurationSummaries(s.ctx)
	s.Require().NoError(err)
	s.Len(summaries, 3)
}

func (s *CompositeCredentialStoreTestSuite) TestListMergesAndDeduplicates() {
	s.seedDB("db-1", "db-handle", "v")
	s.seedFile("file-1", "file-handle", "v")
	s.seedDB("dup", "db-dup", "v-db")
	s.seedFile("dup", "file-dup", "v-file")

	configs, err := s.composite.ListCredentialConfigurations(s.ctx)
	s.Require().NoError(err)
	s.Len(configs, 3)

	byID := make(map[string]CredentialConfigurationDTO, len(configs))
	for _, c := range configs {
		byID[c.ID] = c
	}
	// DB wins on duplicate ID.
	s.Equal("db-dup", byID["dup"].Handle)
}

func (s *CompositeCredentialStoreTestSuite) TestCreateRoutesToDBStore() {
	err := s.composite.CreateCredentialConfiguration(
		s.ctx, CredentialConfigurationDTO{ID: "new", Handle: "h", VCT: "v"})
	s.Require().NoError(err)

	_, err = s.dbStore.GetCredentialConfigurationByID(s.ctx, "new")
	s.Require().NoError(err)
}

func (s *CompositeCredentialStoreTestSuite) TestUpdateRoutesToDBStore() {
	s.seedDB("db-1", "h", "v")

	err := s.composite.UpdateCredentialConfiguration(s.ctx, CredentialConfigurationDTO{
		ID: "db-1", Handle: "h", VCT: "v2",
	})
	s.Require().NoError(err)

	got, _ := s.dbStore.GetCredentialConfigurationByID(s.ctx, "db-1")
	s.Equal("v2", got.VCT)
}

func (s *CompositeCredentialStoreTestSuite) TestUpdateImmutableConfiguration() {
	s.seedFile("file-1", "file-handle", "v")

	err := s.composite.UpdateCredentialConfiguration(s.ctx, CredentialConfigurationDTO{
		ID: "file-1", Handle: "x", VCT: "v",
	})
	s.ErrorIs(err, ErrConfigurationIsImmutable)
}

func (s *CompositeCredentialStoreTestSuite) TestDeleteRoutesToDBStore() {
	s.seedDB("db-1", "h", "v")

	err := s.composite.DeleteCredentialConfiguration(s.ctx, "db-1")
	s.Require().NoError(err)

	_, err = s.dbStore.GetCredentialConfigurationByID(s.ctx, "db-1")
	s.ErrorIs(err, ErrNotFound)
}

func (s *CompositeCredentialStoreTestSuite) TestDeleteImmutableConfiguration() {
	s.seedFile("file-1", "file-handle", "v")

	err := s.composite.DeleteCredentialConfiguration(s.ctx, "file-1")
	s.ErrorIs(err, ErrConfigurationIsImmutable)
}

func (s *CompositeCredentialStoreTestSuite) TestUpdatePropagatesFileStoreError() {
	// A corrupted file-store entry makes existsInFileStore return a non-NotFound error.
	s.Require().NoError(s.fileStore.GenericFileBasedStore.Create("corrupt", "not-a-dto"))

	err := s.composite.UpdateCredentialConfiguration(s.ctx, CredentialConfigurationDTO{
		ID: "corrupt", Handle: "x", VCT: "v",
	})
	s.ErrorIs(err, ErrConfigurationDataCorrupted)
}

func (s *CompositeCredentialStoreTestSuite) TestDeletePropagatesFileStoreError() {
	s.Require().NoError(s.fileStore.GenericFileBasedStore.Create("corrupt", "not-a-dto"))

	err := s.composite.DeleteCredentialConfiguration(s.ctx, "corrupt")
	s.ErrorIs(err, ErrConfigurationDataCorrupted)
}

func (s *CompositeCredentialStoreTestSuite) TestIsDeclarativePropagatesFileStoreError() {
	s.Require().NoError(s.fileStore.GenericFileBasedStore.Create("corrupt", "not-a-dto"))

	_, err := s.composite.IsCredentialConfigurationDeclarative(s.ctx, "corrupt")
	s.ErrorIs(err, ErrConfigurationDataCorrupted)
}

func (s *CompositeCredentialStoreTestSuite) TestListDBStoreError() {
	errStore := newCredentialStoreInterfaceMock(s.T())
	errStore.EXPECT().ListCredentialConfigurations(mock.Anything).
		Return(nil, errors.New("db boom")).Maybe()
	errStore.EXPECT().ListCredentialConfigurationSummaries(mock.Anything).
		Return(nil, errors.New("db boom")).Maybe()
	composite := newCompositeCredentialStore(s.fileStore, errStore)

	_, err := composite.ListCredentialConfigurations(s.ctx)
	s.Error(err)

	_, err = composite.ListCredentialConfigurationSummaries(s.ctx)
	s.Error(err)
}

func (s *CompositeCredentialStoreTestSuite) TestListFileStoreError() {
	// Make the file store's List fail by seeding a DB entry and using a mock file store
	// that returns an error from its list methods.
	errFileStore := newCredentialStoreInterfaceMock(s.T())
	errFileStore.EXPECT().ListCredentialConfigurations(mock.Anything).
		Return(nil, errors.New("file boom")).Maybe()
	errFileStore.EXPECT().ListCredentialConfigurationSummaries(mock.Anything).
		Return(nil, errors.New("file boom")).Maybe()
	composite := newCompositeCredentialStore(errFileStore, s.dbStore)

	_, err := composite.ListCredentialConfigurations(s.ctx)
	s.Error(err)

	_, err = composite.ListCredentialConfigurationSummaries(s.ctx)
	s.Error(err)
}

func (s *CompositeCredentialStoreTestSuite) TestIsDeclarative() {
	s.seedFile("file-1", "file-handle", "v")

	isDeclarative, err := s.composite.IsCredentialConfigurationDeclarative(s.ctx, "file-1")
	s.Require().NoError(err)
	s.True(isDeclarative)

	isDeclarative, err = s.composite.IsCredentialConfigurationDeclarative(s.ctx, "missing")
	s.Require().NoError(err)
	s.False(isDeclarative)
}

func (s *CompositeCredentialStoreTestSuite) TestListSummariesByOUIDMergesAndDeduplicates() {
	storer := &credentialStorer{store: s.fileStore}
	for _, dto := range []CredentialConfigurationDTO{
		{ID: "db-1", Handle: "db-1", VCT: "v", OUID: "ou-1"},
		{ID: "dup", Handle: "db-dup", VCT: "v", OUID: "ou-1"},
		{ID: "db-2", Handle: "db-2", VCT: "v", OUID: "ou-2"},
	} {
		s.Require().NoError(s.dbStore.CreateCredentialConfiguration(s.ctx, dto))
	}
	for _, dto := range []CredentialConfigurationDTO{
		{ID: "file-1", Handle: "file-1", VCT: "v", OUID: "ou-1"},
		{ID: "dup", Handle: "file-dup", VCT: "v", OUID: "ou-1"},
		{ID: "file-2", Handle: "file-2", VCT: "v", OUID: "ou-2"},
	} {
		s.Require().NoError(storer.Create(dto.ID, &dto))
	}

	summaries, err := s.composite.ListCredentialConfigurationSummariesByOUID(s.ctx, "ou-1")
	s.Require().NoError(err)
	s.Require().Len(summaries, 3)
	byID := make(map[string]CredentialConfigurationList, len(summaries))
	for _, summary := range summaries {
		byID[summary.ID] = summary
	}
	s.Contains(byID, "db-1")
	s.Contains(byID, "file-1")
	s.Equal("db-dup", byID["dup"].Handle)
}

func (s *CompositeCredentialStoreTestSuite) TestListSummariesByOUIDStoreErrors() {
	dbStore := newCredentialStoreInterfaceMock(s.T())
	dbStore.EXPECT().ListCredentialConfigurationSummariesByOUID(mock.Anything, "ou-1").
		Return(nil, errors.New("db boom"))
	_, err := newCompositeCredentialStore(s.fileStore, dbStore).
		ListCredentialConfigurationSummariesByOUID(s.ctx, "ou-1")
	s.Error(err)

	fileStore := newCredentialStoreInterfaceMock(s.T())
	fileStore.EXPECT().ListCredentialConfigurationSummariesByOUID(mock.Anything, "ou-1").
		Return(nil, errors.New("file boom"))
	_, err = newCompositeCredentialStore(fileStore, s.dbStore).
		ListCredentialConfigurationSummariesByOUID(s.ctx, "ou-1")
	s.Error(err)
}
