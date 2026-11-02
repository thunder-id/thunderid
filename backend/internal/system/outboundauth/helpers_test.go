// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outboundauth

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/cryptomock"
)

// encryptedPrefix marks a value the test crypto provider encrypted, so a stored secret is visibly
// different from its plaintext.
var encryptedPrefix = []byte("encrypted:")

// setUpCryptoProvider installs a crypto provider mock for secret properties.
func setUpCryptoProvider(t *testing.T) {
	provider := cryptomock.NewConfigCryptoProviderMock(t)
	provider.EXPECT().Encrypt(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, content []byte) ([]byte, error) {
			return append(bytes.Clone(encryptedPrefix), content...), nil
		}).Maybe()
	provider.EXPECT().Decrypt(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, content []byte) ([]byte, error) {
			return bytes.TrimPrefix(content, encryptedPrefix), nil
		}).Maybe()
	cmodels.SetConfigCryptoProvider(provider)
}

func basicConfig() Config {
	return Config{
		Type: TypeBasic,
		Properties: map[string]string{
			FieldBasicUsername: "mailer",
			FieldBasicPassword: "s3cret",
		},
	}
}

func allTypes() []Type {
	return []Type{TypeNone, TypeBasic}
}

// propertyMap resolves a property slice to a name/value map for assertions.
func propertyMap(t *testing.T, props []cmodels.Property) map[string]string {
	values := make(map[string]string, len(props))
	for i := range props {
		value, err := props[i].GetValue()
		require.NoError(t, err)
		values[props[i].GetName()] = value
	}
	return values
}

// withTemporaryMethod runs body with method added to the registry.
func withTemporaryMethod(method Method, body func()) {
	original := registeredMethods
	registeredMethods = append(append([]Method{}, registeredMethods...), method)
	defer func() { registeredMethods = original }()
	body()
}
