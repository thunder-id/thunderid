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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entity"
)

type WebAuthnUserTestSuite struct {
	suite.Suite
}

func TestWebAuthnUserTestSuite(t *testing.T) {
	suite.Run(t, new(WebAuthnUserTestSuite))
}

func (suite *WebAuthnUserTestSuite) TestNewWebAuthnUser() {
	userID := testUserID
	userName := "johndoe"
	displayName := "John Doe"
	credentials := []webauthnCredential{}

	webAuthnUser := newWebAuthnUser(userID, userName, displayName, credentials)

	suite.NotNil(webAuthnUser)
	suite.Equal([]byte(userID), webAuthnUser.WebAuthnID())
	suite.Equal(userName, webAuthnUser.WebAuthnName())
	suite.Equal(displayName, webAuthnUser.WebAuthnDisplayName())
	suite.Equal(credentials, webAuthnUser.WebAuthnCredentials())
}

func (suite *WebAuthnUserTestSuite) TestWebAuthnUser_Methods() {
	userID := testUserID
	userName := "johndoe"
	displayName := "John Doe"
	credentials := []webauthnCredential{
		{
			ID: []byte("credential1"),
		},
	}

	webAuthnUser := &webAuthnUser{
		id:          []byte(userID),
		name:        userName,
		displayName: displayName,
		credentials: credentials,
	}

	suite.Equal([]byte(userID), webAuthnUser.WebAuthnID())
	suite.Equal(userName, webAuthnUser.WebAuthnName())
	suite.Equal(displayName, webAuthnUser.WebAuthnDisplayName())
	suite.Len(webAuthnUser.WebAuthnCredentials(), 1)
	suite.Equal([]byte("credential1"), webAuthnUser.WebAuthnCredentials()[0].ID)
}

func (suite *WebAuthnUserTestSuite) TestNewWebAuthnUserFromCoreUser_WithFullAttributes() {
	attrs := json.RawMessage(`{"given_name":"John","family_name":"Doe","username":"johndoe"}`)
	coreUser := &entity.Entity{
		ID:         "user123",
		Category:   entity.EntityCategoryUser,
		Type:       "person",
		OUID:       "org123",
		Attributes: attrs,
	}
	credentials := []webauthnCredential{}

	webAuthnUser := newWebAuthnUserFromEntity(coreUser, credentials)

	suite.NotNil(webAuthnUser)
	suite.Equal([]byte("user123"), webAuthnUser.WebAuthnID())
	suite.Equal("johndoe", webAuthnUser.WebAuthnName())
	suite.Equal("John Doe", webAuthnUser.WebAuthnDisplayName())
	suite.Equal(credentials, webAuthnUser.WebAuthnCredentials())
}

func (suite *WebAuthnUserTestSuite) TestNewWebAuthnUserFromCoreUser_WithEmailOnly() {
	attrs := json.RawMessage(`{"email":"john@example.com"}`)
	coreUser := &entity.Entity{
		ID:         "user123",
		Category:   entity.EntityCategoryUser,
		Attributes: attrs,
	}
	credentials := []webauthnCredential{}

	webAuthnUser := newWebAuthnUserFromEntity(coreUser, credentials)

	suite.NotNil(webAuthnUser)
	suite.Equal([]byte("user123"), webAuthnUser.WebAuthnID())
	suite.Equal("john@example.com", webAuthnUser.WebAuthnName())
	suite.Equal("user123", webAuthnUser.WebAuthnDisplayName()) // Falls back to ID
}

func (suite *WebAuthnUserTestSuite) TestNewWebAuthnUserFromCoreUser_NoAttributes() {
	coreUser := &entity.Entity{
		ID:       "user123",
		Category: entity.EntityCategoryUser,
	}
	credentials := []webauthnCredential{}

	webAuthnUser := newWebAuthnUserFromEntity(coreUser, credentials)

	suite.NotNil(webAuthnUser)
	suite.Equal([]byte("user123"), webAuthnUser.WebAuthnID())
	suite.Equal("user123", webAuthnUser.WebAuthnName())
	suite.Equal("user123", webAuthnUser.WebAuthnDisplayName())
}
