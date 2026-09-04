// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package inboundclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/cert"
	layoutmgt "github.com/thunder-id/thunderid/internal/design/layout/mgt"
	thememgt "github.com/thunder-id/thunderid/internal/design/theme/mgt"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	entitytypemodel "github.com/thunder-id/thunderid/internal/entitytype/model"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	oauthutils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// InboundClientServiceInterface is the public API of the inbound client subsystem.
type InboundClientServiceInterface interface {
	// CreateInboundClient validates and persists a new inbound auth profile, certificates, and OAuth config.
	CreateInboundClient(ctx context.Context, client *inboundmodel.InboundClient,
		oauthProfile *providers.OAuthProfile, hasClientSecret bool) error
	// GetInboundClientByEntityID returns the inbound client for the given entity.
	GetInboundClientByEntityID(ctx context.Context, entityID string) (*inboundmodel.InboundClient, error)
	// GetInboundClientList returns all inbound clients.
	GetInboundClientList(ctx context.Context) ([]inboundmodel.InboundClient, error)
	// GetEntityIDsByReference returns paginated entity IDs of inbound clients referencing the resource
	// identified by (refType, refID). Unknown reference types resolve to no usages.
	GetEntityIDsByReference(ctx context.Context, refType, refID string, limit, offset int) ([]string, int, error)
	// UpdateInboundClient validates and persists updates to an inbound client, certificates, and OAuth config.
	UpdateInboundClient(ctx context.Context, client *inboundmodel.InboundClient,
		oauthProfile *providers.OAuthProfile, hasClientSecret bool, oauthClientID string) error
	// DeleteInboundClient removes the inbound client, OAuth profile, and certificates for the given entity.
	DeleteInboundClient(ctx context.Context, entityID string) error
	// Validate resolves flow defaults and validates FK constraints and OAuth profile without persisting.
	Validate(ctx context.Context, client *inboundmodel.InboundClient,
		oauthProfile *providers.OAuthProfile, hasClientSecret bool) error
	// RevalidateFKs re-runs FK validation for the inbound client identified by entityID. Used after
	// a referenced resource (e.g. a flow) is updated to detect newly-inconsistent references.
	RevalidateFKs(ctx context.Context, entityID string) error
	// ResolveInboundAuthProfileHandles resolves flow handle fields in-place to their IDs.
	// Only fields with an empty ID but a non-empty handle are resolved.
	ResolveInboundAuthProfileHandles(ctx context.Context, profile *providers.InboundAuthProfile) error

	// GetOAuthProfileByEntityID returns the stored OAuth profile for the given entity.
	GetOAuthProfileByEntityID(ctx context.Context, entityID string) (*providers.OAuthProfile, error)
	// GetOAuthClientByClientID resolves a full OAuthClient by its public client_id.
	GetOAuthClientByClientID(ctx context.Context, clientID string) (*providers.OAuthClient, error)

	// GetInboundClientAttributes returns the configured user attributes for a single inbound client.
	// A missing inbound client is treated as one with no configured attributes.
	GetInboundClientAttributes(ctx context.Context, inboundClientID string) (
		*inboundmodel.InboundClientAttributes, error)
	// ListInboundClientAttributes returns the configured user attributes for all inbound clients.
	ListInboundClientAttributes(ctx context.Context) ([]inboundmodel.InboundClientAttributes, error)

	// IsDeclarative reports whether the entity's inbound profile was loaded from a declarative resource file.
	IsDeclarative(ctx context.Context, entityID string) bool
	// LoadDeclarativeResources loads inbound client profiles from YAML resource files.
	LoadDeclarativeResources(ctx context.Context, cfg inboundmodel.DeclarativeLoaderConfig) error

	// GetCertificate retrieves the certificate for the given reference type and ID.
	GetCertificate(ctx context.Context, refType cert.CertificateReferenceType, refID string) (
		*inboundmodel.Certificate, *CertOperationError)
}

type inboundClientService struct {
	store          inboundClientStoreInterface
	transactioner  providers.Transactioner
	certService    cert.CertificateServiceInterface
	entityProvider entityprovider.EntityProviderInterface
	themeMgt       thememgt.ThemeMgtServiceInterface
	layoutMgt      layoutmgt.LayoutMgtServiceInterface
	flowMgt        flowmgt.FlowMgtServiceInterface
	entityType     entitytype.EntityTypeServiceInterface
	cryptoProvider providers.RuntimeCryptoProvider
	jweService     jwe.JWEServiceInterface
	logger         *log.Logger
}

// newInboundClientService creates and returns an inboundClientService with all dependencies wired.
func newInboundClientService(store inboundClientStoreInterface, transactioner providers.Transactioner,
	certService cert.CertificateServiceInterface,
	entityProvider entityprovider.EntityProviderInterface,
	themeMgt thememgt.ThemeMgtServiceInterface,
	layoutMgt layoutmgt.LayoutMgtServiceInterface,
	flowMgt flowmgt.FlowMgtServiceInterface,
	entityType entitytype.EntityTypeServiceInterface,
	cryptoProvider providers.RuntimeCryptoProvider,
	jweService jwe.JWEServiceInterface,
) InboundClientServiceInterface {
	return &inboundClientService{
		store:          store,
		transactioner:  transactioner,
		certService:    certService,
		entityProvider: entityProvider,
		themeMgt:       themeMgt,
		layoutMgt:      layoutMgt,
		flowMgt:        flowMgt,
		entityType:     entityType,
		cryptoProvider: cryptoProvider,
		jweService:     jweService,
		logger:         log.GetLogger().With(log.String(log.LoggerKeyComponentName, "InboundClientService")),
	}
}

// CreateInboundClient validates and persists a new inbound auth profile, certificates, and OAuth config.
func (s *inboundClientService) CreateInboundClient(ctx context.Context, client *inboundmodel.InboundClient,
	oauthProfile *providers.OAuthProfile, hasClientSecret bool) error {
	if client == nil {
		return fmt.Errorf("inbound client is required")
	}
	if client.ID != "" && s.store.IsDeclarative(ctx, client.ID) {
		return ErrCannotModifyDeclarative
	}
	if err := s.resolveFlowDefaults(ctx, client); err != nil {
		return err
	}
	if err := s.reconcileReferencedFlows(ctx, client); err != nil {
		return err
	}
	if fkErr := s.validateFKs(ctx, client); fkErr != nil {
		return fkErr
	}
	validAttrs, attrErr := s.resolveValidUserAttributes(ctx, client.AllowedUserTypes)
	if attrErr != nil {
		return attrErr
	}
	if err := validateUserAttributes(validAttrs, client.Assertion, oauthProfile); err != nil {
		return err
	}
	if oauthProfile != nil {
		if vErr := validateOAuthProfile(
			ctx, oauthProfile, hasClientSecret, s.cryptoProvider, s.jweService); vErr != nil {
			return vErr
		}
	}
	if err := s.validateSubjectAttributeMapping(
		ctx, client.SubjectAttribute, client.AllowedUserTypes); err != nil {
		return err
	}
	// Must run before applyInboundDefaults: UserInfo inherits the ID token list there.
	seeded := seedScopeClaims(oauthProfile)
	pruneScopeClaims(oauthProfile, scopeClaimPruneSet(seeded, client.AllowedUserTypes, validAttrs))
	seedIDTokenUserAttributes(oauthProfile, validAttrs)
	applyInboundDefaults(client, oauthProfile)
	oauthClientID := s.resolveClientID(ctx, client.ID)
	if err := validateOAuthCertificateClientID(oauthProfile, oauthClientID); err != nil {
		return err
	}
	return s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if err := s.store.CreateInboundClient(txCtx, *client); err != nil {
			return err
		}
		if oauthProfile != nil {
			if oauthProfile.Certificate != nil && oauthClientID != "" {
				if _, vErr, opErr := s.createCertificate(
					txCtx, oauthClientID, oauthProfile.Certificate,
				); vErr != nil {
					return vErr
				} else if opErr != nil {
					return opErr
				}
			}
			if err := s.store.CreateOAuthProfile(txCtx, client.ID, oauthProfile); err != nil {
				return err
			}
		}
		return nil
	})
}

// GetInboundClientByEntityID returns the inbound client for the given entity.
func (s *inboundClientService) GetInboundClientByEntityID(
	ctx context.Context, entityID string,
) (*inboundmodel.InboundClient, error) {
	return s.store.GetInboundClientByEntityID(ctx, entityID)
}

// GetInboundClientList returns all inbound clients.
func (s *inboundClientService) GetInboundClientList(ctx context.Context) ([]inboundmodel.InboundClient, error) {
	return s.store.GetInboundClientList(ctx, serverconst.MaxCompositeStoreRecords)
}

// GetEntityIDsByReference returns paginated entity IDs of inbound clients referencing the resource
// identified by (refType, refID).
func (s *inboundClientService) GetEntityIDsByReference(
	ctx context.Context, refType, refID string, limit, offset int) ([]string, int, error) {
	if limit < 0 {
		return nil, 0, fmt.Errorf("invalid limit: must be non-negative, got %d", limit)
	}
	if offset < 0 {
		return nil, 0, fmt.Errorf("invalid offset: must be non-negative, got %d", offset)
	}
	return s.store.GetEntityIDsByReference(ctx, refType, refID, limit, offset)
}

