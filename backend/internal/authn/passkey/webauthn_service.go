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

package passkey

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/thunder-id/thunderid/internal/system/log"
)

// Wrapper types to abstract the underlying WebAuthn library.

// registrationOption wraps library-specific registration options.
type registrationOption = webauthn.RegistrationOption

// sessionData wraps library-specific session data.
type sessionData = webauthn.SessionData

// credentialCreation wraps library-specific credential creation options.
type credentialCreation = protocol.CredentialCreation

// credentialAssertion wraps library-specific credential assertion options.
type credentialAssertion = protocol.CredentialAssertion

// parsedCredentialCreationData wraps library-specific parsed credential creation data.
type parsedCredentialCreationData = protocol.ParsedCredentialCreationData

// parsedCredentialAssertionData wraps library-specific parsed credential assertion data.
type parsedCredentialAssertionData = protocol.ParsedCredentialAssertionData

// webauthnCredential wraps library-specific credential data.
type webauthnCredential = webauthn.Credential

// authenticator wraps library-specific authenticator data.
type authenticator = webauthn.Authenticator

// relyingPartyEntity wraps library-specific relying party entity.
type relyingPartyEntity = protocol.RelyingPartyEntity

// userEntity wraps library-specific user entity.
type userEntity = protocol.UserEntity

// credentialParameter wraps library-specific credential parameter.
type credentialParameter = protocol.CredentialParameter

// authenticatorSelection wraps library-specific authenticator selection criteria.
type authenticatorSelection = protocol.AuthenticatorSelection

// credentialDescriptor wraps library-specific credential descriptor.
type credentialDescriptor = protocol.CredentialDescriptor

// authenticationExtensions wraps library-specific authentication extensions.
type authenticationExtensions = protocol.AuthenticationExtensions

// conveyancePreference wraps library-specific attestation conveyance preference.
type conveyancePreference = protocol.ConveyancePreference

// userVerificationRequirement wraps library-specific user verification requirement.
type userVerificationRequirement = protocol.UserVerificationRequirement

// authenticatorAttachment wraps library-specific authenticator attachment.
type authenticatorAttachment = protocol.AuthenticatorAttachment

// residentKeyRequirement wraps library-specific resident key requirement.
type residentKeyRequirement = protocol.ResidentKeyRequirement

// credentialType wraps library-specific credential type.
type credentialType = protocol.CredentialType

// credentialMediationRequirement wraps library-specific credential mediation requirement.
type credentialMediationRequirement = protocol.CredentialMediationRequirement

// Wrapper constants for protocol constants.
var (
	verificationPreferred = protocol.VerificationPreferred
)

// Wrapper functions for webauthn library functions.
var (
	withAuthenticatorSelection = webauthn.WithAuthenticatorSelection
	withConveyancePreference   = webauthn.WithConveyancePreference
)

// webAuthnService provides an abstraction layer over the WebAuthn library.
type webAuthnService interface {
	// BeginRegistration initiates the registration ceremony and returns credential creation options.
	BeginRegistration(
		user webauthnUserInterface,
		options []registrationOption,
	) (*credentialCreation, *sessionData, error)

	// CreateCredential validates the registration response and creates a credential.
	CreateCredential(
		user webauthnUserInterface,
		session sessionData,
		response *parsedCredentialCreationData,
	) (*webauthnCredential, error)

	// BeginLogin initiates the authentication ceremony and returns credential request options.
	BeginLogin(
		user webauthnUserInterface,
	) (*credentialAssertion, *sessionData, error)

	// BeginDiscoverableLogin initiates usernameless authentication ceremony for discoverable credentials.
	BeginDiscoverableLogin() (*credentialAssertion, *sessionData, error)

	// ValidateLogin validates the authentication response and returns the verified credential.
	ValidateLogin(
		user webauthnUserInterface,
		session sessionData,
		response *parsedCredentialAssertionData,
	) (*webauthnCredential, error)

	// ValidatePasskeyLogin validates discoverable credential authentication and resolves the user.
	// Returns the authenticated user, verified credential, and any error.
	ValidatePasskeyLogin(
		userHandler func(rawID, userHandle []byte) (webauthnUserInterface, error),
		session sessionData,
		response *parsedCredentialAssertionData,
	) (webauthnUserInterface, *webauthnCredential, error)
}

