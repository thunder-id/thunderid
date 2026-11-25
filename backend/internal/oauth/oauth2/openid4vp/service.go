// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package openid4vp

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"time"

	authncommon "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/vc/verifier"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// OpenID4VPServiceInterface is the external contract for the OpenID4VP verifier.
type OpenID4VPServiceInterface interface {
	// Initiate creates a new presentation request.
	Initiate(ctx context.Context, verifierID string) (*Initiation, *tidcommon.ServiceError)
	// GetResult returns the current state of a request, consuming terminal states from the store.
	GetResult(ctx context.Context, state string) (*RequestState, *tidcommon.ServiceError)
	// Authenticate converts an OpenID4VP credential into an authentication result.
	Authenticate(ctx context.Context, cred *authncommon.OpenID4VPCredential) (
		*authncommon.AuthnResult, *tidcommon.ServiceError,
	)
}

var _ OpenID4VPServiceInterface = (*openid4vpService)(nil)

// openid4vpService drives the OpenID4VP verifier: issues signed requests, verifies encrypted responses.
type openid4vpService struct {
	cfg                    engineconfig.OpenID4VPConfig
	store                  openID4VPStoreInterface
	verifierConfigProvider providers.VCVerifierClientConfigProvider
	cryptoProvider         providers.RuntimeCryptoProvider
	signingKeyRef          providers.KeyRef
	signingAlg             string
	x5c                    []string
	jwtSvc                 jwt.JWTServiceInterface
	requestURL             string
	responseURL            string
	verifierRegistry       verifier.VCVerifierRegistryInterface
	issuerURL              string
	logger                 *log.Logger
}

// newOpenID4VPService creates an OpenID4VP verifier engine.
func newOpenID4VPService(
	cfg engineconfig.OpenID4VPConfig,
	store openID4VPStoreInterface,
	cryptoProvider providers.RuntimeCryptoProvider,
	signingKeyRef providers.KeyRef,
	signingAlg string,
	x5c []string,
	verifierRegistry verifier.VCVerifierRegistryInterface,
	verifierConfigProvider providers.VCVerifierClientConfigProvider,
	jwtSvc jwt.JWTServiceInterface,
	requestURL string,
	responseURL string,
	issuerURL string,
) (*openid4vpService, error) {
	if store == nil || verifierConfigProvider == nil || cryptoProvider == nil || verifierRegistry == nil {
		return nil, fmt.Errorf("%w: store, verifier registry, crypto provider and verifier config provider "+
			"are required", ErrPolicy)
	}
	if requestURL == "" || responseURL == "" {
		return nil, fmt.Errorf("%w: request_uri and response_uri base URLs are required", ErrPolicy)
	}
	return &openid4vpService{
		cfg:                    cfg,
		store:                  store,
		verifierConfigProvider: verifierConfigProvider,
		cryptoProvider:         cryptoProvider,
		signingKeyRef:          signingKeyRef,
		signingAlg:             signingAlg,
		x5c:                    x5c,
		jwtSvc:                 jwtSvc,
		requestURL:             requestURL,
		responseURL:            responseURL,
		verifierRegistry:       verifierRegistry,
		issuerURL:              issuerURL,
		logger:                 log.GetLogger().With(log.String(log.LoggerKeyComponentName, "OpenID4VPService")),
	}, nil
}

func (s *openid4vpService) Initiate(
	ctx context.Context, verifierID string,
) (*Initiation, *tidcommon.ServiceError) {
	init, err := s.initiate(ctx, verifierID)
	if err != nil {
		return nil, toServiceError(err)
	}
	return init, nil
}

// Initiate creates a fresh request for definitionID.
func (s *openid4vpService) initiate(ctx context.Context, verifierID string) (*Initiation, error) {
	verifierClientConfig, err := s.resolveVerifierClientConfig(ctx, verifierID)
	if err != nil {
		return nil, err
	}

	clientID, err := deriveClientID(verifierClientConfig.ClientIDScheme, verifierClientConfig.ClientID,
		s.x5c, s.responseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to derive client_id: %w", err)
	}
	state, err := randomToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}
	nonce, err := randomToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ephemeral key: %w", err)
	}

	expiresAt := time.Now().Add(time.Duration(s.cfg.StateTTLSeconds) * time.Second)
	rs := &RequestState{
		State:        state,
		VerifierID:   verifierID,
		Nonce:        nonce,
		EphemeralKey: key,
		ClientID:     clientID,
		RequestURI:   s.requestURI(state),
		Status:       StatusPending,
		ExpiresAt:    expiresAt,
	}
	if err := s.store.SaveRequestState(ctx, rs); err != nil {
		s.logger.Error(ctx, "Failed to store OpenID4VP request state",
			log.String("verifierClientConfigID", verifierID), log.String("error", err.Error()))
		return nil, fmt.Errorf("failed to store request state: %w", err)
	}

	requestURI := s.requestURI(state)
	return &Initiation{
		State:      state,
		ClientID:   clientID,
		RequestURI: requestURI,
		WalletURI:  WalletAuthorizationURI(clientID, requestURI),
		ExpiresAt:  expiresAt,
	}, nil
}

