// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/defaultkm"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
)

// testCryptoKey is the encryption key used so secret property encryption works in these tests.
const testCryptoKey = "0579f866ac7c9273580d0ff163fa01a7b2401a7ff3ddc3e3b14ae3136fa6025e"

// testESignetSigningKeyPEM is a throwaway RSA key generated for the test binary. Generating it
// rather than embedding one keeps a private key, even a worthless one, out of the repository.
var testESignetSigningKeyPEM = func() string {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}))
}()

type ESignetOIDCExecutorTestSuite struct {
	suite.Suite
	mockIDPService    *idpmock.IDPServiceInterfaceMock
	mockFlowFactory   *coremock.FlowFactoryInterfaceMock
	mockAuthnProvider *managermock.AuthnProviderManagerMock
	executor          *esignetOIDCExecutor
}

func TestESignetOIDCExecutorSuite(t *testing.T) {
	suite.Run(t, new(ESignetOIDCExecutorTestSuite))
}

func (suite *ESignetOIDCExecutorTestSuite) SetupTest() {
	// The signing key is a secret property, so the config crypto provider has to be wired up
	// before the fixtures can encrypt it.
	config.ResetServerRuntime()
	suite.Require().NoError(config.InitializeServerRuntime(suite.T().TempDir(), &config.Config{
		Crypto: config.CryptoConfig{Encryption: engineconfig.EncryptionConfig{Key: testCryptoKey}},
	}))
	suite.T().Cleanup(config.ResetServerRuntime)
	_, cfgCryptoSvc, err := defaultkm.Initialize(nil)
	suite.Require().NoError(err)
	cmodels.SetConfigCryptoProvider(cfgCryptoSvc)

	suite.mockIDPService = idpmock.NewIDPServiceInterfaceMock(suite.T())
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	suite.mockAuthnProvider = managermock.NewAuthnProviderManagerMock(suite.T())

	mockExec := createMockAuthExecutor(suite.T(), ExecutorNameESignetOIDC)
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameESignetOIDC,
		providers.ExecutorTypeAuthentication, mock.Anything, mock.Anything, mock.Anything).Return(mockExec)

	suite.executor = newESignetOIDCExecutor(
		suite.mockFlowFactory, suite.mockIDPService, suite.mockAuthnProvider).(*esignetOIDCExecutor)
}

// esignetConnectionProperties returns a complete eSignet connection property set, with `scopes`
// stored in the comma-separated form the IdP layer persists.
func (suite *ESignetOIDCExecutorTestSuite) esignetConnectionProperties() []cmodels.Property {
	values := []struct {
		name  string
		value string
	}{
		{idp.PropClientID, "thunderid-esignet"},
		{idp.PropRedirectURI, "https://localhost:8090/gate/callback"},
		{idp.PropAuthorizationEndpoint, "https://esignet.example.com/authorize"},
		{idp.PropTokenEndpoint, "https://esignet.example.com/token"},
		{idp.PropUserInfoEndpoint, "https://esignet.example.com/userinfo"},
		{idp.PropJwksEndpoint, "https://esignet.example.com/jwks.json"},
		{idp.PropSigningKey, testESignetSigningKeyPEM},
		{idp.PropSigningKeyID, "kid-1"},
		{idp.PropScopes, "openid,profile"},
		{idp.PropACRValues, "mosip:idp:acr:generated-code"},
		{idp.PropUsernamePrefix, "esignet-"},
	}
	properties := make([]cmodels.Property, 0, len(values))
	for _, entry := range values {
		// The signing key is the one secret property, so it round trips through encryption here
		// exactly as it does in storage.
		prop, err := cmodels.NewProperty(entry.name, entry.value, entry.name == idp.PropSigningKey)
		suite.Require().NoError(err)
		properties = append(properties, *prop)
	}
	return properties
}

func (suite *ESignetOIDCExecutorTestSuite) nodeContext(idpID string) *providers.NodeContext {
	return &providers.NodeContext{
		// The outbound calls to eSignet are built with this context, so it has to be a real one.
		Context:        context.Background(),
		ExecutionID:    "flow-123",
		FlowType:       providers.FlowTypeAuthentication,
		UserInputs:     map[string]string{},
		RuntimeData:    map[string]string{},
		NodeInputs:     []providers.Input{{Identifier: "code", Type: "string", Required: true}},
		NodeProperties: map[string]interface{}{"idpId": idpID},
	}
}

func (suite *ESignetOIDCExecutorTestSuite) TestSupportedPropertiesAreConnectionBacked() {
	// The flow validator rejects unsupported node properties, so the executor must declare
	// exactly the connection-backed shape and none of the old raw endpoint properties.
	var meta *providers.ExecutorMeta
	for _, call := range suite.mockFlowFactory.Calls {
		if call.Method == "CreateExecutor" {
			meta = call.Arguments[4].(*providers.ExecutorMeta)
		}
	}
	suite.Require().NotNil(meta)

	names := make([]string, 0)
	for _, prop := range meta.SupportedProperties {
		names = append(names, prop.Property)
	}
	suite.ElementsMatch([]string{
		"idpId",
		common.NodePropertyAllowAuthenticationWithoutLocalUser,
		common.NodePropertyAllowRegistrationWithExistingUser,
	}, names)
}

