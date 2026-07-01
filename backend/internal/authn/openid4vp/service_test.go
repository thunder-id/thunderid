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
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	authncommon "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/jose/sdjwt"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/vc/presentation"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/cryptomock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/vc/presentationmock"
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

// newStatefulDefinitionReader returns a PresentationDefinitionServiceInterface mock backed by
// byHandle, returning ErrorDefinitionNotFound for unseeded handles.
func newStatefulDefinitionReader(
	t *testing.T, byHandle map[string]presentation.PresentationDefinitionDTO,
) *presentationmock.PresentationDefinitionServiceInterfaceMock {
	t.Helper()
	m := presentationmock.NewPresentationDefinitionServiceInterfaceMock(t)
	m.EXPECT().GetPresentationDefinitionByHandle(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, handle string) (*presentation.PresentationDefinitionDTO, *tidcommon.ServiceError) {
			dto, ok := byHandle[handle]
			if !ok {
				return nil, &presentation.ErrorDefinitionNotFound
			}
			return &dto, nil
		}).Maybe()
	return m
}

// newTestCryptoProvider returns a mock RuntimeCryptoProvider that signs with key (ES256).
func newTestCryptoProvider(t *testing.T, key *ecdsa.PrivateKey) kmprovider.RuntimeCryptoProvider {
	t.Helper()
	if key == nil {
		var err error
		key, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
	}
	provider := cryptomock.NewRuntimeCryptoProviderMock(t)
	provider.EXPECT().Sign(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, _ kmprovider.KeyRef, _ string, data []byte) ([]byte, error) {
			h := sha256.Sum256(data)
			return ecdsa.SignASN1(rand.Reader, key, h[:])
		}).Maybe()
	return provider
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

func newTestService(t *testing.T, b *pidBuilder) (*openid4vpService, map[string]*RequestState) {
	t.Helper()
	svc, store, _ := newTestServiceWithDefs(t, b)
	return svc, store
}

// newTestServiceWithDefs is newTestService plus the live definition map, so tests
// can mutate the seeded definition before initiating (the reader mock reads the
// map at call time).
func newTestServiceWithDefs(
	t *testing.T, b *pidBuilder,
) (*openid4vpService, map[string]*RequestState, map[string]presentation.PresentationDefinitionDTO) {
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
		TTL:               5 * time.Minute,
	}
	byHandle := map[string]presentation.PresentationDefinitionDTO{
		testDefinitionID: {
			ID:              testDefinitionID,
			Handle:          testDefinitionID,
			Name:            "Test PID",
			VCT:             testVCT,
			Format:          presentation.DefaultCredentialFormat,
			RequestedClaims: []string{"given_name", "family_name", "birthdate"},
			MandatoryClaims: []string{"given_name", "family_name"},
		},
	}
	defStore := newStatefulDefinitionReader(t, byHandle)
	provider := newTestCryptoProvider(t, signerKey)
	svc, err := newOpenID4VPService(cfg, store, testAudience,
		provider, kmprovider.KeyRef{KeyID: "test-key"}, "ES256", nil,
		b.trustStore(), defStore, nil, "")
	require.NoError(t, err)
	return svc, stateEntries, byHandle
}

// testDefinitionID is the definition handle, which in the management model also
// serves as the DCQL credential id the wallet keys its vp_token by.
const testDefinitionID = credentialID

