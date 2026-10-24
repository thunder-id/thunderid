// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package defaultprovider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	authncommon "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/authn/magiclink"
	"github.com/thunder-id/thunderid/internal/authn/openid4vp"
	"github.com/thunder-id/thunderid/internal/authn/otp"
	"github.com/thunder-id/thunderid/internal/authn/passkey"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/identitygovernance"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
)

type defaultAuthnProvider struct {
	entitySvc        entity.EntityServiceInterface
	passkeyService   passkey.PasskeyServiceInterface
	otpService       otp.OTPAuthnServiceInterface
	magicLinkService magiclink.MagicLinkAuthnServiceInterface
	openid4vpService openid4vp.OpenID4VPServiceInterface
	federatedAuths   map[providers.IDPType]authncommon.FederatedAuthenticator
	governance       identitygovernance.IdentityGovernanceServiceInterface
	logger           *log.Logger
}

// newDefaultAuthnProvider creates a new internal user authn provider.
func newDefaultAuthnProvider(entitySvc entity.EntityServiceInterface,
	passkeyService passkey.PasskeyServiceInterface, otpService otp.OTPAuthnServiceInterface,
	magicLinkService magiclink.MagicLinkAuthnServiceInterface,
	openid4vpService openid4vp.OpenID4VPServiceInterface,
	federatedAuths map[providers.IDPType]authncommon.FederatedAuthenticator,
	governance identitygovernance.IdentityGovernanceServiceInterface) providers.AuthnProviderInterface {
	return &defaultAuthnProvider{
		entitySvc:        entitySvc,
		passkeyService:   passkeyService,
		otpService:       otpService,
		magicLinkService: magicLinkService,
		openid4vpService: openid4vpService,
		federatedAuths:   federatedAuths,
		governance:       governance,
		logger:           log.GetLogger().With(log.String(log.LoggerKeyComponentName, "DefaultAuthnProvider")),
	}
}

// InitiateAuthentication initiates the authentication process for a given credential type.
func (p *defaultAuthnProvider) InitiateAuthentication(
	ctx context.Context,
	credentialType string,
	initData any,
	metadata *providers.AuthnMetadata,
) (any, *tidcommon.ServiceError) {
	switch credentialType {
	case authnprovidercm.CredentialTypePasskey:
		// Expect initData to be a PasskeyAuthenticationStartRequest struct
		return p.initiateAuthenticationWithPasskey(ctx, initData)
	default:
		return nil, p.logAndReturnServerError(ctx, "Unsupported credential type for authentication initiation",
			log.String("credentialType", credentialType))
	}
}

// Authenticate authenticates the user using the internal entity service.
func (p *defaultAuthnProvider) Authenticate(
	ctx context.Context,
	identifiers, credentials map[string]interface{},
	metadata *providers.AuthnMetadata,
) (*providers.AuthnResult, *tidcommon.ServiceError) {
	if credentials == nil {
		return nil, newClientError(authnprovidercm.ErrorCodeAuthenticationFailed,
			"Credentials are required", "Credentials are required for authentication")
	}
	res, method, failure, svcErr := p.authenticateWithCredential(ctx, identifiers, credentials)
	if svcErr != nil {
		return nil, p.handleAuthenticationFailure(ctx, method, failure, svcErr)
	}
	result, svcErr := p.buildAuthnResult(ctx, res)
	if svcErr != nil {
		return nil, svcErr
	}
	if svcErr := p.checkAuthenticationAccess(ctx, method, result); svcErr != nil {
		return nil, svcErr
	}
	return result, nil
}

// GetEntityReference retrieves the entity reference using the internal entity service.
func (p *defaultAuthnProvider) GetEntityReference(ctx context.Context, entityReferenceToken any,
) (*providers.EntityReference, *tidcommon.ServiceError) {
	parsedToken, ok := entityReferenceToken.(map[string]interface{})
	if !ok {
		return nil, p.logAndReturnServerError(ctx, "Invalid token format")
	}

	entityResult, svcErr := p.resolveEntityFromToken(ctx, parsedToken, "entity reference token")
	if svcErr != nil {
		return nil, svcErr
	}

	return &providers.EntityReference{
		EntityID:       entityResult.ID,
		EntityCategory: string(entityResult.Category),
		EntityType:     entityResult.Type,
		OUID:           entityResult.OUID,
	}, nil
}

