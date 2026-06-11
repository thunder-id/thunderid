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

package openid4vp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// credentialID is the DCQL credential query id used across the package's tests
// (not a credential or secret — gosec false positive).
const credentialID = "pid-sd-jwt" //nolint:gosec // DCQL query id, not a credential

// responseBody wraps a presentation in a DCQL OpenID4VP authorization response.
func responseBody(t *testing.T, presentation, state string) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]interface{}{
		"state":    state,
		"vp_token": map[string]interface{}{credentialID: []string{presentation}},
	})
	require.NoError(t, err)
	return body
}

func TestParseAuthorizationResponse_ArrayAndStringForms(t *testing.T) {
	arrayForm := []byte(`{"state":"st","vp_token":{"pid-sd-jwt":["pres-a"]}}`)
	resp, err := parseAuthorizationResponse(arrayForm)
	require.NoError(t, err)
	assert.Equal(t, "st", resp.State)
	assert.Equal(t, []string{"pres-a"}, resp.Presentations[credentialID])

	stringForm := []byte(`{"vp_token":{"pid-sd-jwt":"pres-b"}}`)
	resp, err = parseAuthorizationResponse(stringForm)
	require.NoError(t, err)
	assert.Equal(t, []string{"pres-b"}, resp.Presentations[credentialID])
}

func TestParseAuthorizationResponse_Errors(t *testing.T) {
	cases := map[string][]byte{
		"not json":         []byte(`{not-json`),
		"missing vp_token": []byte(`{"state":"st"}`),
		"vp_token scalar":  []byte(`{"vp_token":"oops"}`),
		"bad presentation": []byte(`{"vp_token":{"pid-sd-jwt":123}}`),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseAuthorizationResponse(body)
			assert.ErrorIs(t, err, ErrInvalidResponse)
		})
	}
}

func TestPresentationLookup(t *testing.T) {
	resp := &authorizationResponse{Presentations: map[string][]string{
		credentialID: {"only-one"},
		"ambiguous":  {"a", "b"},
		"empty":      {},
	}}

	got, err := resp.presentation(credentialID)
	require.NoError(t, err)
	assert.Equal(t, "only-one", got)

	_, err = resp.presentation("missing")
	assert.ErrorIs(t, err, ErrInvalidResponse)

	_, err = resp.presentation("ambiguous")
	assert.ErrorIs(t, err, ErrInvalidResponse)

	_, err = resp.presentation("empty")
	assert.ErrorIs(t, err, ErrInvalidResponse)
}

func TestVerifyResponse_HappyPath(t *testing.T) {
	b := newPIDBuilder(t)
	v := newTestVerifier(t, b, defaultPolicy())
	presentation := b.build(testNonce, map[string]interface{}{
		"given_name":  "Erika",
		"family_name": "Mustermann",
		"birthdate":   "1984-01-26",
	})
	body := responseBody(t, presentation, "state-123")

	pid, err := v.verifyResponse(context.Background(), body, credentialID, testNonce)
	require.NoError(t, err)
	assert.Equal(t, "Erika", pid.Claims["given_name"])
	assert.Equal(t, testIssuer, pid.Issuer)
}

func TestVerifyResponse_PropagatesVerificationFailure(t *testing.T) {
	b := newPIDBuilder(t)
	v := newTestVerifier(t, b, defaultPolicy())
	presentation := b.build("issued-nonce", map[string]interface{}{"given_name": "Erika", "family_name": "M"})
	body := responseBody(t, presentation, "state-123")

	_, err := v.verifyResponse(context.Background(), body, credentialID, "expected-nonce")
	assert.ErrorIs(t, err, ErrInvalidPresentation)
}

func TestVerifyResponse_MissingCredential(t *testing.T) {
	b := newPIDBuilder(t)
	v := newTestVerifier(t, b, defaultPolicy())
	presentation := b.build(testNonce, map[string]interface{}{"given_name": "Erika", "family_name": "M"})
	body := responseBody(t, presentation, "state-123")

	_, err := v.verifyResponse(context.Background(), body, "other-credential", testNonce)
	assert.ErrorIs(t, err, ErrInvalidResponse)
}
