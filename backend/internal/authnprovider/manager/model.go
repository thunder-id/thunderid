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
	"fmt"
)

// providerUserState represents the local user resolution state from the authn provider.
type providerUserState string

// ProviderUserState values representing local user resolution outcomes.
const (
	ProviderUserStateExists    providerUserState = "exists"
	ProviderUserStateNotExists providerUserState = "not_exists"
	ProviderUserStateAmbiguous providerUserState = "ambiguous"
)

// AuthenticatorReference represents an engaged authenticator in the authentication flow.
type AuthenticatorReference struct {
	// Authenticator is the name of the authenticator
	Authenticator string `json:"authenticator"`
	// Step is the step number in the flow where this authenticator was engaged
	Step int `json:"step"`
	// Timestamp is the authenticator engaged time (Unix epoch time in seconds)
	Timestamp int64 `json:"timestamp"`
}

// AuthUser accumulates per-provider authentication state produced during flow execution.
// All fields are unexported; use the manager methods to interact with this type.
type AuthUser struct {
	authHistory []*authResult
	userHistory []*providerUserResult
	userState   providerUserState
}

type authResult struct {
	authenticator     string
	isVerified        bool
	runtimeAttributes map[string]interface{}
	timestamp         int64
}

type providerUserResult struct {
	userID           string
	userType         string
	ouID             string
	attributes       map[string]interface{}
	isValuesIncluded bool
	token            string
	timestamp        int64
}

// IsSet reports whether this AuthUser has been populated (i.e. is not the zero value).
func (a AuthUser) IsSet() bool {
	return len(a.authHistory) > 0 || len(a.userHistory) > 0
}

// GetUserID returns the user ID of the authenticated user, or an empty string if not set.
func (a AuthUser) GetUserID() string {
	if len(a.userHistory) == 0 {
		return ""
	}
	return a.userHistory[len(a.userHistory)-1].userID
}

// GetOUID returns the organizational unit ID of the authenticated user, or an empty string if not set.
func (a AuthUser) GetOUID() string {
	if len(a.userHistory) == 0 {
		return ""
	}
	return a.userHistory[len(a.userHistory)-1].ouID
}

// GetUserType returns the user type of the authenticated user, or an empty string if not set.
func (a AuthUser) GetUserType() string {
	if len(a.userHistory) == 0 {
		return ""
	}
	return a.userHistory[len(a.userHistory)-1].userType
}

// IsLocalUserExists returns true if all authentication steps indicate that a local user exists for
// the authenticated identity.
func (a AuthUser) IsLocalUserExists() bool {
	return a.userState == ProviderUserStateExists
}

// IsLocalUserAmbiguous returns true if any authentication step indicates that the authenticated identity
// is ambiguously mapped to multiple local users.
func (a AuthUser) IsLocalUserAmbiguous() bool {
	return a.userState == ProviderUserStateAmbiguous
}

// HasPendingVerifications returns true if any authentication step is not yet verified.
func (a AuthUser) HasPendingVerifications() bool {
	for _, authResult := range a.authHistory {
		if !authResult.isVerified {
			return true
		}
	}
	return false
}

// IsAuthenticated returns true if all authentication steps are verified and at least one step exists.
func (a AuthUser) IsAuthenticated() bool {
	if len(a.authHistory) == 0 {
		return false
	}
	if a.userState != ProviderUserStateExists {
		return false
	}
	return !a.HasPendingVerifications()
}

// GetLastFederatedSub returns the subject claim from the most recent federated auth result.
func (a *AuthUser) GetLastFederatedSub() string {
	sub, ok := a.GetRuntimeAttribute("sub").(string)
	if ok {
		return sub
	}
	return ""
}

// GetRuntimeAttribute returns the runtime attribute value for the given key from the last auth result.
func (a *AuthUser) GetRuntimeAttribute(key string) interface{} {
	runtimeAttributes := a.GetRuntimeAttributes()
	if runtimeAttributes == nil {
		return nil
	}
	return runtimeAttributes[key]
}

