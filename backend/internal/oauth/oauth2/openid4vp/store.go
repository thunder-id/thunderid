// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package openid4vp

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// openID4VPStoreInterface persists short-lived OpenID4VP request state keyed by State. The
// production implementation (openID4VPStore) is runtime-store-backed so state
// survives restarts and is visible across replicas.
type openID4VPStoreInterface interface {
	SaveRequestState(ctx context.Context, st *RequestState) error
	GetRequestState(ctx context.Context, state string) (*RequestState, bool)
	DeleteRequestState(ctx context.Context, state string) error
	CompareAndSwapRequestState(ctx context.Context, st *RequestState, expected Status) (bool, error)
}

// openID4VPStore persists OpenID4VP request state in the runtime store so it
// survives restarts and is visible to every replica (the replica that receives
// the wallet response may differ from the one that initiated the request). State
// is stored as a JSON value under the vp:state namespace with a TTL derived from
// its expiry; the ephemeral response-decryption key is encrypted at rest with the
// server's configured symmetric key before serialization.
type openID4VPStore struct {
	store  providers.RuntimeStoreProvider
	crypto providers.RuntimeCryptoProvider
	logger *log.Logger
}

// storedRequestState is the serialized on-store form of RequestState. The ephemeral
// key is held as an encrypted PKCS#8 blob; the transient ResultToken is not persisted.
type storedRequestState struct {
	State         string
	VerifierID    string
	Nonce         string
	EphemeralKey  []byte
	ClientID      string
	RequestURI    string
	Status        Status
	Result        *VerifiedPresentation
	FailureReason string
	ExpiresAt     time.Time
}

// newOpenID4VPStore constructs a runtime-store-backed request state store using the given crypto provider.
func newOpenID4VPStore(
	crypto providers.RuntimeCryptoProvider, store providers.RuntimeStoreProvider,
) openID4VPStoreInterface {
	return &openID4VPStore{
		store:  store,
		crypto: crypto,
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "OpenID4VPStateStore")),
	}
}

// SaveRequestState stores the request state, encrypting the ephemeral key. It overwrites
// any existing entry for the same state, so status transitions re-save the full state.
func (s *openID4VPStore) SaveRequestState(ctx context.Context, st *RequestState) error {
	data, err := s.encode(ctx, st)
	if err != nil {
		return err
	}
	if err := s.store.Put(ctx, providers.NamespaceVPState, st.State, data, ttlUntil(st.ExpiresAt)); err != nil {
		return fmt.Errorf("failed to store request state: %w", err)
	}
	return nil
}

// CompareAndSwapRequestState atomically replaces the stored request state with st, but only
// when the currently stored status equals expected. The existing TTL is preserved. It returns
// false when the status differs or the state is absent or expired.
func (s *openID4VPStore) CompareAndSwapRequestState(
	ctx context.Context, st *RequestState, expected Status,
) (bool, error) {
	data, err := s.encode(ctx, st)
	if err != nil {
		return false, err
	}
	swapped, err := s.store.CompareFieldAndSwap(
		ctx, providers.NamespaceVPState, st.State, "Status", string(expected), data)
	if err != nil {
		return false, fmt.Errorf("failed to swap request state: %w", err)
	}
	return swapped, nil
}

// encode serializes a RequestState to its stored JSON form, encrypting the ephemeral key
// when present.
func (s *openID4VPStore) encode(ctx context.Context, st *RequestState) ([]byte, error) {
	stored := storedRequestState{
		State:         st.State,
		VerifierID:    st.VerifierID,
		Nonce:         st.Nonce,
		ClientID:      st.ClientID,
		RequestURI:    st.RequestURI,
		Status:        st.Status,
		Result:        st.Result,
		FailureReason: st.FailureReason,
		ExpiresAt:     st.ExpiresAt,
	}

	if st.EphemeralKey != nil {
		pkcs8, err := x509.MarshalPKCS8PrivateKey(st.EphemeralKey)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal ephemeral key: %w", err)
		}
		encKey, _, err := s.crypto.Encrypt(ctx, nil, string(cryptolib.AlgorithmAESGCM), nil, pkcs8)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt ephemeral key: %w", err)
		}
		stored.EphemeralKey = encKey
	}

	data, err := json.Marshal(stored)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request state: %w", err)
	}
	return data, nil
}

// GetRequestState retrieves and reconstructs the request state for the given state,
// returning false if it is not found or expired.
func (s *openID4VPStore) GetRequestState(ctx context.Context, state string) (*RequestState, bool) {
	data, err := s.store.Get(ctx, providers.NamespaceVPState, state)
	if err != nil {
		s.logger.Error(ctx, "Failed to get request state from runtime store", log.Error(err))
		return nil, false
	}
	if data == nil {
		return nil, false
	}
	rs, err := s.decode(ctx, data)
	if err != nil {
		s.logger.Error(ctx, "Failed to decode request state", log.Error(err))
		return nil, false
	}
	return rs, true
}

// DeleteRequestState deletes the request state for the given state from the runtime store.
func (s *openID4VPStore) DeleteRequestState(ctx context.Context, state string) error {
	return s.store.Delete(ctx, providers.NamespaceVPState, state)
}

// decode reconstructs a RequestState from its stored JSON form, decrypting the
// ephemeral key when present.
func (s *openID4VPStore) decode(ctx context.Context, data []byte) (*RequestState, error) {
	var stored storedRequestState
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("failed to unmarshal request state: %w", err)
	}

	rs := &RequestState{
		State:         stored.State,
		VerifierID:    stored.VerifierID,
		Nonce:         stored.Nonce,
		ClientID:      stored.ClientID,
		RequestURI:    stored.RequestURI,
		Status:        stored.Status,
		Result:        stored.Result,
		FailureReason: stored.FailureReason,
		ExpiresAt:     stored.ExpiresAt,
	}

	if len(stored.EphemeralKey) > 0 {
		pkcs8, err := s.crypto.Decrypt(ctx, nil, string(cryptolib.AlgorithmAESGCM), nil, stored.EphemeralKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt ephemeral key: %w", err)
		}
		parsed, err := x509.ParsePKCS8PrivateKey(pkcs8)
		if err != nil {
			return nil, fmt.Errorf("failed to parse ephemeral key: %w", err)
		}
		ecKey, ok := parsed.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("%w: ephemeral key is not an EC private key", ErrPolicy)
		}
		rs.EphemeralKey = ecKey
	}
	return rs, nil
}

// ttlUntil returns the whole seconds remaining until expiresAt, rounded up, with a
// floor of one second so the runtime store always applies a positive TTL (a
// non-positive TTL would otherwise persist the entry without expiry).
func ttlUntil(expiresAt time.Time) int64 {
	remaining := time.Until(expiresAt)
	if remaining <= 0 {
		return 1
	}
	return int64((remaining + time.Second - 1) / time.Second)
}
