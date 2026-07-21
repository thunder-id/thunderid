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

package openid4vci

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/runtimestore/inmemory"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// errRuntimeStore is a RuntimeStoreProvider whose reads and writes always fail,
// to exercise the store-error handling paths.
type errRuntimeStore struct{}

func (errRuntimeStore) Put(context.Context, providers.RuntimeStoreNamespace, string, []byte, int64) error {
	return errors.New("store failure")
}

func (errRuntimeStore) Get(context.Context, providers.RuntimeStoreNamespace, string) ([]byte, error) {
	return nil, errors.New("store failure")
}

func (errRuntimeStore) Update(context.Context, providers.RuntimeStoreNamespace, string, []byte) error {
	return errors.New("store failure")
}

func (errRuntimeStore) Delete(context.Context, providers.RuntimeStoreNamespace, string) error {
	return errors.New("store failure")
}

func (errRuntimeStore) Take(context.Context, providers.RuntimeStoreNamespace, string) ([]byte, error) {
	return nil, errors.New("store failure")
}

func (errRuntimeStore) ExtendTTL(context.Context, providers.RuntimeStoreNamespace, string, int64) error {
	return errors.New("store failure")
}

// OpenID4VCIStoreTestSuite exercises the openID4VCIStore adapter against a real in-memory
// runtime store, verifying the marshal/namespace/key round-trip and not-found semantics.
type OpenID4VCIStoreTestSuite struct {
	suite.Suite
	store openID4VCIStoreInterface
	ctx   context.Context
}

func TestOpenID4VCIStoreTestSuite(t *testing.T) {
	suite.Run(t, new(OpenID4VCIStoreTestSuite))
}

func (suite *OpenID4VCIStoreTestSuite) SetupTest() {
	suite.store = newOpenID4VCIStore(inmemory.Initialize("test-deployment"))
	suite.ctx = context.Background()
}

func (suite *OpenID4VCIStoreTestSuite) TestNonceRoundTrip() {
	expiry := time.Now().Add(time.Minute)
	suite.Require().NoError(suite.store.SaveNonce(suite.ctx, "n1", &nonceRecord{ExpiresAt: expiry}))

	rec, ok := suite.store.GetNonce(suite.ctx, "n1")
	suite.Require().True(ok)
	suite.Require().NotNil(rec)
	suite.WithinDuration(expiry, rec.ExpiresAt, time.Second)
}

func (suite *OpenID4VCIStoreTestSuite) TestGetNonceNotFound() {
	rec, ok := suite.store.GetNonce(suite.ctx, "missing")
	suite.False(ok)
	suite.Nil(rec)
}

func (suite *OpenID4VCIStoreTestSuite) TestDeleteNonce() {
	suite.Require().NoError(
		suite.store.SaveNonce(suite.ctx, "n1", &nonceRecord{ExpiresAt: time.Now().Add(time.Minute)}))
	suite.Require().NoError(suite.store.DeleteNonce(suite.ctx, "n1"))

	_, ok := suite.store.GetNonce(suite.ctx, "n1")
	suite.False(ok)
}

func (suite *OpenID4VCIStoreTestSuite) TestDeleteNonceIdempotent() {
	suite.NoError(suite.store.DeleteNonce(suite.ctx, "missing"))
}

func (suite *OpenID4VCIStoreTestSuite) TestOfferRoundTrip() {
	expiry := time.Now().Add(time.Minute)
	rec := &offerRecord{Offer: map[string]interface{}{"k": "v"}, ExpiresAt: expiry}
	suite.Require().NoError(suite.store.SaveOffer(suite.ctx, "o1", rec))

	got, ok := suite.store.GetOffer(suite.ctx, "o1")
	suite.Require().True(ok)
	suite.Require().NotNil(got)
	suite.Equal("v", got.Offer["k"])
	suite.WithinDuration(expiry, got.ExpiresAt, time.Second)
}

func (suite *OpenID4VCIStoreTestSuite) TestGetOfferNotFound() {
	got, ok := suite.store.GetOffer(suite.ctx, "missing")
	suite.False(ok)
	suite.Nil(got)
}

func (suite *OpenID4VCIStoreTestSuite) TestSaveOfferMarshalError() {
	rec := &offerRecord{
		Offer:     map[string]interface{}{"bad": make(chan int)},
		ExpiresAt: time.Now().Add(time.Minute),
	}
	suite.Error(suite.store.SaveOffer(suite.ctx, "o1", rec))
}

func (suite *OpenID4VCIStoreTestSuite) TestTTLUntil() {
	suite.Equal(int64(1), ttlUntil(time.Now().Add(-time.Minute)))
	suite.GreaterOrEqual(ttlUntil(time.Now().Add(90*time.Second)), int64(90))
}

// A runtime-store error on read is logged and reported as not-found.
func (suite *OpenID4VCIStoreTestSuite) TestGetNonceStoreError() {
	rec, ok := newOpenID4VCIStore(errRuntimeStore{}).GetNonce(suite.ctx, "n1")
	suite.False(ok)
	suite.Nil(rec)
}

// A malformed stored record is logged and reported as not-found.
func (suite *OpenID4VCIStoreTestSuite) TestGetNonceUnmarshalError() {
	prov := inmemory.Initialize("test-deployment")
	suite.Require().NoError(prov.Put(suite.ctx, providers.NamespaceVCINonce, "bad", []byte("not-json"), 60))
	rec, ok := newOpenID4VCIStore(prov).GetNonce(suite.ctx, "bad")
	suite.False(ok)
	suite.Nil(rec)
}

func (suite *OpenID4VCIStoreTestSuite) TestGetOfferStoreError() {
	rec, ok := newOpenID4VCIStore(errRuntimeStore{}).GetOffer(suite.ctx, "o1")
	suite.False(ok)
	suite.Nil(rec)
}

func (suite *OpenID4VCIStoreTestSuite) TestGetOfferUnmarshalError() {
	prov := inmemory.Initialize("test-deployment")
	suite.Require().NoError(prov.Put(suite.ctx, providers.NamespaceVCIOffer, "bad", []byte("not-json"), 60))
	rec, ok := newOpenID4VCIStore(prov).GetOffer(suite.ctx, "bad")
	suite.False(ok)
	suite.Nil(rec)
}
