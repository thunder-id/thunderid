// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/cors"
)

// TestGetConfig_EmptyLayersSerializeToArrays wires the real CORS handler with an empty store and asserts
// that unset layers serialize as [] (not null), consistent with the merged layer.
func TestGetConfig_EmptyLayersSerializeToArrays(t *testing.T) {
	store := newServerConfigStoreInterfaceMock(t)
	store.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).Return(storeLayers{}, nil)

	svc := newServerConfigService(store,
		map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: cors.OriginHandler{}}, nil)

	layers, svcErr := svc.GetConfig(context.Background(), ConfigNameCORS)
	require.Nil(t, svcErr)

	out, err := json.Marshal(layers)
	require.NoError(t, err)
	assert.JSONEq(t,
		`{"readOnly":{"allowedOrigins":[]},"writable":{"allowedOrigins":[]},"merged":{"allowedOrigins":[]}}`,
		string(out))
}