// defaultWebAuthnService is the default implementation using the GO-WebAuthn library.
type defaultWebAuthnService struct {
	webAuthnLib *webauthn.WebAuthn
}

// newDefaultWebAuthnService creates a new service instance with the given configuration.
func newDefaultWebAuthnService(
	relyingPartyID, rpDisplayName string,
	rpOrigins []string,
) (webAuthnService, error) {
	config := &webauthn.Config{
		RPDisplayName: rpDisplayName,
		RPID:          relyingPartyID,
		RPOrigins:     rpOrigins,
	}

	webAuthnLib, err := webauthn.New(config)
	if err != nil {
		return nil, err
	}

	return &defaultWebAuthnService{
		webAuthnLib: webAuthnLib,
	}, nil
}

// BeginRegistration wraps the WebAuthn library's BeginRegistration method.
func (a *defaultWebAuthnService) BeginRegistration(
	user webauthnUserInterface,
	options []registrationOption,
) (*credentialCreation, *sessionData, error) {
	return a.webAuthnLib.BeginRegistration(user, options...)
}

// CreateCredential wraps the WebAuthn library's CreateCredential method.
func (a *defaultWebAuthnService) CreateCredential(
	user webauthnUserInterface,
	session sessionData,
	response *parsedCredentialCreationData,
) (*webauthnCredential, error) {
	return a.webAuthnLib.CreateCredential(user, session, response)
}

// BeginLogin wraps the WebAuthn library's BeginLogin method.
func (a *defaultWebAuthnService) BeginLogin(
	user webauthnUserInterface,
) (*credentialAssertion, *sessionData, error) {
	return a.webAuthnLib.BeginLogin(user)
}

// BeginDiscoverableLogin wraps the WebAuthn library's BeginDiscoverableLogin method.
func (a *defaultWebAuthnService) BeginDiscoverableLogin() (*credentialAssertion, *sessionData, error) {
	return a.webAuthnLib.BeginDiscoverableLogin(
		webauthn.WithUserVerification(verificationPreferred),
	)
}

// ValidateLogin wraps the WebAuthn library's ValidateLogin method.
func (a *defaultWebAuthnService) ValidateLogin(
	user webauthnUserInterface,
	session sessionData,
	response *parsedCredentialAssertionData,
) (*webauthnCredential, error) {
	return a.webAuthnLib.ValidateLogin(user, session, response)
}

// ValidatePasskeyLogin wraps the WebAuthn library's ValidatePasskeyLogin method for discoverable credentials.
func (a *defaultWebAuthnService) ValidatePasskeyLogin(
	userHandler func(rawID, userHandle []byte) (webauthnUserInterface, error),
	session sessionData,
	response *parsedCredentialAssertionData,
) (webauthnUserInterface, *webauthnCredential, error) {
	// Create a wrapper handler that converts our interface to the library's User interface
	libUserHandler := func(rawID, userHandle []byte) (webauthn.User, error) {
		user, err := userHandler(rawID, userHandle)
		if err != nil {
			return nil, err
		}
		// Our webauthnUserInterface is compatible with webauthn.User
		return user, nil
	}

	user, cred, err := a.webAuthnLib.ValidatePasskeyLogin(libUserHandler, session, response)
	if err != nil {
		return nil, nil, err
	}

	// Convert the returned user back to our interface type
	// Since the user came from our handler, it's already our type
	ourUser, ok := user.(webauthnUserInterface)
	if !ok {
		return nil, nil, fmt.Errorf("user is not of expected type")
	}

	return ourUser, cred, nil
}

