// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ciba

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/runtimestore/inmemory"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/runtimestoreprovidermock"
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
	err := s.store.MarkAuthenticated(s.ctx, req.AuthReqID, "user-1", "openid", "cache-1", "urn:acr", "sess-1",
		authTime, nil)
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

func (s *CIBAStoreTestSuite) TestMarkAuthenticated_RecordsWhenTheRequestWasAuthorized() {
	req := s.sampleRequest()
	s.Require().NoError(s.store.Add(s.ctx, req))
	sessionAuthTime := time.Now().Add(-time.Hour)

	before := time.Now()
	s.Require().NoError(s.store.MarkAuthenticated(s.ctx, req.AuthReqID, "user-1", "openid", "", "", "",
		sessionAuthTime, nil))
	after := time.Now()

	got, err := s.store.GetByID(s.ctx, req.AuthReqID)
	s.Require().NoError(err)
	s.False(got.AuthorizedAt.Before(before))
	s.False(got.AuthorizedAt.After(after))
}

func (s *CIBAStoreTestSuite) TestMarkAuthenticated_Missing_ReturnsNotFound() {
	err := s.store.MarkAuthenticated(s.ctx, "no-such-req", "u", "", "", "", "", time.Now(), nil)
	s.ErrorIs(err, ErrCIBARequestNotFound)
}

func (s *CIBAStoreTestSuite) TestMarkAuthenticated_NotPending_ReturnsError() {
	req := s.sampleRequest()
	s.Require().NoError(s.store.Add(s.ctx, req))
	s.Require().NoError(s.store.UpdateState(s.ctx, req.AuthReqID, CIBAStateConsumed))

	err := s.store.MarkAuthenticated(s.ctx, req.AuthReqID, "u", "", "", "", "", time.Now(), nil)
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
				"urn:acr", "", time.Now(), nil)
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
	s.Require().NoError(s.store.MarkAuthenticated(s.ctx, req.AuthReqID, "u", "", "", "", "", time.Now(), nil))

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
	s.Require().NoError(s.store.MarkAuthenticated(s.ctx, req.AuthReqID, "u", "", "", "", "", time.Now(), nil))

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

func (s *CIBAStoreTestSuite) TestMarkFailed_RecordsTheReason() {
	req := s.sampleRequest()
	s.Require().NoError(s.store.Add(s.ctx, req))

	s.Require().NoError(s.store.MarkFailed(s.ctx, req.AuthReqID, CIBAStateDenied, "over limit"))

	got, err := s.store.GetByID(s.ctx, req.AuthReqID)
	s.Require().NoError(err)
	s.Equal(CIBAStateDenied, got.State)
	s.Equal("over limit", got.FailureDescription)
	s.Error(s.store.MarkFailed(s.ctx, "no-such-req", CIBAStateDenied, ""))
}

func (s *CIBAStoreTestSuite) TestMarkFailed_NotPending_ReturnsError() {
	req := s.sampleRequest()
	s.Require().NoError(s.store.Add(s.ctx, req))
	s.Require().NoError(s.store.MarkAuthenticated(s.ctx, req.AuthReqID, "u", "", "", "", "", time.Now(), nil))

	s.ErrorIs(s.store.MarkFailed(s.ctx, req.AuthReqID, CIBAStateDenied, "over limit"), ErrCIBARequestNotPending)

	got, err := s.store.GetByID(s.ctx, req.AuthReqID)
	s.Require().NoError(err)
	s.Equal(CIBAStateAuthenticated, got.State)
	s.Empty(got.FailureDescription)
}

func (s *CIBAStoreTestSuite) TestMarkFailed_SwapsOnlyFromPending() {
	data, err := json.Marshal(s.sampleRequest())
	s.Require().NoError(err)

	for name, tc := range map[string]struct {
		swapped bool
		err     error
		wantErr string
	}{
		"swapped":                  {swapped: true},
		"another transition first": {wantErr: "is not pending"},
		"store failure":            {err: errors.New("store down"), wantErr: "failed to mark CIBA request as failed"},
	} {
		s.Run(name, func() {
			rt := runtimestoreprovidermock.NewRuntimeStoreProviderMock(s.T())
			rt.EXPECT().Get(mock.Anything, providers.NamespaceCIBA, "auth-req-1").Return(data, nil)
			rt.EXPECT().CompareFieldAndSwap(mock.Anything, providers.NamespaceCIBA, "auth-req-1", cibaStateField,
				string(CIBAStatePending), mock.MatchedBy(func(value []byte) bool {
					var stored CIBAAuthRequest
					return json.Unmarshal(value, &stored) == nil && stored.State == CIBAStateDenied &&
						stored.FailureDescription == "over limit"
				})).Return(tc.swapped, tc.err)

			err := newCIBAStore(rt).MarkFailed(s.ctx, "auth-req-1", CIBAStateDenied, "over limit")
			if tc.wantErr == "" {
				s.NoError(err)
			} else {
				s.ErrorContains(err, tc.wantErr)
			}
		})
	}
}

