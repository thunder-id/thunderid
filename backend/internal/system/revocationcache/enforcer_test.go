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

package revocationcache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEnforcer_EnsureNotRevoked(t *testing.T) {
	cache := newRevokedCache()
	cache.replace(revokedSnapshot{
		Tokens:   []revokedEntry{{Value: "revoked-jti", ExpiryTime: time.Now().Add(time.Hour)}},
		Families: []revokedEntry{{Value: "revoked-tfid", ExpiryTime: time.Now().Add(time.Hour)}},
	})
	e := newEnforcer(cache)

	assert.NoError(t, e.EnsureNotRevoked(context.Background(), "", ""),
		"empty ids are a no-op")
	assert.NoError(t, e.EnsureNotRevoked(context.Background(), "active-jti", "active-tfid"),
		"a token with a clean jti and family may proceed")
	assert.ErrorIs(t, e.EnsureNotRevoked(context.Background(), "revoked-jti", ""), errTokenRevoked,
		"a jti on the deny list is rejected")
	assert.ErrorIs(t, e.EnsureNotRevoked(context.Background(), "active-jti", "revoked-tfid"), errTokenRevoked,
		"a token whose family is revoked is rejected even with a clean jti")
}

func TestNoopEnforcer_AlwaysAllows(t *testing.T) {
	var e EnforcerInterface = noopEnforcer{}
	assert.NoError(t, e.EnsureNotRevoked(context.Background(), "anything", ""))
	assert.NoError(t, e.EnsureNotRevoked(context.Background(), "", ""))
}
