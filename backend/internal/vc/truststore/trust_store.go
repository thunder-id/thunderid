// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package truststore holds the configured trust anchors (root CAs) that root the
// X.509 chains presented by verifiable credential issuers.
package truststore

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

var (
	// ErrUntrustedIssuer is returned when an issuer certificate chain does not
	// terminate at an allowed trust anchor.
	ErrUntrustedIssuer = errors.New("truststore: untrusted credential issuer")
	// ErrUnknownAnchor is returned when a trust anchor name is not configured.
	ErrUnknownAnchor = errors.New("truststore: unknown trust anchor")
	// ErrInvalidConfig is returned when a configured trust anchor entry is incomplete.
	ErrInvalidConfig = errors.New("truststore: invalid trust anchor configuration")
)

// TrustAnchorInfo is the public view of a configured trust anchor.
type TrustAnchorInfo struct {
	Name     string    `json:"name"`
	Subject  string    `json:"subject"`
	SKI      string    `json:"ski"`
	NotAfter time.Time `json:"not_after"`
}

// TrustStoreInterface resolves and verifies against the configured trust anchors.
type TrustStoreInterface interface {
	// ListAnchors returns the configured trust anchors.
	ListAnchors() []TrustAnchorInfo
	// ValidateNames returns ErrUnknownAnchor if any name is not a configured trust anchor.
	ValidateNames(names []string) error
	// AnchorsFor returns the certificates of the named anchors keyed by their base64url
	// SubjectKeyId. It returns ErrUnknownAnchor if any name is not a configured trust anchor.
	AnchorsFor(names []string) (map[string]*x509.Certificate, error)
	// VerifyChain validates a leaf-first certificate chain against the trust anchors and
	// returns the verified leaf. When allowed is non-empty the chain must terminate at one
	// of the allowed certificates instead.
	VerifyChain(
		chain []*x509.Certificate, now time.Time, allowed map[string]*x509.Certificate,
	) (*x509.Certificate, error)
}

type anchor struct {
	info TrustAnchorInfo
	cert *x509.Certificate
}

type trustStore struct {
	roots   *x509.CertPool
	anchors []anchor
}

var _ TrustStoreInterface = (*trustStore)(nil)

// Initialize builds the trust store from the configured trust anchor entries,
// resolving relative certificate paths against serverHome.
func Initialize(entries []config.TrustedAnchorEntry, serverHome string) (TrustStoreInterface, error) {
	roots := x509.NewCertPool()
	anchors := make([]anchor, 0, len(entries))
	for _, ta := range entries {
		if ta.Name == "" || ta.CertFile == "" {
			return nil, fmt.Errorf("%w: trust anchor requires name and cert_file", ErrInvalidConfig)
		}
		cert, err := loadCertificate(resolvePath(serverHome, ta.CertFile))
		if err != nil {
			return nil, fmt.Errorf("failed to load trust anchor %q: %w", ta.Name, err)
		}
		roots.AddCert(cert)
		anchors = append(anchors, anchor{
			info: TrustAnchorInfo{
				Name:     ta.Name,
				Subject:  cert.Subject.String(),
				SKI:      base64.RawURLEncoding.EncodeToString(cert.SubjectKeyId),
				NotAfter: cert.NotAfter,
			},
			cert: cert,
		})
	}
	return &trustStore{roots: roots, anchors: anchors}, nil
}

// ListAnchors returns the configured trust anchors.
func (s *trustStore) ListAnchors() []TrustAnchorInfo {
	out := make([]TrustAnchorInfo, 0, len(s.anchors))
	for _, a := range s.anchors {
		out = append(out, a.info)
	}
	return out
}

// ValidateNames returns ErrUnknownAnchor if any name is not a configured trust anchor.
func (s *trustStore) ValidateNames(names []string) error {
	known := make(map[string]bool, len(s.anchors))
	for _, a := range s.anchors {
		known[a.info.Name] = true
	}
	var unknown []string
	for _, n := range names {
		if !known[n] {
			unknown = append(unknown, n)
		}
	}
	if len(unknown) > 0 {
		return fmt.Errorf("%w: %s", ErrUnknownAnchor, strings.Join(unknown, ", "))
	}
	return nil
}

// AnchorsFor returns the certificates of the named anchors keyed by their base64url SubjectKeyId.
func (s *trustStore) AnchorsFor(names []string) (map[string]*x509.Certificate, error) {
	if err := s.ValidateNames(names); err != nil {
		return nil, err
	}
	allow := make(map[string]bool, len(names))
	for _, n := range names {
		allow[n] = true
	}
	out := make(map[string]*x509.Certificate, len(names))
	for _, a := range s.anchors {
		if allow[a.info.Name] {
			out[a.info.SKI] = a.cert
		}
	}
	return out, nil
}

// VerifyChain validates a leaf-first certificate chain against the trust anchors.
func (s *trustStore) VerifyChain(
	chain []*x509.Certificate, now time.Time, allowed map[string]*x509.Certificate,
) (*x509.Certificate, error) {
	if len(s.anchors) == 0 {
		return nil, fmt.Errorf("%w: no trust anchors configured", ErrUntrustedIssuer)
	}
	if len(chain) == 0 {
		return nil, fmt.Errorf("%w: empty x5c chain", ErrUntrustedIssuer)
	}
	roots := s.roots
	if len(allowed) > 0 {
		roots = x509.NewCertPool()
		for _, cert := range allowed {
			roots.AddCert(cert)
		}
	}
	leaf := chain[0]
	inter := x509.NewCertPool()
	for _, c := range chain[1:] {
		inter.AddCert(c)
	}
	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots:         roots,
		Intermediates: inter,
		CurrentTime:   now,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUntrustedIssuer, err)
	}
	return leaf, nil
}

// loadCertificate reads an X.509 certificate from a PEM CERTIFICATE file.
func loadCertificate(path string) (*x509.Certificate, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in %s", path)
	}
	if block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("unsupported PEM block type %q in %s", block.Type, path)
	}
	return x509.ParseCertificate(block.Bytes)
}

// resolvePath joins a relative path with the server home directory.
func resolvePath(serverHome, path string) string {
	if path == "" || filepath.IsAbs(path) || serverHome == "" {
		return path
	}
	return filepath.Join(serverHome, path)
}
