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

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/utils"
)

// OAuthParameters represents the parameters required for OAuth2 authorization.
type OAuthParameters struct {
	State               string
	ClientID            string
	RedirectURI         string
	RedirectURIProvided bool
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
	MaxAge              string
	DPoPJkt             string
	Prompt              string
}

// VerifiedClaimsMember is the OIDC Identity Assurance member name that may appear in the
// userinfo and id_token sections of the claims request. Its request form is the nested IDA
// structure (verification + claims) rather than the {essential, value, values} shape, so each
// section's verified_claims is held separately in VerifiedUserInfo and VerifiedIDToken and
// excluded from all normal-claim processing.
const VerifiedClaimsMember = "verified_claims"

// jsonNull is the JSON wire representation of a null value.
const jsonNull = "null"

// ClaimsRequest represents the OIDC claims request parameter structure.
// UserInfo and IDToken carry only normal claims; each section's verified_claims IDA structure is
// held separately in VerifiedUserInfo and VerifiedIDToken, normalized to an array of typed
// VerifiedClaimsRequest entries. On the wire verified_claims remains nested under its
// userinfo/id_token object, handled by the custom MarshalJSON/UnmarshalJSON.
type ClaimsRequest struct {
	UserInfo         map[string]*IndividualClaimRequest `json:"-"`
	VerifiedUserInfo []*VerifiedClaimsRequest           `json:"-"`
	IDToken          map[string]*IndividualClaimRequest `json:"-"`
	VerifiedIDToken  []*VerifiedClaimsRequest           `json:"-"`
}

// IndividualClaimRequest represents a request for an individual claim.
type IndividualClaimRequest struct {
	Essential bool          `json:"essential,omitempty"`
	Value     interface{}   `json:"value,omitempty"`
	Values    []interface{} `json:"values,omitempty"`
}

// VerifiedClaimsRequest is one normalized OIDC Identity Assurance verified_claims request entry.
// Only the verification and claims members are modeled; other IDA members are dropped on decode.
type VerifiedClaimsRequest struct {
	Verification *VerificationRequest               `json:"verification"`
	Claims       map[string]*IndividualClaimRequest `json:"claims"`
}

// VerificationRequest is the verification element of a verified_claims request. TrustFramework is
// required and is a constrainable element (JSON null, decoded to a nil request meaning "any
// framework", or an object honoring the value/values grammar); Time is the optional constraint.
type VerificationRequest struct {
	TrustFramework *TrustFrameworkRequest   `json:"trust_framework"`
	Time           *VerificationTimeRequest `json:"time,omitempty"`
}

// TrustFrameworkRequest is the constrainable trust_framework element of a verification request.
// Value and Values are mutually exclusive string constraints per the OIDC constraint grammar.
type TrustFrameworkRequest struct {
	Value     string   `json:"value,omitempty"`
	Values    []string `json:"values,omitempty"`
	Essential bool     `json:"essential,omitempty"`
}

// Validate checks the trust_framework constraint grammar: value and values are mutually exclusive,
// and a present values array must be non-empty.
func (tf *TrustFrameworkRequest) Validate() error {
	if tf.Value != "" && len(tf.Values) > 0 {
		return errors.New("has both 'value' and 'values' specified (mutually exclusive per OIDC spec)")
	}
	if tf.Values != nil && len(tf.Values) == 0 {
		return errors.New("has empty 'values' array (must contain at least one value)")
	}
	return nil
}

// VerificationTimeRequest is the optional time constraint of a verification element. MaxAge is the
// maximum age of the verification in seconds (a non-negative integer) when present.
type VerificationTimeRequest struct {
	MaxAge *int64 `json:"max_age,omitempty"`
}

