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

package par

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// requestURIRandomBytes is the number of random bytes for the request URI (32 bytes = 256 bits).
const requestURIRandomBytes = 32

// GenerateRandomKey generates a cryptographically random key for the request URI.
// It is shared by the SQL and Redis PAR store implementations.
func GenerateRandomKey() (string, error) {
	b := make([]byte, requestURIRandomBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}
