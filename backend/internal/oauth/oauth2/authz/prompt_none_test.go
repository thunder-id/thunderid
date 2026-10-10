// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	flowsession "github.com/thunder-id/thunderid/internal/flow/session"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	oauth2model "github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/flow/flowexecmock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/sessionmock"
)

const (
	promptNoneAppID      = "app-1"
	promptNoneFlowID     = "flow-1"
	promptNoneSubject    = "user-1"
	promptNoneFlowCookie = "handle-xyz"
)

// promptNoneCtx returns a context carrying the SSO cookie the authorize endpoint would have read
// from the request, named for the client's authentication flow.
func promptNoneCtx() context.Context {
	transport := flowsession.NewHandleTransport(flowsession.TransportConfig{})
	issued := httptest.NewRecorder()
	transport.Write(&flowsession.Exchange{Response: issued}, promptNoneFlowID, promptNoneFlowCookie, time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/oauth2/authorize", nil)
	for _, ck := range issued.Result().Cookies() {
		req.AddCookie(ck)
	}
	return flowsession.WithInbound(context.Background(), transport.Read(&flowsession.Exchange{Request: req}))
}

// promptNoneApp is the OAuth client the request is made for.
func promptNoneApp() *providers.OAuthClient {
	return &providers.OAuthClient{ID: promptNoneAppID, ClientID: "client-1"}
}

// promptNoneSession returns a live session for the subject, authenticated the given duration ago.
func promptNoneSession(authenticatedAgo time.Duration) *flowsession.Session {
	return &flowsession.Session{
		SessionID:       "sess-1",
		SubjectID:       promptNoneSubject,
		FlowID:          promptNoneFlowID,
		FlowVersion:     2,
		HandleID:        promptNoneFlowCookie,
		AuthenticatedAt: time.Now().UTC().Add(-authenticatedAgo),
		State:           flowsession.StateActive,
	}
}

// wirePromptNone builds a service whose client resolves to the authentication flow above and whose
// session service returns the given session (nil for "no live session").
func (suite *AuthorizeServiceTestSuite) wirePromptNone(sess *flowsession.Session) *authorizeService {
	svc := suite.newService()

	flowProvider := flowexecmock.NewFlowProviderMock(suite.T())
	flowProvider.EXPECT().GetFlow(mock.Anything, promptNoneFlowID).
		Return(&providers.CompleteFlowDefinition{ID: promptNoneFlowID, ActiveVersion: 2}, nil).Maybe()
	svc.flowProvider = flowProvider

	ssoService := sessionmock.NewServiceMock(suite.T())
	ssoService.EXPECT().
		Resolve(mock.Anything, promptNoneFlowCookie, promptNoneFlowID, 2, mock.Anything).
		Return(sess, nil).Maybe()
	svc.ssoSession = ssoService

	// The client binds to the authentication flow whose cookie carries the session handle.
	suite.mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, promptNoneAppID).
		Return(&inboundmodel.InboundClient{ID: promptNoneAppID, AuthFlowID: promptNoneFlowID}, nil).Maybe()

	return svc
}

// TestCheckPromptNone_NotRequested covers a request without prompt=none: the check is skipped
// entirely and no session is resolved.
func (suite *AuthorizeServiceTestSuite) TestCheckPromptNone_NotRequested() {
	svc := suite.newService()

	errCode, errMsg := svc.checkPromptNone(promptNoneCtx(),
		&oauth2model.OAuthParameters{Prompt: ""}, promptNoneApp())

	suite.Empty(errCode)
	suite.Empty(errMsg)
}

// TestCheckPromptNone_NoSession covers prompt=none with nobody signed in, which is the one case
// the specification answers with login_required.
func (suite *AuthorizeServiceTestSuite) TestCheckPromptNone_NoSession() {
	svc := suite.wirePromptNone(nil)

	errCode, _ := svc.checkPromptNone(promptNoneCtx(),
		&oauth2model.OAuthParameters{Prompt: oauth2const.PromptNone}, promptNoneApp())

	suite.Equal(oauth2const.ErrorLoginRequired, errCode)
}

// TestCheckPromptNone_LiveSession covers the case the change exists for: an authenticated subject
// makes prompt=none succeed without any interaction.
func (suite *AuthorizeServiceTestSuite) TestCheckPromptNone_LiveSession() {
	svc := suite.wirePromptNone(promptNoneSession(time.Minute))

	errCode, errMsg := svc.checkPromptNone(promptNoneCtx(),
		&oauth2model.OAuthParameters{Prompt: oauth2const.PromptNone}, promptNoneApp())

	suite.Empty(errCode)
	suite.Empty(errMsg)
}

