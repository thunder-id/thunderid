// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authncommon "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/authn/openid4vp"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/authn/openid4vpmock"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

type OpenID4VPVerifierTestSuite struct {
	suite.Suite
	service       *openid4vpmock.OpenID4VPServiceInterfaceMock
	authnProvider *managermock.AuthnProviderManagerMock
	executor      providers.Executor
}

func TestOpenID4VPVerifierTestSuite(t *testing.T) {
	suite.Run(t, new(OpenID4VPVerifierTestSuite))
}

func (s *OpenID4VPVerifierTestSuite) SetupTest() {
	s.service = openid4vpmock.NewOpenID4VPServiceInterfaceMock(s.T())
	s.authnProvider = managermock.NewAuthnProviderManagerMock(s.T())
	factory := coremock.NewFlowFactoryInterfaceMock(s.T())
	base := coremock.NewExecutorInterfaceMock(s.T())
	factory.On("CreateExecutor", ExecutorNameOpenID4VPVerify, providers.ExecutorTypeAuthentication,
		[]providers.Input{}, []providers.Input{}, mock.Anything).Return(base).Maybe()
	s.executor = newOpenID4VPVerifier(factory, s.service, s.authnProvider)
}

func (s *OpenID4VPVerifierTestSuite) SetupSubTest() {
	s.SetupTest()
}

func openid4vpNodeContext(runtime map[string]string, properties map[string]interface{}) *providers.NodeContext {
	if runtime == nil {
		runtime = map[string]string{}
	}
	return &providers.NodeContext{
		Context:        context.Background(),
		ExecutionID:    "exec-1",
		FlowType:       providers.FlowTypeAuthentication,
		RuntimeData:    runtime,
		NodeProperties: properties,
	}
}

func (s *OpenID4VPVerifierTestSuite) TestInitiates() {
	s.service.EXPECT().Initiate(mock.Anything, "custom-def", true).Return(&openid4vp.Initiation{
		State:      "state-123",
		ClientID:   "x509_hash:abc",
		RequestURI: "https://verifier.example/openid4vp/request?state=state-123",
		WalletURI: "openid4vp://authorize?client_id=x509_hash%3Aabc" +
			"&request_uri=https%3A%2F%2Fverifier.example%2Fopenid4vp%2Frequest%3Fstate%3Dstate-123",
	}, nil)

	props := map[string]interface{}{propertyKeyPresentationDefinitionID: "custom-def"}
	resp, err := s.executor.Execute(openid4vpNodeContext(nil, props))
	s.Require().NoError(err)
	s.Equal(providers.ExecUserInputRequired, resp.Status)
	s.Equal("state-123", resp.RuntimeData[common.RuntimeKeyOpenID4VPState])
	s.Equal("x509_hash:abc", resp.AdditionalData[common.DataOpenID4VPClientID])
	s.Contains(resp.AdditionalData[common.DataOpenID4VPRequestURI], "state-123")
	s.Contains(resp.AdditionalData[common.DataOpenID4VPWalletURI], "openid4vp://")
}

// The service mock has no Initiate expectation, so a call to it fails the test.
func (s *OpenID4VPVerifierTestSuite) TestMissingDefinitionID() {
	resp, err := s.executor.Execute(openid4vpNodeContext(nil, nil))
	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Equal(ErrOpenID4VPDefinitionNotConfigured.Code, resp.Error.Code)
}

func (s *OpenID4VPVerifierTestSuite) TestInitiateFailure() {
	cases := []struct {
		name     string
		svcErr   *tidcommon.ServiceError
		wantCode string
	}{
		{name: "a server fault", svcErr: &tidcommon.InternalServerError, wantCode: ErrOpenID4VPInitiateFailed.Code},
		{name: "a definition not set up for sign-in", svcErr: &openid4vp.ErrorDefinitionNotForSignIn,
			wantCode: ErrOpenID4VPDefinitionNotForSignIn.Code},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.service.EXPECT().Initiate(mock.Anything, "custom-def", true).Return(nil, tc.svcErr)

			resp, err := s.executor.Execute(openid4vpNodeContext(nil, map[string]interface{}{
				propertyKeyPresentationDefinitionID: "custom-def",
			}))
			s.Require().NoError(err)
			s.Equal(providers.ExecFailure, resp.Status)
			s.Equal(tc.wantCode, resp.Error.Code)
		})
	}
}