// SearchEntityReferences returns every entity an attribute lookup matches, or none. It answers a
// lookup GetEntityReference has already found ambiguous: the two match differently, so reading it
// for anything else would change which entities a lookup names.
func (p *defaultAuthnProvider) SearchEntityReferences(ctx context.Context,
	filters map[string]interface{}) ([]providers.EntityReference, *tidcommon.ServiceError) {
	entities, err := p.entitySvc.SearchEntities(ctx, filters)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			return nil, nil
		}
		return nil, p.logAndReturnServerError(ctx, "Failed to search entities",
			log.String("error", err.Error()))
	}

	refs := make([]providers.EntityReference, 0, len(entities))
	for _, e := range entities {
		refs = append(refs, providers.EntityReference{
			EntityID:       e.ID,
			EntityCategory: string(e.Category),
			EntityType:     e.Type,
			OUID:           e.OUID,
		})
	}
	return refs, nil
}

// GetAttributes retrieves the user attributes using the internal entity service.
func (p *defaultAuthnProvider) GetAttributes(
	ctx context.Context,
	attributeToken any,
	consentedAttributes *providers.RequestedAttributes,
	metadata *providers.GetAttributesMetadata,
) (*providers.AttributesResponse, *tidcommon.ServiceError) {
	parsedToken, ok := attributeToken.(map[string]interface{})
	if !ok {
		return nil, p.logAndReturnServerError(ctx, "Invalid token format")
	}

	entityResult, svcErr := p.resolveEntityFromToken(ctx, parsedToken, "attribute token")
	if svcErr != nil {
		return nil, svcErr
	}

	var allAttributes map[string]interface{}
	if len(entityResult.Attributes) > 0 {
		if err := json.Unmarshal(entityResult.Attributes, &allAttributes); err != nil {
			return nil, p.logAndReturnServerError(ctx, "Failed to unmarshal entity attributes",
				log.String("error", err.Error()))
		}
	}

	attributesResponse := &providers.AttributesResponse{
		Attributes:    make(map[string]*providers.AttributeResponse),
		Verifications: make(map[string]*providers.VerificationResponse),
	}

	if consentedAttributes != nil && len(consentedAttributes.Attributes) > 0 {
		for attrName := range consentedAttributes.Attributes {
			if val, ok := allAttributes[attrName]; ok {
				attributesResponse.Attributes[attrName] = &providers.AttributeResponse{
					Value: val,
					AssuranceMetadataResponse: &providers.AssuranceMetadataResponse{
						IsVerified:     false,
						VerificationID: "",
					},
				}
			}
		}
	} else {
		for attrName, val := range allAttributes {
			attributesResponse.Attributes[attrName] = &providers.AttributeResponse{
				Value: val,
				AssuranceMetadataResponse: &providers.AssuranceMetadataResponse{
					IsVerified:     false,
					VerificationID: "",
				},
			}
		}
	}

	return attributesResponse, nil
}

// InitiateEnrollment initiates the enrollment process for a given credential type.
func (p *defaultAuthnProvider) InitiateEnrollment(
	ctx context.Context,
	credentialType string,
	initData any,
	metadata *providers.AuthnMetadata) (
	any, *tidcommon.ServiceError) {
	switch credentialType {
	case authnprovidercm.CredentialTypePasskey:
		// Expect initData to be a PasskeyRegistrationStartRequest struct
		return p.initiateEnrollmentWithPasskey(ctx, initData)
	default:
		return nil, p.logAndReturnServerError(ctx, "Unsupported credential type for enrollment initiation",
			log.String("credentialType", credentialType))
	}
}

// Enroll enrolls the user using the internal entity service.
func (p *defaultAuthnProvider) Enroll(
	ctx context.Context,
	identifiers map[string]interface{},
	credentials map[string]interface{},
	metadata *providers.AuthnMetadata,
) (*providers.AuthnResult, *tidcommon.ServiceError) {
	if credentials == nil {
		return nil, newClientError(authnprovidercm.ErrorCodeEnrollmentFailed,
			"Credentials are required", "Credentials are required for enrollment")
	}
	res, svcErr := p.enrollWithCredential(ctx, credentials)
	if svcErr != nil {
		return nil, svcErr
	}
	return p.buildAuthnResult(ctx, res)
}

