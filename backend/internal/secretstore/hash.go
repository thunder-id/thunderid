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
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

// defaultKeySize and defaultIterations are used when a stored hash omits them.
const (
	defaultKeySize    = 32
	defaultIterations = 600000
)

// verifyHash recomputes the hash of a presented value and compares it to the stored one.
//
// Only PBKDF2 with SHA-256 is implemented, which is what ThunderID produces by default. An algorithm
// this service cannot recompute must not be treated as a match, so anything else is rejected rather
// than assumed equal.
func verifyHash(presented string, secret Secret) bool {
	if !strings.EqualFold(secret.Algorithm, "PBKDF2") &&
		!strings.EqualFold(secret.Algorithm, "PBKDF2-SHA256") {
		return false
	}

	salt, err := base64.StdEncoding.DecodeString(secret.Parameters.Salt)
	if err != nil {
		// A salt that is not base64 is taken literally, since ThunderID stores some salts as raw text.
		salt = []byte(secret.Parameters.Salt)
	}

	iterations := secret.Parameters.Iterations
	if iterations <= 0 {
		iterations = defaultIterations
	}
	keySize := secret.Parameters.KeySize
	if keySize <= 0 {
		keySize = defaultKeySize
	}

	computed := pbkdf2.Key([]byte(presented), salt, iterations, keySize, sha256.New)
	encoded := base64.StdEncoding.EncodeToString(computed)

	// Constant-time comparison, so a caller cannot learn the hash from response timing.
	if subtle.ConstantTimeCompare([]byte(encoded), []byte(secret.Value)) == 1 {
		return true
	}
	// Some stores keep the hash hex or raw; compare the raw bytes too rather than reporting a spurious
	// mismatch.
	return subtle.ConstantTimeCompare(computed, []byte(secret.Value)) == 1
}

// tokenMatches compares a presented token in constant time.
func tokenMatches(presented, expected string) bool {
	return subtle.ConstantTimeCompare([]byte(presented), []byte(expected)) == 1
}
