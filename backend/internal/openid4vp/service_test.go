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

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/openid4vp/definition"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

type OpenID4VPServiceTestSuite struct {
	suite.Suite
}

func TestOpenID4VPServiceTestSuite(t *testing.T) {
	suite.Run(t, new(OpenID4VPServiceTestSuite))
}

// newStatefulStore returns an openID4VPStoreInterface mock backed by entries, so
// round-trip tests can read back the service-generated request state.
func newStatefulStore(t *testing.T, entries map[string]*RequestState) *openID4VPStoreInterfaceMock {
	t.Helper()
	m := newOpenID4VPStoreInterfaceMock(t)
	m.EXPECT().SaveRequestState(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, st *RequestState) error {
			entries[st.State] = st
			return nil
		}).Maybe()
	m.EXPECT().GetRequestState(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, state string) (*RequestState, bool) {
			st, ok := entries[state]
			return st, ok
		}).Maybe()
	m.EXPECT().DeleteRequestState(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, state string) error {
			delete(entries, state)
			return nil
		}).Maybe()
	return m
}

// newStatefulDefinitionReader returns a definitionReader mock backed by byHandle,
// returning ErrorDefinitionNotFound for unseeded handles.
func newStatefulDefinitionReader(
	t *testing.T, byHandle map[string]definition.PresentationDefinitionDTO,
) *definitionReaderMock {
	t.Helper()
	m := newDefinitionReaderMock(t)
	m.EXPECT().GetPresentationDefinitionByHandle(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, handle string) (*definition.PresentationDefinitionDTO, *tidcommon.ServiceError) {
			dto, ok := byHandle[handle]
			if !ok {
				return nil, &definition.ErrorDefinitionNotFound
			}
			return &dto, nil
		}).Maybe()
	return m
}

// newSigningMock returns a requestSigner mock that signs request-object claims
// with key (ES256), mirroring the production signer's JWS output.
func newSigningMock(t *testing.T, key *ecdsa.PrivateKey) *requestSignerMock {
	t.Helper()
	m := newRequestSignerMock(t)
	m.EXPECT().signRequestObject(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, claims map[string]interface{}) (string, error) {
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
			sig, err := cryptolib.Generate([]byte(signingInput), cryptolib.ECDSASHA256, key)
			if err != nil {
				return "", err
			}
			return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
		}).Maybe()
	return m
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

func newTestService(t *testing.T, b *pidBuilder) (*service, map[string]*RequestState) {
	t.Helper()
	svc, store, _ := newTestServiceWithDefs(t, b)
	return svc, store
}

// newTestServiceWithDefs is newTestService plus the live definition map, so tests
// can mutate the seeded definition before initiating (the reader mock reads the
// map at call time).
func newTestServiceWithDefs(
	t *testing.T, b *pidBuilder,
) (*service, map[string]*RequestState, map[string]definition.PresentationDefinitionDTO) {
	t.Helper()
	signerKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	stateEntries := map[string]*RequestState{}
	store := newStatefulStore(t, stateEntries)
	cfg := serviceConfig{
		RequestURIBase:    "https://verifier.example/openid4vp/request",
		ResponseURIBase:   "https://verifier.example/openid4vp/response",
		EphemeralKeyID:    "enc-key-1",
		EnforceKeyBinding: true,
	}
	byHandle := map[string]definition.PresentationDefinitionDTO{
		testDefinitionID: {
			ID:              testDefinitionID,
			Handle:          testDefinitionID,
			DisplayName:     "Test PID",
			VCT:             testVCT,
			Format:          definition.DefaultCredentialFormat,
			RequestedClaims: []string{"given_name", "family_name", "birthdate"},
			MandatoryClaims: []string{"given_name", "family_name"},
		},
	}
	defStore := newStatefulDefinitionReader(t, byHandle)
	svc, err := newOpenID4VPService(cfg, store, testAudience, newSigningMock(t, signerKey), b.trustStore(), defStore)
	require.NoError(t, err)
	return svc.(*service), stateEntries, byHandle
}

// testDefinitionID is the definition handle, which in the management model also
// serves as the DCQL credential id the wallet keys its vp_token by.
const testDefinitionID = credentialID

func (suite *OpenID4VPServiceTestSuite) TestNewServiceValidation() {
	signer := newRequestSignerMock(suite.T())
	store := newOpenID4VPStoreInterfaceMock(suite.T())
	defStore := newDefinitionReaderMock(suite.T())
	valid := serviceConfig{
		RequestURIBase:  "https://x/req",
		ResponseURIBase: "https://x/resp",
	}

	_, err := newOpenID4VPService(valid, nil, "x509_hash:x", signer, nil, defStore)
	suite.ErrorIs(err, ErrPolicy)

	_, err = newOpenID4VPService(valid, store, "", signer, nil, defStore)
	suite.ErrorIs(err, ErrPolicy)

	_, err = newOpenID4VPService(valid, store, "x509_hash:x", nil, nil, defStore)
	suite.ErrorIs(err, ErrPolicy)

	_, err = newOpenID4VPService(valid, store, "x509_hash:x", signer, nil, nil)
	suite.ErrorIs(err, ErrPolicy)

	_, err = newOpenID4VPService(serviceConfig{}, store, "x509_hash:x", signer, nil, defStore)
	suite.ErrorIs(err, ErrPolicy)

	svcIface, err := newOpenID4VPService(valid, store, "x509_hash:x", signer, nil, defStore)
	suite.Require().NoError(err)
	svc := svcIface.(*service)
	suite.Equal(defaultStateTTL, svc.cfg.TTL)
	suite.Equal("x509_hash:x", svc.clientID)
}

