// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package credential

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
)

// mutatedValue is written to a returned DTO to prove the store hands out a copy.
const mutatedValue = "mutated"

type CredentialFileBasedStoreTestSuite struct {
	suite.Suite
	store  *credentialFileBasedStore
	storer *credentialStorer
	ctx    context.Context
}

func TestCredentialFileBasedStoreTestSuite(t *testing.T) {
	suite.Run(t, new(CredentialFileBasedStoreTestSuite))
}

func (s *CredentialFileBasedStoreTestSuite) SetupTest() {
	fileStore := newCredentialFileBasedStore()
	// The file store is backed by the singleton entity store; clear this key type
	// so each test starts from a clean slate.
	s.Require().NoError(fileStore.GenericFileBasedStore.ClearByType())
	s.store = fileStore
	s.storer = &credentialStorer{store: fileStore}
	s.ctx = context.Background()
}

func (s *CredentialFileBasedStoreTestSuite) seed(id, handle, vct string) {
	err := s.storer.Create(id, &CredentialConfigurationDTO{
		ID:     id,
		Handle: handle,
		VCT:    vct,
		Format: DefaultCredentialFormat,
	})
	s.Require().NoError(err)
}

func (s *CredentialFileBasedStoreTestSuite) TestStorerCreateAndGetByID() {
	s.seed("cfg-1", "eudi-pid", "urn:eudi:pid:1")

	got, err := s.store.GetCredentialConfigurationByID(s.ctx, "cfg-1")
	s.Require().NoError(err)
	s.Equal("eudi-pid", got.Handle)
	s.Equal("urn:eudi:pid:1", got.VCT)
}

func (s *CredentialFileBasedStoreTestSuite) TestStorerCreateBackfillsID() {
	err := s.storer.Create("cfg-backfill", &CredentialConfigurationDTO{Handle: "h", VCT: "v"})
	s.Require().NoError(err)

	got, err := s.store.GetCredentialConfigurationByID(s.ctx, "cfg-backfill")
	s.Require().NoError(err)
	s.Equal("cfg-backfill", got.ID)
}

func (s *CredentialFileBasedStoreTestSuite) TestStorerCreateRejectsWrongType() {
	err := s.storer.Create("bad", "not-a-dto")
	s.ErrorIs(err, ErrConfigurationDataCorrupted)
}

func (s *CredentialFileBasedStoreTestSuite) TestCreateCredentialConfiguration() {
	err := s.store.CreateCredentialConfiguration(s.ctx, CredentialConfigurationDTO{
		ID: "cfg-direct", Handle: "h", VCT: "v", Format: DefaultCredentialFormat,
	})
	s.Require().NoError(err)

	got, err := s.store.GetCredentialConfigurationByID(s.ctx, "cfg-direct")
	s.Require().NoError(err)
	s.Equal("h", got.Handle)
}

func (s *CredentialFileBasedStoreTestSuite) TestGetByIDNotFound() {
	_, err := s.store.GetCredentialConfigurationByID(s.ctx, "missing")
	s.ErrorIs(err, ErrNotFound)
}

func (s *CredentialFileBasedStoreTestSuite) TestGetByHandle() {
	s.seed("cfg-1", "eudi-pid", "urn:eudi:pid:1")

	got, err := s.store.GetCredentialConfigurationByHandle(s.ctx, "eudi-pid")
	s.Require().NoError(err)
	s.Equal("cfg-1", got.ID)

	_, err = s.store.GetCredentialConfigurationByHandle(s.ctx, "nope")
	s.ErrorIs(err, ErrNotFound)
}

func (s *CredentialFileBasedStoreTestSuite) TestListSummaries() {
	s.seed("cfg-1", "h1", "v1")
	s.seed("cfg-2", "h2", "v2")

	summaries, err := s.store.ListCredentialConfigurationSummaries(s.ctx)
	s.Require().NoError(err)
	s.Len(summaries, 2)
}