// StoreAccountLink records a linked account against the entity the caller-supplied token
// names. The token is this provider's own entity reference token, so it resolves the same way
// GetEntityReference resolves one, and a token that names no entity is a client error.
func (p *defaultAuthnProvider) StoreAccountLink(ctx context.Context, entityReferenceToken any,
	idpID, sub string) *tidcommon.ServiceError {
	if idpID == "" || sub == "" {
		return newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Invalid linked account", "A connection id and a subject are both required")
	}

	parsedToken, ok := entityReferenceToken.(map[string]interface{})
	if !ok || parsedToken == nil {
		return newClientError(authnprovidercm.ErrorCodeInvalidToken,
			"Invalid entity reference token", "The provided entity reference token is invalid")
	}

	entityResult, svcErr := p.resolveEntityFromToken(ctx, parsedToken, "entity reference token")
	if svcErr != nil {
		return svcErr
	}
	// A declarative user cannot be written to, so its links can only be declared with it.
	if entityResult.IsReadOnly {
		return newClientError(authnprovidercm.ErrorCodeNotImplemented, "Account link not stored",
			"Account links of a declarative user cannot be stored")
	}

	if err := p.entitySvc.LinkAccount(ctx, entityResult.ID, idpID, sub); err != nil {
		if errors.Is(err, entity.ErrIndexedValueLimitExceeded) {
			return newClientError(authnprovidercm.ErrorCodeInvalidRequest, "Too many linked accounts",
				"The user has reached the limit of linked accounts")
		}
		return p.logAndReturnServerError(ctx, "Failed to link account",
			log.String(log.LoggerKeyIDPID, idpID), log.String("error", err.Error()))
	}
	return nil
}

func (p *defaultAuthnProvider) initiateAuthenticationWithPasskey(
	ctx context.Context, initData any) (any, *tidcommon.ServiceError) {
	req, ok := initData.(*passkey.PasskeyAuthenticationStartRequest)
	if !ok || req == nil {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Invalid passkey authentication init payload", "The provided passkey init data is invalid")
	}
	result, svcErr := p.passkeyService.StartAuthentication(ctx, req)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ClientErrorType {
			return nil, newClientError(authnprovidercm.ErrorCodeAuthenticationFailed,
				svcErr.Error.DefaultValue, svcErr.ErrorDescription.DefaultValue)
		}
		return nil, p.logAndReturnServerError(ctx, "Passkey authentication initiation failed with server error",
			log.String("error", svcErr.Error.DefaultValue),
			log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
	}
	return result, nil
}