func (suite *ESignetOIDCExecutorTestSuite) TestReadConfigResolvesConnection() {
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, "es-1").
		Return(&providers.IDPDTO{
			ID:         "es-1",
			Type:       providers.IDPTypeESignet,
			Properties: suite.esignetConnectionProperties(),
		}, (*tidcommon.ServiceError)(nil))

	cfg, err := suite.executor.readESignetConfig(suite.nodeContext("es-1"))

	suite.Require().NoError(err)
	suite.Equal("thunderid-esignet", cfg.clientID)
	suite.Equal("https://esignet.example.com/authorize", cfg.authorizeEndpoint)
	suite.Equal("https://esignet.example.com/jwks.json", cfg.jwksURI)
	// The signing key is resolved but left encrypted, so reading it is what decrypts it.
	suite.Require().NotNil(cfg.signingKey)
	signingKey, err := cfg.signingKey.GetValue()
	suite.Require().NoError(err)
	suite.Equal(testESignetSigningKeyPEM, signingKey)
	suite.Equal("kid-1", cfg.signingKeyID)
	suite.Equal("mosip:idp:acr:generated-code", cfg.acrValues)
	suite.Equal("esignet-", cfg.usernamePrefix)
}

// The IdP layer stores scopes comma separated; the authorization request needs them space
// separated, so the conversion happens on read.
func (suite *ESignetOIDCExecutorTestSuite) TestReadConfigConvertsScopesToSpaceSeparated() {
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, "es-1").
		Return(&providers.IDPDTO{
			ID:         "es-1",
			Type:       providers.IDPTypeESignet,
			Properties: suite.esignetConnectionProperties(),
		}, (*tidcommon.ServiceError)(nil))

	cfg, err := suite.executor.readESignetConfig(suite.nodeContext("es-1"))

	suite.Require().NoError(err)
	suite.Equal("openid profile", cfg.scopes)
}

func (suite *ESignetOIDCExecutorTestSuite) TestReadConfigMissingIdpID() {
	ctx := suite.nodeContext("es-1")
	ctx.NodeProperties = map[string]interface{}{}

	_, err := suite.executor.readESignetConfig(ctx)

	suite.Require().Error(err)
	suite.Contains(err.Error(), "idpId")
}

func (suite *ESignetOIDCExecutorTestSuite) TestReadConfigConnectionNotFound() {
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, "missing").
		Return((*providers.IDPDTO)(nil), &idp.ErrorIDPNotFound)

	_, err := suite.executor.readESignetConfig(suite.nodeContext("missing"))

	suite.Require().Error(err)
	suite.Contains(err.Error(), "missing")
}

// Pointing the node at a non-eSignet connection must fail rather than half-configure the
// executor from a provider that carries none of the required properties.
func (suite *ESignetOIDCExecutorTestSuite) TestReadConfigRejectsWrongConnectionType() {
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, "g-1").
		Return(&providers.IDPDTO{ID: "g-1", Type: providers.IDPTypeGoogle}, (*tidcommon.ServiceError)(nil))

	_, err := suite.executor.readESignetConfig(suite.nodeContext("g-1"))

	suite.Require().Error(err)
	suite.Contains(err.Error(), string(providers.IDPTypeESignet))
}

func (suite *ESignetOIDCExecutorTestSuite) TestReadConfigMissingRequiredProperty() {
	properties := suite.esignetConnectionProperties()
	filtered := make([]cmodels.Property, 0, len(properties))
	for _, prop := range properties {
		if prop.GetName() != idp.PropSigningKey {
			filtered = append(filtered, prop)
		}
	}
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, "es-1").
		Return(&providers.IDPDTO{
			ID: "es-1", Type: providers.IDPTypeESignet, Properties: filtered,
		}, (*tidcommon.ServiceError)(nil))

	_, err := suite.executor.readESignetConfig(suite.nodeContext("es-1"))

	suite.Require().Error(err)
	suite.Contains(err.Error(), idp.PropSigningKey)
}

func (suite *ESignetOIDCExecutorTestSuite) TestExecuteBuildsAuthorizeRedirectFromConnection() {
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, "es-1").
		Return(&providers.IDPDTO{
			ID:         "es-1",
			Type:       providers.IDPTypeESignet,
			Properties: suite.esignetConnectionProperties(),
		}, (*tidcommon.ServiceError)(nil))

	execResp, err := suite.executor.Execute(suite.nodeContext("es-1"))

	suite.Require().NoError(err)
	suite.Equal(providers.ExecExternalRedirection, execResp.Status)

	parsed, err := url.Parse(execResp.RedirectURL)
	suite.Require().NoError(err)
	suite.Equal("https://esignet.example.com/authorize", parsed.Scheme+"://"+parsed.Host+parsed.Path)

	query := parsed.Query()
	suite.Equal("thunderid-esignet", query.Get("client_id"))
	suite.Equal("https://localhost:8090/gate/callback", query.Get("redirect_uri"))
	suite.Equal("code", query.Get("response_type"))
	suite.Equal("openid profile", query.Get("scope"))
	suite.Equal("mosip:idp:acr:generated-code", query.Get("acr_values"))
	suite.Equal("S256", query.Get("code_challenge_method"))
	suite.NotEmpty(query.Get("code_challenge"))
	suite.NotEmpty(query.Get("nonce"))

	// The PKCE verifier, nonce and state must be stashed for the callback leg to check against.
	suite.NotEmpty(execResp.RuntimeData[runtimeKeyESignetNonce])
	suite.NotEmpty(execResp.RuntimeData[runtimeKeyESignetCodeVerifier])
	suite.NotEmpty(execResp.RuntimeData[runtimeKeyESignetState])

	// The state on the wire is the one the callback will be checked against.
	suite.Equal(execResp.RuntimeData[runtimeKeyESignetState], query.Get("state"))
	suite.Equal(execResp.RuntimeData[runtimeKeyESignetNonce], query.Get("nonce"))
}

