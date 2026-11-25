// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package openid4vp

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/vc/verifier"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// Initialize wires the OpenID4VP verifier engine.
func Initialize(
	mux *http.ServeMux,
	issuerURL string,
	serverHome string,
	cryptoProvider providers.RuntimeCryptoProvider,
	jwtService jwt.JWTServiceInterface,
	config engineconfig.OpenID4VPConfig,
	verifierClientConfig providers.VCVerifierClientConfigProvider,
	store providers.RuntimeStoreProvider,
	verifierRegistry verifier.VCVerifierRegistryInterface,
) (OpenID4VPServiceInterface, error) {
	if config.SigningKeyID == "" {
		return nil, fmt.Errorf("%w: signing_key_id is required", ErrPolicy)
	}

	keys, err := cryptoProvider.GetPublicKeys(context.Background(),
		providers.PublicKeyFilter{KeyID: config.SigningKeyID})
	if err != nil {
		return nil, fmt.Errorf("failed to load signing key %q: %w", config.SigningKeyID, err)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("%w: no signing key found for key id %q", ErrPolicy, config.SigningKeyID)
	}
	signingKey := keys[0]
	if !slices.Contains(cryptoProvider.GetSupportedSigningAlgorithms(), signingKey.Algorithm) {
		return nil, fmt.Errorf("%w: unsupported signing algorithm for key %q", ErrPolicy, config.SigningKeyID)
	}
	if len(signingKey.CertificateDER) == 0 {
		return nil, fmt.Errorf("%w: signing key %q is not certificate-backed (x5c required)",
			ErrPolicy, config.SigningKeyID)
	}

	chain := signingKey.CertificateChainDER
	if len(chain) == 0 {
		chain = [][]byte{signingKey.CertificateDER}
	}
	x5c := make([]string, 0, len(chain))
	for _, derBytes := range chain {
		x5c = append(x5c, base64.StdEncoding.EncodeToString(derBytes))
	}

	issuerURL = strings.TrimRight(issuerURL, "/")
	requestURL := issuerURL + requestURIPath
	responseURL := issuerURL + responseURIPath

	svc, err := newOpenID4VPService(config,
		newOpenID4VPStore(cryptoProvider, store),
		cryptoProvider,
		providers.KeyRef{KeyID: config.SigningKeyID},
		signingKey.Algorithm,
		x5c,
		verifierRegistry,
		verifierClientConfig,
		jwtService,
		requestURL,
		responseURL,
		issuerURL)
	if err != nil {
		return nil, err
	}

	registerRoutes(mux, newOpenID4VPHandler(svc))

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

	allPaths := []string{requestURIPath, responseURIPath}
	for _, path := range allPaths {
		mux.HandleFunc(middleware.WithCORS("OPTIONS "+path,
			func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }, opts))
	}
}
