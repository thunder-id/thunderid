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
	"crypto/x509"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

type OpenID4VPStoreTestSuite struct {
	suite.Suite
}

func TestOpenID4VPStoreTestSuite(t *testing.T) {
	suite.Run(t, new(OpenID4VPStoreTestSuite))
}

// identityCrypto is a no-op ConfigCryptoProvider for tests: it exercises the
// marshal/encrypt/decrypt/parse path without a real symmetric key.
type identityCrypto struct{}

func (identityCrypto) Encrypt(_ context.Context, content []byte) ([]byte, error) { return content, nil }
func (identityCrypto) Decrypt(_ context.Context, content []byte) ([]byte, error) { return content, nil }

// The runtime store read path reconstructs a RequestState from a result row,
// decrypting the ephemeral key and decoding the verification result.
func (suite *OpenID4VPStoreTestSuite) TestOpenID4VPStoreReadPathRoundTrip() {
	crypto := identityCrypto{}
	store := &openID4VPStore{deploymentID: "test", crypto: crypto}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	suite.Require().NoError(err)
	pkcs8, err := x509.MarshalPKCS8PrivateKey(key)
	suite.Require().NoError(err)
	encKey, err := crypto.Encrypt(context.Background(), pkcs8)
	suite.Require().NoError(err)

	vp := &VerifiedPresentation{
		Subject: "sub-1", VCT: "urn:eudi:pid:de:1",
		Claims: map[string]interface{}{"given_name": "Erika"},
	}
	resultJSON, err := json.Marshal(vp)
	suite.Require().NoError(err)

	expiry := time.Now().Add(time.Minute).UTC()
	row := map[string]interface{}{
		"state":          "state-1",
		"definition_id":  "eudi-pid",
		"nonce":          "nonce-1",
		"ephemeral_key":  encKey,
		"client_id":      "x509_hash:abc",
		"rp_id":          "rp-1",
		"request_uri":    "https://verifier.example/openid4vp/request?state=state-1",
		"status":         "COMPLETED",
		"result":         resultJSON,
		"failure_reason": "",
		"expiry_time":    expiry,
	}

	rs, err := store.buildRequestStateFromRow(context.Background(), row)
	suite.Require().NoError(err)
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

// A row with no ephemeral key and no result yields a state with nil fields.
func (suite *OpenID4VPStoreTestSuite) TestOpenID4VPStoreReadPathPending() {
	store := &openID4VPStore{deploymentID: "test", crypto: identityCrypto{}}
	row := map[string]interface{}{
		"state":       "state-2",
		"status":      "PENDING",
		"expiry_time": time.Now().Add(time.Minute).UTC(),
	}
	rs, err := store.buildRequestStateFromRow(context.Background(), row)
	suite.Require().NoError(err)
	suite.Equal(StatusPending, rs.Status)
	suite.Nil(rs.EphemeralKey)
	suite.Nil(rs.Result)
}

// parseStateTime handles Postgres time.Time and SQLite datetime strings.
func (suite *OpenID4VPStoreTestSuite) TestParseStateTime() {
	now := time.Now().UTC().Truncate(time.Second)

	got, err := parseStateTime(now)
	suite.Require().NoError(err)
	suite.True(now.Equal(got))

	got, err = parseStateTime(now.Format(time.RFC3339))
	suite.Require().NoError(err)
	suite.True(now.Equal(got))

	got, err = parseStateTime(now.Format("2006-01-02 15:04:05"))
	suite.Require().NoError(err)
	suite.Equal(now.Format("2006-01-02 15:04:05"), got.Format("2006-01-02 15:04:05"))

	_, err = parseStateTime([]byte(now.Format(time.RFC3339)))
	suite.Require().NoError(err)

	_, err = parseStateTime(123)
	suite.Require().Error(err)
}

// newOpenID4VPStore builds a database-backed store using the deployment ID from
// the server runtime configuration.
func (suite *OpenID4VPStoreTestSuite) TestNewOpenID4VPStore() {
	config.ResetServerRuntime()
	cfg := &config.Config{}
	cfg.Server.Identifier = "deployment-xyz"
	suite.Require().NoError(config.InitializeServerRuntime("", cfg))
	defer config.ResetServerRuntime()

	st := newOpenID4VPStore(identityCrypto{})
	suite.Require().NotNil(st)
	impl, ok := st.(*openID4VPStore)
	suite.Require().True(ok)
	suite.Equal("deployment-xyz", impl.deploymentID)
	suite.NotNil(impl.dbProvider)
}

func (suite *OpenID4VPStoreTestSuite) newDBMockedStore() (
	*openID4VPStore, *providermock.DBProviderInterfaceMock, *providermock.DBClientInterfaceMock,
) {
	mockProvider := providermock.NewDBProviderInterfaceMock(suite.T())
	mockClient := providermock.NewDBClientInterfaceMock(suite.T())
	store := &openID4VPStore{
		dbProvider:   mockProvider,
		deploymentID: "test-deployment",
		crypto:       identityCrypto{},
	}
	return store, mockProvider, mockClient
}

func (suite *OpenID4VPStoreTestSuite) TestSaveRequestState() {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	suite.Require().NoError(err)

	suite.Run("Success", func() {
		store, provider, client := suite.newDBMockedStore()
		provider.On("GetRuntimeDBClient").Return(client, nil)
		client.On("ExecuteContext", mock.Anything, queryUpsertRequestState,
			mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything,
		).Return(int64(1), nil)

		st := &RequestState{
			State:        "state-1",
			DefinitionID: "eudi-pid",
			Nonce:        "nonce-1",
			EphemeralKey: key,
			ClientID:     "x509_hash:abc",
			RPID:         "rp-1",
			RequestURI:   "https://verifier.example/request",
			Status:       StatusCompleted,
			Result:       &VerifiedPresentation{Subject: "sub-1"},
			ExpiresAt:    time.Now().Add(time.Minute),
		}
		suite.NoError(store.SaveRequestState(context.Background(), st))
	})

	suite.Run("SuccessNoKeyNoResult", func() {
		store, provider, client := suite.newDBMockedStore()
		provider.On("GetRuntimeDBClient").Return(client, nil)
		client.On("ExecuteContext", mock.Anything, queryUpsertRequestState,
			"state-2", "test-deployment", "", "", []byte(nil), "", "", "",
			string(StatusPending), []byte(nil), "", mock.Anything,
		).Return(int64(1), nil)

		st := &RequestState{State: "state-2", Status: StatusPending, ExpiresAt: time.Now()}
		suite.NoError(store.SaveRequestState(context.Background(), st))
	})

	suite.Run("DBClientError", func() {
		store, provider, _ := suite.newDBMockedStore()
		provider.On("GetRuntimeDBClient").Return(nil, errors.New("db error"))
		err := store.SaveRequestState(context.Background(), &RequestState{State: "s"})
		suite.Error(err)
	})

	suite.Run("ExecuteError", func() {
		store, provider, client := suite.newDBMockedStore()
		provider.On("GetRuntimeDBClient").Return(client, nil)
		client.On("ExecuteContext", mock.Anything, queryUpsertRequestState,
			mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything,
		).Return(int64(0), errors.New("upsert failed"))
		err := store.SaveRequestState(context.Background(),
			&RequestState{State: "s", ExpiresAt: time.Now()})
		suite.Error(err)
	})
}

func (suite *OpenID4VPStoreTestSuite) TestGetRequestState() {
	suite.Run("Success", func() {
		store, provider, client := suite.newDBMockedStore()
		expiry := time.Now().Add(time.Minute).UTC()
		provider.On("GetRuntimeDBClient").Return(client, nil)
		client.On("QueryContext", mock.Anything, queryGetRequestState,
			"state-1", "test-deployment",
		).Return([]map[string]interface{}{{
			"state":       "state-1",
			"status":      "PENDING",
			"expiry_time": expiry,
		}}, nil)

		rs, ok := store.GetRequestState(context.Background(), "state-1")
		suite.True(ok)
		suite.Require().NotNil(rs)
		suite.Equal("state-1", rs.State)
		suite.Equal(StatusPending, rs.Status)
	})

	suite.Run("NotFound", func() {
		store, provider, client := suite.newDBMockedStore()
		provider.On("GetRuntimeDBClient").Return(client, nil)
		client.On("QueryContext", mock.Anything, queryGetRequestState,
			"missing", "test-deployment",
		).Return([]map[string]interface{}{}, nil)

		rs, ok := store.GetRequestState(context.Background(), "missing")
		suite.False(ok)
		suite.Nil(rs)
	})

	suite.Run("DBClientError", func() {
		store, provider, _ := suite.newDBMockedStore()
		provider.On("GetRuntimeDBClient").Return(nil, errors.New("db error"))
		rs, ok := store.GetRequestState(context.Background(), "state-1")
		suite.False(ok)
		suite.Nil(rs)
	})

	suite.Run("QueryError", func() {
		store, provider, client := suite.newDBMockedStore()
		provider.On("GetRuntimeDBClient").Return(client, nil)
		client.On("QueryContext", mock.Anything, queryGetRequestState,
			"state-1", "test-deployment",
		).Return(nil, errors.New("query failed"))
		rs, ok := store.GetRequestState(context.Background(), "state-1")
		suite.False(ok)
		suite.Nil(rs)
	})

	suite.Run("BuildError", func() {
		store, provider, client := suite.newDBMockedStore()
		provider.On("GetRuntimeDBClient").Return(client, nil)
		client.On("QueryContext", mock.Anything, queryGetRequestState,
			"state-1", "test-deployment",
		).Return([]map[string]interface{}{{
			"state":       "state-1",
			"status":      "PENDING",
			"expiry_time": "not-a-valid-time",
		}}, nil)
		rs, ok := store.GetRequestState(context.Background(), "state-1")
		suite.False(ok)
		suite.Nil(rs)
	})
}

func (suite *OpenID4VPStoreTestSuite) TestDeleteRequestState() {
	suite.Run("Success", func() {
		store, provider, client := suite.newDBMockedStore()
		provider.On("GetRuntimeDBClient").Return(client, nil)
		client.On("ExecuteContext", mock.Anything, queryDeleteRequestState,
			"state-1", "test-deployment",
		).Return(int64(1), nil)
		suite.NoError(store.DeleteRequestState(context.Background(), "state-1"))
	})

	suite.Run("DBClientError", func() {
		store, provider, _ := suite.newDBMockedStore()
		provider.On("GetRuntimeDBClient").Return(nil, errors.New("db error"))
		suite.Error(store.DeleteRequestState(context.Background(), "state-1"))
	})

	suite.Run("ExecuteError", func() {
		store, provider, client := suite.newDBMockedStore()
		provider.On("GetRuntimeDBClient").Return(client, nil)
		client.On("ExecuteContext", mock.Anything, queryDeleteRequestState,
			"state-1", "test-deployment",
		).Return(int64(0), errors.New("delete failed"))
		suite.Error(store.DeleteRequestState(context.Background(), "state-1"))
	})
}
