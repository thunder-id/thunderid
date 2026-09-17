// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/idp"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/log"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	esignetOIDCLoggerComponentName = "ESignetOIDCExecutor"

	// esignetHTTPTimeout bounds calls to the eSignet token/userinfo/JWKS endpoints.
	esignetHTTPTimeout = 15 * time.Second
	// esignetClientAssertionValidity is the lifetime of the signed client_assertion JWT.
	esignetClientAssertionValidity = 5 * time.Minute
	// esignetDefaultUsernamePrefix prefixes the generated username when none is configured.
	esignetDefaultUsernamePrefix = "esignet-"
	// esignetSubClaim is the runtime-data / schema attribute key holding the eSignet subject.
	esignetSubClaim = "esignet_sub"

	// maxESignetResponseBytes bounds how much of a token/userinfo response body is read into
	// memory, mirroring the cap used elsewhere for external IdP responses (see internal/authn/oauth/utils.go).
	maxESignetResponseBytes = 64 * 1024
	// maxJWKSResponseBytes bounds the JWKS response body, matching internal/oauth/oauth2/jwksresolver.
	maxJWKSResponseBytes = 1 << 20
	// maxLoggedResponseBytes bounds how much of an external error response body is copied into logs.
	maxLoggedResponseBytes = 4096

	// Runtime keys namespaced to this executor; deleted before the flow completes.
	runtimeKeyESignetNonce        = "esignet.nonce"
	runtimeKeyESignetCodeVerifier = "esignet.codeVerifier"
	runtimeKeyESignetState        = "esignet.state"
)

// esignetAllowedJWSAlgorithms are the signature algorithms accepted for eSignet's ID tokens and
// signed userinfo responses. Both are RSA over the same JWKS keys; eSignet advertises them as
// id_token_signing_alg_values_supported and picks per deployment, so both must be accepted. The
// list stays explicit rather than deferring to the token's own alg header, which is what stops a
// token from selecting "none" or an HMAC scheme and being verified against a public key.
var esignetAllowedJWSAlgorithms = []string{"RS256", "PS256"}

// propertyKeyESignetIdpID is the flow node property naming the eSignet connection to use.
const propertyKeyESignetIdpID = "idpId"

// esignetOIDCConfig holds the configuration resolved from the referenced eSignet connection.
type esignetOIDCConfig struct {
	authorizeEndpoint string
	tokenEndpoint     string
	userInfoEndpoint  string
	jwksURI           string
	clientID          string
	// signingKey is held still encrypted and decrypted only where the client assertion is signed,
	// so the PEM private key never enters memory on the authorize leg, which does not sign.
	signingKey     *cmodels.Property
	signingKeyID   string
	redirectURI    string
	acrValues      string
	scopes         string
	usernamePrefix string
}

// esignetOIDCExecutor federates authentication to MOSIP eSignet: it drives the authorization-code
// + PKCE redirect leg, exchanges the code using a `private_key_jwt` client assertion, verifies the
// signed ID token and JWS userinfo response against eSignet's JWKS, and hands the already-verified
// claims to the authn provider for local user resolution/JIT provisioning.
//
// Configuration comes from the eSignet connection (an identity provider record) named by the
// node's idpId property, so credentials and endpoints are managed once under Connections rather
// than restated on every flow node.
type esignetOIDCExecutor struct {
	providers.Executor
	idpService    idp.IDPServiceInterface
	authnProvider providers.AuthnProviderManager
	httpClient    *http.Client
	logger        *log.Logger
}

var _ providers.Executor = (*esignetOIDCExecutor)(nil)

// newESignetOIDCExecutor creates a new instance of the eSignet OIDC executor.
func newESignetOIDCExecutor(
	flowFactory core.FlowFactoryInterface,
	idpService idp.IDPServiceInterface,
	authnProvider providers.AuthnProviderManager,
) providers.Executor {
	base := flowFactory.CreateExecutor(
		ExecutorNameESignetOIDC,
		providers.ExecutorTypeAuthentication,
		[]providers.Input{
			{Identifier: userInputCode, Type: "string", Required: true},
		},
		[]providers.Input{},
		&providers.ExecutorMeta{
			SupportedProperties: []providers.ExecutorSupportedProperties{
				{Property: propertyKeyESignetIdpID, IsRequired: true},
				{Property: common.NodePropertyAllowAuthenticationWithoutLocalUser},
				{Property: common.NodePropertyAllowRegistrationWithExistingUser},
			},
		},
	)
	return &esignetOIDCExecutor{
		Executor:      base,
		idpService:    idpService,
		authnProvider: authnProvider,
		httpClient:    &http.Client{Timeout: esignetHTTPTimeout},
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, esignetOIDCLoggerComponentName),
			log.String(log.LoggerKeyExecutorName, ExecutorNameESignetOIDC)),
	}
}