func (s *OpenID4VPVerifierTestSuite) TestPollPending() {
	s.service.EXPECT().GetResult(mock.Anything, "state-123").Return(&openid4vp.RequestState{
		Status:     openid4vp.StatusPending,
		ClientID:   "x509_hash:abc",
		RequestURI: "https://verifier.example/openid4vp/request?state=state-123",
	}, nil)

	runtime := map[string]string{common.RuntimeKeyOpenID4VPState: "state-123"}
	resp, err := s.executor.Execute(openid4vpNodeContext(runtime, nil))
	s.Require().NoError(err)
	s.Equal(providers.ExecUserInputRequired, resp.Status)
	s.Equal("state-123", resp.RuntimeData[common.RuntimeKeyOpenID4VPState])
	// QR data must persist across polls so the wait view keeps rendering it.
	s.Equal("x509_hash:abc", resp.AdditionalData[common.DataOpenID4VPClientID])
	s.Contains(resp.AdditionalData[common.DataOpenID4VPRequestURI], "state-123")
	s.Contains(resp.AdditionalData[common.DataOpenID4VPWalletURI], "openid4vp://")
}

func (s *OpenID4VPVerifierTestSuite) TestPollCompleted() {
	s.service.EXPECT().GetResult(mock.Anything, "state-123").Return(&openid4vp.RequestState{
		Status: openid4vp.StatusCompleted,
		Result: &openid4vp.VerifiedPresentation{
			DefinitionID:  "def-1",
			Issuer:        "https://issuer.example",
			IssuerTrusted: true,
			SubjectClaim:  "national_id",
			Subject:       "sub-1",
			Claims: map[string]interface{}{
				"given_name":  "Erika",
				"family_name": "Mustermann",
			},
		},
	}, nil)
	s.authnProvider.On("AuthenticateUser",
		mock.Anything, mock.Anything, mock.MatchedBy(func(creds map[string]interface{}) bool {
			cred, ok := creds[authnprovidercm.CredentialTypeOpenID4VP].(*authncommon.OpenID4VPCredential)
			return ok && cred.DefinitionID == "def-1" && cred.Issuer == "https://issuer.example" &&
				cred.IssuerTrusted && cred.Subject == "national_id:sub-1"
		}), mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, providers.AuthenticatedClaims{
			"given_name":     "Erika",
			"family_name":    "Mustermann",
			userAttributeSub: "sub-1",
		}, nil)

	runtime := map[string]string{common.RuntimeKeyOpenID4VPState: "state-123"}
	resp, err := s.executor.Execute(openid4vpNodeContext(runtime, map[string]interface{}{
		common.NodePropertyAllowAuthenticationWithoutLocalUser: true,
	}))
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	// The external identity names the link scoped to the definition and issuer, so provisioning records it,
	// and the subject is kept out of the claims so it never lands in the profile.
	identity := core.GetExternalIdentity(resp.RuntimeData)
	s.Require().NotNil(identity)
	s.Equal("openid4vp:def-1:https://issuer.example", identity.IdpID)
	s.Equal("national_id:sub-1", identity.Sub)
	s.NotContains(identity.Claims, userAttributeSub)
	s.Equal("Erika", externalClaim(resp.RuntimeData, "given_name"))
	// AuthUser is not authenticated (no entity reference resolved), so eligible for provisioning
	s.Equal(dataValueTrue, resp.RuntimeData[common.RuntimeKeyUserEligibleForProvisioning])
}

// An unverified issuer's iss and sub are attacker-chosen, so no link is recorded for them: a recorded
// link would resolve the real holder to whoever minted it.
func (s *OpenID4VPVerifierTestSuite) TestPollCompletedUntrustedIssuerRecordsNoLink() {
	s.service.EXPECT().GetResult(mock.Anything, "state-123").Return(&openid4vp.RequestState{
		Status: openid4vp.StatusCompleted,
		Result: &openid4vp.VerifiedPresentation{
			Issuer:  "https://issuer.example",
			Subject: "sub-1",
		},
	}, nil)
	s.authnProvider.On("AuthenticateUser",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, providers.AuthenticatedClaims{"given_name": "Erika"}, nil)

	runtime := map[string]string{common.RuntimeKeyOpenID4VPState: "state-123"}
	resp, err := s.executor.Execute(openid4vpNodeContext(runtime, nil))
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	identity := core.GetExternalIdentity(resp.RuntimeData)
	s.Require().NotNil(identity)
	s.Empty(identity.IdpID)
	s.Empty(identity.Sub)
}

