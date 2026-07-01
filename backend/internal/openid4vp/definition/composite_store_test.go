/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package definition

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