// UpdateInboundClient validates and persists updates to an inbound client, certificates, and OAuth config.
func (s *inboundClientService) UpdateInboundClient(ctx context.Context, client *inboundmodel.InboundClient,
	oauthProfile *providers.OAuthProfile, hasClientSecret bool, oauthClientID string) error {
	if client == nil {
		return fmt.Errorf("inbound client is required")
	}
	if s.store.IsDeclarative(ctx, client.ID) {
		return ErrCannotModifyDeclarative
	}
	if err := s.resolveFlowDefaults(ctx, client); err != nil {
		return err
	}
	if err := s.reconcileReferencedFlows(ctx, client); err != nil {
		return err
	}
	if fkErr := s.validateFKs(ctx, client); fkErr != nil {
		return fkErr
	}
	// Stripping leaves only attributes the validator accepts, so no separate validation is needed here.
	if err := s.stripUndeclaredUserAttributes(
		ctx, client.AllowedUserTypes, client.Assertion, oauthProfile); err != nil {
		return err
	}
	if oauthProfile != nil {
		if vErr := validateOAuthProfile(
			ctx, oauthProfile, hasClientSecret, s.cryptoProvider, s.jweService); vErr != nil {
			return vErr
		}
	}
	if err := s.validateSubjectAttributeMapping(
		ctx, client.SubjectAttribute, client.AllowedUserTypes); err != nil {
		return err
	}
	applyInboundDefaults(client, oauthProfile)
	// Capture existing OAuth client_id before the caller updates entity system attributes.
	oldOAuthClientID := s.resolveClientID(ctx, client.ID)
	if err := validateOAuthCertificateClientID(oauthProfile, oauthClientID); err != nil {
		return err
	}
	return s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if err := s.store.UpdateInboundClient(txCtx, *client); err != nil {
			return err
		}
		// Clean up the previous OAuth-app cert when the client_id changed or OAuth was removed.
		if oldOAuthClientID != "" && oldOAuthClientID != oauthClientID {
			if opErr := s.deleteCertificate(txCtx, oldOAuthClientID); opErr != nil {
				if opErr.Underlying == nil || opErr.Underlying.Code != cert.ErrorCertificateNotFound.Code {
					return opErr
				}
			}
		}
		if oauthClientID != "" {
			var oauthCert *inboundmodel.Certificate
			if oauthProfile != nil {
				oauthCert = oauthProfile.Certificate
			}
			if _, vErr, opErr := s.syncCertificate(
				txCtx, oauthClientID, oauthCert,
			); vErr != nil {
				return vErr
			} else if opErr != nil {
				return opErr
			}
		}
		return s.syncOAuthProfile(txCtx, client.ID, oauthProfile)
	})
}

// Validate resolves flow defaults and validates FK constraints and OAuth profile without persisting.
func (s *inboundClientService) Validate(ctx context.Context, client *inboundmodel.InboundClient,
	oauthProfile *providers.OAuthProfile, hasClientSecret bool) error {
	if client == nil {
		return nil
	}
	if err := s.resolveFlowDefaults(ctx, client); err != nil {
		return err
	}
	if fkErr := s.validateFKs(ctx, client); fkErr != nil {
		return fkErr
	}
	if err := s.validateUserAttributesAgainstAllowedTypes(
		ctx, client.AllowedUserTypes, client.Assertion, oauthProfile); err != nil {
		return err
	}
	if oauthProfile != nil {
		if vErr := validateOAuthProfile(
			ctx, oauthProfile, hasClientSecret, s.cryptoProvider, s.jweService); vErr != nil {
			return vErr
		}
	}
	if err := s.validateSubjectAttributeMapping(
		ctx, client.SubjectAttribute, client.AllowedUserTypes); err != nil {
		return err
	}
	return nil
}

// RevalidateFKs re-runs FK validation for the inbound client identified by entityID. Used by the
// resource-dependency registry to catch newly-inconsistent references after a referenced resource
// (e.g. a flow) is updated. A missing inbound client is treated as no-op — the reference is
// stale and will be resolved when the entity itself is updated or removed.
func (s *inboundClientService) RevalidateFKs(ctx context.Context, entityID string) error {
	client, err := s.GetInboundClientByEntityID(ctx, entityID)
	if err != nil {
		if errors.Is(err, ErrInboundClientNotFound) {
			return nil
		}
		return err
	}

	return s.validateFKs(ctx, client)
}

func validateOAuthCertificateClientID(oauthProfile *providers.OAuthProfile, oauthClientID string) error {
	if oauthProfile != nil && oauthProfile.Certificate != nil && oauthClientID == "" {
		return ErrOAuthCertificateRequiresClientID
	}
	return nil
}

// ResolveInboundAuthProfileHandles resolves flow handle fields to their IDs in-place.
// Each handle is only resolved when the corresponding ID field is empty.
func (s *inboundClientService) ResolveInboundAuthProfileHandles(
	ctx context.Context, profile *providers.InboundAuthProfile,
) error {
	if s.flowMgt == nil {
		return nil
	}
	if profile.AuthFlowID == "" && profile.AuthFlowHandle != "" {
		flow, svcErr := s.flowMgt.GetFlowByHandle(ctx, profile.AuthFlowHandle, providers.FlowTypeAuthentication)
		if svcErr != nil {
			return ErrFKInvalidAuthFlow
		}
		profile.AuthFlowID = flow.ID
	}
	if profile.RegistrationFlowID == "" && profile.RegistrationFlowHandle != "" {
		flow, svcErr := s.flowMgt.GetFlowByHandle(ctx, profile.RegistrationFlowHandle, providers.FlowTypeRegistration)
		if svcErr != nil {
			return ErrFKInvalidRegistrationFlow
		}
		profile.RegistrationFlowID = flow.ID
	}
	if profile.RecoveryFlowID == "" && profile.RecoveryFlowHandle != "" {
		flow, svcErr := s.flowMgt.GetFlowByHandle(ctx, profile.RecoveryFlowHandle, providers.FlowTypeRecovery)
		if svcErr != nil {
			return ErrFKInvalidRecoveryFlow
		}
		profile.RecoveryFlowID = flow.ID
	}
	if profile.SignOutFlowID == "" && profile.SignOutFlowHandle != "" {
		flow, svcErr := s.flowMgt.GetFlowByHandle(ctx, profile.SignOutFlowHandle, providers.FlowTypeSignOut)
		if svcErr != nil {
			return ErrFKInvalidSignOutFlow
		}
		profile.SignOutFlowID = flow.ID
	}
	return nil
}

// resolveClientID returns the OAuth client_id from an entity's system attributes, or "" if absent.
func (s *inboundClientService) resolveClientID(ctx context.Context, entityID string) string {
	if s.entityProvider == nil {
		return ""
	}
	e, epErr := s.entityProvider.GetEntity(entityID)
	if epErr != nil {
		s.logger.Warn(ctx, "Failed to resolve OAuth client_id from entity provider",
			log.String("entityID", entityID), log.Error(epErr))
		return ""
	}
	if e == nil {
		return ""
	}
	var attrs map[string]interface{}
	if err := json.Unmarshal(e.SystemAttributes, &attrs); err != nil || attrs == nil {
		return ""
	}
	clientID, _ := attrs["clientId"].(string)
	return clientID
}

// DeleteInboundClient removes the inbound client, OAuth profile, and certificates for the given entity.
func (s *inboundClientService) DeleteInboundClient(ctx context.Context, entityID string) error {
	if s.store.IsDeclarative(ctx, entityID) {
		return ErrCannotModifyDeclarative
	}
	// Capture OAuth client_id before the caller deletes the entity itself.
	oauthClientID := s.resolveClientID(ctx, entityID)
	return s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if err := s.store.DeleteInboundClient(txCtx, entityID); err != nil {
			return err
		}
		if oauthClientID != "" {
			if opErr := s.deleteCertificate(txCtx, oauthClientID); opErr != nil {
				if opErr.Underlying == nil || opErr.Underlying.Code != cert.ErrorCertificateNotFound.Code {
					return opErr
				}
			}
		}
		return nil
	})
}

// GetOAuthProfileByEntityID returns the stored OAuth profile for the given entity.
func (s *inboundClientService) GetOAuthProfileByEntityID(ctx context.Context, entityID string) (
	*providers.OAuthProfile, error) {
	return s.store.GetOAuthProfileByEntityID(ctx, entityID)
}

// GetInboundClientAttributes returns the configured user attributes for a single inbound client. A
// missing inbound client is treated as an inbound client with no configured attributes.
func (s *inboundClientService) GetInboundClientAttributes(
	ctx context.Context, inboundClientID string,
) (*inboundmodel.InboundClientAttributes, error) {
	client, err := s.GetInboundClientByEntityID(ctx, inboundClientID)
	if err != nil {
		if errors.Is(err, ErrInboundClientNotFound) {
			return &inboundmodel.InboundClientAttributes{InboundClientID: inboundClientID}, nil
		}
		return nil, err
	}
	profile, err := s.getOAuthProfile(ctx, inboundClientID)
	if err != nil {
		return nil, err
	}
	return &inboundmodel.InboundClientAttributes{
		InboundClientID: client.ID,
		Attributes:      extractConfiguredAttributes(client, profile),
	}, nil
}

// ListInboundClientAttributes returns the configured user attributes for all inbound clients.
func (s *inboundClientService) ListInboundClientAttributes(
	ctx context.Context,
) ([]inboundmodel.InboundClientAttributes, error) {
	clients, err := s.GetInboundClientList(ctx)
	if err != nil {
		return nil, err
	}

	inboundClients := make([]inboundmodel.InboundClientAttributes, 0, len(clients))
	for i := range clients {
		profile, err := s.getOAuthProfile(ctx, clients[i].ID)
		if err != nil {
			return nil, err
		}
		inboundClients = append(inboundClients, inboundmodel.InboundClientAttributes{
			InboundClientID: clients[i].ID,
			Attributes:      extractConfiguredAttributes(&clients[i], profile),
		})
	}
	return inboundClients, nil
}

