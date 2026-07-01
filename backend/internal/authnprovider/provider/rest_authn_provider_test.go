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

package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	sysContext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/tests/mocks/httpmock"
)

type RestAuthnProviderTestSuite struct {
	suite.Suite
}

func TestRestAuthnProviderTestSuite(t *testing.T) {
	suite.Run(t, new(RestAuthnProviderTestSuite))
}

func (suite *RestAuthnProviderTestSuite) setupMockClient() *httpmock.HTTPClientInterfaceMock {
	client := httpmock.NewHTTPClientInterfaceMock(suite.T())
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		return http.DefaultClient.Do(req)
	})
	return client
}

func (suite *RestAuthnProviderTestSuite) initRuntime(rest config.RestConfig) {
	cfg := &config.Config{
		AuthnProvider: config.AuthnProviderConfig{Type: "rest", Rest: rest},
	}
	config.ResetServerRuntime()
	suite.Require().NoError(config.InitializeServerRuntime("/tmp/test", cfg))
	suite.T().Cleanup(config.ResetServerRuntime)
}

func (suite *RestAuthnProviderTestSuite) TestInitializeRestAuthnProvider_DefaultCorrelationHeader() {
	suite.initRuntime(config.RestConfig{BaseURL: "https://authn.example.com"})

	provider := initializeRestAuthnProvider().(*restAuthnProvider)

	suite.Equal(serverconst.CorrelationIDHeaderName, provider.correlationIDHeader)
}

func (suite *RestAuthnProviderTestSuite) TestInitializeRestAuthnProvider_ConfiguredCorrelationHeader() {
	suite.initRuntime(config.RestConfig{
		BaseURL:             "https://authn.example.com",
		CorrelationIDHeader: "X-Trace-Token",
	})

	provider := initializeRestAuthnProvider().(*restAuthnProvider)

	suite.Equal("X-Trace-Token", provider.correlationIDHeader)
}

func (suite *RestAuthnProviderTestSuite) TestAuthenticate_Success() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		suite.Equal("/authenticate", r.URL.Path)
		suite.Equal(http.MethodPost, r.Method)
		suite.Equal("apikey123", r.Header.Get("API-KEY"))

		var req AuthenticateRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		suite.Equal("user", req.Identifiers["username"])
		suite.Equal("pass", req.Credentials["password"])

		resp := providers.AuthnResult{
			EntityReference: &providers.EntityReference{
				EntityID:       "user123",
				EntityCategory: "user",
				EntityType:     "customer",
				OUID:           "ou1",
			},
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	provider := newRestAuthnProvider(ts.URL, "apikey123", "X-Correlation-ID", suite.setupMockClient())
	identifiers := map[string]interface{}{"username": "user"}
	credentials := map[string]interface{}{"password": "pass"}

	result, err := provider.Authenticate(context.Background(), identifiers, credentials, nil)

	suite.Nil(err)
	suite.NotNil(result.EntityReference)
	suite.Equal("user123", result.EntityReference.EntityID)
	suite.Equal("customer", result.EntityReference.EntityType)
}

func (suite *RestAuthnProviderTestSuite) TestAuthenticate_PropagatesCorrelationID() {
	var gotCorrelationID string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCorrelationID = r.Header.Get("X-Correlation-ID")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(providers.AuthnResult{})
	}))
	defer ts.Close()

	provider := newRestAuthnProvider(ts.URL, "", "X-Correlation-ID", suite.setupMockClient())
	ctx := sysContext.WithTraceID(context.Background(), "trace-xyz")

	_, err := provider.Authenticate(ctx, map[string]interface{}{"username": "user"},
		map[string]interface{}{"password": "pass"}, nil)

	suite.Nil(err)
	suite.Equal("trace-xyz", gotCorrelationID)
}