// withProperty returns props with one property's value replaced, so a test can point a single
// endpoint at a local server without restating the whole connection.
func (suite *ESignetOIDCExecutorTestSuite) withProperty(
	props []cmodels.Property, name, value string,
) []cmodels.Property {
	out := make([]cmodels.Property, 0, len(props))
	for i := range props {
		if props[i].GetName() != name {
			out = append(out, props[i])
			continue
		}
		prop, err := cmodels.NewProperty(name, value, props[i].IsSecret())
		suite.Require().NoError(err)
		out = append(out, *prop)
	}
	return out
}

// callbackContext builds a callback-leg node context carrying the values pass one would have
// stashed, so a test only has to vary the state the client returns.
func (suite *ESignetOIDCExecutorTestSuite) callbackContext(returnedState string) *providers.NodeContext {
	ctx := suite.nodeContext("es-1")
	ctx.UserInputs[userInputCode] = "auth-code"
	if returnedState != "" {
		ctx.UserInputs[userInputState] = returnedState
	}
	ctx.RuntimeData[runtimeKeyESignetNonce] = "stashed-nonce"
	ctx.RuntimeData[runtimeKeyESignetCodeVerifier] = "stashed-verifier"
	ctx.RuntimeData[runtimeKeyESignetState] = "stashed-state"
	return ctx
}

// State is a second CSRF check on the callback, alongside the PKCE verifier and the nonce. A
// returned state that does not match the one minted for this execution fails the step before the
// authorization code is ever presented to eSignet.
func (suite *ESignetOIDCExecutorTestSuite) TestCallbackRejectsMismatchedState() {
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, "es-1").
		Return(&providers.IDPDTO{
			ID:         "es-1",
			Type:       providers.IDPTypeESignet,
			Properties: suite.esignetConnectionProperties(),
		}, (*tidcommon.ServiceError)(nil))

	ctx := suite.callbackContext("attacker-state")
	execResp, err := suite.executor.Execute(ctx)

	suite.Require().NoError(err)
	suite.Equal(providers.ExecFailure, execResp.Status)
	suite.Require().NotNil(execResp.Error)
	suite.Equal(ErrInvalidOAuthState.Code, execResp.Error.Code)

	// The single-use values must not survive a rejected callback.
	suite.NotContains(ctx.RuntimeData, runtimeKeyESignetState)
	suite.NotContains(ctx.RuntimeData, runtimeKeyESignetNonce)
	suite.NotContains(ctx.RuntimeData, runtimeKeyESignetCodeVerifier)
}

// A matching state passes the gate: the step goes on to the token exchange, which is what fails
// here, rather than being rejected as a state mismatch.
func (suite *ESignetOIDCExecutorTestSuite) TestCallbackAcceptsMatchingState() {
	suite.Equal(ErrESignetTokenExchangeFailed.Code, suite.runCallbackPastStateGate("stashed-state"))
}

// A client that omits state on the callback is not rejected: the PKCE verifier and the nonce still
// bind the callback to this execution. This matches how the OAuth executor treats an absent state.
func (suite *ESignetOIDCExecutorTestSuite) TestCallbackToleratesAbsentState() {
	suite.Equal(ErrESignetTokenExchangeFailed.Code, suite.runCallbackPastStateGate(""))
}

// runCallbackPastStateGate runs the callback leg against a token endpoint that always rejects,
// and returns the resulting error code. Reaching the token exchange at all is the assertion: it
// proves the state check let the callback through.
func (suite *ESignetOIDCExecutorTestSuite) runCallbackPastStateGate(returnedState string) string {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer tokenServer.Close()

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, "es-1").
		Return(&providers.IDPDTO{
			ID:   "es-1",
			Type: providers.IDPTypeESignet,
			Properties: suite.withProperty(
				suite.esignetConnectionProperties(), idp.PropTokenEndpoint, tokenServer.URL),
		}, (*tidcommon.ServiceError)(nil))

	execResp, err := suite.executor.Execute(suite.callbackContext(returnedState))

	suite.Require().NoError(err)
	suite.Equal(providers.ExecFailure, execResp.Status)
	suite.Require().NotNil(execResp.Error)
	return execResp.Error.Code
}

// rekeyConfigCrypto swaps the config crypto provider for one holding a different key, so any value
// encrypted before the swap can no longer be decrypted.
func (suite *ESignetOIDCExecutorTestSuite) rekeyConfigCrypto() {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	suite.Require().NoError(err)

	config.ResetServerRuntime()
	suite.Require().NoError(config.InitializeServerRuntime(suite.T().TempDir(), &config.Config{
		Crypto: config.CryptoConfig{Encryption: engineconfig.EncryptionConfig{Key: hex.EncodeToString(key)}},
	}))
	_, cfgCryptoSvc, err := defaultkm.Initialize(nil)
	suite.Require().NoError(err)
	cmodels.SetConfigCryptoProvider(cfgCryptoSvc)
}

