// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package httpconnection

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
)

type HTTPConnectionFileBasedStoreTestSuite struct {
	suite.Suite
	ctx   context.Context
	store *fileBasedStore
}

func TestHTTPConnectionFileBasedStoreTestSuite(t *testing.T) {
	suite.Run(t, new(HTTPConnectionFileBasedStoreTestSuite))
}

func (s *HTTPConnectionFileBasedStoreTestSuite) SetupTest() {
	initTestRuntime(s.T(), false)
	s.ctx = context.Background()
	s.store = &fileBasedStore{
		GenericFileBasedStore: declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeHTTPConnection),
	}
}

func (s *HTTPConnectionFileBasedStoreTestSuite) TestReadsReturnReadOnlyConnections() {
	connection := HTTPConnection{ID: "file-1", Name: "Risk", OUID: "ou-1", URL: testURL, TimeoutMS: defaultTimeoutMS,
		AuthProperties: basicAuthProperties(s.T())}
	s.Require().NoError(s.store.Create(connection.ID, &connection))

	got, err := s.store.GetHTTPConnection(s.ctx, "file-1")
	s.Require().NoError(err)
	want := connection
	want.IsReadOnly = true
	s.Equal(&want, got)

	listed, err := s.store.ListHTTPConnections(s.ctx)
	s.Require().NoError(err)
	s.Equal([]HTTPConnection{want}, listed)

	count, err := s.store.CountHTTPConnections(s.ctx)
	s.Require().NoError(err)
	s.Equal(1, count)

	byName, err := s.store.GetHTTPConnectionByName(s.ctx, "Risk")
	s.Require().NoError(err)
	s.Equal(&want, byName)
}

func (s *HTTPConnectionFileBasedStoreTestSuite) TestMissingConnectionIsNil() {
	got, err := s.store.GetHTTPConnection(s.ctx, "missing")
	s.Require().NoError(err)
	s.Nil(got)

	got, err = s.store.GetHTTPConnectionByName(s.ctx, "missing")
	s.Require().NoError(err)
	s.Nil(got)
}

func (s *HTTPConnectionFileBasedStoreTestSuite) TestWritesAreRefused() {
	s.ErrorIs(s.store.CreateHTTPConnection(s.ctx, HTTPConnection{ID: "file-2", Name: "Risk", URL: testURL}),
		errHTTPConnectionIsImmutable)
	got, err := s.store.GetHTTPConnection(s.ctx, "file-2")
	s.Require().NoError(err)
	s.Nil(got, "a refused create must not reach the file store")
	s.ErrorIs(s.store.UpdateHTTPConnection(s.ctx, HTTPConnection{ID: "file-1"}), errHTTPConnectionIsImmutable)
	s.ErrorIs(s.store.DeleteHTTPConnection(s.ctx, "file-1"), errHTTPConnectionIsImmutable)
}