// Execute drives the two-pass authorize/token-exchange flow.
func (e *esignetOIDCExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing eSignet OIDC executor")

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
		AuthUser:       ctx.AuthUser,
	}

	cfg, cfgErr := e.readESignetConfig(ctx)
	if cfgErr != nil {
		logger.Error(ctx.Context, "Invalid eSignet executor configuration", log.String("error", cfgErr.Error()))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrESignetConfigInvalid
		return execResp, nil
	}

	if !e.HasRequiredInputs(ctx, execResp) {
		logger.Debug(ctx.Context, "Authorization code not yet available, redirecting to eSignet")
		if err := e.buildAuthorizeRedirect(cfg, execResp); err != nil {
			logger.Error(ctx.Context, "Failed to build eSignet authorize URL", log.String("error", err.Error()))
			return nil, err
		}
		return execResp, nil
	}

	if err := e.processCallback(ctx, cfg, execResp); err != nil {
		logger.Error(ctx.Context, "Failed to process eSignet callback", log.String("error", err.Error()))
		return nil, err
	}
	return execResp, nil
}

// buildAuthorizeRedirect generates the PKCE pair, nonce and state, stashes them in runtime data,
// and builds the eSignet authorize URL redirect.
func (e *esignetOIDCExecutor) buildAuthorizeRedirect(
	cfg esignetOIDCConfig, execResp *providers.ExecutorResponse,
) error {
	nonce, err := randomURLSafeString(32)
	if err != nil {
		return fmt.Errorf("failed to generate nonce: %w", err)
	}
	state, err := randomURLSafeString(32)
	if err != nil {
		return fmt.Errorf("failed to generate state: %w", err)
	}
	codeVerifier, err := randomURLSafeString(48)
	if err != nil {
		return fmt.Errorf("failed to generate PKCE code verifier: %w", err)
	}
	codeChallenge := pkceS256Challenge(codeVerifier)

	scopes := cfg.scopes
	if scopes == "" {
		scopes = "openid profile"
	}

	query := url.Values{}
	query.Set(oauth2const.RequestParamClientID, cfg.clientID)
	query.Set(oauth2const.RequestParamRedirectURI, cfg.redirectURI)
	query.Set(oauth2const.RequestParamResponseType, "code")
	query.Set(oauth2const.RequestParamScope, scopes)
	query.Set(oauth2const.RequestParamNonce, nonce)
	query.Set(oauth2const.RequestParamState, state)
	query.Set(oauth2const.RequestParamCodeChallenge, codeChallenge)
	query.Set(oauth2const.RequestParamCodeChallengeMethod, "S256")
	if cfg.acrValues != "" {
		query.Set(oauth2const.RequestParamAcrValues, cfg.acrValues)
	}

	separator := "?"
	if strings.Contains(cfg.authorizeEndpoint, "?") {
		separator = "&"
	}
	execResp.Status = providers.ExecExternalRedirection
	execResp.RedirectURL = cfg.authorizeEndpoint + separator + query.Encode()

	execResp.RuntimeData[runtimeKeyESignetNonce] = nonce
	execResp.RuntimeData[runtimeKeyESignetCodeVerifier] = codeVerifier
	execResp.RuntimeData[runtimeKeyESignetState] = state
	return nil
}

