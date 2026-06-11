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
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
)

// Initialize wires the OpenID4VP verifier engine, registers its HTTP endpoints,
// and registers every presentation definition supplied in configuration.
//
// jwtService is used to sign RP-facing result tokens. When nil, the COMPLETED
// status response cannot issue a result token (wallet flow still works).
func Initialize(
	mux *http.ServeMux, cryptoProvider kmprovider.RuntimeCryptoProvider,
	cacheManager cache.CacheManagerInterface, jwtService jwt.JWTServiceInterface,
) (OpenID4VPServiceInterface, error) {
	runtime := config.GetServerRuntime()
	cfg := runtime.Config.OpenID4VP
	serverHome := runtime.ServerHome

	if cfg.ClientID == "" {
		return nil, fmt.Errorf("%w: client_id is required", ErrPolicy)
	}
	signer, err := newRequestSigner(context.Background(), cryptoProvider, cfg.SigningKeyID)
	if err != nil {
		return nil, err
	}

	verifierInfo, err := loadVerifierInfo(cfg.RegistrationCertFile, serverHome)
	if err != nil {
		return nil, err
	}

	base := strings.TrimRight(cfg.BaseURL, "/")
	svc, err := newService(serviceConfig{
		RequestURIBase:        base + requestURIPath,
		ResponseURIBase:       base + responseURIPath,
		ClientIDScheme:        cfg.ClientIDScheme,
		EphemeralKeyID:        cfg.EphemeralKeyID,
		ResponseEncValues:     cfg.ResponseEncValues,
		RequestAudience:       cfg.RequestAudience,
		RequestValidity:       time.Duration(cfg.RequestValiditySeconds) * time.Second,
		TTL:                   time.Duration(cfg.StateTTLSeconds) * time.Second,
		Leeway:                time.Duration(cfg.LeewaySeconds) * time.Second,
		KeyBindingMaxAge:      time.Duration(cfg.KeyBindingMaxAgeSeconds) * time.Second,
		ResultRedirectURIBase: cfg.ResultRedirectURI,
		VerifierInfo:          verifierInfo,
		EnforceTrustedIssuer:  cfg.EnforceTrustedIssuer,
		EnforceKeyBinding:     cfg.EnforceKeyBinding,
	}, newCacheStateStore(cacheManager), cfg.ClientID, signer)
	if err != nil {
		return nil, err
	}
	for _, dc := range cfg.PresentationDefinitions {
		def, err := buildDefinition(dc, svc, serverHome)
		if err != nil {
			return nil, fmt.Errorf("presentation definition %q: %w", dc.ID, err)
		}
		if err := svc.registry.register(def); err != nil {
			return nil, err
		}
	}

	var issuer resultTokenIssuer
	if jwtService != nil {
		issuer = newJWTresultTokenIssuer(jwtService, base, cfg.ClientID)
	}
	resultTokenValidity := time.Duration(cfg.ResultTokenValiditySeconds) * time.Second
	registerRoutes(mux, newOpenID4VPHandler(svc, issuer, base, resultTokenValidity, svc.cfg.TTL))

	return svc, nil
}

// buildDefinition constructs a presentationDefinition from its config.
// Trusted-issuer cert paths are resolved against serverHome.
func buildDefinition(
	dc config.DefinitionConfig, svc *service, serverHome string,
) (*presentationDefinition, error) {
	if dc.ID == "" {
		return nil, fmt.Errorf("%w: presentation definition requires an id", ErrPolicy)
	}

	issuers := make([]trustedIssuer, 0, len(dc.TrustedIssuers))
	for _, ti := range dc.TrustedIssuers {
		issuers = append(issuers, trustedIssuer{
			Issuer:   ti.Issuer,
			CertFile: resolvePath(serverHome, ti.CertFile),
		})
	}

	var trust *staticTrustStore
	if len(issuers) > 0 {
		var err error
		trust, err = buildTrustStore(issuers)
		if err != nil {
			return nil, err
		}
	} else if svc.cfg.EnforceTrustedIssuer {
		return nil, fmt.Errorf("%w: enforce_trusted_issuer requires at least one trusted issuer", ErrPolicy)
	}

	allClaims := dc.RequestedClaims
	if len(allClaims) == 0 {
		merged := make([]string, 0, len(dc.MandatoryClaims)+len(dc.OptionalClaims))
		merged = append(merged, dc.MandatoryClaims...)
		merged = append(merged, dc.OptionalClaims...)
		allClaims = merged
	}

	return &presentationDefinition{
		ID:          dc.ID,
		DisplayName: dc.DisplayName,
		DCQL: dcqlConfig{
			CredentialID: dc.CredentialID,
			VCT:          dc.VCT,
			Claims:       allClaims,
		},
		policy: policy{
			ExpectedVCT:          dc.VCT,
			Audience:             svc.clientID,
			RequestedClaims:      allClaims,
			MandatoryClaims:      dc.MandatoryClaims,
			EnforceTrustedIssuer: svc.cfg.EnforceTrustedIssuer,
			EnforceKeyBinding:    svc.cfg.EnforceKeyBinding,
		},
		Trust:         trust,
		SubjectClaims: dc.SubjectClaims,
		DeriveSubject: defaultSubjectDeriver(dc.SubjectClaims),
	}, nil
}

// resolvePath joins a relative path with the server home directory.
func resolvePath(serverHome, path string) string {
	if path == "" || filepath.IsAbs(path) || serverHome == "" {
		return path
	}
	return filepath.Join(serverHome, path)
}

// loadVerifierInfo reads the Registration Certificate JWT from path and wraps it
// as a verifier_attestations entry. An empty path returns (nil, nil).
func loadVerifierInfo(path, serverHome string) ([]interface{}, error) {
	if path == "" {
		return nil, nil
	}
	resolved := filepath.Clean(resolvePath(serverHome, path))
	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("failed to read registration certificate %q: %w", resolved, err)
	}
	jwt := strings.TrimSpace(string(data))
	if jwt == "" {
		return nil, fmt.Errorf("%w: registration certificate %q is empty", ErrPolicy, resolved)
	}
	return []interface{}{
		map[string]interface{}{
			"format": "registration_cert",
			"data":   jwt,
		},
	}, nil
}

// buildTrustStore loads the configured trusted issuer certificates.
func buildTrustStore(issuers []trustedIssuer) (*staticTrustStore, error) {
	if len(issuers) == 0 {
		return nil, fmt.Errorf("%w: at least one trusted issuer is required", ErrPolicy)
	}
	keys := make(map[string]crypto.PublicKey, len(issuers))
	for _, issuer := range issuers {
		if issuer.Issuer == "" || issuer.CertFile == "" {
			return nil, fmt.Errorf("%w: trusted issuer requires issuer and cert_file", ErrPolicy)
		}
		key, err := loadIssuerKey(issuer.CertFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load trusted issuer %q: %w", issuer.Issuer, err)
		}
		keys[issuer.Issuer] = key
	}
	return newStaticTrustStore(keys), nil
}

// loadIssuerKey reads an issuer signing key from a PEM file containing either an
// X.509 certificate or a PKIX public key.
func loadIssuerKey(path string) (crypto.PublicKey, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in %s", path)
	}
	switch block.Type {
	case "CERTIFICATE":
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		return cert.PublicKey, nil
	case "PUBLIC KEY":
		return x509.ParsePKIXPublicKey(block.Bytes)
	default:
		return nil, fmt.Errorf("unsupported PEM block type %q in %s", block.Type, path)
	}
}