// getOAuthProfile returns the OAuth profile for an inbound client, treating a missing profile as nil
// since inbound clients may rely on assertion configuration alone.
func (s *inboundClientService) getOAuthProfile(
	ctx context.Context, inboundClientID string,
) (*providers.OAuthProfile, error) {
	profile, err := s.GetOAuthProfileByEntityID(ctx, inboundClientID)
	if err != nil {
		if errors.Is(err, ErrInboundClientNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return profile, nil
}

// extractConfiguredAttributes collects the deduplicated user attributes an inbound client is
// configured to release via its assertion config and OAuth profile.
func extractConfiguredAttributes(client *providers.InboundClient, profile *providers.OAuthProfile) []string {
	var assertion *inboundmodel.AssertionConfig
	if client != nil {
		assertion = client.Assertion
	}
	set := collectConfiguredUserAttributes(assertion, profile)
	attributes := make([]string, 0, len(set))
	for attr := range set {
		attributes = append(attributes, attr)
	}
	return attributes
}

// syncOAuthProfile creates, updates, or deletes the stored OAuth profile to match the desired state.
func (s *inboundClientService) syncOAuthProfile(ctx context.Context, entityID string,
	desired *providers.OAuthProfile) error {
	return s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		existing, err := s.store.GetOAuthProfileByEntityID(txCtx, entityID)
		if err != nil && !errors.Is(err, ErrInboundClientNotFound) {
			return err
		}
		switch {
		case desired != nil && existing != nil:
			return s.store.UpdateOAuthProfile(txCtx, entityID, desired)
		case desired != nil && existing == nil:
			return s.store.CreateOAuthProfile(txCtx, entityID, desired)
		case desired == nil && existing != nil:
			return s.store.DeleteOAuthProfile(txCtx, entityID)
		default:
			return nil
		}
	})
}

// GetOAuthClientByClientID resolves a full OAuthClient by its public client_id.
func (s *inboundClientService) GetOAuthClientByClientID(ctx context.Context, clientID string) (
	*providers.OAuthClient, error) {
	if s.entityProvider == nil {
		return nil, fmt.Errorf("entity provider not configured")
	}
	if clientID == "" {
		return nil, nil
	}

	entityIDPtr, epErr := s.entityProvider.IdentifyEntity(map[string]interface{}{"clientId": clientID})
	if epErr != nil {
		if epErr.Code == entityprovider.ErrorCodeEntityNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to resolve client_id: %w", epErr)
	}
	if entityIDPtr == nil {
		return nil, nil
	}
	entityID := *entityIDPtr
	e, epErr := s.entityProvider.GetEntity(entityID)
	if epErr != nil {
		if epErr.Code == entityprovider.ErrorCodeEntityNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to load entity for client_id: %w", epErr)
	}
	ouID := e.OUID

	oauthProfile, err := s.store.GetOAuthProfileByEntityID(ctx, entityID)
	if err != nil && !errors.Is(err, ErrInboundClientNotFound) {
		return nil, err
	}
	if oauthProfile == nil {
		return nil, nil
	}

	client := BuildOAuthClient(entityID, clientID, ouID, e.Category, oauthProfile)

	certificate, opErr := s.GetCertificate(ctx, cert.CertificateReferenceTypeOAuthApp, clientID)
	if opErr != nil {
		return nil, opErr
	}
	client.Certificate = certificate

	return client, nil
}

// BuildOAuthClient assembles an OAuthClient from a stored OAuthProfile and entity context.
func BuildOAuthClient(
	entityID, clientID, ouID string, entityCategory providers.EntityCategory, p *providers.OAuthProfile,
) *providers.OAuthClient {
	client := &providers.OAuthClient{
		ID:                                 entityID,
		OUID:                               ouID,
		ClientID:                           clientID,
		EntityCategory:                     entityCategory,
		RedirectURIs:                       p.RedirectURIs,
		PostLogoutRedirectURIs:             p.PostLogoutRedirectURIs,
		TokenEndpointAuthMethod:            providers.TokenEndpointAuthMethod(p.TokenEndpointAuthMethod),
		PKCERequired:                       p.PKCERequired,
		PublicClient:                       p.PublicClient,
		RequirePushedAuthorizationRequests: p.RequirePushedAuthorizationRequests,
		DPoPBoundAccessTokens:              p.DPoPBoundAccessTokens,
		IncludeActClaim:                    p.IncludeActClaim,
		Scopes:                             p.Scopes,
		ScopeClaims:                        p.ScopeClaims,
		Token:                              p.Token,
		UserInfo:                           p.UserInfo,
		Certificate:                        p.Certificate,
		AcrValues:                          p.AcrValues,
	}
	for _, gt := range p.GrantTypes {
		client.GrantTypes = append(client.GrantTypes, providers.GrantType(gt))
	}
	for _, rt := range p.ResponseTypes {
		client.ResponseTypes = append(client.ResponseTypes, providers.ResponseType(rt))
	}
	return client
}

// resolveFlowDefaults fills AuthFlowID, RegistrationFlowID, RecoveryFlowID, and SignOutFlowID
// using a uniform chain: explicit override → OU default → server default (only auth has a
// server-level default configured; registration/recovery/signout server defaults are intentionally
// left empty to prevent CALL-node mismatches across independently configured flows).
func (s *inboundClientService) resolveFlowDefaults(ctx context.Context, c *inboundmodel.InboundClient) error {
	if c == nil {
		return nil
	}

	// Drop registration/recovery bindings whose enable flag is false before any resolution so we
	// never persist an ID that contradicts the toggle. This must run regardless of whether flowMgt
	// is wired — persistence should still respect the caller's disabled intent.
	if !c.IsRegistrationFlowEnabled {
		c.RegistrationFlowID = ""
	}
	if !c.IsRecoveryFlowEnabled {
		c.RecoveryFlowID = ""
	}

	if s.flowMgt == nil {
		return nil
	}

	ouID := ""
	if s.entityProvider != nil && c.ID != "" {
		if e, epErr := s.entityProvider.GetEntity(c.ID); epErr == nil && e != nil {
			ouID = e.OUID
		}
	}

	resolve := func(flowID string, flowType providers.FlowType) (string, error) {
		id, svcErr := s.flowMgt.ResolveEffectiveFlowID(ctx, flowID, ouID, flowType)
		if svcErr != nil {
			if svcErr.Type == tidcommon.ServerErrorType {
				return "", ErrFKFlowServerError
			}
			if svcErr.Code == flowmgt.ErrorFlowNotFound.Code {
				switch flowType {
				case providers.FlowTypeSignOut, providers.FlowTypeRegistration,
					providers.FlowTypeRecovery, providers.FlowTypeUserOnboarding:
					// Optional flows: leave unconfigured rather than failing.
					return "", nil
				}
			}
			return "", ErrFKFlowDefinitionRetrievalFailed
		}
		return id, nil
	}

	authID, err := resolve(c.AuthFlowID, providers.FlowTypeAuthentication)
	if err != nil {
		return err
	}
	c.AuthFlowID = authID

	// Try to resolve the registration and recovery flows if they are enabled.
	// If the resolved ID is empty, disable the flow.
	if c.IsRegistrationFlowEnabled {
		regID, err := resolve(c.RegistrationFlowID, providers.FlowTypeRegistration)
		if err != nil {
			return err
		}
		c.RegistrationFlowID = regID
		if c.RegistrationFlowID == "" {
			c.IsRegistrationFlowEnabled = false
		}
	}
	if c.IsRecoveryFlowEnabled {
		recID, err := resolve(c.RecoveryFlowID, providers.FlowTypeRecovery)
		if err != nil {
			return err
		}
		c.RecoveryFlowID = recID
		if c.RecoveryFlowID == "" {
			c.IsRecoveryFlowEnabled = false
		}
	}

	signOutID, err := resolve(c.SignOutFlowID, providers.FlowTypeSignOut)
	if err != nil {
		return err
	}
	c.SignOutFlowID = signOutID

	return nil
}

// IsDeclarative reports whether the entity's inbound profile was loaded from a declarative resource file.
func (s *inboundClientService) IsDeclarative(ctx context.Context, entityID string) bool {
	return s.store.IsDeclarative(ctx, entityID)
}

// LoadDeclarativeResources loads inbound client profiles from YAML resource files.
func (s *inboundClientService) LoadDeclarativeResources(ctx context.Context,
	cfg inboundmodel.DeclarativeLoaderConfig) error {
	return loadDeclarativeResources(ctx, s.store, cfg)
}

// GetCertificate retrieves the certificate for the given reference type and ID.
func (s *inboundClientService) GetCertificate(ctx context.Context, refType cert.CertificateReferenceType,
	refID string) (*inboundmodel.Certificate, *CertOperationError) {
	c, svcErr := s.certService.GetCertificateByReference(ctx, refType, refID)
	if svcErr != nil {
		if svcErr.Code == cert.ErrorCertificateNotFound.Code {
			return nil, nil
		}
		return nil, &CertOperationError{Operation: CertOpRetrieve, RefType: refType, Underlying: svcErr}
	}
	if c == nil {
		return nil, nil
	}
	return &inboundmodel.Certificate{Type: c.Type, Value: c.Value}, nil
}

// createCertificate validates and creates a new OAuth-app certificate record.
func (s *inboundClientService) createCertificate(ctx context.Context, refID string,
	in *inboundmodel.Certificate) (*inboundmodel.Certificate, error, *CertOperationError) {
	c, vErr := validateCertificateInput(refID, "", in)
	if vErr != nil {
		return nil, vErr, nil
	}
	if c == nil {
		return nil, nil, nil
	}
	if _, svcErr := s.certService.CreateCertificate(ctx, c); svcErr != nil {
		return nil, nil, &CertOperationError{Operation: CertOpCreate, RefType: c.RefType, Underlying: svcErr}
	}
	return &inboundmodel.Certificate{Type: c.Type, Value: c.Value}, nil, nil
}