// processCallback exchanges the authorization code, verifies the ID token and userinfo response,
// and hands the verified claims off to the authn provider.
func (e *esignetOIDCExecutor) processCallback(
	ctx *providers.NodeContext, cfg esignetOIDCConfig, execResp *providers.ExecutorResponse,
) error {
	code := ctx.UserInputs[userInputCode]
	if code == "" {
		execResp.AuthUser = providers.AuthUser{}
		return nil
	}

	nonce := ctx.RuntimeData[runtimeKeyESignetNonce]
	codeVerifier := ctx.RuntimeData[runtimeKeyESignetCodeVerifier]
	expectedState := ctx.RuntimeData[runtimeKeyESignetState]
	delete(ctx.RuntimeData, runtimeKeyESignetNonce)
	delete(ctx.RuntimeData, runtimeKeyESignetCodeVerifier)
	delete(ctx.RuntimeData, runtimeKeyESignetState)

	// State is a second, independent CSRF check on the callback: the PKCE verifier binds the code
	// and the nonce binds the ID token to this execution, both through runtime data, so a code
	// from another authorization request already fails. It is validated only when the client sends
	// it back, matching the OAuth executor, since a client handling CSRF protection itself may
	// omit it.
	if returnedState, ok := ctx.UserInputs[userInputState]; ok && returnedState != "" {
		if returnedState != expectedState {
			e.logger.Debug(ctx.Context, "eSignet state mismatch")
			execResp.Status = providers.ExecFailure
			execResp.Error = &ErrInvalidOAuthState
			return nil
		}
	}

	tokenResp, err := e.exchangeCode(ctx.Context, cfg, code, codeVerifier)
	if err != nil {
		e.logger.Debug(ctx.Context, "eSignet token exchange failed", log.String("error", err.Error()))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrESignetTokenExchangeFailed
		return nil
	}

	idClaims, err := e.verifyCompactJWS(ctx.Context, cfg.jwksURI, tokenResp.IDToken)
	if err != nil {
		e.logger.Debug(ctx.Context, "eSignet ID token verification failed", log.String("error", err.Error()))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrESignetInvalidIDToken
		return nil
	}
	if !audienceContains(idClaims, cfg.clientID) {
		e.logger.Debug(ctx.Context, "eSignet ID token audience mismatch")
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrESignetInvalidIDToken
		return nil
	}
	if idNonce, _ := idClaims["nonce"].(string); idNonce == "" || idNonce != nonce {
		e.logger.Debug(ctx.Context, "eSignet ID token nonce mismatch")
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrESignetInvalidIDToken
		return nil
	}
	sub, _ := idClaims["sub"].(string)
	if sub == "" {
		e.logger.Debug(ctx.Context, "eSignet ID token missing sub claim")
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrESignetInvalidIDToken
		return nil
	}

	userInfoClaims, err := e.fetchUserInfo(ctx.Context, cfg.jwksURI, cfg.userInfoEndpoint, tokenResp.AccessToken)
	if err != nil {
		e.logger.Debug(ctx.Context, "eSignet userinfo retrieval failed", log.String("error", err.Error()))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrESignetUserInfoFailed
		return nil
	}
	if userInfoSub, _ := userInfoClaims["sub"].(string); userInfoSub == "" || userInfoSub != sub {
		e.logger.Debug(ctx.Context, "eSignet userinfo sub does not match ID token sub")
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrESignetInvalidIDToken
		return nil
	}

	claims := buildProvisioningClaims(sub, userInfoClaims, cfg.usernamePrefix)

	credentials := map[string]interface{}{
		authnprovidercm.CredentialTypeFederatedClaims: &authncm.FederatedClaimsCredential{
			Subject:        sub,
			Claims:         claims,
			MatchAttribute: esignetSubClaim,
		},
	}
	authUser, authenticatedClaims, svcErr := e.authnProvider.AuthenticateUser(
		ctx.Context, nil, credentials, nil, nil, execResp.AuthUser)
	if svcErr != nil {
		e.logger.Error(ctx.Context, "eSignet federated authentication failed", log.String("errorCode", svcErr.Code),
			log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
		return errors.New("eSignet federated authentication failed")
	}
	execResp.AuthUser = authUser

	for key, value := range authenticatedClaims {
		execResp.RuntimeData[key] = systemutils.ConvertInterfaceValueToString(value)
	}

	setFederatedEntityState(ctx.Context, execResp, e.authnProvider)

	if ctx.FlowType == providers.FlowTypeAuthentication && isAuthenticationWithoutLocalUserAllowed(ctx) {
		execResp.RuntimeData[common.RuntimeKeyUserEligibleForProvisioning] = dataValueTrue
	}
	if ctx.FlowType == providers.FlowTypeRegistration && isRegistrationWithExistingUserAllowed(ctx) {
		execResp.RuntimeData[common.RuntimeKeyAllowRegistrationWithExistingUser] = dataValueTrue
	}

	execResp.Status = providers.ExecComplete
	return nil
}

// audienceContains reports whether the token's aud claim names the given client. RFC 7519 allows
// aud to be either a single string or an array of them, and eSignet uses the array form, so this
// defers to MapClaims.GetAudience rather than asserting a string.
func audienceContains(claims jwt.MapClaims, clientID string) bool {
	audience, err := claims.GetAudience()
	if err != nil {
		return false
	}
	return slices.Contains(audience, clientID)
}

// buildProvisioningClaims maps verified eSignet userinfo claims onto the target user-type schema
// attribute names that ProvisioningExecutor reads from RuntimeData.
func buildProvisioningClaims(
	sub string, userInfoClaims map[string]interface{}, usernamePrefix string,
) map[string]interface{} {
	claims := map[string]interface{}{
		esignetSubClaim: sub,
	}
	if name, ok := userInfoClaims["name"]; ok {
		claims["name"] = systemutils.ConvertInterfaceValueToString(name)
	}
	if birthdate, ok := userInfoClaims["birthdate"]; ok {
		claims["birthdate"] = systemutils.ConvertInterfaceValueToString(birthdate)
	}

	prefix := usernamePrefix
	if prefix == "" {
		prefix = esignetDefaultUsernamePrefix
	}
	suffix := sub
	if len(suffix) > 12 {
		suffix = suffix[:12]
	}
	claims[userAttributeUsername] = prefix + suffix

	return claims
}

// readESignetConfig resolves the eSignet connection referenced by the node's idpId property and
// maps its properties onto the executor's configuration.
func (e *esignetOIDCExecutor) readESignetConfig(ctx *providers.NodeContext) (esignetOIDCConfig, error) {
	idpID, err := e.GetIdpID(ctx)
	if err != nil {
		return esignetOIDCConfig{}, err
	}

	connection, svcErr := e.idpService.GetIdentityProvider(ctx.Context, idpID)
	if svcErr != nil {
		return esignetOIDCConfig{}, fmt.Errorf("failed to resolve eSignet connection %q: %s",
			idpID, svcErr.Code)
	}
	if connection == nil {
		return esignetOIDCConfig{}, fmt.Errorf("eSignet connection %q not found", idpID)
	}
	if connection.Type != providers.IDPTypeESignet {
		return esignetOIDCConfig{}, fmt.Errorf("connection %q is of type %q, not %q",
			idpID, connection.Type, providers.IDPTypeESignet)
	}

	cfg := esignetOIDCConfig{
		authorizeEndpoint: idp.GetPropertyValue(connection.Properties, idp.PropAuthorizationEndpoint),
		tokenEndpoint:     idp.GetPropertyValue(connection.Properties, idp.PropTokenEndpoint),
		userInfoEndpoint:  idp.GetPropertyValue(connection.Properties, idp.PropUserInfoEndpoint),
		jwksURI:           idp.GetPropertyValue(connection.Properties, idp.PropJwksEndpoint),
		clientID:          idp.GetPropertyValue(connection.Properties, idp.PropClientID),
		signingKey:        findProperty(connection.Properties, idp.PropSigningKey),
		signingKeyID:      idp.GetPropertyValue(connection.Properties, idp.PropSigningKeyID),
		redirectURI:       idp.GetPropertyValue(connection.Properties, idp.PropRedirectURI),
		acrValues:         idp.GetPropertyValue(connection.Properties, idp.PropACRValues),
		scopes:            authorizeScopes(idp.GetPropertyValue(connection.Properties, idp.PropScopes)),
		usernamePrefix:    idp.GetPropertyValue(connection.Properties, idp.PropUsernamePrefix),
	}

	required := map[string]string{
		idp.PropAuthorizationEndpoint: cfg.authorizeEndpoint,
		idp.PropTokenEndpoint:         cfg.tokenEndpoint,
		idp.PropUserInfoEndpoint:      cfg.userInfoEndpoint,
		idp.PropJwksEndpoint:          cfg.jwksURI,
		idp.PropClientID:              cfg.clientID,
		idp.PropSigningKeyID:          cfg.signingKeyID,
		idp.PropRedirectURI:           cfg.redirectURI,
	}
	for name, value := range required {
		if value == "" {
			return esignetOIDCConfig{}, fmt.Errorf("eSignet connection %q is missing property %q", idpID, name)
		}
	}

	// The signing key is checked for presence only: reading its value would decrypt it, and the
	// authorize leg never signs anything. An empty stored value is caught where it is decrypted.
	if cfg.signingKey == nil {
		return esignetOIDCConfig{}, fmt.Errorf("eSignet connection %q is missing property %q",
			idpID, idp.PropSigningKey)
	}
	return cfg, nil
}

// findProperty returns the named property, or nil when the connection does not carry it. A secret
// property is returned still encrypted, so holding it costs nothing until it is read.
func findProperty(properties []cmodels.Property, name string) *cmodels.Property {
	for i := range properties {
		if properties[i].GetName() == name {
			return &properties[i]
		}
	}
	return nil
}

// GetIdpID retrieves the identity provider ID from the node properties.
func (e *esignetOIDCExecutor) GetIdpID(ctx *providers.NodeContext) (string, error) {
	if len(ctx.NodeProperties) > 0 {
		if val, ok := ctx.NodeProperties[propertyKeyESignetIdpID]; ok {
			if idpID, valid := val.(string); valid && idpID != "" {
				return idpID, nil
			}
		}
	}
	return "", errors.New("idpId is not configured in node properties")
}

// authorizeScopes converts the connection's comma-separated scopes property into the
// space-separated form the authorization request uses.
func authorizeScopes(stored string) string {
	parts := strings.Split(stored, ",")
	scopes := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			scopes = append(scopes, trimmed)
		}
	}
	return strings.Join(scopes, " ")
}

