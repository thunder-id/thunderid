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

package revocation

import (
	"context"
	"time"

	"github.com/thunder-id/thunderid/internal/system/log"
)

// defaultTokenFamilyRevocationTTL bounds a family revocation entry when no refresh-token lifetime is
// configured. It is a safe upper bound: the entry only needs to outlive the longest-lived token of
// the family, and expired tokens are rejected on their own exp regardless.
const defaultTokenFamilyRevocationTTL = 30 * 24 * time.Hour

// CriteriaRevokerInterface is the narrow write seam for criteria-based (many-token) revocation: it
// records a revocation criterion so every token matching it is rejected. The only criterion today is
// token_family, which revokes a whole authorization grant by its token family id (tfid); the seam is
// shaped so future criteria (e.g. by subject or client) reuse the same writer and store. It is
// consumed by the refresh grant (reuse), the RFC 7009 endpoint (explicit), the authorization service
// (code replay), and session sign-out (logout).
type CriteriaRevokerInterface interface {
	// RevokeTokenFamily records a terminal revocation of the token family identified by tokenFamilyID, so
	// every access and refresh token carrying that tfid is rejected. An empty tokenFamilyID is a
	// no-op. The write is idempotent.
	RevokeTokenFamily(ctx context.Context, tokenFamilyID string, reason RevocationReason) error
}

// criteriaRevoker is the default CriteriaRevokerInterface. It writes a token_family criterion to the
// criteria deny list, bounding its lifetime by the configured refresh-token lifetime.
type criteriaRevoker struct {
	store               revocationStoreInterface
	tokenFamilyLifetime time.Duration
	logger              *log.Logger
}

// newCriteriaRevoker creates a criteria revoker over the criteria deny list. tokenFamilyLifetime bounds each
// entry (revoked_at + tokenFamilyLifetime); a non-positive value falls back to defaultTokenFamilyRevocationTTL.
// It is unexported and constructed once via Initialize so the write path has a single owner.
func newCriteriaRevoker(store revocationStoreInterface, tokenFamilyLifetime time.Duration) CriteriaRevokerInterface {
	if tokenFamilyLifetime <= 0 {
		tokenFamilyLifetime = defaultTokenFamilyRevocationTTL
	}
	return &criteriaRevoker{
		store:               store,
		tokenFamilyLifetime: tokenFamilyLifetime,
		logger:              log.GetLogger().With(log.String(log.LoggerKeyComponentName, "CriteriaRevoker")),
	}
}

// RevokeTokenFamily records a terminal revocation of the given token family.
func (r *criteriaRevoker) RevokeTokenFamily(ctx context.Context, tokenFamilyID string,
	reason RevocationReason) error {
	if tokenFamilyID == "" {
		return nil
	}

	now := time.Now().UTC()
	if err := r.store.insertCriterion(ctx, revocationCriterion{
		Type:       criterionTypeTokenFamily,
		Value:      tokenFamilyID,
		Reason:     reason,
		RevokedAt:  now,
		ExpiryTime: now.Add(r.tokenFamilyLifetime),
	}); err != nil {
		return err
	}

	r.logger.Debug(ctx, "Revoked token family", log.String("reason", string(reason)))
	return nil
}
