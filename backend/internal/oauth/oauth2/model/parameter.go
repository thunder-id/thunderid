/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package model

import "github.com/thunder-id/thunderid/internal/system/utils"

// OAuthParameters represents the parameters required for OAuth2 authorization.
type OAuthParameters struct {
	State               string
	ClientID            string
	RedirectURI         string
	ResponseType        string
	StandardScopes      []string
	PermissionScopes    []string
	CodeChallenge       string
	CodeChallengeMethod string
	Resources           []string
	ClaimsRequest       *ClaimsRequest
	ClaimsLocales       string
	Nonce               string
	AcrValues           string
}

// ClaimsRequest represents the OIDC claims request parameter structure.
type ClaimsRequest struct {
	UserInfo map[string]*IndividualClaimRequest `json:"userinfo,omitempty"`
	IDToken  map[string]*IndividualClaimRequest `json:"id_token,omitempty"`
}

// IndividualClaimRequest represents a request for an individual claim.
type IndividualClaimRequest struct {
	Essential bool          `json:"essential,omitempty"`
	Value     interface{}   `json:"value,omitempty"`
	Values    []interface{} `json:"values,omitempty"`
}

// IsEmpty returns true if the ClaimsRequest has no claims requested.
func (cr *ClaimsRequest) IsEmpty() bool {
	return cr == nil || (len(cr.UserInfo) == 0 && len(cr.IDToken) == 0)
}

// MatchesValue checks if the given value matches the requested value or values.
// Returns true if no value/values constraint is specified, or if the value matches.
func (icr *IndividualClaimRequest) MatchesValue(value interface{}) bool {
	if icr == nil {
		return true
	}

	// If no value constraints, any value matches
	if icr.Value == nil && len(icr.Values) == 0 {
		return true
	}

	// Check single value match
	if icr.Value != nil {
		return utils.CompareValues(value, icr.Value)
	}

	// Check values array match
	for _, v := range icr.Values {
		if utils.CompareValues(value, v) {
			return true
		}
	}

	return false
}
