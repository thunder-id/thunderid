// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package extensionauth implements the authentication service for an identity an organization's own
// service verified through an authentication endpoint connection.
package extensionauth

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/authn/common"
	authnoauth "github.com/thunder-id/thunderid/internal/authn/oauth"
	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const (
	loggerComponentName = "ExtensionAuthnService"
	defaultTimeoutMS    = 10000
)

// ExtensionAuthnServiceInterface defines the contract for authenticating an identity an organization's service
// verified. It also resolves the connection's endpoint, so the extension authentication executor reads
// everything about the connection through one service.
type ExtensionAuthnServiceInterface interface {
	httpconnection.HTTPConnectionProvider
	Authenticate(ctx context.Context, credential *common.ExtensionVerifiedCredential) (
		*common.AuthnResult, *tidcommon.ServiceError)
}

// extensionAuthnService is the default implementation of ExtensionAuthnServiceInterface.
type extensionAuthnService struct {
	idpService providers.IDPProvider
	internal   authnoauth.OAuthAuthnCoreServiceInterface
	logger     *log.Logger
}

var _ ExtensionAuthnServiceInterface = (*extensionAuthnService)(nil)

// newExtensionAuthnService creates the extension authentication service. The OAuth service builds the result,
// so attribute mapping and the account-linking filters are applied as they are for every federated connection.
func newExtensionAuthnService(idpService providers.IDPProvider,
	internal authnoauth.OAuthAuthnCoreServiceInterface) ExtensionAuthnServiceInterface {
	return &extensionAuthnService{
		idpService: idpService,
		internal:   internal,
		logger:     log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)),
	}
}

// GetResolvedConnection returns the connection's endpoint ready to call: its URL, its timeout and the
// authorization header of its credential. It answers with the HTTP connection's not found error, since the
// executor treats a missing connection the same way for either kind.
func (s *extensionAuthnService) GetResolvedConnection(ctx context.Context, id string) (
	*httpconnection.ResolvedHTTPConnection, *tidcommon.ServiceError) {
	connection, svcErr := s.getConnection(ctx, id)
	if svcErr != nil {
		return nil, svcErr
	}

	endpoint := strings.TrimSpace(idp.GetPropertyValue(connection.Properties, idp.PropEndpointURL))
	if endpoint == "" {
		s.logger.Error(ctx, "Authentication endpoint connection has no URL", log.String(log.LoggerKeyIDPID, id))
		return nil, &tidcommon.InternalServerError
	}
	timeoutMS := defaultTimeoutMS
	if raw := idp.GetPropertyValue(connection.Properties, idp.PropTimeoutMS); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			s.logger.Error(ctx, "Authentication endpoint connection has an unusable timeout",
				log.String(log.LoggerKeyIDPID, id))
			return nil, &tidcommon.InternalServerError
		}
		timeoutMS = parsed
	}

	headers := make(http.Header)
	username := idp.GetPropertyValue(connection.Properties, idp.PropAuthUsername)
	password := idp.GetPropertyValue(connection.Properties, idp.PropAuthPassword)
	if username != "" || password != "" {
		request := &http.Request{Header: headers}
		request.SetBasicAuth(username, password)
	}
	return &httpconnection.ResolvedHTTPConnection{
		URL:     endpoint,
		Timeout: time.Duration(timeoutMS) * time.Millisecond,
		Headers: headers,
	}, nil
}

// Authenticate builds the authentication result for the identity the service verified. The local user is
// resolved later through the connection's account-linking rules, as it is for any federated sign-in.
func (s *extensionAuthnService) Authenticate(ctx context.Context, credential *common.ExtensionVerifiedCredential) (
	*common.AuthnResult, *tidcommon.ServiceError) {
	if credential == nil || strings.TrimSpace(credential.ConnectionID) == "" {
		return nil, &authnoauth.ErrorEmptyIdpID
	}
	if strings.TrimSpace(credential.Subject) == "" {
		return nil, &common.ErrorSubClaimNotFound
	}
	if _, svcErr := s.getConnection(ctx, credential.ConnectionID); svcErr != nil {
		return nil, svcErr
	}
	s.logger.Debug(ctx, "Building the authentication result for the verified identity",
		log.String(log.LoggerKeyIDPID, credential.ConnectionID), log.MaskedString("subject", credential.Subject))
	return s.internal.BuildFederatedAuthResult(ctx, credential.ConnectionID, credential.Subject, credential.Claims)
}

// getConnection loads the authentication endpoint connection. A connection of another type is reported as
// not found, so a federated connection cannot be called as an endpoint.
func (s *extensionAuthnService) getConnection(ctx context.Context, id string) (
	*providers.IDPDTO, *tidcommon.ServiceError) {
	connection, svcErr := s.idpService.GetIdentityProvider(ctx, id)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ClientErrorType {
			return nil, &httpconnection.ErrorNotFound
		}
		s.logger.Error(ctx, "Failed to retrieve the authentication endpoint connection",
			log.String(log.LoggerKeyIDPID, id), log.String("errorCode", svcErr.Code))
		return nil, &tidcommon.InternalServerError
	}
	if connection == nil || connection.Type != providers.IDPTypeExtensionAuthentication {
		return nil, &httpconnection.ErrorNotFound
	}
	return connection, nil
}
