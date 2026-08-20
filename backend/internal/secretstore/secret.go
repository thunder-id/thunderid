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

package secretstore

import (
	"errors"
	"fmt"
	"strings"
)

// Kind distinguishes the two ways a secret has to be held.
//
// The distinction is forced by how a secret is used, not by preference. A credential that is only ever
// checked against something a caller presents, such as a password or an OAuth client secret, is held as
// a hash: nothing needs the original, so storing it would be an avoidable risk. A credential that has
// to be replayed to a third party, such as a Vonage API secret or an SMS gateway key, cannot be hashed,
// because the outbound call needs the value itself.
type Kind string

const (
	// KindHash holds a one-way hash with the parameters needed to verify against it.
	KindHash Kind = "hash"
	// KindValue holds the secret itself, for credentials that must be replayed to a third party.
	KindValue Kind = "value"
)

// HashParameters carries what a verifier needs alongside the hash. The field names mirror the stored
// credential format so a hash produced by ThunderID can be handed over unchanged.
type HashParameters struct {
	Salt        string `json:"salt,omitempty"`
	Iterations  int    `json:"iterations,omitempty"`
	KeySize     int    `json:"keySize,omitempty"`
	Memory      int    `json:"memory,omitempty"`
	Parallelism int    `json:"parallelism,omitempty"`
}

// Secret is one stored secret.
type Secret struct {
	Name string `json:"name"`
	Kind Kind   `json:"kind"`
	// Value is the hash for KindHash and the secret itself for KindValue.
	Value string `json:"value"`
	// Algorithm and Parameters are set for KindHash only.
	Algorithm  string         `json:"algorithm,omitempty"`
	Parameters HashParameters `json:"parameters,omitempty"`
	// Description is free text to help an operator recognize the entry.
	Description string `json:"description,omitempty"`
}

// ErrInvalidSecret reports a secret that cannot be stored as described.
var ErrInvalidSecret = errors.New("invalid secret")

// Validate checks a secret is self-consistent before it is stored.
func (s *Secret) Validate() error {
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("%w: a name is required", ErrInvalidSecret)
	}
	if s.Value == "" {
		return fmt.Errorf("%w: a value is required", ErrInvalidSecret)
	}

	switch s.Kind {
	case KindValue:
		return nil
	case KindHash:
		// Without the algorithm a hash cannot be verified later, which would leave a credential that
		// silently rejects every attempt.
		if strings.TrimSpace(s.Algorithm) == "" {
			return fmt.Errorf("%w: a hash requires an algorithm", ErrInvalidSecret)
		}
		return nil
	case "":
		return fmt.Errorf("%w: a kind is required, either %q or %q", ErrInvalidSecret, KindHash, KindValue)
	default:
		return fmt.Errorf("%w: unknown kind %q", ErrInvalidSecret, s.Kind)
	}
}

// IsVerifiable reports whether the secret is checked by comparison rather than read back.
func (s *Secret) IsVerifiable() bool {
	return s.Kind == KindHash
}

// Redacted returns a copy safe to log or list: for a hash the parameters are kept, because they are not
// sensitive on their own, while any readable value is removed.
func (s *Secret) Redacted() Secret {
	out := *s
	if s.Kind == KindValue {
		out.Value = ""
	}
	return out
}
