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
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/thunder-id/thunderid/internal/entity"
)

// webAuthnUser adapts generic model to implement the webauthn.User interface.
type webAuthnUser struct {
	id          []byte
	name        string
	displayName string
	credentials []webauthnCredential
}

var _ webauthn.User = (*webAuthnUser)(nil)

// WebAuthnID returns the user's ID as required by webauthn.User interface.
func (u *webAuthnUser) WebAuthnID() []byte {
	return u.id
}

// WebAuthnName returns the user's name as required by webauthn.User interface.
func (u *webAuthnUser) WebAuthnName() string {
	return u.name
}

// WebAuthnDisplayName returns the user's display name as required by webauthn.User interface.
func (u *webAuthnUser) WebAuthnDisplayName() string {
	return u.displayName
}

// WebAuthnCredentials returns the user's credentials as required by webauthn.User interface.
func (u *webAuthnUser) WebAuthnCredentials() []webauthnCredential {
	return u.credentials
}

// newWebAuthnUser creates a new WebAuthn user from raw identity fields.
func newWebAuthnUser(entityID string, name, displayName string, credentials []webauthnCredential) *webAuthnUser {
	return &webAuthnUser{
		id:          []byte(entityID),
		name:        name,
		displayName: displayName,
		credentials: credentials,
	}
}

// newWebAuthnUserFromEntity creates a WebAuthn user from any entity, using its attributes
// to derive the display name and username when available.
func newWebAuthnUserFromEntity(e *entity.Entity, credentials []webauthnCredential) *webAuthnUser {
	displayName, name := extractWebAuthnIdentity(e)
	return newWebAuthnUser(e.ID, name, displayName, credentials)
}
