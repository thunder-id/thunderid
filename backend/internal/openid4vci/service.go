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
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/openid4vci/credential"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/user"
)

const (
	defaultNonceTTL     = 5 * time.Minute
	defaultProofMaxAge  = 5 * time.Minute
	defaultCredValidity = 30 * 24 * time.Hour
	defaultBatchSize    = 5
)

// OpenID4VCIServiceInterface is the contract for the OpenID4VCI issuer service,
// consumed by the HTTP handler.
type OpenID4VCIServiceInterface interface {
	GetMetadata(ctx context.Context) map[string]interface{}
	GenerateCredentialOffer(ctx context.Context, configID string) (map[string]interface{}, string, error)
	GetCredentialOffer(ctx context.Context, id string) (map[string]interface{}, error)
	GenerateNonce(ctx context.Context) (string, error)
	IssueCredential(ctx context.Context, accessToken string, body []byte) (*CredentialResponse, error)
}

var _ OpenID4VCIServiceInterface = (*service)(nil)

// credentialReader resolves managed credential configurations. The issuer engine
// only reads configurations; the credential package owns writes.
type credentialReader interface {
	GetCredentialConfigurationByHandle(
		ctx context.Context, handle string,
	) (*credential.CredentialConfigurationDTO, *tidcommon.ServiceError)
	ListCredentialConfigurations(
		ctx context.Context,
	) ([]credential.CredentialConfigurationDTO, *tidcommon.ServiceError)
}

// serviceConfig is the engine-level configuration of the OpenID4VCI issuer.
type serviceConfig struct {
	CredentialIssuer     string
	BaseURL              string
	AuthorizationServers []string
	NonceTTL             time.Duration
	ProofMaxAge          time.Duration
	CredentialValidity   time.Duration
	BatchSize            int
	EnforceScope         bool
}

// service drives the OpenID4VCI issuer: it advertises issuer metadata, issues
// c_nonces, and issues SD-JWT VCs bound to the holder key after validating the
// access token and holder proof.
type service struct {
	cfg         serviceConfig
	signer      *issuerSigner
	store       openID4VCIStoreInterface
	jwtService  jwt.JWTServiceInterface
	userService user.UserServiceInterface
	creds       credentialReader
}

// newOpenID4VCIService creates an OpenID4VCI issuer engine.
func newOpenID4VCIService(
	cfg serviceConfig, signer *issuerSigner, store openID4VCIStoreInterface,
	jwtService jwt.JWTServiceInterface, userService user.UserServiceInterface,
	creds credentialReader,
) (OpenID4VCIServiceInterface, error) {
	if signer == nil || store == nil ||
		jwtService == nil || userService == nil || creds == nil {
		return nil, fmt.Errorf("%w: required issuer dependencies are missing", ErrPolicy)
	}
	if cfg.CredentialIssuer == "" {
		return nil, fmt.Errorf("%w: credential_issuer is required", ErrPolicy)
	}
	if cfg.NonceTTL == 0 {
		cfg.NonceTTL = defaultNonceTTL
	}
	if cfg.ProofMaxAge == 0 {
		cfg.ProofMaxAge = defaultProofMaxAge
	}
	if cfg.CredentialValidity == 0 {
		cfg.CredentialValidity = defaultCredValidity
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = defaultBatchSize
	}

	return &service{
		cfg:         cfg,
		signer:      signer,
		store:       store,
		jwtService:  jwtService,
		userService: userService,
		creds:       creds,
	}, nil
}

// GetMetadata builds the OpenID4VCI credential issuer metadata from the managed
// credential configurations.
func (s *service) GetMetadata(ctx context.Context) map[string]interface{} {
	configs, svcErr := s.creds.ListCredentialConfigurations(ctx)
	if svcErr != nil {
		log.GetLogger().Error(ctx, "Failed to list credential configurations for issuer metadata")
	}
	return buildMetadata(s.cfg, configs)
}

// GenerateNonce issues a fresh, single-use c_nonce and records it under the nonce TTL.
func (s *service) GenerateNonce(ctx context.Context) (string, error) {
	nonce, err := randomToken()
	if err != nil {
		return "", fmt.Errorf("failed to generate c_nonce: %w", err)
	}
	rec := &nonceRecord{Nonce: nonce, ExpiresAt: time.Now().Add(s.cfg.NonceTTL)}
	if err := s.store.SaveNonce(ctx, rec); err != nil {
		return "", fmt.Errorf("failed to store c_nonce: %w", err)
	}
	return nonce, nil
}

// randomToken returns 32 cryptographically random bytes, base64url-encoded.
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
