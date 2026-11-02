// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package httpconnection

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
)

type HTTPConnectionCompositeStoreTestSuite struct {
	suite.Suite
	ctx       context.Context
	fileStore *httpConnectionStoreInterfaceMock
	dbStore   *httpConnectionStoreInterfaceMock
	store     httpConnectionStoreInterface
}

func TestHTTPConnectionCompositeStoreTestSuite(t *testing.T) {
	suite.Run(t, new(HTTPConnectionCompositeStoreTestSuite))
}

func (s *HTTPConnectionCompositeStoreTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.fileStore = newHttpConnectionStoreInterfaceMock(s.T())
	s.dbStore = newHttpConnectionStoreInterfaceMock(s.T())
	s.store = newCompositeStore(s.fileStore, s.dbStore)
}

func (s *HTTPConnectionCompositeStoreTestSuite) declarative() HTTPConnection {
	return HTTPConnection{ID: "file-1", Name: "Declared", URL: testURL, IsReadOnly: true}
}

func (s *HTTPConnectionCompositeStoreTestSuite) TestCreateWritesToTheDatabase() {
	connection := HTTPConnection{ID: "db-1", Name: "Risk", URL: testURL}
	s.fileStore.On("GetHTTPConnection", mock.Anything, "db-1").Return(nil, nil)
	s.fileStore.On("GetHTTPConnectionByName", mock.Anything, "Risk").Return(nil, nil)
	s.dbStore.On("CreateHTTPConnection", mock.Anything, connection).Return(nil).Once()

	s.NoError(s.store.CreateHTTPConnection(s.ctx, connection))
}

// The database's unique index sees only the mutable connections.
func (s *HTTPConnectionCompositeStoreTestSuite) TestNameOfADeclarativeConnectionIsTaken() {
	declared := s.declarative()
	s.fileStore.On("GetHTTPConnectionByName", mock.Anything, "Declared").Return(&declared, nil)
	s.fileStore.On("GetHTTPConnection", mock.Anything, "db-1").Return(nil, nil)

	s.ErrorIs(s.store.CreateHTTPConnection(s.ctx, HTTPConnection{ID: "db-1", Name: "Declared"}), errNameConflict)
	s.ErrorIs(s.store.UpdateHTTPConnection(s.ctx, HTTPConnection{ID: "db-1", Name: "Declared"}), errNameConflict)
	s.dbStore.AssertNotCalled(s.T(), "CreateHTTPConnection", mock.Anything, mock.Anything)
	s.dbStore.AssertNotCalled(s.T(), "UpdateHTTPConnection", mock.Anything, mock.Anything)
}

// A database entry under a declared ID would shadow the declared connection on every read.
func (s *HTTPConnectionCompositeStoreTestSuite) TestCreateRefusesTheIDOfADeclarativeConnection() {
	declared := s.declarative()
	s.fileStore.On("GetHTTPConnection", mock.Anything, "file-1").Return(&declared, nil)

	s.ErrorIs(s.store.CreateHTTPConnection(s.ctx, HTTPConnection{ID: "file-1", Name: "Other"}), errIDConflict)
	s.dbStore.AssertNotCalled(s.T(), "CreateHTTPConnection", mock.Anything, mock.Anything)
}

func (s *HTTPConnectionCompositeStoreTestSuite) TestCreateFailsWhenTheFileStoreFails() {
	failure := errors.New("file store unavailable")
	s.fileStore.On("GetHTTPConnection", mock.Anything, "id-lookup").Return(nil, failure)
	s.fileStore.On("GetHTTPConnection", mock.Anything, "name-lookup").Return(nil, nil)
	s.fileStore.On("GetHTTPConnectionByName", mock.Anything, "Risk").Return(nil, failure)

	s.ErrorIs(s.store.CreateHTTPConnection(s.ctx, HTTPConnection{ID: "id-lookup", Name: "Risk"}), failure)
	s.ErrorIs(s.store.CreateHTTPConnection(s.ctx, HTTPConnection{ID: "name-lookup", Name: "Risk"}), failure)
	s.dbStore.AssertNotCalled(s.T(), "CreateHTTPConnection", mock.Anything, mock.Anything)
}

func (s *HTTPConnectionCompositeStoreTestSuite) TestDeclarativeConnectionsAreImmutable() {
	declared := s.declarative()
	s.fileStore.On("GetHTTPConnection", mock.Anything, "file-1").Return(&declared, nil)

	s.ErrorIs(s.store.UpdateHTTPConnection(s.ctx, HTTPConnection{ID: "file-1", Name: "Other"}),
		errHTTPConnectionIsImmutable)
	s.ErrorIs(s.store.DeleteHTTPConnection(s.ctx, "file-1"), errHTTPConnectionIsImmutable)
}

func (s *HTTPConnectionCompositeStoreTestSuite) TestMutableConnectionsAreWritten() {
	connection := HTTPConnection{ID: "db-1", Name: "Risk", URL: testURL}
	s.fileStore.On("GetHTTPConnection", mock.Anything, "db-1").Return(nil, nil)
	s.fileStore.On("GetHTTPConnectionByName", mock.Anything, "Risk").Return(nil, nil)
	s.dbStore.On("UpdateHTTPConnection", mock.Anything, connection).Return(nil).Once()
	s.dbStore.On("DeleteHTTPConnection", mock.Anything, "db-1").Return(nil).Once()

	s.NoError(s.store.UpdateHTTPConnection(s.ctx, connection))
	s.NoError(s.store.DeleteHTTPConnection(s.ctx, "db-1"))
}