// syncCertificate creates, updates, or deletes the OAuth-app certificate to match the desired state.
func (s *inboundClientService) syncCertificate(ctx context.Context, refID string,
	in *inboundmodel.Certificate) (*inboundmodel.Certificate, error, *CertOperationError) {
	refType := cert.CertificateReferenceTypeOAuthApp
	existing, svcErr := s.certService.GetCertificateByReference(ctx, refType, refID)
	if svcErr != nil && svcErr.Code != cert.ErrorCertificateNotFound.Code {
		return nil, nil, &CertOperationError{Operation: CertOpRetrieve, RefType: refType, Underlying: svcErr}
	}

	existingID := ""
	if existing != nil {
		existingID = existing.ID
	}
	desired, vErr := validateCertificateInput(refID, existingID, in)
	if vErr != nil {
		return nil, vErr, nil
	}

	if desired != nil {
		if existing != nil {
			if _, opErr := s.certService.UpdateCertificateByID(ctx, existing.ID, desired); opErr != nil {
				return nil, nil, &CertOperationError{Operation: CertOpUpdate, RefType: refType, Underlying: opErr}
			}
		} else {
			if _, opErr := s.certService.CreateCertificate(ctx, desired); opErr != nil {
				return nil, nil, &CertOperationError{Operation: CertOpCreate, RefType: refType, Underlying: opErr}
			}
		}
		return &inboundmodel.Certificate{Type: desired.Type, Value: desired.Value}, nil, nil
	}

	if existing != nil {
		if opErr := s.certService.DeleteCertificateByReference(ctx, refType, refID); opErr != nil {
			return nil, nil, &CertOperationError{Operation: CertOpDelete, RefType: refType, Underlying: opErr}
		}
	}
	return nil, nil, nil
}

// deleteCertificate removes the OAuth-app certificate for the given client ID.
func (s *inboundClientService) deleteCertificate(ctx context.Context, refID string) *CertOperationError {
	refType := cert.CertificateReferenceTypeOAuthApp
	if s.certService == nil {
		return nil
	}
	if svcErr := s.certService.DeleteCertificateByReference(ctx, refType, refID); svcErr != nil {
		return &CertOperationError{Operation: CertOpDelete, RefType: refType, Underlying: svcErr}
	}
	return nil
}

// validateCertificateInput validates and maps inbound certificate input to a cert.Certificate.
func validateCertificateInput(refID, existingCertID string, in *inboundmodel.Certificate) (*cert.Certificate, error) {
	if in == nil || in.Type == "" {
		return nil, nil
	}
	refType := cert.CertificateReferenceTypeOAuthApp
	switch in.Type {
	case cert.CertificateTypeJWKS:
		if in.Value == "" {
			return nil, ErrCertValueRequired
		}
		return &cert.Certificate{
			ID: existingCertID, RefType: refType, RefID: refID,
			Type: cert.CertificateTypeJWKS, Value: in.Value,
		}, nil
	case cert.CertificateTypeJWKSURI:
		if !sysutils.IsValidURI(in.Value) {
			return nil, ErrCertInvalidJWKSURI
		}
		return &cert.Certificate{
			ID: existingCertID, RefType: refType, RefID: refID,
			Type: cert.CertificateTypeJWKSURI, Value: in.Value,
		}, nil
	default:
		return nil, ErrCertInvalidType
	}
}

// validateOAuthProfile validates all fields of an OAuth profile data object.
func validateOAuthProfile(ctx context.Context, p *providers.OAuthProfile, hasClientSecret bool,
	cryptoProvider providers.RuntimeCryptoProvider, jweService jwe.JWEServiceInterface) error {
	if p == nil {
		return nil
	}
	if err := validateRedirectURIs(p); err != nil {
		return err
	}
	if err := validateGrantAndResponseTypes(p); err != nil {
		return err
	}
	if err := validateTokenEndpointAuthMethod(p, hasClientSecret); err != nil {
		return err
	}
	if p.PublicClient {
		if err := validatePublicClient(p); err != nil {
			return err
		}
	}
	if err := validateUserInfoConfig(ctx, p, cryptoProvider, jweService); err != nil {
		return err
	}
	if err := validateIDTokenConfig(ctx, p, cryptoProvider, jweService); err != nil {
		return err
	}
	if err := validateAccessTokenConfig(p); err != nil {
		return err
	}
	return nil
}

// maxDefaultAudienceLength bounds the access token default audience, a single audience identifier
// (typically a URI), to a sane length.
const maxDefaultAudienceLength = 2048

// validateAccessTokenConfig validates the access token configuration.
func validateAccessTokenConfig(p *providers.OAuthProfile) error {
	if p.Token == nil || p.Token.AccessToken == nil {
		return nil
	}
	if len(p.Token.AccessToken.DefaultAudience) > maxDefaultAudienceLength {
		return ErrOAuthDefaultAudienceTooLong
	}
	return nil
}

// configuredSigningAlgorithms returns the JWS algorithms the deployment's configured signing keys
// support. This is narrower than the algorithms the crypto provider can compute, and matches what
// discovery advertises, so a client cannot register an algorithm that has no key to sign with.
func configuredSigningAlgorithms(
	ctx context.Context, cryptoProvider providers.RuntimeCryptoProvider,
) ([]string, error) {
	keys, err := cryptoProvider.GetPublicKeys(ctx, providers.PublicKeyFilter{})
	if err != nil {
		log.GetLogger().Error(ctx, "Failed to retrieve public keys for signing algorithm validation",
			log.Error(err))
		return nil, fmt.Errorf("failed to retrieve signing keys: %w", err)
	}

	algs := make([]string, 0, len(keys))
	for _, key := range keys {
		if key.Algorithm != "" && !slices.Contains(algs, key.Algorithm) {
			algs = append(algs, key.Algorithm)
		}
	}
	return algs, nil
}

// validateUserInfoConfig validates the UserInfo signing and encryption configuration.
func validateUserInfoConfig(ctx context.Context, p *providers.OAuthProfile,
	cryptoProvider providers.RuntimeCryptoProvider, jweService jwe.JWEServiceInterface) error {
	if p.UserInfo == nil {
		return nil
	}
	cfg := p.UserInfo

	if cfg.SigningAlg != "" {
		algs, err := configuredSigningAlgorithms(ctx, cryptoProvider)
		if err != nil {
			return err
		}
		if !slices.Contains(algs, cfg.SigningAlg) {
			return ErrOAuthUserInfoUnsupportedSigningAlg
		}
	}

	if cfg.EncryptionEnc != "" && cfg.EncryptionAlg == "" {
		return ErrOAuthUserInfoEncryptionEncRequiresAlg
	}

	if cfg.EncryptionAlg != "" {
		if !slices.Contains(jweService.SupportedKeyEncryptionAlgorithms(), cfg.EncryptionAlg) {
			return ErrOAuthUserInfoUnsupportedEncryptionAlg
		}
		if cfg.EncryptionEnc == "" {
			return ErrOAuthUserInfoEncryptionAlgRequiresEnc
		}
		if !slices.Contains(jweService.SupportedContentEncryptionAlgorithms(), cfg.EncryptionEnc) {
			return ErrOAuthUserInfoUnsupportedEncryptionEnc
		}
		hasCert := p.Certificate != nil && p.Certificate.Type != ""
		if !hasCert {
			return ErrOAuthUserInfoEncryptionRequiresCertificate
		}
		if p.Certificate.Type == cert.CertificateTypeJWKSURI {
			if err := syshttp.IsSSRFSafeURL(p.Certificate.Value); err != nil {
				return ErrOAuthUserInfoJWKSURINotSSRFSafe
			}
		}
	}

	if cfg.ResponseType == "" && (cfg.SigningAlg != "" || cfg.EncryptionAlg != "" || cfg.EncryptionEnc != "") {
		return ErrOAuthUserInfoAlgRequiresResponseType
	}

	if cfg.ResponseType != "" {
		switch cfg.ResponseType {
		case providers.UserInfoResponseTypeJWS:
			// Signing uses only the server's signing key as of now.
		case providers.UserInfoResponseTypeJWE:
			if cfg.EncryptionAlg == "" || cfg.EncryptionEnc == "" {
				return ErrOAuthUserInfoJWERequiresEncryption
			}
		case providers.UserInfoResponseTypeNESTEDJWT:
			if cfg.EncryptionAlg == "" || cfg.EncryptionEnc == "" {
				return ErrOAuthUserInfoNestedJWTRequiresAll
			}
		case providers.UserInfoResponseTypeJSON:
			// no additional requirements
		default:
			return ErrOAuthUserInfoUnsupportedResponseType
		}
	}
	return nil
}

// validateIDTokenConfig validates the ID token configuration.
// responseType is the authoritative field; empty defaults to JWT.
func validateIDTokenConfig(ctx context.Context, p *providers.OAuthProfile,
	cryptoProvider providers.RuntimeCryptoProvider, jweService jwe.JWEServiceInterface) error {
	if p.Token == nil || p.Token.IDToken == nil {
		return nil
	}
	cfg := p.Token.IDToken

	if cfg.SigningAlg != "" {
		algs, err := configuredSigningAlgorithms(ctx, cryptoProvider)
		if err != nil {
			return err
		}
		if !slices.Contains(algs, cfg.SigningAlg) {
			return ErrOAuthIDTokenUnsupportedSigningAlg
		}
	}

	if cfg.ResponseType == "" {
		cfg.ResponseType = providers.IDTokenResponseTypeJWT
	}

	switch cfg.ResponseType {
	case providers.IDTokenResponseTypeJWT:
		if cfg.EncryptionAlg != "" || cfg.EncryptionEnc != "" {
			return ErrOAuthIDTokenEncryptionFieldsNotAllowed
		}
	case providers.IDTokenResponseTypeJWE, providers.IDTokenResponseTypeNESTEDJWT:
		if cfg.EncryptionAlg == "" || cfg.EncryptionEnc == "" {
			return ErrOAuthIDTokenEncryptionAlgRequiresEnc
		}
		if !slices.Contains(jweService.SupportedKeyEncryptionAlgorithms(), cfg.EncryptionAlg) {
			return ErrOAuthIDTokenUnsupportedEncryptionAlg
		}
		if !slices.Contains(jweService.SupportedContentEncryptionAlgorithms(), cfg.EncryptionEnc) {
			return ErrOAuthIDTokenUnsupportedEncryptionEnc
		}
		hasCert := p.Certificate != nil && p.Certificate.Type != ""
		if !hasCert {
			return ErrOAuthIDTokenEncryptionRequiresCertificate
		}
		if p.Certificate.Type == cert.CertificateTypeJWKSURI {
			if err := syshttp.IsSSRFSafeURL(p.Certificate.Value); err != nil {
				return ErrOAuthIDTokenJWKSURINotSSRFSafe
			}
		}
	default:
		return ErrOAuthIDTokenUnsupportedResponseType
	}
	return nil
}

