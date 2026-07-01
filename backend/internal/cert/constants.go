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

package cert

import (
	"errors"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// CertificateType represents the type of certificates in the system.
type CertificateType = providers.CertificateType

const (
	// CertificateTypeJWKS represents a JSON Web Key Set (JWKS) certificate.
	CertificateTypeJWKS = providers.CertificateTypeJWKS
	// CertificateTypeJWKSURI represents a JWKS URI certificate.
	CertificateTypeJWKSURI = providers.CertificateTypeJWKSURI
)

// CertificateReferenceType represents the type of certificate reference in the system.
type CertificateReferenceType string

const (
	// CertificateReferenceTypeIDP represents a certificate reference for an identity provider.
	CertificateReferenceTypeIDP CertificateReferenceType = "IDP"
	// CertificateReferenceTypeOAuthApp represents a certificate reference for an OAuth client.
	CertificateReferenceTypeOAuthApp CertificateReferenceType = "OAUTH_APP"
)

// ErrCertificateNotFound is the error message when a certificate is not found.
var ErrCertificateNotFound = errors.New("certificate not found")
