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

package openid4vp

import (
	"context"
	"crypto"
	"fmt"
	"maps"
)

// staticTrustStore pins a fixed set of issuer keys.
type staticTrustStore struct {
	keys map[string]crypto.PublicKey
}

func newStaticTrustStore(keys map[string]crypto.PublicKey) *staticTrustStore {
	pinned := make(map[string]crypto.PublicKey, len(keys))
	maps.Copy(pinned, keys)
	return &staticTrustStore{keys: pinned}
}

func (s *staticTrustStore) resolveIssuerKey(_ context.Context, issuer string) (crypto.PublicKey, error) {
	key, ok := s.keys[issuer]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUntrustedIssuer, issuer)
	}
	return key, nil
}
