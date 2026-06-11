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

package openid4vp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// pidClaims is a sample identity-claim set for subject-derivation tests.
var pidClaims = []string{"family_name", "given_name", "birthdate"}

func TestDefaultSubjectDeriverPrefersSubClaim(t *testing.T) {
	derive := defaultSubjectDeriver(pidClaims)
	got := derive(&VerifiedPresentation{Subject: "stable-sub", Issuer: "iss"})
	assert.Equal(t, "stable-sub", got)
}

func TestDefaultSubjectDeriverIsStableAndPerPerson(t *testing.T) {
	derive := defaultSubjectDeriver(pidClaims)
	erika := func() *VerifiedPresentation {
		return &VerifiedPresentation{
			Issuer: "https://issuer.example",
			Claims: map[string]interface{}{
				"given_name":  "Erika",
				"family_name": "Mustermann",
				"birthdate":   "1984-01-26",
			},
		}
	}
	a := derive(erika())
	b := derive(erika())
	assert.NotEmpty(t, a)
	assert.Equal(t, a, b, "same person + issuer -> stable subject")

	max := derive(&VerifiedPresentation{
		Issuer: "https://issuer.example",
		Claims: map[string]interface{}{
			"given_name":  "Max",
			"family_name": "Mustermann",
			"birthdate":   "1990-05-05",
		},
	})
	assert.NotEqual(t, a, max, "different person -> different subject")
}

// Claim list order in the configuration must not affect derivation: the
// deriver sorts its claim set before hashing.
func TestDefaultSubjectDeriverIsIndependentOfClaimOrder(t *testing.T) {
	person := &VerifiedPresentation{
		Issuer: "https://issuer.example",
		Claims: map[string]interface{}{
			"given_name":  "Erika",
			"family_name": "Mustermann",
			"birthdate":   "1984-01-26",
		},
	}
	a := defaultSubjectDeriver([]string{"family_name", "given_name", "birthdate"})(person)
	b := defaultSubjectDeriver([]string{"birthdate", "given_name", "family_name"})(person)
	assert.Equal(t, a, b)
}

// Nil input must not panic.
func TestDefaultSubjectDeriverNilPresentation(t *testing.T) {
	got := defaultSubjectDeriver(pidClaims)(nil)
	assert.Empty(t, got)
}
