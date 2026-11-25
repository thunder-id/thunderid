// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/clientauth"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// A DPoP proof as a client sends it, which no event may carry.
const testAuditDPoPProof = "eyJ0eXAiOiJkcG9wK2p3dCJ9.eyJodG0iOiJQT1NUIn0.c2lnbmF0dXJlLWRwb3A"

type TokenHandlerAuditTestSuite struct {
	suite.Suite
}

func TestTokenHandlerAuditTestSuite(t *testing.T) {
	suite.Run(t, new(TokenHandlerAuditTestSuite))
}

// A request the handler refuses before the token service sees it is one token.issue event by the
// authenticated client, failed with the error it was answered with. The service, which publishes for
// every request it sees, is never reached, so nothing publishes twice.
func (suite *TokenHandlerAuditTestSuite) TestEveryRefusalOfTheHandlerPublishesOneEvent() {
	cases := []struct {
		name          string
		grantType     string
		dpopProofs    []string
		accessingOUID string
		outcome       audit.Outcome
	}{
		{
			name: "two DPoP proofs", grantType: string(providers.GrantTypeClientCredentials),
			dpopProofs: []string{testAuditDPoPProof, testAuditDPoPProof},
			outcome:    audit.OutcomeFailed(constants.ErrorInvalidDPoPProof),
		},
		{
			name:      "a grant the organization-unit-scoped endpoint does not serve",
			grantType: string(providers.GrantTypeRefreshToken), accessingOUID: "01900000-0000-7000-8000-000000000063",
			outcome: audit.OutcomeFailed(constants.ErrorInvalidRequest),
		},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			bus, published := newRecordingBus(suite.T())
			service := NewTokenServiceInterfaceMock(suite.T())
			form := url.Values{constants.RequestParamGrantType: {tc.grantType},
				"refresh_token": {testAuditRefreshToken}}
			request := httptest.NewRequest(http.MethodPost, "/oauth2/token", strings.NewReader(form.Encode()))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			for _, proof := range tc.dpopProofs {
				request.Header.Add(constants.HeaderDPoP, proof)
			}
			ctx := context.WithValue(request.Context(), clientauth.OAuthClientKey, &clientauth.OAuthClientInfo{
				ClientID: testAuditClientID, ClientSecret: testAuditSecret,
				OAuthApp: client(providers.EntityCategoryApp, providers.GrantType(tc.grantType)),
			})
			if tc.accessingOUID != "" {
				ctx = syscontext.WithAccessingOUID(ctx, tc.accessingOUID)
			}
			recorder := httptest.NewRecorder()

			newTokenHandler(service, nil, bus).HandleTokenRequest(recorder, request.WithContext(ctx))

			suite.Require().Equal(http.StatusBadRequest, recorder.Code)
			events := published()
			suite.Require().Len(events, 1)
			event := events[0]
			suite.Equal(audit.EventTokenIssue, event.name)
			suite.Equal(audit.Ref{Type: audit.TargetTypeToken}, event.payload.Target())
			suite.Equal(audit.Actor{Type: audit.ActorTypeApplication, ID: testAuditAppID}, event.payload.Actor())
			suite.Equal(audit.Ref{}, event.payload.Subject())
			suite.Equal(tc.outcome, event.payload.Outcome())
			suite.Equal(map[string]string{audit.DetailGrantType: tc.grantType}, event.payload.Details())
			requireNoCredential(suite.T(), event)
			suite.NotContains(fmt.Sprintf("%+v", event.payload), testAuditDPoPProof, "a DPoP proof is never recorded")
		})
	}
}
