// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// TestCredentialHashRoundTrip guards against parameter propagation regressions like #5303.
func TestCredentialHashRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		cfg  cryptolib.HashConfig
	}{
		{name: "SHA256", cfg: cryptolib.HashConfig{Algorithm: cryptolib.SHA256, SaltSize: 16}},
		{name: "PBKDF2", cfg: cryptolib.HashConfig{
			Algorithm: cryptolib.PBKDF2, SaltSize: 16, Iterations: 10000, KeySize: 32,
		}},
		{name: "ARGON2ID", cfg: cryptolib.HashConfig{
			Algorithm: cryptolib.ARGON2ID, SaltSize: 16, Memory: 65536, Iterations: 3,
			Parallelism: 2, KeySize: 32,
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hashService, err := cryptolib.Initialize(tt.cfg)
			require.NoError(t, err)

			svc := &entityService{
				hashService: hashService,
				logger:      log.GetLogger(),
			}

			plaintextJSON, err := json.Marshal(map[string]interface{}{"password": "correct horse battery staple"})
			require.NoError(t, err)

			storedJSON, err := svc.hashPlaintextCredentials(plaintextJSON)
			require.NoError(t, err)

			err = svc.verifyCredentials(t.Context(),
				map[string]interface{}{"password": "correct horse battery staple"}, storedJSON, nil)
			require.NoError(t, err, "correct password must verify against the persisted credential")

			err = svc.verifyCredentials(t.Context(),
				map[string]interface{}{"password": "wrong password"}, storedJSON, nil)
			require.ErrorIs(t, err, ErrAuthenticationFailed)
		})
	}
}
