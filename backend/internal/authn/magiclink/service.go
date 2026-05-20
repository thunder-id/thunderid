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
	VerifyMagicLink(ctx context.Context, token string,
		subjectAttribute string) (*entityprovider.Entity, *serviceerror.ServiceError)
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
	s.logger.Debug("Generating magic link", log.MaskedString("subject", subject))

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

	verifyURL := s.buildMagicLinkURL(magicLinkURL, token, queryParams)
	s.logger.Debug("Magic link generated successfully",
		log.MaskedString("subject", subject))

	return verifyURL, nil
}

// VerifyMagicLink verifies the validity of a magic link token and retrieves the associated user information.
// Returns a user object on success or a localized service error if the token is invalid, expired, or malformed.
func (s *magicLinkAuthnService) VerifyMagicLink(_ context.Context,
	token string, subjectAttribute string) (*entityprovider.Entity, *serviceerror.ServiceError) {
	s.logger.Debug("Verifying magic link token")

	token = strings.TrimSpace(token)
	if token == "" {
		return nil, &ErrorInvalidToken
	}

	issuer := config.GetServerRuntime().Config.JWT.Issuer
	verifyErr := s.jwtService.VerifyJWT(token, tokenAudience, issuer)
	if verifyErr != nil {
		if verifyErr.Code == jwt.ErrorTokenExpired.Code {
			return nil, &ErrorExpiredToken
		}
		s.logger.Debug("Invalid magic link token", log.String("errorCode", verifyErr.Code))
		return nil, &ErrorInvalidToken
	}

	payload, decodeErr := jwt.DecodeJWTPayload(token)
	if decodeErr != nil {
		s.logger.Debug("Failed to decode magic link token payload", log.Error(decodeErr))
		return nil, &ErrorInvalidToken
	}

	subject := utils.ConvertInterfaceValueToString(payload["sub"])
	if subject == "" {
		s.logger.Debug("Subject claim not found or invalid")
		return nil, &ErrorMalformedTokenClaims
	}
	user, svcErr := s.resolveUserFromSubject(subject, strings.TrimSpace(subjectAttribute))
	if svcErr != nil {
		return nil, svcErr
	}

	s.logger.Debug("Magic link verification successful", log.String("userId", user.ID))
	return user, nil
}

// resolveUserFromSubject resolves the token subject either as a user ID or as a configured destination attribute.
func (s *magicLinkAuthnService) resolveUserFromSubject(
	subject string, subjectAttribute string) (*entityprovider.Entity, *serviceerror.ServiceError) {
	if subjectAttribute == "" {
		user, upErr := s.entityProvider.GetEntity(subject)
		if upErr != nil {
			return nil, s.handleEntityProviderError(upErr)
		}
		return user, nil
	}

	userID, upErr := s.entityProvider.IdentifyEntity(map[string]interface{}{subjectAttribute: subject})
	if upErr != nil {
		return nil, s.handleEntityProviderError(upErr)
	}
	if userID == nil || *userID == "" {
		return nil, &common.ErrorUserNotFound
	}

	user, upErr := s.entityProvider.GetEntity(*userID)
	if upErr != nil {
		return nil, s.handleEntityProviderError(upErr)
	}
	return user, nil
}

// buildMagicLinkURL constructs a magic link URL by appending query parameters to a base URL or default configuration.
func (s *magicLinkAuthnService) buildMagicLinkURL(magicLinkURL string, token string,
	queryParams map[string]string) string {
	var u *url.URL
	var err error

	if magicLinkURL != "" && strings.TrimSpace(magicLinkURL) != "" {
		u, err = url.Parse(strings.TrimSpace(magicLinkURL))
		if err != nil {
			s.logger.Debug("Failed to parse custom magic link URL; falling back to default configuration",
				log.Error(err))
		}
	}

	if u == nil {
		u = config.GetServerRuntime().GateClientLoginURL
	}

	q := u.Query()
	for key, value := range queryParams {
		q.Set(key, value)
	}
	q.Set("token", token)
	u.RawQuery = q.Encode()

	return u.String()
}

// handleEntityProviderError maps entity provider errors to appropriate service errors with localization support.
func (s *magicLinkAuthnService) handleEntityProviderError(
	upErr *entityprovider.EntityProviderError,
) *serviceerror.ServiceError {
	if upErr.Code == entityprovider.ErrorCodeEntityNotFound {
		return &common.ErrorUserNotFound
	}
	if upErr.Code == entityprovider.ErrorCodeSystemError {
		return &serviceerror.InternalServerError
	}
	s.logger.Debug("User provider returned an error while resolving user",
		log.String("description", upErr.Description))
	return &ErrorClientErrorWhileResolvingUser
}

// getMetadata returns the metadata information for the magic link authenticator.
func (s *magicLinkAuthnService) getMetadata() common.AuthenticatorMeta {
	return common.AuthenticatorMeta{
		Name:    common.AuthenticatorMagicLink,
		Factors: []common.AuthenticationFactor{common.FactorPossession},
	}
}