// parseAssertionResponse converts raw string parameters to parsedCredentialAssertionData.
func parseAssertionResponse(credentialID, credentialType, clientDataJSON,
	authenticatorData, signature, userHandle string) (*parsedCredentialAssertionData, error) {
	// Decode all base64url encoded parameters
	rawID, err := decodeBase64(credentialID)
	if err != nil {
		return nil, fmt.Errorf("failed to decode credential ID: %w", err)
	}

	clientData, err := decodeBase64(clientDataJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to decode client data JSON: %w", err)
	}

	authData, err := decodeBase64(authenticatorData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode authenticator data: %w", err)
	}

	sig, err := decodeBase64(signature)
	if err != nil {
		return nil, fmt.Errorf("failed to decode signature: %w", err)
	}

	var userHandleBytes []byte
	if userHandle != "" {
		userHandleBytes, err = decodeBase64(userHandle)
		if err != nil {
			// User handle is optional, so we can continue without it
			userHandleBytes = nil
		}
	}

	// Parse client data JSON to extract required fields
	var clientDataParsed protocol.CollectedClientData
	if err := json.Unmarshal(clientData, &clientDataParsed); err != nil {
		return nil, fmt.Errorf("failed to parse client data JSON: %w", err)
	}

	// Parse authenticator data
	authDataParsed := protocol.AuthenticatorData{}
	if err := authDataParsed.Unmarshal(authData); err != nil {
		return nil, fmt.Errorf("failed to parse authenticator data: %w", err)
	}

	// Create the parsed credential assertion data structure
	parsed := &parsedCredentialAssertionData{
		ParsedPublicKeyCredential: protocol.ParsedPublicKeyCredential{
			RawID: rawID,
			ParsedCredential: protocol.ParsedCredential{
				ID:   credentialID,
				Type: credentialType,
			},
		},
		Response: protocol.ParsedAssertionResponse{
			CollectedClientData: clientDataParsed,
			AuthenticatorData:   authDataParsed,
			Signature:           sig,
			UserHandle:          userHandleBytes,
		},
		Raw: protocol.CredentialAssertionResponse{
			PublicKeyCredential: protocol.PublicKeyCredential{
				Credential: protocol.Credential{
					ID:   credentialID,
					Type: credentialType,
				},
				RawID: rawID,
			},
			AssertionResponse: protocol.AuthenticatorAssertionResponse{
				AuthenticatorResponse: protocol.AuthenticatorResponse{
					ClientDataJSON: clientData,
				},
				AuthenticatorData: authData,
				Signature:         sig,
				UserHandle:        userHandleBytes,
			},
		},
	}

	return parsed, nil
}

// parseAttestationResponse converts raw credential data to parsedCredentialCreationData.
// This function uses the protocol package's ParseCredentialCreationResponseBytes to properly
// parse the credential data with all required fields populated.
func parseAttestationResponse(
	credentialID, credentialType, clientDataJSON, attestationObject string,
) (*parsedCredentialCreationData, error) {
	// Decode inputs to ensure we have valid bytes, regardless of input encoding
	clientDataBytes, err := decodeBase64(clientDataJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to decode client data JSON: %w", err)
	}

	attestationBytes, err := decodeBase64(attestationObject)
	if err != nil {
		return nil, fmt.Errorf("failed to decode attestation object: %w", err)
	}

	// Re-encode to RawURLEncoding to ensure consistent format for the protocol parser
	// The protocol package expects RawURLEncoding for these fields in the JSON
	clientDataEncoded := base64.RawURLEncoding.EncodeToString(clientDataBytes)
	attestationEncoded := base64.RawURLEncoding.EncodeToString(attestationBytes)

	// Construct the JSON payload for the protocol parser
	// The protocol package expects the data in this format
	payload := map[string]interface{}{
		"id":    credentialID,
		"rawId": credentialID,
		"type":  credentialType,
		"response": map[string]interface{}{
			"clientDataJSON":    clientDataEncoded,
			"attestationObject": attestationEncoded,
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal credential: %w", err)
	}

	// Parse the attestation response using the protocol package's byte parser
	// This properly parses the CBOR-encoded attestation object and populates all fields
	parsed, err := protocol.ParseCredentialCreationResponseBytes(jsonData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse credential creation response: %w", err)
	}

	return parsed, nil
}

// webAuthnErrorFields returns structured log fields describing an error returned by the
// underlying WebAuthn library. When the error is a go-webauthn protocol.Error, its category,
// detail and debug info are captured as separate fields so that failures can be diagnosed
// server-side. Callers log these fields but always return a generic internal ServiceError to
// clients, so that library-specific details are never exposed externally.
func webAuthnErrorFields(err error) []log.Field {
	var protoErr *protocol.Error
	if errors.As(err, &protoErr) {
		return []log.Field{
			log.String("webAuthnErrorType", protoErr.Type),
			log.String("webAuthnErrorDetail", protoErr.Details),
			log.String("webAuthnErrorDebug", protoErr.DevInfo),
		}
	}
	return []log.Field{log.String("error", err.Error())}
}