// Failing to store the presented claims is a server fault, so it surfaces as an error rather than as a
// failed presentation.
func (s *OpenID4VPVerifierTestSuite) TestPollCompletedStoreClaimsFails() {
	s.service.EXPECT().GetResult(mock.Anything, "state-123").Return(&openid4vp.RequestState{
		Status: openid4vp.StatusCompleted,
		Result: &openid4vp.VerifiedPresentation{Subject: "sub-1"},
	}, nil)
	s.authnProvider.On("AuthenticateUser",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, providers.AuthenticatedClaims{"unencodable": make(chan int)}, nil)

	runtime := map[string]string{common.RuntimeKeyOpenID4VPState: "state-123"}
	resp, err := s.executor.Execute(openid4vpNodeContext(runtime, nil))
	s.Require().Error(err)
	s.Nil(resp)
}

// A hold the provider states fails the step with its own code; another refusal is a failed
// presentation, and a server fault surfaces as an error.
func (s *OpenID4VPVerifierTestSuite) TestPollCompletedProviderRefuses() {
	cases := []struct {
		name     string
		svcErr   *tidcommon.ServiceError
		wantCode string
	}{
		{name: "a stated lock", svcErr: &authnprovidermgr.ErrorAccountLocked, wantCode: ErrAccountLocked.Code},
		{name: "a stated suspension", svcErr: &authnprovidermgr.ErrorAccountSuspended,
			wantCode: ErrAccountSuspended.Code},
		{name: "another refusal", svcErr: &authnprovidermgr.ErrorAuthenticationFailed,
			wantCode: ErrOpenID4VPVerificationFailed.Code},
		{name: "a server fault", svcErr: &tidcommon.InternalServerError},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.service.EXPECT().GetResult(mock.Anything, "state-123").Return(&openid4vp.RequestState{
				Status: openid4vp.StatusCompleted,
				Result: &openid4vp.VerifiedPresentation{Subject: "sub-1"},
			}, nil)
			s.authnProvider.On("AuthenticateUser",
				mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(providers.AuthUser{}, providers.AuthenticatedClaims(nil), tc.svcErr)

			runtime := map[string]string{common.RuntimeKeyOpenID4VPState: "state-123"}
			resp, err := s.executor.Execute(openid4vpNodeContext(runtime, nil))
			if tc.wantCode == "" {
				s.Require().Error(err)
				s.Nil(resp)
				return
			}
			s.Require().NoError(err)
			s.Equal(providers.ExecFailure, resp.Status)
			s.Equal(tc.wantCode, resp.Error.Code)
		})
	}
}

func (s *OpenID4VPVerifierTestSuite) TestPollFailed() {
	s.service.EXPECT().GetResult(mock.Anything, "state-123").Return(&openid4vp.RequestState{
		Status: openid4vp.StatusFailed, FailureReason: "nonce mismatch",
	}, nil)

	runtime := map[string]string{common.RuntimeKeyOpenID4VPState: "state-123"}
	resp, err := s.executor.Execute(openid4vpNodeContext(runtime, nil))
	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Equal(ErrOpenID4VPVerificationFailed.Code, resp.Error.Code)
	s.Contains(resp.Error.ErrorDescription.String(), "nonce mismatch")
}

func (s *OpenID4VPVerifierTestSuite) TestPollExpired() {
	s.service.EXPECT().GetResult(mock.Anything, "gone").Return(nil, &tidcommon.InternalServerError)

	runtime := map[string]string{common.RuntimeKeyOpenID4VPState: "gone"}
	resp, err := s.executor.Execute(openid4vpNodeContext(runtime, nil))
	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Equal(ErrOpenID4VPExpired.Code, resp.Error.Code)
}
