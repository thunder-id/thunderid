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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/runtimestore/inmemory"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// identityCrypto is a no-op ConfigCryptoProvider for tests: it exercises the
// marshal/encrypt/decrypt/parse path without a real symmetric key.
type identityCrypto struct{}

func (identityCrypto) Encrypt(_ context.Context, content []byte) ([]byte, error) { return content, nil }
func (identityCrypto) Decrypt(_ context.Context, content []byte) ([]byte, error) { return content, nil }

// failingDecryptCrypto encrypts as identity but fails to decrypt, to exercise the read error path.
type failingDecryptCrypto struct{ identityCrypto }

func (failingDecryptCrypto) Decrypt(_ context.Context, _ []byte) ([]byte, error) {
	return nil, errors.New("decrypt failed")
}

// failingEncryptCrypto fails to encrypt, to exercise the write error path.
type failingEncryptCrypto struct{ identityCrypto }

func (failingEncryptCrypto) Encrypt(_ context.Context, _ []byte) ([]byte, error) {
	return nil, errors.New("encrypt failed")
}

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

// OpenID4VPStoreTestSuite exercises the openID4VPStore adapter against a real in-memory
// runtime store, verifying the encrypt/marshal/namespace round-trip and not-found semantics.
type OpenID4VPStoreTestSuite struct {
	suite.Suite
	store openID4VPStoreInterface
	ctx   context.Context
}

func TestOpenID4VPStoreTestSuite(t *testing.T) {
	suite.Run(t, new(OpenID4VPStoreTestSuite))
}

func (suite *OpenID4VPStoreTestSuite) SetupTest() {
	suite.store = newOpenID4VPStore(identityCrypto{}, inmemory.Initialize("test-deployment"))
	suite.ctx = context.Background()
}

func (suite *OpenID4VPStoreTestSuite) TestRoundTripCompleted() {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	suite.Require().NoError(err)

	expiry := time.Now().Add(time.Minute)
	st := &RequestState{
		State:        "state-1",
		DefinitionID: "eudi-pid",
		Nonce:        "nonce-1",
		EphemeralKey: key,
		ClientID:     "x509_hash:abc",
		RequestURI:   "https://verifier.example/openid4vp/request?state=state-1",
		Status:       StatusCompleted,
		Result:       &VerifiedPresentation{Subject: "sub-1", Claims: map[string]interface{}{"given_name": "Erika"}},
		ExpiresAt:    expiry,
	}
	suite.Require().NoError(suite.store.SaveRequestState(suite.ctx, st))

	rs, ok := suite.store.GetRequestState(suite.ctx, "state-1")
	suite.Require().True(ok)
	suite.Require().NotNil(rs)
	suite.Equal("state-1", rs.State)
	suite.Equal("eudi-pid", rs.DefinitionID)
	suite.Equal("nonce-1", rs.Nonce)
	suite.Equal(StatusCompleted, rs.Status)
	suite.Require().NotNil(rs.EphemeralKey)
	suite.True(key.Equal(rs.EphemeralKey))
	suite.Require().NotNil(rs.Result)
	suite.Equal("Erika", rs.Result.Claims["given_name"])
	suite.WithinDuration(expiry, rs.ExpiresAt, time.Second)
}

func (suite *OpenID4VPStoreTestSuite) TestRoundTripPending() {
	st := &RequestState{State: "state-2", Status: StatusPending, ExpiresAt: time.Now().Add(time.Minute)}
	suite.Require().NoError(suite.store.SaveRequestState(suite.ctx, st))

	rs, ok := suite.store.GetRequestState(suite.ctx, "state-2")
	suite.Require().True(ok)
	suite.Equal(StatusPending, rs.Status)
	suite.Nil(rs.EphemeralKey)
	suite.Nil(rs.Result)
}

// SaveRequestState overwrites the existing entry, so status transitions are persisted.
func (suite *OpenID4VPStoreTestSuite) TestUpsertOverwrites() {
	st := &RequestState{State: "state-3", Status: StatusPending, ExpiresAt: time.Now().Add(time.Minute)}
	suite.Require().NoError(suite.store.SaveRequestState(suite.ctx, st))

	st.Status = StatusFailed
	st.FailureReason = "untrusted_issuer"
	suite.Require().NoError(suite.store.SaveRequestState(suite.ctx, st))

	rs, ok := suite.store.GetRequestState(suite.ctx, "state-3")
	suite.Require().True(ok)
	suite.Equal(StatusFailed, rs.Status)
	suite.Equal("untrusted_issuer", rs.FailureReason)
}

func (suite *OpenID4VPStoreTestSuite) TestGetNotFound() {
	rs, ok := suite.store.GetRequestState(suite.ctx, "missing")
	suite.False(ok)
	suite.Nil(rs)
}

func (suite *OpenID4VPStoreTestSuite) TestDelete() {
	st := &RequestState{State: "state-4", Status: StatusPending, ExpiresAt: time.Now().Add(time.Minute)}
	suite.Require().NoError(suite.store.SaveRequestState(suite.ctx, st))
	suite.Require().NoError(suite.store.DeleteRequestState(suite.ctx, "state-4"))

	_, ok := suite.store.GetRequestState(suite.ctx, "state-4")
	suite.False(ok)
}

