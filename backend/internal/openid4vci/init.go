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

package openid4vci

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/jose/jws"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/user"
	"github.com/thunder-id/thunderid/internal/vc/credential"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize wires the OpenID4VCI issuer engine and the wallet-facing endpoints.
// credSvc is the already-initialized credential-configuration service (owned by the caller).
// When no signing key is configured, issuance is disabled and Initialize returns nil without error.
func Initialize(
	mux *http.ServeMux, cryptoProvider kmprovider.RuntimeCryptoProvider,
	jwtService jwt.JWTServiceInterface, userService user.UserServiceInterface,
	dpopVerifier dpop.VerifierInterface, credSvc credential.CredentialConfigurationServiceInterface,
	store providers.RuntimeStoreProvider,
) (OpenID4VCIServiceInterface, error) {
	runtime := config.GetServerRuntime()
	cfg := runtime.Config.OpenID4VCI

	// Engine URLs default to the server's public URL; explicit config overrides.
	serverCfg := runtime.Config.Server
	serverURL := strings.TrimRight(config.GetServerURL(&serverCfg), "/")
	credentialIssuer := cfg.CredentialIssuer
	if credentialIssuer == "" {
		credentialIssuer = serverURL
	}
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = serverURL
	}
	authServers := cfg.AuthorizationServers
	if len(authServers) == 0 {
		authServers = []string{serverURL}
	}

	if cfg.SigningKeyID == "" {
		return nil, nil
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
	chain := signingKey.CertificateChainDER
	if len(chain) == 0 {
		chain = [][]byte{signingKey.CertificateDER}
	}
	x5c := make([]string, 0, len(chain))
	for _, derBytes := range chain {
		x5c = append(x5c, base64.StdEncoding.EncodeToString(derBytes))
	}

	svc, err := newOpenID4VCIService(serviceConfig{
		CredentialIssuer:     credentialIssuer,
		BaseURL:              baseURL,
		AuthorizationServers: authServers,
		NonceTTL:             time.Duration(cfg.NonceTTLSeconds) * time.Second,
		ProofMaxAge:          time.Duration(cfg.ProofMaxAgeSeconds) * time.Second,
		CredentialValidity:   time.Duration(cfg.CredentialValiditySeconds) * time.Second,
		BatchSize:            cfg.BatchSize,
		EnforceScope:         cfg.EnforceScope,
	}, cryptoProvider, kmprovider.KeyRef{KeyID: cfg.SigningKeyID},
		string(signingKey.Algorithm), signingKey.Thumbprint, x5c,
		newOpenID4VCIStore(store), jwtService, userService, credSvc)
	if err != nil {
		return nil, err
	}

	nonceTTL := time.Duration(cfg.NonceTTLSeconds) * time.Second
	registerRoutes(mux, newOpenID4VCIHandler(svc, dpopVerifier, baseURL+credentialPath, nonceTTL))
	return svc, nil
}

// registerRoutes registers the OpenID4VCI HTTP routes with CORS middleware on the given mux.
func registerRoutes(mux *http.ServeMux, h *openID4VCIHandler) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	mux.HandleFunc(middleware.WithCORS("GET "+metadataPath,
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleMetadata)).ServeHTTP, opts))
	mux.HandleFunc(middleware.WithCORS("GET "+offerPath,
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleOffer)).ServeHTTP, opts))
	mux.HandleFunc(middleware.WithCORS("GET "+credentialOfferPath+"/{id}",
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleCredentialOffer)).ServeHTTP, opts))
	mux.HandleFunc(middleware.WithCORS("POST "+noncePath,
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleNonce)).ServeHTTP, opts))
	mux.HandleFunc(middleware.WithCORS("POST "+credentialPath,
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleCredential)).ServeHTTP, opts))

	for _, path := range []string{metadataPath, offerPath, noncePath, credentialPath} {
		mux.HandleFunc(middleware.WithCORS("OPTIONS "+path,
			func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }, opts))
	}
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+credentialOfferPath+"/{id}",
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }, opts))
}