func (p *defaultAuthnProvider) initiateEnrollmentWithPasskey(
	ctx context.Context, initData any) (any, *tidcommon.ServiceError) {
	req, ok := initData.(*passkey.PasskeyRegistrationStartRequest)
	if !ok || req == nil {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Invalid passkey enrollment init payload", "The provided passkey init data is invalid")
	}
	result, svcErr := p.passkeyService.StartRegistration(ctx, req)
	if svcErr != nil {
		if stated := passkeyEnrollmentRefusal(svcErr); stated != nil {
			return nil, stated
		}
		if svcErr.Type == tidcommon.ClientErrorType {
			return nil, newClientError(authnprovidercm.ErrorCodeEnrollmentFailed,
				svcErr.Error.DefaultValue, svcErr.ErrorDescription.DefaultValue)
		}
		return nil, p.logAndReturnServerError(ctx, "Passkey enrollment initiation failed with server error",
			log.String("error", svcErr.Error.DefaultValue),
			log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
	}
	return result, nil
}

func (p *defaultAuthnProvider) enrollWithCredential(
	ctx context.Context,
	credentials map[string]interface{},
) (*authncommon.AuthnResult, *tidcommon.ServiceError) {
	if passkeyCredential, ok := credentials[authnprovidercm.CredentialTypePasskey]; ok {
		return p.enrollWithPasskey(ctx, passkeyCredential)
	}
	return nil, p.logAndReturnServerError(ctx, "Unsupported credential type for enrollment",
		log.String("credentialTypes", fmt.Sprintf("%v", reflect.ValueOf(credentials).MapKeys())))
}

// enrollWithPasskey registers the user using the passkey service.
// The raw credential is expected to be a PasskeyRegistrationFinishRequest struct.
func (p *defaultAuthnProvider) enrollWithPasskey(
	ctx context.Context, passkeyCredential interface{},
) (*authncommon.AuthnResult, *tidcommon.ServiceError) {
	req, ok := passkeyCredential.(*passkey.PasskeyRegistrationFinishRequest)
	if !ok || req == nil {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Invalid passkey enrollment finish payload", "The provided passkey enrollment data is invalid")
	}
	result, svcErr := p.passkeyService.FinishRegistration(ctx, req)
	if svcErr != nil {
		if stated := passkeyEnrollmentRefusal(svcErr); stated != nil {
			return nil, stated
		}
		if svcErr.Type == tidcommon.ClientErrorType {
			return nil, newClientError(authnprovidercm.ErrorCodeEnrollmentFailed,
				svcErr.Error.DefaultValue, svcErr.ErrorDescription.DefaultValue)
		}
		return nil, p.logAndReturnServerError(ctx, "Passkey enrollment failed with server error",
			log.String("error", svcErr.Error.DefaultValue),
			log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
	}
	return result, nil
}

// resolveLinkedEntities returns the local users the recorded (connection, subject) link names. Several
// users may hold one link.
func (p *defaultAuthnProvider) resolveLinkedEntities(ctx context.Context,
	token map[string]interface{}) ([]string, *tidcommon.ServiceError) {
	idpID, _ := token[authnprovidercm.UserAttributeFederatedIdpID].(string)
	sub, _ := token[authnprovidercm.UserAttributeSub].(string)

	linkedIDs, err := p.entitySvc.ResolveLinkedAccounts(ctx, idpID, sub)
	if err != nil {
		return nil, p.logAndReturnServerError(ctx, "Failed to resolve linked accounts",
			log.String("error", err.Error()))
	}
	return linkedIDs, nil
}

func (p *defaultAuthnProvider) buildAuthnResult(
	ctx context.Context, authnResult *authncommon.AuthnResult,
) (*providers.AuthnResult, *tidcommon.ServiceError) {
	result := &providers.AuthnResult{
		AuthenticatedClaims: authnResult.AuthenticatedClaims,
	}

	entityID := ""
	if idVal, ok := authnResult.Token[authnprovidercm.UserAttributeUserID]; ok {
		entityID, _ = idVal.(string)
	}
	if entityID == "" && authnprovidercm.IsFederatedToken(authnResult.Token) {
		linkedIDs, svcErr := p.resolveLinkedEntities(ctx, authnResult.Token)
		if svcErr != nil {
			return nil, svcErr
		}
		if len(linkedIDs) != 1 {
			// No single local user for this identity. Hand the token back so the caller can provision
			// and record the link, or let the user pick between the users holding it.
			result.EntityReferenceToken = authnResult.Token
			result.AttributeToken = authnResult.Token
			return result, nil
		}
		entityID = linkedIDs[0]
	}
	if entityID == "" {
		identifiedEntityID, identifyErr := p.entitySvc.IdentifyEntity(ctx, authnResult.Token)
		if identifyErr != nil {
			if errors.Is(identifyErr, entity.ErrEntityNotFound) || errors.Is(identifyErr, entity.ErrAmbiguousEntity) {
				// Entity does not yet exist at provider. Return the entity reference token and attribute token
				// to the caller to retrieve the entity reference and attributes later.
				result.EntityReferenceToken = authnResult.Token
				result.AttributeToken = authnResult.Token
				return result, nil
			}
			return nil, p.logAndReturnServerError(ctx, "Failed to identify entity after authentication",
				log.String("error", identifyErr.Error()))
		}
		entityID = *identifiedEntityID
	}

	res, svcErr := p.buildEntityAuthnResult(ctx, entityID)
	if svcErr != nil {
		return nil, svcErr
	}
	res.AuthenticatedClaims = result.AuthenticatedClaims
	return res, nil
}

// buildEntityAuthnResult builds a resolved AuthnResult (entity reference and attributes) for an entity
// that is known to exist.
func (p *defaultAuthnProvider) buildEntityAuthnResult(
	ctx context.Context, entityID string,
) (*providers.AuthnResult, *tidcommon.ServiceError) {
	entityResult, getErr := p.entitySvc.GetEntityProfile(ctx, entityID)
	if getErr != nil {
		return nil, p.logAndReturnServerError(ctx, "Failed to get entity after authentication",
			log.String("error", getErr.Error()))
	}
	result := &providers.AuthnResult{
		EntityReference: &providers.EntityReference{
			EntityID:       entityResult.ID,
			EntityCategory: string(entityResult.Category),
			EntityType:     entityResult.Type,
			OUID:           entityResult.OUID,
		},
	}
	attributes := make(map[string]interface{})
	if len(entityResult.Attributes) > 0 {
		if err := json.Unmarshal(entityResult.Attributes, &attributes); err != nil {
			return nil, p.logAndReturnServerError(ctx, "Failed to get attributes", log.String("error", err.Error()))
		}
	}
	result.Attributes = buildAttributesResponse(attributes)
	return result, nil
}

// resolveEntityFromToken resolves the entity for a caller-supplied token (entity
// reference token or attribute token). Failure to uniquely identify the entity is
// reported as a client error since the caller controls the token contents.
// tokenLabel ("entity reference token" / "attribute token") is inserted into the
// client error descriptions.
func (p *defaultAuthnProvider) resolveEntityFromToken(
	ctx context.Context, token map[string]interface{}, tokenLabel string,
) (*providers.Entity, *tidcommon.ServiceError) {
	entityID := ""
	if idVal, ok := token[authnprovidercm.UserAttributeUserID]; ok {
		entityID, _ = idVal.(string)
	}
	if entityID == "" && authnprovidercm.IsFederatedToken(token) {
		// A federated token resolves only through its recorded links. IdentifyEntity would scan for
		// keys no index can answer.
		linkedIDs, svcErr := p.resolveLinkedEntities(ctx, token)
		if svcErr != nil {
			return nil, svcErr
		}
		switch len(linkedIDs) {
		case 0:
			return nil, newClientError(authnprovidercm.ErrorCodeUserNotFound,
				"User not found", "No user found matching the provided "+tokenLabel)
		case 1:
			entityID = linkedIDs[0]
		default:
			return nil, newClientError(authnprovidercm.ErrorCodeAmbiguousUser,
				"Ambiguous user", "Multiple users are linked to the provided "+tokenLabel)
		}
	}
	if entityID == "" {
		identifiedEntityID, identifyErr := p.entitySvc.IdentifyEntity(ctx, token)
		if identifyErr != nil {
			if errors.Is(identifyErr, entity.ErrEntityNotFound) {
				return nil, newClientError(authnprovidercm.ErrorCodeUserNotFound,
					"User not found", "No user found matching the provided "+tokenLabel)
			}
			if errors.Is(identifyErr, entity.ErrAmbiguousEntity) {
				return nil, newClientError(authnprovidercm.ErrorCodeAmbiguousUser,
					"Ambiguous user", "Multiple users found matching the provided "+tokenLabel)
			}
			return nil, p.logAndReturnServerError(ctx, "Failed to identify entity after authentication",
				log.String("error", identifyErr.Error()))
		}
		entityID = *identifiedEntityID
	}

	entityResult, getErr := p.entitySvc.GetEntityProfile(ctx, entityID)
	if getErr != nil {
		return nil, p.logAndReturnServerError(ctx, "Failed to get entity attributes",
			log.String("error", getErr.Error()))
	}
	return entityResult, nil
}

// authenticateWithCredential selects the authentication mechanism by credential type. Every
// credential type dispatched here must also appear in authnprovidercm.InternalCredentialTypes,
// otherwise the credentials authentication API would let a client select that mechanism directly.
func (p *defaultAuthnProvider) authenticateWithCredential(
	ctx context.Context,
	identifiers, credentials map[string]interface{},
) (*authncommon.AuthnResult, providers.AuthenticationMethod, *attributedFailure, *tidcommon.ServiceError) {
	if provisioned, ok := credentials[authnprovidercm.CredentialTypeProvisionedEntityID]; ok {
		res, svcErr := p.authenticateForProvisioning(provisioned)
		// An entity the flow bound by ID checks no secret, so it is admitted as a federated sign-in.
		return res, providers.AuthenticationMethodFederated, nil, svcErr
	}
	if passkeyCredential, ok := credentials[authnprovidercm.CredentialTypePasskey]; ok {
		res, svcErr := p.authenticateWithPasskey(ctx, passkeyCredential)
		return res, providers.AuthenticationMethodPasskey, nil, svcErr
	}
	if otpCredential, ok := credentials[authnprovidercm.CredentialTypeOTP]; ok {
		res, failure, svcErr := p.authenticateWithOTP(ctx, otpCredential)
		return res, providers.AuthenticationMethodOTP, failure, svcErr
	}
	if fedCred, ok := credentials[authnprovidercm.CredentialTypeFederated]; ok {
		res, svcErr := p.authenticateWithFederated(ctx, fedCred)
		return res, providers.AuthenticationMethodFederated, nil, svcErr
	}
	if mlCred, ok := credentials[authnprovidercm.CredentialTypeMagicLink]; ok {
		res, svcErr := p.authenticateWithMagicLink(ctx, mlCred)
		return res, providers.AuthenticationMethodMagicLink, nil, svcErr
	}
	if vpCred, ok := credentials[authnprovidercm.CredentialTypeOpenID4VP]; ok {
		res, svcErr := p.authenticateWithOpenID4VP(ctx, vpCred)
		return res, providers.AuthenticationMethodOpenID4VP, nil, svcErr
	}
	method := credentialAuthenticationMethod(credentials)
	if userID, ok := identifiers[authnprovidercm.UserAttributeUserID]; ok && userID != "" {
		res, failure, svcErr := p.authenticateByUserID(ctx, userID, credentials)
		return res, method, failure, svcErr
	}
	res, failure, svcErr := p.authenticateByIdentifiers(ctx, identifiers, credentials)
	return res, method, failure, svcErr
}

// authenticateForProvisioning simulates a successful authentication for a provisioned entity.
// The raw credential is expected to be a non-empty userID string.
func (p *defaultAuthnProvider) authenticateForProvisioning(
	raw interface{},
) (*authncommon.AuthnResult, *tidcommon.ServiceError) {
	userID, ok := raw.(string)
	if !ok || userID == "" {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Invalid provisioning credential payload",
			"The provided provisioning credential must be a non-empty userID string")
	}
	return &authncommon.AuthnResult{
		Token:               map[string]interface{}{authnprovidercm.UserAttributeUserID: userID},
		AuthenticatedClaims: map[string]interface{}{authnprovidercm.UserAttributeUserID: userID},
	}, nil
}

// authenticateWithPasskey authenticates the user using the passkey service.
// The raw credential is expected to be a PasskeyAuthenticationFinishRequest struct.
func (p *defaultAuthnProvider) authenticateWithPasskey(
	ctx context.Context, raw interface{},
) (*authncommon.AuthnResult, *tidcommon.ServiceError) {
	cred, ok := raw.(*passkey.PasskeyAuthenticationFinishRequest)
	if !ok || cred == nil {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Invalid passkey payload", "The provided passkey credential is invalid")
	}
	result, authErr := p.passkeyService.FinishAuthentication(ctx, cred)
	if authErr != nil {
		return nil, newClientError(authnprovidercm.ErrorCodeAuthenticationFailed,
			authErr.Error.DefaultValue, authErr.ErrorDescription.DefaultValue)
	}
	return result, nil
}

// authenticateWithOTP authenticates the user using the OTP service.
// The raw credential is expected to be a map with "sessionToken" and "otp" string fields.
func (p *defaultAuthnProvider) authenticateWithOTP(
	ctx context.Context, raw interface{},
) (*authncommon.AuthnResult, *attributedFailure, *tidcommon.ServiceError) {
	otpCredential, ok := raw.(map[string]interface{})
	if !ok {
		return nil, nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Invalid OTP payload", "The provided OTP credential is invalid")
	}
	sessionToken, ok := otpCredential["sessionToken"].(string)
	if !ok || sessionToken == "" {
		return nil, nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Invalid OTP payload", "sessionToken is required")
	}
	otpValue, ok := otpCredential["otp"].(string)
	if !ok || otpValue == "" {
		return nil, nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Invalid OTP payload", "otp is required")
	}
	result, failure, authErr := p.otpService.Authenticate(ctx, sessionToken, otpValue)
	if authErr != nil {
		if authErr.Type == tidcommon.ClientErrorType {
			if authErr.Code == otp.ErrorIncorrectOTP.Code {
				return nil, otpFailureOf(failure),
					newClientError(authnprovidercm.ErrorCodeAuthenticationFailed,
						authErr.Error.DefaultValue, authErr.ErrorDescription.DefaultValue)
			}
			return nil, nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
				authErr.Error.DefaultValue, authErr.ErrorDescription.DefaultValue)
		}
		return nil, nil, p.logAndReturnServerError(ctx, "OTP authentication failed with server error",
			log.String("error", authErr.Error.DefaultValue),
			log.String("errorDescription", authErr.ErrorDescription.DefaultValue))
	}
	return result, nil, nil
}

