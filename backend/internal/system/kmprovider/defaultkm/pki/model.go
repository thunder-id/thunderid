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

package pki

import (
	"crypto"
	"crypto/tls"
)

// PKIAlgorithm represents the algorithm used in the PKI.
type PKIAlgorithm string

const (
	// RSA represents the RSA algorithm.
	RSA PKIAlgorithm = "RSA"
	// P256 represents the P-256 elliptic curve algorithm.
	P256 PKIAlgorithm = "P-256"
	// P384 represents the P-384 elliptic curve algorithm.
	P384 PKIAlgorithm = "P-384"
	// P521 represents the P-521 elliptic curve algorithm.
	P521 PKIAlgorithm = "P-521"
	// Ed25519 represents the Ed25519 elliptic curve algorithm.
	Ed25519 PKIAlgorithm = "Ed25519"
	// MLDSA44 represents the ML-DSA-44 post-quantum signature algorithm.
	MLDSA44 PKIAlgorithm = "ML-DSA-44"
	// MLDSA65 represents the ML-DSA-65 post-quantum signature algorithm.
	MLDSA65 PKIAlgorithm = "ML-DSA-65"
	// MLDSA87 represents the ML-DSA-87 post-quantum signature algorithm.
	MLDSA87 PKIAlgorithm = "ML-DSA-87"
)

// PKI represents a Public Key Infrastructure entity.
type PKI struct {
	ID          string
	Algorithm   PKIAlgorithm
	PrivateKey  crypto.PrivateKey
	Certificate tls.Certificate
	ThumbPrint  string
}
