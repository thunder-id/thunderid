// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzdetailexec

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/cryptomock"
)

type ProcessorTestSuite struct {
	suite.Suite
	client processorClientInterface
}

func TestProcessorTestSuite(t *testing.T) {
	suite.Run(t, new(ProcessorTestSuite))
}

func (s *ProcessorTestSuite) SetupTest() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("", &config.Config{Server: engineconfig.ServerConfig{Identifier: "test"}})
	s.client = newProcessorClient(5 * time.Second)
}

func sampleProcessorRequest() processorRequest {
	return processorRequest{
		Subject:  processorEntity{Type: string(providers.EntityCategoryUser), ID: testUserID},
		Client:   processorEntity{Type: oauth2const.SubTypeAgent, ID: testClientID},
		Resource: "https://bank.example.com",
		Details: []processorDetail{{
			DetailID: "d-1", Type: testPaymentType, TypeVersion: 1, Detail: json.RawMessage(`{"amount":1}`),
		}},
	}
}

func (s *ProcessorTestSuite) TestProcess_Success() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Equal(http.MethodPost, r.Method)
		s.Empty(r.Header.Get("Authorization"), "a processor without authentication gets no credential")
		s.Equal("application/json", r.Header.Get("Content-Type"))

		body, err := io.ReadAll(r.Body)
		s.Require().NoError(err)
		s.JSONEq(`{"subject":{"type":"user","id":"`+testUserID+`"},"client":{"type":"agent","id":"`+
			testClientID+`"},"resource":"https://bank.example.com",`+
			`"details":[{"detailId":"d-1","type":"`+testPaymentType+`","typeVersion":1,`+
			`"detail":{"amount":1}}]}`, string(body))

		_, _ = w.Write([]byte(`{"results":[{"detailId":"d-1","decision":false,` +
			`"patch":{"/debtorAccount/iban":"DE12"},"context":{"reason_user":"Over limit","reason_admin":"rule 7"}}]}`))
	}))
	defer server.Close()

	resp, err := s.client.Process(context.Background(), providers.AuthorizationDetailProcessor{Endpoint: server.URL},
		sampleProcessorRequest())
	s.Require().NoError(err)
	s.Require().Len(resp.Results, 1)
	s.Require().NotNil(resp.Results[0].Decision)
	s.False(*resp.Results[0].Decision)
	s.Equal(`"DE12"`, string(resp.Results[0].Patch["/debtorAccount/iban"]))
	s.Equal(processorResultContext{ReasonUser: "Over limit", ReasonAdmin: "rule 7"}, resp.Results[0].Context)
}

func (s *ProcessorTestSuite) TestProcess_DoesNotFollowRedirects() {
	followed := false
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { followed = true }))
	defer target.Close()
	redirecting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirecting.Close()
	_, err := s.client.Process(context.Background(), providers.AuthorizationDetailProcessor{Endpoint: redirecting.URL},
		sampleProcessorRequest())
	s.True(errors.Is(err, errProcessorUnavailable))
	s.False(followed, "the details are not re-posted to the redirect target")
}

func (s *ProcessorTestSuite) TestProcess_FailuresAreUnavailable() {
	statusServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer statusServer.Close()
	bodyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer bodyServer.Close()
	closed := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	closed.Close()
	// A valid answer followed by padding past the limit: the response is refused, not cut short.
	oversizedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"detailId":"d-1","decision":true}]}`))
		_, _ = w.Write(bytes.Repeat([]byte(" "), maxProcessorResponseBytes))
	}))
	defer oversizedServer.Close()

	for name, endpoint := range map[string]string{
		"non-200 status": statusServer.URL, "invalid body": bodyServer.URL, "unreachable": closed.URL,
		"invalid url": "://bad", "oversized body": oversizedServer.URL,
	} {
		_, err := s.client.Process(context.Background(), providers.AuthorizationDetailProcessor{Endpoint: endpoint},
			sampleProcessorRequest())
		s.True(errors.Is(err, errProcessorUnavailable), name)
	}
}

func (s *ProcessorTestSuite) TestProcess_ReusesConnections() {
	var connections atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"detailId":"d-1","decision":true}]}`))
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			connections.Add(1)
		}
	}
	server.Start()
	defer server.Close()

	for range 3 {
		_, err := s.client.Process(context.Background(), providers.AuthorizationDetailProcessor{Endpoint: server.URL},
			sampleProcessorRequest())
		s.Require().NoError(err)
	}
	s.Equal(int32(1), connections.Load())
}

