// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package passkey

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/runtimestore/inmemory"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/identitygovernancemock"
	"github.com/thunder-id/thunderid/tests/mocks/runtimestoreprovidermock"
)

const testDeploymentID = "test-deployment"

// TestInitialize verifies the service wiring builds a store backed by the injected runtime store.
func TestInitialize(t *testing.T) {
	svc := Initialize(entitymock.NewEntityServiceInterfaceMock(t), inmemory.Initialize(testDeploymentID),
		identitygovernancemock.NewIdentityGovernanceServiceInterfaceMock(t))
	assert.NotNil(t, svc)
}

// SessionStoreTestSuite exercises the passkey session store adapter against the in-memory runtime
// store, mirroring the way the CIBA store tests its adapter.
type SessionStoreTestSuite struct {
	suite.Suite
	store sessionStoreInterface
	ctx   context.Context
}

func TestSessionStoreTestSuite(t *testing.T) {
	suite.Run(t, new(SessionStoreTestSuite))
}

func (s *SessionStoreTestSuite) SetupTest() {
	s.store = newSessionStore(inmemory.Initialize(testDeploymentID))
	s.ctx = context.Background()
}

func (s *SessionStoreTestSuite) sampleSession() *sessionData {
	return &sessionData{
		Challenge:            "challenge-123",
		RelyingPartyID:       "example.com",
		UserID:               []byte("user-123"),
		AllowedCredentialIDs: [][]byte{[]byte("cred-1"), []byte("cred-2")},
		UserVerification:     protocol.VerificationPreferred,
		Expires:              time.Now().Add(2 * time.Minute).UTC(),
	}
}

func (s *SessionStoreTestSuite) TestNewSessionStore() {
	s.NotNil(s.store)
	s.Implements((*sessionStoreInterface)(nil), s.store)
}

// TestStoreRetrieveDelete_RoundTrip stores, reads back, and deletes a range of session shapes
// through the real in-memory runtime store.
func (s *SessionStoreTestSuite) TestStoreRetrieveDelete_RoundTrip() {
	tests := []struct {
		name    string
		session *sessionData
	}{
		{
			name: "minimal",
			session: &sessionData{
				Challenge:      "challenge-min",
				RelyingPartyID: "example.com",
				Expires:        time.Now().Add(time.Minute).UTC(),
			},
		},
		{
			name:    "full",
			session: s.sampleSession(),
		},
		{
			name: "with extensions",
			session: &sessionData{
				Challenge:        "challenge-ext",
				RelyingPartyID:   "example.com",
				UserID:           []byte("user-ext"),
				UserVerification: protocol.VerificationRequired,
				Extensions:       protocol.AuthenticationExtensions{"appid": "https://example.com"},
				Expires:          time.Now().Add(time.Minute).UTC(),
			},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			const key = "session-key"
			s.Require().NoError(s.store.storeSession(s.ctx, key, tt.session, sessionTTLSeconds))

			got, err := s.store.retrieveSession(s.ctx, key)
			s.Require().NoError(err)
			s.Require().NotNil(got)
			s.Equal(tt.session.Challenge, got.Challenge)
			s.Equal(tt.session.RelyingPartyID, got.RelyingPartyID)
			s.Equal(tt.session.UserID, got.UserID)
			s.Equal(tt.session.AllowedCredentialIDs, got.AllowedCredentialIDs)
			s.Equal(tt.session.UserVerification, got.UserVerification)
			s.Equal(tt.session.Extensions, got.Extensions)
			s.WithinDuration(tt.session.Expires, got.Expires, time.Second)

			s.Require().NoError(s.store.deleteSession(s.ctx, key))

			gone, err := s.store.retrieveSession(s.ctx, key)
			s.NoError(err)
			s.Nil(gone)
		})
	}
}

// TestRetrieveSession_ReturnsNil covers the lookups that yield no session without an error.
func (s *SessionStoreTestSuite) TestRetrieveSession_ReturnsNil() {
	tests := []struct {
		name string
		key  string
	}{
		{name: "empty key", key: ""},
		{name: "missing key", key: "no-such-session"},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			got, err := s.store.retrieveSession(s.ctx, tt.key)
			s.NoError(err)
			s.Nil(got)
		})
	}
}

// TestDeleteSession_NoError covers deletes that must succeed without an existing session.
func (s *SessionStoreTestSuite) TestDeleteSession_NoError() {
	tests := []struct {
		name string
		key  string
	}{
		{name: "empty key", key: ""},
		{name: "missing key", key: "no-such-session"},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.NoError(s.store.deleteSession(s.ctx, tt.key))
		})
	}
}

// TestStoreSession_MarshalError covers the serialization failure path: Extensions holds a value the
// JSON encoder cannot represent, so storeSession returns before the session reaches the store.
func (s *SessionStoreTestSuite) TestStoreSession_MarshalError() {
	session := s.sampleSession()
	session.Extensions = protocol.AuthenticationExtensions{"bad": make(chan int)}

	err := s.store.storeSession(s.ctx, "session-key", session, sessionTTLSeconds)
	s.Require().Error(err)
	s.Contains(err.Error(), "failed to marshal passkey session")
}

// TestProviderErrors covers the branches where the underlying runtime store provider fails.
func (s *SessionStoreTestSuite) TestProviderErrors() {
	tests := []struct {
		name    string
		setup   func(*runtimestoreprovidermock.RuntimeStoreProviderMock)
		invoke  func(sessionStoreInterface) error
		wantErr string
	}{
		{
			name: "store put error",
			setup: func(m *runtimestoreprovidermock.RuntimeStoreProviderMock) {
				m.EXPECT().Put(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(fmt.Errorf("put failed"))
			},
			invoke: func(st sessionStoreInterface) error {
				return st.storeSession(s.ctx, "session-key", s.sampleSession(), sessionTTLSeconds)
			},
			wantErr: "put failed",
		},
		{
			name: "retrieve get error",
			setup: func(m *runtimestoreprovidermock.RuntimeStoreProviderMock) {
				m.EXPECT().Get(mock.Anything, mock.Anything, mock.Anything).
					Return(nil, fmt.Errorf("get failed"))
			},
			invoke: func(st sessionStoreInterface) error {
				_, err := st.retrieveSession(s.ctx, "session-key")
				return err
			},
			wantErr: "failed to get passkey session",
		},
		{
			name: "retrieve unmarshal error",
			setup: func(m *runtimestoreprovidermock.RuntimeStoreProviderMock) {
				m.EXPECT().Get(mock.Anything, mock.Anything, mock.Anything).
					Return([]byte("not-valid-json"), nil)
			},
			invoke: func(st sessionStoreInterface) error {
				_, err := st.retrieveSession(s.ctx, "session-key")
				return err
			},
			wantErr: "failed to unmarshal passkey session",
		},
		{
			name: "delete error",
			setup: func(m *runtimestoreprovidermock.RuntimeStoreProviderMock) {
				m.EXPECT().Delete(mock.Anything, mock.Anything, mock.Anything).
					Return(fmt.Errorf("delete failed"))
			},
			invoke: func(st sessionStoreInterface) error {
				return st.deleteSession(s.ctx, "session-key")
			},
			wantErr: "failed to delete passkey session",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			m := runtimestoreprovidermock.NewRuntimeStoreProviderMock(s.T())
			tt.setup(m)
			st := newSessionStore(m)

			err := tt.invoke(st)
			s.Require().Error(err)
			s.Contains(err.Error(), tt.wantErr)
		})
	}
}
