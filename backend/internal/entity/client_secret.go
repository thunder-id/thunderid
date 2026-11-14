// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"encoding/json"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	flowcommon "github.com/thunder-id/thunderid/internal/flow/common"
	oauthutils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// The system flows run around a client secret regeneration, and the inputs naming the entity they act on.
const (
	// SystemFlowSecretRotatePre revokes what the old secret issued, up to a cutoff, before it is replaced.
	SystemFlowSecretRotatePre = "system-entity-secret-rotate-pre"
	// systemFlowRevocationPost moves the cutoff past the write, so a token issued while it ran is revoked too.
	systemFlowRevocationPost = "system-revocation-post"
	// SystemFlowInputEntity names the entity whose client secret is replaced.
	SystemFlowInputEntity = "targetEntityId"
	// SystemFlowInputEntityCategory names the entity's category, which selects the service that decides
	// whether its secret may be replaced.
	SystemFlowInputEntityCategory = "targetEntityCategory"
)

// ClientSecretProfile is what the service that owns an entity reports for the artifacts its OAuth client has
// issued. The revocation around a regeneration is bounded by it.
type ClientSecretProfile struct {
	// ClientID is the OAuth client the secret authenticates.
	ClientID string
	// MaxLifetimeSeconds is the longest an artifact issued to the client can stay valid. Zero leaves the
	// revocation on its default.
	MaxLifetimeSeconds int64
}

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
	ValidateClientSecretRegeneration(ctx context.Context, entityID string) (
		*ClientSecretProfile, *tidcommon.ServiceError)
}

// SystemFlowProviderInterface runs a system flow to completion. The flow engine implements it.
type SystemFlowProviderInterface interface {
	ExecuteSystemFlow(ctx context.Context, sysCtx *flowcommon.SystemFlowContext) (*flowcommon.SystemFlowResult,
		*tidcommon.ServiceError)
}

// SetSystemFlowProvider gives the entity service the engine that runs the system flows around a client
// secret regeneration. It is bound after the service, once the engine exists, and only on a server that
// issues tokens.
func (s *entityService) SetSystemFlowProvider(provider SystemFlowProviderInterface) {
	s.systemFlows = provider
}

// RegisterClientSecretHolder names the service that decides whether the client secret of an entity of the
// category may be regenerated. It is called while the server starts, before any request is served.
func (s *entityService) RegisterClientSecretHolder(category providers.EntityCategory,
	holder ClientSecretHolderInterface) {
	s.secretHolders[category] = holder
}

// ValidateClientSecretRegeneration asks the service that owns entities of the category whether the caller
// may regenerate the entity's client secret, and returns the profile the revocation around it is bounded by.
func (s *entityService) ValidateClientSecretRegeneration(ctx context.Context, category providers.EntityCategory,
	entityID string) (*ClientSecretProfile, *tidcommon.ServiceError) {
	holder, ok := s.secretHolders[category]
	if !ok {
		s.logger.Error(ctx, "No service decides on client secret regeneration for the entity category",
			log.String("category", string(category)))
		return nil, &tidcommon.InternalServerError
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
	if _, svcErr := s.ValidateClientSecretRegeneration(ctx, category, entityID); svcErr != nil {
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

	inputs := map[string]string{SystemFlowInputEntity: entityID, SystemFlowInputEntityCategory: string(category)}
	shared, svcErr := s.runSystemFlow(ctx, SystemFlowSecretRotatePre, inputs, nil)
	if svcErr != nil {
		return nil, svcErr
	}
	if err := s.UpdateSystemCredentials(ctx, entityID, credentials); err != nil {
		s.logger.Error(ctx, "Failed to store the regenerated client secret", log.Error(err),
			log.MaskedString("entityID", entityID))
		return nil, &tidcommon.InternalServerError
	}
	_, postErr := s.runSystemFlow(ctx, systemFlowRevocationPost, inputs, shared)
	if postErr != nil {
		s.logger.Error(ctx, "The client secret was regenerated, but revoking the tokens issued while it was "+
			"replaced failed; it is returned, and a token issued in that window can stay valid until it expires",
			log.MaskedString("entityID", entityID), log.String("code", postErr.Code))
	}
	return &ClientSecretRegeneration{ClientSecret: secret, Revoked: s.systemFlows != nil && postErr == nil,
		RevocationError: postErr}, nil
}

// runSystemFlow runs a system flow with the given inputs, starting from what an earlier run shared, and
// returns what this run shared. Where system flows are off it runs nothing.
func (s *entityService) runSystemFlow(ctx context.Context, handle string,
	inputs, runtimeData map[string]string) (map[string]string, *tidcommon.ServiceError) {
	if s.systemFlows == nil {
		return nil, nil
	}
	result, svcErr := s.systemFlows.ExecuteSystemFlow(ctx, &flowcommon.SystemFlowContext{
		Handle: handle, Inputs: inputs, RuntimeData: runtimeData})
	if svcErr != nil {
		return nil, svcErr
	}
	return result.RuntimeData, nil
}
