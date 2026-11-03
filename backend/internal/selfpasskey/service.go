// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package selfpasskey lets a signed-in user add, list and remove their own passkeys.
package selfpasskey

import (
	"context"
	"errors"
	"net/url"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/authn/passkey"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// selfPasskeyServiceInterface manages the passkeys of the signed-in user.
type selfPasskeyServiceInterface interface {
	StartRegistration(ctx context.Context, userID, clientID, relyingPartyID string) (
		*passkey.PasskeyRegistrationStartData, *tidcommon.ServiceError)
	FinishRegistration(ctx context.Context, userID, clientID string, req *registrationFinishRequest) (
		*passkey.PasskeyInfo, *tidcommon.ServiceError)
	ListPasskeys(ctx context.Context, userID string) ([]passkey.PasskeyInfo, *tidcommon.ServiceError)
	DeletePasskey(ctx context.Context, userID, passkeyID string) *tidcommon.ServiceError
}

// appPasskeyConfig holds the passkey settings that apply to the caller's app.
type appPasskeyConfig struct {
	origins        []string
	relyingPartyID string
}

// selfPasskeyService is the default implementation of selfPasskeyServiceInterface.
type selfPasskeyService struct {
	passkeyService       passkey.PasskeyServiceInterface
	inboundClientService inboundclient.InboundClientServiceInterface
	logger               *log.Logger
}

// newSelfPasskeyService creates a new self-service passkey service.
func newSelfPasskeyService(
	passkeyService passkey.PasskeyServiceInterface,
	inboundClientService inboundclient.InboundClientServiceInterface,
) selfPasskeyServiceInterface {
	return &selfPasskeyService{
		passkeyService:       passkeyService,
		inboundClientService: inboundClientService,
		logger:               log.GetLogger().With(log.String(log.LoggerKeyComponentName, "SelfPasskeyService")),
	}
}

// StartRegistration starts adding a passkey for the user, for a relying party the token's app allows.
// Without a relying party ID, the app's passkey relying party ID is used.
func (s *selfPasskeyService) StartRegistration(ctx context.Context, userID, clientID, relyingPartyID string) (
	*passkey.PasskeyRegistrationStartData, *tidcommon.ServiceError) {
	cfg, svcErr := s.passkeyConfig(ctx, clientID)
	if svcErr != nil {
		return nil, svcErr
	}

	relyingPartyID = strings.TrimSpace(relyingPartyID)
	if relyingPartyID == "" {
		relyingPartyID = cfg.relyingPartyID
	}
	if relyingPartyID == "" {
		return nil, &passkey.ErrorEmptyRelyingPartyID
	}
	if relyingPartyID != cfg.relyingPartyID && !isRelyingPartyAllowed(relyingPartyID, cfg.origins) {
		return nil, &ErrorRelyingPartyNotAllowed
	}

	return s.passkeyService.StartRegistration(ctx, &passkey.PasskeyRegistrationStartRequest{
		UserID:         userID,
		RelyingPartyID: relyingPartyID,
		AllowedOrigins: cfg.origins,
	})
}

// FinishRegistration verifies and saves the passkey the user's device created.
func (s *selfPasskeyService) FinishRegistration(ctx context.Context, userID, clientID string,
	req *registrationFinishRequest) (*passkey.PasskeyInfo, *tidcommon.ServiceError) {
	cfg, svcErr := s.passkeyConfig(ctx, clientID)
	if svcErr != nil {
		return nil, svcErr
	}

	name := strings.TrimSpace(req.Name)
	if _, svcErr := s.passkeyService.FinishRegistration(ctx, &passkey.PasskeyRegistrationFinishRequest{
		CredentialID:      req.PublicKeyCredential.ID,
		CredentialType:    req.PublicKeyCredential.Type,
		ClientDataJSON:    req.PublicKeyCredential.Response.ClientDataJSON,
		AttestationObject: req.PublicKeyCredential.Response.AttestationObject,
		SessionToken:      req.SessionToken,
		AllowedOrigins:    cfg.origins,
		UserID:            userID,
		Name:              name,
	}); svcErr != nil {
		return nil, svcErr
	}

	return &passkey.PasskeyInfo{ID: req.PublicKeyCredential.ID, Name: name}, nil
}

// ListPasskeys returns the user's passkeys.
func (s *selfPasskeyService) ListPasskeys(ctx context.Context, userID string) (
	[]passkey.PasskeyInfo, *tidcommon.ServiceError) {
	return s.passkeyService.ListPasskeys(ctx, userID)
}

// DeletePasskey removes one of the user's passkeys.
func (s *selfPasskeyService) DeletePasskey(ctx context.Context, userID, passkeyID string) *tidcommon.ServiceError {
	return s.passkeyService.DeletePasskey(ctx, userID, passkeyID)
}

// passkeyConfig returns the passkey settings of the app the token was issued to. The origins fall
// back to the server's passkey origins when the app sets none.
func (s *selfPasskeyService) passkeyConfig(ctx context.Context, clientID string) (
	*appPasskeyConfig, *tidcommon.ServiceError) {
	cfg := &appPasskeyConfig{}
	if clientID != "" {
		client, err := s.inboundClientService.GetOAuthClientByClientID(ctx, clientID)
		if err != nil {
			s.logger.Error(ctx, "Failed to resolve the token's OAuth client", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		if client != nil {
			app, err := s.inboundClientService.GetInboundClientByEntityID(ctx, client.ID)
			if err != nil && !errors.Is(err, inboundclient.ErrInboundClientNotFound) {
				s.logger.Error(ctx, "Failed to load the token's application", log.Error(err))
				return nil, &tidcommon.InternalServerError
			}
			if app != nil {
				cfg.origins = app.PasskeyAllowedOrigins
				cfg.relyingPartyID = app.PasskeyRelyingPartyID
			}
		}
	}
	if len(cfg.origins) == 0 {
		cfg.origins = config.GetServerRuntime().Config.Passkey.AllowedOrigins
	}
	return cfg, nil
}

// isRelyingPartyAllowed reports whether relyingPartyID is the host, or a parent domain of the host,
// of one of the web origins. Origins without a host, such as Android app origins, never match.
func isRelyingPartyAllowed(relyingPartyID string, origins []string) bool {
	for _, origin := range origins {
		u, err := url.Parse(origin)
		if err != nil {
			continue
		}
		host := u.Hostname()
		if host == "" {
			continue
		}
		if host == relyingPartyID || strings.HasSuffix(host, "."+relyingPartyID) {
			return true
		}
	}
	return false
}
