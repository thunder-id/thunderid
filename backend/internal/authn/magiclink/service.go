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

// Package magiclink implements the magic link authentication service.
package magiclink

import (
	"context"
	"net/url"
	"strings"

	"github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// userAttributeUserID is the user attribute key for user ID in authentication results.
const userAttributeUserID = "userID"

// MagicLinkAuthnServiceInterface defines the interface for magic link authentication operations.
type MagicLinkAuthnServiceInterface interface {
	GenerateMagicLink(
		ctx context.Context,
		subject string,
		expirySeconds int64,
		queryParams map[string]string,
		additionalClaims map[string]interface{},
		magicLinkURL string,
	) (string, *serviceerror.ServiceError)
	Authenticate(ctx context.Context, token string,
		subjectAttribute string) (*common.AuthnResult, *serviceerror.ServiceError)
}

// magicLinkAuthnService is the default implementation of MagicLinkAuthnServiceInterface.
type magicLinkAuthnService struct {
	jwtService     jwt.JWTServiceInterface
	entityProvider entityprovider.EntityProviderInterface
	logger         *log.Logger
}

// newMagicLinkAuthnService creates a new instance of magicLinkAuthnService with the provided dependencies.
func newMagicLinkAuthnService(
	jwtSvc jwt.JWTServiceInterface,
	entityProvider entityprovider.EntityProviderInterface,
) MagicLinkAuthnServiceInterface {
	service := &magicLinkAuthnService{
		jwtService:     jwtSvc,
		entityProvider: entityProvider,
		logger:         log.GetLogger().With(log.String(log.LoggerKeyComponentName, "MagicLinkAuthnService")),
	}
	common.RegisterAuthenticator(service.getMetadata())

	return service
}

// GenerateMagicLink generates a magic link URL for the specified subject.
func (s *magicLinkAuthnService) GenerateMagicLink(ctx context.Context,
	subject string,
	expirySeconds int64,
	queryParams map[string]string,
	additionalClaims map[string]interface{},
	magicLinkURL string) (string, *serviceerror.ServiceError) {
	s.logger.Debug(ctx, "Generating magic link", log.MaskedString("subject", subject))

	if subject == "" {
		return "", &ErrorTokenGenerationFailed
	}

	issuer := config.GetServerRuntime().Config.JWT.Issuer
	expiry := int64(DefaultExpirySeconds)
	if expirySeconds > 0 {
		expiry = expirySeconds
	}

	claims := make(map[string]interface{})
	for k, v := range additionalClaims {
		claims[k] = v
	}
	claims["aud"] = tokenAudience

	token, _, jwtErr := s.jwtService.GenerateJWT(
		ctx,
		subject,
		issuer,
		expiry,
		claims,
		jwt.TokenTypeJWT,
		"",
	)
	if jwtErr != nil {
		return "", &ErrorTokenGenerationFailed
	}

	verifyURL := s.buildMagicLinkURL(ctx, magicLinkURL, token, queryParams)
	s.logger.Debug(ctx, "Magic link generated successfully",
		log.MaskedString("subject", subject))

	return verifyURL, nil
}

// Authenticate verifies a magic link token and returns an authentication result.
// A missing local user is NOT an error — the result carries VerifiedIdentifiers instead,
// allowing callers to handle registration flows.
func (s *magicLinkAuthnService) Authenticate(ctx context.Context,
	token string, subjectAttribute string) (*common.AuthnResult, *serviceerror.ServiceError) {
	s.logger.Debug(ctx, "Authenticating with magic link token")

	token = strings.TrimSpace(token)
	if token == "" {
		return nil, &ErrorInvalidToken
	}

	if svcErr := s.verifyToken(ctx, token); svcErr != nil {
		return nil, svcErr
	}

	subject, svcErr := s.extractSubject(ctx, token)
	if svcErr != nil {
		return nil, svcErr
	}

	subjectAttribute = strings.TrimSpace(subjectAttribute)
	if subjectAttribute == "" {
		subjectAttribute = userAttributeUserID
	}

	s.logger.Debug(ctx, "Magic link authentication successful", log.String("subjectAttribute", subject))

	return &common.AuthnResult{
		Token:               map[string]interface{}{subjectAttribute: subject},
		AuthenticatedClaims: map[string]interface{}{subjectAttribute: subject},
	}, nil
}

// verifyToken checks the validity of the provided JWT token and returns service errors for invalid or expired tokens.
func (s *magicLinkAuthnService) verifyToken(ctx context.Context, token string) *serviceerror.ServiceError {
	issuer := config.GetServerRuntime().Config.JWT.Issuer
	verifyErr := s.jwtService.VerifyJWT(ctx, token, tokenAudience, issuer)
	if verifyErr != nil {
		if verifyErr.Code == jwt.ErrorTokenExpired.Code {
			return &ErrorExpiredToken
		}
		s.logger.Debug(ctx, "Invalid magic link token", log.String("errorCode", verifyErr.Code))
		return &ErrorInvalidToken
	}
	return nil
}

// extractSubject retrieves the subject claim from the JWT token payload and returns it as a string.
// along with any service errors encountered during decoding or extraction.
func (s *magicLinkAuthnService) extractSubject(ctx context.Context, token string) (string, *serviceerror.ServiceError) {
	payload, decodeErr := jwt.DecodeJWTPayload(token)
	if decodeErr != nil {
		s.logger.Debug(ctx, "Failed to decode magic link token payload", log.Error(decodeErr))
		return "", &ErrorInvalidToken
	}

	subject := utils.ConvertInterfaceValueToString(payload["sub"])
	if subject == "" {
		s.logger.Debug(ctx, "Subject claim not found or invalid")
		return "", &ErrorMalformedTokenClaims
	}

	return subject, nil
}

// buildMagicLinkURL constructs a magic link URL by appending query parameters to a base URL or default configuration.
func (s *magicLinkAuthnService) buildMagicLinkURL(ctx context.Context, magicLinkURL string, token string,
	queryParams map[string]string) string {
	var u *url.URL
	var err error

	if magicLinkURL != "" && strings.TrimSpace(magicLinkURL) != "" {
		u, err = url.Parse(strings.TrimSpace(magicLinkURL))
		if err != nil {
			s.logger.Debug(ctx,
				"Failed to parse custom magic link URL; falling back to default configuration",
				log.Error(err))
		}
	}

	if u == nil {
		u = config.GetServerRuntime().GateClientCallbackURL
	}

	q := u.Query()
	for key, value := range queryParams {
		q.Set(key, value)
	}
	q.Set("token", token)
	u.RawQuery = q.Encode()

	return u.String()
}

// getMetadata returns the metadata information for the magic link authenticator.
func (s *magicLinkAuthnService) getMetadata() common.AuthenticatorMeta {
	return common.AuthenticatorMeta{
		Name:    common.AuthenticatorMagicLink,
		Factors: []common.AuthenticationFactor{common.FactorPossession},
	}
}
