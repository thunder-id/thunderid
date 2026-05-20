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

package kmprovider

import (
	gocrypto "crypto"
	"crypto/tls"

	"github.com/thunder-id/thunderid/internal/system/cryptolab"
)

// KeyRef identifies a cryptographic key by its ID.
type KeyRef struct {
	KeyID string
}

// PublicKeyFilter specifies criteria for filtering public keys in GetPublicKeys.
type PublicKeyFilter struct {
	KeyID     string
	Algorithm cryptolab.Algorithm
}

// PublicKeyInfo describes a public key returned by GetPublicKeys.
type PublicKeyInfo struct {
	KeyID          string
	Algorithm      cryptolab.Algorithm
	PublicKey      gocrypto.PublicKey
	Thumbprint     string
	CertificateDER []byte // raw DER-encoded X.509 certificate; nil if not certificate-backed
}

// TLSMaterial holds the TLS certificate material for a key reference.
type TLSMaterial struct {
	Certificate tls.Certificate
}