// UnmarshalJSON decodes a single verified_claims entry, requiring a verification element and a
// non-empty claims object. Each claims entry follows the normal-claim constraint grammar.
func (vcr *VerifiedClaimsRequest) UnmarshalJSON(data []byte) error {
	var raw struct {
		Verification *VerificationRequest       `json:"verification"`
		Claims       map[string]json.RawMessage `json:"claims"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if raw.Verification == nil {
		return errors.New("'verified_claims' is missing the required 'verification' member")
	}
	if raw.Claims == nil {
		return errors.New("'verified_claims' is missing the required 'claims' member")
	}
	if len(raw.Claims) == 0 {
		return errors.New("'verified_claims.claims' must request at least one claim")
	}

	vcr.Verification = raw.Verification
	vcr.Claims = make(map[string]*IndividualClaimRequest, len(raw.Claims))
	for name, value := range raw.Claims {
		if string(value) == jsonNull {
			vcr.Claims[name] = nil
			continue
		}
		claimReq, err := DecodeIndividualClaimRequest(value)
		if err != nil {
			return fmt.Errorf("claim '%s' in verified_claims.claims is malformed: %w", name, err)
		}
		if err := claimReq.Validate(); err != nil {
			return fmt.Errorf("claim '%s' in verified_claims.claims %w", name, err)
		}
		vcr.Claims[name] = claimReq
	}
	return nil
}

// decodeTrustFramework decodes the trust_framework constrainable element per OIDC Identity
// Assurance: JSON null yields a nil request (no constraint), an object is decoded and checked
// against the value/values grammar, and any other shape (a bare scalar, an array) is rejected.
func decodeTrustFramework(raw json.RawMessage) (*TrustFrameworkRequest, error) {
	trimmed := bytes.TrimSpace(raw)
	if string(trimmed) == jsonNull {
		return nil, nil
	}
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, errors.New("must be null or an object")
	}
	var tf TrustFrameworkRequest
	if err := json.Unmarshal(trimmed, &tf); err != nil {
		return nil, fmt.Errorf("is malformed: %w", err)
	}
	if err := tf.Validate(); err != nil {
		return nil, err
	}
	return &tf, nil
}

// UnmarshalJSON decodes a verification element, enforcing the presence of trust_framework and
// validating the optional time constraint.
func (vr *VerificationRequest) UnmarshalJSON(data []byte) error {
	var raw struct {
		TrustFramework json.RawMessage `json:"trust_framework"`
		Time           json.RawMessage `json:"time"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if raw.TrustFramework == nil {
		return errors.New(
			"'verified_claims.verification' is missing the required 'trust_framework' member")
	}
	trustFramework, err := decodeTrustFramework(raw.TrustFramework)
	if err != nil {
		return fmt.Errorf("'verified_claims.verification.trust_framework' %w", err)
	}
	vr.TrustFramework = trustFramework

	if raw.Time == nil || string(raw.Time) == jsonNull {
		return nil
	}

	var timeObj struct {
		MaxAge json.RawMessage `json:"max_age"`
	}
	if err := json.Unmarshal(raw.Time, &timeObj); err != nil {
		return errors.New("'verified_claims.verification.time' must be an object")
	}
	vr.Time = &VerificationTimeRequest{}
	if timeObj.MaxAge == nil {
		return nil
	}

	var maxAgeNum json.Number
	if err := json.Unmarshal(timeObj.MaxAge, &maxAgeNum); err != nil {
		return errors.New(
			"'verified_claims.verification.time.max_age' must be a non-negative integer")
	}
	maxAge, err := maxAgeNum.Int64()
	if err != nil || maxAge < 0 {
		return fmt.Errorf(
			"'verified_claims.verification.time.max_age' must be a non-negative integer, got %s",
			string(timeObj.MaxAge))
	}
	vr.Time.MaxAge = &maxAge
	return nil
}

// decodeVerifiedClaims normalizes the verified_claims member, which may be a single object or an
// array of objects on the wire, into an array of typed entries.
func decodeVerifiedClaims(raw json.RawMessage) ([]*VerifiedClaimsRequest, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, errors.New("'verified_claims' must be an object or an array of objects")
	}

	switch trimmed[0] {
	case '[':
		var entries []*VerifiedClaimsRequest
		if err := json.Unmarshal(trimmed, &entries); err != nil {
			return nil, err
		}
		if len(entries) == 0 {
			return nil, errors.New("'verified_claims' array must contain at least one entry")
		}
		for _, entry := range entries {
			if entry == nil {
				return nil, errors.New("'verified_claims' array must not contain null entries")
			}
		}
		return entries, nil
	case '{':
		var entry VerifiedClaimsRequest
		if err := json.Unmarshal(trimmed, &entry); err != nil {
			return nil, err
		}
		return []*VerifiedClaimsRequest{&entry}, nil
	default:
		return nil, errors.New("'verified_claims' must be an object or an array of objects")
	}
}

