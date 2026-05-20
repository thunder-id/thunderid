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

package common

import (
	"errors"
	"sync"

	"github.com/thunder-id/thunderid/internal/idp"
)

var (
	authenticatorRegistry map[string]AuthenticatorMeta
	registryMu            sync.RWMutex
)

func init() {
	authenticatorRegistry = make(map[string]AuthenticatorMeta)
}

// RegisterAuthenticator registers an authenticator's metadata in the registry.
// Each authenticator service should call this during its initialization.
func RegisterAuthenticator(meta AuthenticatorMeta) {
	registryMu.Lock()
	defer registryMu.Unlock()
	authenticatorRegistry[meta.Name] = meta
}

// getAuthenticatorMetaData returns the authenticator metadata for the given authenticator.
func getAuthenticatorMetaData(name string) *AuthenticatorMeta {
	registryMu.RLock()
	defer registryMu.RUnlock()

	if auth, ok := authenticatorRegistry[name]; ok {
		return &auth
	}
	return nil
}

// GetAuthenticatorFactors returns the authentication factors for the given authenticator.
func GetAuthenticatorFactors(name string) []AuthenticationFactor {
	if auth := getAuthenticatorMetaData(name); auth != nil {
		return auth.Factors
	}
	return []AuthenticationFactor{}
}

// GetAuthenticatorNameForIDPType returns the authenticator name for a given IDP type.
func GetAuthenticatorNameForIDPType(idpType idp.IDPType) (string, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	if idpType != "" {
		for _, meta := range authenticatorRegistry {
			if meta.AssociatedIDP == idpType {
				return meta.Name, nil
			}
		}
	}

	return "", errors.New("no authenticator found for the given IDP type")
}
