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
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/jose/jws"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/vc/presentation"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize wires the OpenID4VP verifier engine.
func Initialize(
	mux *http.ServeMux, cryptoProvider kmprovider.RuntimeCryptoProvider,
	configCrypto kmprovider.ConfigCryptoProvider, jwtService jwt.JWTServiceInterface,
	defSvc presentation.PresentationDefinitionServiceInterface,
	store providers.RuntimeStoreProvider,
) (OpenID4VPServiceInterface, error) {
	runtime := config.GetServerRuntime()
	cfg := runtime.Config.OpenID4VP
	serverHome := runtime.ServerHome

	if cfg.ClientIDScheme == "" {
		return nil, fmt.Errorf("%w: client_id_scheme is required", ErrPolicy)
	}
	if cfg.SigningKeyID == "" {
		return nil, fmt.Errorf("%w: signing_key_id is required", ErrPolicy)
	}

	keys, err := cryptoProvider.GetPublicKeys(context.Background(), kmprovider.PublicKeyFilter{KeyID: cfg.SigningKeyID})
	if err != nil {
		return nil, fmt.Errorf("failed to load signing key %q: %w", cfg.SigningKeyID, err)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("%w: no signing key found for key id %q", ErrPolicy, cfg.SigningKeyID)
	}
	signingKey := keys[0]
	if _, err := jws.MapAlgorithmToSignAlg(jws.Algorithm(signingKey.Algorithm)); err != nil {
		return nil, fmt.Errorf("%w: unsupported signing algorithm for key %q", ErrPolicy, cfg.SigningKeyID)
	}
	if len(signingKey.CertificateDER) == 0 {
		return nil, fmt.Errorf("%w: signing key %q is not certificate-backed (x5c required)",
			ErrPolicy, cfg.SigningKeyID)
	}
	base := strings.TrimRight(config.GetServerURL(&runtime.Config.Server), "/")
	clientID, err := deriveClientID(cfg.ClientIDScheme, signingKey.CertificateDER, base+responseURIPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrPolicy, err)
	}
	chain := signingKey.CertificateChainDER
	if len(chain) == 0 {
		chain = [][]byte{signingKey.CertificateDER}
	}
	x5c := make([]string, 0, len(chain))
	for _, derBytes := range chain {
		x5c = append(x5c, base64.StdEncoding.EncodeToString(derBytes))
	}

	verifierInfo, err := loadVerifierInfo(cfg.RegistrationCertFile, serverHome)
	if err != nil {
		return nil, err
	}

	trust, err := buildTrustStore(cfg.TrustedAnchors, serverHome)
	if err != nil {
		return nil, err
	}

	stateTTL := time.Duration(cfg.StateTTLSeconds) * time.Second
	resultTokenValidity := time.Duration(cfg.ResultTokenValiditySeconds) * time.Second

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
		ResultTokenValidity:   resultTokenValidity,
		VerifierInfo:          verifierInfo,
		EnforceKeyBinding:     cfg.EnforceKeyBinding,
	}, newOpenID4VPStore(configCrypto, store), clientID,
		cryptoProvider, kmprovider.KeyRef{KeyID: cfg.SigningKeyID}, string(signingKey.Algorithm), x5c,
		trust, defSvc, jwtService, base)
	if err != nil {
		return nil, err
	}

	registerRoutes(mux, newOpenID4VPHandler(svc, svc))

	return svc, nil
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
	mux.HandleFunc(middleware.WithCORS("GET "+apiTrustAnchorsPath,
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleTrustAnchors)).ServeHTTP, opts))
	mux.HandleFunc(middleware.WithCORS("POST "+initiateURIPath,
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleInitiate)).ServeHTTP, opts))
	mux.HandleFunc(middleware.WithCORS("GET "+statusURIPath,
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleStatus)).ServeHTTP, opts))

	allPaths := []string{requestURIPath, responseURIPath, apiTrustAnchorsPath, initiateURIPath, statusURIPath}
	for _, path := range allPaths {
		mux.HandleFunc(middleware.WithCORS("OPTIONS "+path,
			func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }, opts))
	}
}

// deriveClientID builds the full client_id value from the configured scheme and
// the signing certificate. responseURI is used only for the redirect_uri scheme.
func deriveClientID(scheme string, certDER []byte, responseURI string) (string, error) {
	switch scheme {
	case "x509_hash":
		hash := sha256.Sum256(certDER)
		return "x509_hash:" + base64.RawURLEncoding.EncodeToString(hash[:]), nil
	case "x509_san_dns":
		cert, err := x509.ParseCertificate(certDER)
		if err != nil {
			return "", fmt.Errorf("failed to parse signing certificate: %w", err)
		}
		if len(cert.DNSNames) == 0 {
			return "", fmt.Errorf("signing certificate has no DNS SAN entries")
		}
		return "x509_san_dns:" + cert.DNSNames[0], nil
	case "redirect_uri":
		return "redirect_uri:" + responseURI, nil
	default:
		return "", fmt.Errorf("unsupported client_id_scheme %q", scheme)
	}
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