// truncateForLog caps a response body at maxLoggedResponseBytes before it is written to logs.
func truncateForLog(body []byte) string {
	if len(body) > maxLoggedResponseBytes {
		return string(body[:maxLoggedResponseBytes])
	}
	return string(body)
}

// randomURLSafeString returns a base64url-encoded (no padding) random string of n raw bytes.
func randomURLSafeString(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// pkceS256Challenge computes the PKCE S256 code_challenge for the given code_verifier.
func pkceS256Challenge(codeVerifier string) string {
	sum := sha256.Sum256([]byte(codeVerifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// esignetTokenResponse mirrors the fields of eSignet's token endpoint response that this
// executor needs.
type esignetTokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
}

// exchangeCode builds the private_key_jwt client assertion and exchanges the authorization code
// for tokens at eSignet's token endpoint.
func (e *esignetOIDCExecutor) exchangeCode(
	ctx context.Context, cfg esignetOIDCConfig, code, codeVerifier string,
) (*esignetTokenResponse, error) {
	// Decrypt the signing key here, at the one point that signs with it, rather than carrying the
	// PEM around for the whole execution. Go strings cannot be zeroed, so the shorter its reach
	// the less of the process image it lingers in.
	keyPEM, err := cfg.signingKey.GetValue()
	if err != nil {
		return nil, fmt.Errorf("failed to read the eSignet signing key: %w", err)
	}
	if keyPEM == "" {
		return nil, errors.New("the eSignet signing key is empty")
	}

	assertion, err := buildClientAssertion(cfg.clientID, cfg.tokenEndpoint, keyPEM, cfg.signingKeyID)
	if err != nil {
		return nil, fmt.Errorf("failed to build client assertion: %w", err)
	}

	form := url.Values{}
	form.Set(oauth2const.RequestParamGrantType, "authorization_code")
	form.Set(oauth2const.RequestParamCode, code)
	form.Set(oauth2const.RequestParamRedirectURI, cfg.redirectURI)
	form.Set(oauth2const.RequestParamClientID, cfg.clientID)
	form.Set(oauth2const.RequestParamClientAssertionType, "urn:ietf:params:oauth:client-assertion-type:jwt-bearer")
	form.Set(oauth2const.RequestParamClientAssertion, assertion)
	form.Set(oauth2const.RequestParamCodeVerifier, codeVerifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxESignetResponseBytes))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		e.logger.Debug(ctx, "eSignet token endpoint returned an error response",
			log.Int("statusCode", resp.StatusCode), log.String("response", truncateForLog(body)))
		return nil, fmt.Errorf("token endpoint returned status %d", resp.StatusCode)
	}

	var tokenResp esignetTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}
	if tokenResp.IDToken == "" {
		return nil, errors.New("token response has no id_token")
	}
	return &tokenResp, nil
}

