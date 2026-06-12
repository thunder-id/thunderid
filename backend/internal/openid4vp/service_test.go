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
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
)

// fakeStore is an in-memory stateStore for tests.
type fakeStore struct {
	m map[string]*RequestState
}

func newFakeStore() *fakeStore { return &fakeStore{m: map[string]*RequestState{}} }

func (f *fakeStore) Save(_ context.Context, st *RequestState) error {
	f.m[st.State] = st
	return nil
}

func (f *fakeStore) Get(_ context.Context, state string) (*RequestState, bool) {
	st, ok := f.m[state]
	return st, ok
}

func (f *fakeStore) Delete(_ context.Context, state string) error {
	delete(f.m, state)
	return nil
}

// fakeSigner signs request-object claims with an ECDSA key.
type fakeSigner struct {
	key *ecdsa.PrivateKey
}

func (s *fakeSigner) signRequestObject(_ context.Context, claims map[string]interface{}) (string, error) {
	headerJSON, err := json.Marshal(map[string]interface{}{"alg": "ES256", "typ": "oauth-authz-req+jwt"})
	if err != nil {
		return "", err
	}
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(payloadJSON)
	sig, err := cryptolib.Generate([]byte(signingInput), cryptolib.ECDSASHA256, s.key)
	if err != nil {
		return "", err
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// fabricateResponseJWE encrypts plaintext to recipientPub as an ECDH-ES/A128GCM
// JWE, mirroring what an OpenID4VP wallet posts to response_uri.
func fabricateResponseJWE(t *testing.T, recipientPub *ecdsa.PublicKey, plaintext []byte) string {
	t.Helper()
	params := cryptolib.AlgorithmParams{
		Algorithm: cryptolib.AlgorithmECDHES,
		ECDHES:    cryptolib.ECDHESParams{ContentEncryptionAlgorithm: cryptolib.Algorithm("A128GCM")},
	}
	encryptedKey, details, err := cryptolib.Encrypt(recipientPub, &params, nil)
	require.NoError(t, err)

	epk := details.EPK.(*ecdh.PublicKey)
	raw := epk.Bytes()
	require.Len(t, raw, 65)
	header := map[string]interface{}{
		"typ": "JWE", "alg": "ECDH-ES", "enc": "A128GCM",
		"epk": map[string]interface{}{
			"kty": "EC", "crv": "P-256",
			"x": base64.RawURLEncoding.EncodeToString(raw[1:33]),
			"y": base64.RawURLEncoding.EncodeToString(raw[33:]),
		},
	}
	headerJSON, err := json.Marshal(header)
	require.NoError(t, err)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	block, err := aes.NewCipher(details.CEK)
	require.NoError(t, err)
	gcm, err := cipher.NewGCM(block)
	require.NoError(t, err)
	iv := make([]byte, gcm.NonceSize())
	_, err = rand.Read(iv)
	require.NoError(t, err)
	sealed := gcm.Seal(nil, iv, plaintext, []byte(headerB64))
	ciphertext := sealed[:len(sealed)-gcm.Overhead()]
	tag := sealed[len(sealed)-gcm.Overhead():]

	return strings.Join([]string{
		headerB64,
		base64.RawURLEncoding.EncodeToString(encryptedKey),
		base64.RawURLEncoding.EncodeToString(iv),
		base64.RawURLEncoding.EncodeToString(ciphertext),
		base64.RawURLEncoding.EncodeToString(tag),
	}, ".")
}

func newTestService(t *testing.T, b *pidBuilder) (*service, *fakeStore) {
	t.Helper()
	signerKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	store := newFakeStore()
	cfg := serviceConfig{
		RequestURIBase:  "https://verifier.example/openid4vp/request",
		ResponseURIBase: "https://verifier.example/openid4vp/response",
		EphemeralKeyID:  "enc-key-1",
	}
	svc, err := newService(cfg, store, testAudience, &fakeSigner{key: signerKey})
	require.NoError(t, err)
	require.NoError(t, svc.registry.register(&presentationDefinition{
		ID:          testDefinitionID,
		DisplayName: "Test PID",
		DCQL: dcqlConfig{
			CredentialID: credentialID, VCT: testVCT,
			Claims: []string{"given_name", "family_name"},
		},
		policy: defaultPolicy(),
		Trust:  b.trustStore(),
	}))
	return svc, store
}

const testDefinitionID = "test-pid"

func TestNewServiceValidation(t *testing.T) {
	signer := &fakeSigner{}
	store := newFakeStore()
	valid := serviceConfig{
		RequestURIBase:  "https://x/req",
		ResponseURIBase: "https://x/resp",
	}

	_, err := newService(valid, nil, "x509_hash:x", signer)
	assert.ErrorIs(t, err, ErrPolicy)

	_, err = newService(valid, store, "", signer)
	assert.ErrorIs(t, err, ErrPolicy)

	_, err = newService(valid, store, "x509_hash:x", nil)
	assert.ErrorIs(t, err, ErrPolicy)

	_, err = newService(serviceConfig{}, store, "x509_hash:x", signer)
	assert.ErrorIs(t, err, ErrPolicy)

	svc, err := newService(valid, store, "x509_hash:x", signer)
	require.NoError(t, err)
	assert.Equal(t, defaultStateTTL, svc.cfg.TTL)
	assert.Equal(t, "x509_hash:x", svc.clientID)
}

func TestInitiateStoresPendingState(t *testing.T) {
	b := newPIDBuilder(t)
	svc, store := newTestService(t, b)

	init, err := svc.Initiate(context.Background(), testDefinitionID)
	require.NoError(t, err)

	assert.NotEmpty(t, init.State)
	assert.Equal(t, testAudience, init.ClientID)
	assert.Contains(t, init.RequestURI, "state=")

	rs := store.m[init.State]
	require.NotNil(t, rs)
	assert.Equal(t, StatusPending, rs.Status)
	assert.NotEmpty(t, rs.Nonce)
	assert.NotNil(t, rs.EphemeralKey)
}

func TestRequestObjectBuildsSignedJAR(t *testing.T) {
	b := newPIDBuilder(t)
	svc, store := newTestService(t, b)

	init, err := svc.Initiate(context.Background(), testDefinitionID)
	require.NoError(t, err)

	jar, err := svc.RequestObject(context.Background(), init.State)
	require.NoError(t, err)

	parts := strings.Split(jar, ".")
	require.Len(t, parts, 3)
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims map[string]interface{}
	require.NoError(t, json.Unmarshal(payloadJSON, &claims))

	assert.Equal(t, ResponseModeDirectPostJWT, claims["response_mode"])
	assert.Equal(t, testAudience, claims["client_id"])
	assert.Equal(t, init.State, claims["state"])
	assert.Equal(t, store.m[init.State].Nonce, claims["nonce"])
	assert.Contains(t, claims["response_uri"], "state=")
	assert.Contains(t, claims, "dcql_query")
	assert.Contains(t, claims, "client_metadata")
}

func TestRequestObjectUnknownState(t *testing.T) {
	b := newPIDBuilder(t)
	svc, _ := newTestService(t, b)
	_, err := svc.RequestObject(context.Background(), "nope")
	assert.ErrorIs(t, err, ErrUnknownState)
}

func TestSubmitResponseHappyPath(t *testing.T) {
	b := newPIDBuilder(t)
	svc, store := newTestService(t, b)

	init, err := svc.Initiate(context.Background(), testDefinitionID)
	require.NoError(t, err)
	rs := store.m[init.State]

	presentation := b.build(rs.Nonce, map[string]interface{}{
		"given_name": "Erika", "family_name": "Mustermann", "birthdate": "1984-01-26",
	})
	body, err := json.Marshal(map[string]interface{}{
		"state":    init.State,
		"vp_token": map[string]interface{}{credentialID: []string{presentation}},
	})
	require.NoError(t, err)
	jweToken := fabricateResponseJWE(t, &rs.EphemeralKey.PublicKey, body)

	pid, err := svc.SubmitResponse(context.Background(), init.State, []byte(jweToken))
	require.NoError(t, err)
	assert.Equal(t, "Erika", pid.Claims["given_name"])

	stored := store.m[init.State]
	assert.Equal(t, StatusCompleted, stored.Status)
	assert.NotNil(t, stored.Result)

	// Result polling reflects completion.
	res, err := svc.Result(context.Background(), init.State)
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, res.Status)
}

func TestSubmitResponseWrongNonceMarksFailed(t *testing.T) {
	b := newPIDBuilder(t)
	svc, store := newTestService(t, b)

	init, err := svc.Initiate(context.Background(), testDefinitionID)
	require.NoError(t, err)
	rs := store.m[init.State]

	// Presentation bound to a different nonce than the one issued.
	presentation := b.build("attacker-nonce", map[string]interface{}{"given_name": "Erika", "family_name": "M"})
	body, err := json.Marshal(map[string]interface{}{
		"state":    init.State,
		"vp_token": map[string]interface{}{credentialID: []string{presentation}},
	})
	require.NoError(t, err)
	jweToken := fabricateResponseJWE(t, &rs.EphemeralKey.PublicKey, body)

	_, err = svc.SubmitResponse(context.Background(), init.State, []byte(jweToken))
	assert.ErrorIs(t, err, ErrInvalidPresentation)
	assert.Equal(t, StatusFailed, store.m[init.State].Status)
	assert.NotEmpty(t, store.m[init.State].FailureReason)
}

func TestSubmitResponseStateMismatch(t *testing.T) {
	b := newPIDBuilder(t)
	svc, store := newTestService(t, b)

	init, err := svc.Initiate(context.Background(), testDefinitionID)
	require.NoError(t, err)
	rs := store.m[init.State]

	presentation := b.build(rs.Nonce, map[string]interface{}{"given_name": "Erika", "family_name": "M"})
	body, err := json.Marshal(map[string]interface{}{
		"state":    "a-different-state",
		"vp_token": map[string]interface{}{credentialID: []string{presentation}},
	})
	require.NoError(t, err)
	jweToken := fabricateResponseJWE(t, &rs.EphemeralKey.PublicKey, body)

	_, err = svc.SubmitResponse(context.Background(), init.State, []byte(jweToken))
	assert.ErrorIs(t, err, ErrStateMismatch)
}

func TestSubmitResponseUndecryptable(t *testing.T) {
	b := newPIDBuilder(t)
	svc, _ := newTestService(t, b)

	init, err := svc.Initiate(context.Background(), testDefinitionID)
	require.NoError(t, err)

	// JWE encrypted to an unrelated key cannot be decrypted by the stored key.
	other, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	jweToken := fabricateResponseJWE(t, &other.PublicKey, []byte(`{"state":"x"}`))

	_, err = svc.SubmitResponse(context.Background(), init.State, []byte(jweToken))
	assert.ErrorIs(t, err, ErrInvalidResponse)
}

func TestSubmitResponseUnknownState(t *testing.T) {
	b := newPIDBuilder(t)
	svc, _ := newTestService(t, b)
	_, err := svc.SubmitResponse(context.Background(), "nope", []byte("x.y.z.a.b"))
	assert.ErrorIs(t, err, ErrUnknownState)
}

func TestExpiredStateRejected(t *testing.T) {
	b := newPIDBuilder(t)
	svc, store := newTestService(t, b)

	init, err := svc.Initiate(context.Background(), testDefinitionID)
	require.NoError(t, err)

	// Force expiry.
	store.m[init.State].ExpiresAt = time.Now().Add(-time.Minute)

	_, err = svc.Result(context.Background(), init.State)
	assert.ErrorIs(t, err, ErrUnknownState)
	// Expired entry is evicted.
	_, ok := store.m[init.State]
	assert.False(t, ok)
}

func TestWalletAuthorizationURI(t *testing.T) {
	uri := WalletAuthorizationURI("x509_hash:abc", "https://v/req?state=s")
	assert.Contains(t, uri, "openid4vp://?")
	assert.Contains(t, uri, "client_id=x509_hash%3Aabc")
	assert.Contains(t, uri, "request_uri=https")
}

func TestResultRedirectURIBase(t *testing.T) {
	b := newPIDBuilder(t)
	svc, _ := newTestService(t, b)
	assert.Empty(t, svc.resultRedirectURIBase())

	svc.cfg.ResultRedirectURIBase = resultRedirectURIBase
	assert.Equal(t, resultRedirectURIBase, svc.resultRedirectURIBase())
}

func TestResultRedirectURI(t *testing.T) {
	b := newPIDBuilder(t)
	svc, _ := newTestService(t, b)

	// Unconfigured base => empty URL.
	assert.Equal(t, "", svc.ResultRedirectURI("some-state"))

	svc.cfg.ResultRedirectURIBase = resultRedirectURIBase
	got := svc.ResultRedirectURI("xyz state")
	assert.Equal(t, "https://verifier.example/result?state=xyz+state", got)

	svc.cfg.ResultRedirectURIBase = "https://verifier.example/result?ui=qr"
	got = svc.ResultRedirectURI("abc")
	assert.Equal(t, "https://verifier.example/result?ui=qr&state=abc", got)
}

func TestInitiateForRPRecordsRPID(t *testing.T) {
	b := newPIDBuilder(t)
	svc, store := newTestService(t, b)

	// With an empty RPID the field is not set.
	init, err := svc.InitiateForRP(context.Background(), testDefinitionID, "")
	require.NoError(t, err)
	rs := store.m[init.State]
	require.NotNil(t, rs)
	assert.Empty(t, rs.RPID)

	// With a non-empty RPID the field is persisted.
	init2, err := svc.InitiateForRP(context.Background(), testDefinitionID, "scholarbooks")
	require.NoError(t, err)
	rs2 := store.m[init2.State]
	require.NotNil(t, rs2)
	assert.Equal(t, "scholarbooks", rs2.RPID)
}

func TestInitiateRejectsUnknownDefinition(t *testing.T) {
	b := newPIDBuilder(t)
	svc, _ := newTestService(t, b)

	_, err := svc.Initiate(context.Background(), "no-such-def")
	assert.ErrorIs(t, err, ErrPolicy)

	_, err = svc.InitiateForRP(context.Background(), "no-such-def", "rp")
	assert.ErrorIs(t, err, ErrPolicy)
}

func TestFirstNonEmptyString(t *testing.T) {
	assert.Equal(t, "", firstNonEmptyString())
	assert.Equal(t, "", firstNonEmptyString("", ""))
	assert.Equal(t, "a", firstNonEmptyString("a"))
	assert.Equal(t, "first", firstNonEmptyString("first", "second"))
	assert.Equal(t, "second", firstNonEmptyString("", "second"))
	assert.Equal(t, "third", firstNonEmptyString("", "", "third"))
}

func TestFirstNonEmptyStringSlice(t *testing.T) {
	assert.Nil(t, firstNonEmptyStringSlice())
	assert.Nil(t, firstNonEmptyStringSlice(nil, []string{}))
	assert.Equal(t, []string{"a"}, firstNonEmptyStringSlice([]string{"a"}))
	assert.Equal(t, []string{"a"}, firstNonEmptyStringSlice([]string{"a"}, []string{"b"}))
	assert.Equal(t, []string{"b"}, firstNonEmptyStringSlice(nil, []string{"b"}))
}

func TestFirstNonZeroDuration(t *testing.T) {
	assert.Equal(t, time.Duration(0), firstNonZeroDuration())
	assert.Equal(t, time.Duration(0), firstNonZeroDuration(0, 0))
	assert.Equal(t, time.Second, firstNonZeroDuration(time.Second))
	assert.Equal(t, time.Second, firstNonZeroDuration(time.Second, time.Minute))
	assert.Equal(t, time.Minute, firstNonZeroDuration(0, time.Minute))
}

func TestWithState(t *testing.T) {
	assert.Equal(t, "https://x/req?state=abc", withState("https://x/req", "abc"))
	assert.Equal(t, "https://x/req?foo=bar&state=abc", withState("https://x/req?foo=bar", "abc"))
	assert.Contains(t, withState("https://x/req", "a b"), "state=a+b")
}

func TestRandomTokenIsRandomAndBase64URL(t *testing.T) {
	a, err := randomToken()
	require.NoError(t, err)
	bb, err := randomToken()
	require.NoError(t, err)

	assert.NotEqual(t, a, bb)
	// 32 raw bytes base64url-encoded == 43 chars (no padding).
	assert.Len(t, a, 43)

	decoded, err := base64.RawURLEncoding.DecodeString(a)
	require.NoError(t, err)
	assert.Len(t, decoded, 32)
}

func TestRegistryRegisterRejectsInvalid(t *testing.T) {
	r := newRegistry()

	assert.ErrorIs(t, r.register(nil), ErrPolicy)
	assert.ErrorIs(t, r.register(&presentationDefinition{}), ErrPolicy)

	def := &presentationDefinition{ID: "x"}
	require.NoError(t, r.register(def))
	// Duplicate id is rejected.
	assert.ErrorIs(t, r.register(&presentationDefinition{ID: "x"}), ErrPolicy)
}

func TestRegistryListSortsIDs(t *testing.T) {
	r := newRegistry()
	require.NoError(t, r.register(&presentationDefinition{ID: "charlie"}))
	require.NoError(t, r.register(&presentationDefinition{ID: "alpha"}))
	require.NoError(t, r.register(&presentationDefinition{ID: "bravo"}))

	assert.Equal(t, []string{"alpha", "bravo", "charlie"}, r.list())
}

func TestRegistryListEmpty(t *testing.T) {
	r := newRegistry()
	assert.Empty(t, r.list())
}

func TestAuthenticate_UnknownState(t *testing.T) {
	svc, _ := newTestService(t, newPIDBuilder(t))
	result, err := svc.Authenticate(context.Background(), "no-such-state")
	assert.Nil(t, result)
	assert.NotNil(t, err)
	assert.Equal(t, ErrorUnknownState.Code, err.Code)
}

func TestAuthenticate_PendingState(t *testing.T) {
	svc, store := newTestService(t, newPIDBuilder(t))
	store.m["s1"] = &RequestState{
		State:     "s1",
		Status:    StatusPending,
		ExpiresAt: time.Now().Add(time.Minute),
	}
	result, err := svc.Authenticate(context.Background(), "s1")
	assert.Nil(t, result)
	assert.NotNil(t, err)
	assert.Equal(t, ErrorVerificationFailed.Code, err.Code)
}

func TestAuthenticate_FailedState(t *testing.T) {
	svc, store := newTestService(t, newPIDBuilder(t))
	store.m["s2"] = &RequestState{
		State:         "s2",
		Status:        StatusFailed,
		FailureReason: "nonce mismatch",
		ExpiresAt:     time.Now().Add(time.Minute),
	}
	result, err := svc.Authenticate(context.Background(), "s2")
	assert.Nil(t, result)
	assert.NotNil(t, err)
	assert.Equal(t, ErrorVerificationFailed.Code, err.Code)
}

func TestAuthenticate_CompletedNilPresentation(t *testing.T) {
	svc, store := newTestService(t, newPIDBuilder(t))
	store.m["s3"] = &RequestState{
		State:     "s3",
		Status:    StatusCompleted,
		Result:    nil,
		ExpiresAt: time.Now().Add(time.Minute),
	}
	result, err := svc.Authenticate(context.Background(), "s3")
	assert.Nil(t, result)
	assert.NotNil(t, err)
}

func TestAuthenticate_CompletedValidPresentation(t *testing.T) {
	svc, store := newTestService(t, newPIDBuilder(t))
	store.m["s4"] = &RequestState{
		State:  "s4",
		Status: StatusCompleted,
		Result: &VerifiedPresentation{
			Subject: "sub-42",
			Issuer:  "https://issuer.example",
			VCT:     testVCT,
			Claims:  map[string]interface{}{"given_name": "Ada", "family_name": "Lovelace"},
		},
		ExpiresAt: time.Now().Add(time.Minute),
	}
	result, err := svc.Authenticate(context.Background(), "s4")
	require.Nil(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "sub-42", result.Token["sub"])
	assert.Equal(t, "https://issuer.example", result.Token["openid4vp_issuer"])
	assert.Equal(t, testVCT, result.Token["openid4vp_vct"])
	assert.Equal(t, "Ada", result.Token["given_name"])
	assert.Equal(t, "Lovelace", result.Token["family_name"])
	// AuthenticatedClaims is the same map as Token
	assert.Equal(t, result.Token, result.AuthenticatedClaims)
}

func TestAuthenticate_CompletedNoSubject(t *testing.T) {
	svc, store := newTestService(t, newPIDBuilder(t))
	store.m["s5"] = &RequestState{
		State:  "s5",
		Status: StatusCompleted,
		Result: &VerifiedPresentation{
			Issuer: "https://issuer.example",
			VCT:    testVCT,
			Claims: map[string]interface{}{"tax_id": "DE123456"},
		},
		ExpiresAt: time.Now().Add(time.Minute),
	}
	result, err := svc.Authenticate(context.Background(), "s5")
	require.Nil(t, err)
	require.NotNil(t, result)
	_, hasSub := result.Token["sub"]
	assert.False(t, hasSub, "sub should not be present when VerifiedPresentation.Subject is empty")
	assert.Equal(t, "DE123456", result.Token["tax_id"])
}
