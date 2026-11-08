// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package presentation

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type CompositeDefinitionStoreTestSuite struct {
	suite.Suite
	composite *compositeDefinitionStore
	fileStore *definitionFileBasedStore
	dbStore   *definitionStoreInterfaceMock
	ctx       context.Context
}

func TestCompositeDefinitionStoreTestSuite(t *testing.T) {
	suite.Run(t, new(CompositeDefinitionStoreTestSuite))
}

func (s *CompositeDefinitionStoreTestSuite) SetupTest() {
	fileStore := newDefinitionFileBasedStore()
	s.Require().NoError(fileStore.GenericFileBasedStore.ClearByType())
	s.fileStore = fileStore
	s.dbStore = newStatefulDefinitionStore(s.T())
	s.composite = newCompositeDefinitionStore(fileStore, s.dbStore)
	s.ctx = context.Background()
}

func (s *CompositeDefinitionStoreTestSuite) seedFile(id, handle, vct string) {
	storer := &definitionStorer{store: s.fileStore}
	s.Require().NoError(storer.Create(id, &PresentationDefinitionDTO{
		ID: id, Handle: handle, VCT: vct, Format: DefaultCredentialFormat,
	}))
}

func (s *CompositeDefinitionStoreTestSuite) seedDB(id, handle, vct string) {
	s.Require().NoError(s.dbStore.CreatePresentationDefinition(s.ctx, PresentationDefinitionDTO{
		ID: id, Handle: handle, VCT: vct, Format: DefaultCredentialFormat,
	}))
}

func (s *CompositeDefinitionStoreTestSuite) TestGetByIDFallsBackToFileStore() {
	s.seedFile("file-1", "file-handle", "v")

	got, err := s.composite.GetPresentationDefinitionByID(s.ctx, "file-1")
	s.Require().NoError(err)
	s.Equal("file-handle", got.Handle)
}

func (s *CompositeDefinitionStoreTestSuite) TestGetByIDPrefersDBStore() {
	s.seedDB("dup", "db-handle", "v-db")
	s.seedFile("dup", "file-handle", "v-file")

	got, err := s.composite.GetPresentationDefinitionByID(s.ctx, "dup")
	s.Require().NoError(err)
	s.Equal("db-handle", got.Handle)
}

func (s *CompositeDefinitionStoreTestSuite) TestGetByIDNotFound() {
	_, err := s.composite.GetPresentationDefinitionByID(s.ctx, "missing")
	s.ErrorIs(err, ErrNotFound)
}

func (s *CompositeDefinitionStoreTestSuite) TestGetByHandleFallsBackToFileStore() {
	s.seedFile("file-1", "file-handle", "v")

	got, err := s.composite.GetPresentationDefinitionByHandle(s.ctx, "file-handle")
	s.Require().NoError(err)
	s.Equal("file-1", got.ID)
}

func (s *CompositeDefinitionStoreTestSuite) TestListMergesAndDeduplicates() {
	s.seedDB("db-1", "db-handle", "v")
	s.seedFile("file-1", "file-handle", "v")
	s.seedDB("dup", "db-dup", "v-db")
	s.seedFile("dup", "file-dup", "v-file")

	defs, err := s.composite.ListPresentationDefinitions(s.ctx)
	s.Require().NoError(err)
	s.Len(defs, 3)

	byID := make(map[string]PresentationDefinitionDTO, len(defs))
	for _, d := range defs {
		byID[d.ID] = d
	}
	// DB wins on duplicate ID.
	s.Equal("db-dup", byID["dup"].Handle)
}

func (s *CompositeDefinitionStoreTestSuite) TestCreateRoutesToDBStore() {
	err := s.composite.CreatePresentationDefinition(s.ctx, PresentationDefinitionDTO{ID: "new", Handle: "h", VCT: "v"})
	s.Require().NoError(err)

	_, err = s.dbStore.GetPresentationDefinitionByID(s.ctx, "new")
	s.Require().NoError(err)
}

func (s *CompositeDefinitionStoreTestSuite) TestUpdateRoutesToDBStore() {
	s.seedDB("db-1", "h", "v")

	err := s.composite.UpdatePresentationDefinition(s.ctx, PresentationDefinitionDTO{
		ID: "db-1", Handle: "h", VCT: "v2",
	})
	s.Require().NoError(err)

	got, _ := s.dbStore.GetPresentationDefinitionByID(s.ctx, "db-1")
	s.Equal("v2", got.VCT)
}

func (s *CompositeDefinitionStoreTestSuite) TestUpdateImmutableDefinition() {
	s.seedFile("file-1", "file-handle", "v")

	err := s.composite.UpdatePresentationDefinition(s.ctx, PresentationDefinitionDTO{
		ID: "file-1", Handle: "x", VCT: "v",
	})
	s.ErrorIs(err, ErrDefinitionIsImmutable)
}

