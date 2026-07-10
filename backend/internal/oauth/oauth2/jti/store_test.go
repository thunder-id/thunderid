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

package jti

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/runtimestore/inmemory"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// erroringStoreProvider is a minimal RuntimeStoreProvider fake used to exercise the
// backend-failure branches of RecordJTI, which a real store cannot be made to fail on demand.
type erroringStoreProvider struct {
	providers.RuntimeStoreProvider
	getErr error
	putErr error
}

func (f *erroringStoreProvider) Get(
	_ context.Context, _ providers.RuntimeStoreNamespace, _ string,
) ([]byte, error) {
	return nil, f.getErr
}

func (f *erroringStoreProvider) Put(
	_ context.Context, _ providers.RuntimeStoreNamespace, _ string, _ []byte, _ int64,
) error {
	return f.putErr
}

// JTIStoreTestSuite exercises the jtiStore adapter against a real in-memory runtime store,
// verifying the insert/replay/namespace-isolation semantics.
type JTIStoreTestSuite struct {
	suite.Suite
	store *jtiStore
	ctx   context.Context
}

func TestJTIStoreTestSuite(t *testing.T) {
	suite.Run(t, new(JTIStoreTestSuite))
}

func (suite *JTIStoreTestSuite) SetupTest() {
	suite.store = &jtiStore{storeProvider: inmemory.Initialize("test-deployment")}
	suite.ctx = context.Background()
}

func (suite *JTIStoreTestSuite) TestRecordJTI_Inserted() {
	inserted, err := suite.store.RecordJTI(suite.ctx, "dpop", "jti-1", time.Now().Add(time.Minute))
	suite.Require().NoError(err)
	suite.True(inserted)
}

func (suite *JTIStoreTestSuite) TestRecordJTI_Replay() {
	expiry := time.Now().Add(time.Minute)
	inserted, err := suite.store.RecordJTI(suite.ctx, "dpop", "jti-1", expiry)
	suite.Require().NoError(err)
	suite.True(inserted)

	inserted, err = suite.store.RecordJTI(suite.ctx, "dpop", "jti-1", expiry)
	suite.Require().NoError(err)
	suite.False(inserted, "a repeated jti within the same namespace must be reported as a replay")
}

// TestRecordJTI_NamespaceIsolation locks in the contract that two distinct namespaces can carry
// the same jti without colliding — i.e. namespace participates in the store key.
func (suite *JTIStoreTestSuite) TestRecordJTI_NamespaceIsolation() {
	expiry := time.Now().Add(time.Minute)

	ok1, err := suite.store.RecordJTI(suite.ctx, "dpop", "j", expiry)
	suite.Require().NoError(err)
	suite.True(ok1)

	ok2, err := suite.store.RecordJTI(suite.ctx, "client_assertion", "j", expiry)
	suite.Require().NoError(err)
	suite.True(ok2, "the same jti under a different namespace must not be treated as a replay")
}

func (suite *JTIStoreTestSuite) TestRecordJTI_GetError() {
	store := &jtiStore{storeProvider: &erroringStoreProvider{getErr: errors.New("conn failed")}}

	inserted, err := store.RecordJTI(suite.ctx, "dpop", "jti-1", time.Now().Add(time.Minute))
	suite.Require().Error(err)
	suite.False(inserted)
	suite.Contains(err.Error(), "failed to check jti")
}

func (suite *JTIStoreTestSuite) TestRecordJTI_PutError() {
	store := &jtiStore{storeProvider: &erroringStoreProvider{putErr: errors.New("insert failed")}}

	inserted, err := store.RecordJTI(suite.ctx, "dpop", "jti-1", time.Now().Add(time.Minute))
	suite.Require().Error(err)
	suite.False(inserted)
	suite.Contains(err.Error(), "failed to insert jti")
}
