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
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

// resultTokenIssuerFake is a deterministic resultTokenIssuer for the API tests.
// It produces a JWS-shaped string with the recorded claims so the API
// handler tests can decode and assert on the payload without standing up a
// real signer.
type resultTokenIssuerFake struct {
	lastRPID   string
	lastState  *RequestState
	lastValid  int64
	errToThrow error
}

func (f *resultTokenIssuerFake) issueResultToken(
	_ context.Context, rpID string, rs *RequestState, validitySeconds int64,
) (string, error) {
	if f.errToThrow != nil {
		return "", f.errToThrow
	}
	f.lastRPID = rpID
	f.lastState = rs
	f.lastValid = validitySeconds

	claims := map[string]interface{}{
		"aud":             rpID,
		"txn":             rs.State,
		"definition_id":   rs.DefinitionID,
		"subject":         rs.Result.Subject,
		"verified_claims": rs.Result.Claims,
	}
	payload, _ := json.Marshal(claims)
	header, _ := json.Marshal(map[string]string{"alg": "none", "typ": "JWT"})
	return base64.RawURLEncoding.EncodeToString(header) + "." +
			base64.RawURLEncoding.EncodeToString(payload) + ".",
		nil
}

// decodeFakeToken decodes the payload of a token produced by resultTokenIssuerFake.
func decodeFakeToken(t *testing.T, token string) map[string]interface{} {
	t.Helper()
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims map[string]interface{}
	require.NoError(t, json.Unmarshal(payload, &claims))
	return claims
}

func TestJWTresultTokenIssuerIssuesSignedToken(t *testing.T) {
	jwtSvc := jwtmock.NewJWTServiceInterfaceMock(t)
	expected := "header.payload.signature"
	jwtSvc.EXPECT().
		GenerateJWT(
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything, mock.Anything,
		).
		RunAndReturn(func(
			_ context.Context, sub, iss string, validity int64,
			claims map[string]interface{}, typ, alg string,
		) (string, int64, *serviceerror.ServiceError) {
			assert.Equal(t, "user-123", sub)
			assert.Equal(t, "https://verifier.example", iss)
			assert.EqualValues(t, 300, validity)
			assert.Equal(t, "shop.example", claims["aud"])
			assert.Equal(t, "txn-abc", claims["txn"])
			assert.Equal(t, "eudi-pid", claims["definition_id"])
			assert.Equal(t, "user-123", claims["subject"])
			vc, ok := claims["verified_claims"].(map[string]interface{})
			require.True(t, ok)
			assert.Equal(t, "Erika", vc["given_name"])
			assert.Equal(t, "x509_hash:dev", claims["verifier"])
			assert.Equal(t, "JWT", typ)
			assert.Equal(t, "", alg)
			return expected, 0, nil
		}).Once()

	issuer := newJWTresultTokenIssuer(jwtSvc, "https://verifier.example", "x509_hash:dev")
	tok, err := issuer.issueResultToken(context.Background(), "shop.example", &RequestState{
		State:        "txn-abc",
		DefinitionID: "eudi-pid",
		Status:       StatusCompleted,
		Result: &VerifiedPresentation{
			Subject: "user-123",
			Claims:  map[string]interface{}{"given_name": "Erika"},
		},
	}, 300)
	require.NoError(t, err)
	assert.Equal(t, expected, tok)
}

func TestJWTresultTokenIssuerRejectsNonCompletedStates(t *testing.T) {
	jwtSvc := jwtmock.NewJWTServiceInterfaceMock(t)
	issuer := newJWTresultTokenIssuer(jwtSvc, "iss", "cid")

	_, err := issuer.issueResultToken(context.Background(), "rp", nil, 300)
	assert.ErrorIs(t, err, ErrPolicy)

	_, err = issuer.issueResultToken(context.Background(), "rp", &RequestState{State: "x"}, 300)
	assert.ErrorIs(t, err, ErrPolicy)

	_, err = issuer.issueResultToken(context.Background(), "rp", &RequestState{
		State: "x", Status: StatusFailed,
	}, 300)
	assert.ErrorIs(t, err, ErrPolicy)
}

func TestJWTresultTokenIssuerSurfacesSigningErrors(t *testing.T) {
	jwtSvc := jwtmock.NewJWTServiceInterfaceMock(t)
	jwtSvc.EXPECT().
		GenerateJWT(
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything, mock.Anything,
		).
		Return("", 0, &serviceerror.InternalServerError).Once()
	issuer := newJWTresultTokenIssuer(jwtSvc, "iss", "cid")

	_, err := issuer.issueResultToken(context.Background(), "rp", &RequestState{
		State: "x", Status: StatusCompleted,
		Result: &VerifiedPresentation{Subject: "s"},
	}, 300)
	assert.Error(t, err)
}
