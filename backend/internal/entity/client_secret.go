// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"encoding/json"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	oauthutils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// ClientSecretRegeneration is a client secret just generated and stored. It is the only time the secret
// can be read, since it is hashed on write.
type ClientSecretRegeneration struct {
	ClientSecret string
	// Revoked reports that the system flows ran around the write and revoked what the old secret issued,
	// up to the write. It is false where system flows are off, as on a control plane, and when the
	// revocation after the write failed.
	Revoked bool
	// RevocationError is set when the secret is stored but revoking the tokens issued while it was written
	// failed. Regenerating again revokes them.
	RevocationError *tidcommon.ServiceError
}

// ClientSecretHolderInterface is implemented by the service that owns the entities of one category that
// authenticate with a client secret. It decides whether the caller may regenerate an entity's secret: the
// entity exists and is of its category, the caller may change it, and its client uses a secret.
type ClientSecretHolderInterface interface {
	ValidateClientSecretRegeneration(ctx context.Context, entityID string) *tidcommon.ServiceError
}

// RegisterClientSecretHolder names the service that decides whether the client secret of an entity of the
// category may be regenerated. It is called while the server starts, before any request is served.
func (s *entityService) RegisterClientSecretHolder(category providers.EntityCategory,
	holder ClientSecretHolderInterface) {
	s.secretHolders[category] = holder
}

// validateClientSecretRegeneration asks the service that owns entities of the category whether the caller
// may regenerate the entity's client secret.
func (s *entityService) validateClientSecretRegeneration(ctx context.Context, category providers.EntityCategory,
	entityID string) *tidcommon.ServiceError {
	holder, ok := s.secretHolders[category]
	if !ok {
		s.logger.Error(ctx, "No service decides on client secret regeneration for the entity category",
			log.String("category", string(category)))
		return &tidcommon.InternalServerError
	}
	return holder.ValidateClientSecretRegeneration(ctx, entityID)
}

// RegenerateClientSecret replaces the entity's client secret with one generated here and returns it. The
// system flows run around the write: the pre flow revokes what the old secret issued, and the post flow
// moves the cutoff past the write. A refusal, or a failure before the write, stores nothing. A failure of
// the post flow is returned in the result with the secret, since the secret is stored by then and nobody
// could read it again.
func (s *entityService) RegenerateClientSecret(ctx context.Context, category providers.EntityCategory,
	entityID string) (*ClientSecretRegeneration, *tidcommon.ServiceError) {
	if svcErr := s.validateClientSecretRegeneration(ctx, category, entityID); svcErr != nil {
		return nil, svcErr
	}

	secret, err := oauthutils.GenerateOAuth2ClientSecret()
	if err != nil {
		s.logger.Error(ctx, "Failed to generate a client secret", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	credentials, err := json.Marshal(map[string]string{authnprovidercm.CredentialTypeClientSecret: secret})
	if err != nil {
		s.logger.Error(ctx, "Failed to build the client secret credential", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	svcErr, postErr := s.RotateClientSecret(ctx, entityID, func() *tidcommon.ServiceError {
		if err := s.UpdateSystemCredentials(ctx, entityID, credentials); err != nil {
			s.logger.Error(ctx, "Failed to store the regenerated client secret", log.Error(err),
				log.MaskedString("entityID", entityID))
			return &tidcommon.InternalServerError
		}
		return nil
	})
	if svcErr != nil {
		return nil, svcErr
	}
	if postErr != nil {
		s.logger.Error(ctx, "The client secret was regenerated, but revoking the tokens issued while it was "+
			"replaced failed; it is returned, and a token issued in that window can stay valid until it expires",
			log.MaskedString("entityID", entityID), log.String("code", postErr.Code))
	}
	revoked := s.systemFlows != nil && postErr == nil &&
		config.GetServerRuntime().Config.OAuth.TokenRevocation.IsEnabled()
	return &ClientSecretRegeneration{ClientSecret: secret, Revoked: revoked, RevocationError: postErr}, nil
}

// RotateClientSecret runs write, which replaces the entity's client secret, between the system flows that
// revoke what the entity's OAuth client issued before it. The pre flow runs first, so it resolves the client as
// it was before anything changed. A failure before or in write is returned as svcErr, and nothing past it
// runs. A failure of the post flow is returned as revocationErr, since the secret is stored by then.
func (s *entityService) RotateClientSecret(ctx context.Context, entityID string,
	write func() *tidcommon.ServiceError) (svcErr, revocationErr *tidcommon.ServiceError) {
	inputs := map[string]string{systemFlowInputEntity: entityID}
	shared, svcErr := s.runSystemFlow(ctx, systemFlowSecretRotatePre, inputs, nil)
	if svcErr != nil {
		return svcErr, nil
	}
	if writeErr := write(); writeErr != nil {
		return writeErr, nil
	}
	_, revocationErr = s.runSystemFlow(ctx, systemFlowRevocationPost, inputs, shared)
	return nil, revocationErr
}
