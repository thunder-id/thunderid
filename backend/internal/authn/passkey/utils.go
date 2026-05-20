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
	"fmt"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

const (
	// defaultCredentialNameFormat is the format for auto-generated credential names.
	defaultCredentialNameFormat = "Passkey %s"
	// defaultCredentialDateFormat is the date format for credential names.
	defaultCredentialDateFormat = "2006-01-02" // nolint:gosec // This is a date format, not a credential
	// defaultOriginHTTP is the default HTTP origin for local development.
	defaultOriginHTTP = "https://localhost:8090"
)

// generateDefaultCredentialName generates a default credential name with the current date.
func generateDefaultCredentialName() string {
	return fmt.Sprintf(defaultCredentialNameFormat, time.Now().Format(defaultCredentialDateFormat))
}

// getConfiguredOrigins retrieves the allowed origins from runtime configuration.
func getConfiguredOrigins() []string {
	// Default origins if not configured
	defaultOrigins := []string{defaultOriginHTTP}

	// Try to get runtime configuration with panic recovery
	var originList []string
	func() {
		defer func() {
			if r := recover(); r != nil {
				// If configuration access fails, originList stays nil
				originList = nil
			}
		}()

		runtime := config.GetServerRuntime()
		if runtime != nil {
			originList = runtime.Config.Passkey.AllowedOrigins
		}
	}()

	// If no origins configured, return defaults
	if len(originList) == 0 {
		return defaultOrigins
	}

	return originList
}

// parseEntityAttributes parses an entity's attributes JSON into a generic map.
func parseEntityAttributes(attributes json.RawMessage) map[string]interface{} {
	if len(attributes) == 0 {
		return nil
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(attributes, &parsed); err != nil {
		return nil
	}

	return parsed
}

// buildWebAuthnDisplayName builds a WebAuthn-friendly display name from entity attributes,
// falling back to the entity ID when no human-readable name attributes are present.
func buildWebAuthnDisplayName(entityID string, attributes map[string]interface{}) string {
	if attributes == nil {
		return entityID
	}

	firstName, firstOk := attributes["given_name"].(string)
	lastName, lastOk := attributes["family_name"].(string)

	if firstOk && firstName != "" {
		if lastOk && lastName != "" {
			return firstName + " " + lastName
		}
		return firstName
	}

	return entityID
}

// resolveWebAuthnName resolves a WebAuthn-friendly username from entity attributes,
// falling back to the entity ID when no username/email attributes are present.
func resolveWebAuthnName(entityID string, attributes map[string]interface{}) string {
	if attributes == nil {
		return entityID
	}

	if username, ok := attributes["username"].(string); ok && username != "" {
		return username
	}

	if email, ok := attributes["email"].(string); ok && email != "" {
		return email
	}

	return entityID
}

// extractWebAuthnIdentity derives a WebAuthn display name and username from any entity's
// attributes, falling back to the entity ID when no name attributes are present.
func extractWebAuthnIdentity(e *entity.Entity) (displayName, name string) {
	attributes := parseEntityAttributes(e.Attributes)
	displayName = buildWebAuthnDisplayName(e.ID, attributes)
	name = resolveWebAuthnName(e.ID, attributes)
	return displayName, name
}

// decodeBase64 attempts to decode a base64 string using multiple encodings.
func decodeBase64(s string) ([]byte, error) {
	// Try RawURLEncoding (RFC 4648 section 5, no padding) - preferred for WebAuthn
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	// Try URLEncoding (RFC 4648 section 5, with padding)
	if b, err := base64.URLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	// Try RawStdEncoding (RFC 4648 section 4, no padding)
	if b, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	// Try StdEncoding (RFC 4648 section 4, with padding)
	return base64.StdEncoding.DecodeString(s)
}

// validateRegistrationStartRequest validates the registration start request.
func validateRegistrationStartRequest(req *PasskeyRegistrationStartRequest) *serviceerror.ServiceError {
	if strings.TrimSpace(req.UserID) == "" {
		return &ErrorEmptyUserIdentifier
	}
	if strings.TrimSpace(req.RelyingPartyID) == "" {
		return &ErrorEmptyRelyingPartyID
	}
	return nil
}

// validateRegistrationFinishRequest validates the registration finish request.
func validateRegistrationFinishRequest(req *PasskeyRegistrationFinishRequest) *serviceerror.ServiceError {
	if req == nil {
		return &ErrorInvalidFinishData
	}
	if strings.TrimSpace(req.SessionToken) == "" {
		return &ErrorEmptySessionToken
	}
	if strings.TrimSpace(req.CredentialID) == "" {
		return &ErrorInvalidFinishData
	}
	if strings.TrimSpace(req.ClientDataJSON) == "" {
		return &ErrorInvalidFinishData
	}
	if strings.TrimSpace(req.AttestationObject) == "" {
		return &ErrorInvalidFinishData
	}
	return nil
}

// validateAuthenticationStartRequest validates the authentication start request.
func validateAuthenticationStartRequest(req *PasskeyAuthenticationStartRequest) *serviceerror.ServiceError {
	if req == nil {
		return &ErrorInvalidFinishData
	}
	if strings.TrimSpace(req.RelyingPartyID) == "" {
		return &ErrorEmptyRelyingPartyID
	}
	return nil
}

// validateAuthenticationFinishRequest validates the authentication finish request.
func validateAuthenticationFinishRequest(req *PasskeyAuthenticationFinishRequest) *serviceerror.ServiceError {
	if req == nil {
		return &ErrorInvalidFinishData
	}
	if strings.TrimSpace(req.CredentialID) == "" {
		return &ErrorEmptyCredentialID
	}
	if strings.TrimSpace(req.CredentialType) == "" {
		return &ErrorEmptyCredentialType
	}
	if strings.TrimSpace(req.ClientDataJSON) == "" ||
		strings.TrimSpace(req.AuthenticatorData) == "" ||
		strings.TrimSpace(req.Signature) == "" {
		return &ErrorInvalidAuthenticatorResponse
	}
	if strings.TrimSpace(req.SessionToken) == "" {
		return &ErrorEmptySessionToken
	}
	return nil
}

// buildRegistrationOptions builds registration options from the request.
func buildRegistrationOptions(req *PasskeyRegistrationStartRequest) []registrationOption {
	var registrationOptions []registrationOption

	// Set authenticator selection if provided
	if req.AuthenticatorSelection != nil {
		authSelection := buildAuthenticatorSelection(req.AuthenticatorSelection)
		registrationOptions = append(registrationOptions, withAuthenticatorSelection(authSelection))
	}

	// Set attestation conveyance preference
	if req.Attestation != "" {
		conveyance := conveyancePreference(req.Attestation)
		registrationOptions = append(registrationOptions, withConveyancePreference(conveyance))
	}

	return registrationOptions
}

// buildAuthenticatorSelection builds authenticator selection from request.
func buildAuthenticatorSelection(sel *AuthenticatorSelection) authenticatorSelection {
	authSelection := authenticatorSelection{}

	if sel.AuthenticatorAttachment != "" {
		attachment := authenticatorAttachment(sel.AuthenticatorAttachment)
		authSelection.AuthenticatorAttachment = attachment
	}

	if sel.ResidentKey != "" {
		residentKey := residentKeyRequirement(sel.ResidentKey)
		authSelection.ResidentKey = residentKey
	}

	if sel.UserVerification != "" {
		authSelection.UserVerification = userVerificationRequirement(sel.UserVerification)
	} else {
		authSelection.UserVerification = verificationPreferred
	}

	return authSelection
}