func (s *CIBAStoreTestSuite) TestMarkAuthenticated_RecordsAuthorizationDetails() {
	req := s.sampleRequest()
	s.Require().NoError(s.store.Add(s.ctx, req))
	granted := []providers.AuthorizationDetail{{ID: "d-1", Type: "payment_initiation", TypeID: "type-2",
		TypeVersion: 2, Requested: json.RawMessage(`{"type":"payment_initiation"}`),
		Value: json.RawMessage(`{"type":"payment_initiation","amount":45.00}`), Admitted: true}}

	s.Require().NoError(s.store.MarkAuthenticated(s.ctx, req.AuthReqID, "u", "", "", "", "", time.Now(), granted))

	got, err := s.store.GetByID(s.ctx, req.AuthReqID)
	s.Require().NoError(err)
	s.Equal(granted, got.AuthorizationDetails)
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

func (s *CIBAStoreTestSuite) TestUpdateLastPolled_NotPending_KeepsTheDecision() {
	granted := []providers.AuthorizationDetail{{ID: "d-1", Type: "payment_initiation", TypeID: "type-2",
		TypeVersion: 2, Value: json.RawMessage(`{"type":"payment_initiation","amount":45.00}`), Admitted: true}}

	for name, decide := range map[string]func(string) error{
		"denied": func(id string) error {
			return s.store.MarkFailed(s.ctx, id, CIBAStateDenied, "over limit")
		},
		"authenticated": func(id string) error {
			return s.store.MarkAuthenticated(s.ctx, id, "u", "", "", "", "", time.Now(), granted)
		},
	} {
		s.Run(name, func() {
			req := s.sampleRequest()
			s.Require().NoError(s.store.Add(s.ctx, req))
			s.Require().NoError(decide(req.AuthReqID))
			before, err := s.store.GetByID(s.ctx, req.AuthReqID)
			s.Require().NoError(err)

			s.NoError(s.store.UpdateLastPolled(s.ctx, req.AuthReqID, time.Now()))

			got, err := s.store.GetByID(s.ctx, req.AuthReqID)
			s.Require().NoError(err)
			s.Equal(before.State, got.State)
			s.Equal(before.FailureDescription, got.FailureDescription)
			s.Equal(before.AuthorizationDetails, got.AuthorizationDetails)
			s.True(got.LastPolledAt.IsZero())
		})
	}
}

func (s *CIBAStoreTestSuite) TestUpdateLastPolled_SwapsOnlyFromPending() {
	data, err := json.Marshal(s.sampleRequest())
	s.Require().NoError(err)
	polledAt := time.Now().Truncate(time.Second)

	for name, tc := range map[string]struct {
		swapped bool
		err     error
		wantErr string
	}{
		"swapped":                  {swapped: true},
		"another transition first": {},
		"store failure":            {err: errors.New("store down"), wantErr: "failed to update CIBA last polled time"},
	} {
		s.Run(name, func() {
			rt := runtimestoreprovidermock.NewRuntimeStoreProviderMock(s.T())
			rt.EXPECT().Get(mock.Anything, providers.NamespaceCIBA, "auth-req-1").Return(data, nil)
			rt.EXPECT().CompareFieldAndSwap(mock.Anything, providers.NamespaceCIBA, "auth-req-1", cibaStateField,
				string(CIBAStatePending), mock.MatchedBy(func(value []byte) bool {
					var stored CIBAAuthRequest
					return json.Unmarshal(value, &stored) == nil && stored.State == CIBAStatePending &&
						polledAt.Equal(stored.LastPolledAt)
				})).Return(tc.swapped, tc.err)

			err := newCIBAStore(rt).UpdateLastPolled(s.ctx, "auth-req-1", polledAt)
			if tc.wantErr == "" {
				s.NoError(err)
			} else {
				s.ErrorContains(err, tc.wantErr)
			}
		})
	}
}

func (s *CIBAStoreTestSuite) TestUpdateLastPolled_NotPending_DoesNotWrite() {
	req := s.sampleRequest()
	req.State = CIBAStateDenied
	req.FailureDescription = "over limit"
	data, err := json.Marshal(req)
	s.Require().NoError(err)

	rt := runtimestoreprovidermock.NewRuntimeStoreProviderMock(s.T())
	rt.EXPECT().Get(mock.Anything, providers.NamespaceCIBA, "auth-req-1").Return(data, nil)

	s.NoError(newCIBAStore(rt).UpdateLastPolled(s.ctx, "auth-req-1", time.Now()))
}
