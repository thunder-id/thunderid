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
	"time"

	"github.com/go-webauthn/webauthn/protocol/webauthncose"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// sessionStoreInterface defines the interface for WebAuthn session storage.
type sessionStoreInterface interface {
	storeSession(
		sessionKey string,
		session *sessionData,
		expirySeconds int64,
	) error
	retrieveSession(sessionKey string) (*sessionData, error)
	deleteSession(sessionKey string) error
}

// sessionStore provides the WebAuthn session store functionality using database.
type sessionStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
	logger       *log.Logger
}

// newSessionStore creates a new instance of sessionStore.
func newSessionStore() sessionStoreInterface {
	return &sessionStore{
		dbProvider:   provider.GetDBProvider(),
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
		logger:       log.GetLogger().With(log.String(log.LoggerKeyComponentName, "WebAuthnSessionStore")),
	}
}

// storeSession stores a WebAuthn session in the database.
func (s *sessionStore) storeSession(sessionKey string, session *sessionData, expirySeconds int64) error {
	dbClient, err := s.dbProvider.GetRuntimeDBClient()
	if err != nil {
		s.logger.Error("Failed to get database client", log.Error(err))
		return err
	}

	// Serialize session data to JSON
	jsonDataBytes, err := s.serializeSessionData(session)
	if err != nil {
		s.logger.Error("Failed to marshal session data to JSON", log.Error(err))
		return err
	}

	if s.logger.IsDebugEnabled() {
		s.logger.Debug("Storing session data",
			log.MaskedString("sessionKey", sessionKey),
			log.String("jsonDataLength", fmt.Sprintf("%d bytes", len(jsonDataBytes))))
	}

	expiryTime := time.Now().UTC().Add(time.Duration(expirySeconds) * time.Second)
	_, err = dbClient.Execute(queryInsertSession, sessionKey, jsonDataBytes, expiryTime, s.deploymentID)
	if err != nil {
		s.logger.Error("Failed to insert WebAuthn session", log.Error(err))
		return err
	}

	s.logger.Debug("WebAuthn session stored successfully",
		log.MaskedString("sessionKey", sessionKey))

	return nil
}

// retrieveSession retrieves a WebAuthn session from the database.
func (s *sessionStore) retrieveSession(sessionKey string) (*sessionData, error) {
	if sessionKey == "" {
		return nil, nil
	}

	dbClient, err := s.dbProvider.GetRuntimeDBClient()
	if err != nil {
		s.logger.Error("Failed to get database client", log.Error(err))
		return nil, err
	}

	// Check expiry by comparing with current time
	now := time.Now().UTC()
	results, err := dbClient.Query(queryGetSession, sessionKey, now, s.deploymentID)
	if err != nil {
		s.logger.Error("Failed to query WebAuthn session", log.Error(err))
		return nil, err
	}

	if len(results) == 0 {
		s.logger.Debug("WebAuthn session not found or expired",
			log.MaskedString("sessionKey", sessionKey))
		return nil, nil
	}

	row := results[0]

	if s.logger.IsDebugEnabled() {
		s.logger.Debug("Retrieved session row from database",
			log.MaskedString("sessionKey", sessionKey),
			log.String("rowKeys", fmt.Sprintf("%v", getMapKeys(row))))
	}

	sessionData, err := s.buildSessionDataFromResultRow(row)
	if err != nil {
		s.logger.Error("Failed to build session data from result", log.Error(err))
		return nil, err
	}

	s.logger.Debug("WebAuthn session retrieved successfully",
		log.MaskedString("sessionKey", sessionKey))

	return sessionData, nil
}

// deleteSession removes a specific WebAuthn session from the database.
func (s *sessionStore) deleteSession(sessionKey string) error {
	if sessionKey == "" {
		return nil
	}

	dbClient, err := s.dbProvider.GetRuntimeDBClient()
	if err != nil {
		s.logger.Error("Failed to get database client", log.Error(err))
		return err
	}

	_, err = dbClient.Execute(queryDeleteSession, sessionKey, s.deploymentID)
	if err != nil {
		s.logger.Error("Failed to delete WebAuthn session", log.Error(err))
		return err
	}

	s.logger.Debug("WebAuthn session deleted successfully",
		log.MaskedString("sessionKey", sessionKey))

	return nil
}