// buildClientAssertion signs a private_key_jwt client assertion per RFC 7523 / OIDC Core,
// using the PEM-encoded RSA private key (PKCS1 or PKCS8) held by the connection.
func buildClientAssertion(clientID, tokenEndpoint, keyPEM, kid string) (string, error) {
	privateKey, err := parseRSAPrivateKey(keyPEM)
	if err != nil {
		return "", err
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iss": clientID,
		"sub": clientID,
		"aud": tokenEndpoint,
		"jti": uuid.NewString(),
		"iat": now.Unix(),
		"exp": now.Add(esignetClientAssertionValidity).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid

	return token.SignedString(privateKey)
}

// parseRSAPrivateKey parses a PEM-encoded RSA private key (PKCS1 or PKCS8). The key arrives
// decrypted from the connection's secret property, so it is never read from the filesystem and
// no caller-supplied path reaches the runtime. The key is stored on a single line, with its
// newlines written as the two-character sequence \n, so that configuration export can carry it in
// an environment file; the newlines are restored here, where the key is actually parsed.
func parseRSAPrivateKey(keyPEM string) (*rsa.PrivateKey, error) {
	keyPEM = strings.ReplaceAll(keyPEM, `\n`, "\n")
	block, _ := pem.Decode([]byte(strings.TrimSpace(keyPEM)))
	if block == nil {
		return nil, errors.New("signing key is not valid PEM")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse signing key: %w", err)
	}
	rsaKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("signing key is not an RSA private key")
	}
	return rsaKey, nil
}

