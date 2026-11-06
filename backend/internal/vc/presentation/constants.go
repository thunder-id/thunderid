// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package presentation

const (
	// maxTrustedAuthorities caps the trusted issuer certificates one presentation definition may carry.
	maxTrustedAuthorities = 10
	// pemBlockTypeCertificate is the PEM block type of an X.509 certificate.
	pemBlockTypeCertificate = "CERTIFICATE"
)