// validateRedirectURIs validates redirect URIs and authorization_code grant requirements.
func validateRedirectURIs(p *providers.OAuthProfile) error {
	for _, redirectURI := range p.RedirectURIs {
		// Reject wildcards in the scheme before URL parsing — url.Parse may misinterpret them.
		if idx := strings.Index(redirectURI, "://"); idx != -1 {
			if strings.ContainsRune(redirectURI[:idx], '*') {
				return ErrOAuthInvalidRedirectURI
			}
		}
		parsedURI, err := sysutils.ParseURL(redirectURI)
		if err != nil {
			return ErrOAuthInvalidRedirectURI
		}
		// Custom URI schemes (RFC 8252 §7.1) don't require a host; path-only like "myapp:/callback" is valid.
		isWebScheme := parsedURI.Scheme == "http" || parsedURI.Scheme == "https"
		if parsedURI.Scheme == "" || (isWebScheme && parsedURI.Host == "") ||
			(!isWebScheme && parsedURI.Host == "" && parsedURI.Path == "") {
			return ErrOAuthInvalidRedirectURI
		}
		if parsedURI.Fragment != "" {
			return ErrOAuthRedirectURIFragmentNotAllowed
		}
		wildcardEnabled := config.GetServerRuntime().Config.OAuth.AllowWildcardRedirectURI
		if strings.ContainsRune(parsedURI.Host, '*') {
			if !wildcardEnabled {
				return ErrOAuthInvalidRedirectURI
			}
			if err := validateHostWildcardPattern(parsedURI.Host); err != nil {
				return err
			}
		}
		if strings.ContainsRune(parsedURI.RawQuery, '*') {
			return ErrOAuthInvalidRedirectURI
		}
		if containsInvalidWildcardSegment(parsedURI.Path) {
			return ErrOAuthInvalidRedirectURI
		}
		if strings.ContainsRune(parsedURI.Path, '*') && !wildcardEnabled {
			return ErrOAuthInvalidRedirectURI
		}
	}
	if slices.Contains(p.GrantTypes, string(providers.GrantTypeAuthorizationCode)) &&
		len(p.RedirectURIs) == 0 {
		return ErrOAuthAuthCodeRequiresRedirectURIs
	}
	return nil
}

// validateHostWildcardPattern enforces structural rules for wildcards in the host
// component: no * in the port portion of host:port, and no whole-label *. * matches one
// or more alphanumeric characters at match time, enforced by the matcher itself.
func validateHostWildcardPattern(host string) error {
	if i := strings.LastIndex(host, ":"); i != -1 {
		if strings.ContainsRune(host[i+1:], '*') {
			return ErrOAuthInvalidRedirectURI
		}
		host = host[:i]
	}
	for _, label := range strings.Split(host, ".") {
		if label == "*" {
			return ErrOAuthInvalidRedirectURI
		}
	}
	return nil
}

// containsInvalidWildcardSegment returns true if any path segment mixes * with other
// characters (e.g. "foo*") or contains regex metacharacters (e.g. "[a-z]+").
func containsInvalidWildcardSegment(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if strings.ContainsRune(seg, '*') && seg != "*" && seg != "**" {
			return true
		}
		if strings.ContainsAny(seg, "[](){}+?|^$\\") {
			return true
		}
	}
	return false
}

// validateGrantAndResponseTypes validates grant types, response types, and their combinations.
func validateGrantAndResponseTypes(p *providers.OAuthProfile) error {
	err := validateWithAllowedGrantTypes(p.GrantTypes)
	if err != nil {
		return err
	}
	err = validateWithAllowedResponseTypes(p.ResponseTypes)
	if err != nil {
		return err
	}
	if len(p.GrantTypes) == 1 &&
		slices.Contains(p.GrantTypes, string(providers.GrantTypeClientCredentials)) &&
		len(p.ResponseTypes) > 0 {
		return ErrOAuthClientCredentialsCannotUseResponseTypes
	}
	if slices.Contains(p.GrantTypes, string(providers.GrantTypeAuthorizationCode)) {
		if len(p.ResponseTypes) == 0 ||
			!slices.Contains(p.ResponseTypes, string(providers.ResponseTypeCode)) {
			return ErrOAuthAuthCodeRequiresCodeResponseType
		}
	}
	if slices.Contains(p.GrantTypes, string(providers.GrantTypeRefreshToken)) &&
		!providers.AnyIssuesRefreshToken(p.GrantTypes) {
		return ErrOAuthRefreshTokenRequiresTokenIssuingGrant
	}
	if p.PKCERequired &&
		!slices.Contains(p.GrantTypes, string(providers.GrantTypeAuthorizationCode)) {
		return ErrOAuthPKCERequiresAuthCode
	}
	if len(p.ResponseTypes) > 0 &&
		!slices.Contains(p.GrantTypes, string(providers.GrantTypeAuthorizationCode)) {
		return ErrOAuthResponseTypesRequireAuthCode
	}
	return nil
}

// validateTokenEndpointAuthMethod validates the token endpoint auth method against cert and secret state.
func validateTokenEndpointAuthMethod(p *providers.OAuthProfile, hasClientSecret bool) error {
	err := validateWithAllowedTokenEndpointAuthMethod(p.TokenEndpointAuthMethod)
	if err != nil {
		return err
	}
	hasCert := p.Certificate != nil && p.Certificate.Type != ""

	switch providers.TokenEndpointAuthMethod(p.TokenEndpointAuthMethod) {
	case providers.TokenEndpointAuthMethodPrivateKeyJWT:
		if !hasCert {
			return ErrOAuthPrivateKeyJWTRequiresCertificate
		}
		if hasClientSecret {
			return ErrOAuthPrivateKeyJWTCannotHaveClientSecret
		}
	case providers.TokenEndpointAuthMethodClientSecretBasic, providers.TokenEndpointAuthMethodClientSecretPost:
		// A certificate is allowed: it carries the client's public key for token encryption
		// (JWE / NESTED_JWT), independent of how the client authenticates.
	case providers.TokenEndpointAuthMethodNone:
		if !p.PublicClient {
			return ErrOAuthNoneAuthRequiresPublicClient
		}
		if hasClientSecret {
			return ErrOAuthNoneAuthCannotHaveSecret
		}
		if slices.Contains(p.GrantTypes, string(providers.GrantTypeClientCredentials)) {
			return ErrOAuthClientCredentialsCannotUseNoneAuth
		}
		// The jwt-bearer (ID-JAG) grant is bound to the client via client_id only, so it requires a
		// confidential client.
		if slices.Contains(p.GrantTypes, string(providers.GrantTypeJWTBearer)) {
			return ErrOAuthClientJWTBearerCannotUseNoneAuth
		}
		// Requesting ID-JAGs is likewise restricted to confidential clients.
		if p.Token != nil && p.Token.IDJAG != nil && p.Token.IDJAG.Enabled {
			return ErrOAuthClientIDJAGCannotUseNoneAuth
		}
	}
	return nil
}

// validateAllowedGrantTypes rejects grant types not permitted by the deployment's configured
// oauth.allowed_grant_types allow-list. An empty allow-list permits all grant types.
func validateWithAllowedGrantTypes(grantTypes []string) error {
	allowed := config.GetServerRuntime().Config.OAuth.AllowedGrantTypes
	for _, grantType := range grantTypes {
		if !providers.GrantType(grantType).IsValid() {
			return ErrOAuthInvalidGrantType
		}
		if len(allowed) > 0 && !slices.Contains(allowed, grantType) {
			return ErrOAuthInvalidGrantType
		}
	}
	return nil
}

// validateAllowedResponseTypes rejects response types not permitted by the deployment's configured
// oauth.allowed_response_types allow-list. An empty allow-list permits all response types.
func validateWithAllowedResponseTypes(responseTypes []string) error {
	allowed := config.GetServerRuntime().Config.OAuth.AllowedResponseTypes
	for _, responseType := range responseTypes {
		if !providers.ResponseType(responseType).IsValid() {
			return ErrOAuthInvalidResponseType
		}
		if len(allowed) > 0 && !slices.Contains(allowed, responseType) {
			return ErrOAuthInvalidResponseType
		}
	}
	return nil
}

// validateAllowedTokenEndpointAuthMethod rejects a token endpoint auth method not permitted by the
// deployment's configured oauth.allowed_auth_methods allow-list. An empty allow-list permits all methods.
func validateWithAllowedTokenEndpointAuthMethod(method string) error {
	if !providers.TokenEndpointAuthMethod(method).IsValid() {
		return ErrOAuthInvalidTokenEndpointAuthMethod
	}
	allowed := config.GetServerRuntime().Config.OAuth.AllowedAuthMethods
	if len(allowed) == 0 || slices.Contains(allowed, method) {
		return nil
	}
	return ErrOAuthInvalidTokenEndpointAuthMethod
}

// validatePublicClient validates constraints required for public clients.
func validatePublicClient(p *providers.OAuthProfile) error {
	if providers.TokenEndpointAuthMethod(p.TokenEndpointAuthMethod) != providers.TokenEndpointAuthMethodNone {
		return ErrOAuthPublicClientMustUseNoneAuth
	}
	if !p.PKCERequired {
		return ErrOAuthPublicClientMustHavePKCE
	}
	return nil
}