// The authorize leg signs nothing, so it must not decrypt the signing key. Re-keying the crypto
// provider makes the stored key undecryptable, which is invisible to the redirect and fatal only to
// the callback: that difference is what proves the key is read at the signing point and nowhere else.
func (suite *ESignetOIDCExecutorTestSuite) TestSigningKeyIsDecryptedOnlyOnTheCallbackLeg() {
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, "es-1").
		Return(&providers.IDPDTO{
			ID:         "es-1",
			Type:       providers.IDPTypeESignet,
			Properties: suite.esignetConnectionProperties(),
		}, (*tidcommon.ServiceError)(nil))

	suite.rekeyConfigCrypto()

	redirect, err := suite.executor.Execute(suite.nodeContext("es-1"))
	suite.Require().NoError(err)
	suite.Equal(providers.ExecExternalRedirection, redirect.Status, "the redirect must not need the key")
	suite.NotEmpty(redirect.RedirectURL)

	// The callback does sign, so the same connection now fails there, before any request is sent.
	callback, err := suite.executor.Execute(suite.callbackContext("stashed-state"))
	suite.Require().NoError(err)
	suite.Equal(providers.ExecFailure, callback.Status)
	suite.Require().NotNil(callback.Error)
	suite.Equal(ErrESignetTokenExchangeFailed.Code, callback.Error.Code)
}

func (suite *ESignetOIDCExecutorTestSuite) TestExecuteFailsWhenConnectionIsMissing() {
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, "missing").
		Return((*providers.IDPDTO)(nil), &idp.ErrorIDPNotFound)

	execResp, err := suite.executor.Execute(suite.nodeContext("missing"))

	suite.Require().NoError(err)
	suite.Equal(providers.ExecFailure, execResp.Status)
	suite.Require().NotNil(execResp.Error)
	suite.Equal(ErrESignetConfigInvalid.Code, execResp.Error.Code)
}

// RFC 7519 lets aud be a single string or an array of them, and eSignet emits the array form.
// Asserting a plain string yielded "" for the array case, which failed every login.
func (suite *ESignetOIDCExecutorTestSuite) TestAudienceContainsAcceptsBothClaimShapes() {
	cases := []struct {
		name  string
		aud   interface{}
		match bool
	}{
		{"single string", "thunderid-esignet", true},
		{"single string, other client", "someone-else", false},
		{"array with one entry", []interface{}{"thunderid-esignet"}, true},
		{"array among several", []interface{}{"someone-else", "thunderid-esignet"}, true},
		{"array without the client", []interface{}{"someone-else"}, false},
		{"typed string slice", []string{"thunderid-esignet"}, true},
		{"empty array", []interface{}{}, false},
		{"absent", nil, false},
		{"wrong element type", []interface{}{42}, false},
	}

	for _, tc := range cases {
		claims := jwt.MapClaims{}
		if tc.aud != nil {
			claims["aud"] = tc.aud
		}
		suite.Equal(tc.match, audienceContains(claims, "thunderid-esignet"), tc.name)
	}
}

// The client assertion must be signable from the stored PEM alone, with no filesystem access.
func (suite *ESignetOIDCExecutorTestSuite) TestBuildClientAssertionSignsFromStoredPEM() {
	assertion, err := buildClientAssertion(
		"thunderid-esignet", "https://esignet.example.com/token", testESignetSigningKeyPEM, "kid-1")
	suite.Require().NoError(err)

	parsed, _, err := jwt.NewParser().ParseUnverified(assertion, jwt.MapClaims{})
	suite.Require().NoError(err)
	suite.Equal("kid-1", parsed.Header["kid"])
	suite.Equal("RS256", parsed.Header["alg"])

	claims, ok := parsed.Claims.(jwt.MapClaims)
	suite.Require().True(ok)
	suite.Equal("thunderid-esignet", claims["iss"])
	suite.Equal("thunderid-esignet", claims["sub"])
	suite.Equal("https://esignet.example.com/token", claims["aud"])
	suite.NotEmpty(claims["jti"])
}

// The key is stored on a single line, its newlines written as the two-character sequence \n, so
// that configuration export can carry it in a .env file. That form must sign just as the
// real-newline form does, and both must verify against the same public key.
func (suite *ESignetOIDCExecutorTestSuite) TestBuildClientAssertionAcceptsSingleLineSigningKey() {
	singleLine := strings.ReplaceAll(testESignetSigningKeyPEM, "\n", `\n`)
	suite.Require().NotContains(singleLine, "\n")

	key, err := parseRSAPrivateKey(testESignetSigningKeyPEM)
	suite.Require().NoError(err)

	for _, form := range []struct {
		name string
		pem  string
	}{
		{"single line", singleLine},
		{"real newlines", testESignetSigningKeyPEM},
	} {
		assertion, err := buildClientAssertion(
			"thunderid-esignet", "https://esignet.example.com/token", form.pem, "kid-1")
		suite.Require().NoError(err, form.name)

		parsed, err := jwt.NewParser().Parse(assertion, func(*jwt.Token) (interface{}, error) {
			return &key.PublicKey, nil
		})
		suite.Require().NoError(err, form.name)
		suite.True(parsed.Valid, form.name)
	}
}

// A PEM that arrives with surrounding whitespace, as a pasted key often does, must still parse.
func (suite *ESignetOIDCExecutorTestSuite) TestParseRSAPrivateKeyTolerantOfSurroundingWhitespace() {
	key, err := parseRSAPrivateKey("\n  " + testESignetSigningKeyPEM + "  \n")
	suite.Require().NoError(err)
	suite.NotNil(key)
}