func (suite *OpenID4VPServiceTestSuite) TestNewServiceValidation() {
	provider := newTestCryptoProvider(suite.T(), nil)
	store := newOpenID4VPStoreInterfaceMock(suite.T())
	defStore := presentationmock.NewPresentationDefinitionServiceInterfaceMock(suite.T())
	valid := serviceConfig{
		RequestURIBase:  "https://x/req",
		ResponseURIBase: "https://x/resp",
		TTL:             5 * time.Minute,
	}
	keyRef := kmprovider.KeyRef{KeyID: "test-key"}

	_, err := newOpenID4VPService(valid, nil, "x509_hash:x", provider, keyRef, "ES256", nil, nil, defStore, nil, "")
	suite.ErrorIs(err, ErrPolicy)

	_, err = newOpenID4VPService(valid, store, "", provider, keyRef, "ES256", nil, nil, defStore, nil, "")
	suite.ErrorIs(err, ErrPolicy)

	_, err = newOpenID4VPService(valid, store, "x509_hash:x", nil, keyRef, "ES256", nil, nil, defStore, nil, "")
	suite.ErrorIs(err, ErrPolicy)

	_, err = newOpenID4VPService(valid, store, "x509_hash:x", provider, keyRef, "ES256", nil, nil, nil, nil, "")
	suite.ErrorIs(err, ErrPolicy)

	_, err = newOpenID4VPService(serviceConfig{}, store, "x509_hash:x", provider, keyRef, "ES256",
		nil, nil, defStore, nil, "")
	suite.ErrorIs(err, ErrPolicy)

	svc, err := newOpenID4VPService(valid, store, "x509_hash:x", provider, keyRef, "ES256", nil, nil, defStore, nil, "")
	suite.Require().NoError(err)
	suite.Equal(5*time.Minute, svc.cfg.TTL)
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
	svc.jwtSvc = stubResultTokenIssuer(suite.T())
	svc.issuerURL = testIssuerURL

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

	pid, _, svcErr := svc.SubmitResponse(context.Background(), init.State, []byte(jweToken))
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

	pid, _, svcErr := svc.SubmitResponse(context.Background(), init.State, []byte(jweToken))
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

	_, _, svcErr = svc.SubmitResponse(context.Background(), init.State, []byte(jweToken))
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

	pid, _, svcErr := svc.SubmitResponse(context.Background(), init.State, []byte(jweToken))
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

	_, _, svcErr = svc.SubmitResponse(context.Background(), init.State, []byte(jweToken))
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

	_, _, svcErr = svc.SubmitResponse(context.Background(), init.State, []byte(jweToken))
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

	_, _, svcErr = svc.SubmitResponse(context.Background(), init.State, []byte(jweToken))
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorVerificationFailed.Code, svcErr.Code)
}

func (suite *OpenID4VPServiceTestSuite) TestSubmitResponseUnknownState() {
	b := newPIDBuilder(suite.T())
	svc, _ := newTestService(suite.T(), b)
	_, _, err := svc.SubmitResponse(context.Background(), "nope", []byte("x.y.z.a.b"))
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

	// GetResult returns StatusExpired without error and deletes the entry.
	rs, svcErr := svc.GetResult(context.Background(), init.State)
	suite.Require().Nil(svcErr)
	suite.Equal(StatusExpired, rs.Status)
	_, ok := store[init.State]
	suite.False(ok, "expired entry must be deleted by GetResult")
}

func (suite *OpenID4VPServiceTestSuite) TestWalletAuthorizationURI() {
	uri := WalletAuthorizationURI("x509_hash:abc", "https://v/req?state=s")
	suite.Contains(uri, "openid4vp://?")
	suite.Contains(uri, "client_id=x509_hash%3Aabc")
	suite.Contains(uri, "request_uri=https")
}

func (suite *OpenID4VPServiceTestSuite) TestSubmitResponseRedirectURI() {
	submitVP := func(svc *openid4vpService, store map[string]*RequestState, b *pidBuilder) (
		string, *tidcommon.ServiceError,
	) {
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
		_, redirect, svcErr := svc.SubmitResponse(context.Background(), init.State, []byte(jweToken))
		return redirect, svcErr
	}

	b := newPIDBuilder(suite.T())
	svc, store := newTestService(suite.T(), b)

	// Unconfigured base => no redirect.
	redirect, svcErr := submitVP(svc, store, b)
	suite.Nil(svcErr)
	suite.Empty(redirect)

	// Plain base URL => state appended with ?.
	svc.cfg.ResultRedirectURIBase = resultRedirectURIBase
	svc, store = newTestService(suite.T(), b)
	svc.cfg.ResultRedirectURIBase = resultRedirectURIBase
	redirect, svcErr = submitVP(svc, store, b)
	suite.Nil(svcErr)
	suite.Contains(redirect, "state=")
	suite.Contains(redirect, resultRedirectURIBase)

	// Base URL with existing query param => state appended with &.
	svc, store = newTestService(suite.T(), b)
	svc.cfg.ResultRedirectURIBase = "https://verifier.example/result?ui=qr"
	redirect, svcErr = submitVP(svc, store, b)
	suite.Nil(svcErr)
	suite.Contains(redirect, "ui=qr&state=")
}