func (s *CredentialFileBasedStoreTestSuite) TestList() {
	s.seed("cfg-1", "h1", "v1")
	s.seed("cfg-2", "h2", "v2")

	configs, err := s.store.ListCredentialConfigurations(s.ctx)
	s.Require().NoError(err)
	s.Len(configs, 2)
}

func (s *CredentialFileBasedStoreTestSuite) TestGetByIDCorruptedType() {
	// Store a value of the wrong type directly so the type assertion fails.
	s.Require().NoError(s.store.GenericFileBasedStore.Create("corrupt", "not-a-dto"))

	_, err := s.store.GetCredentialConfigurationByID(s.ctx, "corrupt")
	s.ErrorIs(err, ErrConfigurationDataCorrupted)
}

func (s *CredentialFileBasedStoreTestSuite) TestListSkipsCorruptedEntries() {
	s.seed("cfg-1", "h1", "v1")
	s.Require().NoError(s.store.GenericFileBasedStore.Create("corrupt", "not-a-dto"))

	configs, err := s.store.ListCredentialConfigurations(s.ctx)
	s.Require().NoError(err)
	s.Len(configs, 1)

	summaries, err := s.store.ListCredentialConfigurationSummaries(s.ctx)
	s.Require().NoError(err)
	s.Len(summaries, 1)
}

func (s *CredentialFileBasedStoreTestSuite) TestUpdateNotSupported() {
	s.seed("cfg-1", "h1", "v1")
	err := s.store.UpdateCredentialConfiguration(
		s.ctx, CredentialConfigurationDTO{ID: "cfg-1", Handle: "h1", VCT: "v2"})
	s.ErrorIs(err, ErrConfigurationIsImmutable)
}

func (s *CredentialFileBasedStoreTestSuite) TestDeleteNotSupported() {
	s.seed("cfg-1", "h1", "v1")
	err := s.store.DeleteCredentialConfiguration(s.ctx, "cfg-1")
	s.ErrorIs(err, ErrConfigurationIsImmutable)
}

func (s *CredentialFileBasedStoreTestSuite) TestIsDeclarative() {
	s.seed("cfg-1", "h1", "v1")

	isDeclarative, err := s.store.IsCredentialConfigurationDeclarative(s.ctx, "cfg-1")
	s.Require().NoError(err)
	s.True(isDeclarative)

	isDeclarative, err = s.store.IsCredentialConfigurationDeclarative(s.ctx, "missing")
	s.Require().NoError(err)
	s.False(isDeclarative)
}

func (s *CredentialFileBasedStoreTestSuite) TestGetReturnsIsolatedCopy() {
	s.seed("cfg-copy", "eudi-pid", "urn:eudi:pid:1")

	// The service stamps OUHandle onto every configuration it reads, so without a copy
	// each read would write into the shared declarative entry.
	first, err := s.store.GetCredentialConfigurationByID(s.ctx, "cfg-copy")
	s.Require().NoError(err)
	first.OUHandle = mutatedValue
	first.Handle = mutatedValue

	second, err := s.store.GetCredentialConfigurationByID(s.ctx, "cfg-copy")
	s.Require().NoError(err)
	s.Empty(second.OUHandle, "a caller must not be able to mutate the declarative store")
	s.Equal("eudi-pid", second.Handle)
}

func (s *CredentialFileBasedStoreTestSuite) TestGetByHandleReturnsIsolatedCopy() {
	s.seed("cfg-copy-h", "eudi-pid", "urn:eudi:pid:1")

	first, err := s.store.GetCredentialConfigurationByHandle(s.ctx, "eudi-pid")
	s.Require().NoError(err)
	first.VCT = mutatedValue

	second, err := s.store.GetCredentialConfigurationByHandle(s.ctx, "eudi-pid")
	s.Require().NoError(err)
	s.Equal("urn:eudi:pid:1", second.VCT)
}