// authenticateWithFederated authenticates the user using a federated identity provider.
// The raw credential is expected to be a FederatedAuthCredential struct with non-empty IDP ID and authorization code.
func (p *defaultAuthnProvider) authenticateWithFederated(
	ctx context.Context, raw interface{},
) (*authncommon.AuthnResult, *tidcommon.ServiceError) {
	cred, ok := raw.(*authncommon.FederatedAuthCredential)
	if !ok || cred == nil {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Invalid federated credential payload", "The provided federated credential is invalid")
	}
	if cred.IDPID == "" {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Missing IDP ID", "The federated credential must include a non-empty IDP ID")
	}
	if cred.AuthorizationData.Code == "" {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Missing authorization code", "The federated credential must include a non-empty authorization code")
	}
	svc, ok := p.federatedAuths[cred.IDPType]
	if !ok {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Unsupported IDP type", "The provided IDP type is not supported for federated authentication")
	}
	result, authErr := svc.Authenticate(ctx, cred.IDPID, cred.AuthorizationData)
	if authErr != nil {
		if authErr.Type == tidcommon.ClientErrorType {
			return nil, newClientError(authnprovidercm.ErrorCodeAuthenticationFailed,
				authErr.Error.DefaultValue, authErr.ErrorDescription.DefaultValue)
		}
		return nil, p.logAndReturnServerError(ctx, "Federated authentication failed with server error",
			log.String("error", authErr.Error.DefaultValue),
			log.String("errorDescription", authErr.ErrorDescription.DefaultValue))
	}
	return result, nil
}