// TestCheckPromptNone_NoSessionService covers a deployment without a session store (the embedded
// engine), where prompt=none keeps answering login_required rather than panicking.
func (suite *AuthorizeServiceTestSuite) TestCheckPromptNone_NoSessionService() {
	svc := suite.newService()

	errCode, _ := svc.checkPromptNone(promptNoneCtx(),
		&oauth2model.OAuthParameters{Prompt: oauth2const.PromptNone}, promptNoneApp())

	suite.Equal(oauth2const.ErrorLoginRequired, errCode)
}

// TestCheckPromptNone_NoInboundCookie covers a request carrying no SSO cookie at all: there is no
// handle to resolve, so the answer is login_required.
func (suite *AuthorizeServiceTestSuite) TestCheckPromptNone_NoInboundCookie() {
	svc := suite.wirePromptNone(promptNoneSession(time.Minute))

	errCode, _ := svc.checkPromptNone(context.Background(),
		&oauth2model.OAuthParameters{Prompt: oauth2const.PromptNone}, promptNoneApp())

	suite.Equal(oauth2const.ErrorLoginRequired, errCode)
}

// TestCheckPromptNone_MaxAgeExceeded covers a live session too old to satisfy max_age. Silently
// reusing it would report an authentication older than the client asked for.
func (suite *AuthorizeServiceTestSuite) TestCheckPromptNone_MaxAgeExceeded() {
	svc := suite.wirePromptNone(promptNoneSession(time.Hour))

	errCode, _ := svc.checkPromptNone(promptNoneCtx(), &oauth2model.OAuthParameters{
		Prompt: oauth2const.PromptNone,
		MaxAge: "60",
	}, promptNoneApp())

	suite.Equal(oauth2const.ErrorLoginRequired, errCode)
}

// TestCheckPromptNone_MaxAgeWithinLimit covers a max_age the session still satisfies.
func (suite *AuthorizeServiceTestSuite) TestCheckPromptNone_MaxAgeWithinLimit() {
	svc := suite.wirePromptNone(promptNoneSession(time.Minute))

	errCode, _ := svc.checkPromptNone(promptNoneCtx(), &oauth2model.OAuthParameters{
		Prompt: oauth2const.PromptNone,
		MaxAge: "3600",
	}, promptNoneApp())

	suite.Empty(errCode)
}

// TestCheckPromptNone_MalformedMaxAge covers a max_age that does not parse: it is treated as no
// constraint, matching how the flow's assurance check reads the same value.
func (suite *AuthorizeServiceTestSuite) TestCheckPromptNone_MalformedMaxAge() {
	svc := suite.wirePromptNone(promptNoneSession(time.Hour))

	errCode, _ := svc.checkPromptNone(promptNoneCtx(), &oauth2model.OAuthParameters{
		Prompt: oauth2const.PromptNone,
		MaxAge: "not-a-number",
	}, promptNoneApp())

	suite.Empty(errCode)
}

// TestCheckPromptNone_MaxAgeBoundary covers a session inside the limit by a margin the comparison
// cannot lose to a clock tick. Deriving max_age from a separate time.Now() reading would race the
// one checkPromptNone takes: a second elapsing in between turns "exactly at the limit" into "one
// past it". The margin keeps the reuse assertion meaningful without depending on that timing.
func (suite *AuthorizeServiceTestSuite) TestCheckPromptNone_MaxAgeBoundary() {
	svc := suite.wirePromptNone(promptNoneSession(60 * time.Second))

	errCode, _ := svc.checkPromptNone(promptNoneCtx(), &oauth2model.OAuthParameters{
		Prompt: oauth2const.PromptNone,
		MaxAge: "120",
	}, promptNoneApp())

	suite.Empty(errCode, "a session well inside max_age must satisfy the request")
}

