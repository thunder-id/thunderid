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

package tokenstatus

import (
	"time"

	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
)

// Initialize builds the Token Status List service from cfg. The composition root calls it only when the
// subsystem is enabled and injects the returned service into the consumers (issuance, revocation,
// enforcement) as their own narrow interfaces. A missing base URL is rejected because every issued
// reference embeds an absolute list URI that cannot be changed after the fact; the JWT service signs
// published list tokens and must be present; and the TTL must be positive so published tokens carry a
// coherent ttl/exp.
func Initialize(cfg Config, jwtService jwt.JWTServiceInterface) (ServiceInterface, error) {
	if cfg.BaseURL == "" {
		return nil, errEmptyBaseURL
	}
	if jwtService == nil {
		return nil, errNilJWTService
	}
	if cfg.TTL <= 0 {
		return nil, errNonPositiveTTL
	}
	return &service{
		store:      newStatusStore(cfg.ListSize, cfg.Bits, retentionFor(cfg.MaxTokenTTL)),
		jwtService: jwtService,
		baseURL:    cfg.BaseURL,
		ttlSeconds: int(cfg.TTL.Seconds()),
	}, nil
}

// retentionFor derives the sealed-list retention window from the longest token lifetime by adding the
// safety grace. A non-positive maxTokenTTL disables reaping (returns zero) so a list is never dropped
// while live tokens may still reference it.
func retentionFor(maxTokenTTL time.Duration) time.Duration {
	if maxTokenTTL <= 0 {
		return 0
	}
	return maxTokenTTL + retentionGrace
}