// decodeClaimsSection splits one section (userinfo or id_token) of the claims request into its
// normal claims, each decoded as *IndividualClaimRequest, and the verified_claims member held
// separately. An absent or empty section yields a nil normal map and nil verified entries.
func decodeClaimsSection(section string, raw map[string]json.RawMessage) (
	map[string]*IndividualClaimRequest, []*VerifiedClaimsRequest, error) {
	if len(raw) == 0 {
		return nil, nil, nil
	}

	normal := make(map[string]*IndividualClaimRequest, len(raw))
	var verified []*VerifiedClaimsRequest
	for name, value := range raw {
		if name == VerifiedClaimsMember {
			entries, err := decodeVerifiedClaims(value)
			if err != nil {
				return nil, nil, err
			}
			verified = entries
			continue
		}
		if string(value) == jsonNull {
			normal[name] = nil
			continue
		}
		claimReq, err := DecodeIndividualClaimRequest(value)
		if err != nil {
			return nil, nil, fmt.Errorf("%s claim %q is malformed: %w", section, name, err)
		}
		normal[name] = claimReq
	}
	return normal, verified, nil
}

// UnmarshalJSON decodes a ClaimsRequest, splitting each of the userinfo and id_token objects into
// normal claims (each decoded as *IndividualClaimRequest) and the verified_claims member (held
// separately in VerifiedUserInfo / VerifiedIDToken). This applies on every decode path:
// ParseClaimsRequest as well as the Redis-backed request/code/PAR stores that restore the struct
// with a plain json.Unmarshal.
func (cr *ClaimsRequest) UnmarshalJSON(data []byte) error {
	var raw struct {
		UserInfo map[string]json.RawMessage `json:"userinfo"`
		IDToken  map[string]json.RawMessage `json:"id_token"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	userInfo, verifiedUserInfo, err := decodeClaimsSection("userinfo", raw.UserInfo)
	if err != nil {
		return err
	}
	idToken, verifiedIDToken, err := decodeClaimsSection("id_token", raw.IDToken)
	if err != nil {
		return err
	}

	cr.UserInfo = userInfo
	cr.VerifiedUserInfo = verifiedUserInfo
	cr.IDToken = idToken
	cr.VerifiedIDToken = verifiedIDToken
	return nil
}

// marshalClaimsSection rebuilds one section's wire object from its normal claims and verified_claims
// member, re-nesting verified_claims under the section. Returns nil when the section has nothing.
func marshalClaimsSection(normal map[string]*IndividualClaimRequest,
	verified []*VerifiedClaimsRequest) map[string]any {
	if len(normal) == 0 && len(verified) == 0 {
		return nil
	}

	section := make(map[string]any, len(normal)+1)
	for name, claim := range normal {
		section[name] = claim
	}
	if len(verified) > 0 {
		section[VerifiedClaimsMember] = verified
	}
	return section
}

// MarshalJSON encodes a ClaimsRequest back into the OIDC wire shape, re-nesting each section's
// verified_claims under its userinfo/id_token object so the serialized form round-trips through
// the stores and the access-token claims_request claim unchanged.
func (cr *ClaimsRequest) MarshalJSON() ([]byte, error) {
	out := make(map[string]any, 2)

	if userInfo := marshalClaimsSection(cr.UserInfo, cr.VerifiedUserInfo); userInfo != nil {
		out["userinfo"] = userInfo
	}
	if idToken := marshalClaimsSection(cr.IDToken, cr.VerifiedIDToken); idToken != nil {
		out["id_token"] = idToken
	}

	return json.Marshal(out)
}

// DecodeIndividualClaimRequest decodes a raw claim request value into an *IndividualClaimRequest.
// A nil value ("email": null) means "requested, no constraint" and yields a nil request.
func DecodeIndividualClaimRequest(raw any) (*IndividualClaimRequest, error) {
	if raw == nil {
		return nil, nil
	}
	if icr, ok := raw.(*IndividualClaimRequest); ok {
		return icr, nil
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var icr IndividualClaimRequest
	if err := json.Unmarshal(data, &icr); err != nil {
		return nil, err
	}
	return &icr, nil
}

// IsEmpty returns true if the ClaimsRequest has no claims requested.
func (cr *ClaimsRequest) IsEmpty() bool {
	return cr == nil || (len(cr.UserInfo) == 0 && len(cr.VerifiedUserInfo) == 0 &&
		len(cr.IDToken) == 0 && len(cr.VerifiedIDToken) == 0)
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

// Validate checks an individual claim request against the OIDC constraint grammar: value and
// values are mutually exclusive, and a present values array must be non-empty. A nil request
// (requested without constraint) is always valid.
func (icr *IndividualClaimRequest) Validate() error {
	if icr == nil {
		return nil
	}

	if icr.Value != nil && len(icr.Values) > 0 {
		return errors.New("has both 'value' and 'values' specified (mutually exclusive per OIDC spec)")
	}

	if icr.Values != nil && len(icr.Values) == 0 {
		return errors.New("has empty 'values' array (must contain at least one value)")
	}

	return nil
}