// validateFKs validates all FK references on an inbound client.
func (s *inboundClientService) validateFKs(ctx context.Context, c *inboundmodel.InboundClient) error {
	if c == nil {
		return nil
	}
	if err := s.validateAuthFlowID(ctx, c.AuthFlowID); err != nil {
		return err
	}
	if err := s.validateRegistrationFlowID(ctx, c.RegistrationFlowID); err != nil {
		return err
	}
	if err := s.validateRecoveryFlowID(ctx, c.RecoveryFlowID); err != nil {
		return err
	}
	if err := s.validateSignOutFlowID(ctx, c.SignOutFlowID); err != nil {
		return err
	}
	if err := s.validateReferencedFlows(ctx, c); err != nil {
		return err
	}
	if err := s.validateThemeID(ctx, c.ThemeID); err != nil {
		return err
	}
	if err := s.validateLayoutID(ctx, c.LayoutID); err != nil {
		return err
	}
	if err := s.validateAllowedUserTypes(ctx, c.AllowedUserTypes); err != nil {
		return err
	}
	if err := s.validateAllowedAgentTypes(ctx, c.AllowedAgentTypes); err != nil {
		return err
	}
	return nil
}

// validateAuthFlowID validates that the auth flow ID exists and is of the correct type.
func (s *inboundClientService) validateAuthFlowID(ctx context.Context, flowID string) error {
	if flowID == "" || s.flowMgt == nil {
		return nil
	}
	valid, svcErr := s.flowMgt.IsValidFlow(ctx, flowID, providers.FlowTypeAuthentication)
	if svcErr != nil {
		return ErrFKFlowServerError
	}
	if !valid {
		return ErrFKInvalidAuthFlow
	}
	return nil
}

// validateRegistrationFlowID validates that the registration flow ID exists and is of the correct type.
func (s *inboundClientService) validateRegistrationFlowID(ctx context.Context, flowID string) error {
	if flowID == "" || s.flowMgt == nil {
		return nil
	}
	valid, svcErr := s.flowMgt.IsValidFlow(ctx, flowID, providers.FlowTypeRegistration)
	if svcErr != nil {
		return ErrFKFlowServerError
	}
	if !valid {
		return ErrFKInvalidRegistrationFlow
	}
	return nil
}

// validateRecoveryFlowID validates that the recovery flow ID exists and is of the correct type.
func (s *inboundClientService) validateRecoveryFlowID(ctx context.Context, flowID string) error {
	if flowID == "" || s.flowMgt == nil {
		return nil
	}
	valid, svcErr := s.flowMgt.IsValidFlow(ctx, flowID, providers.FlowTypeRecovery)
	if svcErr != nil {
		return ErrFKFlowServerError
	}
	if !valid {
		return ErrFKInvalidRecoveryFlow
	}
	return nil
}

// validateSignOutFlowID validates that the sign-out flow ID exists and is of the correct type.
func (s *inboundClientService) validateSignOutFlowID(ctx context.Context, flowID string) error {
	if flowID == "" || s.flowMgt == nil {
		return nil
	}
	valid, svcErr := s.flowMgt.IsValidFlow(ctx, flowID, providers.FlowTypeSignOut)
	if svcErr != nil {
		return ErrFKFlowServerError
	}
	if !valid {
		return ErrFKInvalidSignOutFlow
	}
	return nil
}

// validateThemeID validates that the theme ID exists.
func (s *inboundClientService) validateThemeID(ctx context.Context, themeID string) error {
	if themeID == "" || s.themeMgt == nil {
		return nil
	}
	exists, svcErr := s.themeMgt.IsThemeExist(ctx, themeID)
	if svcErr != nil || !exists {
		return ErrFKThemeNotFound
	}
	return nil
}

// validateLayoutID validates that the layout ID exists.
func (s *inboundClientService) validateLayoutID(ctx context.Context, layoutID string) error {
	if layoutID == "" || s.layoutMgt == nil {
		return nil
	}
	exists, svcErr := s.layoutMgt.IsLayoutExist(ctx, layoutID)
	if svcErr != nil || !exists {
		return ErrFKLayoutNotFound
	}
	return nil
}

// validateAllowedUserTypes validates that each allowed user type corresponds to an existing user type.
func (s *inboundClientService) validateAllowedUserTypes(
	ctx context.Context, allowedUserTypes []string,
) error {
	return s.validateAllowedEntityTypes(ctx, entitytype.TypeCategoryUser, allowedUserTypes,
		ErrFKInvalidUserType, ErrUserSchemaLookupFailed)
}

// validateAllowedAgentTypes validates that each allowed agent type corresponds to an existing agent type.
func (s *inboundClientService) validateAllowedAgentTypes(
	ctx context.Context, allowedAgentTypes []string,
) error {
	return s.validateAllowedEntityTypes(ctx, entitytype.TypeCategoryAgent, allowedAgentTypes,
		ErrFKInvalidAgentType, ErrAgentSchemaLookupFailed)
}

// validateAllowedEntityTypes validates that each name in allowedTypes corresponds to an existing
// entity type in the given category. invalidErr is returned for an unknown name; lookupErr is
// returned when the entity type service itself fails, so the caller can tell a client validation
// failure from a server fault.
func (s *inboundClientService) validateAllowedEntityTypes(
	ctx context.Context, category entitytype.TypeCategory, allowedTypes []string,
	invalidErr, lookupErr error,
) error {
	if len(allowedTypes) == 0 || s.entityType == nil {
		return nil
	}
	existingTypes := make(map[string]bool)
	limit := serverconst.MaxPageSize
	offset := 0
	for {
		// Runtime context: skip authorization checks when fetching entity types.
		entityTypeList, svcErr := s.entityType.GetEntityTypeList(
			security.WithRuntimeContext(ctx), category, limit, offset, false)
		if svcErr != nil {
			s.logger.Error(ctx, "Failed to retrieve entity type list for validation",
				log.String("category", string(category)),
				log.String("error", svcErr.Error.DefaultValue), log.String("code", svcErr.Code))
			return lookupErr
		}
		for _, schema := range entityTypeList.Types {
			existingTypes[schema.Name] = true
		}
		if len(entityTypeList.Types) == 0 ||
			offset+len(entityTypeList.Types) >= entityTypeList.TotalResults {
			break
		}
		offset += limit
	}
	for _, entityTypeName := range allowedTypes {
		if entityTypeName == "" || !existingTypes[entityTypeName] {
			return invalidErr
		}
	}
	return nil
}

// validateSubjectAttributeMapping validates that the subject attribute mapping is well-formed.
// assumes that allowedUserTypes has been validated.
func (s *inboundClientService) validateSubjectAttributeMapping(
	ctx context.Context, subjectAttributeMapping map[string]string, allowedUserTypes []string,
) error {
	if len(subjectAttributeMapping) == 0 || s.entityType == nil {
		return nil
	}
	for entityType, subjectAttribute := range subjectAttributeMapping {
		if entityType == "" || subjectAttribute == "" {
			return ErrFKInvalidSubjectAttributeMapping
		}
		if !slices.Contains(allowedUserTypes, entityType) {
			return ErrFKInvalidSubjectAttributeMapping
		}
		// A subject attribute must be unique, required, non-credential, and string-typed: unique so it
		// identifies a single user, required so every user of the type has it (a stable sub), and
		// string so it can be emitted as the string sub claim.
		attrs, svcErr := s.entityType.GetAttributes(
			security.WithRuntimeContext(ctx), entitytype.TypeCategoryUser, entityType,
			entitytype.AttributeFilter{
				AllowNonCredential: true,
				RequiredOnly:       true,
				UniqueOnly:         true,
				Type:               entitytypemodel.TypeString,
			})
		if svcErr != nil {
			s.logger.Error(ctx, "Failed to retrieve attributes for subject mapping validation",
				log.String("error", svcErr.Error.DefaultValue), log.String("code", svcErr.Code))
			return ErrUniqueAttributeLookupFailed
		}
		if !slices.ContainsFunc(attrs, func(a entitytype.AttributeInfo) bool {
			return a.Attribute == subjectAttribute
		}) {
			return ErrFKInvalidSubjectAttributeMapping
		}
	}
	return nil
}

// validateUserAttributesAgainstAllowedTypes validates that every user attribute specified in the
// assertion, token, and userinfo configs is a non-credential attribute defined in at least one
// of the application's allowed entity types. Returns ErrInvalidUserAttribute when any attribute
// fails the check.
func (s *inboundClientService) validateUserAttributesAgainstAllowedTypes(
	ctx context.Context,
	allowedEntityTypes []string,
	assertion *inboundmodel.AssertionConfig,
	oauthProfile *providers.OAuthProfile,
) error {
	// Skip the schema lookup entirely when there is nothing to validate.
	if len(collectConfiguredUserAttributes(assertion, oauthProfile)) == 0 {
		return nil
	}
	validAttrs, err := s.resolveValidUserAttributes(ctx, allowedEntityTypes)
	if err != nil {
		return err
	}
	return validateUserAttributes(validAttrs, assertion, oauthProfile)
}

// validateUserAttributes checks the configured attribute lists against an already-resolved set of
// valid attributes. A nil set means nothing to validate against, so anything is accepted.
func validateUserAttributes(
	validAttrs map[string]bool,
	assertion *inboundmodel.AssertionConfig,
	oauthProfile *providers.OAuthProfile,
) error {
	if validAttrs == nil {
		return nil
	}

	for attr := range collectConfiguredUserAttributes(assertion, oauthProfile) {
		if isComputedAttribute(attr) {
			continue
		}
		if !validAttrs[attr] {
			return ErrInvalidUserAttribute
		}
	}
	return nil
}