func (s *CompositeDefinitionStoreTestSuite) TestDeleteRoutesToDBStore() {
	s.seedDB("db-1", "h", "v")

	err := s.composite.DeletePresentationDefinition(s.ctx, "db-1")
	s.Require().NoError(err)

	_, err = s.dbStore.GetPresentationDefinitionByID(s.ctx, "db-1")
	s.ErrorIs(err, ErrNotFound)
}

func (s *CompositeDefinitionStoreTestSuite) TestDeleteImmutableDefinition() {
	s.seedFile("file-1", "file-handle", "v")

	err := s.composite.DeletePresentationDefinition(s.ctx, "file-1")
	s.ErrorIs(err, ErrDefinitionIsImmutable)
}

func (s *CompositeDefinitionStoreTestSuite) TestListSummariesMergesAndDeduplicates() {
	s.seedDB("db-1", "db-handle", "v")
	s.seedFile("file-1", "file-handle", "v")
	s.seedDB("dup", "db-dup", "v-db")
	s.seedFile("dup", "file-dup", "v-file")

	summaries, err := s.composite.ListPresentationDefinitionSummaries(s.ctx)
	s.Require().NoError(err)
	s.Len(summaries, 3)

	byID := make(map[string]PresentationDefinitionList, len(summaries))
	for _, sm := range summaries {
		byID[sm.ID] = sm
	}
	// DB wins on duplicate ID.
	s.Equal("db-dup", byID["dup"].Handle)
}

func (s *CompositeDefinitionStoreTestSuite) TestListDBStoreError() {
	dbStore := newDefinitionStoreInterfaceMock(s.T())
	dbStore.EXPECT().ListPresentationDefinitions(mock.Anything).Return(nil, errors.New("db boom"))
	composite := newCompositeDefinitionStore(s.fileStore, dbStore)

	_, err := composite.ListPresentationDefinitions(s.ctx)
	s.Error(err)
}

func (s *CompositeDefinitionStoreTestSuite) TestListFileStoreError() {
	fileStore := newDefinitionStoreInterfaceMock(s.T())
	fileStore.EXPECT().ListPresentationDefinitions(mock.Anything).Return(nil, errors.New("file boom"))
	composite := newCompositeDefinitionStore(fileStore, s.dbStore)

	_, err := composite.ListPresentationDefinitions(s.ctx)
	s.Error(err)
}

func (s *CompositeDefinitionStoreTestSuite) TestListSummariesDBStoreError() {
	dbStore := newDefinitionStoreInterfaceMock(s.T())
	dbStore.EXPECT().ListPresentationDefinitionSummaries(mock.Anything).Return(nil, errors.New("db boom"))
	composite := newCompositeDefinitionStore(s.fileStore, dbStore)

	_, err := composite.ListPresentationDefinitionSummaries(s.ctx)
	s.Error(err)
}

func (s *CompositeDefinitionStoreTestSuite) TestListSummariesFileStoreError() {
	fileStore := newDefinitionStoreInterfaceMock(s.T())
	fileStore.EXPECT().ListPresentationDefinitionSummaries(mock.Anything).Return(nil, errors.New("file boom"))
	composite := newCompositeDefinitionStore(fileStore, s.dbStore)

	_, err := composite.ListPresentationDefinitionSummaries(s.ctx)
	s.Error(err)
}

func (s *CompositeDefinitionStoreTestSuite) TestExistsInFileStorePropagatesError() {
	fileStore := newDefinitionStoreInterfaceMock(s.T())
	fileStore.EXPECT().GetPresentationDefinitionByID(mock.Anything, mock.Anything).
		Return(nil, errors.New("file boom"))
	composite := newCompositeDefinitionStore(fileStore, s.dbStore)

	// existsInFileStore propagates non-NotFound errors to the caller.
	_, err := composite.IsPresentationDefinitionDeclarative(s.ctx, "id-1")
	s.Error(err)
}

func (s *CompositeDefinitionStoreTestSuite) TestIsDeclarative() {
	s.seedFile("file-1", "file-handle", "v")
	s.seedDB("db-1", "db-handle", "v")

	isDeclarative, err := s.composite.IsPresentationDefinitionDeclarative(s.ctx, "file-1")
	s.Require().NoError(err)
	s.True(isDeclarative)

	isDeclarative, err = s.composite.IsPresentationDefinitionDeclarative(s.ctx, "db-1")
	s.Require().NoError(err)
	s.False(isDeclarative)

	isDeclarative, err = s.composite.IsPresentationDefinitionDeclarative(s.ctx, "missing")
	s.Require().NoError(err)
	s.False(isDeclarative)
}