func (suite *OpenID4VPServiceTestSuite) TestInitiateStoresPendingState() {
	b := newPIDBuilder(suite.T())
	svc, store := newTestService(suite.T(), b)

	init, svcErr := svc.Initiate(context.Background(), testDefinitionID)
	suite.Require().Nil(svcErr)

	suite.NotEmpty(init.State)
	suite.Equal(testAudience, init.ClientID)
	suite.Contains(init.RequestURI, "state=")

	rs := store[init.State]
	suite.Require().NotNil(rs)
	suite.Equal(StatusPending, rs.Status)
	suite.NotEmpty(rs.Nonce)
	suite.NotNil(rs.EphemeralKey)
}

func (suite *OpenID4VPServiceTestSuite) TestRequestObjectBuildsSignedJAR() {
	b := newPIDBuilder(suite.T())
	svc, store := newTestService(suite.T(), b)

	init, svcErr := svc.Initiate(context.Background(), testDefinitionID)
	suite.Require().Nil(svcErr)

	jar, svcErr := svc.GetRequestObject(context.Background(), init.State)
	suite.Require().Nil(svcErr)

	parts := strings.Split(jar, ".")
	suite.Require().Len(parts, 3)
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	suite.Require().NoError(err)
	var claims map[string]interface{}
	suite.Require().NoError(json.Unmarshal(payloadJSON, &claims))

	suite.Equal(ResponseModeDirectPostJWT, claims["response_mode"])
	suite.Equal(testAudience, claims["client_id"])
	suite.Equal(init.State, claims["state"])
	suite.Equal(store[init.State].Nonce, claims["nonce"])
	suite.Contains(claims["response_uri"], "state=")
	suite.Contains(claims, "dcql_query")
	suite.Contains(claims, "client_metadata")
}

func (suite *OpenID4VPServiceTestSuite) TestRequestObjectUnknownState() {
	b := newPIDBuilder(suite.T())
	svc, _ := newTestService(suite.T(), b)
	_, svcErr := svc.GetRequestObject(context.Background(), "nope")
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorUnknownState.Code, svcErr.Code)
}

func (suite *OpenID4VPServiceTestSuite) TestSubmitResponseHappyPath() {
	b := newPIDBuilder(suite.T())
	svc, store := newTestService(suite.T(), b)

	init, svcErr := svc.Initiate(context.Background(), testDefinitionID)
	suite.Require().Nil(svcErr)
	rs := store[init.State]

	presentation := b.build(rs.Nonce, map[string]interface{}{
		"given_name": "Erika", "family_name": "Mustermann", "birthdate": "1984-01-26",
	})
	body, err := json.Marshal(map[string]interface{}{
		"state":    init.State,
		"vp_token": map[string]interface{}{credentialID: []string{presentation}},
	})
	suite.Require().NoError(err)
	jweToken := fabricateResponseJWE(suite.T(), &rs.EphemeralKey.PublicKey, body)

	pid, svcErr := svc.SubmitResponse(context.Background(), init.State, []byte(jweToken))
	suite.Require().Nil(svcErr)
	suite.Equal("Erika", pid.Claims["given_name"])

	stored := store[init.State]
	suite.Equal(StatusCompleted, stored.Status)
	suite.NotNil(stored.Result)

	// Result polling reflects completion.
	res, svcErr := svc.GetResult(context.Background(), init.State)
	suite.Require().Nil(svcErr)
	suite.Equal(StatusCompleted, res.Status)
}

// A holder may return several matching credentials for one query; verification
// must accept the first that satisfies the policy rather than failing as ambiguous.
func (suite *OpenID4VPServiceTestSuite) TestSubmitResponseAcceptsFirstValidOfMany() {
	b := newPIDBuilder(suite.T())
	svc, store := newTestService(suite.T(), b)

	init, svcErr := svc.Initiate(context.Background(), testDefinitionID)
	suite.Require().Nil(svcErr)
	rs := store[init.State]

	valid := b.build(rs.Nonce, map[string]interface{}{
		"given_name": "Erika", "family_name": "Mustermann", "birthdate": "1984-01-26",
	})
	body, err := json.Marshal(map[string]interface{}{
		"state":    init.State,
		"vp_token": map[string]interface{}{credentialID: []string{"not-a-valid-sd-jwt", valid}},
	})
	suite.Require().NoError(err)
	jweToken := fabricateResponseJWE(suite.T(), &rs.EphemeralKey.PublicKey, body)

	pid, svcErr := svc.SubmitResponse(context.Background(), init.State, []byte(jweToken))
	suite.Require().Nil(svcErr)
	suite.Equal("Erika", pid.Claims["given_name"])
	suite.Equal(StatusCompleted, store[init.State].Status)
}

func (suite *OpenID4VPServiceTestSuite) TestSubmitResponseRestrictsTrustedAuthority() {
	b := newPIDBuilder(suite.T())
	svc, store, byHandle := newTestServiceWithDefs(suite.T(), b)

	// The definition enables trust enforcement and pins an anchor the credential's leaf does NOT chain to.
	enforceIssuer := true
	def := byHandle[testDefinitionID]
	def.EnforceTrustedIssuer = &enforceIssuer
	def.TrustedAuthorities = []string{"unrelated-root"}
	byHandle[testDefinitionID] = def

	init, svcErr := svc.Initiate(context.Background(), testDefinitionID)
	suite.Require().Nil(svcErr)
	rs := store[init.State]

	presentation := b.build(rs.Nonce, map[string]interface{}{"given_name": "Erika", "family_name": "M"})
	body, err := json.Marshal(map[string]interface{}{
		"state":    init.State,
		"vp_token": map[string]interface{}{credentialID: []string{presentation}},
	})
	suite.Require().NoError(err)
	jweToken := fabricateResponseJWE(suite.T(), &rs.EphemeralKey.PublicKey, body)

	_, svcErr = svc.SubmitResponse(context.Background(), init.State, []byte(jweToken))
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorVerificationFailed.Code, svcErr.Code)
	suite.Equal(StatusFailed, store[init.State].Status)
}

