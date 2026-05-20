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

type providerKey string

const defaultProvider providerKey = "default"

type providerData struct {
	token                     string
	attributes                *authnprovidercm.AttributesResponse
	isAttributeValuesIncluded bool
}

// AuthUser accumulates per-provider authentication state produced during flow execution.
// All fields are unexported; use the manager methods to interact with this type.
type AuthUser struct {
	userID            string
	userType          string
	ouID              string
	providersAuthData map[providerKey]providerData
}

// AuthnBasicResult is returned by AuthenticateUser and carries the identity fields
// extracted from the provider's authentication result.
type AuthnBasicResult struct {
	UserID   string
	OUID     string
	UserType string

	// Federated authentication fields. Set when the authentication flow is federated
	// and no internal user was found (IsExistingUser = false).
	ExternalSub     string
	ExternalClaims  map[string]interface{}
	IsExistingUser  bool
	IsAmbiguousUser bool
}

func (a *AuthUser) setIdentity(userID, userType, ouID string) {
	a.userID = userID
	a.userType = userType
	a.ouID = ouID
}

// p is currently always defaultProvider; the parameter exists to support multiple providers without a signature change.
func (a *AuthUser) setProviderData(p providerKey, data providerData) { //nolint:unparam
	if a.providersAuthData == nil {
		a.providersAuthData = make(map[providerKey]providerData)
	}
	a.providersAuthData[p] = data
}

// p is currently always defaultProvider; the parameter exists to support multiple providers without a signature change.
func (a *AuthUser) getProviderData(p providerKey) (providerData, bool) { //nolint:unparam
	data, ok := a.providersAuthData[p]
	return data, ok
}

// IsAuthenticated reports whether this AuthUser has been populated by a successful
// authentication.
func (a AuthUser) IsAuthenticated() bool {
	return a.userID != ""
}

// authUserJSON is the internal proxy used for JSON serialization of AuthUser.
type authUserJSON struct {
	UserID            string                      `json:"userId"`
	UserType          string                      `json:"userType"`
	OUID              string                      `json:"ouId"`
	ProvidersAuthData map[string]providerDataJSON `json:"providersAuthData"`
}

// providerDataJSON is the internal proxy used for JSON serialization of providerData.
type providerDataJSON struct {
	Token                     string                              `json:"token"`
	Attributes                *authnprovidercm.AttributesResponse `json:"attributes,omitempty"`
	IsAttributeValuesIncluded bool                                `json:"isAttributeValuesIncluded"`
}

// MarshalJSON implements json.Marshaler.
func (a *AuthUser) MarshalJSON() ([]byte, error) {
	proxy := authUserJSON{
		UserID:            a.userID,
		UserType:          a.userType,
		OUID:              a.ouID,
		ProvidersAuthData: make(map[string]providerDataJSON, len(a.providersAuthData)),
	}

	for p, data := range a.providersAuthData {
		proxy.ProvidersAuthData[string(p)] = providerDataJSON{
			Token:                     data.token,
			Attributes:                data.attributes,
			IsAttributeValuesIncluded: data.isAttributeValuesIncluded,
		}
	}

	return json.Marshal(proxy)
}

// UnmarshalJSON implements json.Unmarshaler.
func (a *AuthUser) UnmarshalJSON(b []byte) error {
	var proxy authUserJSON
	if err := json.Unmarshal(b, &proxy); err != nil {
		return err
	}

	a.userID = proxy.UserID
	a.userType = proxy.UserType
	a.ouID = proxy.OUID

	a.providersAuthData = make(map[providerKey]providerData, len(proxy.ProvidersAuthData))

	for k, v := range proxy.ProvidersAuthData {
		a.providersAuthData[providerKey(k)] = providerData{
			token:                     v.Token,
			attributes:                v.Attributes,
			isAttributeValuesIncluded: v.IsAttributeValuesIncluded,
		}
	}

	return nil
}