func (suite *OpenID4VPServiceTestSuite) TestInitiateRejectsUnknownDefinition() {
	b := newPIDBuilder(suite.T())
	svc, _ := newTestService(suite.T(), b)

	_, err := svc.Initiate(context.Background(), "no-such-def")
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

func (suite *OpenID4VPServiceTestSuite) TestAuthenticate_NilCredential() {
	svc, _ := newTestService(suite.T(), newPIDBuilder(suite.T()))
	result, err := svc.Authenticate(context.Background(), nil)
	suite.Nil(result)
	suite.NotNil(err)
}

func (suite *OpenID4VPServiceTestSuite) TestAuthenticate_ValidCredential() {
	svc, _ := newTestService(suite.T(), newPIDBuilder(suite.T()))
	cred := &authncommon.OpenID4VPCredential{
		Subject: "sub-42",
		Claims:  map[string]interface{}{"given_name": "Ada", "family_name": "Lovelace"},
	}
	result, err := svc.Authenticate(context.Background(), cred)
	suite.Require().Nil(err)
	suite.Require().NotNil(result)
	// Identification token keys on subject only so IdentifyEntity matches by sub.
	suite.Equal("sub-42", result.Token["sub"])
	suite.Len(result.Token, 1)
	// AuthenticatedClaims carries subject + all disclosed claims.
	suite.Equal("sub-42", result.AuthenticatedClaims["sub"])
	suite.Equal("Ada", result.AuthenticatedClaims["given_name"])
	suite.Equal("Lovelace", result.AuthenticatedClaims["family_name"])
}

func (suite *OpenID4VPServiceTestSuite) TestAuthenticate_NoSubject() {
	svc, _ := newTestService(suite.T(), newPIDBuilder(suite.T()))
	cred := &authncommon.OpenID4VPCredential{
		Claims: map[string]interface{}{"tax_id": "DE123456"},
	}
	result, err := svc.Authenticate(context.Background(), cred)
	suite.Require().Nil(err)
	suite.Require().NotNil(result)
	// With no subject the identification token is empty — IdentifyEntity finds
	// nothing and the holder is provisioned just-in-time.
	suite.Empty(result.Token)
	suite.Equal("DE123456", result.AuthenticatedClaims["tax_id"])
}

func (suite *OpenID4VPServiceTestSuite) TestGetResultDeletesOnCompleted() {
	b := newPIDBuilder(suite.T())
	svc, store := newTestService(suite.T(), b)
	svc.jwtSvc = stubResultTokenIssuer(suite.T())
	svc.issuerURL = testIssuerURL

	init, svcErr := svc.Initiate(context.Background(), testDefinitionID)
	suite.Require().Nil(svcErr)
	store[init.State].Status = StatusCompleted
	store[init.State].Result = &VerifiedPresentation{Subject: "sub-1"}

	rs, svcErr := svc.GetResult(context.Background(), init.State)
	suite.Require().Nil(svcErr)
	suite.Equal(StatusCompleted, rs.Status)
	suite.NotNil(rs.Result)
	_, ok := store[init.State]
	suite.False(ok, "completed entry must be deleted by GetResult")
}

func (suite *OpenID4VPServiceTestSuite) TestGetResultDeletesOnFailed() {
	b := newPIDBuilder(suite.T())
	svc, store := newTestService(suite.T(), b)

	init, svcErr := svc.Initiate(context.Background(), testDefinitionID)
	suite.Require().Nil(svcErr)
	store[init.State].Status = StatusFailed
	store[init.State].FailureReason = "nonce mismatch"

	rs, svcErr := svc.GetResult(context.Background(), init.State)
	suite.Require().Nil(svcErr)
	suite.Equal(StatusFailed, rs.Status)
	_, ok := store[init.State]
	suite.False(ok, "failed entry must be deleted by GetResult")
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

const (
	testVCT       = "urn:eudi:pid:de:1"
	testNonce     = "n-0S6_WzA2Mj"
	testIssuer    = "https://pid.bundesdruckerei.de"
	testAudience  = "x509_hash:test-verifier"
	testIssuerURL = "https://verifier.example"
)

// pidBuilder assembles valid SD-JWT PID credentials backed by a generated
// root CA and issuer certificate, for use as test fixtures.
type pidBuilder struct {
	t          *testing.T
	issuerKey  *ecdsa.PrivateKey
	issuerCert *x509.Certificate
	holderKey  *ecdsa.PrivateKey
	rootCert   *x509.Certificate
}

func newPIDBuilder(t *testing.T) *pidBuilder {
	t.Helper()
	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	rootTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-root"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(10 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		SubjectKeyId:          []byte{1, 2, 3, 4},
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTmpl, rootTmpl, &rootKey.PublicKey, rootKey)
	require.NoError(t, err)
	rootCert, err := x509.ParseCertificate(rootDER)
	require.NoError(t, err)

	issuerKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	issuerTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "test-issuer"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(10 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		SubjectKeyId:          []byte{5, 6, 7, 8},
	}
	issuerDER, err := x509.CreateCertificate(rand.Reader, issuerTmpl, rootCert, &issuerKey.PublicKey, rootKey)
	require.NoError(t, err)
	issuerCert, err := x509.ParseCertificate(issuerDER)
	require.NoError(t, err)

	holderKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	return &pidBuilder{
		t:          t,
		issuerKey:  issuerKey,
		issuerCert: issuerCert,
		holderKey:  holderKey,
		rootCert:   rootCert,
	}
}

func (b *pidBuilder) trustStore() *trustAnchorStore {
	b.t.Helper()
	return newTrustAnchorStore([]*x509.Certificate{b.rootCert}, []string{"test-root"})
}

// build creates a valid SD-JWT PID presentation disclosing claims, with a Key
// Binding JWT signed by the holder key using the given nonce.
func (b *pidBuilder) build(nonce string, claims map[string]interface{}) string {
	b.t.Helper()

	raw, err := b.holderKey.PublicKey.Bytes()
	require.NoError(b.t, err)
	require.Len(b.t, raw, 65)
	cnf := map[string]interface{}{
		"kty": "EC", "crv": "P-256",
		"x": base64.RawURLEncoding.EncodeToString(raw[1:33]),
		"y": base64.RawURLEncoding.EncodeToString(raw[33:]),
	}

	combined, _, err := sdjwt.Issue(sdjwt.IssueParams{
		Header: map[string]interface{}{
			"alg": "ES256", "typ": "dc+sd-jwt",
			"x5c": []string{base64.StdEncoding.EncodeToString(b.issuerCert.Raw)},
		},
		Issuer:          testIssuer,
		VCT:             testVCT,
		IssuedAt:        time.Now(),
		ExpiresAt:       time.Now().Add(time.Hour),
		SelectiveClaims: claims,
		ConfirmationJWK: cnf,
	}, func(signingInput string) ([]byte, error) {
		h := sha256.Sum256([]byte(signingInput))
		r, s, signErr := ecdsa.Sign(rand.Reader, b.issuerKey, h[:])
		if signErr != nil {
			return nil, signErr
		}
		sig := make([]byte, 64)
		r.FillBytes(sig[:32])
		s.FillBytes(sig[32:])
		return sig, nil
	})
	require.NoError(b.t, err)

	sdHash := sha256.Sum256([]byte(combined))
	kbHeader, _ := json.Marshal(map[string]interface{}{"alg": "ES256", "typ": "kb+jwt"})
	kbPayload, _ := json.Marshal(map[string]interface{}{
		"aud":     testAudience,
		"nonce":   nonce,
		"iat":     time.Now().Unix(),
		"sd_hash": base64.RawURLEncoding.EncodeToString(sdHash[:]),
	})
	kbInput := base64.RawURLEncoding.EncodeToString(kbHeader) + "." +
		base64.RawURLEncoding.EncodeToString(kbPayload)
	h := sha256.Sum256([]byte(kbInput))
	r, s, err := ecdsa.Sign(rand.Reader, b.holderKey, h[:])
	require.NoError(b.t, err)
	kbSig := make([]byte, 64)
	r.FillBytes(kbSig[:32])
	s.FillBytes(kbSig[32:])

	return combined + kbInput + "." + base64.RawURLEncoding.EncodeToString(kbSig)
}

// testVerifier wraps the verification path for use in table-driven tests.
type testVerifier struct {
	trust  *trustAnchorStore
	policy policy
}

func newTestVerifier(t *testing.T, b *pidBuilder, p policy) *testVerifier {
	t.Helper()
	return &testVerifier{trust: b.trustStore(), policy: p}
}

func (v *testVerifier) verifyResponse(body []byte, credID, nonce string) (*VerifiedPresentation, error) {
	resp, err := parseAuthorizationResponse(body)
	if err != nil {
		return nil, err
	}
	presentations, err := resp.presentationsFor(credID)
	if err != nil {
		return nil, err
	}
	if len(presentations) == 0 {
		return nil, fmt.Errorf("no presentations found for %q", credID)
	}
	cred, err := verifySDJWTPresentation(
		presentations[0], v.trust, v.policy.Audience, nonce,
		v.policy.Leeway, v.policy.KeyBindingMaxAge,
		v.policy.EnforceTrustedIssuer, v.policy.EnforceKeyBinding,
		v.policy.TrustedAuthorities)
	if err != nil {
		return nil, err
	}
	return finalizePresentation(cred, v.policy)
}

func defaultPolicy() policy {
	return policy{
		ExpectedVCT:          testVCT,
		Audience:             testAudience,
		RequestedClaims:      []string{"given_name", "family_name", "birthdate"},
		MandatoryClaims:      []string{"given_name", "family_name"},
		EnforceTrustedIssuer: true,
		EnforceKeyBinding:    true,
	}
}

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

// stubResultTokenIssuer returns a mock JWT service that emits an unsigned JWT, so
// API tests can decode and assert on the payload without standing up a real signer.
func stubResultTokenIssuer(t *testing.T) *jwtmock.JWTServiceInterfaceMock {
	t.Helper()
	jwtSvc := jwtmock.NewJWTServiceInterfaceMock(t)
	jwtSvc.EXPECT().GenerateJWT(mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, _ string, _ string, _ int64,
			claims map[string]interface{}, _ string, _ string) (string, int64, *tidcommon.ServiceError) {
			payload, _ := json.Marshal(claims)
			header, _ := json.Marshal(map[string]string{"alg": "none", "typ": "JWT"})
			token := base64.RawURLEncoding.EncodeToString(header) + "." +
				base64.RawURLEncoding.EncodeToString(payload) + "."
			return token, 0, nil
		}).Maybe()
	return jwtSvc
}

