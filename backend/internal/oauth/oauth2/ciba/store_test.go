// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ciba

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/runtimestore/inmemory"
)

const testDeploymentID = "test-deployment"

// CIBAStoreTestSuite exercises the CIBA store adapter against the in-memory runtime store,
// mirroring the way flow context tests its adapter.
type CIBAStoreTestSuite struct {
	suite.Suite
	store CIBARequestStoreInterface
	ctx   context.Context
}

func TestCIBAStoreTestSuite(t *testing.T) {
	suite.Run(t, new(CIBAStoreTestSuite))
}

func (s *CIBAStoreTestSuite) SetupTest() {
	s.store = newCIBAStore(inmemory.Initialize(testDeploymentID))
	s.ctx = context.Background()
}

func (s *CIBAStoreTestSuite) sampleRequest() *CIBAAuthRequest {
	return &CIBAAuthRequest{
		AuthReqID:      "auth-req-1",
		ClientID:       "client-1",
		StandardScopes: "openid profile",
		Resources:      []string{"https://api.example.com"},
		State:          CIBAStatePending,
		ExpiryTime:     time.Now().Add(2 * time.Minute),
	}
}

func (s *CIBAStoreTestSuite) TestNewCIBAStore() {
	s.NotNil(s.store)
	s.Implements((*CIBARequestStoreInterface)(nil), s.store)
}

func (s *CIBAStoreTestSuite) TestAddAndGetByID_RoundTrip() {
	req := s.sampleRequest()
	s.Require().NoError(s.store.Add(s.ctx, req))

	got, err := s.store.GetByID(s.ctx, req.AuthReqID)
	s.NoError(err)
	s.Require().NotNil(got)
	s.Equal(req.AuthReqID, got.AuthReqID)
	s.Equal(req.ClientID, got.ClientID)
	s.Equal(req.StandardScopes, got.StandardScopes)
	s.Equal(req.Resources, got.Resources)
	s.Equal(CIBAStatePending, got.State)
	s.WithinDuration(req.ExpiryTime, got.ExpiryTime, time.Second)
}

func (s *CIBAStoreTestSuite) TestAdd_AlreadyExpired_ReturnsError() {
	req := s.sampleRequest()
	req.ExpiryTime = time.Now().Add(-time.Second)

	err := s.store.Add(s.ctx, req)
	s.Error(err)
}

func (s *CIBAStoreTestSuite) TestGetByID_Missing_ReturnsNotFound() {
	got, err := s.store.GetByID(s.ctx, "no-such-req")
	s.ErrorIs(err, ErrCIBARequestNotFound)
	s.Nil(got)
}

func (s *CIBAStoreTestSuite) TestGetByID_EmptyID_ReturnsNotFound() {
	got, err := s.store.GetByID(s.ctx, "")
	s.ErrorIs(err, ErrCIBARequestNotFound)
	s.Nil(got)
}

func (s *CIBAStoreTestSuite) TestMarkAuthenticated_TransitionsAndRecordsClaims() {
	req := s.sampleRequest()
	s.Require().NoError(s.store.Add(s.ctx, req))

	authTime := time.Now().Truncate(time.Second)
	err := s.store.MarkAuthenticated(s.ctx, req.AuthReqID, "user-1", "openid", "cache-1", "urn:acr", "sess-1", authTime)
	s.NoError(err)

	got, err := s.store.GetByID(s.ctx, req.AuthReqID)
	s.Require().NoError(err)
	s.Equal(CIBAStateAuthenticated, got.State)
	s.Equal("user-1", got.UserID)
	s.Equal("openid", got.AuthorizedScopes)
	s.Equal("cache-1", got.AttributeCacheID)
	s.Equal("urn:acr", got.CompletedACR)
	s.Equal("sess-1", got.SessionID)
	s.True(authTime.Equal(got.AuthTime))
}

func (s *CIBAStoreTestSuite) TestMarkAuthenticated_Missing_ReturnsNotFound() {
	err := s.store.MarkAuthenticated(s.ctx, "no-such-req", "u", "", "", "", "", time.Now())
	s.ErrorIs(err, ErrCIBARequestNotFound)
}

