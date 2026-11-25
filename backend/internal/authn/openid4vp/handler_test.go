// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package openid4vp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	vpprotocol "github.com/thunder-id/thunderid/internal/oauth/oauth2/openid4vp"
	"github.com/thunder-id/thunderid/internal/vc/truststore"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/openid4vpmock"
	"github.com/thunder-id/thunderid/tests/mocks/vc/truststoremock"
)

type OpenID4VPHandlerTestSuite struct {
	suite.Suite
	svc   *openid4vpmock.OpenID4VPServiceInterfaceMock
	trust *truststoremock.TrustStoreInterfaceMock
	h     *openID4VPHandler
	mux   *http.ServeMux
}

func TestOpenID4VPHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(OpenID4VPHandlerTestSuite))
}

func (suite *OpenID4VPHandlerTestSuite) SetupTest() {
	suite.svc = openid4vpmock.NewOpenID4VPServiceInterfaceMock(suite.T())
	suite.trust = truststoremock.NewTrustStoreInterfaceMock(suite.T())
	suite.h = newOpenID4VPHandler(suite.svc, suite.trust)
	suite.mux = http.NewServeMux()
	Initialize(suite.mux, suite.svc, suite.trust)
}

func (suite *OpenID4VPHandlerTestSuite) serve(method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	suite.mux.ServeHTTP(rec, req)
	return rec
}

func (suite *OpenID4VPHandlerTestSuite) decode(rec *httptest.ResponseRecorder) map[string]interface{} {
	var resp map[string]interface{}
	suite.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp
}

func (suite *OpenID4VPHandlerTestSuite) TestHandleTrustAnchors() {
	suite.trust.EXPECT().ListAnchors().Return([]truststore.TrustAnchorInfo{
		{Name: "root-a", Subject: "CN=root-a"},
	})

	rec := suite.serve(http.MethodGet, "/openid4vp/trust-anchors", "")

	suite.Require().Equal(http.StatusOK, rec.Code)
	var anchors []truststore.TrustAnchorInfo
	suite.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &anchors))
	suite.Require().Len(anchors, 1)
	suite.Equal("root-a", anchors[0].Name)
	suite.Equal("CN=root-a", anchors[0].Subject)
}

func (suite *OpenID4VPHandlerTestSuite) TestHandleInitiateSuccess() {
	expiresAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	suite.svc.EXPECT().Initiate(mock.Anything, "pid").Return(&vpprotocol.Initiation{
		State: "txn-1", WalletURI: "openid4vp://?request_uri=x", ExpiresAt: expiresAt,
	}, nil)

	rec := suite.serve(http.MethodPost, "/openid4vp/initiate", `{"definition_id":"pid"}`)

	suite.Require().Equal(http.StatusOK, rec.Code)
	resp := suite.decode(rec)
	suite.Equal("txn-1", resp["txn_id"])
	suite.Equal("openid4vp://?request_uri=x", resp["wallet_url"])
	suite.Equal("/openid4vp/status/txn-1", resp["status_url"])
	suite.Equal("2026-01-02T03:04:05Z", resp["expires_at"])
}

func (suite *OpenID4VPHandlerTestSuite) TestHandleInitiateMissingDefinitionID() {
	rec := suite.serve(http.MethodPost, "/openid4vp/initiate", `{}`)
	suite.Equal(http.StatusBadRequest, rec.Code)
}

func (suite *OpenID4VPHandlerTestSuite) TestHandleInitiateMalformedBody() {
	rec := suite.serve(http.MethodPost, "/openid4vp/initiate", `{not json`)
	suite.Equal(http.StatusBadRequest, rec.Code)
	suite.Equal("INVALID_REQUEST", suite.decode(rec)["code"])
}

func (suite *OpenID4VPHandlerTestSuite) TestHandleInitiateServiceErrors() {
	suite.Run("client error", func() {
		suite.SetupTest()
		suite.svc.EXPECT().Initiate(mock.Anything, "pid").Return(nil, &vpprotocol.ErrorInvalidRequest)
		rec := suite.serve(http.MethodPost, "/openid4vp/initiate", `{"definition_id":"pid"}`)
		suite.Equal(http.StatusBadRequest, rec.Code)
		suite.Equal(vpprotocol.ErrorInvalidRequest.Code, suite.decode(rec)["code"])
	})

	suite.Run("server error", func() {
		suite.SetupTest()
		suite.svc.EXPECT().Initiate(mock.Anything, "pid").Return(nil, &tidcommon.ServiceError{
			Type: tidcommon.ServerErrorType, Code: "SSE-1",
		})
		rec := suite.serve(http.MethodPost, "/openid4vp/initiate", `{"definition_id":"pid"}`)
		suite.Equal(http.StatusInternalServerError, rec.Code)
	})
}

func (suite *OpenID4VPHandlerTestSuite) TestHandleStatus() {
	suite.Run("completed", func() {
		suite.SetupTest()
		suite.svc.EXPECT().GetResult(mock.Anything, "txn-1").Return(&vpprotocol.RequestState{
			Status: vpprotocol.StatusCompleted, ResultToken: "token",
		}, nil)
		rec := suite.serve(http.MethodGet, "/openid4vp/status/txn-1", "")
		suite.Require().Equal(http.StatusOK, rec.Code)
		resp := suite.decode(rec)
		suite.Equal(string(vpprotocol.StatusCompleted), resp["status"])
		suite.Equal("token", resp["result_token"])
		suite.NotContains(resp, "error")
	})

	suite.Run("failed", func() {
		suite.SetupTest()
		suite.svc.EXPECT().GetResult(mock.Anything, "txn-1").Return(&vpprotocol.RequestState{
			Status: vpprotocol.StatusFailed, FailureReason: "access_denied",
		}, nil)
		rec := suite.serve(http.MethodGet, "/openid4vp/status/txn-1", "")
		suite.Require().Equal(http.StatusOK, rec.Code)
		resp := suite.decode(rec)
		suite.Equal(string(vpprotocol.StatusFailed), resp["status"])
		suite.Equal("access_denied", resp["error"])
		suite.NotContains(resp, "result_token")
	})

	suite.Run("unknown state", func() {
		suite.SetupTest()
		suite.svc.EXPECT().GetResult(mock.Anything, "nope").Return(nil, &vpprotocol.ErrorUnknownState)
		rec := suite.serve(http.MethodGet, "/openid4vp/status/nope", "")
		suite.Equal(http.StatusNotFound, rec.Code)
		suite.Equal(vpprotocol.ErrorUnknownState.Code, suite.decode(rec)["code"])
	})
}

func (suite *OpenID4VPHandlerTestSuite) TestHandleStatusMissingTxnID() {
	req := httptest.NewRequest(http.MethodGet, "/openid4vp/status/", nil)
	rec := httptest.NewRecorder()
	suite.h.HandleStatus(rec, req)
	suite.Equal(http.StatusBadRequest, rec.Code)
	suite.Equal("INVALID_REQUEST", suite.decode(rec)["code"])
}

func (suite *OpenID4VPHandlerTestSuite) TestPreflightRoutes() {
	for _, target := range []string{"/openid4vp/trust-anchors", "/openid4vp/initiate", "/openid4vp/status/txn-1"} {
		rec := suite.serve(http.MethodOptions, target, "")
		suite.Equal(http.StatusNoContent, rec.Code, target)
	}
}
