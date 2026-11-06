// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package openid4vp

import (
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/vc/presentation"
)

// trustAnchorStore holds the trusted issuer certificates of a presentation definition,
// used to validate x5c chains.
type trustAnchorStore struct {
	roots *x509.CertPool
	certs []*x509.Certificate
}

// newTrustAnchorStore builds a trust anchor store from trusted issuer certificates.
func newTrustAnchorStore(certs []*x509.Certificate) *trustAnchorStore {
	roots := x509.NewCertPool()
	for _, cert := range certs {
		roots.AddCert(cert)
	}
	return &trustAnchorStore{roots: roots, certs: certs}
}

// verifyChain validates an x5c chain (leaf-first) against the trust anchors.
func (s *trustAnchorStore) verifyChain(chain []*x509.Certificate, now time.Time) (*x509.Certificate, error) {
	if len(chain) == 0 {
		return nil, fmt.Errorf("%w: empty x5c chain", ErrUntrustedIssuer)
	}
	leaf := chain[0]
	inter := x509.NewCertPool()
	for _, c := range chain[1:] {
		inter.AddCert(c)
	}
	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots:         s.roots,
		Intermediates: inter,
		CurrentTime:   now,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUntrustedIssuer, err)
	}
	return leaf, nil
}

// keyIDs returns the deduplicated base64url SubjectKeyId of each trust anchor. It returns nil when
// any anchor has no SubjectKeyId, since a partial list would make wallets drop credentials that verify.
func (s *trustAnchorStore) keyIDs() []string {
	out := make([]string, 0, len(s.certs))
	seen := make(map[string]bool, len(s.certs))
	for _, cert := range s.certs {
		if len(cert.SubjectKeyId) == 0 {
			return nil
		}
		ski := base64.RawURLEncoding.EncodeToString(cert.SubjectKeyId)
		if seen[ski] {
			continue
		}
		seen[ski] = true
		out = append(out, ski)
	}
	return out
}

// buildTrustStore builds the trust anchor store of a presentation definition from its trusted authorities.
// Returns nil without error when the definition has none.
func buildTrustStore(authorities []presentation.TrustedAuthority) (*trustAnchorStore, error) {
	if len(authorities) == 0 {
		return nil, nil
	}
	certs := make([]*x509.Certificate, 0, len(authorities))
	for _, authority := range authorities {
		cert, err := presentation.ParseTrustedAuthorityCertificate(authority.Certificate)
		if err != nil {
			return nil, fmt.Errorf("failed to load trusted authority %q: %w", authority.Name, err)
		}
		certs = append(certs, cert)
	}
	return newTrustAnchorStore(certs), nil
}