// authenticateWithMagicLink authenticates the user using the magic link service.
// The raw credential is expected to be a map with a "token" string field and an
// optional "subjectAttribute" string field.
func (p *defaultAuthnProvider) authenticateWithMagicLink(
	ctx context.Context, raw interface{},
) (*authncommon.AuthnResult, *tidcommon.ServiceError) {
	mlCredential, ok := raw.(map[string]interface{})
	if !ok {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Invalid magic link payload", "The provided magic link credential is invalid")
	}
	token, ok := mlCredential["token"].(string)
	if !ok || token == "" {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Invalid magic link payload", "token is required")
	}
	subjectAttribute, _ := mlCredential["subjectAttribute"].(string)

	result, authErr := p.magicLinkService.Authenticate(ctx, token, subjectAttribute)
	if authErr != nil {
		if authErr.Type == tidcommon.ClientErrorType {
			return nil, newClientError(authnprovidercm.ErrorCodeAuthenticationFailed,
				authErr.Error.DefaultValue, authErr.ErrorDescription.DefaultValue)
		}
		return nil, p.logAndReturnServerError(ctx, "Magic link authentication failed with server error",
			log.String("error", authErr.Error.DefaultValue),
			log.String("errorDescription", authErr.ErrorDescription.DefaultValue))
	}
	return result, nil
}

