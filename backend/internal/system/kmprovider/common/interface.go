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

// Package common defines interfaces for key manager providers.
package common

import (
	"context"
	"crypto/tls"
)

// ConfigCryptoProvider provides symmetric encryption and decryption functionality
// using statically configured keys.
type ConfigCryptoProvider interface {
	Encrypt(ctx context.Context, content []byte) ([]byte, error)
	Decrypt(ctx context.Context, content []byte) ([]byte, error)
}

// TLSMaterialProvider provides TLS certificate material for a given key reference.
type TLSMaterialProvider interface {
	GetTLSMaterial(ctx context.Context) (*TLSMaterial, error)
}

// TLSMaterial holds the TLS certificate material for a key reference.
type TLSMaterial struct {
	Certificate tls.Certificate
	MinVersion  uint16
}
