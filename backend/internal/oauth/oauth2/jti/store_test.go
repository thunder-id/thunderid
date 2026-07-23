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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/runtimestore/inmemory"
)

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

func (suite *JTIStoreTestSuite) TestRecordJTI_AlreadyExpired() {
	inserted, err := suite.store.RecordJTI(suite.ctx, "dpop", "jti-1", time.Now().Add(-time.Minute))
	suite.Require().NoError(err)
	suite.True(inserted, "an already-expired jti needs no replay tracking and must not be reported as a replay")
}

func (suite *JTIStoreTestSuite) TestRecordJTI_PutIfNotExistsError() {
	rt := NewRuntimeStoreProviderMock(suite.T())
	rt.EXPECT().PutIfNotExists(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(false, fmt.Errorf("insert failed"))
	store := &jtiStore{storeProvider: rt}

	inserted, err := store.RecordJTI(suite.ctx, "dpop", "jti-1", time.Now().Add(time.Minute))
	suite.Require().Error(err)
	suite.False(inserted)
	suite.Contains(err.Error(), "failed to insert jti")
}
