// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	selfPasskeyRelyingPartyID = "localhost"
	selfPasskeyOrigin         = "https://localhost:8095"
	// selfPasskeyAndroidOrigin is the origin an Android app's passkey prompt reports. The server only
	// compares it with the app's allowed origins, so any signing certificate hash works here.
	selfPasskeyAndroidOrigin   = "android:apk-key-hash:3q2-796tvu_erb7v3q2-796tvu_erb7v3q2-796tvu8"
	selfPasskeyAndroidClientID = "self_passkey_android_client"
	selfPasskeyAndroidRedirect = "https://localhost:3000/callback"
	selfPasskeyOwnerUsername   = "self.passkey.owner"
	selfPasskeyOwnerPassword   = "OwnerP@ssw0rd!"
)

// selfPasskeyUser is a user the suite signs in as.
type selfPasskeyUser struct {
	id     string
	client *http.Client
}

// selfPasskey is one entry of the passkey list.
type selfPasskey struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// SelfPasskeyAPITestSuite covers the endpoints a signed-in user calls to add, list and remove their
// own passkeys.
type SelfPasskeyAPITestSuite struct {
	suite.Suite
	ouID         string
	schemaID     string
	androidAppID string
	owner        selfPasskeyUser
	other        selfPasskeyUser
	// ownerOnAndroid is the owner signed in through an Android app whose only passkey origin is the
	// app's signing certificate hash.
	ownerOnAndroid selfPasskeyUser
}

func TestSelfPasskeyAPITestSuite(t *testing.T) {
	suite.Run(t, new(SelfPasskeyAPITestSuite))
}

func (s *SelfPasskeyAPITestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "self-passkey-ou",
		Name:   "Self Passkey OU",
	})
	s.Require().NoError(err)
	s.ouID = ouID

	schemaID, err := testutils.CreateUserType(testutils.UserType{
		Handle:      "self-passkey-user",
		DisplayName: "Self Passkey User",
		OUID:        ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string", "required": true, "unique": true},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	})
	s.Require().NoError(err)
	s.schemaID = schemaID

	s.owner = s.createUser(selfPasskeyOwnerUsername, selfPasskeyOwnerPassword)
	s.other = s.createUser("self.passkey.other", "OtherP@ssw0rd!")

	appID, err := testutils.CreateApplication(testutils.Application{
		Name:                  "Self Passkey Android App",
		Description:           "Android app for self-service passkey tests",
		OUID:                  ouID,
		Type:                  "mobile",
		AllowedUserTypes:      []string{"self-passkey-user"},
		PasskeyAllowedOrigins: []string{selfPasskeyAndroidOrigin},
		PasskeyRelyingPartyID: selfPasskeyRelyingPartyID,
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                selfPasskeyAndroidClientID,
				"redirectUris":            []string{selfPasskeyAndroidRedirect},
				"grantTypes":              []string{"authorization_code"},
				"responseTypes":           []string{"code"},
				"tokenEndpointAuthMethod": "none",
				"publicClient":            true,
				"pkceRequired":            true,
			},
		}},
	})
	s.Require().NoError(err)
	s.androidAppID = appID

	tokenResp, err := testutils.ObtainAccessTokenWithPassword(selfPasskeyAndroidClientID,
		selfPasskeyAndroidRedirect, "openid", selfPasskeyOwnerUsername, selfPasskeyOwnerPassword, true)
	s.Require().NoError(err)
	s.ownerOnAndroid = selfPasskeyUser{id: s.owner.id, client: testutils.GetHTTPClientWithToken(tokenResp.AccessToken)}
}

func (s *SelfPasskeyAPITestSuite) TearDownSuite() {
	if s.androidAppID != "" {
		s.NoError(testutils.DeleteApplication(s.androidAppID))
	}
	for _, id := range []string{s.owner.id, s.other.id} {
		if id != "" {
			s.NoError(testutils.DeleteUser(id))
		}
	}
	if s.schemaID != "" {
		s.NoError(testutils.DeleteUserType(s.schemaID))
	}
	if s.ouID != "" {
		s.NoError(testutils.DeleteOrganizationUnit(s.ouID))
	}
}

// createUser creates a user and signs in as them.
func (s *SelfPasskeyAPITestSuite) createUser(username, password string) selfPasskeyUser {
	attrs, err := json.Marshal(map[string]interface{}{"username": username, "password": password})
	s.Require().NoError(err)
	id, err := testutils.CreateUser(testutils.User{OUID: s.ouID, Type: "self-passkey-user", Attributes: attrs})
	s.Require().NoError(err)
	client, err := testutils.GetHTTPClientForUser(username, password)
	s.Require().NoError(err)
	return selfPasskeyUser{id: id, client: client}
}

