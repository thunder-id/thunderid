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

// Package kmprovider defines interfaces for key manager providers.
package kmprovider

import (
	"context"

	"github.com/thunder-id/thunderid/internal/system/cryptolab"
)

// ConfigCryptoProvider provides symmetric encryption and decryption functionality
// using statically configured keys.
type ConfigCryptoProvider interface {
	Encrypt(ctx context.Context, content []byte) ([]byte, error)
	Decrypt(ctx context.Context, content []byte) ([]byte, error)
}

// RuntimeCryptoProvider provides asymmetric cryptographic operations including
// encryption, decryption, signing, and key discovery.
type RuntimeCryptoProvider interface {
	Encrypt(
		ctx context.Context, keyRef *KeyRef, params cryptolab.AlgorithmParams, content []byte,
	) ([]byte, *cryptolab.CryptoDetails, error)
	Decrypt(ctx context.Context, keyRef *KeyRef, params cryptolab.AlgorithmParams, content []byte) ([]byte, error)
	Sign(ctx context.Context, keyRef KeyRef, algorithm cryptolab.SignAlgorithm, content []byte) ([]byte, error)
	GetPublicKeys(ctx context.Context, filter PublicKeyFilter) ([]PublicKeyInfo, error)
	GetTLSMaterial(ctx context.Context, keyRef *KeyRef) (*TLSMaterial, error)
}