func (s *HTTPConnectionCompositeStoreTestSuite) TestFileStoreFailureRefusesTheWrite() {
	failure := errors.New("file store unavailable")
	s.fileStore.On("GetHTTPConnection", mock.Anything, "db-1").Return(nil, failure)

	s.ErrorIs(s.store.UpdateHTTPConnection(s.ctx, HTTPConnection{ID: "db-1", Name: "Risk"}), failure)
	s.ErrorIs(s.store.DeleteHTTPConnection(s.ctx, "db-1"), failure)
}

func (s *HTTPConnectionCompositeStoreTestSuite) TestGetPrefersTheDatabase() {
	mutable := &HTTPConnection{ID: "db-1", Name: "Risk"}
	declared := s.declarative()
	s.dbStore.On("GetHTTPConnection", mock.Anything, "db-1").Return(mutable, nil)
	s.dbStore.On("GetHTTPConnection", mock.Anything, "file-1").Return(nil, nil)
	s.fileStore.On("GetHTTPConnection", mock.Anything, "file-1").Return(&declared, nil)

	got, err := s.store.GetHTTPConnection(s.ctx, "db-1")
	s.Require().NoError(err)
	s.Equal(mutable, got)
	got, err = s.store.GetHTTPConnection(s.ctx, "file-1")
	s.Require().NoError(err)
	s.Equal(&declared, got)
}

func (s *HTTPConnectionCompositeStoreTestSuite) TestGetByNamePrefersTheDatabase() {
	mutable := &HTTPConnection{ID: "db-1", Name: "Risk"}
	declared := s.declarative()
	s.dbStore.On("GetHTTPConnectionByName", mock.Anything, "Risk").Return(mutable, nil)
	s.dbStore.On("GetHTTPConnectionByName", mock.Anything, "Declared").Return(nil, nil)
	s.fileStore.On("GetHTTPConnectionByName", mock.Anything, "Declared").Return(&declared, nil)
	failure := errors.New("database unavailable")
	s.dbStore.On("GetHTTPConnectionByName", mock.Anything, "Broken").Return(nil, failure)

	got, err := s.store.GetHTTPConnectionByName(s.ctx, "Risk")
	s.Require().NoError(err)
	s.Equal(mutable, got)
	got, err = s.store.GetHTTPConnectionByName(s.ctx, "Declared")
	s.Require().NoError(err)
	s.Equal(&declared, got)
	_, err = s.store.GetHTTPConnectionByName(s.ctx, "Broken")
	s.ErrorIs(err, failure)
}

func (s *HTTPConnectionCompositeStoreTestSuite) TestListMergesByIDPreferringTheDatabase() {
	shadowed := HTTPConnection{ID: "shared", Name: "Shared file", IsReadOnly: true}
	s.dbStore.On("CountHTTPConnections", mock.Anything).Return(2, nil)
	s.fileStore.On("CountHTTPConnections", mock.Anything).Return(2, nil)
	s.dbStore.On("ListHTTPConnections", mock.Anything).Return([]HTTPConnection{
		{ID: "db-1", Name: "Risk"}, {ID: "shared", Name: "Shared db"},
	}, nil)
	s.fileStore.On("ListHTTPConnections", mock.Anything).Return([]HTTPConnection{s.declarative(), shadowed}, nil)

	listed, err := s.store.ListHTTPConnections(s.ctx)

	s.Require().NoError(err)
	s.Equal([]HTTPConnection{
		{ID: "db-1", Name: "Risk"}, {ID: "shared", Name: "Shared db"}, s.declarative(),
	}, listed)
}

func (s *HTTPConnectionCompositeStoreTestSuite) TestListOverTheCompositeLimitFails() {
	s.dbStore.On("CountHTTPConnections", mock.Anything).Return(serverconst.MaxCompositeStoreRecords, nil)
	s.fileStore.On("CountHTTPConnections", mock.Anything).Return(1, nil)

	_, err := s.store.ListHTTPConnections(s.ctx)

	s.ErrorIs(err, errResultLimitExceededInCompositeMode)
}

func (s *HTTPConnectionCompositeStoreTestSuite) TestListFailsWhenEitherCountFails() {
	failure := errors.New("count failed")
	s.Run("database", func() {
		s.SetupTest()
		s.dbStore.On("CountHTTPConnections", mock.Anything).Return(0, failure)

		_, err := s.store.ListHTTPConnections(s.ctx)

		s.ErrorIs(err, failure)
		s.fileStore.AssertNotCalled(s.T(), "CountHTTPConnections", mock.Anything)
	})
	s.Run("file", func() {
		s.SetupTest()
		s.dbStore.On("CountHTTPConnections", mock.Anything).Return(1, nil)
		s.fileStore.On("CountHTTPConnections", mock.Anything).Return(0, failure)

		_, err := s.store.ListHTTPConnections(s.ctx)

		s.ErrorIs(err, failure)
		s.dbStore.AssertNotCalled(s.T(), "ListHTTPConnections", mock.Anything)
	})
}

func (s *HTTPConnectionCompositeStoreTestSuite) TestCountAddsBothStores() {
	s.dbStore.On("CountHTTPConnections", mock.Anything).Return(2, nil)
	s.fileStore.On("CountHTTPConnections", mock.Anything).Return(3, nil)

	count, err := s.store.CountHTTPConnections(s.ctx)

	s.Require().NoError(err)
	s.Equal(5, count)
}