// do sends a request as the given user and returns the status and body.
func (s *SelfPasskeyAPITestSuite) do(user selfPasskeyUser, method, path string, payload interface{}) (
	int, []byte) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		s.Require().NoError(err)
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, testutils.TestServerURL+path, body)
	s.Require().NoError(err)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := user.client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	return resp.StatusCode, raw
}

// requireErrorCode asserts the body carries the error code.
func (s *SelfPasskeyAPITestSuite) requireErrorCode(body []byte, code string) {
	var errResp struct {
		Code string `json:"code"`
	}
	s.Require().NoError(json.Unmarshal(body, &errResp), string(body))
	s.Equal(code, errResp.Code, string(body))
}

// start starts adding a passkey and returns the challenge and session token.
func (s *SelfPasskeyAPITestSuite) start(user selfPasskeyUser) (string, string) {
	status, body := s.do(user, http.MethodPost, "/users/me/passkeys/register/start",
		map[string]string{"relyingPartyId": selfPasskeyRelyingPartyID})
	s.Require().Equal(http.StatusOK, status, string(body))

	var resp struct {
		PublicKeyCredentialCreationOptions struct {
			Challenge string `json:"challenge"`
		} `json:"publicKeyCredentialCreationOptions"`
		SessionToken string `json:"sessionToken"`
	}
	s.Require().NoError(json.Unmarshal(body, &resp))
	return resp.PublicKeyCredentialCreationOptions.Challenge, resp.SessionToken
}

// finishPayload builds a finish request for a passkey the authenticator creates.
func (s *SelfPasskeyAPITestSuite) finishPayload(authenticator *testutils.VirtualAuthenticator,
	challenge, sessionToken, name string) map[string]interface{} {
	credentialID, clientDataJSON, attestationObject, err := authenticator.CreateAttestationResponse(challenge, true)
	s.Require().NoError(err)
	return map[string]interface{}{
		"publicKeyCredential": map[string]interface{}{
			"id":   credentialID,
			"type": "public-key",
			"response": map[string]string{
				"clientDataJSON":    clientDataJSON,
				"attestationObject": attestationObject,
			},
		},
		"sessionToken": sessionToken,
		"name":         name,
	}
}

// addPasskey adds a passkey for the user and returns its ID.
func (s *SelfPasskeyAPITestSuite) addPasskey(user selfPasskeyUser, name string) string {
	authenticator, err := testutils.NewVirtualAuthenticator(selfPasskeyRelyingPartyID, selfPasskeyOrigin)
	s.Require().NoError(err)
	challenge, sessionToken := s.start(user)

	status, body := s.do(user, http.MethodPost, "/users/me/passkeys/register/finish",
		s.finishPayload(authenticator, challenge, sessionToken, name))
	s.Require().Equal(http.StatusCreated, status, string(body))

	var saved selfPasskey
	s.Require().NoError(json.Unmarshal(body, &saved))
	s.Equal(authenticator.CredentialID(), saved.ID)
	s.Equal(name, saved.Name)
	return saved.ID
}

// list returns the user's passkeys.
func (s *SelfPasskeyAPITestSuite) list(user selfPasskeyUser) []selfPasskey {
	status, body := s.do(user, http.MethodGet, "/users/me/passkeys", nil)
	s.Require().Equal(http.StatusOK, status, string(body))

	var resp struct {
		Passkeys []selfPasskey `json:"passkeys"`
	}
	s.Require().NoError(json.Unmarshal(body, &resp))
	s.Require().NotNil(resp.Passkeys, string(body))
	return resp.Passkeys
}

func (s *SelfPasskeyAPITestSuite) TestAddListAndRemovePasskeys() {
	s.Empty(s.list(s.owner))

	laptop := s.addPasskey(s.owner, "Work laptop")
	phone := s.addPasskey(s.owner, "")
	s.ElementsMatch([]selfPasskey{{ID: laptop, Name: "Work laptop"}, {ID: phone}}, s.list(s.owner))

	status, body := s.do(s.owner, http.MethodDelete, "/users/me/passkeys/"+laptop, nil)
	s.Require().Equal(http.StatusNoContent, status, string(body))
	s.Equal([]selfPasskey{{ID: phone}}, s.list(s.owner))

	status, body = s.do(s.owner, http.MethodDelete, "/users/me/passkeys/"+laptop, nil)
	s.Equal(http.StatusNotFound, status)
	s.requireErrorCode(body, "PSK-1010")

	status, body = s.do(s.owner, http.MethodDelete, "/users/me/passkeys/"+phone, nil)
	s.Require().Equal(http.StatusNoContent, status, string(body))
	s.Empty(s.list(s.owner))
}