// idTokenHint builds an unsigned JWT carrying the given issuer and subject. The signature is a
// placeholder because the test mocks signature verification; only the payload is read here.
func idTokenHint(issuer, subject string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payload, _ := json.Marshal(map[string]interface{}{"iss": issuer, "sub": subject})
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

// TestCheckPromptNone_IDTokenHintMatches covers a hint naming the subject who is signed in: the
// request identifies the same user, so it succeeds without interaction.
func (suite *AuthorizeServiceTestSuite) TestCheckPromptNone_IDTokenHintMatches() {
	svc := suite.wirePromptNone(promptNoneSession(time.Minute))
	suite.mockJWTService.EXPECT().VerifyJWTSignature(mock.Anything, mock.Anything).Return(nil)

	errCode, _ := svc.checkPromptNone(promptNoneCtx(), &oauth2model.OAuthParameters{
		Prompt:      oauth2const.PromptNone,
		IDTokenHint: idTokenHint("https://localhost:8090", promptNoneSubject),
	}, promptNoneApp())

	suite.Empty(errCode)
}

// TestCheckPromptNone_IDTokenHintDifferentSubject covers a hint naming someone other than the
// signed-in subject. The server cannot silently switch users, so it answers login_required.
func (suite *AuthorizeServiceTestSuite) TestCheckPromptNone_IDTokenHintDifferentSubject() {
	svc := suite.wirePromptNone(promptNoneSession(time.Minute))
	suite.mockJWTService.EXPECT().VerifyJWTSignature(mock.Anything, mock.Anything).Return(nil)

	errCode, _ := svc.checkPromptNone(promptNoneCtx(), &oauth2model.OAuthParameters{
		Prompt:      oauth2const.PromptNone,
		IDTokenHint: idTokenHint("https://localhost:8090", "someone-else"),
	}, promptNoneApp())

	suite.Equal(oauth2const.ErrorLoginRequired, errCode)
}

// TestCheckPromptNone_IDTokenHintForeignIssuer covers a hint this server did not issue, which
// cannot be trusted to identify anyone.
func (suite *AuthorizeServiceTestSuite) TestCheckPromptNone_IDTokenHintForeignIssuer() {
	svc := suite.wirePromptNone(promptNoneSession(time.Minute))
	suite.mockJWTService.EXPECT().VerifyJWTSignature(mock.Anything, mock.Anything).Return(nil)

	errCode, _ := svc.checkPromptNone(promptNoneCtx(), &oauth2model.OAuthParameters{
		Prompt:      oauth2const.PromptNone,
		IDTokenHint: idTokenHint("https://attacker.example.com", promptNoneSubject),
	}, promptNoneApp())

	suite.Equal(oauth2const.ErrorInvalidRequest, errCode)
}

// TestCheckPromptNone_IDTokenHintBadSignature covers a hint whose signature does not verify.
func (suite *AuthorizeServiceTestSuite) TestCheckPromptNone_IDTokenHintBadSignature() {
	svc := suite.wirePromptNone(promptNoneSession(time.Minute))
	suite.mockJWTService.EXPECT().VerifyJWTSignature(mock.Anything, mock.Anything).
		Return(&tidcommon.ServiceError{Code: "JWT-1001"})

	errCode, _ := svc.checkPromptNone(promptNoneCtx(), &oauth2model.OAuthParameters{
		Prompt:      oauth2const.PromptNone,
		IDTokenHint: idTokenHint("https://localhost:8090", promptNoneSubject),
	}, promptNoneApp())

	suite.Equal(oauth2const.ErrorInvalidRequest, errCode)
}

// TestCheckPromptNone_IDTokenHintNotAnIDToken covers a hint that is another JWT this server signs, such
// as an access or logout token, which does not identify a sign-in.
func (suite *AuthorizeServiceTestSuite) TestCheckPromptNone_IDTokenHintNotAnIDToken() {
	for _, typ := range []string{"at+jwt", "logout+jwt"} {
		suite.Run(typ, func() {
			svc := suite.wirePromptNone(promptNoneSession(time.Minute))
			suite.mockJWTService.EXPECT().VerifyJWTSignature(mock.Anything, mock.Anything).Return(nil).Once()
			header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"` + typ + `"}`))
			payload, _ := json.Marshal(map[string]interface{}{
				"iss": "https://localhost:8090",
				"sub": promptNoneSubject,
			})
			hint := header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"

			errCode, _ := svc.checkPromptNone(promptNoneCtx(), &oauth2model.OAuthParameters{
				Prompt:      oauth2const.PromptNone,
				IDTokenHint: hint,
			}, promptNoneApp())

			suite.Equal(oauth2const.ErrorInvalidRequest, errCode)
		})
	}
}

// TestCheckPromptNone_IDTokenHintLegacyRefreshToken covers a refresh token minted before rt+jwt, which
// shares the generic type with ID tokens and is told apart by its access_token_sub claim.
func (suite *AuthorizeServiceTestSuite) TestCheckPromptNone_IDTokenHintLegacyRefreshToken() {
	svc := suite.wirePromptNone(promptNoneSession(time.Minute))
	suite.mockJWTService.EXPECT().VerifyJWTSignature(mock.Anything, mock.Anything).Return(nil)
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payload, _ := json.Marshal(map[string]interface{}{
		"iss":              "https://localhost:8090",
		"sub":              promptNoneSubject,
		"access_token_sub": promptNoneSubject,
	})
	hint := header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"

	errCode, _ := svc.checkPromptNone(promptNoneCtx(), &oauth2model.OAuthParameters{
		Prompt:      oauth2const.PromptNone,
		IDTokenHint: hint,
	}, promptNoneApp())

	suite.Equal(oauth2const.ErrorInvalidRequest, errCode)
}

// TestCheckPromptNone_IDTokenHintNoSubject covers a hint carrying no sub claim: there is nothing to
// compare the session against.
func (suite *AuthorizeServiceTestSuite) TestCheckPromptNone_IDTokenHintNoSubject() {
	svc := suite.wirePromptNone(promptNoneSession(time.Minute))
	suite.mockJWTService.EXPECT().VerifyJWTSignature(mock.Anything, mock.Anything).Return(nil)

	errCode, _ := svc.checkPromptNone(promptNoneCtx(), &oauth2model.OAuthParameters{
		Prompt:      oauth2const.PromptNone,
		IDTokenHint: idTokenHint("https://localhost:8090", ""),
	}, promptNoneApp())

	suite.Equal(oauth2const.ErrorInvalidRequest, errCode)
}

// TestResolveSSOSession_NoInboundHandle covers a request that never carried the SSO cookies, which
// happens on any path that did not read them off the request (PAR resolution, the embedded engine).
// There is no handle to resolve, so prompt=none cannot be satisfied.
func (suite *AuthorizeServiceTestSuite) TestResolveSSOSession_NoInboundHandle() {
	svc := suite.wirePromptNone(promptNoneSession(time.Minute))

	// A bare context, without the inbound handle the transport would have attached.
	suite.Nil(svc.resolveSSOSession(context.Background(), promptNoneApp()),
		"without the inbound cookies there is no handle to resolve a session from")
}

// TestResolveSSOSession_ClientLookupFails covers the inbound client not resolving: the flow that
// names the session's cookie is unknown, so no session can be attributed to this request.
func (suite *AuthorizeServiceTestSuite) TestResolveSSOSession_ClientLookupFails() {
	svc := suite.newService()
	svc.flowProvider = flowexecmock.NewFlowProviderMock(suite.T())
	svc.ssoSession = sessionmock.NewServiceMock(suite.T())
	suite.mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, promptNoneAppID).
		Return(nil, errors.New("inbound client lookup failed"))

	suite.Nil(svc.resolveSSOSession(promptNoneCtx(), promptNoneApp()),
		"an unresolvable client cannot yield a session")
}

// TestResolveSSOSession_FlowLookupFails covers the client's authentication flow not resolving. The
// session must match the flow's active version, so without the flow there is nothing to match.
func (suite *AuthorizeServiceTestSuite) TestResolveSSOSession_FlowLookupFails() {
	svc := suite.newService()
	flowProvider := flowexecmock.NewFlowProviderMock(suite.T())
	flowProvider.EXPECT().GetFlow(mock.Anything, promptNoneFlowID).
		Return(nil, &tidcommon.ServiceError{Code: "FLM-1001"})
	svc.flowProvider = flowProvider
	svc.ssoSession = sessionmock.NewServiceMock(suite.T())
	suite.mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, promptNoneAppID).
		Return(&inboundmodel.InboundClient{ID: promptNoneAppID, AuthFlowID: promptNoneFlowID}, nil)

	suite.Nil(svc.resolveSSOSession(promptNoneCtx(), promptNoneApp()),
		"without the flow there is no active version to match the session against")
}

// TestResolveSSOSession_ResolveErrors covers the session store failing. A store error is not a
// silent success: the request falls back to login_required rather than assuming a session exists.
func (suite *AuthorizeServiceTestSuite) TestResolveSSOSession_ResolveErrors() {
	svc := suite.newService()
	flowProvider := flowexecmock.NewFlowProviderMock(suite.T())
	flowProvider.EXPECT().GetFlow(mock.Anything, promptNoneFlowID).
		Return(&providers.CompleteFlowDefinition{ID: promptNoneFlowID, ActiveVersion: 2}, nil)
	svc.flowProvider = flowProvider

	ssoService := sessionmock.NewServiceMock(suite.T())
	ssoService.EXPECT().
		Resolve(mock.Anything, promptNoneFlowCookie, promptNoneFlowID, 2, mock.Anything).
		Return(nil, errors.New("session store unavailable"))
	svc.ssoSession = ssoService

	suite.mockInboundClient.EXPECT().GetInboundClientByEntityID(mock.Anything, promptNoneAppID).
		Return(&inboundmodel.InboundClient{ID: promptNoneAppID, AuthFlowID: promptNoneFlowID}, nil)

	suite.Nil(svc.resolveSSOSession(promptNoneCtx(), promptNoneApp()),
		"a store failure must not be mistaken for a live session")
}

// TestSubjectFromIDTokenHint_Undecodable covers a hint that passes signature verification but whose
// payload cannot be decoded, so no subject can be read from it.
func (suite *AuthorizeServiceTestSuite) TestSubjectFromIDTokenHint_Undecodable() {
	svc := suite.wirePromptNone(promptNoneSession(time.Minute))
	suite.mockJWTService.EXPECT().VerifyJWTSignature(mock.Anything, mock.Anything).Return(nil)

	_, err := svc.subjectFromIDTokenHint(context.Background(), "header.%%%.signature")

	suite.Error(err, "an undecodable payload cannot yield a subject")
}

// TestCheckPromptNone_MaxAgeZeroIsNeverSatisfied covers max_age=0 against a session authenticated
// moments ago. No existing authentication can satisfy a zero window, so the silent path is refused
// rather than reusing a session that happens to fall in the same Unix second.
func (suite *AuthorizeServiceTestSuite) TestCheckPromptNone_MaxAgeZeroIsNeverSatisfied() {
	svc := suite.wirePromptNone(promptNoneSession(0))

	errCode, _ := svc.checkPromptNone(promptNoneCtx(), &oauth2model.OAuthParameters{
		Prompt: oauth2const.PromptNone,
		MaxAge: "0",
	}, promptNoneApp())

	suite.Equal(oauth2const.ErrorLoginRequired, errCode,
		"max_age=0 cannot be answered from an existing session")
}

// promptNoneParams returns a prompt=none request.
func promptNoneParams() *oauth2model.OAuthParameters {
	return &oauth2model.OAuthParameters{
		Prompt:      oauth2const.PromptNone,
		RedirectURI: "https://client.example.com/callback",
		State:       "state-1",
	}
}

// TestTryCompleteWithSSOSession_FlowNeedsUser: a flow that needs input returns login_required.
func (suite *AuthorizeServiceTestSuite) TestTryCompleteWithSSOSession_FlowNeedsUser() {
	svc := suite.newService()
	suite.mockFlowExecService.EXPECT().Execute(mock.Anything, promptNoneAppID, "exec-1",
		string(providers.FlowTypeAuthentication), false, "", mock.Anything, "", "", "").
		Return(&flowexec.FlowStep{Status: providers.FlowStatusIncomplete}, nil)
	suite.mockAuthReqStore.EXPECT().ClearRequest(mock.Anything, "auth-1").Return(nil)

	redirectURI, authErr := svc.tryCompleteWithSSOSession(context.Background(), promptNoneParams(),
		promptNoneApp(), "auth-1", "exec-1")

	suite.Empty(redirectURI)
	suite.Require().NotNil(authErr)
	suite.Equal(oauth2const.ErrorLoginRequired, authErr.Code)
	suite.True(authErr.SendErrorToClient, "the error must go back to the client, not to a page")
	suite.Equal("https://client.example.com/callback", authErr.ClientRedirectURI)
	suite.Equal("state-1", authErr.State)
}

// TestTryCompleteWithSSOSession_ExecuteFails: a flow engine error without an assertion returns
// server_error when server errors may reach the client, and login_required otherwise.
func (suite *AuthorizeServiceTestSuite) TestTryCompleteWithSSOSession_ExecuteFails() {
	for enabled, code := range map[bool]string{
		true:  oauth2const.ErrorServerError,
		false: oauth2const.ErrorLoginRequired,
	} {
		suite.Run(fmt.Sprintf("enabled=%t", enabled), func() {
			suite.SetupTest()
			svc := suite.newService()
			svc.cfg.OAuth.SendServerErrorsToClient = new(enabled)
			suite.mockFlowExecService.EXPECT().Execute(mock.Anything, promptNoneAppID, "exec-1",
				string(providers.FlowTypeAuthentication), false, "", mock.Anything, "", "", "").
				Return(nil, &tidcommon.ServiceError{Code: "FES-1001"})
			suite.mockAuthReqStore.EXPECT().ClearRequest(mock.Anything, "auth-1").Return(nil)

			redirectURI, authErr := svc.tryCompleteWithSSOSession(context.Background(), promptNoneParams(),
				promptNoneApp(), "auth-1", "exec-1")

			suite.Empty(redirectURI)
			suite.Require().NotNil(authErr)
			suite.Equal(code, authErr.Code)
			suite.True(authErr.SendErrorToClient)
			suite.Equal("state-1", authErr.State)
		})
	}
}

// TestTryCompleteWithSSOSession_ServerErrorAssertion: a flow server error goes to the client when
// server errors may reach it, and is answered with login_required instead of the error page
// otherwise.
func (suite *AuthorizeServiceTestSuite) TestTryCompleteWithSSOSession_ServerErrorAssertion() {
	for enabled, code := range map[bool]string{
		true:  oauth2const.ErrorServerError,
		false: oauth2const.ErrorLoginRequired,
	} {
		suite.Run(fmt.Sprintf("enabled=%t", enabled), func() {
			suite.SetupTest()
			svc := suite.newService()
			svc.cfg.OAuth.SendServerErrorsToClient = new(enabled)
			suite.mockFlowExecService.EXPECT().Execute(mock.Anything, promptNoneAppID, "exec-1",
				string(providers.FlowTypeAuthentication), false, "", mock.Anything, "", "", "").
				Return(&flowexec.FlowStep{ErrorAssertion: errAssertionServerError},
					&tidcommon.ServiceError{Code: "FES-1001"})
			suite.mockJWTService.EXPECT().VerifyJWT(mock.Anything, errAssertionServerError, "", "").Return(nil)
			suite.mockAuthReqStore.EXPECT().
				GetRequest(mock.Anything, testAuthID).Return(true, failedCallbackAuthCtx(), nil)
			clears := 1
			if !enabled {
				clears = 2
			}
			suite.mockAuthReqStore.EXPECT().ClearRequest(mock.Anything, testAuthID).Return(nil).Times(clears)

			redirectURI, authErr := svc.tryCompleteWithSSOSession(context.Background(), promptNoneParams(),
				promptNoneApp(), testAuthID, "exec-1")

			suite.Empty(redirectURI)
			suite.Require().NotNil(authErr)
			suite.Equal(code, authErr.Code)
			suite.True(authErr.SendErrorToClient, "a silent request must never end on the error page")
		})
	}
}

// TestTryCompleteWithSSOSession_UnverifiableErrorAssertion: an error assertion the callback rejects
// before consuming the request is answered with login_required, and the request is cleared.
func (suite *AuthorizeServiceTestSuite) TestTryCompleteWithSSOSession_UnverifiableErrorAssertion() {
	svc := suite.newService()
	suite.mockFlowExecService.EXPECT().Execute(mock.Anything, promptNoneAppID, "exec-1",
		string(providers.FlowTypeAuthentication), false, "", mock.Anything, "", "", "").
		Return(&flowexec.FlowStep{Status: providers.FlowStatusError, ErrorAssertion: "not-a-jwt"}, nil)
	suite.mockJWTService.EXPECT().VerifyJWT(mock.Anything, "not-a-jwt", "", "").
		Return(&tidcommon.ServiceError{Code: "JWT-1001"})
	suite.mockAuthReqStore.EXPECT().ClearRequest(mock.Anything, "auth-1").Return(nil).Once()

	redirectURI, authErr := svc.tryCompleteWithSSOSession(context.Background(), promptNoneParams(),
		promptNoneApp(), "auth-1", "exec-1")

	suite.Empty(redirectURI)
	suite.Require().NotNil(authErr)
	suite.Equal(oauth2const.ErrorLoginRequired, authErr.Code)
	suite.True(authErr.SendErrorToClient)
}

// TestTryCompleteWithSSOSession_CallbackServerError: a server error from exchanging a completed
// flow's assertion goes to the client when server errors may reach it, and is answered with
// login_required otherwise.
func (suite *AuthorizeServiceTestSuite) TestTryCompleteWithSSOSession_CallbackServerError() {
	for enabled, code := range map[bool]string{
		true:  oauth2const.ErrorServerError,
		false: oauth2const.ErrorLoginRequired,
	} {
		suite.Run(fmt.Sprintf("enabled=%t", enabled), func() {
			suite.SetupTest()
			svc := suite.newService()
			svc.cfg.OAuth.SendServerErrorsToClient = new(enabled)
			suite.mockFlowExecService.EXPECT().Execute(mock.Anything, promptNoneAppID, "exec-1",
				string(providers.FlowTypeAuthentication), false, "", mock.Anything, "", "", "").
				Return(&flowexec.FlowStep{Status: providers.FlowStatusComplete, Assertion: svcJWTWithIat}, nil)
			suite.mockJWTService.EXPECT().VerifyJWT(mock.Anything, svcJWTWithIat, "", "").Return(nil)
			suite.mockAuthReqStore.EXPECT().GetRequest(mock.Anything, testAuthID).Return(true, authRequestContext{
				OAuthParameters: oauth2model.OAuthParameters{
					ClientID:    "test-client",
					RedirectURI: "https://client.example.com/callback",
				},
			}, nil)
			suite.mockAuthReqStore.EXPECT().ClearRequest(mock.Anything, testAuthID).Return(nil)
			suite.mockAuthzCodeStore.EXPECT().InsertAuthorizationCode(mock.Anything, mock.Anything).
				Return(errors.New("db error"))

			redirectURI, authErr := svc.tryCompleteWithSSOSession(context.Background(), promptNoneParams(),
				promptNoneApp(), testAuthID, "exec-1")

			suite.Empty(redirectURI)
			suite.Require().NotNil(authErr)
			suite.Equal(code, authErr.Code)
			suite.True(authErr.SendErrorToClient, "a silent request must never end on the error page")
			suite.Equal("state-1", authErr.State)
		})
	}
}

// TestTryCompleteWithSSOSession_FlowFails: a flow failure's error assertion is mapped by the
// callback, like any other flow failure.
func (suite *AuthorizeServiceTestSuite) TestTryCompleteWithSSOSession_FlowFails() {
	svc := suite.newService()
	suite.mockFlowExecService.EXPECT().Execute(mock.Anything, promptNoneAppID, "exec-1",
		string(providers.FlowTypeAuthentication), false, "", mock.Anything, "", "", "").
		Return(&flowexec.FlowStep{Status: providers.FlowStatusError, ErrorAssertion: errAssertionEndUser}, nil)
	suite.mockJWTService.EXPECT().VerifyJWT(mock.Anything, errAssertionEndUser, "", "").Return(nil)
	suite.mockAuthReqStore.EXPECT().
		GetRequest(mock.Anything, testAuthID).Return(true, failedCallbackAuthCtx(), nil)
	suite.mockAuthReqStore.EXPECT().ClearRequest(mock.Anything, testAuthID).Return(nil)

	redirectURI, authErr := svc.tryCompleteWithSSOSession(context.Background(), promptNoneParams(),
		promptNoneApp(), testAuthID, "exec-1")

	suite.Empty(redirectURI)
	suite.Require().NotNil(authErr)
	suite.Equal(oauth2const.ErrorAccessDenied, authErr.Code)
	suite.True(authErr.SendErrorToClient)
}

// TestTryCompleteWithSSOSession_CallbackFails: a rejected assertion is redirected to the client.
func (suite *AuthorizeServiceTestSuite) TestTryCompleteWithSSOSession_CallbackFails() {
	svc := suite.newService()
	suite.mockFlowExecService.EXPECT().Execute(mock.Anything, promptNoneAppID, "exec-1",
		string(providers.FlowTypeAuthentication), false, "", mock.Anything, "", "", "").
		Return(&flowexec.FlowStep{Status: providers.FlowStatusComplete, Assertion: "not-a-jwt"}, nil)
	suite.mockJWTService.EXPECT().VerifyJWT(mock.Anything, "not-a-jwt", "", "").
		Return(&tidcommon.ServiceError{Code: "JWT-1001"})
	suite.mockAuthReqStore.EXPECT().ClearRequest(mock.Anything, "auth-1").Return(nil).Once()

	redirectURI, authErr := svc.tryCompleteWithSSOSession(context.Background(), promptNoneParams(),
		promptNoneApp(), "auth-1", "exec-1")

	suite.Empty(redirectURI)
	suite.Require().NotNil(authErr)
	suite.True(authErr.SendErrorToClient)
	suite.Equal("https://client.example.com/callback", authErr.ClientRedirectURI)
	suite.Equal("state-1", authErr.State)
}

// TestHandleInitialAuthorizationRequest_PromptNoneRedirectsToClient: prompt=none over a live
// session returns the client redirect with the code instead of login page params.
func (suite *AuthorizeServiceTestSuite) TestHandleInitialAuthorizationRequest_PromptNoneRedirectsToClient() {
	app := suite.testApp()
	app.ID = promptNoneAppID
	suite.mockInboundClient.EXPECT().GetOAuthClientByClientID(mock.Anything, "test-client-id").Return(app, nil)
	suite.mockValidator.On("validateInitialAuthorizationRequest", mock.Anything, mock.Anything, app).
		Return(false, "", "")
	svc := suite.wirePromptNone(promptNoneSession(time.Minute))

	suite.mockAuthReqStore.EXPECT().AddRequest(mock.Anything, mock.Anything).Return(testAuthID, nil)
	suite.mockFlowExecService.EXPECT().InitiateFlow(mock.Anything, mock.Anything).Return("exec-1", nil)
	suite.mockFlowExecService.EXPECT().Execute(mock.Anything, promptNoneAppID, "exec-1",
		string(providers.FlowTypeAuthentication), false, "", mock.Anything, "", "", "").
		Return(&flowexec.FlowStep{Status: providers.FlowStatusComplete, Assertion: svcJWTWithIat}, nil)

	suite.mockJWTService.EXPECT().VerifyJWT(mock.Anything, svcJWTWithIat, "", "").Return(nil)
	suite.mockAuthReqStore.EXPECT().GetRequest(mock.Anything, testAuthID).Return(true, authRequestContext{
		OAuthParameters: oauth2model.OAuthParameters{
			ClientID:    "test-client-id",
			RedirectURI: "https://client.example.com/callback",
			State:       "test-state",
		},
	}, nil)
	suite.mockAuthReqStore.EXPECT().ClearRequest(mock.Anything, testAuthID).Return(nil)
	suite.mockAuthzCodeStore.EXPECT().InsertAuthorizationCode(mock.Anything, mock.Anything).Return(nil)

	msg := suite.testMsg()
	msg.RequestQueryParams["prompt"] = []string{oauth2const.PromptNone}

	result, authErr := svc.HandleInitialAuthorizationRequest(promptNoneCtx(), msg)

	suite.Require().Nil(authErr)
	suite.Require().NotNil(result)
	suite.Contains(result.RedirectURI, "https://client.example.com/callback")
	suite.Contains(result.RedirectURI, "code=")
	suite.Contains(result.RedirectURI, "state=test-state")
	suite.Empty(result.QueryParams, "the login page must not be used")
}

// TestHandleInitialAuthorizationRequest_PromptNoneFlowNeedsUser: prompt=none whose flow needs
// input returns login_required to the client.
func (suite *AuthorizeServiceTestSuite) TestHandleInitialAuthorizationRequest_PromptNoneFlowNeedsUser() {
	app := suite.testApp()
	app.ID = promptNoneAppID
	suite.mockInboundClient.EXPECT().GetOAuthClientByClientID(mock.Anything, "test-client-id").Return(app, nil)
	suite.mockValidator.On("validateInitialAuthorizationRequest", mock.Anything, mock.Anything, app).
		Return(false, "", "")
	svc := suite.wirePromptNone(promptNoneSession(time.Minute))

	suite.mockAuthReqStore.EXPECT().AddRequest(mock.Anything, mock.Anything).Return(testAuthID, nil)
	suite.mockFlowExecService.EXPECT().InitiateFlow(mock.Anything, mock.Anything).Return("exec-1", nil)
	suite.mockFlowExecService.EXPECT().Execute(mock.Anything, promptNoneAppID, "exec-1",
		string(providers.FlowTypeAuthentication), false, "", mock.Anything, "", "", "").
		Return(&flowexec.FlowStep{Status: providers.FlowStatusIncomplete}, nil)
	suite.mockAuthReqStore.EXPECT().ClearRequest(mock.Anything, testAuthID).Return(nil)

	msg := suite.testMsg()
	msg.RequestQueryParams["prompt"] = []string{oauth2const.PromptNone}

	result, authErr := svc.HandleInitialAuthorizationRequest(promptNoneCtx(), msg)

	suite.Nil(result)
	suite.Require().NotNil(authErr)
	suite.Equal(oauth2const.ErrorLoginRequired, authErr.Code)
	suite.True(authErr.SendErrorToClient)
	suite.Equal("test-state", authErr.State)
}