func (suite *ESignetOIDCExecutorTestSuite) TestParseRSAPrivateKeyRejectsGarbage() {
	_, err := parseRSAPrivateKey("not a pem block")
	suite.Require().Error(err)
	suite.Contains(err.Error(), "PEM")
}

// jwksDocument builds the JWK set exposing the public half of the test signing key under kid.
func (suite *ESignetOIDCExecutorTestSuite) jwksDocument(kid string) map[string]interface{} {
	key, err := parseRSAPrivateKey(testESignetSigningKeyPEM)
	suite.Require().NoError(err)

	return map[string]interface{}{"keys": []map[string]string{{
		"kty": "RSA",
		"kid": kid,
		"use": "sig",
		"n":   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
	}}}
}

// serveJWKS starts a JWK set endpoint exposing the public half of the test signing key, so the
// verification path runs for real rather than against a stub.
func (suite *ESignetOIDCExecutorTestSuite) serveJWKS(kid string) *httptest.Server {
	jwks := suite.jwksDocument(kid)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		suite.Require().NoError(json.NewEncoder(w).Encode(jwks))
	}))
	suite.T().Cleanup(server.Close)
	return server
}

// signTestJWS signs claims with the test key using the given method, tagged with kid.
func (suite *ESignetOIDCExecutorTestSuite) signTestJWS(
	method jwt.SigningMethod, kid string, claims jwt.MapClaims,
) string {
	key, err := parseRSAPrivateKey(testESignetSigningKeyPEM)
	suite.Require().NoError(err)
	token := jwt.NewWithClaims(method, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(key)
	suite.Require().NoError(err)
	return signed
}

// eSignet advertises both RS256 and PS256 for id_token_signing_alg_values_supported and picks per
// deployment. Accepting only RS256 failed every login against a PS256 deployment with a signature
// error, so both must verify.
func (suite *ESignetOIDCExecutorTestSuite) TestVerifyCompactJWSAcceptsRS256AndPS256() {
	server := suite.serveJWKS("kid-1")

	for _, method := range []jwt.SigningMethod{jwt.SigningMethodRS256, jwt.SigningMethodPS256} {
		compact := suite.signTestJWS(method, "kid-1", jwt.MapClaims{"sub": "citizen-1"})

		claims, err := suite.executor.verifyCompactJWS(context.Background(), server.URL, compact)

		suite.Require().NoError(err, method.Alg())
		suite.Equal("citizen-1", claims["sub"], method.Alg())
	}
}

// The allowlist is what stops a token from choosing its own verification scheme, so an algorithm
// outside it must be refused even though the signature itself is well formed.
func (suite *ESignetOIDCExecutorTestSuite) TestVerifyCompactJWSRejectsUnlistedAlgorithm() {
	server := suite.serveJWKS("kid-1")

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "citizen-1"})
	token.Header["kid"] = "kid-1"
	compact, err := token.SignedString([]byte("symmetric-secret"))
	suite.Require().NoError(err)

	_, err = suite.executor.verifyCompactJWS(context.Background(), server.URL, compact)

	suite.Require().Error(err)
	suite.Contains(err.Error(), "HS256")
}

func (suite *ESignetOIDCExecutorTestSuite) TestVerifyCompactJWSRejectsUnknownKid() {
	server := suite.serveJWKS("kid-1")
	compact := suite.signTestJWS(jwt.SigningMethodRS256, "kid-other", jwt.MapClaims{"sub": "citizen-1"})

	_, err := suite.executor.verifyCompactJWS(context.Background(), server.URL, compact)

	suite.Require().Error(err)
	suite.Contains(err.Error(), "kid-other")
}

func (suite *ESignetOIDCExecutorTestSuite) TestAuthorizeScopesNormalizesStoredValue() {
	suite.Equal("openid profile", authorizeScopes("openid,profile"))
	suite.Equal("openid profile", authorizeScopes(" openid , profile "))
	suite.Equal("openid", authorizeScopes("openid,,"))
	suite.Equal("", authorizeScopes(""))
}

// The subject and access token the fake eSignet issues on the happy path.
const (
	testESignetSub         = "citizen-subject-0001"
	testESignetAccessToken = "esignet-access-token"
)

// fakeESignet is a stand-in eSignet deployment: it serves the token, userinfo and JWKS endpoints
// the callback leg calls, so the exchange, JWS verification and claim mapping all run for real
// rather than against stubbed-out internals. Each response is overridable so a test can inject the
// failure it is about, and the requests received are captured so the wire contract can be asserted.
type fakeESignet struct {
	server *httptest.Server

	// Response knobs.
	tokenStatus     int
	tokenBody       string
	userInfoStatus  int
	userInfoBody    string
	accessTokenSent string

	// Captured requests.
	tokenForm          url.Values
	userInfoAuthHeader string
}

