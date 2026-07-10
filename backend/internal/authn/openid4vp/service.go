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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	authncommon "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/vc/presentation"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// OpenID4VPServiceInterface is the external contract for the OpenID4VP verifier.
type OpenID4VPServiceInterface interface {
	// Initiate creates a new presentation request.
	Initiate(ctx context.Context, definitionID string) (*Initiation, *tidcommon.ServiceError)
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
	cfg            serviceConfig
	store          openID4VPStoreInterface
	defStore       presentation.PresentationDefinitionServiceInterface
	clientID       string
	cryptoProvider providers.RuntimeCryptoProvider
	signingKeyRef  providers.KeyRef
	signingAlg     string
	x5c            []string
	jwtSvc         jwt.JWTServiceInterface
	issuerURL      string
	trust          *trustAnchorStore
	logger         *log.Logger
}

// newOpenID4VPService creates an OpenID4VP verifier engine.
func newOpenID4VPService(
	cfg serviceConfig, store openID4VPStoreInterface, clientID string,
	cryptoProvider providers.RuntimeCryptoProvider, signingKeyRef providers.KeyRef, signingAlg string, x5c []string,
	trust *trustAnchorStore, defStore presentation.PresentationDefinitionServiceInterface,
	jwtSvc jwt.JWTServiceInterface, issuerURL string,
) (*openid4vpService, error) {
	if store == nil || clientID == "" || cryptoProvider == nil || defStore == nil {
		return nil, fmt.Errorf("%w: store, client id, crypto provider and definition store are required", ErrPolicy)
	}
	if cfg.RequestURIBase == "" || cfg.ResponseURIBase == "" {
		return nil, fmt.Errorf("%w: request_uri and response_uri base URLs are required", ErrPolicy)
	}
	return &openid4vpService{
		cfg:            cfg,
		store:          store,
		defStore:       defStore,
		clientID:       clientID,
		cryptoProvider: cryptoProvider,
		signingKeyRef:  signingKeyRef,
		signingAlg:     signingAlg,
		x5c:            x5c,
		jwtSvc:         jwtSvc,
		issuerURL:      issuerURL,
		trust:          trust,
		logger:         log.GetLogger().With(log.String(log.LoggerKeyComponentName, "OpenID4VPService")),
	}, nil
}

// Initiate creates a fresh request for definitionID.
func (s *openid4vpService) Initiate(
	ctx context.Context, definitionID string,
) (*Initiation, *tidcommon.ServiceError) {
	init, err := s.initiate(ctx, definitionID)
	if err != nil {
		return nil, toServiceError(err)
	}
	return init, nil
}

func (s *openid4vpService) initiate(ctx context.Context, definitionID string) (*Initiation, error) {
	def, err := s.resolveDefinition(ctx, definitionID)
	if err != nil {
		return nil, err
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

	expiresAt := time.Now().Add(s.cfg.TTL)
	rs := &RequestState{
		State:        state,
		DefinitionID: def.ID,
		Nonce:        nonce,
		EphemeralKey: key,
		ClientID:     s.clientID,
		RequestURI:   s.requestURI(state),
		Status:       StatusPending,
		ExpiresAt:    expiresAt,
	}
	if err := s.store.SaveRequestState(ctx, rs); err != nil {
		s.logger.Error(ctx, "Failed to store OpenID4VP request state",
			log.String("definitionID", definitionID), log.String("error", err.Error()))
		return nil, fmt.Errorf("failed to store request state: %w", err)
	}

	requestURI := s.requestURI(state)
	return &Initiation{
		State:      state,
		ClientID:   s.clientID,
		RequestURI: requestURI,
		WalletURI:  WalletAuthorizationURI(s.clientID, requestURI),
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
			"definition_id":   rs.DefinitionID,
			"subject":         rs.Result.Subject,
			"verified_claims": rs.Result.Claims,
			"verifier":        s.clientID,
		}
		token, _, svcErr := s.jwtSvc.GenerateJWT(ctx, rs.Result.Subject, s.issuerURL,
			int64(s.cfg.ResultTokenValidity.Seconds()), claims, "JWT", "")
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

// resolveDefinition fetches the presentation definition by handle and builds its engine representation.
func (s *openid4vpService) resolveDefinition(
	ctx context.Context, definitionID string,
) (*presentationDefinition, error) {
	dto, svcErr := s.defStore.GetPresentationDefinitionByHandle(ctx, definitionID)
	if svcErr != nil {
		if svcErr.Code == presentation.ErrorDefinitionNotFound.Code {
			return nil, fmt.Errorf("%w: no presentation definition registered for %q",
				ErrUnknownDefinition, definitionID)
		}
		return nil, fmt.Errorf("failed to resolve presentation definition %q", definitionID)
	}
	return dtoToDefinition(*dto, s.clientID, s.cfg.EnforceKeyBinding), nil
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
