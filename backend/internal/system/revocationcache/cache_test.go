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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRevokedCache_ReplaceAndIsRevoked(t *testing.T) {
	c := newRevokedCache()
	future := time.Now().Add(time.Hour)

	assert.False(t, c.isTokenRevoked("jti-1"), "empty cache reports nothing revoked")

	c.replace(revokedSnapshot{
		Tokens: []revokedEntry{
			{Value: "jti-1", ExpiryTime: future},
			{Value: "jti-2", ExpiryTime: future},
		},
		Families: []revokedEntry{{Value: "tfid-1", ExpiryTime: future}},
	})

	assert.True(t, c.isTokenRevoked("jti-1"))
	assert.True(t, c.isTokenRevoked("jti-2"))
	assert.False(t, c.isTokenRevoked("jti-3"))
	assert.True(t, c.isTokenFamilyRevoked("tfid-1"))
	assert.False(t, c.isTokenFamilyRevoked("jti-1"), "a jti must not match the family dimension")
	assert.False(t, c.isTokenRevoked("tfid-1"), "a tfid must not match the token dimension")
}

func TestRevokedCache_ReplaceSwapsSnapshot(t *testing.T) {
	c := newRevokedCache()
	future := time.Now().Add(time.Hour)

	c.replace(revokedSnapshot{Tokens: []revokedEntry{{Value: "old", ExpiryTime: future}}})
	assert.True(t, c.isTokenRevoked("old"))

	c.replace(revokedSnapshot{Tokens: []revokedEntry{{Value: "new", ExpiryTime: future}}})
	assert.False(t, c.isTokenRevoked("old"), "prior entries are dropped on replace")
	assert.True(t, c.isTokenRevoked("new"))
}

func TestRevokedCache_ExpiredEntryNotRevoked(t *testing.T) {
	c := newRevokedCache()
	c.replace(revokedSnapshot{Tokens: []revokedEntry{{Value: "expired", ExpiryTime: time.Now().Add(-time.Second)}}})

	assert.False(t, c.isTokenRevoked("expired"), "an entry past its expiry is treated as not revoked")
}

func TestRevokedCache_ConcurrentAccess(t *testing.T) {
	c := newRevokedCache()
	future := time.Now().Add(time.Hour)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			c.replace(revokedSnapshot{Tokens: []revokedEntry{{Value: "jti", ExpiryTime: future}}})
		}()
		go func() {
			defer wg.Done()
			_ = c.isTokenRevoked("jti")
		}()
	}
	wg.Wait()

	assert.True(t, c.isTokenRevoked("jti"))
}
