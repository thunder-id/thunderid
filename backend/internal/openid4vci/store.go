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

package openid4vci

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// openID4VCIStoreInterface persists the OpenID4VCI issuer's short-lived runtime
// state — c_nonces and issuer-initiated credential offers — keyed by nonce/id.
type openID4VCIStoreInterface interface {
	SaveNonce(ctx context.Context, nonce string, rec *nonceRecord) error
	GetNonce(ctx context.Context, nonce string) (*nonceRecord, bool)
	DeleteNonce(ctx context.Context, nonce string) error
	SaveOffer(ctx context.Context, id string, rec *offerRecord) error
	GetOffer(ctx context.Context, id string) (*offerRecord, bool)
}

// openID4VCIStore persists the issuer's runtime state (c_nonces and credential
// offers) in the runtime store so it is visible across replicas — the replica
// that issues a nonce/offer may differ from the one that consumes it. Each record
// is stored as a JSON value under its namespace with a TTL derived from its expiry.
type openID4VCIStore struct {
	store  providers.RuntimeStoreProvider
	logger *log.Logger
}

// newOpenID4VCIStore creates a new openID4VCIStore backed by the runtime store provider.
func newOpenID4VCIStore(store providers.RuntimeStoreProvider) openID4VCIStoreInterface {
	return &openID4VCIStore{
		store:  store,
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "OpenID4VCIStateStore")),
	}
}

// SaveNonce persists a nonce record to the runtime store.
func (s *openID4VCIStore) SaveNonce(ctx context.Context, nonce string, rec *nonceRecord) error {
	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("failed to marshal nonce record: %w", err)
	}
	return s.store.Put(ctx, providers.NamespaceVCINonce, nonce, data, ttlUntil(rec.ExpiresAt))
}

// GetNonce retrieves a stored nonce record, returning false if it is not found or expired.
func (s *openID4VCIStore) GetNonce(ctx context.Context, nonce string) (*nonceRecord, bool) {
	data, err := s.store.Get(ctx, providers.NamespaceVCINonce, nonce)
	if err != nil {
		s.logger.Error(ctx, "Failed to get nonce from runtime store", log.Error(err))
		return nil, false
	}
	if data == nil {
		return nil, false
	}
	var rec nonceRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		s.logger.Error(ctx, "Failed to unmarshal nonce record", log.Error(err))
		return nil, false
	}
	return &rec, true
}

// DeleteNonce removes a nonce record from the runtime store.
func (s *openID4VCIStore) DeleteNonce(ctx context.Context, nonce string) error {
	return s.store.Delete(ctx, providers.NamespaceVCINonce, nonce)
}

// SaveOffer persists a credential offer record to the runtime store.
func (s *openID4VCIStore) SaveOffer(ctx context.Context, id string, rec *offerRecord) error {
	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("failed to marshal credential offer: %w", err)
	}
	return s.store.Put(ctx, providers.NamespaceVCIOffer, id, data, ttlUntil(rec.ExpiresAt))
}

// GetOffer retrieves a stored credential offer by ID, returning false if it is not found or expired.
func (s *openID4VCIStore) GetOffer(ctx context.Context, id string) (*offerRecord, bool) {
	data, err := s.store.Get(ctx, providers.NamespaceVCIOffer, id)
	if err != nil {
		s.logger.Error(ctx, "Failed to get credential offer from runtime store", log.Error(err))
		return nil, false
	}
	if data == nil {
		return nil, false
	}
	var rec offerRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		s.logger.Error(ctx, "Failed to unmarshal credential offer", log.Error(err))
		return nil, false
	}
	return &rec, true
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
