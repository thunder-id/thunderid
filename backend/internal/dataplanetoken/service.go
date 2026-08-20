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

package dataplanetoken

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/kmprovider"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/defaultkm"
)

// tokenBytes is the entropy of an issued token. It is the only thing standing between a network peer
// and a data plane's identity, so it is sized to be infeasible to guess rather than convenient to
// type.
const tokenBytes = 32

// Service issues and verifies data plane channel tokens.
//
// The persistence and crypto operations are fields rather than a concrete store, so a test can drive
// the issuing rules without a database or a key manager.
type Service struct {
	encrypt func(ctx context.Context, plaintext []byte) ([]byte, error)
	decrypt func(ctx context.Context, ciphertext []byte) ([]byte, error)
	putFn   func(ctx context.Context, dataPlaneID, deploymentID, ciphertext string) error
	getFn   func(ctx context.Context, dataPlaneID string) (string, bool, error)
	deleteF func(ctx context.Context, dataPlaneID string) error
}

// New builds the service over the configuration database and the configuration crypto service. That
// is the same key management protecting connection secrets, rather than a second scheme with its own
// keys to manage.
func New() *Service {
	st := newStore()
	return &Service{
		encrypt: configCrypto(func(p kmprovider.ConfigCryptoProvider) cryptoOp { return p.Encrypt }),
		decrypt: configCrypto(func(p kmprovider.ConfigCryptoProvider) cryptoOp { return p.Decrypt }),
		putFn:   st.Put,
		getFn:   st.Get,
		deleteF: st.Delete,
	}
}

type cryptoOp func(ctx context.Context, content []byte) ([]byte, error)

// configCrypto resolves the configuration crypto service on each call rather than at construction,
// because this service is built during start-up and the key manager may not be installed yet.
func configCrypto(pick func(kmprovider.ConfigCryptoProvider) cryptoOp) cryptoOp {
	return func(ctx context.Context, content []byte) ([]byte, error) {
		provider, err := defaultkm.GetConfigCryptoService()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize the encryption service: %w", err)
		}
		return pick(provider)(ctx, content)
	}
}

// Issue mints a new token for a data plane, replacing any previous one, and returns it. This is the
// only time the token is readable: what is kept is the ciphertext, and nothing decrypts it back to a
// caller.
func (s *Service) Issue(ctx context.Context, dataPlaneID, deploymentID string) (string, error) {
	if strings.TrimSpace(dataPlaneID) == "" {
		return "", fmt.Errorf("a data plane id is required to issue a token")
	}

	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("failed to generate a data plane token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	ciphertext, err := s.encrypt(ctx, []byte(token))
	if err != nil {
		return "", fmt.Errorf("failed to encrypt the data plane token: %w", err)
	}
	if err := s.putFn(ctx, dataPlaneID, deploymentID, string(ciphertext)); err != nil {
		return "", err
	}
	return token, nil
}

// TokenFor returns the token issued to a data plane, for comparing against the one it presents. The
// boolean reports whether one has been issued.
func (s *Service) TokenFor(ctx context.Context, dataPlaneID string) (string, bool, error) {
	ciphertext, found, err := s.getFn(ctx, dataPlaneID)
	if err != nil || !found {
		return "", false, err
	}
	token, err := s.decrypt(ctx, []byte(ciphertext))
	if err != nil {
		return "", false, fmt.Errorf("failed to decrypt the data plane token: %w", err)
	}
	return string(token), true, nil
}

// Revoke removes a data plane's token, which stops it reconnecting.
func (s *Service) Revoke(ctx context.Context, dataPlaneID string) error {
	return s.deleteF(ctx, dataPlaneID)
}