func (s *CIBAStoreTestSuite) TestMarkAuthenticated_NotPending_ReturnsError() {
	req := s.sampleRequest()
	s.Require().NoError(s.store.Add(s.ctx, req))
	s.Require().NoError(s.store.UpdateState(s.ctx, req.AuthReqID, CIBAStateConsumed))

	err := s.store.MarkAuthenticated(s.ctx, req.AuthReqID, "u", "", "", "", "", time.Now())
	s.Error(err)
}

// TestMarkAuthenticated_ConcurrentCallbacks verifies the compare-and-swap guard: only the first of
// several concurrent callbacks may transition a pending request.
func (s *CIBAStoreTestSuite) TestMarkAuthenticated_ConcurrentCallbacks() {
	req := s.sampleRequest()
	s.Require().NoError(s.store.Add(s.ctx, req))

	const workers = 10
	var wg sync.WaitGroup
	results := make([]error, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			results[i] = s.store.MarkAuthenticated(s.ctx, req.AuthReqID, "user-1", "openid", "cache-1",
				"urn:acr", "", time.Now())
		}(i)
	}
	wg.Wait()

	wins := 0
	for _, err := range results {
		if err == nil {
			wins++
		}
	}
	s.Equal(1, wins, "exactly one concurrent callback should win the PENDING->AUTHENTICATED transition")
}

func (s *CIBAStoreTestSuite) TestMarkConsumed_OnceThenFalse() {
	req := s.sampleRequest()
	s.Require().NoError(s.store.Add(s.ctx, req))
	s.Require().NoError(s.store.MarkAuthenticated(s.ctx, req.AuthReqID, "u", "", "", "", "", time.Now()))

	consumed, err := s.store.MarkConsumed(s.ctx, req.AuthReqID)
	s.NoError(err)
	s.True(consumed)

	again, err := s.store.MarkConsumed(s.ctx, req.AuthReqID)
	s.NoError(err)
	s.False(again, "a second consume must not succeed")

	got, err := s.store.GetByID(s.ctx, req.AuthReqID)
	s.Require().NoError(err)
	s.Equal(CIBAStateConsumed, got.State)
}

func (s *CIBAStoreTestSuite) TestMarkConsumed_NotAuthenticated_ReturnsFalse() {
	req := s.sampleRequest()
	s.Require().NoError(s.store.Add(s.ctx, req))

	consumed, err := s.store.MarkConsumed(s.ctx, req.AuthReqID)
	s.NoError(err)
	s.False(consumed)
}

func (s *CIBAStoreTestSuite) TestMarkConsumed_Missing_ReturnsFalseNoError() {
	consumed, err := s.store.MarkConsumed(s.ctx, "no-such-req")
	s.NoError(err)
	s.False(consumed)
}

// TestMarkConsumed_ConcurrentPolls verifies one-time-use under concurrent token polls.
func (s *CIBAStoreTestSuite) TestMarkConsumed_ConcurrentPolls() {
	req := s.sampleRequest()
	s.Require().NoError(s.store.Add(s.ctx, req))
	s.Require().NoError(s.store.MarkAuthenticated(s.ctx, req.AuthReqID, "u", "", "", "", "", time.Now()))

	const workers = 10
	var wg sync.WaitGroup
	results := make([]bool, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			ok, _ := s.store.MarkConsumed(s.ctx, req.AuthReqID)
			results[i] = ok
		}(i)
	}
	wg.Wait()

	wins := 0
	for _, ok := range results {
		if ok {
			wins++
		}
	}
	s.Equal(1, wins, "exactly one concurrent poll should consume the request")
}

func (s *CIBAStoreTestSuite) TestUpdateState() {
	req := s.sampleRequest()
	s.Require().NoError(s.store.Add(s.ctx, req))

	s.Require().NoError(s.store.UpdateState(s.ctx, req.AuthReqID, CIBAStateDenied))

	got, err := s.store.GetByID(s.ctx, req.AuthReqID)
	s.Require().NoError(err)
	s.Equal(CIBAStateDenied, got.State)
}

func (s *CIBAStoreTestSuite) TestUpdateLastPolled() {
	req := s.sampleRequest()
	s.Require().NoError(s.store.Add(s.ctx, req))

	polledAt := time.Now().Truncate(time.Second)
	s.Require().NoError(s.store.UpdateLastPolled(s.ctx, req.AuthReqID, polledAt))

	got, err := s.store.GetByID(s.ctx, req.AuthReqID)
	s.Require().NoError(err)
	s.True(polledAt.Equal(got.LastPolledAt))
}
