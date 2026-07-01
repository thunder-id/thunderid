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
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/openid4vp/definition"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/config"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize wires the OpenID4VP verifier engine and the presentation-definition management API.
func Initialize(
	mux *http.ServeMux, cryptoProvider kmprovider.RuntimeCryptoProvider,
	configCrypto kmprovider.ConfigCryptoProvider, jwtService jwt.JWTServiceInterface,
	ouService ou.OrganizationUnitServiceInterface,
) (
	OpenID4VPServiceInterface, definition.PresentationDefinitionServiceInterface,
	declarativeresource.ResourceExporter, error,
) {
	runtime := config.GetServerRuntime()
	cfg := runtime.Config.OpenID4VP
	serverHome := runtime.ServerHome

	// The presentation-definition management API is always available; only the
	// wallet/RP-facing verifier engine is gated on the signing key.
	defSvc, defExporter, err := definition.Initialize(mux, ouService)
	if err != nil {
		return nil, nil, nil, err
	}

	if cfg.SigningKeyID == "" {
		log.GetLogger().Debug(context.Background(),
			"OpenID4VP verifier not configured; presentation verification disabled")
		return nil, defSvc, defExporter, nil
	}
	if cfg.ClientID == "" {
		return nil, nil, nil, fmt.Errorf("%w: client_id is required", ErrPolicy)
	}
	signer, err := newRequestSigner(context.Background(), cryptoProvider, cfg.SigningKeyID)
	if err != nil {
		return nil, nil, nil, err
	}

	verifierInfo, err := loadVerifierInfo(cfg.RegistrationCertFile, serverHome)
	if err != nil {
		return nil, nil, nil, err
	}

	trust, err := buildSharedTrustStore(cfg.TrustedAnchors, serverHome)
	if err != nil {
		return nil, nil, nil, err
	}

	base := strings.TrimRight(config.GetServerURL(&runtime.Config.Server), "/")
	stateTTL := time.Duration(cfg.StateTTLSeconds) * time.Second
	if stateTTL == 0 {
		stateTTL = defaultStateTTL
	}
	svc, err := newOpenID4VPService(serviceConfig{
		RequestURIBase:        base + requestURIPath,
		ResponseURIBase:       base + responseURIPath,
		EphemeralKeyID:        cfg.EphemeralKeyID,
		ResponseEncValues:     cfg.ResponseEncValues,
		RequestAudience:       cfg.RequestAudience,
		RequestValidity:       time.Duration(cfg.RequestValiditySeconds) * time.Second,
		TTL:                   stateTTL,
		Leeway:                time.Duration(cfg.LeewaySeconds) * time.Second,
		KeyBindingMaxAge:      time.Duration(cfg.KeyBindingMaxAgeSeconds) * time.Second,
		ResultRedirectURIBase: cfg.ResultRedirectURI,
		VerifierInfo:          verifierInfo,
		EnforceKeyBinding:     cfg.EnforceKeyBinding,
	}, newOpenID4VPStore(configCrypto), cfg.ClientID, signer, trust, defSvc)
	if err != nil {
		return nil, nil, nil, err
	}

	var issuer resultTokenIssuer
	if jwtService != nil {
		issuer = newJWTresultTokenIssuer(jwtService, base, cfg.ClientID)
	}
	resultTokenValidity := time.Duration(cfg.ResultTokenValiditySeconds) * time.Second
	registerRoutes(mux, newOpenID4VPHandler(svc, issuer, base, resultTokenValidity, stateTTL))

	return svc, defSvc, defExporter, nil
}

// registerRoutes registers the OpenID4VP HTTP routes on mux with CORS middleware.
func registerRoutes(mux *http.ServeMux, h *openID4VPHandler) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	mux.HandleFunc(middleware.WithCORS("GET "+requestURIPath,
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleRequestObject)).ServeHTTP, opts))
	mux.HandleFunc(middleware.WithCORS("POST "+responseURIPath,
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleResponse)).ServeHTTP, opts))
	mux.HandleFunc(middleware.WithCORS("POST "+apiInitiatePath,
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleInitiate)).ServeHTTP, opts))
	mux.HandleFunc(middleware.WithCORS("GET "+apiStatusPath,
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleStatus)).ServeHTTP, opts))
	mux.HandleFunc(middleware.WithCORS("GET "+apiTrustAnchorsPath,
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleTrustAnchors)).ServeHTTP, opts))

	walletAndRPPaths := []string{requestURIPath, responseURIPath, apiInitiatePath, apiStatusPath, apiTrustAnchorsPath}
	for _, path := range walletAndRPPaths {
		mux.HandleFunc(middleware.WithCORS("OPTIONS "+path,
			func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }, opts))
	}
}

// buildSharedTrustStore builds the engine-wide trust anchor store; returns nil when no anchors are configured.
func buildSharedTrustStore(entries []config.TrustedAnchorEntry, serverHome string) (*trustAnchorStore, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	anchors := make([]trustAnchor, 0, len(entries))
	for _, ta := range entries {
		anchors = append(anchors, trustAnchor{
			Name:     ta.Name,
			CertFile: resolvePath(serverHome, ta.CertFile),
		})
	}
	return buildTrustStore(anchors)
}

// resolvePath joins a relative path with the server home directory.
func resolvePath(serverHome, path string) string {
	if path == "" || filepath.IsAbs(path) || serverHome == "" {
		return path
	}
	return filepath.Join(serverHome, path)
}

// loadVerifierInfo reads the Registration Certificate JWT from path and wraps it
// as a verifier_info entry. An empty path returns (nil, nil).
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

// buildTrustStore loads the configured trust anchor (root CA) certificates.
func buildTrustStore(anchors []trustAnchor) (*trustAnchorStore, error) {
	if len(anchors) == 0 {
		return nil, fmt.Errorf("%w: at least one trust anchor is required", ErrPolicy)
	}
	certs := make([]*x509.Certificate, 0, len(anchors))
	names := make([]string, 0, len(anchors))
	for _, anchor := range anchors {
		if anchor.Name == "" || anchor.CertFile == "" {
			return nil, fmt.Errorf("%w: trust anchor requires name and cert_file", ErrPolicy)
		}
		cert, err := loadCertificate(anchor.CertFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load trust anchor %q: %w", anchor.Name, err)
		}
		certs = append(certs, cert)
		names = append(names, anchor.Name)
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