// authenticateWithOpenID4VP authenticates the user using the OpenID4VP service.
// The raw credential is expected to be an OpenID4VPCredential carrying the verified presentation fields.
func (p *defaultAuthnProvider) authenticateWithOpenID4VP(
	ctx context.Context, raw interface{},
) (*authncommon.AuthnResult, *tidcommon.ServiceError) {
	cred, ok := raw.(*authncommon.OpenID4VPCredential)
	if !ok || cred == nil {
		return nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Invalid OpenID4VP payload", "The provided OpenID4VP credential is invalid")
	}
	result, svcErr := p.openid4vpService.Authenticate(ctx, cred)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ClientErrorType {
			return nil, newClientError(authnprovidercm.ErrorCodeAuthenticationFailed,
				svcErr.Error.DefaultValue, svcErr.ErrorDescription.DefaultValue)
		}
		return nil, p.logAndReturnServerError(ctx, "OpenID4VP authentication failed with server error",
			log.String("error", svcErr.ErrorDescription.DefaultValue))
	}
	return result, nil
}

// authenticateByUserID authenticates the user using a user ID and credentials.
func (p *defaultAuthnProvider) authenticateByUserID(
	ctx context.Context, userID interface{}, credentials map[string]interface{},
) (*authncommon.AuthnResult, *attributedFailure, *tidcommon.ServiceError) {
	userIDStr, ok := userID.(string)
	if !ok || userIDStr == "" {
		return nil, nil, newClientError(authnprovidercm.ErrorCodeInvalidRequest,
			"Invalid user ID", "The provided userID is invalid")
	}
	result, mismatchedEntityID, authErr := p.entitySvc.AuthenticateEntityByID(ctx, userIDStr, credentials)
	if authErr != nil {
		return nil, credentialFailureOf(mismatchedEntityID),
			p.handleEntityAuthError(ctx, authErr, "Basic authentication by ID failed with server error")
	}
	return &authncommon.AuthnResult{
		Token:               map[string]interface{}{authnprovidercm.UserAttributeUserID: result.EntityID},
		AuthenticatedClaims: map[string]interface{}{authnprovidercm.UserAttributeUserID: result.EntityID},
	}, nil, nil
}

// authenticateByIdentifiers authenticates the user using a set of identifiers and credentials.
func (p *defaultAuthnProvider) authenticateByIdentifiers(
	ctx context.Context, identifiers, credentials map[string]interface{},
) (*authncommon.AuthnResult, *attributedFailure, *tidcommon.ServiceError) {
	result, mismatchedEntityID, authErr := p.entitySvc.AuthenticateEntity(ctx, identifiers, credentials)
	if authErr != nil {
		return nil, credentialFailureOf(mismatchedEntityID),
			p.handleEntityAuthError(ctx, authErr, "Basic authentication failed with server error")
	}
	return &authncommon.AuthnResult{
		Token:               map[string]interface{}{authnprovidercm.UserAttributeUserID: result.EntityID},
		AuthenticatedClaims: map[string]interface{}{authnprovidercm.UserAttributeUserID: result.EntityID},
	}, nil, nil
}

