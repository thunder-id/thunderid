// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package clientauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/actorprovider"
	"github.com/thunder-id/thunderid/internal/application"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

const refusalOUID = "ou-customer"

// ouAccessStub refuses every organization unit but the application's own. The embedded interface
// supplies the application-service methods client resolution never reaches.
type ouAccessStub struct {
	application.ApplicationServiceInterface
	visible bool
}

func (s ouAccessStub) IsApplicationVisibleToOU(
	_ context.Context, _, _ string,
) (bool, *tidcommon.ServiceError) {
	return s.visible, nil
}

// OURefusalTestSuite pins what a caller sees when it names an organization unit it may not act for.
//
// Both ways of failing that get one answer: an organization unit that does not exist and one the
// client has no standing in are indistinguishable, so the endpoint cannot be used to enumerate
// organization units. A wrong secret stays distinct, because that is a different failure and a
// caller needs to tell a bad credential from a missing policy.
type OURefusalTestSuite struct {
	suite.Suite
	mockInboundClient *inboundclientmock.InboundClientServiceInterfaceMock
}

func TestOURefusalTestSuite(t *testing.T) {
	suite.Run(t, new(OURefusalTestSuite))
}

func (suite *OURefusalTestSuite) SetupTest() {
	suite.mockInboundClient = inboundclientmock.NewInboundClientServiceInterfaceMock(suite.T())
}

// authenticate runs the middleware over a client_secret_post request made for an organization unit
// the client may not act for. Only the refusal is exercised here: it is decided during client
// resolution, before any credential is verified, so the request never reaches the parts of
// authentication this stub does not stand up. The admitted path is covered in internal/actorprovider,
// where resolution can be driven directly.
func (suite *OURefusalTestSuite) authenticate() (int, map[string]any) {
	suite.T().Helper()

	suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, testClientID).
		Return(&providers.OAuthClient{
			ID:                      "app-1",
			ClientID:                testClientID,
			OUID:                    "ou-owner",
			TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretPost,
		}, nil).Maybe()

	actors := actorprovider.Initialize(suite.mockInboundClient,
		entityprovidermock.NewEntityProviderInterfaceMock(suite.T()), noopAuthnMgr(), nil,
		ouAccessStub{visible: false})

	form := url.Values{}
	form.Set(constants.RequestParamClientID, testClientID)
	form.Set(constants.RequestParamClientSecret, testClientSecret)

	req := httptest.NewRequest(http.MethodPost, "/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(syscontext.WithAccessingOUID(req.Context(), refusalOUID))

	recorder := httptest.NewRecorder()
	middleware := ClientAuthMiddleware(actors, noopAuthnMgr(),
		jwtmock.NewJWTServiceInterfaceMock(suite.T()), nil, "https://localhost", 0)
	middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(recorder, req)

	body := map[string]any{}
	if recorder.Body.Len() > 0 {
		_ = json.Unmarshal(recorder.Body.Bytes(), &body)
	}
	return recorder.Code, body
}

// The case this exists for: the organization unit is real, the credentials are fine, and the client
// simply has no standing there. The organization unit is echoed back, which leaks nothing because it
// is the caller's own input.
func (suite *OURefusalTestSuite) TestUnauthorizedOUIsRefusedAsAnInvalidRequest() {
	status, body := suite.authenticate()

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal(constants.ErrorInvalidRequest, body["error"])
	suite.Equal("Client is not authorized to access the OU:"+refusalOUID+" resources",
		body["error_description"])
}

// The invariant the whole arrangement rests on: what a client with no standing receives must be
// byte-identical to what an organization unit that does not exist receives.
//
// The two are produced in different packages, and a difference in either the code or the wording
// would reopen the distinction. Both read the same helper, and this asserts they still agree.
func TestTheTwoOURefusalsAreIndistinguishable(t *testing.T) {
	noStanding := errClientNotAuthorizedForOU(refusalOUID)

	require.Equal(t, constants.ErrorInvalidRequest, noStanding.ErrorCode)
	require.Equal(t, constants.OUAccessRefusalDescription(refusalOUID), noStanding.ErrorDescription,
		"the refusal must be the one an unknown organization unit also gets")
	require.Equal(t, http.StatusBadRequest, noStanding.StatusCode)

	// A wrong secret stays its own answer: collapsing that too would leave a caller unable to tell a
	// bad credential from a missing policy.
	require.NotEqual(t, errInvalidClientCredentials.ErrorCode, noStanding.ErrorCode)
}