func (suite *OpenID4VPServiceTestSuite) TestSubmitResponseAcceptsCorrectTrustedAuthority() {
	b := newPIDBuilder(suite.T())
	svc, store, byHandle := newTestServiceWithDefs(suite.T(), b)

	// b.trustStore() names the credential's root "test-root"; pin to it with enforcement enabled.
	enforceIssuer := true
	def := byHandle[testDefinitionID]
	def.EnforceTrustedIssuer = &enforceIssuer
	def.TrustedAuthorities = []string{"test-root"}
	byHandle[testDefinitionID] = def

	init, svcErr := svc.Initiate(context.Background(), testDefinitionID)
	suite.Require().Nil(svcErr)
	rs := store[init.State]

	presentation := b.build(rs.Nonce, map[string]interface{}{"given_name": "Erika", "family_name": "M"})
	body, err := json.Marshal(map[string]interface{}{
		"state":    init.State,
		"vp_token": map[string]interface{}{credentialID: []string{presentation}},
	})
	suite.Require().NoError(err)
	jweToken := fabricateResponseJWE(suite.T(), &rs.EphemeralKey.PublicKey, body)

	pid, svcErr := svc.SubmitResponse(context.Background(), init.State, []byte(jweToken))
	suite.Require().Nil(svcErr)
	suite.Equal("Erika", pid.Claims["given_name"])
}

func (suite *OpenID4VPServiceTestSuite) TestRequestObjectEmitsTrustedAuthorityAKI() {
	b := newPIDBuilder(suite.T())
	svc, _, byHandle := newTestServiceWithDefs(suite.T(), b)

	def := byHandle[testDefinitionID]
	def.TrustedAuthorities = []string{"test-root"}
	byHandle[testDefinitionID] = def

	init, svcErr := svc.Initiate(context.Background(), testDefinitionID)
	suite.Require().Nil(svcErr)

	jar, svcErr := svc.GetRequestObject(context.Background(), init.State)
	suite.Require().Nil(svcErr)

	parts := strings.Split(jar, ".")
	suite.Require().Len(parts, 3)
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	suite.Require().NoError(err)
	var claims map[string]interface{}
	suite.Require().NoError(json.Unmarshal(payloadJSON, &claims))

	dcql := claims["dcql_query"].(map[string]interface{})
	cred := dcql["credentials"].([]interface{})[0].(map[string]interface{})
	authorities := cred["trusted_authorities"].([]interface{})
	suite.Require().Len(authorities, 1)
	entry := authorities[0].(map[string]interface{})
	suite.Equal("aki", entry["type"])

	ski := base64.RawURLEncoding.EncodeToString(b.rootCert.SubjectKeyId)
	suite.Equal([]interface{}{ski}, entry["values"])
}

func (suite *OpenID4VPServiceTestSuite) TestSubmitResponseWrongNonceMarksFailed() {
	b := newPIDBuilder(suite.T())
	svc, store := newTestService(suite.T(), b)

	init, svcErr := svc.Initiate(context.Background(), testDefinitionID)
	suite.Require().Nil(svcErr)
	rs := store[init.State]

	// Presentation bound to a different nonce than the one issued.
	presentation := b.build("attacker-nonce", map[string]interface{}{"given_name": "Erika", "family_name": "M"})
	body, err := json.Marshal(map[string]interface{}{
		"state":    init.State,
		"vp_token": map[string]interface{}{credentialID: []string{presentation}},
	})
	suite.Require().NoError(err)
	jweToken := fabricateResponseJWE(suite.T(), &rs.EphemeralKey.PublicKey, body)

	_, svcErr = svc.SubmitResponse(context.Background(), init.State, []byte(jweToken))
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorVerificationFailed.Code, svcErr.Code)
	suite.Equal(StatusFailed, store[init.State].Status)
	suite.NotEmpty(store[init.State].FailureReason)
}

func (suite *OpenID4VPServiceTestSuite) TestSubmitResponseStateMismatch() {
	b := newPIDBuilder(suite.T())
	svc, store := newTestService(suite.T(), b)

	init, svcErr := svc.Initiate(context.Background(), testDefinitionID)
	suite.Require().Nil(svcErr)
	rs := store[init.State]

	presentation := b.build(rs.Nonce, map[string]interface{}{"given_name": "Erika", "family_name": "M"})
	body, err := json.Marshal(map[string]interface{}{
		"state":    "a-different-state",
		"vp_token": map[string]interface{}{credentialID: []string{presentation}},
	})
	suite.Require().NoError(err)
	jweToken := fabricateResponseJWE(suite.T(), &rs.EphemeralKey.PublicKey, body)

	_, svcErr = svc.SubmitResponse(context.Background(), init.State, []byte(jweToken))
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorVerificationFailed.Code, svcErr.Code)
}

