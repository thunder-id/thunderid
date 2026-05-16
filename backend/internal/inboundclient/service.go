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

package inboundclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/thunder-id/thunderid/internal/cert"
	"github.com/thunder-id/thunderid/internal/consent"
	layoutmgt "github.com/thunder-id/thunderid/internal/design/layout/mgt"
	thememgt "github.com/thunder-id/thunderid/internal/design/theme/mgt"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	flowcommon "github.com/thunder-id/thunderid/internal/flow/common"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// InboundClientServiceInterface is the public API of the inbound client subsystem.
type InboundClientServiceInterface interface {
	// CreateInboundClient validates and persists a new inbound auth profile, certificates, and OAuth config.
	CreateInboundClient(ctx context.Context, client *inboundmodel.InboundClient, appCert *inboundmodel.Certificate,
		oauthProfile *inboundmodel.OAuthProfile, hasClientSecret bool, entityName string) error
	// GetInboundClientByEntityID returns the inbound client for the given entity.
	GetInboundClientByEntityID(ctx context.Context, entityID string) (*inboundmodel.InboundClient, error)
	// GetInboundClientList returns all inbound clients.
	GetInboundClientList(ctx context.Context) ([]inboundmodel.InboundClient, error)
	// UpdateInboundClient validates and persists updates to an inbound client, certificates, and OAuth config.
	UpdateInboundClient(ctx context.Context, client *inboundmodel.InboundClient,
		appCert *inboundmodel.Certificate, oauthProfile *inboundmodel.OAuthProfile,
		hasClientSecret bool, oauthClientID string, entityName string) error
	// DeleteInboundClient removes the inbound client, OAuth profile, and certificates for the given entity.
	DeleteInboundClient(ctx context.Context, entityID string) error
	// Validate resolves flow defaults and validates FK constraints and OAuth profile without persisting.
	Validate(ctx context.Context, client *inboundmodel.InboundClient,
		oauthProfile *inboundmodel.OAuthProfile, hasClientSecret bool) error

	// GetOAuthProfileByEntityID returns the stored OAuth profile for the given entity.
	GetOAuthProfileByEntityID(ctx context.Context, entityID string) (*inboundmodel.OAuthProfile, error)
	// GetOAuthClientByClientID resolves a full OAuthClient by its public client_id.
	GetOAuthClientByClientID(ctx context.Context, clientID string) (*inboundmodel.OAuthClient, error)

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
	transactioner  transaction.Transactioner
	certService    cert.CertificateServiceInterface
	entityProvider entityprovider.EntityProviderInterface
	themeMgt       thememgt.ThemeMgtServiceInterface
	layoutMgt      layoutmgt.LayoutMgtServiceInterface
	flowMgt        flowmgt.FlowMgtServiceInterface
	entityType     entitytype.EntityTypeServiceInterface
	consentService consent.ConsentServiceInterface
	logger         *log.Logger
}

// newInboundClientService creates and returns an inboundClientService with all dependencies wired.
func newInboundClientService(store inboundClientStoreInterface, transactioner transaction.Transactioner,
	certService cert.CertificateServiceInterface,
	entityProvider entityprovider.EntityProviderInterface,
	themeMgt thememgt.ThemeMgtServiceInterface,
	layoutMgt layoutmgt.LayoutMgtServiceInterface,
	flowMgt flowmgt.FlowMgtServiceInterface,
	entityType entitytype.EntityTypeServiceInterface,
	consentService consent.ConsentServiceInterface,
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
		consentService: consentService,
		logger:         log.GetLogger().With(log.String(log.LoggerKeyComponentName, "InboundClientService")),
	}
}