// startFakeESignet serves a deployment that completes the callback successfully. A test tweaks the
// returned fixture's response fields before calling Execute to model a misbehaving eSignet.
func (suite *ESignetOIDCExecutorTestSuite) startFakeESignet() *fakeESignet {
	fake := &fakeESignet{
		tokenStatus:     http.StatusOK,
		userInfoStatus:  http.StatusOK,
		accessTokenSent: testESignetAccessToken,
	}
	fake.tokenBody = suite.tokenResponseBody(suite.signIDToken(nil))
	fake.userInfoBody = suite.signUserInfo(nil)

	mux := http.NewServeMux()
	mux.HandleFunc("/jwks.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		suite.Require().NoError(json.NewEncoder(w).Encode(suite.jwksDocument("kid-1")))
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		suite.Require().NoError(r.ParseForm())
		fake.tokenForm = r.PostForm

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(fake.tokenStatus)
		_, err := w.Write([]byte(fake.tokenBody))
		suite.Require().NoError(err)
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		fake.userInfoAuthHeader = r.Header.Get("Authorization")

		// eSignet is configured with userinfo_response_type=JWS, so the body is the compact JWS
		// itself rather than a JSON object.
		w.Header().Set("Content-Type", "application/jwt")
		w.WriteHeader(fake.userInfoStatus)
		_, err := w.Write([]byte(fake.userInfoBody))
		suite.Require().NoError(err)
	})

	fake.server = httptest.NewServer(mux)
	suite.T().Cleanup(fake.server.Close)
	return fake
}

// tokenResponseBody renders a token endpoint response carrying the given ID token.
func (suite *ESignetOIDCExecutorTestSuite) tokenResponseBody(idToken string) string {
	body, err := json.Marshal(map[string]string{
		"access_token": testESignetAccessToken,
		"id_token":     idToken,
		"token_type":   "Bearer",
	})
	suite.Require().NoError(err)
	return string(body)
}

// signIDToken signs an ID token that satisfies every check in the callback leg. Any claim can be
// replaced through overrides, and a nil value there drops the claim entirely, so a test can express
// exactly the one thing that is wrong with it.
func (suite *ESignetOIDCExecutorTestSuite) signIDToken(overrides map[string]interface{}) string {
	now := time.Now()
	claims := jwt.MapClaims{
		"iss": "https://esignet.example.com",
		// eSignet emits aud as an array, which is the shape audienceContains has to cope with.
		"aud":   []interface{}{"thunderid-esignet"},
		"sub":   testESignetSub,
		"nonce": "stashed-nonce",
		"iat":   now.Unix(),
		"exp":   now.Add(5 * time.Minute).Unix(),
	}
	for name, value := range overrides {
		if value == nil {
			delete(claims, name)
			continue
		}
		claims[name] = value
	}
	return suite.signTestJWS(jwt.SigningMethodRS256, "kid-1", claims)
}

// signUserInfo signs a userinfo response whose sub matches the ID token, with the profile claims
// the provisioning mapping reads. Overrides follow the same rules as signIDToken.
func (suite *ESignetOIDCExecutorTestSuite) signUserInfo(overrides map[string]interface{}) string {
	claims := jwt.MapClaims{
		"sub":       testESignetSub,
		"name":      "Kamala Perera",
		"birthdate": "1990-04-17",
	}
	for name, value := range overrides {
		if value == nil {
			delete(claims, name)
			continue
		}
		claims[name] = value
	}
	return suite.signTestJWS(jwt.SigningMethodRS256, "kid-1", claims)
}

// expectESignetConnection stubs the connection lookup with every endpoint pointed at the fake
// deployment, leaving the rest of the fixture connection intact.
func (suite *ESignetOIDCExecutorTestSuite) expectESignetConnection(fake *fakeESignet) {
	properties := suite.esignetConnectionProperties()
	properties = suite.withProperty(properties, idp.PropTokenEndpoint, fake.server.URL+"/token")
	properties = suite.withProperty(properties, idp.PropUserInfoEndpoint, fake.server.URL+"/userinfo")
	properties = suite.withProperty(properties, idp.PropJwksEndpoint, fake.server.URL+"/jwks.json")

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, "es-1").
		Return(&providers.IDPDTO{
			ID:         "es-1",
			Type:       providers.IDPTypeESignet,
			Properties: properties,
		}, (*tidcommon.ServiceError)(nil))
}

// expectFederatedAuthentication stubs a successful federated authentication and returns a pointer
// that receives the credential the executor handed over, so the mapped claims can be asserted.
func (suite *ESignetOIDCExecutorTestSuite) expectFederatedAuthentication() **authncm.FederatedClaimsCredential {
	captured := new(*authncm.FederatedClaimsCredential)
	authUser := newOAuthAuthenticatedUser()

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			credentials, ok := args.Get(2).(map[string]interface{})
			suite.Require().True(ok)
			federated := credentials[authnprovidercm.CredentialTypeFederatedClaims]
			credential, ok := federated.(*authncm.FederatedClaimsCredential)
			suite.Require().True(ok)
			*captured = credential
		}).
		Return(authUser, providers.AuthenticatedClaims{"sub": testESignetSub}, (*tidcommon.ServiceError)(nil))
	expectEntityReferenceResolved(suite.mockAuthnProvider, authUser)

	return captured
}

// runESignetCallback drives the callback leg against the fake deployment and returns the context it
// ran with alongside the response, so single-use runtime data can be inspected afterwards.
func (suite *ESignetOIDCExecutorTestSuite) runESignetCallback(
	fake *fakeESignet,
) (*providers.NodeContext, *providers.ExecutorResponse) {
	suite.expectESignetConnection(fake)

	ctx := suite.callbackContext("stashed-state")
	execResp, err := suite.executor.Execute(ctx)
	suite.Require().NoError(err)

	return ctx, execResp
}