func (suite *RestAuthnProviderTestSuite) TestAuthenticate_PropagatesConfiguredCorrelationIDHeader() {
	var defaultHeader, customHeader string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defaultHeader = r.Header.Get("X-Correlation-ID")
		customHeader = r.Header.Get("X-Trace-Token")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(providers.AuthnResult{})
	}))
	defer ts.Close()

	provider := newRestAuthnProvider(ts.URL, "", "X-Trace-Token", suite.setupMockClient())
	ctx := sysContext.WithTraceID(context.Background(), "trace-xyz")

	_, err := provider.Authenticate(ctx, map[string]interface{}{"username": "user"},
		map[string]interface{}{"password": "pass"}, nil)

	suite.Nil(err)
	suite.Equal("trace-xyz", customHeader)
	suite.Empty(defaultHeader)
}

func (suite *RestAuthnProviderTestSuite) TestAuthenticate_Failure() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(apiErrorResponse{
			Code:    authnprovidercm.ErrorCodeAuthenticationFailed,
			Message: "Auth Failed",
		})
	}))
	defer ts.Close()

	provider := newRestAuthnProvider(ts.URL, "", "X-Correlation-ID", suite.setupMockClient())
	result, err := provider.Authenticate(context.Background(), nil, nil, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeAuthenticationFailed, err.Code)
}

func (suite *RestAuthnProviderTestSuite) TestGetEntityReference_Success() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		suite.Equal("/entity-reference", r.URL.Path)
		suite.Equal(http.MethodPost, r.Method)
		suite.Equal("apikey123", r.Header.Get("API-KEY"))

		resp := providers.EntityReference{
			EntityID:       "user123",
			EntityCategory: "user",
			EntityType:     "customer",
			OUID:           "ou1",
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	provider := newRestAuthnProvider(ts.URL, "apikey123", "X-Correlation-ID", suite.setupMockClient())
	result, err := provider.GetEntityReference(context.Background(), "ref-token-123")

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("user123", result.EntityID)
	suite.Equal("user", result.EntityCategory)
	suite.Equal("customer", result.EntityType)
	suite.Equal("ou1", result.OUID)
}

func (suite *RestAuthnProviderTestSuite) TestGetEntityReference_Failure() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(apiErrorResponse{
			Code:    authnprovidercm.ErrorCodeInvalidToken,
			Message: "Invalid token",
		})
	}))
	defer ts.Close()

	provider := newRestAuthnProvider(ts.URL, "", "X-Correlation-ID", suite.setupMockClient())
	result, err := provider.GetEntityReference(context.Background(), "invalid-token")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidToken, err.Code)
}

func (suite *RestAuthnProviderTestSuite) TestGetAttributes_Success() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		suite.Equal("/attributes", r.URL.Path)
		suite.Equal(http.MethodPost, r.Method)

		var req GetAttributesRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		suite.Equal("token123", req.Token)
		suite.NotNil(req.RequestedAttributes)
		suite.Len(req.RequestedAttributes.Attributes, 1)
		suite.Contains(req.RequestedAttributes.Attributes, "email")

		resp := providers.AttributesResponse{
			Attributes: map[string]*providers.AttributeResponse{
				"email": {Value: "test@example.com"},
			},
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	provider := newRestAuthnProvider(ts.URL, "apikey123", "X-Correlation-ID", suite.setupMockClient())
	reqAttrs := &providers.RequestedAttributes{
		Attributes: map[string]*providers.AttributeMetadataRequest{
			"email": nil,
		},
	}
	result, err := provider.GetAttributes(context.Background(), "token123", reqAttrs, nil)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("test@example.com", result.Attributes["email"].Value)
}

func (suite *RestAuthnProviderTestSuite) TestGetAttributes_InvalidToken() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(apiErrorResponse{
			Code: authnprovidercm.ErrorCodeInvalidToken,
		})
	}))
	defer ts.Close()

	provider := newRestAuthnProvider(ts.URL, "", "X-Correlation-ID", suite.setupMockClient())
	result, err := provider.GetAttributes(context.Background(), "invalid", nil, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidToken, err.Code)
}

func (suite *RestAuthnProviderTestSuite) TestSystemError_Decoding() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return malformed JSON
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{invalid-json`))
	}))
	defer ts.Close()

	provider := newRestAuthnProvider(ts.URL, "", "X-Correlation-ID", suite.setupMockClient())
	result, err := provider.Authenticate(context.Background(), nil, nil, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
	suite.Equal(tidcommon.ServerErrorType, err.Type)
}
