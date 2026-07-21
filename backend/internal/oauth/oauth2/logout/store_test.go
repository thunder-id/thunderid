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

package logout

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/runtimestore/inmemory"
)

// LogoutRequestStoreTestSuite exercises the runtime-store-backed logout request store against the
// in-memory runtime store backend, which shares the RuntimeStoreProvider contract with the Redis and
// relational backends.
type LogoutRequestStoreTestSuite struct {
	suite.Suite
	store logoutRequestStoreInterface
}

func TestLogoutRequestStoreTestSuite(t *testing.T) {
	suite.Run(t, new(LogoutRequestStoreTestSuite))
}

func (suite *LogoutRequestStoreTestSuite) SetupTest() {
	suite.store = newLogoutRequestStore(inmemory.Initialize("test-deployment"))
}

func (suite *LogoutRequestStoreTestSuite) TestAddThenGetRoundTrips() {
	want := logoutRequestContext{
		AppID:                 "app-1",
		PostLogoutRedirectURI: "https://rp.example/after",
		State:                 "xyz",
	}
	id, err := suite.store.AddRequest(context.Background(), want)
	suite.Require().NoError(err)
	suite.Require().NotEmpty(id)

	found, got, err := suite.store.GetRequest(context.Background(), id)
	suite.Require().NoError(err)
	suite.True(found)
	suite.Equal(want, got)
}

func (suite *LogoutRequestStoreTestSuite) TestGetUnknownKeyReportsNotFound() {
	found, _, err := suite.store.GetRequest(context.Background(), "does-not-exist")
	suite.Require().NoError(err)
	suite.False(found)
}

func (suite *LogoutRequestStoreTestSuite) TestGetEmptyKeyReportsNotFound() {
	found, _, err := suite.store.GetRequest(context.Background(), "")
	suite.Require().NoError(err)
	suite.False(found)
}

func (suite *LogoutRequestStoreTestSuite) TestClearRemovesEntry() {
	id, err := suite.store.AddRequest(context.Background(), logoutRequestContext{AppID: "app-1"})
	suite.Require().NoError(err)

	suite.Require().NoError(suite.store.ClearRequest(context.Background(), id))

	found, _, err := suite.store.GetRequest(context.Background(), id)
	suite.Require().NoError(err)
	suite.False(found, "a cleared logout request must not be retrievable")
}

func (suite *LogoutRequestStoreTestSuite) TestClearEmptyKeyIsNoOp() {
	suite.Require().NoError(suite.store.ClearRequest(context.Background(), ""))
}

// The following cases drive the underlying runtime-store failure paths via a mock backend.

func (suite *LogoutRequestStoreTestSuite) TestAddRequest_PutError() {
	rt := NewRuntimeStoreProviderMock(suite.T())
	rt.EXPECT().Put(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fmt.Errorf("put failed"))
	store := newLogoutRequestStore(rt)

	_, err := store.AddRequest(context.Background(), logoutRequestContext{AppID: "app-1"})

	suite.Require().Error(err)
}

func (suite *LogoutRequestStoreTestSuite) TestGetRequest_StoreError() {
	rt := NewRuntimeStoreProviderMock(suite.T())
	rt.EXPECT().Get(mock.Anything, mock.Anything, "k").Return(nil, fmt.Errorf("get failed"))
	store := newLogoutRequestStore(rt)

	found, _, err := store.GetRequest(context.Background(), "k")

	suite.Require().Error(err)
	suite.False(found)
}

func (suite *LogoutRequestStoreTestSuite) TestGetRequest_UnmarshalError() {
	rt := NewRuntimeStoreProviderMock(suite.T())
	rt.EXPECT().Get(mock.Anything, mock.Anything, "k").Return([]byte("not-json"), nil)
	store := newLogoutRequestStore(rt)

	found, _, err := store.GetRequest(context.Background(), "k")

	suite.Require().Error(err)
	suite.False(found)
}

func (suite *LogoutRequestStoreTestSuite) TestClearRequest_DeleteError() {
	rt := NewRuntimeStoreProviderMock(suite.T())
	rt.EXPECT().Delete(mock.Anything, mock.Anything, "k").Return(fmt.Errorf("delete failed"))
	store := newLogoutRequestStore(rt)

	err := store.ClearRequest(context.Background(), "k")

	suite.Require().Error(err)
}