// The callback leg is the executor's reason to exist: it has to exchange the code, verify both
// signed responses against the JWKS and map the verified claims onto the provisioning attributes.
// This runs that whole chain against a stand-in eSignet.
func (suite *ESignetOIDCExecutorTestSuite) TestCallbackCompletesFederatedAuthentication() {
	fake := suite.startFakeESignet()
	captured := suite.expectFederatedAuthentication()

	ctx, execResp := suite.runESignetCallback(fake)

	suite.Require().Nil(execResp.Error)
	suite.Equal(providers.ExecComplete, execResp.Status)
	suite.True(execResp.AuthUser.IsAuthenticated())

	// The verified identity is what reaches the authn provider, matched on the eSignet subject.
	credential := *captured
	suite.Require().NotNil(credential)
	suite.Equal(testESignetSub, credential.Subject)
	suite.Equal(esignetSubClaim, credential.MatchAttribute)
	suite.Equal(testESignetSub, credential.Claims[esignetSubClaim])
	suite.Equal("Kamala Perera", credential.Claims["name"])
	suite.Equal("1990-04-17", credential.Claims["birthdate"])
	suite.Equal("esignet-"+testESignetSub[:12], credential.Claims[userAttributeUsername])

	// Claims the provider authenticated with are published for later nodes in the flow.
	suite.Equal(testESignetSub, execResp.RuntimeData["sub"])
	suite.Equal(entityStateExists, execResp.RuntimeData[common.RuntimeKeyEntityState])

	// The nonce, verifier and state are single use and must not survive the callback.
	suite.NotContains(ctx.RuntimeData, runtimeKeyESignetNonce)
	suite.NotContains(ctx.RuntimeData, runtimeKeyESignetCodeVerifier)
	suite.NotContains(ctx.RuntimeData, runtimeKeyESignetState)
}

// eSignet authenticates clients with private_key_jwt and requires PKCE, so the token request has to
// carry a signed client assertion and the verifier stashed on the authorize leg. Getting either
// wrong fails every login against a real deployment, which a mocked token service would not catch.
func (suite *ESignetOIDCExecutorTestSuite) TestCallbackSendsPrivateKeyJWTAndPKCEToTokenEndpoint() {
	fake := suite.startFakeESignet()
	suite.expectFederatedAuthentication()

	_, execResp := suite.runESignetCallback(fake)
	suite.Require().Equal(providers.ExecComplete, execResp.Status)

	suite.Require().NotNil(fake.tokenForm)
	suite.Equal("authorization_code", fake.tokenForm.Get("grant_type"))
	suite.Equal("auth-code", fake.tokenForm.Get("code"))
	suite.Equal("stashed-verifier", fake.tokenForm.Get("code_verifier"))
	suite.Equal("https://localhost:8090/gate/callback", fake.tokenForm.Get("redirect_uri"))
	suite.Equal("thunderid-esignet", fake.tokenForm.Get("client_id"))
	suite.Equal("urn:ietf:params:oauth:client-assertion-type:jwt-bearer",
		fake.tokenForm.Get("client_assertion_type"))

	assertion, _, err := jwt.NewParser().ParseUnverified(
		fake.tokenForm.Get("client_assertion"), jwt.MapClaims{})
	suite.Require().NoError(err)
	suite.Equal("kid-1", assertion.Header["kid"])

	claims, ok := assertion.Claims.(jwt.MapClaims)
	suite.Require().True(ok)
	suite.Equal("thunderid-esignet", claims["iss"])
	suite.Equal("thunderid-esignet", claims["sub"])
	suite.Equal(fake.server.URL+"/token", claims["aud"])

	// Userinfo is fetched with the access token the exchange returned.
	suite.Equal("Bearer "+testESignetAccessToken, fake.userInfoAuthHeader)
}

// An ID token minted for a different client must not authenticate anyone here, even though it
// verifies against the same JWKS.
func (suite *ESignetOIDCExecutorTestSuite) TestCallbackRejectsIDTokenWithWrongAudience() {
	fake := suite.startFakeESignet()
	fake.tokenBody = suite.tokenResponseBody(suite.signIDToken(map[string]interface{}{
		"aud": []interface{}{"another-client"},
	}))

	_, execResp := suite.runESignetCallback(fake)

	suite.Equal(providers.ExecFailure, execResp.Status)
	suite.Require().NotNil(execResp.Error)
	suite.Equal(ErrESignetInvalidIDToken.Code, execResp.Error.Code)
}

// The nonce binds the ID token to this execution. Without this check a token captured from an
// earlier login could be replayed into a fresh flow.
func (suite *ESignetOIDCExecutorTestSuite) TestCallbackRejectsIDTokenWithNonceMismatch() {
	fake := suite.startFakeESignet()
	fake.tokenBody = suite.tokenResponseBody(suite.signIDToken(map[string]interface{}{
		"nonce": "nonce-from-another-execution",
	}))

	_, execResp := suite.runESignetCallback(fake)

	suite.Equal(providers.ExecFailure, execResp.Status)
	suite.Require().NotNil(execResp.Error)
	suite.Equal(ErrESignetInvalidIDToken.Code, execResp.Error.Code)
}

func (suite *ESignetOIDCExecutorTestSuite) TestCallbackRejectsIDTokenMissingSub() {
	fake := suite.startFakeESignet()
	fake.tokenBody = suite.tokenResponseBody(suite.signIDToken(map[string]interface{}{"sub": nil}))

	_, execResp := suite.runESignetCallback(fake)

	suite.Equal(providers.ExecFailure, execResp.Status)
	suite.Require().NotNil(execResp.Error)
	suite.Equal(ErrESignetInvalidIDToken.Code, execResp.Error.Code)
}

