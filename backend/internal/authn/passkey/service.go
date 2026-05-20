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

// Package passkey implements the Passkey authentication service.
package passkey

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const (
	// loggerComponentName is the component name for logging.
	loggerComponentName = "PasskeyService"

	// passkeyCredentialType is the credential type key.
	passkeyCredentialType = "passkey"
)

// PasskeyServiceInterface defines the interface for passkey authentication and registration operations.
type PasskeyServiceInterface interface {
	// Registration methods
	StartRegistration(
		ctx context.Context, req *PasskeyRegistrationStartRequest,
	) (*PasskeyRegistrationStartData, *serviceerror.ServiceError)
	FinishRegistration(
		ctx context.Context, req *PasskeyRegistrationFinishRequest,
	) (*PasskeyRegistrationFinishData, *serviceerror.ServiceError)

	// Authentication methods
	StartAuthentication(
		ctx context.Context, req *PasskeyAuthenticationStartRequest,
	) (*PasskeyAuthenticationStartData, *serviceerror.ServiceError)
	FinishAuthentication(
		ctx context.Context, req *PasskeyAuthenticationFinishRequest,
	) (*common.AuthenticationResponse, *serviceerror.ServiceError)
}

// passkeyService is the default implementation of PasskeyServiceInterface.
type passkeyService struct {
	entityService entity.EntityServiceInterface
	sessionStore  sessionStoreInterface
	logger        *log.Logger
}

// newPasskeyService creates a new instance of passkey service.
func newPasskeyService(
	entitySvc entity.EntityServiceInterface, sessionStore sessionStoreInterface,
) PasskeyServiceInterface {
	return &passkeyService{
		entityService: entitySvc,
		sessionStore:  sessionStore,
		logger:        log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)),
	}
}

// StartRegistration initiates passkey credential registration for a user.
func (w *passkeyService) StartRegistration(
	ctx context.Context, req *PasskeyRegistrationStartRequest,
) (*PasskeyRegistrationStartData, *serviceerror.ServiceError) {
	if req == nil {
		return nil, &ErrorInvalidFinishData
	}

	logger := w.logger.With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug("Starting passkey credential registration",
		log.MaskedString("userID", req.UserID),
		log.String("relyingPartyID", req.RelyingPartyID))

	// Validate input
	if svcErr := validateRegistrationStartRequest(req); svcErr != nil {
		return nil, svcErr
	}

	// Retrieve entity
	coreEntity, svcErr := w.getEntity(ctx, req.UserID)
	if svcErr != nil {
		return nil, svcErr
	}

	// Retrieve any existing passkey credentials
	entries, svcErr := w.getStoredPasskeyEntries(ctx, req.UserID)
	if svcErr != nil {
		return nil, svcErr
	}
	credentials := w.decodePasskeyCredentials(req.UserID, entries)

	// Build relying party display name
	rpDisplayName := req.RelyingPartyName
	if rpDisplayName == "" {
		rpDisplayName = req.RelyingPartyID
	}

	logger.Debug("Retrieved existing credentials",
		log.MaskedString("entityID", req.UserID),
		log.Int("credentialCount", len(credentials)))

	// Create passkey user
	webAuthnUser := newWebAuthnUserFromEntity(coreEntity, credentials)

	// Initialize WebAuthn service with relying party configuration
	rpOrigins := getConfiguredOrigins()
	webAuthnService, err := newDefaultWebAuthnService(req.RelyingPartyID, rpDisplayName, rpOrigins)
	if err != nil {
		logger.Error("Failed to initialize WebAuthn service", log.String("error", err.Error()))
		return nil, &serviceerror.InternalServerError
	}

	// Configure registration options
	registrationOptions := buildRegistrationOptions(req)

	// Begin registration ceremony using the WebAuthn service
	// The WebAuthn service will generate challenge and set timeout automatically
	options, sessionData, err := webAuthnService.BeginRegistration(webAuthnUser, registrationOptions)
	if err != nil {
		logger.Error("Failed to begin passkey registration", log.String("error", err.Error()))
		return nil, &serviceerror.InternalServerError
	}

	// Store session data in cache with TTL
	sessionToken, svcErr := w.storeSessionData(sessionData)
	if svcErr != nil {
		logger.Error("Failed to store session data", log.String("error", svcErr.Error.DefaultValue))
		return nil, svcErr
	}

	logger.Debug("Passkey credential creation options generated successfully",
		log.MaskedString("userID", req.UserID),
		log.Int("credentialsCount", len(credentials)))

	// Convert to custom structure with properly encoded challenge
	creationOptions := PublicKeyCredentialCreationOptions{
		Challenge:              base64.RawURLEncoding.EncodeToString(options.Response.Challenge),
		RelyingParty:           options.Response.RelyingParty,
		User:                   options.Response.User,
		Parameters:             options.Response.Parameters,
		AuthenticatorSelection: options.Response.AuthenticatorSelection,
		Timeout:                options.Response.Timeout,
		CredentialExcludeList:  options.Response.CredentialExcludeList,
		Extensions:             options.Response.Extensions,
		Attestation:            options.Response.Attestation,
	}

	return &PasskeyRegistrationStartData{
		PublicKeyCredentialCreationOptions: creationOptions,
		SessionToken:                       sessionToken,
	}, nil
}

