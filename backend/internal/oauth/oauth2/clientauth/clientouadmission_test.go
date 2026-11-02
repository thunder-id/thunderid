// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package clientauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/actorprovidermock"
)

// ClientOUAdmissionMiddlewareTestSuite covers the step that decides whether the authenticated client may
// act for the organization unit a request named. It is the gate that used to sit inside client
// resolution, made explicit so a route can be registered only where it is wired in.
type ClientOUAdmissionMiddlewareTestSuite struct {
	suite.Suite
}

func TestClientOUAdmissionMiddlewareTestSuite(t *testing.T) {
	suite.Run(t, new(ClientOUAdmissionMiddlewareTestSuite))
}

const admissionClientID = "m2m-client"

// serve runs one request through the middleware and reports what happened.
func (s *ClientOUAdmissionMiddlewareTestSuite) serve(
	actorProvider providers.ActorProvider, accessingOUID string, withClient bool,
) (reached bool, recorder *httptest.ResponseRecorder, body map[string]any) {
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true })

	ctx := context.Background()
	if withClient {
		ctx = withOAuthClient(ctx, &OAuthClientInfo{
			ClientID: admissionClientID,
			OAuthApp: &providers.OAuthClient{ID: "m2m-app", ClientID: admissionClientID},
		})
	}
	if accessingOUID != "" {
		ctx = syscontext.WithAccessingOUID(ctx, accessingOUID)
	}

	recorder = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/oauth2/token", nil).WithContext(ctx)
	ClientOUAdmissionMiddleware(actorProvider)(next).ServeHTTP(recorder, req)

	_ = json.Unmarshal(recorder.Body.Bytes(), &body)
	return reached, recorder, body
}

// answering returns a provider giving the same answer to every question.
func (s *ClientOUAdmissionMiddlewareTestSuite) answering(
	accessible bool, svcErr *tidcommon.ServiceError,
) providers.ActorProvider {
	actors := actorprovidermock.NewActorProviderMock(s.T())
	actors.EXPECT().IsOAuthClientAccessibleFromOU(mock.Anything, mock.Anything, mock.Anything).
		Return(accessible, svcErr).Maybe()
	return actors
}

// A request naming no organization unit is not this middleware's business, and is never asked
// about, so the bare token endpoint is unaffected wherever the middleware is mounted.
func (s *ClientOUAdmissionMiddlewareTestSuite) TestNoAccessingOUPassesThrough() {
	// No expectation is set, so any question put to the provider fails the test on the spot.
	actors := actorprovidermock.NewActorProviderMock(s.T())

	reached, _, _ := s.serve(actors, "", true)

	s.True(reached)
}

// An admitted client reaches the handler.
func (s *ClientOUAdmissionMiddlewareTestSuite) TestAnAdmittedClientReachesTheHandler() {
	reached, recorder, _ := s.serve(s.answering(true, nil), "customer-a", true)

	s.True(reached)
	s.Equal(http.StatusOK, recorder.Code)
}

// A refused client does not, and is answered in OAuth2's envelope with the same text a request
// naming an unknown organization unit receives, so the two cannot be told apart.
func (s *ClientOUAdmissionMiddlewareTestSuite) TestARefusedClientIsStoppedBeforeTheHandler() {
	reached, recorder, body := s.serve(s.answering(false, nil), "customer-a", true)

	s.False(reached, "the request never reaches the token handler")
	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Equal(constants.ErrorUnauthorizedClient, body["error"])
	s.Equal(constants.OUAccessRefusal, body["error_description"])
}

// A provider that could not answer is a broken dependency, not a policy decision. Reporting it as
// a refusal would tell the caller it may not act for the unit, which nobody established, and hide
// the fault from whoever watches for server errors.
func (s *ClientOUAdmissionMiddlewareTestSuite) TestAnUnresolvedAnswerIsAServerError() {
	reached, recorder, body := s.serve(
		s.answering(false, &tidcommon.InternalServerError), "customer-a", true)

	s.False(reached)
	s.Equal(http.StatusInternalServerError, recorder.Code)
	s.NotEqual(constants.OUAccessRefusal, body["error_description"],
		"a broken dependency must not read as a refusal")
}

// Fail closed. A middleware mounted without an answer, or reached before client authentication,
// must refuse rather than admit: admitting would hand out a token for an organization unit nobody
// established the client may act for.
func (s *ClientOUAdmissionMiddlewareTestSuite) TestItFailsClosed() {
	for _, tc := range []struct {
		name       string
		access     providers.ActorProvider
		withClient bool
	}{
		{"no actor provider wired", nil, true},
		{"no authenticated client", s.answering(true, nil), false},
	} {
		reached, recorder, body := s.serve(tc.access, "customer-a", tc.withClient)

		s.False(reached, tc.name)
		s.Equal(http.StatusBadRequest, recorder.Code, tc.name)
		s.Equal(constants.OUAccessRefusal, body["error_description"], tc.name)
	}
}