func (suite *OpenID4VPServiceTestSuite) TestSubmitResponseUndecryptable() {
	b := newPIDBuilder(suite.T())
	svc, _ := newTestService(suite.T(), b)

	init, svcErr := svc.Initiate(context.Background(), testDefinitionID)
	suite.Require().Nil(svcErr)

	// JWE encrypted to an unrelated key cannot be decrypted by the stored key.
	other, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	suite.Require().NoError(err)
	jweToken := fabricateResponseJWE(suite.T(), &other.PublicKey, []byte(`{"state":"x"}`))

	_, svcErr = svc.SubmitResponse(context.Background(), init.State, []byte(jweToken))
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorVerificationFailed.Code, svcErr.Code)
}

func (suite *OpenID4VPServiceTestSuite) TestSubmitResponseUnknownState() {
	b := newPIDBuilder(suite.T())
	svc, _ := newTestService(suite.T(), b)
	_, err := svc.SubmitResponse(context.Background(), "nope", []byte("x.y.z.a.b"))
	suite.Require().NotNil(err)
	suite.Equal(ErrorUnknownState.Code, err.Code)
}

func (suite *OpenID4VPServiceTestSuite) TestSubmitErrorMarksFailed() {
	b := newPIDBuilder(suite.T())
	svc, store := newTestService(suite.T(), b)

	init, svcErr := svc.Initiate(context.Background(), testDefinitionID)
	suite.Require().Nil(svcErr)

	svcErr = svc.SubmitError(context.Background(), init.State,
		"access_denied", "The End-User did not give consent")
	suite.Require().Nil(svcErr)

	rs := store[init.State]
	suite.Equal(StatusFailed, rs.Status)
	suite.Contains(rs.FailureReason, "access_denied")
	suite.Contains(rs.FailureReason, "did not give consent")
}

func (suite *OpenID4VPServiceTestSuite) TestSubmitErrorUnknownState() {
	b := newPIDBuilder(suite.T())
	svc, _ := newTestService(suite.T(), b)
	err := svc.SubmitError(context.Background(), "nope", "access_denied", "")
	suite.Require().NotNil(err)
	suite.Equal(ErrorUnknownState.Code, err.Code)
}

func (suite *OpenID4VPServiceTestSuite) TestExpiredStateRejected() {
	b := newPIDBuilder(suite.T())
	svc, store := newTestService(suite.T(), b)

	init, svcErr := svc.Initiate(context.Background(), testDefinitionID)
	suite.Require().Nil(svcErr)

	// Force expiry.
	store[init.State].ExpiresAt = time.Now().Add(-time.Minute)

	_, svcErr = svc.GetResult(context.Background(), init.State)
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorUnknownState.Code, svcErr.Code)
	// Expired entry is evicted.
	_, ok := store[init.State]
	suite.False(ok)
}

func (suite *OpenID4VPServiceTestSuite) TestWalletAuthorizationURI() {
	uri := WalletAuthorizationURI("x509_hash:abc", "https://v/req?state=s")
	suite.Contains(uri, "openid4vp://?")
	suite.Contains(uri, "client_id=x509_hash%3Aabc")
	suite.Contains(uri, "request_uri=https")
}

func (suite *OpenID4VPServiceTestSuite) TestResultRedirectURIBase() {
	b := newPIDBuilder(suite.T())
	svc, _ := newTestService(suite.T(), b)
	suite.Empty(svc.cfg.ResultRedirectURIBase)

	svc.cfg.ResultRedirectURIBase = resultRedirectURIBase
	suite.Equal(resultRedirectURIBase, svc.cfg.ResultRedirectURIBase)
}

func (suite *OpenID4VPServiceTestSuite) TestResultRedirectURI() {
	b := newPIDBuilder(suite.T())
	svc, _ := newTestService(suite.T(), b)

	// Unconfigured base => empty URL.
	suite.Equal("", svc.GetResultRedirectURI("some-state"))

	svc.cfg.ResultRedirectURIBase = resultRedirectURIBase
	got := svc.GetResultRedirectURI("xyz state")
	suite.Equal("https://verifier.example/result?state=xyz+state", got)

	svc.cfg.ResultRedirectURIBase = "https://verifier.example/result?ui=qr"
	got = svc.GetResultRedirectURI("abc")
	suite.Equal("https://verifier.example/result?ui=qr&state=abc", got)
}

func (suite *OpenID4VPServiceTestSuite) TestInitiateForRPRecordsRPID() {
	b := newPIDBuilder(suite.T())
	svc, store := newTestService(suite.T(), b)

	// With an empty RPID the field is not set.
	init, svcErr := svc.InitiateForRP(context.Background(), testDefinitionID, "")
	suite.Require().Nil(svcErr)
	rs := store[init.State]
	suite.Require().NotNil(rs)
	suite.Empty(rs.RPID)

	// With a non-empty RPID the field is persisted.
	init2, svcErr := svc.InitiateForRP(context.Background(), testDefinitionID, "scholarbooks")
	suite.Require().Nil(svcErr)
	rs2 := store[init2.State]
	suite.Require().NotNil(rs2)
	suite.Equal("scholarbooks", rs2.RPID)
}

func (suite *OpenID4VPServiceTestSuite) TestInitiateRejectsUnknownDefinition() {
	b := newPIDBuilder(suite.T())
	svc, _ := newTestService(suite.T(), b)

	_, err := svc.Initiate(context.Background(), "no-such-def")
	suite.Require().NotNil(err)
	suite.Equal(ErrorUnknownDefinition.Code, err.Code)

	_, err = svc.InitiateForRP(context.Background(), "no-such-def", "rp")
	suite.Require().NotNil(err)
	suite.Equal(ErrorUnknownDefinition.Code, err.Code)
}