// serializeSessionData converts WebAuthn session data to JSON bytes.
func (s *sessionStore) serializeSessionData(sessionData *sessionData) ([]byte, error) {
	jsonData := map[string]interface{}{
		jsonKeyChallenge:        sessionData.Challenge,
		jsonKeyUserVerification: string(sessionData.UserVerification),
	}

	// Add RelyingPartyID (REQUIRED for verification)
	if sessionData.RelyingPartyID != "" {
		jsonData[jsonKeyRelyingPartyID] = sessionData.RelyingPartyID
	}

	// Add UserID if present (REQUIRED for user verification)
	if len(sessionData.UserID) > 0 {
		jsonData[jsonKeyUserID] = base64.StdEncoding.EncodeToString(sessionData.UserID)
	}

	// Add Expires time (REQUIRED for session validation)
	if !sessionData.Expires.IsZero() {
		jsonData[jsonKeyExpires] = sessionData.Expires.Unix()
	}

	// Add extensions if present
	if sessionData.Extensions != nil {
		jsonData[jsonKeyExtensions] = sessionData.Extensions
	}

	// Add allowed credentials if present
	if len(sessionData.AllowedCredentialIDs) > 0 {
		allowedCreds := make([]string, len(sessionData.AllowedCredentialIDs))
		for i, credID := range sessionData.AllowedCredentialIDs {
			allowedCreds[i] = base64.StdEncoding.EncodeToString(credID)
		}
		jsonData[jsonKeyAllowedCredentials] = allowedCreds
	}

	// Add credential parameters if present (for registration)
	if len(sessionData.CredParams) > 0 {
		jsonData[jsonKeyCredParams] = sessionData.CredParams
	}

	// Add mediation if present
	if sessionData.Mediation != "" {
		jsonData[jsonKeyMediation] = string(sessionData.Mediation)
	}

	return json.Marshal(jsonData)
}

// buildSessionDataFromResultRow builds WebAuthn session data from database result row.
func (s *sessionStore) buildSessionDataFromResultRow(
	row map[string]interface{},
) (*sessionData, error) {
	// Handle PAYLOAD as either string or []byte (depending on database driver)
	var payloadJSON string
	if val, ok := row[dbColumnSessionData].(string); ok && val != "" {
		payloadJSON = val
	} else if val, ok := row[dbColumnSessionData].([]byte); ok && len(val) > 0 {
		payloadJSON = string(val)
	} else {
		s.logger.Error("SESSION_DATA is missing or of unexpected type",
			log.String("type", fmt.Sprintf("%T", row[dbColumnSessionData])))
		return nil, fmt.Errorf("SESSION_DATA is missing or invalid")
	}

	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(payloadJSON), &jsonData); err != nil {
		s.logger.Error("Failed to unmarshal session payload JSON",
			log.Error(err))
		return nil, err
	}

	// Extract challenge
	challengeStr, _ := jsonData[jsonKeyChallenge].(string)

	sessionData := &sessionData{
		Challenge: challengeStr,
	}

	// Decode RelyingPartyID (REQUIRED)
	if rpID, ok := jsonData[jsonKeyRelyingPartyID].(string); ok {
		sessionData.RelyingPartyID = rpID
	}

	// Decode user ID if present (REQUIRED)
	if userIDStr, ok := jsonData[jsonKeyUserID].(string); ok && userIDStr != "" {
		userIDBytes, err := base64.StdEncoding.DecodeString(userIDStr)
		if err != nil {
			s.logger.Error("Failed to decode UserID from session data", log.Error(err))
			return nil, err
		}
		sessionData.UserID = userIDBytes
	}

	// Decode Expires time (REQUIRED)
	if expiresUnix, ok := jsonData[jsonKeyExpires].(float64); ok {
		sessionData.Expires = time.Unix(int64(expiresUnix), 0)
	}

	// Decode user verification
	if userVerificationStr, ok := jsonData[jsonKeyUserVerification].(string); ok {
		sessionData.UserVerification = userVerificationRequirement(userVerificationStr)
	}

	// Decode extensions if present
	if extensions, ok := jsonData[jsonKeyExtensions].(map[string]interface{}); ok {
		sessionData.Extensions = extensions
	}

	// Decode allowed credentials if present
	if allowedCredsJSON, ok := jsonData[jsonKeyAllowedCredentials].([]interface{}); ok {
		allowedCreds := make([][]byte, len(allowedCredsJSON))
		for i, credJSON := range allowedCredsJSON {
			credStr, _ := credJSON.(string)
			credBytes, err := base64.StdEncoding.DecodeString(credStr)
			if err != nil {
				return nil, err
			}
			allowedCreds[i] = credBytes
		}
		sessionData.AllowedCredentialIDs = allowedCreds
	}

	// Decode credential parameters if present (for registration)
	if credParamsJSON, ok := jsonData[jsonKeyCredParams].([]interface{}); ok {
		credParams := make([]credentialParameter, len(credParamsJSON))
		for i, paramJSON := range credParamsJSON {
			paramMap, _ := paramJSON.(map[string]interface{})
			if paramMap != nil {
				if typ, ok := paramMap["type"].(string); ok {
					credParams[i].Type = credentialType(typ)
				}
				if alg, ok := paramMap["alg"].(float64); ok {
					credParams[i].Algorithm = webauthncose.COSEAlgorithmIdentifier(int(alg))
				}
			}
		}
		sessionData.CredParams = credParams
	}

	// Decode mediation if present
	if mediationStr, ok := jsonData[jsonKeyMediation].(string); ok {
		sessionData.Mediation = credentialMediationRequirement(mediationStr)
	}

	return sessionData, nil
}

// getMapKeys returns the keys of a map for debugging.
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