// GetResult returns the request state, consuming terminal states from the store (result delivered once).
func (s *openid4vpService) GetResult(ctx context.Context, state string) (*RequestState, *tidcommon.ServiceError) {
	rs, ok := s.store.GetRequestState(ctx, state)
	if !ok || rs == nil {
		return nil, &ErrorUnknownState
	}
	if time.Now().After(rs.ExpiresAt) {
		s.logger.Debug(ctx, "OpenID4VP request state expired", log.String("state", state))
		rs.Status = StatusExpired
		_ = s.store.DeleteRequestState(ctx, state)
		return rs, nil
	}
	switch rs.Status {
	case StatusCompleted:
		if s.jwtSvc == nil {
			return nil, &tidcommon.InternalServerError
		}
		claims := map[string]interface{}{
			"aud":             s.issuerURL,
			"jti":             rs.State,
			"txn":             rs.State,
			"definition_id":   rs.VerifierID,
			"subject":         rs.Result.Subject,
			"verified_claims": rs.Result.Claims,
			"verifier":        rs.ClientID,
		}
		token, _, svcErr := s.jwtSvc.GenerateJWT(ctx, rs.Result.Subject, s.issuerURL,
			int64(s.cfg.ResultTokenValiditySeconds), claims, "JWT", "")
		if svcErr != nil {
			s.logger.Error(ctx, "Failed to issue OpenID4VP result token",
				log.String("state", state), log.String("error", svcErr.Code))
			return nil, &tidcommon.InternalServerError
		}
		rs.ResultToken = token
		_ = s.store.DeleteRequestState(ctx, state)
	case StatusFailed:
		_ = s.store.DeleteRequestState(ctx, state)
	}
	return rs, nil
}

// Authenticate converts a verified presentation credential into an AuthnResult.
func (s *openid4vpService) Authenticate(_ context.Context, cred *authncommon.OpenID4VPCredential) (
	*authncommon.AuthnResult, *tidcommon.ServiceError) {
	if cred == nil {
		return nil, &tidcommon.InternalServerError
	}

	claims := make(map[string]interface{}, len(cred.Claims)+1)
	for k, v := range cred.Claims {
		claims[k] = v
	}
	if cred.Subject != "" {
		claims["sub"] = cred.Subject
	}

	// Identification keys on subject only so IdentifyEntity matches a returning
	// holder by sub rather than over all disclosed claims.
	identification := map[string]interface{}{}
	if cred.Subject != "" {
		identification["sub"] = cred.Subject
	}

	return &authncommon.AuthnResult{
		Token:               identification,
		AuthenticatedClaims: claims,
	}, nil
}

// resolveVerifierClientConfig fetches the verifier client configuration for verifierID and builds its
// engine representation.
func (s *openid4vpService) resolveVerifierClientConfig(
	ctx context.Context, verifierID string,
) (*providers.VCVerifierClientConfig, error) {
	cfg, svcErr := s.verifierConfigProvider.GetVCVerifierClientConfigByID(ctx, verifierID)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ClientErrorType {
			return nil, fmt.Errorf("%w: no verifier client configuration found for %q",
				ErrUnknownDefinition, verifierID)
		}
		return nil, fmt.Errorf("failed to resolve verifier client configuration %q", verifierID)
	}
	if cfg == nil {
		return nil, fmt.Errorf("%w: no verifier client configuration found for %q",
			ErrUnknownDefinition, verifierID)
	}
	return cfg, nil
}

// load fetches non-expired state, deleting and rejecting expired entries.
func (s *openid4vpService) load(ctx context.Context, state string) (*RequestState, error) {
	rs, ok := s.store.GetRequestState(ctx, state)
	if !ok || rs == nil {
		return nil, ErrUnknownState
	}
	if time.Now().After(rs.ExpiresAt) {
		_ = s.store.DeleteRequestState(ctx, state)
		return nil, ErrUnknownState
	}
	return rs, nil
}

// fail records a verification failure and returns the wrapped reason.
func (s *openid4vpService) fail(ctx context.Context, rs *RequestState, reason error) error {
	rs.Status = StatusFailed
	rs.FailureReason = reason.Error()
	s.logger.Debug(ctx, "OpenID4VP presentation verification failed",
		log.String("state", rs.State), log.String("reason", reason.Error()))
	if err := s.store.SaveRequestState(ctx, rs); err != nil {
		s.logger.Error(ctx, "Failed to persist OpenID4VP failure state",
			log.String("state", rs.State), log.String("error", err.Error()))
	}
	return reason
}

// randomToken returns 32 cryptographically random bytes, base64url-encoded.
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// deriveClientID builds the full client_id value from the configured scheme and
// the signing certificate. responseURI is used only for the redirect_uri scheme.
func deriveClientID(scheme string, id string, x5c []string, responseURL string) (string, error) {
	switch scheme {
	case "preregistered":
		return "preregistered:" + id, nil
	case "x509_hash":
		der, err := leafCertDER(x5c)
		if err != nil {
			return "", err
		}
		hash := sha256.Sum256(der)
		return "x509_hash:" + base64.RawURLEncoding.EncodeToString(hash[:]), nil
	case "x509_san_dns":
		der, err := leafCertDER(x5c)
		if err != nil {
			return "", err
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return "", fmt.Errorf("failed to parse signing certificate: %w", err)
		}
		if len(cert.DNSNames) == 0 {
			return "", fmt.Errorf("signing certificate has no DNS SAN entries")
		}
		return "x509_san_dns:" + cert.DNSNames[0], nil
	case "redirect_uri":
		return "redirect_uri:" + responseURL, nil
	default:
		return "", fmt.Errorf("unsupported client_id_scheme %q", scheme)
	}
}

// leafCertDER decodes the leaf certificate from a standard base64 encoded x5c chain.
func leafCertDER(x5c []string) ([]byte, error) {
	if len(x5c) == 0 {
		return nil, fmt.Errorf("signing certificate chain is empty")
	}
	der, err := base64.StdEncoding.DecodeString(x5c[0])
	if err != nil {
		return nil, fmt.Errorf("failed to decode signing certificate: %w", err)
	}
	return der, nil
}