// jsonWebKey is the subset of RFC 7517 JWK fields needed to reconstruct an RSA public key.
type jsonWebKey struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// fetchJWKS retrieves and parses the JWK set from jwksURI.
func (e *esignetOIDCExecutor) fetchJWKS(ctx context.Context, jwksURI string) (map[string]jsonWebKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURI, nil)
	if err != nil {
		return nil, err
	}
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxJWKSResponseBytes))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jwks endpoint returned status %d", resp.StatusCode)
	}

	var jwks struct {
		Keys []jsonWebKey `json:"keys"`
	}
	if err := json.Unmarshal(body, &jwks); err != nil {
		return nil, fmt.Errorf("failed to parse jwks: %w", err)
	}

	byKid := make(map[string]jsonWebKey, len(jwks.Keys))
	for _, key := range jwks.Keys {
		if key.Kty == "RSA" && key.Kid != "" {
			byKid[key.Kid] = key
		}
	}
	return byKid, nil
}

// rsaPublicKeyFromJWK reconstructs an *rsa.PublicKey from a JWK's base64url-encoded n/e fields.
func rsaPublicKeyFromJWK(key jsonWebKey) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		return nil, fmt.Errorf("invalid jwk modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		return nil, fmt.Errorf("invalid jwk exponent: %w", err)
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(new(big.Int).SetBytes(eBytes).Int64()),
	}, nil
}

// verifyCompactJWS verifies a compact-serialized JWS (an ID token or userinfo response) against
// the JWK set published at jwksURI and returns its claims. The accepted algorithms are pinned to
// esignetAllowedJWSAlgorithms so a token cannot dictate its own verification scheme.
func (e *esignetOIDCExecutor) verifyCompactJWS(
	ctx context.Context, jwksURI, compact string,
) (jwt.MapClaims, error) {
	if compact == "" {
		return nil, errors.New("empty JWS")
	}

	keys, err := e.fetchJWKS(ctx, jwksURI)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch jwks: %w", err)
	}

	claims := jwt.MapClaims{}
	_, err = jwt.ParseWithClaims(compact, claims, func(token *jwt.Token) (interface{}, error) {
		kid, _ := token.Header["kid"].(string)
		key, ok := keys[kid]
		if !ok {
			return nil, fmt.Errorf("no matching jwk for kid %q", kid)
		}
		return rsaPublicKeyFromJWK(key)
	}, jwt.WithValidMethods(esignetAllowedJWSAlgorithms))
	if err != nil {
		return nil, fmt.Errorf("jws verification failed: %w", err)
	}
	return claims, nil
}

// fetchUserInfo retrieves the userinfo response (a compact JWS per eSignet's
// userinfo_response_type=JWS configuration) and verifies it against the JWKS.
func (e *esignetOIDCExecutor) fetchUserInfo(
	ctx context.Context, jwksURI, userInfoEndpoint, accessToken string,
) (jwt.MapClaims, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userInfoEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxESignetResponseBytes))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo endpoint returned status %d", resp.StatusCode)
	}

	return e.verifyCompactJWS(ctx, jwksURI, strings.TrimSpace(string(body)))
}