func (suite *OpenID4VPServiceTestSuite) TestWithState() {
	suite.Equal("https://x/req?state=abc", withState("https://x/req", "abc"))
	suite.Equal("https://x/req?foo=bar&state=abc", withState("https://x/req?foo=bar", "abc"))
	suite.Contains(withState("https://x/req", "a b"), "state=a+b")
}

func (suite *OpenID4VPServiceTestSuite) TestRandomTokenIsRandomAndBase64URL() {
	a, err := randomToken()
	suite.Require().NoError(err)
	bb, err := randomToken()
	suite.Require().NoError(err)

	suite.NotEqual(a, bb)
	// 32 raw bytes base64url-encoded == 43 chars (no padding).
	suite.Len(a, 43)

	decoded, err := base64.RawURLEncoding.DecodeString(a)
	suite.Require().NoError(err)
	suite.Len(decoded, 32)
}

func (suite *OpenID4VPServiceTestSuite) TestAuthenticate_UnknownState() {
	svc, _ := newTestService(suite.T(), newPIDBuilder(suite.T()))
	result, err := svc.Authenticate(context.Background(), "no-such-state")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorUnknownState.Code, err.Code)
}

func (suite *OpenID4VPServiceTestSuite) TestAuthenticate_PendingState() {
	svc, store := newTestService(suite.T(), newPIDBuilder(suite.T()))
	store["s1"] = &RequestState{
		State:     "s1",
		Status:    StatusPending,
		ExpiresAt: time.Now().Add(time.Minute),
	}
	result, err := svc.Authenticate(context.Background(), "s1")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorVerificationFailed.Code, err.Code)
}

func (suite *OpenID4VPServiceTestSuite) TestAuthenticate_FailedState() {
	svc, store := newTestService(suite.T(), newPIDBuilder(suite.T()))
	store["s2"] = &RequestState{
		State:         "s2",
		Status:        StatusFailed,
		FailureReason: "nonce mismatch",
		ExpiresAt:     time.Now().Add(time.Minute),
	}
	result, err := svc.Authenticate(context.Background(), "s2")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorVerificationFailed.Code, err.Code)
}

func (suite *OpenID4VPServiceTestSuite) TestAuthenticate_CompletedNilPresentation() {
	svc, store := newTestService(suite.T(), newPIDBuilder(suite.T()))
	store["s3"] = &RequestState{
		State:     "s3",
		Status:    StatusCompleted,
		Result:    nil,
		ExpiresAt: time.Now().Add(time.Minute),
	}
	result, err := svc.Authenticate(context.Background(), "s3")
	suite.Nil(result)
	suite.NotNil(err)
}