// withTestCrypto installs a crypto provider that hex-encodes what it encrypts.
func withTestCrypto(t *testing.T) {
	provider := cryptomock.NewConfigCryptoProviderMock(t)
	provider.EXPECT().Encrypt(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, content []byte) ([]byte, error) {
			return []byte(hex.EncodeToString(content)), nil
		}).Maybe()
	provider.EXPECT().Decrypt(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, content []byte) ([]byte, error) {
			return hex.DecodeString(string(content))
		}).Maybe()
	cmodels.SetConfigCryptoProvider(provider)
}

// processorWith returns a processor using the given outbound authentication, stored encrypted.
func (s *ProcessorTestSuite) processorWith(endpoint string, authType outboundauth.Type,
	properties map[string]string) providers.AuthorizationDetailProcessor {
	stored, err := outboundauth.ToProperties(outboundauth.Config{Type: authType, Properties: properties})
	s.Require().NoError(err)
	return providers.AuthorizationDetailProcessor{Endpoint: endpoint, AuthenticationProperties: stored}
}

func (s *ProcessorTestSuite) bearerProcessor(endpoint, token string) providers.AuthorizationDetailProcessor {
	return s.processorWith(endpoint, outboundauth.TypeBearer, map[string]string{outboundauth.FieldBearerToken: token})
}

func (s *ProcessorTestSuite) TestProcess_AuthenticatesTheCall() {
	withTestCrypto(s.T())
	var header string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"results":[{"detailId":"d-1","decision":true}]}`))
	}))
	defer server.Close()

	_, err := s.client.Process(context.Background(), s.bearerProcessor(server.URL, "processor-token"),
		sampleProcessorRequest())

	s.Require().NoError(err)
	s.Equal("Bearer processor-token", header)
}

func (s *ProcessorTestSuite) TestProcess_UnreadableAuthenticationFailsClosed() {
	withTestCrypto(s.T())
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	defer server.Close()
	unknown, err := cmodels.NewProperty(outboundauth.PropertyKeyType, "kerberos", false)
	s.Require().NoError(err)

	_, err = s.client.Process(context.Background(), providers.AuthorizationDetailProcessor{Endpoint: server.URL,
		AuthenticationProperties: []cmodels.Property{*unknown}}, sampleProcessorRequest())

	s.Require().ErrorIs(err, errProcessorUnavailable)
	s.False(called, "the details are not sent without the authentication the type declares")
}

func (s *ProcessorTestSuite) TestBatchKey_GroupsByEndpointAndCredential() {
	withTestCrypto(s.T())
	first, err := batchKey(s.bearerProcessor(testProcessorURL, "token-a"))
	s.Require().NoError(err)
	sameCredential, err := batchKey(s.bearerProcessor(testProcessorURL, "token-a"))
	s.Require().NoError(err)
	otherCredential, err := batchKey(s.bearerProcessor(testProcessorURL, "token-b"))
	s.Require().NoError(err)
	none, err := batchKey(providers.AuthorizationDetailProcessor{Endpoint: testProcessorURL})
	s.Require().NoError(err)

	s.Equal(first, sameCredential, "types sharing an endpoint and a credential share a call")
	s.NotEqual(first, otherCredential, "a credential never goes with another type's details")
	s.NotEqual(first, none)
}
