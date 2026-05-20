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
	"crypto/rand"
	"encoding/base64"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const (
	// sessionKeyLength is the length of the random session key in bytes.
	sessionKeyLength = 32
	// sessionTTLSeconds is the session time-to-live in seconds.
	sessionTTLSeconds = 120
)

// generateSessionKey generates a random base64-encoded session key.
func generateSessionKey() (string, error) {
	bytes := make([]byte, sessionKeyLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(bytes), nil
}

// storeSessionData stores session data in the database and returns a session key.
func (w *passkeyService) storeSessionData(
	sessionData *sessionData,
) (string, *serviceerror.ServiceError) {
	// Generate a random session key
	sessionKey, err := generateSessionKey()
	if err != nil {
		return "", &serviceerror.InternalServerError
	}

	// Store session data in database
	err = w.sessionStore.storeSession(sessionKey, sessionData, sessionTTLSeconds)
	if err != nil {
		return "", &serviceerror.InternalServerError
	}

	return sessionKey, nil
}

// retrieveSessionData retrieves the session data from the database using the session key.
func (w *passkeyService) retrieveSessionData(
	sessionKey string,
) (*sessionData, string, string, *serviceerror.ServiceError) {
	// Retrieve session data from database
	session, err := w.sessionStore.retrieveSession(sessionKey)
	if err != nil {
		w.logger.Debug("Failed to retrieve passkey session", log.Error(err))
		return nil, "", "", &serviceerror.InternalServerError
	}

	if session == nil {
		return nil, "", "", &ErrorSessionExpired
	}

	return session, string(session.UserID), session.RelyingPartyID, nil
}

// clearSessionData removes the session data from the database.
func (w *passkeyService) clearSessionData(sessionKey string) {
	// Remove session from database
	_ = w.sessionStore.deleteSession(sessionKey)
}
