// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package presentation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
)

// mutatedValue is written to a returned DTO to prove the store hands out a copy.
const mutatedValue = "mutated"

type DefinitionFileBasedStoreTestSuite struct {
	suite.Suite
	store  *definitionFileBasedStore
	storer *definitionStorer
	ctx    context.Context
}

func TestDefinitionFileBasedStoreTestSuite(t *testing.T) {
	suite.Run(t, new(DefinitionFileBasedStoreTestSuite))
}

func (s *DefinitionFileBasedStoreTestSuite) SetupTest() {
	fileStore := newDefinitionFileBasedStore()
	// The file store is backed by the singleton entity store; clear this key type
	// so each test starts from a clean slate.
	s.Require().NoError(fileStore.GenericFileBasedStore.ClearByType())
	s.store = fileStore
	s.storer = &definitionStorer{store: fileStore}
	s.ctx = context.Background()
}

func (s *DefinitionFileBasedStoreTestSuite) seed(id, handle, vct string) {
	err := s.storer.Create(id, &PresentationDefinitionDTO{
		ID:     id,
		Handle: handle,
		VCT:    vct,
		Format: DefaultCredentialFormat,
	})
	s.Require().NoError(err)
}

func (s *DefinitionFileBasedStoreTestSuite) TestStorerCreateAndGetByID() {
	s.seed("def-1", "eudi-pid", "urn:eudi:pid:1")

	got, err := s.store.GetPresentationDefinitionByID(s.ctx, "def-1")
	s.Require().NoError(err)
	s.Equal("eudi-pid", got.Handle)
	s.Equal("urn:eudi:pid:1", got.VCT)
}

func (s *DefinitionFileBasedStoreTestSuite) TestStorerCreateBackfillsID() {
	err := s.storer.Create("def-backfill", &PresentationDefinitionDTO{Handle: "h", VCT: "v"})
	s.Require().NoError(err)

	got, err := s.store.GetPresentationDefinitionByID(s.ctx, "def-backfill")
	s.Require().NoError(err)
	s.Equal("def-backfill", got.ID)
}

func (s *DefinitionFileBasedStoreTestSuite) TestStorerCreateRejectsWrongType() {
	err := s.storer.Create("bad", "not-a-dto")
	s.ErrorIs(err, ErrDefinitionDataCorrupted)
}

func (s *DefinitionFileBasedStoreTestSuite) TestGetByIDNotFound() {
	_, err := s.store.GetPresentationDefinitionByID(s.ctx, "missing")
	s.ErrorIs(err, ErrNotFound)
}

func (s *DefinitionFileBasedStoreTestSuite) TestGetByHandle() {
	s.seed("def-1", "eudi-pid", "urn:eudi:pid:1")

	got, err := s.store.GetPresentationDefinitionByHandle(s.ctx, "eudi-pid")
	s.Require().NoError(err)
	s.Equal("def-1", got.ID)

	_, err = s.store.GetPresentationDefinitionByHandle(s.ctx, "nope")
	s.ErrorIs(err, ErrNotFound)
}

func (s *DefinitionFileBasedStoreTestSuite) TestList() {
	s.seed("def-1", "h1", "v1")
	s.seed("def-2", "h2", "v2")

	defs, err := s.store.ListPresentationDefinitions(s.ctx)
	s.Require().NoError(err)
	s.Len(defs, 2)
}

func (s *DefinitionFileBasedStoreTestSuite) TestCreatePresentationDefinition() {
	err := s.store.CreatePresentationDefinition(s.ctx, PresentationDefinitionDTO{
		ID: "def-1", Handle: "eudi-pid", VCT: "v", Format: DefaultCredentialFormat,
	})
	s.Require().NoError(err)

	got, err := s.store.GetPresentationDefinitionByID(s.ctx, "def-1")
	s.Require().NoError(err)
	s.Equal("eudi-pid", got.Handle)
}

func (s *DefinitionFileBasedStoreTestSuite) TestListSummaries() {
	s.seed("def-1", "h1", "v1")
	s.seed("def-2", "h2", "v2")

	summaries, err := s.store.ListPresentationDefinitionSummaries(s.ctx)
	s.Require().NoError(err)
	s.Len(summaries, 2)

	handles := []string{summaries[0].Handle, summaries[1].Handle}
	s.ElementsMatch([]string{"h1", "h2"}, handles)
}

func (s *DefinitionFileBasedStoreTestSuite) TestIsDeclarative() {
	s.seed("def-1", "h1", "v1")

	isDeclarative, err := s.store.IsPresentationDefinitionDeclarative(s.ctx, "def-1")
	s.Require().NoError(err)
	s.True(isDeclarative)

	isDeclarative, err = s.store.IsPresentationDefinitionDeclarative(s.ctx, "missing")
	s.Require().NoError(err)
	s.False(isDeclarative)
}

func (s *DefinitionFileBasedStoreTestSuite) TestUpdateNotSupported() {
	s.seed("def-1", "h1", "v1")
	err := s.store.UpdatePresentationDefinition(s.ctx, PresentationDefinitionDTO{ID: "def-1", Handle: "h1", VCT: "v2"})
	s.Error(err)
}

func (s *DefinitionFileBasedStoreTestSuite) TestDeleteNotSupported() {
	s.seed("def-1", "h1", "v1")
	err := s.store.DeletePresentationDefinition(s.ctx, "def-1")
	s.Error(err)
}

func (s *DefinitionFileBasedStoreTestSuite) TestGetReturnsIsolatedCopy() {
	s.seed("def-copy", "eudi-pid", "urn:eudi:pid:1")

	// The service stamps OUHandle onto every definition it reads, so without a copy each
	// read would write into the shared declarative entry.
	first, err := s.store.GetPresentationDefinitionByID(s.ctx, "def-copy")
	s.Require().NoError(err)
	first.OUHandle = mutatedValue
	first.Handle = mutatedValue

	second, err := s.store.GetPresentationDefinitionByID(s.ctx, "def-copy")
	s.Require().NoError(err)
	s.Empty(second.OUHandle, "a caller must not be able to mutate the declarative store")
	s.Equal("eudi-pid", second.Handle)
}

func (s *DefinitionFileBasedStoreTestSuite) TestGetByHandleReturnsIsolatedCopy() {
	s.seed("def-copy-h", "eudi-pid", "urn:eudi:pid:1")

	first, err := s.store.GetPresentationDefinitionByHandle(s.ctx, "eudi-pid")
	s.Require().NoError(err)
	first.VCT = mutatedValue

	second, err := s.store.GetPresentationDefinitionByHandle(s.ctx, "eudi-pid")
	s.Require().NoError(err)
	s.Equal("urn:eudi:pid:1", second.VCT)
}
