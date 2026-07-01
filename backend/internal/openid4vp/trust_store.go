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

package openid4vp

import (
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"time"
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