// FinishRegistration completes passkey credential registration.
func (w *passkeyService) FinishRegistration(ctx context.Context, req *PasskeyRegistrationFinishRequest) (
	*PasskeyRegistrationFinishData, *serviceerror.ServiceError) {
	logger := w.logger.With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug("Finishing passkey credential registration")

	// Validate input
	if svcErr := validateRegistrationFinishRequest(req); svcErr != nil {
		logger.Debug("Registration finish request validation failed")
		return nil, svcErr
	}

	// Default credential type to "public-key" if not provided
	credentialType := strings.TrimSpace(req.CredentialType)
	if credentialType == "" {
		credentialType = "public-key"
	}

	logger.Debug("Parsing attestation response",
		log.String("credentialID", req.CredentialID),
		log.String("credentialType", credentialType),
		log.Int("clientDataJSONLen", len(req.ClientDataJSON)),
		log.Int("attestationObjectLen", len(req.AttestationObject)))

	// Parse the attestation response
	// This ensures all required fields including the Raw field are properly populated
	parsedCredential, err := parseAttestationResponse(
		req.CredentialID,
		credentialType,
		req.ClientDataJSON,
		req.AttestationObject,
	)
	if err != nil {
		logger.Debug("Failed to parse attestation response",
			log.String("error", err.Error()),
			log.String("credentialID", req.CredentialID),
			log.String("credentialType", credentialType))
		return nil, &ErrorInvalidAttestationResponse
	}

	logger.Debug("Successfully parsed attestation response",
		log.String("credentialID", parsedCredential.ID),
		log.String("credentialType", parsedCredential.Type))

	// Retrieve session data from cache
	sessionData, userID, relyingPartyID, svcErr := w.retrieveSessionData(req.SessionToken)
	if svcErr != nil {
		logger.Error("Failed to retrieve session data", log.String("error", svcErr.Error.DefaultValue))
		return nil, svcErr
	}

	// Retrieve entity
	coreEntity, svcErr := w.getEntity(ctx, userID)
	if svcErr != nil {
		return nil, svcErr
	}

	// Retrieve any existing passkey credentials
	entries, svcErr := w.getStoredPasskeyEntries(ctx, userID)
	if svcErr != nil {
		return nil, svcErr
	}
	credentials := w.decodePasskeyCredentials(userID, entries)

	logger.Debug("Retrieved existing credentials for entity",
		log.MaskedString("entityID", userID),
		log.Int("credentialCount", len(credentials)))

	// Create WebAuthn user from entity
	webAuthnUser := newWebAuthnUserFromEntity(coreEntity, credentials)

	// Initialize WebAuthn service with relying party configuration
	rpOrigins := getConfiguredOrigins()
	webAuthnService, err := newDefaultWebAuthnService(relyingPartyID, relyingPartyID, rpOrigins)
	if err != nil {
		logger.Error("Failed to initialize WebAuthn service", log.String("error", err.Error()))
		return nil, &serviceerror.InternalServerError
	}

	// Verify the credential using WebAuthn service
	credential, err := webAuthnService.CreateCredential(webAuthnUser, *sessionData, parsedCredential)
	if err != nil {
		logger.Error("Failed to verify and create credential", log.String("error", err.Error()))
		return nil, &ErrorInvalidAttestationResponse
	}

	// Generate credential name if not provided
	credentialName := req.CredentialName
	if credentialName == "" {
		credentialName = generateDefaultCredentialName()
	}

	// Encode credential ID to base64url
	credentialID := base64.StdEncoding.EncodeToString(credential.ID)

	// Store credential in database using user service
	if err := w.storePasskeyCredential(ctx, userID, credential); err != nil {
		logger.Error("Failed to store credential in database", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	// Clear session data
	w.clearSessionData(req.SessionToken)

	return &PasskeyRegistrationFinishData{
		CredentialID:   credentialID,
		CredentialName: credentialName,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// StartAuthentication initiates passkey authentication for a user.
func (w *passkeyService) StartAuthentication(ctx context.Context, req *PasskeyAuthenticationStartRequest) (
	*PasskeyAuthenticationStartData, *serviceerror.ServiceError) {
	if req == nil {
		return nil, &ErrorInvalidFinishData
	}

	logger := w.logger.With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	// Check if this is usernameless flow
	isUsernameless := strings.TrimSpace(req.UserID) == ""

	if isUsernameless {
		logger.Debug("Starting usernameless passkey authentication",
			log.String("relyingPartyID", req.RelyingPartyID))
	} else {
		logger.Debug("Starting passkey authentication",
			log.MaskedString("userID", req.UserID),
			log.String("relyingPartyID", req.RelyingPartyID))
	}

	// Validate input
	if svcErr := validateAuthenticationStartRequest(req); svcErr != nil {
		return nil, svcErr
	}

	// Initialize WebAuthn service with relying party configuration
	rpOrigins := getConfiguredOrigins()
	webAuthnService, err := newDefaultWebAuthnService(req.RelyingPartyID, req.RelyingPartyID, rpOrigins)
	if err != nil {
		logger.Error("Failed to initialize WebAuthn service", log.String("error", err.Error()))
		return nil, &serviceerror.InternalServerError
	}

	var options *credentialAssertion
	var sessionData *sessionData

	if isUsernameless {
		// Usernameless flow: Use discoverable credentials
		options, sessionData, err = webAuthnService.BeginDiscoverableLogin()
		if err != nil {
			logger.Error("Failed to begin usernameless passkey login", log.String("error", err.Error()))
			return nil, &serviceerror.InternalServerError
		}
	} else {
		// Username-based flow: Retrieve entity and its credentials
		coreEntity, svcErr := w.getEntity(ctx, req.UserID)
		if svcErr != nil {
			return nil, svcErr
		}

		entries, svcErr := w.getStoredPasskeyEntries(ctx, req.UserID)
		if svcErr != nil {
			return nil, svcErr
		}
		credentials := w.decodePasskeyCredentials(req.UserID, entries)

		logger.Debug("Retrieved credentials for authentication",
			log.MaskedString("entityID", req.UserID),
			log.Int("credentialCount", len(credentials)))

		if len(credentials) == 0 {
			logger.Debug("No credentials found for entity", log.MaskedString("entityID", req.UserID))
			return nil, &ErrorNoCredentialsFound
		}

		// Create WebAuthn user from entity
		webAuthnUser := newWebAuthnUserFromEntity(coreEntity, credentials)

		// Begin login ceremony using the WebAuthn service
		// The WebAuthn service will generate challenge and set timeout automatically
		options, sessionData, err = webAuthnService.BeginLogin(webAuthnUser)
		if err != nil {
			logger.Error("Failed to begin passkey login", log.String("error", err.Error()))
			return nil, &serviceerror.InternalServerError
		}
	}

	// Store session data in cache with TTL
	sessionToken, svcErr := w.storeSessionData(sessionData)
	if svcErr != nil {
		logger.Error("Failed to store session data", log.String("error", svcErr.Error.DefaultValue))
		return nil, svcErr
	}

	// Convert to custom structure with properly encoded challenge
	requestOptions := PublicKeyCredentialRequestOptions{
		Challenge:        base64.RawURLEncoding.EncodeToString(options.Response.Challenge),
		Timeout:          options.Response.Timeout,
		RelyingPartyID:   options.Response.RelyingPartyID,
		AllowCredentials: options.Response.AllowedCredentials,
		UserVerification: options.Response.UserVerification,
		Extensions:       options.Response.Extensions,
	}

	return &PasskeyAuthenticationStartData{
		PublicKeyCredentialRequestOptions: requestOptions,
		SessionToken:                      sessionToken,
	}, nil
}

// FinishAuthentication completes passkey authentication.
func (w *passkeyService) FinishAuthentication(ctx context.Context, req *PasskeyAuthenticationFinishRequest) (
	*common.AuthenticationResponse, *serviceerror.ServiceError) {
	logger := w.logger.With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug("Finishing passkey authentication")

	// Validate input
	if svcErr := validateAuthenticationFinishRequest(req); svcErr != nil {
		return nil, svcErr
	}

	// Retrieve session data from cache
	sessionData, sessionUserID, relyingPartyID, svcErr := w.retrieveSessionData(req.SessionToken)
	if svcErr != nil {
		logger.Error("Failed to retrieve session data", log.String("error", svcErr.Error.DefaultValue))
		return nil, svcErr
	}

	// Check if this is usernameless flow (session was created without userID)
	isUsernameless := strings.TrimSpace(sessionUserID) == ""

	var userID string

	if isUsernameless {
		// Usernameless flow: Resolve user from userHandle in the authentication response
		if req.UserHandle == "" {
			logger.Error("UserHandle is required for usernameless authentication")
			return nil, &ErrorInvalidAuthenticatorResponse
		}

		// Decode userHandle to get userID
		userHandleBytes, err := base64.StdEncoding.DecodeString(req.UserHandle)
		if err != nil {
			// Try RawURLEncoding if standard encoding fails
			userHandleBytes, err = base64.RawURLEncoding.DecodeString(req.UserHandle)
			if err != nil {
				logger.Error("Failed to decode userHandle", log.Error(err))
				return nil, &ErrorInvalidAuthenticatorResponse
			}
		}

		userID = string(userHandleBytes)
		logger.Debug("Resolved userID from userHandle for usernameless authentication",
			log.MaskedString("userID", userID))
	} else {
		// Username-based flow: Use userID from session
		userID = sessionUserID
		logger.Debug("Processing passkey authentication",
			log.MaskedString("userID", userID),
			log.String("relyingPartyID", relyingPartyID))
	}

	// Retrieve entity and its credentials
	coreEntity, svcErr := w.getEntity(ctx, userID)
	if svcErr != nil {
		return nil, svcErr
	}

	entries, svcErr := w.getStoredPasskeyEntries(ctx, userID)
	if svcErr != nil {
		return nil, svcErr
	}
	credentials := w.decodePasskeyCredentials(userID, entries)

	logger.Debug("Retrieved credentials for authentication verification",
		log.MaskedString("entityID", userID),
		log.Int("credentialCount", len(credentials)))

	if len(credentials) == 0 {
		logger.Debug("No credentials found for entity", log.MaskedString("entityID", userID))
		return nil, &ErrorNoCredentialsFound
	}

	// Create WebAuthn user from entity
	webAuthnUser := newWebAuthnUserFromEntity(coreEntity, credentials)

	// Initialize WebAuthn service with relying party configuration
	rpOrigins := getConfiguredOrigins()
	webAuthnService, err := newDefaultWebAuthnService(relyingPartyID, relyingPartyID, rpOrigins)
	if err != nil {
		logger.Error("Failed to initialize WebAuthn service", log.String("error", err.Error()))
		return nil, &serviceerror.InternalServerError
	}

	// Parse the assertion response from the raw credential data
	parsedResponse, err := parseAssertionResponse(req.CredentialID, req.CredentialType,
		req.ClientDataJSON, req.AuthenticatorData, req.Signature, req.UserHandle)
	if err != nil {
		logger.Debug("Failed to parse assertion response", log.String("error", err.Error()))
		return nil, &ErrorInvalidAuthenticatorResponse
	}

	var credential *webauthnCredential

	if isUsernameless {
		// Usernameless flow: Use ValidatePasskeyLogin with user handler
		userHandler := func(rawID, userHandle []byte) (webauthnUserInterface, error) {
			// The user has already been resolved and validated above
			return webAuthnUser, nil
		}

		_, credential, err = webAuthnService.ValidatePasskeyLogin(userHandler, *sessionData, parsedResponse)
		if err != nil {
			logger.Debug("Failed to validate passkey assertion", log.String("error", err.Error()))
			return nil, &ErrorInvalidSignature
		}
	} else {
		// Username-based flow: Use ValidateLogin with specific user
		credential, err = webAuthnService.ValidateLogin(webAuthnUser, *sessionData, parsedResponse)
		if err != nil {
			logger.Debug("Failed to validate WebAuthn assertion", log.String("error", err.Error()))
			return nil, &ErrorInvalidSignature
		}
	}

	logger.Debug("Passkey authentication verified successfully",
		log.String("credentialID", base64.StdEncoding.EncodeToString(credential.ID)),
		log.Any("signCount", credential.Authenticator.SignCount))

	// Update credential in database to prevent replay attacks
	if err := w.updatePasskeyCredential(ctx, userID, credential); err != nil {
		logger.Error("Failed to update credential sign count in database", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	logger.Debug("Updated credential sign count in database",
		log.MaskedString("userID", userID),
		log.String("credentialID", base64.StdEncoding.EncodeToString(credential.ID)),
		log.Any("newSignCount", credential.Authenticator.SignCount))

	// Clear session data
	w.clearSessionData(req.SessionToken)

	// Build authentication response
	authResponse := &common.AuthenticationResponse{
		ID:   coreEntity.ID,
		Type: coreEntity.Type,
		OUID: coreEntity.OUID,
	}

	logger.Debug("Passkey authentication completed successfully",
		log.MaskedString("entityID", userID))

	return authResponse, nil
}

// getEntity retrieves an entity by ID, mapping entity-layer errors to passkey service errors.
func (w *passkeyService) getEntity(
	ctx context.Context, entityID string,
) (*entity.Entity, *serviceerror.ServiceError) {
	logger := w.logger.With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	e, err := w.entityService.GetEntity(ctx, entityID)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			logger.Debug("Entity not found", log.MaskedString("entityID", entityID))
			return nil, &ErrorUserNotFound
		}
		logger.Error("Failed to retrieve entity", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	return e, nil
}

// getStoredPasskeyEntries fetches the raw stored passkey credential entries for an entity.
// Returns a nil slice when the entity has no passkey credentials registered.
func (w *passkeyService) getStoredPasskeyEntries(
	ctx context.Context, entityID string,
) ([]entity.StoredCredential, *serviceerror.ServiceError) {
	logger := w.logger.With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	entries, err := w.entityService.GetCredentialsByType(ctx, entityID, passkeyCredentialType)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			logger.Debug("Entity not found", log.MaskedString("entityID", entityID))
			return nil, &ErrorUserNotFound
		}
		logger.Error("Failed to retrieve passkey credentials", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	return entries, nil
}

// decodePasskeyCredentials converts stored passkey entries into decoded webauthnCredential
// values, skipping any entries with empty or malformed values.
func (w *passkeyService) decodePasskeyCredentials(
	entityID string, entries []entity.StoredCredential,
) []webauthnCredential {
	logger := w.logger.With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	credentials := make([]webauthnCredential, 0, len(entries))
	for _, entry := range entries {
		if entry.Value == "" {
			logger.Error("Empty credential value", log.MaskedString("entityID", entityID))
			continue
		}
		var credential webauthnCredential
		if err := json.Unmarshal([]byte(entry.Value), &credential); err != nil {
			logger.Error("Failed to unmarshal passkey credential",
				log.MaskedString("entityID", entityID),
				log.Error(err))
			continue
		}
		credentials = append(credentials, credential)
	}
	return credentials
}

// storePasskeyCredential appends a new passkey credential to the entity's stored set.
func (w *passkeyService) storePasskeyCredential(
	ctx context.Context, entityID string, credential *webauthnCredential,
) error {
	logger := w.logger.With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	credentialJSON, err := json.Marshal(credential)
	if err != nil {
		logger.Error("Failed to marshal credential",
			log.MaskedString("entityID", entityID),
			log.Error(err))
		return fmt.Errorf("failed to marshal credential: %w", err)
	}

	existingEntries, svcErr := w.getStoredPasskeyEntries(ctx, entityID)
	if svcErr != nil {
		return fmt.Errorf("failed to load existing passkey credentials: %s", svcErr.Error.DefaultValue)
	}

	existingEntries = append(existingEntries, entity.StoredCredential{
		Value: string(credentialJSON),
	})

	payload, err := json.Marshal(map[string][]entity.StoredCredential{
		passkeyCredentialType: existingEntries,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal passkey credentials: %w", err)
	}
	if err := w.entityService.UpdateSystemCredentials(ctx, entityID, payload); err != nil {
		logger.Error("Failed to update passkey credentials",
			log.MaskedString("entityID", entityID),
			log.Error(err))
		return fmt.Errorf("failed to update passkey credentials: %w", err)
	}

	logger.Debug("Successfully stored passkey credential in database",
		log.MaskedString("entityID", entityID),
		log.String("credentialID", base64.StdEncoding.EncodeToString(credential.ID)))

	return nil
}

// updatePasskeyCredential updates an existing passkey credential, preserving the storage
// metadata (StorageAlgo, StorageAlgoParams) of the original entry.
func (w *passkeyService) updatePasskeyCredential(
	ctx context.Context, entityID string, updatedCredential *webauthnCredential,
) error {
	logger := w.logger.With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	existingEntries, svcErr := w.getStoredPasskeyEntries(ctx, entityID)
	if svcErr != nil {
		return fmt.Errorf("failed to load existing passkey credentials: %s", svcErr.Error.DefaultValue)
	}

	found := false
	updatedEntries := make([]entity.StoredCredential, 0, len(existingEntries))

	for _, entry := range existingEntries {
		var credential webauthnCredential
		if err := json.Unmarshal([]byte(entry.Value), &credential); err != nil {
			logger.Warn("Failed to unmarshal credential, keeping original",
				log.MaskedString("entityID", entityID),
				log.Error(err))
			updatedEntries = append(updatedEntries, entry)
			continue
		}

		if string(credential.ID) == string(updatedCredential.ID) {
			credentialJSON, marshalErr := json.Marshal(updatedCredential)
			if marshalErr != nil {
				logger.Error("Failed to marshal updated credential",
					log.MaskedString("entityID", entityID),
					log.Error(marshalErr))
				return fmt.Errorf("failed to marshal updated credential: %w", marshalErr)
			}
			updatedEntries = append(updatedEntries, entity.StoredCredential{
				StorageAlgo:       entry.StorageAlgo,
				StorageAlgoParams: entry.StorageAlgoParams,
				Value:             string(credentialJSON),
			})
			found = true

			logger.Debug("Updated credential in memory",
				log.MaskedString("entityID", entityID),
				log.String("credentialID", base64.StdEncoding.EncodeToString(updatedCredential.ID)),
				log.Any("newSignCount", updatedCredential.Authenticator.SignCount))
		} else {
			updatedEntries = append(updatedEntries, entry)
		}
	}

	if !found {
		logger.Warn("Passkey credential not found for update",
			log.MaskedString("entityID", entityID),
			log.String("credentialID", base64.StdEncoding.EncodeToString(updatedCredential.ID)))
		return fmt.Errorf("credential not found for update")
	}

	payload, err := json.Marshal(map[string][]entity.StoredCredential{
		passkeyCredentialType: updatedEntries,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal passkey credentials: %w", err)
	}
	if err := w.entityService.UpdateSystemCredentials(ctx, entityID, payload); err != nil {
		logger.Error("Failed to update credentials",
			log.MaskedString("entityID", entityID),
			log.Error(err))
		return fmt.Errorf("failed to update passkey credentials: %w", err)
	}

	logger.Debug("Successfully updated passkey credential in database",
		log.MaskedString("entityID", entityID),
		log.String("credentialID", base64.StdEncoding.EncodeToString(updatedCredential.ID)),
		log.Any("newSignCount", updatedCredential.Authenticator.SignCount))

	return nil
}
