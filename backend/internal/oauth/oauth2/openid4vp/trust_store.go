// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package openid4vp

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
)

// anchorInfo describes a single configured trust anchor (root CA).
type anchorInfo struct {
	Name     string
	Subject  string
	SKI      string
	NotAfter time.Time
	cert     *x509.Certificate
}

// trustAnchorStore holds the configured root CAs used to validate x5c chains.
type trustAnchorStore struct {
	roots   *x509.CertPool
	anchors []anchorInfo
}

// newTrustAnchorStore builds a trust anchor store from root CA certificates; names is parallel to certs.
func newTrustAnchorStore(certs []*x509.Certificate, names []string) *trustAnchorStore {
	roots := x509.NewCertPool()
	anchors := make([]anchorInfo, 0, len(certs))
	for i, cert := range certs {
		roots.AddCert(cert)
		anchors = append(anchors, anchorInfo{
			Name:     names[i],
			Subject:  cert.Subject.String(),
			SKI:      base64.RawURLEncoding.EncodeToString(cert.SubjectKeyId),
			NotAfter: cert.NotAfter,
			cert:     cert,
		})
	}
	return &trustAnchorStore{roots: roots, anchors: anchors}
}

// verifyChain validates an x5c chain (leaf-first) against the configured trust anchors.
// When allowed is non-empty the chain must terminate at a named anchor; unknown names fail closed.
func (s *trustAnchorStore) verifyChain(
	chain []*x509.Certificate, now time.Time, allowed []string,
) (*x509.Certificate, error) {
	if len(chain) == 0 {
		return nil, fmt.Errorf("%w: empty x5c chain", ErrUntrustedIssuer)
	}
	roots := s.roots
	if len(allowed) > 0 {
		roots = s.rootsFor(allowed)
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

// rootsFor builds a CertPool containing only the anchors whose Name is in names.
func (s *trustAnchorStore) rootsFor(names []string) *x509.CertPool {
	allow := make(map[string]bool, len(names))
	for _, n := range names {
		allow[n] = true
	}
	pool := x509.NewCertPool()
	for _, a := range s.anchors {
		if allow[a.Name] {
			pool.AddCert(a.cert)
		}
	}
	return pool
}

// skisFor returns the base64url SubjectKeyId of each named anchor, skipping unknown names and deduping.
func (s *trustAnchorStore) skisFor(names []string) []string {
	byName := make(map[string]string, len(s.anchors))
	for _, a := range s.anchors {
		byName[a.Name] = a.SKI
	}
	out := make([]string, 0, len(names))
	seen := make(map[string]bool, len(names))
	for _, n := range names {
		ski, ok := byName[n]
		if !ok || seen[ski] {
			continue
		}
		seen[ski] = true
		out = append(out, ski)
	}
	return out
}

// list returns a copy of the configured trust anchors.
func (s *trustAnchorStore) list() []anchorInfo {
	out := make([]anchorInfo, len(s.anchors))
	copy(out, s.anchors)
	return out
}

// buildTrustStore builds the engine-wide trust anchor store from configured entries.
// Returns nil without error when no anchors are configured (trust verification disabled).
func buildTrustStore(entries []config.TrustedAnchorEntry, serverHome string) (*trustAnchorStore, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	certs := make([]*x509.Certificate, 0, len(entries))
	names := make([]string, 0, len(entries))
	for _, ta := range entries {
		if ta.Name == "" || ta.CertFile == "" {
			return nil, fmt.Errorf("%w: trust anchor requires name and cert_file", ErrPolicy)
		}
		cert, err := loadCertificate(resolvePath(serverHome, ta.CertFile))
		if err != nil {
			return nil, fmt.Errorf("failed to load trust anchor %q: %w", ta.Name, err)
		}
		certs = append(certs, cert)
		names = append(names, ta.Name)
	}
	return newTrustAnchorStore(certs, names), nil
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
