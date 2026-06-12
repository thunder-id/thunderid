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

package manager

import (
	"encoding/json"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
)

// AuthUser accumulates per-provider authentication state produced during flow execution.
// All fields are unexported; use the manager methods to interact with this type.
type AuthUser struct {
	entityReferenceToken any
	entityReference      *authnprovidercm.EntityReference
	attributeToken       any
	attributes           *authnprovidercm.AttributesResponse
}

// IsAuthenticated reports whether this AuthUser has been populated by a successful
// authentication.
func (a AuthUser) IsAuthenticated() bool {
	return (a.entityReference != nil || a.entityReferenceToken != nil) &&
		(a.attributes != nil || a.attributeToken != nil)
}

// authUserJSON is the internal proxy used for JSON serialization of AuthUser.
type authUserJSON struct {
	EntityReferenceToken any                                 `json:"entityReferenceToken"`
	EntityReference      *authnprovidercm.EntityReference    `json:"entityReference,omitempty"`
	AttributeToken       any                                 `json:"attributeToken"`
	Attributes           *authnprovidercm.AttributesResponse `json:"attributes,omitempty"`
}

// MarshalJSON implements json.Marshaler.
func (a *AuthUser) MarshalJSON() ([]byte, error) {
	proxy := authUserJSON{
		EntityReferenceToken: a.entityReferenceToken,
		EntityReference:      a.entityReference,
		AttributeToken:       a.attributeToken,
		Attributes:           a.attributes,
	}

	return json.Marshal(proxy)
}

// UnmarshalJSON implements json.Unmarshaler.
func (a *AuthUser) UnmarshalJSON(b []byte) error {
	var proxy authUserJSON
	if err := json.Unmarshal(b, &proxy); err != nil {
		return err
	}

	a.entityReferenceToken = proxy.EntityReferenceToken
	a.entityReference = proxy.EntityReference
	a.attributeToken = proxy.AttributeToken
	a.attributes = proxy.Attributes

	return nil
}