func (suite *OpenID4VPServiceTestSuite) TestAuthenticate_CompletedValidPresentation() {
	svc, store := newTestService(suite.T(), newPIDBuilder(suite.T()))
	store["s4"] = &RequestState{
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
	suite.Require().Nil(err)
	suite.Require().NotNil(result)
	// Identification token keys on the subject only (like OIDC), so IdentifyEntity
	// matches a returning holder by sub instead of ANDing over non-identifier claims.
	suite.Equal("sub-42", result.Token["sub"])
	suite.Len(result.Token, 1)
	// AuthenticatedClaims carries the full disclosed set plus openid4vp metadata.
	suite.Equal("sub-42", result.AuthenticatedClaims["sub"])
	suite.Equal("https://issuer.example", result.AuthenticatedClaims["openid4vp_issuer"])
	suite.Equal(testVCT, result.AuthenticatedClaims["openid4vp_vct"])
	suite.Equal("Ada", result.AuthenticatedClaims["given_name"])
	suite.Equal("Lovelace", result.AuthenticatedClaims["family_name"])
}

func (suite *OpenID4VPServiceTestSuite) TestAuthenticate_CompletedNoSubject() {
	svc, store := newTestService(suite.T(), newPIDBuilder(suite.T()))
	store["s5"] = &RequestState{
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
	suite.Require().Nil(err)
	suite.Require().NotNil(result)
	// With no subject, the identification token carries no sub (and stays empty),
	// so IdentifyEntity finds nothing and the holder is provisioned just-in-time.
	_, hasSub := result.Token["sub"]
	suite.False(hasSub, "sub should not be present when VerifiedPresentation.Subject is empty")
	suite.Empty(result.Token)
	// Disclosed claims remain available for provisioning via AuthenticatedClaims.
	suite.Equal("DE123456", result.AuthenticatedClaims["tax_id"])
}

func testRequestConfig() requestConfig {
	return requestConfig{
		ClientID:    "x509_hash:abc123",
		ResponseURI: "https://verifier.example/openid4vp/response",
		DCQL: dcqlConfig{
			CredentialID: "pid-sd-jwt",
			VCT:          "urn:eudi:pid:de:1",
			Claims:       []string{"given_name", "family_name", "birthdate"},
		},
	}
}

func testRequestParams(t *testing.T) requestParams {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return requestParams{
		Nonce:          "nonce-abc",
		State:          "state-xyz",
		EphemeralKey:   &key.PublicKey,
		EphemeralKeyID: "enc-key-1",
		IssuedAt:       time.Unix(1_700_000_000, 0),
	}
}

func (suite *OpenID4VPServiceTestSuite) TestBuildRequestObjectHappyPath() {
	params := testRequestParams(suite.T())
	req, err := buildRequestObject(testRequestConfig(), params)
	suite.Require().NoError(err)

	suite.Equal(ResponseTypeVPToken, req["response_type"])
	suite.Equal(ResponseModeDirectPostJWT, req["response_mode"])
	suite.Equal("x509_hash:abc123", req["client_id"])
	suite.Equal("https://verifier.example/openid4vp/response", req["response_uri"])
	suite.Equal("nonce-abc", req["nonce"])
	suite.Equal("state-xyz", req["state"])
	suite.NotContains(req, "aud") // aud is omitted when no audience is configured
	suite.Equal(int64(1_700_000_000), req["iat"])
	suite.Equal(int64(1_700_000_000)+int64(defaultRequestValidity.Seconds()), req["exp"])
	suite.NotContains(req, "verifier_info")

	_, ok := req["dcql_query"].(*dcqlQuery)
	suite.True(ok)
}

func (suite *OpenID4VPServiceTestSuite) TestBuildRequestObjectClientMetadata() {
	params := testRequestParams(suite.T())
	req, err := buildRequestObject(testRequestConfig(), params)
	suite.Require().NoError(err)

	meta := req["client_metadata"].(map[string]interface{})
	suite.Equal([]string{DefaultResponseEncValue}, meta["encrypted_response_enc_values_supported"])

	formats := meta["vp_formats_supported"].(map[string]interface{})
	suite.Contains(formats, FormatSDJWTVC)

	jwks := meta["jwks"].(map[string]interface{})
	keys := jwks["keys"].([]interface{})
	suite.Require().Len(keys, 1)
	jwk := keys[0].(map[string]interface{})
	suite.Equal("EC", jwk["kty"])
	suite.Equal("P-256", jwk["crv"])
	suite.Equal("enc", jwk["use"])
	suite.Equal("ECDH-ES", jwk["alg"])
	suite.Equal("enc-key-1", jwk["kid"])
	suite.NotEmpty(jwk["x"])
	suite.NotEmpty(jwk["y"])
}

func (suite *OpenID4VPServiceTestSuite) TestBuildRequestObjectEphemeralKeyMatchesInput() {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	suite.Require().NoError(err)
	params := requestParams{
		Nonce: "n", State: "s", EphemeralKey: &key.PublicKey, EphemeralKeyID: "k",
	}
	req, err := buildRequestObject(testRequestConfig(), params)
	suite.Require().NoError(err)

	clientMetadata := req["client_metadata"].(map[string]interface{})
	jwks := clientMetadata["jwks"].(map[string]interface{})
	jwk := jwks["keys"].([]interface{})[0].(map[string]interface{})

	// The advertised JWK coordinates must match the EC public key the wallet
	// will encrypt to.
	raw, err := key.PublicKey.Bytes()
	suite.Require().NoError(err)
	suite.Require().Len(raw, 65)
	suite.Equal(base64.RawURLEncoding.EncodeToString(raw[1:33]), jwk["x"])
	suite.Equal(base64.RawURLEncoding.EncodeToString(raw[33:]), jwk["y"])
}

func (suite *OpenID4VPServiceTestSuite) TestBuildRequestObjectVerifierInfoAndOverrides() {
	cfg := testRequestConfig()
	cfg.Audience = "https://wallet.example"
	cfg.Validity = 10 * time.Minute
	cfg.ResponseEncValues = []string{"A128GCM", "A256GCM"}
	cfg.VerifierInfo = []interface{}{map[string]interface{}{"registration": "cert"}}

	params := testRequestParams(suite.T())
	req, err := buildRequestObject(cfg, params)
	suite.Require().NoError(err)

	suite.Equal("https://wallet.example", req["aud"])
	suite.Equal(int64(1_700_000_000)+int64((10*time.Minute).Seconds()), req["exp"])
	suite.Contains(req, "verifier_info")

	meta := req["client_metadata"].(map[string]interface{})
	suite.Equal([]string{"A128GCM", "A256GCM"}, meta["encrypted_response_enc_values_supported"])
}

func (suite *OpenID4VPServiceTestSuite) TestBuildRequestObjectValidation() {
	valid := testRequestParams(suite.T())
	tests := map[string]struct {
		cfg    requestConfig
		params requestParams
	}{
		"missing client_id":    {requestConfig{ResponseURI: "https://x"}, valid},
		"missing response_uri": {requestConfig{ClientID: "x509_hash:x"}, valid},
		"missing nonce":        {testRequestConfig(), requestParams{State: "s", EphemeralKey: valid.EphemeralKey}},
		"missing ephemeral":    {testRequestConfig(), requestParams{Nonce: "n", State: "s"}},
	}
	for name, tc := range tests {
		suite.Run(name, func() {
			_, err := buildRequestObject(tc.cfg, tc.params)
			suite.ErrorIs(err, ErrPolicy)
		})
	}
}

func (suite *OpenID4VPServiceTestSuite) TestEcdsaPublicKeyToEncJWKCurves() {
	cases := []struct {
		name     string
		curve    elliptic.Curve
		expected string
	}{
		{"P-256", elliptic.P256(), "P-256"},
		{"P-384", elliptic.P384(), "P-384"},
		{"P-521", elliptic.P521(), "P-521"},
	}
	for _, c := range cases {
		suite.Run(c.name, func() {
			key, err := ecdsa.GenerateKey(c.curve, rand.Reader)
			suite.Require().NoError(err)
			jwk, err := ecdsaPublicKeyToEncJWK(&key.PublicKey, "kid-1")
			suite.Require().NoError(err)
			suite.Equal("EC", jwk["kty"])
			suite.Equal(c.expected, jwk["crv"])
			suite.Equal("kid-1", jwk["kid"])
			suite.NotEmpty(jwk["x"])
			suite.NotEmpty(jwk["y"])
		})
	}
}

func (suite *OpenID4VPServiceTestSuite) TestEcdsaPublicKeyToEncJWKOmitsEmptyKid() {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	suite.Require().NoError(err)
	jwk, err := ecdsaPublicKeyToEncJWK(&key.PublicKey, "")
	suite.Require().NoError(err)
	_, ok := jwk["kid"]
	suite.False(ok)
}

func (suite *OpenID4VPServiceTestSuite) TestEcdsaPublicKeyToEncJWKUnsupportedCurve() {
	key, err := ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	suite.Require().NoError(err)
	_, err = ecdsaPublicKeyToEncJWK(&key.PublicKey, "kid")
	suite.ErrorIs(err, ErrPolicy)
}

func (suite *OpenID4VPServiceTestSuite) TestBuildRequestObjectSerialises() {
	params := testRequestParams(suite.T())
	req, err := buildRequestObject(testRequestConfig(), params)
	suite.Require().NoError(err)

	raw, err := json.Marshal(req)
	suite.Require().NoError(err)
	suite.Contains(string(raw), `"dcql_query"`)
	suite.Contains(string(raw), `"response_mode":"direct_post.jwt"`)
}

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

func (suite *OpenID4VPServiceTestSuite) TestParseAuthorizationResponse_ArrayAndStringForms() {
	arrayForm := []byte(`{"state":"st","vp_token":{"pid-sd-jwt":["pres-a"]}}`)
	resp, err := parseAuthorizationResponse(arrayForm)
	suite.Require().NoError(err)
	suite.Equal("st", resp.State)
	suite.Equal([]string{"pres-a"}, resp.Presentations[credentialID])

	stringForm := []byte(`{"vp_token":{"pid-sd-jwt":"pres-b"}}`)
	resp, err = parseAuthorizationResponse(stringForm)
	suite.Require().NoError(err)
	suite.Equal([]string{"pres-b"}, resp.Presentations[credentialID])
}

func (suite *OpenID4VPServiceTestSuite) TestParseAuthorizationResponse_Errors() {
	cases := map[string][]byte{
		"not json":         []byte(`{not-json`),
		"missing vp_token": []byte(`{"state":"st"}`),
		"bad presentation": []byte(`{"vp_token":{"pid-sd-jwt":123}}`),
		"vp_token number":  []byte(`{"vp_token":123}`),
	}
	for name, body := range cases {
		suite.Run(name, func() {
			_, err := parseAuthorizationResponse(body)
			suite.ErrorIs(err, ErrInvalidResponse)
		})
	}
}

// A Presentation Exchange response carries a single, un-keyed presentation
// string; it is parsed and resolvable for the requested credential id.
func (suite *OpenID4VPServiceTestSuite) TestParseAuthorizationResponse_RejectsNonDCQL() {
	// A non-keyed vp_token (a bare presentation string, as the removed DIF
	// Presentation Exchange format used) is rejected: OpenID4VP 1.0 requires a
	// DCQL object keyed by credential id.
	_, err := parseAuthorizationResponse([]byte(`{"state":"st","vp_token":"sd-jwt~disc~kb~"}`))
	suite.Require().Error(err)
	suite.ErrorIs(err, ErrInvalidResponse)
}

func (suite *OpenID4VPServiceTestSuite) TestPresentationLookup() {
	resp := &authorizationResponse{Presentations: map[string][]string{
		credentialID: {"only-one"},
		"ambiguous":  {"a", "b"},
		"empty":      {},
	}}

	got, err := resp.presentationsFor(credentialID)
	suite.Require().NoError(err)
	suite.Equal([]string{"only-one"}, got)

	// Multiple matching credentials are returned as-is; the verifier tries each.
	multi, err := resp.presentationsFor("ambiguous")
	suite.Require().NoError(err)
	suite.Equal([]string{"a", "b"}, multi)

	_, err = resp.presentationsFor("missing")
	suite.ErrorIs(err, ErrInvalidResponse)

	_, err = resp.presentationsFor("empty")
	suite.ErrorIs(err, ErrInvalidResponse)
}

func (suite *OpenID4VPServiceTestSuite) TestVerifyResponse_HappyPath() {
	b := newPIDBuilder(suite.T())
	v := newTestVerifier(suite.T(), b, defaultPolicy())
	presentation := b.build(testNonce, map[string]interface{}{
		"given_name":  "Erika",
		"family_name": "Mustermann",
		"birthdate":   "1984-01-26",
	})
	body := responseBody(suite.T(), presentation, "state-123")

	pid, err := v.verifyResponse(body, credentialID, testNonce)
	suite.Require().NoError(err)
	suite.Equal("Erika", pid.Claims["given_name"])
	suite.Equal(testIssuer, pid.Issuer)
}

func (suite *OpenID4VPServiceTestSuite) TestVerifyResponse_PropagatesVerificationFailure() {
	b := newPIDBuilder(suite.T())
	v := newTestVerifier(suite.T(), b, defaultPolicy())
	presentation := b.build("issued-nonce", map[string]interface{}{"given_name": "Erika", "family_name": "M"})
	body := responseBody(suite.T(), presentation, "state-123")

	_, err := v.verifyResponse(body, credentialID, "expected-nonce")
	suite.ErrorIs(err, ErrInvalidPresentation)
}

func (suite *OpenID4VPServiceTestSuite) TestVerifyResponse_MissingCredential() {
	b := newPIDBuilder(suite.T())
	v := newTestVerifier(suite.T(), b, defaultPolicy())
	presentation := b.build(testNonce, map[string]interface{}{"given_name": "Erika", "family_name": "M"})
	body := responseBody(suite.T(), presentation, "state-123")

	_, err := v.verifyResponse(body, "other-credential", testNonce)
	suite.ErrorIs(err, ErrInvalidResponse)
}

// policyWithValues returns the default policy with claim value constraints added.
func policyWithValues(cv map[string][]string) policy {
	p := defaultPolicy()
	p.ClaimValues = cv
	return p
}

// A disclosed claim whose value is in the allowed set passes; one outside it is
// rejected; an undisclosed constrained (optional) claim is not enforced.
func (suite *OpenID4VPServiceTestSuite) TestVerifyResponse_ClaimValueConstraint() {
	b := newPIDBuilder(suite.T())
	build := func() string {
		return b.build(testNonce, map[string]interface{}{
			"given_name": "Erika", "family_name": "Mustermann", "birthdate": "1984-01-26",
		})
	}

	suite.Run("allowed value passes", func() {
		v := newTestVerifier(suite.T(), b, policyWithValues(map[string][]string{"given_name": {"Erika", "Max"}}))
		_, err := v.verifyResponse(responseBody(suite.T(), build(), "s"), credentialID, testNonce)
		suite.Require().NoError(err)
	})

	suite.Run("disallowed value rejected", func() {
		v := newTestVerifier(suite.T(), b, policyWithValues(map[string][]string{"given_name": {"Max"}}))
		_, err := v.verifyResponse(responseBody(suite.T(), build(), "s"), credentialID, testNonce)
		suite.ErrorIs(err, ErrClaimValueNotAllowed)
	})

	suite.Run("undisclosed constrained claim ignored", func() {
		v := newTestVerifier(suite.T(), b, policyWithValues(map[string][]string{"nationality": {"DE"}}))
		_, err := v.verifyResponse(responseBody(suite.T(), build(), "s"), credentialID, testNonce)
		suite.Require().NoError(err)
	})
}

// stubResultTokenIssuer returns a resultTokenIssuer mock that emits an unsigned
// JWT carrying the result claims, so API tests can decode and assert on the
// payload without standing up a real signer.
func stubResultTokenIssuer(t *testing.T) *resultTokenIssuerMock {
	t.Helper()
	m := newResultTokenIssuerMock(t)
	m.EXPECT().issueResultToken(mock.Anything, mock.Anything, mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, rpID string, rs *RequestState, _ int64) (string, error) {
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
				base64.RawURLEncoding.EncodeToString(payload) + ".", nil
		}).Maybe()
	return m
}

// failingResultTokenIssuer returns a resultTokenIssuer mock whose issueResultToken always fails.
func failingResultTokenIssuer(t *testing.T, err error) *resultTokenIssuerMock {
	t.Helper()
	m := newResultTokenIssuerMock(t)
	m.EXPECT().issueResultToken(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("", err).Maybe()
	return m
}

// decodeFakeToken decodes the payload of a token produced by stubResultTokenIssuer.
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

func (suite *OpenID4VPServiceTestSuite) TestJWTresultTokenIssuerIssuesSignedToken() {
	t := suite.T()
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
		) (string, int64, *tidcommon.ServiceError) {
			suite.Equal("user-123", sub)
			suite.Equal("https://verifier.example", iss)
			suite.EqualValues(300, validity)
			suite.Equal("shop.example", claims["aud"])
			suite.Equal("txn-abc", claims["txn"])
			suite.Equal("eudi-pid", claims["definition_id"])
			suite.Equal("user-123", claims["subject"])
			vc, ok := claims["verified_claims"].(map[string]interface{})
			suite.Require().True(ok)
			suite.Equal("Erika", vc["given_name"])
			suite.Equal("x509_hash:dev", claims["verifier"])
			suite.Equal("JWT", typ)
			suite.Equal("", alg)
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
	suite.Require().NoError(err)
	suite.Equal(expected, tok)
}

func (suite *OpenID4VPServiceTestSuite) TestJWTresultTokenIssuerRejectsNonCompletedStates() {
	jwtSvc := jwtmock.NewJWTServiceInterfaceMock(suite.T())
	issuer := newJWTresultTokenIssuer(jwtSvc, "iss", "cid")

	_, err := issuer.issueResultToken(context.Background(), "rp", nil, 300)
	suite.ErrorIs(err, ErrPolicy)

	_, err = issuer.issueResultToken(context.Background(), "rp", &RequestState{State: "x"}, 300)
	suite.ErrorIs(err, ErrPolicy)

	_, err = issuer.issueResultToken(context.Background(), "rp", &RequestState{
		State: "x", Status: StatusFailed,
	}, 300)
	suite.ErrorIs(err, ErrPolicy)
}

func (suite *OpenID4VPServiceTestSuite) TestJWTresultTokenIssuerSurfacesSigningErrors() {
	jwtSvc := jwtmock.NewJWTServiceInterfaceMock(suite.T())
	jwtSvc.EXPECT().
		GenerateJWT(
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything, mock.Anything,
		).
		Return("", 0, &tidcommon.InternalServerError).Once()
	issuer := newJWTresultTokenIssuer(jwtSvc, "iss", "cid")

	_, err := issuer.issueResultToken(context.Background(), "rp", &RequestState{
		State: "x", Status: StatusCompleted,
		Result: &VerifiedPresentation{Subject: "s"},
	}, 300)
	suite.Error(err)
}