// GetRuntimeAttributes returns all runtime attributes from the last auth result.
func (a *AuthUser) GetRuntimeAttributes() map[string]interface{} {
	if len(a.authHistory) == 0 {
		return nil
	}
	lastAuthResult := a.authHistory[len(a.authHistory)-1]
	return lastAuthResult.runtimeAttributes
}

// GetAuthenticatorReference returns a slice of unique authenticators engaged during the authentication flow.
func (a *AuthUser) GetAuthenticatorReference() []AuthenticatorReference {
	refs := make([]AuthenticatorReference, 0)
	seenAuthenticators := make(map[string]bool)

	for _, authResult := range a.authHistory {
		if seenAuthenticators[authResult.authenticator] {
			continue
		}
		seenAuthenticators[authResult.authenticator] = true
		refs = append(refs, AuthenticatorReference{
			Authenticator: authResult.authenticator,
			Step:          len(refs) + 1,
			Timestamp:     authResult.timestamp,
		})
	}

	return refs
}

func (a *AuthUser) setUserState(newState providerUserState) error {
	if a.userState == ProviderUserStateExists && newState != ProviderUserStateExists {
		return fmt.Errorf("cannot change user state from 'exists' to '%s'", newState)
	}
	a.userState = newState
	return nil
}

// authUserJSON is the internal proxy used for JSON serialization of AuthUser.
type authUserJSON struct {
	AuthHistory []authResultJSON         `json:"authHistory"`
	UserHistory []providerUserResultJSON `json:"userHistory"`
	UserState   string                   `json:"userState"`
}

// authResultJSON is the internal proxy used for JSON serialization of AuthResult.
type authResultJSON struct {
	AuthType          string                 `json:"authType"`
	IsVerified        bool                   `json:"isVerified"`
	RuntimeAttributes map[string]interface{} `json:"runtimeAttributes,omitempty"`
	Timestamp         int64                  `json:"timestamp"`
}

// providerUserResultJSON is the internal proxy used for JSON serialization of ProviderUserResult.
type providerUserResultJSON struct {
	UserID           string                 `json:"userId"`
	UserType         string                 `json:"userType"`
	OUID             string                 `json:"ouId"`
	Attributes       map[string]interface{} `json:"attributes,omitempty"`
	IsValuesIncluded bool                   `json:"isValuesIncluded"`
	Token            string                 `json:"token,omitempty"`
	Timestamp        int64                  `json:"timestamp"`
}

// MarshalJSON implements json.Marshaler.
func (a *AuthUser) MarshalJSON() ([]byte, error) {
	proxy := authUserJSON{
		AuthHistory: make([]authResultJSON, len(a.authHistory)),
		UserHistory: make([]providerUserResultJSON, len(a.userHistory)),
		UserState:   string(a.userState),
	}

	for i, r := range a.authHistory {
		proxy.AuthHistory[i] = authResultJSON{
			AuthType:          r.authenticator,
			IsVerified:        r.isVerified,
			RuntimeAttributes: r.runtimeAttributes,
			Timestamp:         r.timestamp,
		}
	}

	for i, u := range a.userHistory {
		proxy.UserHistory[i] = providerUserResultJSON{
			UserID:           u.userID,
			UserType:         u.userType,
			OUID:             u.ouID,
			Attributes:       u.attributes,
			IsValuesIncluded: u.isValuesIncluded,
			Token:            u.token,
			Timestamp:        u.timestamp,
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

	a.userState = providerUserState(proxy.UserState)
	a.authHistory = make([]*authResult, len(proxy.AuthHistory))
	a.userHistory = make([]*providerUserResult, len(proxy.UserHistory))

	for i, r := range proxy.AuthHistory {
		a.authHistory[i] = &authResult{
			authenticator:     r.AuthType,
			isVerified:        r.IsVerified,
			runtimeAttributes: r.RuntimeAttributes,
			timestamp:         r.Timestamp,
		}
	}

	for i, u := range proxy.UserHistory {
		a.userHistory[i] = &providerUserResult{
			userID:           u.UserID,
			userType:         u.UserType,
			ouID:             u.OUID,
			attributes:       u.Attributes,
			isValuesIncluded: u.IsValuesIncluded,
			token:            u.Token,
			timestamp:        u.Timestamp,
		}
	}

	return nil
}