// failingResultTokenIssuer returns a mock JWT service whose GenerateJWT always fails.
func failingResultTokenIssuer(t *testing.T) *jwtmock.JWTServiceInterfaceMock {
	t.Helper()
	jwtSvc := jwtmock.NewJWTServiceInterfaceMock(t)
	jwtSvc.EXPECT().GenerateJWT(mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return("", int64(0), &tidcommon.InternalServerError).Maybe()
	return jwtSvc
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

func (suite *OpenID4VPServiceTestSuite) TestGetStatusUnknownTxn() {
	b := newPIDBuilder(suite.T())
	svc, _ := newTestService(suite.T(), b)

	ts, svcErr := svc.GetResult(context.Background(), "no-such-txn")
	suite.Nil(ts)
	suite.NotNil(svcErr)
}

func (suite *OpenID4VPServiceTestSuite) TestGetStatusPending() {
	b := newPIDBuilder(suite.T())
	svc, _ := newTestService(suite.T(), b)

	init, svcErr := svc.Initiate(context.Background(), testDefinitionID)
	suite.Require().Nil(svcErr)

	ts, svcErr := svc.GetResult(context.Background(), init.State)
	suite.Require().Nil(svcErr)
	suite.Equal(StatusPending, ts.Status)
	suite.Empty(ts.ResultToken)
}

func (suite *OpenID4VPServiceTestSuite) TestGetStatusFailed() {
	b := newPIDBuilder(suite.T())
	svc, store := newTestService(suite.T(), b)

	init, svcErr := svc.Initiate(context.Background(), testDefinitionID)
	suite.Require().Nil(svcErr)

	store[init.State].Status = StatusFailed
	store[init.State].FailureReason = "untrusted_issuer"

	ts, svcErr := svc.GetResult(context.Background(), init.State)
	suite.Require().Nil(svcErr)
	suite.Equal(StatusFailed, ts.Status)
	suite.Equal("untrusted_issuer", ts.FailureReason)
}

func (suite *OpenID4VPServiceTestSuite) TestGetStatusExpired() {
	b := newPIDBuilder(suite.T())
	svc, store := newTestService(suite.T(), b)

	init, svcErr := svc.Initiate(context.Background(), testDefinitionID)
	suite.Require().Nil(svcErr)

	store[init.State].ExpiresAt = time.Now().Add(-time.Minute)

	ts, svcErr := svc.GetResult(context.Background(), init.State)
	suite.Require().Nil(svcErr)
	suite.Equal(StatusExpired, ts.Status)
}

func (suite *OpenID4VPServiceTestSuite) TestGetStatusCompletedWithIssuer() {
	t := suite.T()
	b := newPIDBuilder(t)
	svc, store := newTestService(t, b)
	svc.jwtSvc = stubResultTokenIssuer(t)
	svc.issuerURL = testIssuerURL

	init, svcErr := svc.Initiate(context.Background(), testDefinitionID)
	suite.Require().Nil(svcErr)

	store[init.State].Status = StatusCompleted
	store[init.State].Result = &VerifiedPresentation{
		Subject: "sub",
		Claims:  map[string]interface{}{"given_name": "Erika"},
	}

	ts, svcErr := svc.GetResult(context.Background(), init.State)
	suite.Require().Nil(svcErr)
	suite.Equal(StatusCompleted, ts.Status)
	suite.NotEmpty(ts.ResultToken)

	claims := decodeFakeToken(t, ts.ResultToken)
	suite.Equal(testIssuerURL, claims["aud"])
	suite.Equal(init.State, claims["jti"])
	suite.Equal(init.State, claims["txn"])
}

func (suite *OpenID4VPServiceTestSuite) TestGetStatusCompletedNoIssuer() {
	b := newPIDBuilder(suite.T())
	svc, store := newTestService(suite.T(), b)

	init, svcErr := svc.Initiate(context.Background(), testDefinitionID)
	suite.Require().Nil(svcErr)

	store[init.State].Status = StatusCompleted
	store[init.State].Result = &VerifiedPresentation{Subject: "sub"}

	ts, svcErr := svc.GetResult(context.Background(), init.State)
	suite.Nil(ts)
	suite.NotNil(svcErr)
}

func (suite *OpenID4VPServiceTestSuite) TestGetStatusCompletedIssuerError() {
	t := suite.T()
	b := newPIDBuilder(t)
	svc, store := newTestService(t, b)
	svc.jwtSvc = failingResultTokenIssuer(t)
	svc.issuerURL = testIssuerURL

	init, svcErr := svc.Initiate(context.Background(), testDefinitionID)
	suite.Require().Nil(svcErr)

	store[init.State].Status = StatusCompleted
	store[init.State].Result = &VerifiedPresentation{Subject: "sub"}

	ts, svcErr := svc.GetResult(context.Background(), init.State)
	suite.Nil(ts)
	suite.NotNil(svcErr)
}

func (suite *OpenID4VPServiceTestSuite) TestGetStatusUnknownStatusValue() {
	b := newPIDBuilder(suite.T())
	svc, store := newTestService(suite.T(), b)

	init, svcErr := svc.Initiate(context.Background(), testDefinitionID)
	suite.Require().Nil(svcErr)

	store[init.State].Status = Status("BOGUS")

	// GetResult passes unknown status values through without erroring.
	ts, svcErr := svc.GetResult(context.Background(), init.State)
	suite.Nil(svcErr)
	suite.Equal(Status("BOGUS"), ts.Status)
}

// newTestRootCA generates a self-signed root CA certificate and returns the cert and key.
func newTestRootCA(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-root"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(10 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		SubjectKeyId:          []byte{1, 2, 3, 4},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return cert, key
}