func (p *defaultAuthnProvider) handleEntityAuthError(
	ctx context.Context, err error, serverMsg string) *tidcommon.ServiceError {
	if errors.Is(err, entity.ErrEntityNotFound) {
		return newClientError(authnprovidercm.ErrorCodeUserNotFound,
			"User not found", "The specified user does not exist")
	}
	if errors.Is(err, entity.ErrAuthenticationFailed) {
		return newClientError(authnprovidercm.ErrorCodeAuthenticationFailed,
			"Authentication failed", "Invalid credentials provided")
	}
	if errors.Is(err, entity.ErrAmbiguousEntity) {
		return newClientError(authnprovidercm.ErrorCodeAmbiguousUser,
			"Ambiguous user", "Multiple users found matching the provided identifiers")
	}
	return p.logAndReturnServerError(ctx, serverMsg, log.String("error", err.Error()))
}

// checkAuthenticationAccess submits a verified identity to governance, at the recovery plane inside a
// recovery flow.
func (p *defaultAuthnProvider) checkAuthenticationAccess(ctx context.Context,
	method providers.AuthenticationMethod, result *providers.AuthnResult) *tidcommon.ServiceError {
	if result.EntityReference == nil || result.EntityReference.EntityID == "" {
		return nil
	}
	entityID := result.EntityReference.EntityID

	authorityCtx := security.WithRuntimeContext(ctx)
	var decision providers.AccessDecision
	var svcErr *tidcommon.ServiceError
	if proofPlane(ctx) == providers.AccessPlaneRecovery {
		decision, svcErr = p.governance.EvaluateAccess(authorityCtx, entityID, providers.AccessPlaneRecovery, method)
	} else {
		decision, svcErr = p.governance.AdmitAuthenticationStep(authorityCtx, entityID, method)
	}
	return accessRefusal(decision, svcErr)
}

// handleAuthenticationFailure submits a failed proof attributed to an identity to governance. It
// returns the refusal governance states, or the original error.
func (p *defaultAuthnProvider) handleAuthenticationFailure(ctx context.Context,
	method providers.AuthenticationMethod, failure *attributedFailure,
	svcErr *tidcommon.ServiceError) *tidcommon.ServiceError {
	if failure == nil || svcErr.Code != authnprovidercm.ErrorCodeAuthenticationFailed {
		return svcErr
	}
	entityID, resolveErr := p.resolveFailedSubject(ctx, failure)
	if resolveErr != nil {
		return resolveErr
	}
	if entityID == "" {
		return svcErr
	}

	decision, govErr := p.governance.RecordFailure(security.WithRuntimeContext(ctx), entityID,
		proofPlane(ctx), method)
	if refused := accessRefusal(decision, govErr); refused != nil {
		return refused
	}
	return svcErr
}

// resolveFailedSubject returns the entity a failed proof is counted against, or an empty string when
// none resolves. A failed read is a server error, as it is for a verified proof.
func (p *defaultAuthnProvider) resolveFailedSubject(ctx context.Context,
	failure *attributedFailure) (string, *tidcommon.ServiceError) {
	if failure.entityID != "" {
		return failure.entityID, nil
	}
	if entityID, _ := failure.recipient[authnprovidercm.UserAttributeUserID].(string); entityID != "" {
		return entityID, nil
	}
	entityID, err := p.entitySvc.IdentifyEntity(ctx, failure.recipient)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) || errors.Is(err, entity.ErrAmbiguousEntity) {
			p.logger.Debug(ctx, "Authentication failure not counted: its subject did not resolve to one entity")
			return "", nil
		}
		return "", p.logAndReturnServerError(ctx, "Failed to resolve the subject of an authentication failure",
			log.Error(err))
	}
	return *entityID, nil
}

func (p *defaultAuthnProvider) logAndReturnServerError(
	ctx context.Context, msg string, fields ...log.Field) *tidcommon.ServiceError {
	p.logger.Error(ctx, msg, fields...)
	err := tidcommon.InternalServerError
	return &err
}

func buildAttributesResponse(attrs map[string]interface{}) *providers.AttributesResponse {
	resp := &providers.AttributesResponse{
		Attributes:    make(map[string]*providers.AttributeResponse),
		Verifications: make(map[string]*providers.VerificationResponse),
	}
	for k, v := range attrs {
		resp.Attributes[k] = &providers.AttributeResponse{
			Value: v,
			AssuranceMetadataResponse: &providers.AssuranceMetadataResponse{
				IsVerified: false,
			},
		}
	}
	return resp
}

func newClientError(code, msg, desc string) *tidcommon.ServiceError {
	return &tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: code,
		Error: tidcommon.I18nMessage{
			Key:          "error.authnproviderservice." + code,
			DefaultValue: msg,
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authnproviderservice." + code + "_description",
			DefaultValue: desc,
		},
	}
}