// scopeClaimPruneSet returns the attribute set the mapping is pruned against. Defaults we seeded
// for a client with no allowed user types are pruned against an empty set: the standard scopes are
// kept, but none of their attributes are, since no schema declares them yet. A mapping the caller
// supplied is never pruned this way, so it is still stored as sent.
func scopeClaimPruneSet(seeded bool, allowedUserTypes []string, validAttrs map[string]bool) map[string]bool {
	if seeded && len(allowedUserTypes) == 0 {
		return map[string]bool{}
	}
	return validAttrs
}

// pruneScopeClaims drops attributes the allowed user types' schemas do not declare, keeping
// computed ones. A scope emptied by the prune keeps its key, so it stays grantable with no claims.
// A nil valid set leaves the mapping untouched.
func pruneScopeClaims(oauthProfile *providers.OAuthProfile, validAttrs map[string]bool) {
	if oauthProfile == nil || validAttrs == nil {
		return
	}
	for scope, attrs := range oauthProfile.ScopeClaims {
		kept := make([]string, 0, len(attrs))
		for _, attr := range attrs {
			if isComputedAttribute(attr) || validAttrs[attr] {
				kept = append(kept, attr)
			}
		}
		oauthProfile.ScopeClaims[scope] = kept
	}
}

// seedIDTokenUserAttributes derives a new client's ID token attribute list from its scope-to-claims
// mapping when the caller configured none. UserInfo inherits the list in applyInboundDefaults; the
// access token and the assertion are not scope-driven and keep what was sent. Skipped without
// allowed user types (nil validAttrs), since the mapping was then never pruned to a real schema.
func seedIDTokenUserAttributes(oauthProfile *providers.OAuthProfile, validAttrs map[string]bool) {
	if oauthProfile == nil || validAttrs == nil || len(oauthProfile.ScopeClaims) == 0 {
		return
	}
	if oauthProfile.Token != nil && oauthProfile.Token.IDToken != nil &&
		len(oauthProfile.Token.IDToken.UserAttributes) > 0 {
		return
	}

	// The mapping is already seeded and pruned to the schema by this point.
	derived := make(map[string]bool)
	for _, attrs := range oauthProfile.ScopeClaims {
		for _, attr := range attrs {
			derived[attr] = true
		}
	}
	if len(derived) == 0 {
		return
	}

	attributes := slices.Collect(maps.Keys(derived))
	slices.Sort(attributes)
	if oauthProfile.Token == nil {
		oauthProfile.Token = &providers.OAuthTokenConfig{}
	}
	if oauthProfile.Token.IDToken == nil {
		oauthProfile.Token.IDToken = &providers.IDTokenConfig{}
	}
	oauthProfile.Token.IDToken.UserAttributes = attributes
}

// resolveValidUserAttributes returns the union of non-credential attribute names declared in the
// schemas of the given allowed user types. Returns (nil, nil) when there are no allowed types or the
// entity-type service is unavailable, signaling callers to skip attribute reconciliation.
func (s *inboundClientService) resolveValidUserAttributes(
	ctx context.Context,
	allowedEntityTypes []string,
) (map[string]bool, error) {
	if len(allowedEntityTypes) == 0 || s.entityType == nil {
		return nil, nil
	}
	validAttrs := make(map[string]bool)
	for _, entityTypeName := range allowedEntityTypes {
		attrInfos, svcErr := s.entityType.GetAttributes(
			security.WithRuntimeContext(ctx), entitytype.TypeCategoryUser, entityTypeName,
			entitytype.AttributeFilter{AllowNonCredential: true})
		if svcErr != nil {
			if svcErr.Type == tidcommon.ServerErrorType {
				return nil, ErrUserSchemaLookupFailed
			}
			return nil, ErrFKInvalidUserType
		}
		for _, info := range attrInfos {
			validAttrs[info.Attribute] = true
		}
	}
	return validAttrs, nil
}

// stripUndeclaredUserAttributes removes from the token allow-lists and the scope-to-claims mapping
// any user attribute no longer declared in the allowed user types' schemas, keeping computed ones.
// Both are pruned together so a scope can never expose an attribute the lists may not carry. No-op
// when there are no allowed types or the entity-type service is unavailable.
func (s *inboundClientService) stripUndeclaredUserAttributes(
	ctx context.Context,
	allowedEntityTypes []string,
	assertion *inboundmodel.AssertionConfig,
	oauthProfile *providers.OAuthProfile,
) error {
	lists := configuredUserAttributeLists(assertion, oauthProfile)
	configured := 0
	for _, list := range lists {
		configured += len(*list)
	}
	if configured == 0 && (oauthProfile == nil || len(oauthProfile.ScopeClaims) == 0) {
		return nil
	}
	validAttrs, err := s.resolveValidUserAttributes(ctx, allowedEntityTypes)
	if err != nil || validAttrs == nil {
		return err
	}
	pruneScopeClaims(oauthProfile, validAttrs)

	dropped := make(map[string]bool)
	for _, list := range lists {
		kept := make([]string, 0, len(*list))
		for _, attr := range *list {
			if isComputedAttribute(attr) || validAttrs[attr] {
				kept = append(kept, attr)
				continue
			}
			dropped[attr] = true
		}
		*list = kept
	}

	if len(dropped) > 0 && s.logger.IsDebugEnabled() {
		names := make([]string, 0, len(dropped))
		for attr := range dropped {
			names = append(names, attr)
		}
		slices.Sort(names)
		s.logger.Debug(ctx, "Stripped user attributes no longer declared in the entity type schema",
			log.String("droppedAttributes", strings.Join(names, ", ")))
	}
	return nil
}

// isComputedAttribute returns true for attributes that are derived at runtime (e.g. from group
// memberships or OU associations) and are not defined in the entity type schema.
func isComputedAttribute(attr string) bool {
	switch attr {
	case oauth2const.UserAttributeGroups,
		oauth2const.UserAttributeRoles,
		oauth2const.ClaimOUID,
		oauth2const.ClaimOUName,
		oauth2const.ClaimOUHandle,
		oauth2const.ClaimUserType:
		return true
	}
	return false
}

// collectConfiguredUserAttributes returns the distinct set of user attribute names explicitly
// configured across assertion, access token, ID token, and userinfo configs.
func collectConfiguredUserAttributes(
	assertion *inboundmodel.AssertionConfig,
	oauthProfile *providers.OAuthProfile,
) map[string]bool {
	attrs := make(map[string]bool)
	for _, list := range configuredUserAttributeLists(assertion, oauthProfile) {
		for _, attr := range *list {
			attrs[attr] = true
		}
	}
	return attrs
}

// configuredUserAttributeLists returns pointers to the user attribute allow-lists present in the
// assertion, access token, ID token, and userinfo configs, so callers can read or rewrite them.
func configuredUserAttributeLists(
	assertion *inboundmodel.AssertionConfig,
	oauthProfile *providers.OAuthProfile,
) []*[]string {
	lists := make([]*[]string, 0, 4)
	if assertion != nil {
		lists = append(lists, &assertion.UserAttributes)
	}
	if oauthProfile != nil {
		if oauthProfile.Token != nil {
			if oauthProfile.Token.AccessToken != nil && oauthProfile.Token.AccessToken.UserConfig != nil {
				lists = append(lists, &oauthProfile.Token.AccessToken.UserConfig.Attributes)
			}
			if oauthProfile.Token.IDToken != nil {
				lists = append(lists, &oauthProfile.Token.IDToken.UserAttributes)
			}
		}
		if oauthProfile.UserInfo != nil {
			lists = append(lists, &oauthProfile.UserInfo.UserAttributes)
		}
	}
	return lists
}

// applyInboundDefaults fills default values for assertion, OAuth tokens, user info, and scope claims.
func applyInboundDefaults(c *inboundmodel.InboundClient, oauthProfile *providers.OAuthProfile) {
	if c != nil {
		c.Assertion = resolveAssertion(c.Assertion, getDefaultAssertionFromDeployment())
	}
	if oauthProfile == nil {
		return
	}
	var assertion *inboundmodel.AssertionConfig
	if c != nil {
		assertion = c.Assertion
	}
	accessToken, idToken, refreshToken := resolveOAuthTokens(oauthProfile.Token, assertion)
	oauthProfile.Token = &providers.OAuthTokenConfig{
		AccessToken:  accessToken,
		IDToken:      idToken,
		RefreshToken: refreshToken,
		IDJAG:        resolveIDJAG(oauthProfile.Token),
	}
	oauthProfile.UserInfo = resolveUserInfo(oauthProfile.UserInfo, idToken)
}

// seedScopeClaims gives a client the standard OIDC mapping when it is created without one; a
// supplied mapping is stored exactly as sent. Create-only, so a scope removed later stays removed.
// Declarative files are excluded by design: they are explicit configuration and get no defaults.
// Reports whether the defaults were applied, so the caller can tell them apart from a mapping the
// client supplied.
func seedScopeClaims(oauthProfile *providers.OAuthProfile) bool {
	if oauthProfile == nil || len(oauthProfile.ScopeClaims) > 0 {
		return false
	}
	oauthProfile.ScopeClaims = oauthutils.DefaultScopeClaims()
	return true
}

// getDefaultAssertionFromDeployment returns the assertion config from the deployment-level JWT settings.
func getDefaultAssertionFromDeployment() *inboundmodel.AssertionConfig {
	jwtConfig := config.GetServerRuntime().Config.JWT
	return &inboundmodel.AssertionConfig{ValidityPeriod: jwtConfig.ValidityPeriod}
}

// resolveAssertion merges the input assertion config with the deployment default.
func resolveAssertion(input, deploymentDefault *inboundmodel.AssertionConfig) *inboundmodel.AssertionConfig {
	var assertion *inboundmodel.AssertionConfig
	switch {
	case input != nil:
		assertion = &inboundmodel.AssertionConfig{
			ValidityPeriod: input.ValidityPeriod,
			UserAttributes: input.UserAttributes,
		}
		if assertion.ValidityPeriod == 0 && deploymentDefault != nil {
			assertion.ValidityPeriod = deploymentDefault.ValidityPeriod
		}
	case deploymentDefault != nil:
		assertion = &inboundmodel.AssertionConfig{
			ValidityPeriod: deploymentDefault.ValidityPeriod,
			UserAttributes: deploymentDefault.UserAttributes,
		}
	default:
		assertion = &inboundmodel.AssertionConfig{}
	}
	if assertion.UserAttributes == nil {
		assertion.UserAttributes = make([]string, 0)
	}
	return assertion
}

