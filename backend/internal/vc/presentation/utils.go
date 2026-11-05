// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package presentation

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

// ParseTrustedAuthorityCertificate parses the PEM-encoded X.509 certificate of a trusted authority.
// The input must hold exactly one CERTIFICATE block.
func ParseTrustedAuthorityCertificate(certificatePEM string) (*x509.Certificate, error) {
	block, rest := pem.Decode([]byte(certificatePEM))
	if block == nil || block.Type != pemBlockTypeCertificate || len(bytes.TrimSpace(rest)) != 0 {
		return nil, errInvalidCertificatePEM
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errInvalidCertificatePEM, err)
	}
	return cert, nil
}
