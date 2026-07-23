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

package inmemory

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	testDeploymentID = "test-deployment"
	testNamespace    = providers.RuntimeStoreNamespace("test-ns")
	testKey          = "key1"
)

type InMemoryStoreTestSuite struct {
	suite.Suite
	store *inMemoryStore
	ctx   context.Context
}

func TestInMemoryStoreTestSuite(t *testing.T) {
	suite.Run(t, new(InMemoryStoreTestSuite))
}

func (s *InMemoryStoreTestSuite) SetupTest() {
	s.store = &inMemoryStore{
		data:         make(map[string]*entry),
		deploymentID: testDeploymentID,
		logger:       log.GetLogger(),
	}
	s.ctx = context.Background()
}

func (s *InMemoryStoreTestSuite) TestPut_Get_RoundTrip() {
	err := s.store.Put(s.ctx, testNamespace, testKey, []byte("value"), 60)
	s.NoError(err)

	got, err := s.store.Get(s.ctx, testNamespace, testKey)
	s.NoError(err)
	s.Equal([]byte("value"), got)
}

func (s *InMemoryStoreTestSuite) TestGet_MissingKey_ReturnsNil() {
	got, err := s.store.Get(s.ctx, testNamespace, "missing")
	s.NoError(err)
	s.Nil(got)
}

func (s *InMemoryStoreTestSuite) TestGet_ExpiredKey_ReturnsNil() {
	fk := s.store.getFormattedKey(testNamespace, testKey)
	s.store.data[fk] = &entry{
		value:     []byte("stale"),
		expiresAt: time.Now().Add(-time.Second),
	}

	got, err := s.store.Get(s.ctx, testNamespace, testKey)
	s.NoError(err)
	s.Nil(got)
}

func (s *InMemoryStoreTestSuite) TestPut_ZeroTTL_NeverExpires() {
	err := s.store.Put(s.ctx, testNamespace, testKey, []byte("forever"), 0)
	s.NoError(err)

	fk := s.store.getFormattedKey(testNamespace, testKey)
	s.True(s.store.data[fk].expiresAt.IsZero())

	got, err := s.store.Get(s.ctx, testNamespace, testKey)
	s.NoError(err)
	s.Equal([]byte("forever"), got)
}

func (s *InMemoryStoreTestSuite) TestPutIfNotExists_MissingKey_Stores() {
	ok, err := s.store.PutIfNotExists(s.ctx, testNamespace, testKey, []byte("value"), 60)
	s.NoError(err)
	s.True(ok)

	got, err := s.store.Get(s.ctx, testNamespace, testKey)
	s.NoError(err)
	s.Equal([]byte("value"), got)
}

func (s *InMemoryStoreTestSuite) TestPutIfNotExists_ExistingUnexpiredKey_Rejected() {
	_, err := s.store.PutIfNotExists(s.ctx, testNamespace, testKey, []byte("first"), 60)
	s.Require().NoError(err)

	ok, err := s.store.PutIfNotExists(s.ctx, testNamespace, testKey, []byte("second"), 60)
	s.NoError(err)
	s.False(ok)

	got, err := s.store.Get(s.ctx, testNamespace, testKey)
	s.NoError(err)
	s.Equal([]byte("first"), got, "a rejected PutIfNotExists must not overwrite the existing value")
}

func (s *InMemoryStoreTestSuite) TestPutIfNotExists_ExistingExpiredKey_Overwrites() {
	fk := s.store.getFormattedKey(testNamespace, testKey)
	s.store.data[fk] = &entry{
		value:     []byte("stale"),
		expiresAt: time.Now().Add(-time.Second),
	}

	ok, err := s.store.PutIfNotExists(s.ctx, testNamespace, testKey, []byte("fresh"), 60)
	s.NoError(err)
	s.True(ok)

	got, err := s.store.Get(s.ctx, testNamespace, testKey)
	s.NoError(err)
	s.Equal([]byte("fresh"), got)
}

func (s *InMemoryStoreTestSuite) TestConcurrentPutIfNotExists() {
	const workers = 20
	var wg sync.WaitGroup
	results := make([]bool, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			ok, _ := s.store.PutIfNotExists(s.ctx, testNamespace, testKey, []byte("v"), 60)
			results[i] = ok
		}(i)
	}
	wg.Wait()

	// Exactly one goroutine should win the claim.
	wins := 0
	for _, ok := range results {
		if ok {
			wins++
		}
	}
	s.Equal(1, wins)
}

func (s *InMemoryStoreTestSuite) TestUpdate_UpdatesValuePreservesExpiry() {
	fk := s.store.getFormattedKey(testNamespace, testKey)
	expiry := time.Now().Add(time.Minute)
	s.store.data[fk] = &entry{value: []byte("old"), expiresAt: expiry}

	err := s.store.Update(s.ctx, testNamespace, testKey, []byte("new"))
	s.NoError(err)

	got, err := s.store.Get(s.ctx, testNamespace, testKey)
	s.NoError(err)
	s.Equal([]byte("new"), got)
	s.Equal(expiry.Unix(), s.store.data[fk].expiresAt.Unix())
}