// CreateInboundClient validates and persists a new inbound auth profile, certificates, and OAuth config.
func (s *inboundClientService) CreateInboundClient(ctx context.Context, client *inboundmodel.InboundClient,
	appCert *inboundmodel.Certificate, oauthProfile *inboundmodel.OAuthProfile,
	hasClientSecret bool, entityName string) error {
	if client == nil {
		return fmt.Errorf("inbound client is required")
	}
	if client.ID != "" && s.store.IsDeclarative(ctx, client.ID) {
		return ErrCannotModifyDeclarative
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
		if vErr := validateOAuthProfile(oauthProfile, hasClientSecret); vErr != nil {
			return vErr
		}
	}
	applyInboundDefaults(client, oauthProfile)
	oauthClientID := s.resolveClientID(client.ID)
	return s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if _, vErr, opErr := s.createCertificate(
			txCtx, cert.CertificateReferenceTypeApplication, client.ID, appCert,
		); vErr != nil {
			return vErr
		} else if opErr != nil {
			return opErr
		}
		if err := s.store.CreateInboundClient(txCtx, *client); err != nil {
			return err
		}
		if oauthProfile != nil {
			if oauthProfile.Certificate != nil && oauthClientID != "" {
				if _, vErr, opErr := s.createCertificate(
					txCtx, cert.CertificateReferenceTypeOAuthApp, oauthClientID, oauthProfile.Certificate,
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
		if s.consentService != nil && s.consentService.IsEnabled() {
			if err := s.syncConsentOnCreate(txCtx, client.ID, entityName, client, oauthProfile); err != nil {
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

// UpdateInboundClient validates and persists updates to an inbound client, certificates, and OAuth config.
func (s *inboundClientService) UpdateInboundClient(ctx context.Context, client *inboundmodel.InboundClient,
	appCert *inboundmodel.Certificate, oauthProfile *inboundmodel.OAuthProfile,
	hasClientSecret bool, oauthClientID string, entityName string) error {
	if client == nil {
		return fmt.Errorf("inbound client is required")
	}
	if s.store.IsDeclarative(ctx, client.ID) {
		return ErrCannotModifyDeclarative
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
		if vErr := validateOAuthProfile(oauthProfile, hasClientSecret); vErr != nil {
			return vErr
		}
	}
	applyInboundDefaults(client, oauthProfile)
	// Capture existing OAuth client_id before the caller updates entity system attributes.
	oldOAuthClientID := s.resolveClientID(client.ID)
	return s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if _, vErr, opErr := s.syncCertificate(
			txCtx, cert.CertificateReferenceTypeApplication, client.ID, appCert,
		); vErr != nil {
			return vErr
		} else if opErr != nil {
			return opErr
		}
		if err := s.store.UpdateInboundClient(txCtx, *client); err != nil {
			return err
		}
		// Clean up the previous OAuth-app cert when the client_id changed or OAuth was removed.
		if oldOAuthClientID != "" && oldOAuthClientID != oauthClientID {
			if opErr := s.deleteCertificate(
				txCtx, cert.CertificateReferenceTypeOAuthApp, oldOAuthClientID,
			); opErr != nil {
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
				txCtx, cert.CertificateReferenceTypeOAuthApp, oauthClientID, oauthCert,
			); vErr != nil {
				return vErr
			} else if opErr != nil {
				return opErr
			}
		}
		if s.consentService != nil && s.consentService.IsEnabled() {
			if err := s.syncConsentOnUpdate(txCtx, client.ID, entityName, client, oauthProfile); err != nil {
				return err
			}
		}
		return s.syncOAuthProfile(txCtx, client.ID, oauthProfile)
	})
}

// Validate resolves flow defaults and validates FK constraints and OAuth profile without persisting.
func (s *inboundClientService) Validate(ctx context.Context, client *inboundmodel.InboundClient,
	oauthProfile *inboundmodel.OAuthProfile, hasClientSecret bool) error {
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
		if vErr := validateOAuthProfile(oauthProfile, hasClientSecret); vErr != nil {
			return vErr
		}
	}
	return nil
}

// resolveClientID returns the OAuth client_id from an entity's system attributes, or "" if absent.
func (s *inboundClientService) resolveClientID(entityID string) string {
	if s.entityProvider == nil {
		return ""
	}
	entity, epErr := s.entityProvider.GetEntity(entityID)
	if epErr != nil {
		s.logger.Warn("Failed to resolve OAuth client_id from entity provider",
			log.String("entityID", entityID), log.Error(epErr))
		return ""
	}
	if entity == nil {
		return ""
	}
	var attrs map[string]interface{}
	if err := json.Unmarshal(entity.SystemAttributes, &attrs); err != nil || attrs == nil {
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
	oauthClientID := s.resolveClientID(entityID)
	return s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if s.consentService != nil && s.consentService.IsEnabled() {
			if err := s.syncDeleteConsent(txCtx, entityID); err != nil {
				return err
			}
		}
		if err := s.store.DeleteInboundClient(txCtx, entityID); err != nil {
			return err
		}
		if opErr := s.deleteCertificate(txCtx, cert.CertificateReferenceTypeApplication, entityID); opErr != nil {
			if opErr.Underlying == nil || opErr.Underlying.Code != cert.ErrorCertificateNotFound.Code {
				return opErr
			}
		}
		if oauthClientID != "" {
			if opErr := s.deleteCertificate(txCtx, cert.CertificateReferenceTypeOAuthApp, oauthClientID); opErr != nil {
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
	*inboundmodel.OAuthProfile, error) {
	return s.store.GetOAuthProfileByEntityID(ctx, entityID)
}

// syncOAuthProfile creates, updates, or deletes the stored OAuth profile to match the desired state.
func (s *inboundClientService) syncOAuthProfile(ctx context.Context, entityID string,
	desired *inboundmodel.OAuthProfile) error {
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
	*inboundmodel.OAuthClient, error) {
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
	entity, epErr := s.entityProvider.GetEntity(entityID)
	if epErr != nil {
		if epErr.Code == entityprovider.ErrorCodeEntityNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to load entity for client_id: %w", epErr)
	}
	ouID := entity.OUID

	oauthProfile, err := s.store.GetOAuthProfileByEntityID(ctx, entityID)
	if err != nil && !errors.Is(err, ErrInboundClientNotFound) {
		return nil, err
	}
	if oauthProfile == nil {
		return nil, nil
	}

	client := BuildOAuthClient(entityID, clientID, ouID, oauthProfile)

	certificate, opErr := s.GetCertificate(ctx, cert.CertificateReferenceTypeOAuthApp, clientID)
	if opErr != nil {
		return nil, opErr
	}
	client.Certificate = certificate

	return client, nil
}

// BuildOAuthClient assembles an OAuthClient from a stored OAuthProfile and entity context.
func BuildOAuthClient(entityID, clientID, ouID string, p *inboundmodel.OAuthProfile) *inboundmodel.OAuthClient {
	client := &inboundmodel.OAuthClient{
		ID:                                 entityID,
		OUID:                               ouID,
		ClientID:                           clientID,
		RedirectURIs:                       p.RedirectURIs,
		TokenEndpointAuthMethod:            oauth2const.TokenEndpointAuthMethod(p.TokenEndpointAuthMethod),
		PKCERequired:                       p.PKCERequired,
		PublicClient:                       p.PublicClient,
		RequirePushedAuthorizationRequests: p.RequirePushedAuthorizationRequests,
		DPoPBoundAccessTokens:              p.DPoPBoundAccessTokens,
		Scopes:                             p.Scopes,
		ScopeClaims:                        p.ScopeClaims,
		Token:                              p.Token,
		UserInfo:                           p.UserInfo,
		Certificate:                        p.Certificate,
		AcrValues:                          p.AcrValues,
	}
	for _, gt := range p.GrantTypes {
		client.GrantTypes = append(client.GrantTypes, oauth2const.GrantType(gt))
	}
	for _, rt := range p.ResponseTypes {
		client.ResponseTypes = append(client.ResponseTypes, oauth2const.ResponseType(rt))
	}
	return client
}

// resolveFlowDefaults fills AuthFlowID, RegistrationFlowID, and RecoveryFlowID with system
// defaults when empty, using the auth flow's handle to locate matching flows of each type.
func (s *inboundClientService) resolveFlowDefaults(ctx context.Context, c *inboundmodel.InboundClient) error {
	if s.flowMgt == nil || c == nil {
		return nil
	}
	if c.AuthFlowID == "" {
		defaultHandle := config.GetServerRuntime().Config.Flow.DefaultAuthFlowHandle
		flow, svcErr := s.flowMgt.GetFlowByHandle(ctx, defaultHandle, flowcommon.FlowTypeAuthentication)
		if svcErr != nil {
			if svcErr.Type == serviceerror.ServerErrorType {
				return ErrFKFlowServerError
			}
			return ErrFKFlowDefinitionRetrievalFailed
		}
		c.AuthFlowID = flow.ID
	}
	if c.RegistrationFlowID == "" && c.AuthFlowID != "" && config.GetServerRuntime().Config.Flow.AutoInferRegistration {
		authFlow, svcErr := s.flowMgt.GetFlow(ctx, c.AuthFlowID)
		if svcErr != nil {
			if svcErr.Type == serviceerror.ServerErrorType {
				return ErrFKFlowServerError
			}
			return ErrFKFlowDefinitionRetrievalFailed
		}
		regFlow, svcErr := s.flowMgt.GetFlowByHandle(ctx, authFlow.Handle, flowcommon.FlowTypeRegistration)
		if svcErr != nil {
			if svcErr.Type == serviceerror.ServerErrorType {
				return ErrFKFlowServerError
			}
			return ErrFKFlowDefinitionRetrievalFailed
		}
		c.RegistrationFlowID = regFlow.ID
	}
	if c.RecoveryFlowID == "" {
		// If a recovery flow is not defined, disable recovery flow for the application.
		c.IsRecoveryFlowEnabled = false
	}
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

// createCertificate validates and creates a new certificate record.
func (s *inboundClientService) createCertificate(ctx context.Context, refType cert.CertificateReferenceType,
	refID string, in *inboundmodel.Certificate) (*inboundmodel.Certificate, error, *CertOperationError) {
	c, vErr := validateCertificateInput(refType, refID, "", in)
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

// syncCertificate creates, updates, or deletes the certificate to match the desired state.
func (s *inboundClientService) syncCertificate(ctx context.Context, refType cert.CertificateReferenceType,
	refID string, in *inboundmodel.Certificate) (*inboundmodel.Certificate, error, *CertOperationError) {
	existing, svcErr := s.certService.GetCertificateByReference(ctx, refType, refID)
	if svcErr != nil && svcErr.Code != cert.ErrorCertificateNotFound.Code {
		return nil, nil, &CertOperationError{Operation: CertOpRetrieve, RefType: refType, Underlying: svcErr}
	}

	existingID := ""
	if existing != nil {
		existingID = existing.ID
	}
	desired, vErr := validateCertificateInput(refType, refID, existingID, in)
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

// deleteCertificate removes the certificate for the given reference type and ID.
func (s *inboundClientService) deleteCertificate(ctx context.Context, refType cert.CertificateReferenceType,
	refID string) *CertOperationError {
	if s.certService == nil {
		return nil
	}
	if svcErr := s.certService.DeleteCertificateByReference(ctx, refType, refID); svcErr != nil {
		return &CertOperationError{Operation: CertOpDelete, RefType: refType, Underlying: svcErr}
	}
	return nil
}

// validateCertificateInput validates and maps inbound certificate input to a cert.Certificate.
func validateCertificateInput(refType cert.CertificateReferenceType,
	refID, existingCertID string, in *inboundmodel.Certificate) (*cert.Certificate, error) {
	if in == nil || in.Type == "" {
		return nil, nil
	}
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
func validateOAuthProfile(p *inboundmodel.OAuthProfile, hasClientSecret bool) error {
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
	if err := validateUserInfoConfig(p); err != nil {
		return err
	}
	if err := validateIDTokenConfig(p); err != nil {
		return err
	}
	return nil
}

// validateUserInfoConfig validates the UserInfo signing and encryption configuration.
func validateUserInfoConfig(p *inboundmodel.OAuthProfile) error {
	if p.UserInfo == nil {
		return nil
	}
	cfg := p.UserInfo

	if cfg.SigningAlg != "" && !slices.Contains(inboundmodel.SupportedUserInfoSigningAlgs, cfg.SigningAlg) {
		return ErrOAuthUserInfoUnsupportedSigningAlg
	}

	if cfg.EncryptionEnc != "" && cfg.EncryptionAlg == "" {
		return ErrOAuthUserInfoEncryptionEncRequiresAlg
	}

	if cfg.EncryptionAlg != "" {
		if !slices.Contains(inboundmodel.SupportedUserInfoEncryptionAlgs, cfg.EncryptionAlg) {
			return ErrOAuthUserInfoUnsupportedEncryptionAlg
		}
		if cfg.EncryptionEnc == "" {
			return ErrOAuthUserInfoEncryptionAlgRequiresEnc
		}
		if !slices.Contains(inboundmodel.SupportedUserInfoEncryptionEncs, cfg.EncryptionEnc) {
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
		case inboundmodel.UserInfoResponseTypeJWS:
			if cfg.SigningAlg == "" {
				return ErrOAuthUserInfoJWSRequiresSigningAlg
			}
		case inboundmodel.UserInfoResponseTypeJWE:
			if cfg.EncryptionAlg == "" || cfg.EncryptionEnc == "" {
				return ErrOAuthUserInfoJWERequiresEncryption
			}
		case inboundmodel.UserInfoResponseTypeNESTEDJWT:
			if cfg.SigningAlg == "" || cfg.EncryptionAlg == "" || cfg.EncryptionEnc == "" {
				return ErrOAuthUserInfoNestedJWTRequiresAll
			}
		case inboundmodel.UserInfoResponseTypeJSON:
			// no additional requirements
		default:
			return ErrOAuthUserInfoUnsupportedResponseType
		}
	}
	return nil
}

// validateIDTokenConfig validates the ID token configuration.
// responseType is the authoritative field; empty defaults to JWT.
func validateIDTokenConfig(p *inboundmodel.OAuthProfile) error {
	if p.Token == nil || p.Token.IDToken == nil {
		return nil
	}
	cfg := p.Token.IDToken

	if cfg.ResponseType == "" {
		cfg.ResponseType = inboundmodel.IDTokenResponseTypeJWT
	}

	switch cfg.ResponseType {
	case inboundmodel.IDTokenResponseTypeJWT:
		if cfg.EncryptionAlg != "" || cfg.EncryptionEnc != "" {
			return ErrOAuthIDTokenEncryptionFieldsNotAllowed
		}
	case inboundmodel.IDTokenResponseTypeJWE, inboundmodel.IDTokenResponseTypeNESTEDJWT:
		if cfg.EncryptionAlg == "" || cfg.EncryptionEnc == "" {
			return ErrOAuthIDTokenEncryptionAlgRequiresEnc
		}
		if !slices.Contains(inboundmodel.SupportedIDTokenEncryptionAlgs, cfg.EncryptionAlg) {
			return ErrOAuthIDTokenUnsupportedEncryptionAlg
		}
		if !slices.Contains(inboundmodel.SupportedIDTokenEncryptionEncs, cfg.EncryptionEnc) {
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
func validateRedirectURIs(p *inboundmodel.OAuthProfile) error {
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
		if parsedURI.Scheme == "" || parsedURI.Host == "" {
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
	if slices.Contains(p.GrantTypes, string(oauth2const.GrantTypeAuthorizationCode)) &&
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
func validateGrantAndResponseTypes(p *inboundmodel.OAuthProfile) error {
	for _, grantType := range p.GrantTypes {
		if !oauth2const.GrantType(grantType).IsValid() {
			return ErrOAuthInvalidGrantType
		}
	}
	for _, responseType := range p.ResponseTypes {
		if !oauth2const.ResponseType(responseType).IsValid() {
			return ErrOAuthInvalidResponseType
		}
	}
	if len(p.GrantTypes) == 1 &&
		slices.Contains(p.GrantTypes, string(oauth2const.GrantTypeClientCredentials)) &&
		len(p.ResponseTypes) > 0 {
		return ErrOAuthClientCredentialsCannotUseResponseTypes
	}
	if slices.Contains(p.GrantTypes, string(oauth2const.GrantTypeAuthorizationCode)) {
		if len(p.ResponseTypes) == 0 ||
			!slices.Contains(p.ResponseTypes, string(oauth2const.ResponseTypeCode)) {
			return ErrOAuthAuthCodeRequiresCodeResponseType
		}
	}
	if len(p.GrantTypes) == 1 &&
		slices.Contains(p.GrantTypes, string(oauth2const.GrantTypeRefreshToken)) {
		return ErrOAuthRefreshTokenCannotBeSoleGrant
	}
	if p.PKCERequired &&
		!slices.Contains(p.GrantTypes, string(oauth2const.GrantTypeAuthorizationCode)) {
		return ErrOAuthPKCERequiresAuthCode
	}
	if len(p.ResponseTypes) > 0 &&
		!slices.Contains(p.GrantTypes, string(oauth2const.GrantTypeAuthorizationCode)) {
		return ErrOAuthResponseTypesRequireAuthCode
	}
	return nil
}

// validateTokenEndpointAuthMethod validates the token endpoint auth method against cert and secret state.
func validateTokenEndpointAuthMethod(p *inboundmodel.OAuthProfile, hasClientSecret bool) error {
	method := oauth2const.TokenEndpointAuthMethod(p.TokenEndpointAuthMethod)
	if !method.IsValid() {
		return ErrOAuthInvalidTokenEndpointAuthMethod
	}
	hasCert := p.Certificate != nil && p.Certificate.Type != ""
	userInfoNeedsCert := p.UserInfo != nil && p.UserInfo.EncryptionAlg != ""
	idTokenNeedsCert := p.Token != nil && p.Token.IDToken != nil &&
		(p.Token.IDToken.ResponseType == inboundmodel.IDTokenResponseTypeJWE ||
			p.Token.IDToken.ResponseType == inboundmodel.IDTokenResponseTypeNESTEDJWT)
	needsCert := userInfoNeedsCert || idTokenNeedsCert

	switch method {
	case oauth2const.TokenEndpointAuthMethodPrivateKeyJWT:
		if !hasCert {
			return ErrOAuthPrivateKeyJWTRequiresCertificate
		}
		if hasClientSecret {
			return ErrOAuthPrivateKeyJWTCannotHaveClientSecret
		}
	case oauth2const.TokenEndpointAuthMethodClientSecretBasic, oauth2const.TokenEndpointAuthMethodClientSecretPost:
		if hasCert && !needsCert {
			return ErrOAuthClientSecretCannotHaveCertificate
		}
	case oauth2const.TokenEndpointAuthMethodNone:
		if !p.PublicClient {
			return ErrOAuthNoneAuthRequiresPublicClient
		}
		if (hasCert && !needsCert) || hasClientSecret {
			return ErrOAuthNoneAuthCannotHaveCertOrSecret
		}
		if slices.Contains(p.GrantTypes, string(oauth2const.GrantTypeClientCredentials)) {
			return ErrOAuthClientCredentialsCannotUseNoneAuth
		}
	}
	return nil
}

// validatePublicClient validates constraints required for public clients.
func validatePublicClient(p *inboundmodel.OAuthProfile) error {
	if oauth2const.TokenEndpointAuthMethod(p.TokenEndpointAuthMethod) != oauth2const.TokenEndpointAuthMethodNone {
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
	if err := s.validateThemeID(c.ThemeID); err != nil {
		return err
	}
	if err := s.validateLayoutID(c.LayoutID); err != nil {
		return err
	}
	if err := s.validateAllowedUserTypes(ctx, c.AllowedUserTypes); err != nil {
		return err
	}
	return nil
}

// validateAuthFlowID validates that the auth flow ID exists and is of the correct type.
func (s *inboundClientService) validateAuthFlowID(ctx context.Context, flowID string) error {
	if flowID == "" || s.flowMgt == nil {
		return nil
	}
	valid, svcErr := s.flowMgt.IsValidFlow(ctx, flowID, flowcommon.FlowTypeAuthentication)
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
	valid, svcErr := s.flowMgt.IsValidFlow(ctx, flowID, flowcommon.FlowTypeRegistration)
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
	valid, svcErr := s.flowMgt.IsValidFlow(ctx, flowID, flowcommon.FlowTypeRecovery)
	if svcErr != nil {
		return ErrFKFlowServerError
	}
	if !valid {
		return ErrFKInvalidRecoveryFlow
	}
	return nil
}

// validateThemeID validates that the theme ID exists.
func (s *inboundClientService) validateThemeID(themeID string) error {
	if themeID == "" || s.themeMgt == nil {
		return nil
	}
	exists, svcErr := s.themeMgt.IsThemeExist(themeID)
	if svcErr != nil || !exists {
		return ErrFKThemeNotFound
	}
	return nil
}

// validateLayoutID validates that the layout ID exists.
func (s *inboundClientService) validateLayoutID(layoutID string) error {
	if layoutID == "" || s.layoutMgt == nil {
		return nil
	}
	exists, svcErr := s.layoutMgt.IsLayoutExist(layoutID)
	if svcErr != nil || !exists {
		return ErrFKLayoutNotFound
	}
	return nil
}

// validateAllowedUserTypes validates that each allowed user type corresponds to an existing user type.
func (s *inboundClientService) validateAllowedUserTypes(
	ctx context.Context, allowedUserTypes []string,
) error {
	if len(allowedUserTypes) == 0 || s.entityType == nil {
		return nil
	}
	existingUserTypes := make(map[string]bool)
	limit := serverconst.MaxPageSize
	offset := 0
	for {
		// Runtime context: skip authorization checks when fetching entity types.
		entityTypeList, svcErr := s.entityType.GetEntityTypeList(
			security.WithRuntimeContext(ctx), entitytype.TypeCategoryUser, limit, offset, false)
		if svcErr != nil {
			s.logger.Error("Failed to retrieve user type list for validation",
				log.String("error", svcErr.Error.DefaultValue), log.String("code", svcErr.Code))
			return ErrUserSchemaLookupFailed
		}
		for _, schema := range entityTypeList.Types {
			existingUserTypes[schema.Name] = true
		}
		if len(entityTypeList.Types) == 0 ||
			offset+len(entityTypeList.Types) >= entityTypeList.TotalResults {
			break
		}
		offset += limit
	}
	for _, userType := range allowedUserTypes {
		if userType == "" || !existingUserTypes[userType] {
			return ErrFKInvalidUserType
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
	oauthProfile *inboundmodel.OAuthProfile,
) error {
	if len(allowedEntityTypes) == 0 || s.entityType == nil {
		return nil
	}

	attrs := collectConfiguredUserAttributes(assertion, oauthProfile)
	if len(attrs) == 0 {
		return nil
	}

	validAttrs := make(map[string]bool)
	for _, entityTypeName := range allowedEntityTypes {
		attrInfos, svcErr := s.entityType.GetAttributes(
			security.WithRuntimeContext(ctx), entitytype.TypeCategoryUser, entityTypeName, false, true, false)
		if svcErr != nil {
			if svcErr.Type == serviceerror.ServerErrorType {
				return ErrUserSchemaLookupFailed
			}
			return ErrFKInvalidUserType
		}
		for _, info := range attrInfos {
			validAttrs[info.Attribute] = true
		}
	}

	for attr := range attrs {
		if isComputedAttribute(attr) {
			continue
		}
		if !validAttrs[attr] {
			return ErrInvalidUserAttribute
		}
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
	oauthProfile *inboundmodel.OAuthProfile,
) map[string]bool {
	attrs := make(map[string]bool)
	if assertion != nil {
		for _, a := range assertion.UserAttributes {
			attrs[a] = true
		}
	}
	if oauthProfile != nil {
		if oauthProfile.Token != nil {
			if oauthProfile.Token.AccessToken != nil {
				for _, a := range oauthProfile.Token.AccessToken.UserAttributes {
					attrs[a] = true
				}
			}
			if oauthProfile.Token.IDToken != nil {
				for _, a := range oauthProfile.Token.IDToken.UserAttributes {
					attrs[a] = true
				}
			}
		}
		if oauthProfile.UserInfo != nil {
			for _, a := range oauthProfile.UserInfo.UserAttributes {
				attrs[a] = true
			}
		}
	}
	return attrs
}

// applyInboundDefaults fills default values for assertion, OAuth tokens, user info, and scope claims.
func applyInboundDefaults(c *inboundmodel.InboundClient, oauthProfile *inboundmodel.OAuthProfile) {
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
	accessToken, idToken := resolveOAuthTokens(oauthProfile.Token, assertion)
	oauthProfile.Token = &inboundmodel.OAuthTokenConfig{AccessToken: accessToken, IDToken: idToken}
	oauthProfile.UserInfo = resolveUserInfo(oauthProfile.UserInfo, idToken)
	oauthProfile.ScopeClaims = resolveScopeClaims(oauthProfile.ScopeClaims)
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
func resolveOAuthTokens(in *inboundmodel.OAuthTokenConfig,
	assertion *inboundmodel.AssertionConfig) (*inboundmodel.AccessTokenConfig, *inboundmodel.IDTokenConfig) {
	if assertion == nil {
		assertion = &inboundmodel.AssertionConfig{}
	}

	var accessToken *inboundmodel.AccessTokenConfig
	if in != nil && in.AccessToken != nil {
		accessToken = &inboundmodel.AccessTokenConfig{
			ValidityPeriod: in.AccessToken.ValidityPeriod,
			UserAttributes: in.AccessToken.UserAttributes,
		}
	}
	if accessToken != nil {
		if accessToken.ValidityPeriod == 0 {
			accessToken.ValidityPeriod = assertion.ValidityPeriod
		}
		if accessToken.UserAttributes == nil {
			accessToken.UserAttributes = make([]string, 0)
		}
	} else {
		accessToken = &inboundmodel.AccessTokenConfig{
			ValidityPeriod: assertion.ValidityPeriod,
			UserAttributes: assertion.UserAttributes,
		}
	}

	var idToken *inboundmodel.IDTokenConfig
	if in != nil && in.IDToken != nil {
		idToken = &inboundmodel.IDTokenConfig{
			ValidityPeriod: in.IDToken.ValidityPeriod,
			UserAttributes: in.IDToken.UserAttributes,
			ResponseType:   in.IDToken.ResponseType,
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
		idToken = &inboundmodel.IDTokenConfig{
			ValidityPeriod: assertion.ValidityPeriod,
			UserAttributes: assertion.UserAttributes,
		}
	}

	return accessToken, idToken
}

// resolveUserInfo resolves user info config, defaulting user attributes to the ID token config.
func resolveUserInfo(in *inboundmodel.UserInfoConfig,
	idToken *inboundmodel.IDTokenConfig) *inboundmodel.UserInfoConfig {
	out := &inboundmodel.UserInfoConfig{}
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
		out.ResponseType = inboundmodel.UserInfoResponseTypeJSON
	}
	if out.UserAttributes == nil && idToken != nil {
		out.UserAttributes = idToken.UserAttributes
	}
	return out
}

// resolveScopeClaims returns the input scope claims map, defaulting to an empty map if nil.
func resolveScopeClaims(in map[string][]string) map[string][]string {
	if in == nil {
		return make(map[string][]string)
	}
	return in
}

// syncConsentOnCreate creates consent purpose elements for a newly registered application.
func (s *inboundClientService) syncConsentOnCreate(ctx context.Context,
	entityID, entityName string, client *inboundmodel.InboundClient, profile *inboundmodel.OAuthProfile) error {
	// TODO: Replace with the entity's actual OU when multi-OU consent is supported.
	const ouID = "default"
	attrMap := extractRequestedAttributesFromInbound(client, profile)
	if len(attrMap) == 0 {
		return nil
	}
	attrs := make([]string, 0, len(attrMap))
	for a := range attrMap {
		attrs = append(attrs, a)
	}
	if err := s.createMissingConsentElements(ctx, ouID, attrs); err != nil {
		return err
	}
	purpose := consent.ConsentPurposeInput{
		Name:        entityName,
		Description: "Consent purpose for application " + entityName,
		GroupID:     entityID,
		Elements:    attributesToPurposeElements(attrMap),
	}
	if _, err := s.consentService.CreateConsentPurpose(ctx, ouID, &purpose); err != nil {
		return s.wrapConsentServiceError(err)
	}
	return nil
}

// syncConsentOnUpdate updates or creates the consent purpose for an existing application.
func (s *inboundClientService) syncConsentOnUpdate(ctx context.Context,
	entityID, entityName string, client *inboundmodel.InboundClient, profile *inboundmodel.OAuthProfile) error {
	// TODO: Replace with the entity's actual OU when multi-OU consent is supported.
	const ouID = "default"
	newAttrs := extractRequestedAttributesFromInbound(client, profile)
	required := make([]string, 0, len(newAttrs))
	for a := range newAttrs {
		required = append(required, a)
	}
	if len(required) > 0 {
		if err := s.createMissingConsentElements(ctx, ouID, required); err != nil {
			return err
		}
	}
	existing, err := s.consentService.ListConsentPurposes(ctx, ouID, entityID)
	if err != nil {
		return s.wrapConsentServiceError(err)
	}
	if len(existing) == 0 {
		if len(newAttrs) > 0 {
			purpose := consent.ConsentPurposeInput{
				Name:        entityName,
				Description: "Consent purpose for application " + entityName,
				GroupID:     entityID,
				Elements:    attributesToPurposeElements(newAttrs),
			}
			if _, createErr := s.consentService.CreateConsentPurpose(ctx, ouID, &purpose); createErr != nil {
				return s.wrapConsentServiceError(createErr)
			}
		}
		return nil
	}
	if len(newAttrs) == 0 {
		return s.syncDeleteConsent(ctx, entityID)
	}
	updated := consent.ConsentPurposeInput{
		Name:        entityName,
		Description: "Consent purpose for application " + entityName,
		GroupID:     entityID,
		Elements:    attributesToPurposeElements(newAttrs),
	}
	if _, updateErr := s.consentService.UpdateConsentPurpose(ctx, ouID, existing[0].ID, &updated); updateErr != nil {
		return s.wrapConsentServiceError(updateErr)
	}
	return nil
}

// syncDeleteConsent removes the consent purpose for the given entity if it exists.
func (s *inboundClientService) syncDeleteConsent(ctx context.Context, entityID string) error {
	// TODO: Replace with the entity's actual OU when multi-OU consent is supported.
	const ouID = "default"
	purposes, err := s.consentService.ListConsentPurposes(ctx, ouID, entityID)
	if err != nil {
		return s.wrapConsentServiceError(err)
	}
	if len(purposes) == 0 {
		return nil
	}
	if delErr := s.consentService.DeleteConsentPurpose(ctx, ouID, purposes[0].ID); delErr != nil {
		if delErr.Code == consent.ErrorDeletingConsentPurposeWithAssociatedRecords.Code {
			s.logger.Warn("Cannot delete consent purpose due to existing consents",
				log.String("entityID", entityID))
			return nil
		}
		return s.wrapConsentServiceError(delErr)
	}
	return nil
}

// createMissingConsentElements creates any consent elements not yet present in the consent service.
func (s *inboundClientService) createMissingConsentElements(ctx context.Context,
	ouID string, names []string) error {
	if len(names) == 0 {
		return nil
	}
	validNames, err := s.consentService.ValidateConsentElements(ctx, ouID, names)
	if err != nil {
		return s.wrapConsentServiceError(err)
	}
	existingMap := make(map[string]bool, len(validNames))
	for _, n := range validNames {
		existingMap[n] = true
	}
	var toCreate []consent.ConsentElementInput
	for _, n := range names {
		if !existingMap[n] {
			toCreate = append(toCreate, consent.ConsentElementInput{
				Name:      n,
				Namespace: consent.NamespaceAttribute,
			})
		}
	}
	if len(toCreate) > 0 {
		if _, createErr := s.consentService.CreateConsentElements(ctx, ouID, toCreate); createErr != nil {
			return s.wrapConsentServiceError(createErr)
		}
	}
	return nil
}

// wrapConsentServiceError wraps a consent service error in a ConsentSyncError.
func (s *inboundClientService) wrapConsentServiceError(err *serviceerror.ServiceError) error {
	if err == nil {
		return nil
	}
	return &ConsentSyncError{Underlying: err}
}

// extractRequestedAttributesFromInbound collects user attributes referenced by the client and profile.
func extractRequestedAttributesFromInbound(
	client *inboundmodel.InboundClient, profile *inboundmodel.OAuthProfile,
) map[string]bool {
	attrMap := make(map[string]bool)
	if client != nil && client.Assertion != nil {
		for _, a := range client.Assertion.UserAttributes {
			attrMap[a] = true
		}
	}
	if profile != nil {
		if profile.Token != nil {
			if profile.Token.AccessToken != nil {
				for _, a := range profile.Token.AccessToken.UserAttributes {
					attrMap[a] = true
				}
			}
			if profile.Token.IDToken != nil {
				for _, a := range profile.Token.IDToken.UserAttributes {
					attrMap[a] = true
				}
			}
		}
		if profile.UserInfo != nil {
			for _, a := range profile.UserInfo.UserAttributes {
				attrMap[a] = true
			}
		}
	}
	return attrMap
}

// attributesToPurposeElements maps an attribute set to consent purpose element inputs.
func attributesToPurposeElements(attributes map[string]bool) []consent.PurposeElement {
	elements := make([]consent.PurposeElement, 0, len(attributes))
	for attr := range attributes {
		elements = append(elements, consent.PurposeElement{
			Name:        attr,
			Namespace:   consent.NamespaceAttribute,
			IsMandatory: false,
		})
	}
	return elements
}