// The profile claims come from userinfo, not the ID token, so a userinfo outage has to fail the
// step rather than provision a user with nothing but a subject.
func (suite *ESignetOIDCExecutorTestSuite) TestCallbackFailsWhenUserInfoUnavailable() {
	fake := suite.startFakeESignet()
	fake.userInfoStatus = http.StatusInternalServerError
	fake.userInfoBody = "upstream failure"

	_, execResp := suite.runESignetCallback(fake)

	suite.Equal(providers.ExecFailure, execResp.Status)
	suite.Require().NotNil(execResp.Error)
	suite.Equal(ErrESignetUserInfoFailed.Code, execResp.Error.Code)
}

// The two responses are fetched separately, so their subjects are compared before either is
// trusted: a userinfo response describing a different citizen must not be attached to this login.
func (suite *ESignetOIDCExecutorTestSuite) TestCallbackRejectsUserInfoSubMismatch() {
	fake := suite.startFakeESignet()
	fake.userInfoBody = suite.signUserInfo(map[string]interface{}{"sub": "a-different-citizen"})

	_, execResp := suite.runESignetCallback(fake)

	suite.Equal(providers.ExecFailure, execResp.Status)
	suite.Require().NotNil(execResp.Error)
	suite.Equal(ErrESignetInvalidIDToken.Code, execResp.Error.Code)
}

// The provisioning flags are what let a flow continue when eSignet authenticated someone who has no
// local account yet, so each has to be set only for the flow type and node property it belongs to.
func (suite *ESignetOIDCExecutorTestSuite) TestCallbackSetsProvisioningRuntimeFlags() {
	cases := []struct {
		name         string
		flowType     providers.FlowType
		nodeProperty string
		runtimeKey   string
		expectSet    bool
	}{
		{
			name:         "authentication flow allowing no local user",
			flowType:     providers.FlowTypeAuthentication,
			nodeProperty: common.NodePropertyAllowAuthenticationWithoutLocalUser,
			runtimeKey:   common.RuntimeKeyUserEligibleForProvisioning,
			expectSet:    true,
		},
		{
			name:       "authentication flow without the property",
			flowType:   providers.FlowTypeAuthentication,
			runtimeKey: common.RuntimeKeyUserEligibleForProvisioning,
			expectSet:  false,
		},
		{
			name:         "registration flow allowing an existing user",
			flowType:     providers.FlowTypeRegistration,
			nodeProperty: common.NodePropertyAllowRegistrationWithExistingUser,
			runtimeKey:   common.RuntimeKeyAllowRegistrationWithExistingUser,
			expectSet:    true,
		},
		{
			name:       "registration flow without the property",
			flowType:   providers.FlowTypeRegistration,
			runtimeKey: common.RuntimeKeyAllowRegistrationWithExistingUser,
			expectSet:  false,
		},
	}

	for _, tc := range cases {
		suite.Run(tc.name, func() {
			suite.SetupTest()

			fake := suite.startFakeESignet()
			suite.expectFederatedAuthentication()
			suite.expectESignetConnection(fake)

			ctx := suite.callbackContext("stashed-state")
			ctx.FlowType = tc.flowType
			if tc.nodeProperty != "" {
				ctx.NodeProperties[tc.nodeProperty] = true
			}

			execResp, err := suite.executor.Execute(ctx)

			suite.Require().NoError(err)
			suite.Require().Equal(providers.ExecComplete, execResp.Status)
			if tc.expectSet {
				suite.Equal(dataValueTrue, execResp.RuntimeData[tc.runtimeKey])
			} else {
				suite.NotContains(execResp.RuntimeData, tc.runtimeKey)
			}
		})
	}
}

// The generated username has to stay within the local schema's bounds while remaining traceable to
// the eSignet subject, which is longer than the username allows.
func (suite *ESignetOIDCExecutorTestSuite) TestBuildProvisioningClaims() {
	longSub := "1234567890123456789"

	suite.Run("maps profile claims and respects the configured prefix", func() {
		claims := buildProvisioningClaims("citizen-1", map[string]interface{}{
			"name":      "Kamala Perera",
			"birthdate": "1990-04-17",
		}, "mosip-")

		suite.Equal("citizen-1", claims[esignetSubClaim])
		suite.Equal("Kamala Perera", claims["name"])
		suite.Equal("1990-04-17", claims["birthdate"])
		suite.Equal("mosip-citizen-1", claims[userAttributeUsername])
	})

	suite.Run("falls back to the default prefix when none is configured", func() {
		claims := buildProvisioningClaims("citizen-1", map[string]interface{}{}, "")

		suite.Equal(esignetDefaultUsernamePrefix+"citizen-1", claims[userAttributeUsername])
	})

	suite.Run("truncates the subject in the username but keeps it whole in the claim", func() {
		claims := buildProvisioningClaims(longSub, map[string]interface{}{}, "esignet-")

		suite.Equal("esignet-"+longSub[:12], claims[userAttributeUsername])
		suite.Equal(longSub, claims[esignetSubClaim])
	})

	suite.Run("omits profile claims the userinfo response did not carry", func() {
		claims := buildProvisioningClaims("citizen-1", map[string]interface{}{}, "esignet-")

		suite.NotContains(claims, "name")
		suite.NotContains(claims, "birthdate")
	})
}