// resolveOAuthTokens resolves access token and ID token configs, defaulting to assertion settings.
func resolveOAuthTokens(in *providers.OAuthTokenConfig,
	assertion *inboundmodel.AssertionConfig) (*providers.AccessTokenConfig,
	*providers.IDTokenConfig,
	*providers.RefreshTokenConfig) {
	if assertion == nil {
		assertion = &inboundmodel.AssertionConfig{}
	}

	accessToken := &providers.AccessTokenConfig{
		UserConfig: resolveUserAccessTokenSubConfig(in, assertion),
	}
	if in != nil && in.AccessToken != nil {
		accessToken.ClientConfig = in.AccessToken.ClientConfig
		accessToken.DefaultAudience = in.AccessToken.DefaultAudience
	}

	var idToken *providers.IDTokenConfig
	if in != nil && in.IDToken != nil {
		idToken = &providers.IDTokenConfig{
			ValidityPeriod: in.IDToken.ValidityPeriod,
			UserAttributes: in.IDToken.UserAttributes,
			ResponseType:   in.IDToken.ResponseType,
			SigningAlg:     in.IDToken.SigningAlg,
			EncryptionAlg:  in.IDToken.EncryptionAlg,
			EncryptionEnc:  in.IDToken.EncryptionEnc,
		}
	}
	if idToken != nil {
		if idToken.ValidityPeriod == 0 {
			idToken.ValidityPeriod = assertion.ValidityPeriod
		}
		if idToken.UserAttributes == nil {
			idToken.UserAttributes = make([]string, 0)
		}
	} else {
		idToken = &providers.IDTokenConfig{
			ValidityPeriod: assertion.ValidityPeriod,
			UserAttributes: assertion.UserAttributes,
		}
	}

	var refreshToken *providers.RefreshTokenConfig
	if in != nil && in.RefreshToken != nil {
		refreshToken = &providers.RefreshTokenConfig{
			ValidityPeriod: in.RefreshToken.ValidityPeriod,
		}
	}

	refreshTokenValidity := config.GetServerRuntime().Config.OAuth.RefreshToken.ValidityPeriod

	if refreshToken != nil {
		if refreshToken.ValidityPeriod == 0 {
			refreshToken.ValidityPeriod = refreshTokenValidity
		}
	} else {
		refreshToken = &providers.RefreshTokenConfig{
			ValidityPeriod: refreshTokenValidity,
		}
	}

	return accessToken, idToken, refreshToken
}

// resolveUserAccessTokenSubConfig resolves the user-subject access token sub-config, defaulting
// validity period and attributes from the assertion config when not explicitly set — preserving
// the pre-split AccessTokenConfig defaulting behavior. ClientConfig has no equivalent default:
// it is new and only ever set when the caller explicitly provides it.
func resolveUserAccessTokenSubConfig(
	in *providers.OAuthTokenConfig, assertion *inboundmodel.AssertionConfig,
) *providers.AccessTokenSubConfig {
	var userConfig *providers.AccessTokenSubConfig
	if in != nil && in.AccessToken != nil && in.AccessToken.UserConfig != nil {
		userConfig = &providers.AccessTokenSubConfig{
			ValidityPeriod: in.AccessToken.UserConfig.ValidityPeriod,
			Attributes:     in.AccessToken.UserConfig.Attributes,
		}
	}
	if userConfig != nil {
		if userConfig.ValidityPeriod == 0 {
			userConfig.ValidityPeriod = assertion.ValidityPeriod
		}
		if userConfig.Attributes == nil {
			userConfig.Attributes = make([]string, 0)
		}
	} else {
		userConfig = &providers.AccessTokenSubConfig{
			ValidityPeriod: assertion.ValidityPeriod,
			Attributes:     assertion.UserAttributes,
		}
	}
	return userConfig
}

// resolveIDJAG resolves the ID-JAG config, defaulting the validity period when unset. Returns nil when
// the application has no ID-JAG block configured, which disables ID-JAG issuance for the application.
func resolveIDJAG(in *providers.OAuthTokenConfig) *providers.IDJAGConfig {
	if in == nil || in.IDJAG == nil {
		return nil
	}
	validityPeriod := in.IDJAG.ValidityPeriod
	if validityPeriod <= 0 {
		validityPeriod = providers.DefaultIDJAGValidityPeriod
	}
	return &providers.IDJAGConfig{
		Enabled:          in.IDJAG.Enabled,
		AllowedAudiences: in.IDJAG.AllowedAudiences,
		ValidityPeriod:   validityPeriod,
	}
}

// resolveUserInfo resolves user info config, defaulting user attributes to the ID token config.
func resolveUserInfo(in *providers.UserInfoConfig,
	idToken *providers.IDTokenConfig) *providers.UserInfoConfig {
	out := &providers.UserInfoConfig{}
	if in != nil {
		out.UserAttributes = in.UserAttributes
		out.ResponseType = in.ResponseType
		out.SigningAlg = in.SigningAlg
		out.EncryptionAlg = in.EncryptionAlg
		out.EncryptionEnc = in.EncryptionEnc
	}
	// Safe to default: validateUserInfoConfig rejects any config where algo fields are set without
	// an explicit responseType, so an empty responseType here means no crypto intent to preserve.
	if out.ResponseType == "" {
		out.ResponseType = providers.UserInfoResponseTypeJSON
	}
	if out.UserAttributes == nil && idToken != nil {
		out.UserAttributes = idToken.UserAttributes
	}
	return out
}

// reconcileReferencedFlows walks call-node targets reachable from the inbound client's configured
// flows and reconciles the client's flow bindings before persistence.
// This mutates the client in place. It is intended for the create/update paths; the flow-update
// revalidation path uses validateReferencedFlows instead, which never mutates.
func (s *inboundClientService) reconcileReferencedFlows(
	ctx context.Context, c *inboundmodel.InboundClient) error {
	return s.walkReferencedFlows(ctx, c, true)
}

// validateReferencedFlows verifies that call-node targets reachable from the inbound client's
// configured flows do not contradict the client's existing flow bindings. An unset field on the
// client is treated as OK — no auto-fill happens here.
func (s *inboundClientService) validateReferencedFlows(
	ctx context.Context, c *inboundmodel.InboundClient) error {
	return s.walkReferencedFlows(ctx, c, false)
}

// walkReferencedFlows walks call-node targets reachable from the inbound client's configured
// flows and either reconciles or validates the client's flow bindings, depending on the reconcile flag.
// If reconcile is true, the client is mutated in place to auto-fill unset flow IDs and disable the
// corresponding flow flags. If reconcile is false, the function only validates and returns errors for
// mismatches without mutating the client.
func (s *inboundClientService) walkReferencedFlows(
	ctx context.Context, c *inboundmodel.InboundClient, reconcile bool) error {
	if s.flowMgt == nil {
		return nil
	}

	starts := []struct {
		id       string
		flowType providers.FlowType
	}{
		{c.AuthFlowID, providers.FlowTypeAuthentication},
		{c.RegistrationFlowID, providers.FlowTypeRegistration},
		{c.RecoveryFlowID, providers.FlowTypeRecovery},
		{c.SignOutFlowID, providers.FlowTypeSignOut},
	}
	for _, start := range starts {
		if start.id == "" {
			continue
		}

		targets, svcErr := s.flowMgt.GetReachableCallTargets(ctx, start.id)
		if svcErr != nil {
			if svcErr.Type == tidcommon.ClientErrorType {
				switch start.flowType {
				case providers.FlowTypeAuthentication:
					return ErrFKInvalidAuthFlow
				case providers.FlowTypeRegistration:
					return ErrFKInvalidRegistrationFlow
				case providers.FlowTypeRecovery:
					return ErrFKInvalidRecoveryFlow
				case providers.FlowTypeSignOut:
					return ErrFKInvalidSignOutFlow
				}
			}
			return ErrFKFlowServerError
		}

		for _, t := range targets {
			// Same-type CALL is subroutine composition, not an alternate entry point for
			// that type. So it never conflicts with the inbound client's per-type binding.
			if t.FlowType == start.flowType {
				continue
			}

			var expected string
			switch t.FlowType {
			case providers.FlowTypeAuthentication:
				expected = c.AuthFlowID
			case providers.FlowTypeRegistration:
				expected = c.RegistrationFlowID
			case providers.FlowTypeRecovery:
				expected = c.RecoveryFlowID
			case providers.FlowTypeSignOut:
				expected = c.SignOutFlowID
			default:
				continue
			}

			if expected == "" {
				// The inbound client has no binding for this type. On the reconcile path (create/update),
				// we auto-fill the sign-out binding with the reachable target so the FK graph stays
				// complete. Registration and recovery are never auto-filled: if the caller left the
				// flag disabled, we must not persist a binding that will surface later as a phantom
				// configuration and clash with a future auth-flow change.
				if !reconcile {
					continue
				}
				if t.FlowType == providers.FlowTypeSignOut {
					c.SignOutFlowID = t.FlowID
				}
				continue
			}

			if t.FlowID != expected {
				return &FlowMismatchError{
					SourceFlowType: start.flowType,
					FlowType:       t.FlowType,
					msg: fmt.Sprintf("configured %s flow invokes a %s flow that is not the configured %s flow",
						strings.ToLower(string(start.flowType)),
						strings.ToLower(string(t.FlowType)),
						strings.ToLower(string(t.FlowType))),
				}
			}
		}
	}

	return nil
}