func (s *SelfPasskeyAPITestSuite) TestCannotRemoveAnotherUsersPasskey() {
	id := s.addPasskey(s.owner, "Owner key")
	defer s.do(s.owner, http.MethodDelete, "/users/me/passkeys/"+id, nil)

	status, body := s.do(s.other, http.MethodDelete, "/users/me/passkeys/"+id, nil)

	s.Equal(http.StatusNotFound, status)
	s.requireErrorCode(body, "PSK-1010")
	s.Contains(s.list(s.owner), selfPasskey{ID: id, Name: "Owner key"})
}

func (s *SelfPasskeyAPITestSuite) TestCannotFinishAnotherUsersRegistration() {
	authenticator, err := testutils.NewVirtualAuthenticator(selfPasskeyRelyingPartyID, selfPasskeyOrigin)
	s.Require().NoError(err)
	challenge, sessionToken := s.start(s.owner)

	status, body := s.do(s.other, http.MethodPost, "/users/me/passkeys/register/finish",
		s.finishPayload(authenticator, challenge, sessionToken, ""))

	s.Equal(http.StatusBadRequest, status)
	s.requireErrorCode(body, "PSK-1013")
	s.Empty(s.list(s.other))
}

func (s *SelfPasskeyAPITestSuite) TestStartRejectsRelyingPartyNotAllowed() {
	status, body := s.do(s.owner, http.MethodPost, "/users/me/passkeys/register/start",
		map[string]string{"relyingPartyId": "evil.example.com"})

	s.Equal(http.StatusBadRequest, status)
	s.requireErrorCode(body, "SPK-1003")
}

func (s *SelfPasskeyAPITestSuite) TestFinishRejectsLongName() {
	authenticator, err := testutils.NewVirtualAuthenticator(selfPasskeyRelyingPartyID, selfPasskeyOrigin)
	s.Require().NoError(err)
	challenge, sessionToken := s.start(s.owner)
	longName := fmt.Sprintf("%065d", 0)

	status, body := s.do(s.owner, http.MethodPost, "/users/me/passkeys/register/finish",
		s.finishPayload(authenticator, challenge, sessionToken, longName))

	s.Equal(http.StatusBadRequest, status)
	s.requireErrorCode(body, "PSK-1016")
}

func (s *SelfPasskeyAPITestSuite) TestRequiresSignedInUser() {
	resp, err := testutils.GetNoRedirectHTTPClient().Get(testutils.TestServerURL + "/users/me/passkeys")
	s.Require().NoError(err)
	defer resp.Body.Close()
	s.Equal(http.StatusUnauthorized, resp.StatusCode)
}

func (s *SelfPasskeyAPITestSuite) TestAddPasskeyFromAndroidApp() {
	status, body := s.do(s.ownerOnAndroid, http.MethodPost, "/users/me/passkeys/register/start",
		map[string]string{})
	s.Require().Equal(http.StatusOK, status, string(body))
	var start struct {
		PublicKeyCredentialCreationOptions struct {
			Challenge    string `json:"challenge"`
			RelyingParty struct {
				ID string `json:"id"`
			} `json:"rp"`
		} `json:"publicKeyCredentialCreationOptions"`
		SessionToken string `json:"sessionToken"`
	}
	s.Require().NoError(json.Unmarshal(body, &start))
	s.Equal(selfPasskeyRelyingPartyID, start.PublicKeyCredentialCreationOptions.RelyingParty.ID,
		"the app's passkey relying party ID applies when the request names none")

	authenticator, err := testutils.NewVirtualAuthenticator(selfPasskeyRelyingPartyID, selfPasskeyAndroidOrigin)
	s.Require().NoError(err)
	status, body = s.do(s.ownerOnAndroid, http.MethodPost, "/users/me/passkeys/register/finish",
		s.finishPayload(authenticator, start.PublicKeyCredentialCreationOptions.Challenge, start.SessionToken, "Pixel"))
	s.Require().Equal(http.StatusCreated, status, string(body))

	s.Contains(s.list(s.ownerOnAndroid), selfPasskey{ID: authenticator.CredentialID(), Name: "Pixel"})
	status, body = s.do(s.ownerOnAndroid, http.MethodDelete, "/users/me/passkeys/"+authenticator.CredentialID(), nil)
	s.Equal(http.StatusNoContent, status, string(body))
}

func (s *SelfPasskeyAPITestSuite) TestAndroidAppRejectsWebOrigin() {
	challenge, sessionToken := s.start(s.ownerOnAndroid)
	authenticator, err := testutils.NewVirtualAuthenticator(selfPasskeyRelyingPartyID, selfPasskeyOrigin)
	s.Require().NoError(err)

	status, body := s.do(s.ownerOnAndroid, http.MethodPost, "/users/me/passkeys/register/finish",
		s.finishPayload(authenticator, challenge, sessionToken, ""))

	s.Equal(http.StatusBadRequest, status)
	s.requireErrorCode(body, "PSK-1011")
	s.NotContains(s.list(s.ownerOnAndroid), selfPasskey{ID: authenticator.CredentialID()})
}
