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

package channel

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTokenVerifierAcceptsMatchingBearer(t *testing.T) {
	v := newTokenVerifier("s3cret")
	r := httptest.NewRequest(http.MethodGet, "/cp/connect", nil)
	r.Header.Set("Authorization", "Bearer s3cret")
	id, err := v.Verify(r)
	assert.NoError(t, err)
	// A shared token proves no identity, so the caller falls back to the claimed header.
	assert.Empty(t, id)
}

func TestTokenVerifierRejectsWrongOrMissingBearer(t *testing.T) {
	v := newTokenVerifier("s3cret")

	r := httptest.NewRequest(http.MethodGet, "/cp/connect", nil)
	r.Header.Set("Authorization", "Bearer nope")
	_, err := v.Verify(r)
	assert.ErrorIs(t, err, errUnauthorized)

	r2 := httptest.NewRequest(http.MethodGet, "/cp/connect", nil)
	_, err2 := v.Verify(r2)
	assert.ErrorIs(t, err2, errUnauthorized)
}

func TestTokenVerifierRejectsWhenNotConfigured(t *testing.T) {
	v := newTokenVerifier("")
	r := httptest.NewRequest(http.MethodGet, "/cp/connect", nil)
	r.Header.Set("Authorization", "Bearer anything")
	_, err := v.Verify(r)
	assert.ErrorIs(t, err, errAuthNotConfigured)
}

// fakeTokenStore holds issued tokens in memory.
type fakeTokenStore map[string]string

func (f fakeTokenStore) TokenFor(_ context.Context, dataPlaneID string) (string, bool, error) {
	token, ok := f[dataPlaneID]
	return token, ok, nil
}

// An issued per-Data-Plane token is what makes the handshake know which Data Plane connected, instead
// of believing the id in the header.
func TestPerDataPlaneTokenAuthenticatesTheClaimedIdentity(t *testing.T) {
	v := newStoredTokenVerifier(fakeTokenStore{"org1:dev": "dev-token", "org1:stage": "stage-token"})

	r := httptest.NewRequest(http.MethodGet, "/cp/connect", nil)
	r.Header.Set(HeaderDataPlaneID, "org1:dev")
	r.Header.Set("Authorization", "Bearer dev-token")

	id, err := v.Verify(r)

	assert.NoError(t, err)
	assert.Equal(t, "org1:dev", id)
}

// With a token per Data Plane, holding one is not enough to be another: this is the impersonation a
// single shared token allows, where any holder can evict a connection and receive its commands.
func TestPerDataPlaneTokenRefusesAnotherDataPlanesToken(t *testing.T) {
	v := newStoredTokenVerifier(fakeTokenStore{"org1:dev": "dev-token", "org1:stage": "stage-token"})

	r := httptest.NewRequest(http.MethodGet, "/cp/connect", nil)
	r.Header.Set(HeaderDataPlaneID, "org1:stage")
	r.Header.Set("Authorization", "Bearer dev-token")

	_, err := v.Verify(r)

	assert.ErrorIs(t, err, errUnauthorized)
}

func TestPerDataPlaneTokenRefusesAnUnknownDataPlane(t *testing.T) {
	v := newStoredTokenVerifier(fakeTokenStore{"org1:dev": "dev-token"})

	r := httptest.NewRequest(http.MethodGet, "/cp/connect", nil)
	r.Header.Set(HeaderDataPlaneID, "org9:dev")
	r.Header.Set("Authorization", "Bearer dev-token")

	_, err := v.Verify(r)

	assert.ErrorIs(t, err, errUnauthorized)
}

func TestPerDataPlaneTokenRequiresAClaimedIdentity(t *testing.T) {
	v := newStoredTokenVerifier(fakeTokenStore{"org1:dev": "dev-token"})

	r := httptest.NewRequest(http.MethodGet, "/cp/connect", nil)
	r.Header.Set("Authorization", "Bearer dev-token")

	_, err := v.Verify(r)

	assert.ErrorIs(t, err, errMissingDataPlaneID)
}

// Per-Data-Plane tokens win when both are configured: the shared token is the weaker of the two, and
// leaving it in place would keep the impersonation the other form exists to close.
func TestPerDataPlaneTokensWinOverTheSharedToken(t *testing.T) {
	v := serverVerifier(ServerConfig{AuthToken: "shared"}, fakeTokenStore{"org1:dev": "dev-token"})

	shared := httptest.NewRequest(http.MethodGet, "/cp/connect", nil)
	shared.Header.Set(HeaderDataPlaneID, "org1:dev")
	shared.Header.Set("Authorization", "Bearer shared")
	_, err := v.Verify(shared)
	assert.ErrorIs(t, err, errUnauthorized)

	own := httptest.NewRequest(http.MethodGet, "/cp/connect", nil)
	own.Header.Set(HeaderDataPlaneID, "org1:dev")
	own.Header.Set("Authorization", "Bearer dev-token")
	authenticated, err := v.Verify(own)
	assert.NoError(t, err)
	assert.Equal(t, "org1:dev", authenticated)
}