func (s *InMemoryStoreTestSuite) TestUpdate_MissingKey_ReturnsError() {
	err := s.store.Update(s.ctx, testNamespace, "no-such-key", []byte("v"))
	s.Error(err)
}

func (s *InMemoryStoreTestSuite) TestUpdate_ExpiredKey_ReturnsError() {
	fk := s.store.getFormattedKey(testNamespace, testKey)
	s.store.data[fk] = &entry{
		value:     []byte("old"),
		expiresAt: time.Now().Add(-time.Second),
	}

	err := s.store.Update(s.ctx, testNamespace, testKey, []byte("new"))
	s.Error(err)
}

func (s *InMemoryStoreTestSuite) TestDelete_RemovesKey() {
	_ = s.store.Put(s.ctx, testNamespace, testKey, []byte("v"), 60)

	err := s.store.Delete(s.ctx, testNamespace, testKey)
	s.NoError(err)

	got, err := s.store.Get(s.ctx, testNamespace, testKey)
	s.NoError(err)
	s.Nil(got)
}

func (s *InMemoryStoreTestSuite) TestDelete_MissingKey_NoError() {
	err := s.store.Delete(s.ctx, testNamespace, "no-such-key")
	s.NoError(err)
}

func (s *InMemoryStoreTestSuite) TestTake_ReturnsValueAndRemoves() {
	_ = s.store.Put(s.ctx, testNamespace, testKey, []byte("take-me"), 60)

	got, err := s.store.Take(s.ctx, testNamespace, testKey)
	s.NoError(err)
	s.Equal([]byte("take-me"), got)

	// Key must no longer exist after Take.
	again, err := s.store.Get(s.ctx, testNamespace, testKey)
	s.NoError(err)
	s.Nil(again)
}

func (s *InMemoryStoreTestSuite) TestTake_MissingKey_ReturnsNil() {
	got, err := s.store.Take(s.ctx, testNamespace, "missing")
	s.NoError(err)
	s.Nil(got)
}

func (s *InMemoryStoreTestSuite) TestTake_ExpiredKey_ReturnsNil() {
	fk := s.store.getFormattedKey(testNamespace, testKey)
	s.store.data[fk] = &entry{
		value:     []byte("stale"),
		expiresAt: time.Now().Add(-time.Second),
	}

	got, err := s.store.Take(s.ctx, testNamespace, testKey)
	s.NoError(err)
	s.Nil(got)
}

func (s *InMemoryStoreTestSuite) TestExtendTTL_UpdatesExpiryPreservesValue() {
	fk := s.store.getFormattedKey(testNamespace, testKey)
	s.store.data[fk] = &entry{value: []byte("v"), expiresAt: time.Now().Add(time.Second)}

	err := s.store.ExtendTTL(s.ctx, testNamespace, testKey, 60)
	s.NoError(err)

	got, err := s.store.Get(s.ctx, testNamespace, testKey)
	s.NoError(err)
	s.Equal([]byte("v"), got)

	expected := time.Now().Add(60 * time.Second)
	s.WithinDuration(expected, s.store.data[fk].expiresAt, time.Second)
}

func (s *InMemoryStoreTestSuite) TestExtendTTL_MissingKey_ReturnsError() {
	err := s.store.ExtendTTL(s.ctx, testNamespace, "no-such-key", 60)
	s.ErrorIs(err, providers.ErrRuntimeStoreKeyNotFound)
}

func (s *InMemoryStoreTestSuite) TestExtendTTL_ExpiredKey_ReturnsError() {
	fk := s.store.getFormattedKey(testNamespace, testKey)
	s.store.data[fk] = &entry{
		value:     []byte("stale"),
		expiresAt: time.Now().Add(-time.Second),
	}

	err := s.store.ExtendTTL(s.ctx, testNamespace, testKey, 60)
	s.ErrorIs(err, providers.ErrRuntimeStoreKeyNotFound)
}

func (s *InMemoryStoreTestSuite) TestGetFormattedKey() {
	key := s.store.getFormattedKey("ns", "k")
	s.Equal("runtime:test-deployment:ns:k", key)
}

func (s *InMemoryStoreTestSuite) TestConcurrentPutGet() {
	const workers = 20
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			k := "ck"
			_ = s.store.Put(s.ctx, testNamespace, k, []byte("v"), 60)
			_, _ = s.store.Get(s.ctx, testNamespace, k)
		}()
	}
	wg.Wait()
}

func (s *InMemoryStoreTestSuite) TestConcurrentTake() {
	const workers = 10
	_ = s.store.Put(s.ctx, testNamespace, testKey, []byte("shared"), 60)

	var wg sync.WaitGroup
	wins := make([][]byte, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			v, _ := s.store.Take(s.ctx, testNamespace, testKey)
			wins[i] = v
		}(i)
	}
	wg.Wait()

	// Exactly one goroutine should win the Take.
	nonNil := 0
	for _, v := range wins {
		if v != nil {
			nonNil++
		}
	}
	s.Equal(1, nonNil)
}
