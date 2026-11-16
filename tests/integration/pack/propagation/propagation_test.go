// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package propagation

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/pack/packtest"
)

// managedUserCode is the error a data plane answers with to a change of a user the control plane
// deployed. Only a deploy from the control plane changes or removes one.
const managedUserCode = "USR-1044"

type PropagationTestSuite struct {
	packtest.Suite
}

func TestPropagationTestSuite(t *testing.T) {
	suite.Run(t, new(PropagationTestSuite))
}

// A change made on the control plane reaches the data plane both ways: an update shows there, and a
// delete removes it.
func (ts *PropagationTestSuite) TestAnUpdateAndADeleteOnTheControlPlaneReachTheDataPlane() {
	clientID := packtest.Unique("pack-propagate")
	appID := ts.CreateClientCredentialsApp(clientID, packtest.Unique("pack-secret"))
	ts.Sync()

	var held map[string]any
	status, raw := ts.Call(ts.DP, http.MethodGet, "/applications/"+appID, nil, &held)
	ts.Require().Equal(http.StatusOK, status, string(raw))

	// The change is made on the control plane's own copy of the application.
	var current map[string]any
	status, raw = ts.Call(ts.CP, http.MethodGet, "/applications/"+appID, nil, &current)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	renamed := clientID + "-renamed"
	current["name"] = renamed
	status, raw = ts.Call(ts.CP, http.MethodPut, "/applications/"+appID, current, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Sync()

	status, raw = ts.Call(ts.DP, http.MethodGet, "/applications/"+appID, nil, &held)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Equal(renamed, held["name"], "the update did not reach the data plane")

	status, raw = ts.Call(ts.CP, http.MethodDelete, "/applications/"+appID, nil, nil)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))
	ts.Sync()

	status, raw = ts.Call(ts.DP, http.MethodGet, "/applications/"+appID, nil, nil)
	ts.Equal(http.StatusNotFound, status, "the delete did not reach the data plane: %s", raw)
}

// A user the control plane syncs is read-only on the data plane: the data plane refuses to change or
// remove it, since the control plane owns it.
func (ts *PropagationTestSuite) TestAUserSyncedFromTheControlPlaneIsReadOnlyOnTheDataPlane() {
	username := packtest.Unique("pack-managed")
	var user struct {
		ID string `json:"id"`
	}
	status, raw := ts.Call(ts.CP, http.MethodPost, "/users", map[string]any{
		"type":       "person",
		"ouId":       packtest.DefaultOUID,
		"attributes": map[string]any{"username": username, "email": username + "@example.com"},
	}, &user)
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	ts.RemoveOnCleanup("/users/" + user.ID)
	ts.Sync()

	status, raw = ts.Call(ts.DP, http.MethodGet, "/users/"+user.ID, nil, nil)
	ts.Require().Equal(http.StatusOK, status, "the user did not reach the data plane: %s", raw)

	status, raw = ts.Call(ts.DP, http.MethodPut, "/users/"+user.ID, map[string]any{
		"type":       "person",
		"ouId":       packtest.DefaultOUID,
		"attributes": map[string]any{"username": username, "email": "edited-" + username + "@example.com"},
	}, nil)
	ts.True(status >= http.StatusBadRequest && status < http.StatusInternalServerError &&
		strings.Contains(string(raw), managedUserCode),
		"the data plane did not refuse an edit with %s: %d %s", managedUserCode, status, raw)

	status, raw = ts.Call(ts.DP, http.MethodDelete, "/users/"+user.ID, nil, nil)
	ts.True(status >= http.StatusBadRequest && status < http.StatusInternalServerError &&
		strings.Contains(string(raw), managedUserCode),
		"the data plane did not refuse a delete with %s: %d %s", managedUserCode, status, raw)
}

// A trusted issuer that accepts ID-JAGs without token exchange deploys to the data plane with ID-JAG
// acceptance still on.
func (ts *PropagationTestSuite) TestATrustedIssuerAcceptingIDJAGsReachesTheDataPlane() {
	issuer := "https://" + packtest.Unique("idjag-issuer") + ".example.com"
	var created struct {
		ID string `json:"id"`
	}
	status, raw := ts.Call(ts.CP, http.MethodPost, "/connections/oidc", map[string]any{
		"name":                 packtest.Unique("pack-idjag"),
		"ouId":                 packtest.DefaultOUID,
		"issuer":               issuer,
		"jwksEndpoint":         issuer + "/oauth2/jwks",
		"tokenExchangeEnabled": false,
		"idJagEnabled":         true,
	}, &created)
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	ts.RemoveOnCleanup("/connections/oidc/" + created.ID)
	ts.Sync()

	var held struct {
		Issuer               string `json:"issuer"`
		TokenExchangeEnabled *bool  `json:"tokenExchangeEnabled"`
		IDJagEnabled         *bool  `json:"idJagEnabled"`
	}
	status, raw = ts.Call(ts.DP, http.MethodGet, "/connections/oidc/"+created.ID, nil, &held)
	ts.Require().Equal(http.StatusOK, status, "the trusted issuer did not reach the data plane: %s", raw)
	ts.Equal(issuer, held.Issuer)
	ts.Require().NotNil(held.IDJagEnabled, "ID-JAG acceptance was dropped on the way to the data plane: %s", raw)
	ts.True(*held.IDJagEnabled)
	ts.Require().NotNil(held.TokenExchangeEnabled)
	ts.False(*held.TokenExchangeEnabled)
}