// A decrypt failure on read yields not-found rather than a partially built state.
func (suite *OpenID4VPStoreTestSuite) TestReadDecryptError() {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	suite.Require().NoError(err)

	store := newOpenID4VPStore(failingDecryptCrypto{}, inmemory.Initialize("test-deployment"))
	st := &RequestState{
		State: "state-5", EphemeralKey: key, Status: StatusPending, ExpiresAt: time.Now().Add(time.Minute),
	}
	suite.Require().NoError(store.SaveRequestState(suite.ctx, st))

	rs, ok := store.GetRequestState(suite.ctx, "state-5")
	suite.False(ok)
	suite.Nil(rs)
}

// A runtime-store error on write is surfaced as an error.
func (suite *OpenID4VPStoreTestSuite) TestSaveStoreError() {
	store := newOpenID4VPStore(identityCrypto{}, errRuntimeStore{})
	st := &RequestState{State: "s", Status: StatusPending, ExpiresAt: time.Now().Add(time.Minute)}
	suite.Error(store.SaveRequestState(suite.ctx, st))
}

// An ephemeral-key encryption failure is surfaced as an error.
func (suite *OpenID4VPStoreTestSuite) TestSaveEncryptError() {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	suite.Require().NoError(err)
	store := newOpenID4VPStore(failingEncryptCrypto{}, inmemory.Initialize("test-deployment"))
	st := &RequestState{State: "s", EphemeralKey: key, Status: StatusPending, ExpiresAt: time.Now().Add(time.Minute)}
	suite.Error(store.SaveRequestState(suite.ctx, st))
}

// A state whose result cannot be marshaled is surfaced as an error.
func (suite *OpenID4VPStoreTestSuite) TestSaveMarshalError() {
	store := newOpenID4VPStore(identityCrypto{}, inmemory.Initialize("test-deployment"))
	st := &RequestState{
		State: "s", Status: StatusPending, ExpiresAt: time.Now().Add(time.Minute),
		Result: &VerifiedPresentation{Claims: map[string]interface{}{"bad": make(chan int)}},
	}
	suite.Error(store.SaveRequestState(suite.ctx, st))
}

// A runtime-store error on read is logged and reported as not-found.
func (suite *OpenID4VPStoreTestSuite) TestGetStoreError() {
	store := newOpenID4VPStore(identityCrypto{}, errRuntimeStore{})
	rs, ok := store.GetRequestState(suite.ctx, "s")
	suite.False(ok)
	suite.Nil(rs)
}

// A malformed stored record is logged and reported as not-found.
func (suite *OpenID4VPStoreTestSuite) TestGetUnmarshalError() {
	prov := inmemory.Initialize("test-deployment")
	suite.Require().NoError(prov.Put(suite.ctx, providers.NamespaceVPState, "bad", []byte("not-json"), 60))
	rs, ok := newOpenID4VPStore(identityCrypto{}, prov).GetRequestState(suite.ctx, "bad")
	suite.False(ok)
	suite.Nil(rs)
}

// A stored ephemeral key that is not valid PKCS#8 is reported as not-found.
func (suite *OpenID4VPStoreTestSuite) TestGetBadEphemeralKey() {
	prov := inmemory.Initialize("test-deployment")
	blob := []byte(`{"State":"s","EphemeralKey":"bm90LWtleQ==","Status":"PENDING"}`)
	suite.Require().NoError(prov.Put(suite.ctx, providers.NamespaceVPState, "s", blob, 60))
	rs, ok := newOpenID4VPStore(identityCrypto{}, prov).GetRequestState(suite.ctx, "s")
	suite.False(ok)
	suite.Nil(rs)
}

// A stored ephemeral key that is valid PKCS#8 but not an EC key is reported as not-found.
func (suite *OpenID4VPStoreTestSuite) TestGetNonECEphemeralKey() {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	suite.Require().NoError(err)
	pkcs8, err := x509.MarshalPKCS8PrivateKey(rsaKey)
	suite.Require().NoError(err)
	blob, err := json.Marshal(storedRequestState{State: "s", Status: StatusPending, EphemeralKey: pkcs8})
	suite.Require().NoError(err)

	prov := inmemory.Initialize("test-deployment")
	suite.Require().NoError(prov.Put(suite.ctx, providers.NamespaceVPState, "s", blob, 60))
	rs, ok := newOpenID4VPStore(identityCrypto{}, prov).GetRequestState(suite.ctx, "s")
	suite.False(ok)
	suite.Nil(rs)
}

func (suite *OpenID4VPStoreTestSuite) TestTTLUntil() {
	suite.Equal(int64(1), ttlUntil(time.Now().Add(-time.Minute)))
	suite.GreaterOrEqual(ttlUntil(time.Now().Add(90*time.Second)), int64(90))
}
