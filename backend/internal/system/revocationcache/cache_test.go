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

	assert.False(t, c.isRevoked("jti-1"), "empty cache reports nothing revoked")

	c.replace([]revokedEntry{
		{JTI: "jti-1", ExpiryTime: future},
		{JTI: "jti-2", ExpiryTime: future},
	})

	assert.True(t, c.isRevoked("jti-1"))
	assert.True(t, c.isRevoked("jti-2"))
	assert.False(t, c.isRevoked("jti-3"))
}

func TestRevokedCache_ReplaceSwapsSnapshot(t *testing.T) {
	c := newRevokedCache()
	future := time.Now().Add(time.Hour)

	c.replace([]revokedEntry{{JTI: "old", ExpiryTime: future}})
	assert.True(t, c.isRevoked("old"))

	c.replace([]revokedEntry{{JTI: "new", ExpiryTime: future}})
	assert.False(t, c.isRevoked("old"), "prior entries are dropped on replace")
	assert.True(t, c.isRevoked("new"))
}

func TestRevokedCache_ExpiredEntryNotRevoked(t *testing.T) {
	c := newRevokedCache()
	c.replace([]revokedEntry{{JTI: "expired", ExpiryTime: time.Now().Add(-time.Second)}})

	assert.False(t, c.isRevoked("expired"), "an entry past its expiry is treated as not revoked")
}

func TestRevokedCache_ConcurrentAccess(t *testing.T) {
	c := newRevokedCache()
	future := time.Now().Add(time.Hour)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			c.replace([]revokedEntry{{JTI: "jti", ExpiryTime: future}})
		}()
		go func() {
			defer wg.Done()
			_ = c.isRevoked("jti")
		}()
	}
	wg.Wait()

	assert.True(t, c.isRevoked("jti"))
}
