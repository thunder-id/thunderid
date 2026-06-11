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

// Package sdjwt implements verification of Selective Disclosure JWTs
// (draft-ietf-oauth-selective-disclosure-jwt) including Key Binding JWTs.
// It operates only on keys and data passed by the caller; trust decisions
// (resolving the issuer key, checking the credential type) are the caller's
// responsibility.
package sdjwt

import (
	"crypto"
	"errors"
	"time"
)

const (
	// separator delimits the issuer-signed JWT, the disclosures, and the
	// optional Key Binding JWT in the combined SD-JWT format.
	separator = "~"
	// keyBindingType is the required "typ" header value of a Key Binding JWT.
	keyBindingType = "kb+jwt"
	// defaultHashAlg is the digest algorithm assumed when "_sd_alg" is absent.
	defaultHashAlg = "sha-256"

	claimSD       = "_sd"
	claimSDAlg    = "_sd_alg"
	claimArrayElt = "..."
	claimCnf      = "cnf"
	claimSDHash   = "sd_hash"
	claimNonce    = "nonce"
	claimAud      = "aud"
	claimIat      = "iat"
)

// Errors returned by the package. Each maps to a layer of the verification
// stack so callers (and tests) can assert exactly where a presentation failed.
var (
	// ErrInvalidFormat indicates the combined SD-JWT could not be parsed.
	ErrInvalidFormat = errors.New("sdjwt: invalid format")
	// ErrInvalidDisclosure indicates a disclosure was malformed.
	ErrInvalidDisclosure = errors.New("sdjwt: invalid disclosure")
	// ErrUnsupportedHashAlg indicates an unsupported "_sd_alg" value.
	ErrUnsupportedHashAlg = errors.New("sdjwt: unsupported _sd_alg")

	// ErrIssuerSignature indicates the issuer-signed JWT signature is invalid
	// (credential layer).
	ErrIssuerSignature = errors.New("sdjwt: issuer signature verification failed")

	// ErrUnusedDisclosure indicates a provided disclosure was not referenced by
	// any digest in the credential (selective-disclosure layer).
	ErrUnusedDisclosure = errors.New("sdjwt: disclosure does not match any digest")
	// ErrDuplicateDigest indicates a digest was referenced more than once
	// (selective-disclosure layer).
	ErrDuplicateDigest = errors.New("sdjwt: digest referenced more than once")
	// ErrClaimCollision indicates a disclosed claim collided with an existing
	// claim or used a reserved name (selective-disclosure layer).
	ErrClaimCollision = errors.New("sdjwt: disclosed claim collides with existing claim")

	// ErrMissingKeyBinding indicates a Key Binding JWT was required but absent
	// (holder-binding layer).
	ErrMissingKeyBinding = errors.New("sdjwt: key binding JWT required but missing")
	// ErrMissingConfirmationKey indicates the issuer credential carried no
	// holder confirmation key (cnf.jwk) (holder-binding layer).
	ErrMissingConfirmationKey = errors.New("sdjwt: missing cnf.jwk confirmation key")
	// ErrKeyBindingSignature indicates the Key Binding JWT signature is invalid
	// (holder-binding layer).
	ErrKeyBindingSignature = errors.New("sdjwt: key binding signature verification failed")
	// ErrKeyBindingClaims indicates a Key Binding JWT claim (typ, aud, nonce,
	// iat, or sd_hash) failed validation (holder-binding layer).
	ErrKeyBindingClaims = errors.New("sdjwt: key binding claim validation failed")
)

// Disclosure is a single parsed SD-JWT disclosure.
type Disclosure struct {
	// Raw is the base64url-encoded disclosure string as it appears on the wire.
	Raw string
	// Digest is base64url(hash(Raw)) under the credential's _sd_alg.
	Digest string
	// Salt is the disclosure salt.
	Salt string
	// ClaimName is the object property name. Empty for array-element disclosures.
	ClaimName string
	// Value is the disclosed claim value.
	Value interface{}
	// ArrayElement reports whether this is an array-element disclosure.
	ArrayElement bool
}

// Presentation is a parsed combined-format SD-JWT with an optional Key Binding JWT.
type Presentation struct {
	IssuerJWT     string
	Disclosures   []Disclosure
	KeyBindingJWT string
}

// HasKeyBinding reports whether the presentation carries a Key Binding JWT.
func (p *Presentation) HasKeyBinding() bool {
	return p.KeyBindingJWT != ""
}

// VerifyOptions controls verification of a presentation.
type VerifyOptions struct {
	// IssuerKey verifies the issuer-signed JWT. Required.
	IssuerKey crypto.PublicKey
	// RequireKeyBinding fails verification when no Key Binding JWT is present.
	RequireKeyBinding bool
	// ExpectedAudience is the value the Key Binding JWT "aud" must equal.
	ExpectedAudience string
	// ExpectedNonce is the value the Key Binding JWT "nonce" must equal.
	ExpectedNonce string
	// Now is the reference time for iat checks. Defaults to time.Now() when zero.
	Now time.Time
	// Leeway tolerates clock skew on the Key Binding JWT iat. Defaults to 0.
	Leeway time.Duration
	// MaxIATAge rejects KB-JWTs with iat older than Now-MaxIATAge (replay protection). 0 = disabled.
	MaxIATAge time.Duration
}

// VerifiedCredential is the result of a successful verification.
type VerifiedCredential struct {
	// Claims is the issuer payload with all provided disclosures resolved in
	// place and the SD machinery (_sd, _sd_alg, "...") removed.
	Claims map[string]interface{}
	// DisclosedPaths lists the dotted paths of object claims revealed via
	// selective disclosure.
	DisclosedPaths []string
	// ConfirmationKey is the holder confirmation JWK (cnf.jwk), when present.
	ConfirmationKey map[string]interface{}
}