func (s *CompositeDefinitionStoreTestSuite) TestListSummariesByOUIDMergesAndDeduplicates() {
	storer := &definitionStorer{store: s.fileStore}
	for _, dto := range []PresentationDefinitionDTO{
		{ID: "db-1", Handle: "db-1", VCT: "v", OUID: "ou-1"},
		{ID: "dup", Handle: "db-dup", VCT: "v", OUID: "ou-1"},
		{ID: "db-2", Handle: "db-2", VCT: "v", OUID: "ou-2"},
	} {
		s.Require().NoError(s.dbStore.CreatePresentationDefinition(s.ctx, dto))
	}
	for _, dto := range []PresentationDefinitionDTO{
		{ID: "file-1", Handle: "file-1", VCT: "v", OUID: "ou-1"},
		{ID: "dup", Handle: "file-dup", VCT: "v", OUID: "ou-1"},
		{ID: "file-2", Handle: "file-2", VCT: "v", OUID: "ou-2"},
	} {
		s.Require().NoError(storer.Create(dto.ID, &dto))
	}

	summaries, err := s.composite.ListPresentationDefinitionSummariesByOUID(s.ctx, "ou-1")
	s.Require().NoError(err)
	s.Require().Len(summaries, 3)
	byID := make(map[string]PresentationDefinitionList, len(summaries))
	for _, summary := range summaries {
		byID[summary.ID] = summary
	}
	s.Contains(byID, "db-1")
	s.Contains(byID, "file-1")
	s.Equal("db-dup", byID["dup"].Handle)
}

func (s *CompositeDefinitionStoreTestSuite) TestListSummariesByOUIDStoreErrors() {
	dbStore := newDefinitionStoreInterfaceMock(s.T())
	dbStore.EXPECT().ListPresentationDefinitionSummariesByOUID(mock.Anything, "ou-1").
		Return(nil, errors.New("db boom"))
	_, err := newCompositeDefinitionStore(s.fileStore, dbStore).
		ListPresentationDefinitionSummariesByOUID(s.ctx, "ou-1")
	s.Error(err)

	fileStore := newDefinitionStoreInterfaceMock(s.T())
	fileStore.EXPECT().ListPresentationDefinitionSummariesByOUID(mock.Anything, "ou-1").
		Return(nil, errors.New("file boom"))
	_, err = newCompositeDefinitionStore(fileStore, s.dbStore).
		ListPresentationDefinitionSummariesByOUID(s.ctx, "ou-1")
	s.Error(err)
}

// A project's definitions merge across both stores, and a failing store fails the list.
func (s *CompositeDefinitionStoreTestSuite) TestListSummariesByProject() {
	s.Require().NoError(s.dbStore.CreatePresentationDefinition(s.ctx,
		PresentationDefinitionDTO{ID: "db-1", Handle: "db-1", VCT: "v", OUID: "ou-1", ProjectID: "proj-1"}))
	fileDTO := PresentationDefinitionDTO{ID: "file-1", Handle: "file-1", VCT: "v", OUID: "ou-1", ProjectID: "proj-1"}
	s.Require().NoError((&definitionStorer{store: s.fileStore}).Create(fileDTO.ID, &fileDTO))

	summaries, err := s.composite.ListPresentationDefinitionSummariesByProject(s.ctx, "proj-1")
	s.Require().NoError(err)
	s.Len(summaries, 2)

	failing := newDefinitionStoreInterfaceMock(s.T())
	failing.EXPECT().ListPresentationDefinitionSummariesByProject(mock.Anything, "proj-1").
		Return(nil, errors.New("boom")).Twice()
	_, err = newCompositeDefinitionStore(s.fileStore, failing).
		ListPresentationDefinitionSummariesByProject(s.ctx, "proj-1")
	s.Error(err)
	_, err = newCompositeDefinitionStore(failing, s.dbStore).
		ListPresentationDefinitionSummariesByProject(s.ctx, "proj-1")
	s.Error(err)
}

// A project's definitions are counted across both stores, and a failing store fails the count.
func (s *CompositeDefinitionStoreTestSuite) TestCountByProject() {
	s.Require().NoError(s.dbStore.CreatePresentationDefinition(s.ctx,
		PresentationDefinitionDTO{ID: "db-1", Handle: "db-1", VCT: "v", OUID: "ou-1", ProjectID: "proj-1"}))
	fileDTO := PresentationDefinitionDTO{ID: "file-1", Handle: "file-1", VCT: "v", OUID: "ou-1", ProjectID: "proj-1"}
	s.Require().NoError((&definitionStorer{store: s.fileStore}).Create(fileDTO.ID, &fileDTO))

	count, err := s.composite.CountPresentationDefinitionsByProject(s.ctx, "proj-1")
	s.Require().NoError(err)
	s.Equal(2, count)

	failing := newDefinitionStoreInterfaceMock(s.T())
	failing.EXPECT().CountPresentationDefinitionsByProject(mock.Anything, "proj-1").
		Return(0, errors.New("boom")).Once()
	_, err = newCompositeDefinitionStore(s.fileStore, failing).CountPresentationDefinitionsByProject(s.ctx, "proj-1")
	s.Error(err)
}
