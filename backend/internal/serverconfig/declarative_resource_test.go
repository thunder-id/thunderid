// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cors"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// setDeploymentCORSConfig points the server runtime's Config.CORS at the given allowedOrigins YAML
// sequence (e.g. "- https://app.example.com\n") and returns a cleanup func that resets the runtime.
func setDeploymentCORSConfig(t *testing.T, allowedOriginsYAML string) func() {
	t.Helper()
	config.ResetServerRuntime()

	var corsCfg cors.OriginConfig
	if allowedOriginsYAML != "" {
		require.NoError(t, yaml.Unmarshal(
			[]byte("allowedOrigins:\n"+allowedOriginsYAML), &corsCfg))
	}
	cfg := &config.Config{
		Server: engineconfig.ServerConfig{Identifier: "test-deployment"},
		CORS:   corsCfg,
	}
	require.NoError(t, config.InitializeServerRuntime("", cfg))
	return config.ResetServerRuntime
}

func TestYamlNodeToJSON(t *testing.T) {
	var doc struct {
		Value yaml.Node `yaml:"value"`
	}
	require.NoError(t, yaml.Unmarshal(
		[]byte("value:\n  - https://app.example.com\n  - regex: \"^https://x$\"\n"), &doc))

	raw, err := yamlNodeToJSON(doc.Value)
	require.NoError(t, err)
	assert.JSONEq(t, `["https://app.example.com", {"regex":"^https://x$"}]`, string(raw))
}

func TestParseServerConfigDoc_OK(t *testing.T) {
	parsed, err := parseServerConfigDoc([]byte("name: cors\nvalue:\n  - https://app.example.com\n"))
	require.NoError(t, err)

	doc := parsed.(*serverConfigDoc)
	assert.Equal(t, ConfigNameCORS, doc.Name)
	assert.JSONEq(t, `["https://app.example.com"]`, string(doc.Value))
}

func TestParseServerConfigDoc_BadYAML(t *testing.T) {
	_, err := parseServerConfigDoc([]byte("name: cors\nvalue: [unclosed"))
	assert.Error(t, err)
}

func newTestFileStore(t *testing.T) *fileBasedStore {
	store := newFileBasedStore()
	require.NoError(t, store.ClearByType())
	return store
}

func TestValidateServerConfigDoc_OK(t *testing.T) {
	handler := NewServerConfigHandlerInterfaceMock(t)
	handler.EXPECT().Decode(corsValue).Return("decoded", nil)
	handler.EXPECT().Validate("decoded", nil, nil).Return(nil)

	err := validateServerConfigDoc(&serverConfigDoc{Name: ConfigNameCORS, Value: corsValue},
		newTestFileStore(t), map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: handler})
	assert.NoError(t, err)
}

func TestValidateServerConfigDoc_UnsupportedName(t *testing.T) {
	err := validateServerConfigDoc(&serverConfigDoc{Name: ConfigName("bogus"), Value: corsValue},
		newTestFileStore(t), map[ConfigName]ServerConfigHandlerInterface{})
	assert.Error(t, err)
}

func TestValidateServerConfigDoc_Duplicate(t *testing.T) {
	store := newTestFileStore(t)
	require.NoError(t, store.Create("cors", &serverConfigDoc{Name: ConfigNameCORS, Value: corsValue}))

	err := validateServerConfigDoc(&serverConfigDoc{Name: ConfigNameCORS, Value: corsValue},
		store, map[ConfigName]ServerConfigHandlerInterface{})
	assert.Error(t, err)
}

func TestValidateServerConfigDoc_NoHandler(t *testing.T) {
	err := validateServerConfigDoc(&serverConfigDoc{Name: ConfigNameCORS, Value: corsValue},
		newTestFileStore(t), map[ConfigName]ServerConfigHandlerInterface{})
	assert.Error(t, err)
}

func TestValidateServerConfigDoc_HandlerRejects(t *testing.T) {
	handler := NewServerConfigHandlerInterfaceMock(t)
	handler.EXPECT().Decode(corsValue).Return("decoded", nil)
	handler.EXPECT().Validate(mock.Anything, mock.Anything, mock.Anything).Return(errors.New("bad value"))

	err := validateServerConfigDoc(&serverConfigDoc{Name: ConfigNameCORS, Value: corsValue},
		newTestFileStore(t), map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: handler})
	assert.Error(t, err)
}

func TestValidateServerConfigDoc_DecodeFails(t *testing.T) {
	handler := NewServerConfigHandlerInterfaceMock(t)
	handler.EXPECT().Decode(corsValue).Return(nil, errors.New("bad shape"))

	err := validateServerConfigDoc(&serverConfigDoc{Name: ConfigNameCORS, Value: corsValue},
		newTestFileStore(t), map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: handler})
	assert.Error(t, err)
}

func TestSeedCORSFromDeploymentConfig_NoOriginsIsNoop(t *testing.T) {
	defer setDeploymentCORSConfig(t, "")()

	store := newTestFileStore(t)
	require.NoError(t, seedCORSFromDeploymentConfig(store, map[ConfigName]ServerConfigHandlerInterface{}))

	_, ok := store.GetByName(ConfigNameCORS)
	assert.False(t, ok)
}

func TestSeedCORSFromDeploymentConfig_OK(t *testing.T) {
	defer setDeploymentCORSConfig(t, "  - https://app.example.com\n")()

	store := newTestFileStore(t)
	err := seedCORSFromDeploymentConfig(store,
		map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: cors.OriginHandler{}})
	require.NoError(t, err)

	value, ok := store.GetByName(ConfigNameCORS)
	require.True(t, ok)
	assert.JSONEq(t, `{"allowedOrigins":["https://app.example.com"]}`, string(value))
}

func TestSeedCORSFromDeploymentConfig_NoHandler(t *testing.T) {
	defer setDeploymentCORSConfig(t, "  - https://app.example.com\n")()

	err := seedCORSFromDeploymentConfig(newTestFileStore(t), map[ConfigName]ServerConfigHandlerInterface{})
	assert.Error(t, err)
}

func TestSeedCORSFromDeploymentConfig_HandlerRejects(t *testing.T) {
	defer setDeploymentCORSConfig(t, "  - not-a-valid-origin\n")()

	err := seedCORSFromDeploymentConfig(newTestFileStore(t),
		map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: cors.OriginHandler{}})
	assert.Error(t, err)
}

func TestSeedCORSFromDeploymentConfig_DuplicateRejectsExisting(t *testing.T) {
	defer setDeploymentCORSConfig(t, "  - https://app.example.com\n")()

	store := newTestFileStore(t)
	require.NoError(t, store.Create("cors", &serverConfigDoc{Name: ConfigNameCORS, Value: corsValue}))

	err := seedCORSFromDeploymentConfig(store,
		map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: cors.OriginHandler{}})
	assert.Error(t, err)
}

func TestSeedMutableCORSFromDeploymentConfig_NoOriginsIsNoop(t *testing.T) {
	defer setDeploymentCORSConfig(t, "")()

	store := newServerConfigStoreInterfaceMock(t)
	require.NoError(t, seedMutableCORSFromDeploymentConfig(store, map[ConfigName]ServerConfigHandlerInterface{}))
}

func TestSeedMutableCORSFromDeploymentConfig_NoHandler(t *testing.T) {
	defer setDeploymentCORSConfig(t, "  - https://app.example.com\n")()

	store := newServerConfigStoreInterfaceMock(t)
	err := seedMutableCORSFromDeploymentConfig(store, map[ConfigName]ServerConfigHandlerInterface{})
	assert.Error(t, err)
}

func TestSeedMutableCORSFromDeploymentConfig_InvalidDeploymentValue(t *testing.T) {
	defer setDeploymentCORSConfig(t, "  - not-a-valid-origin\n")()

	store := newServerConfigStoreInterfaceMock(t)
	err := seedMutableCORSFromDeploymentConfig(store,
		map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: cors.OriginHandler{}})
	assert.Error(t, err)
}

func TestSeedMutableCORSFromDeploymentConfig_NoExistingWritableUpsertsDeploymentValue(t *testing.T) {
	defer setDeploymentCORSConfig(t, "  - https://app.example.com\n")()

	store := newServerConfigStoreInterfaceMock(t)
	store.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).Return(storeLayers{}, nil)
	store.EXPECT().UpsertServerConfig(mock.Anything, mock.MatchedBy(func(cfg ServerConfig) bool {
		return cfg.Name == ConfigNameCORS &&
			assert.JSONEq(t, `{"allowedOrigins":["https://app.example.com"]}`, string(cfg.Value))
	})).Return(nil)

	err := seedMutableCORSFromDeploymentConfig(store,
		map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: cors.OriginHandler{}})
	require.NoError(t, err)
}

func TestSeedMutableCORSFromDeploymentConfig_MergesWithExistingWritable(t *testing.T) {
	defer setDeploymentCORSConfig(t, "  - https://app.example.com\n")()

	store := newServerConfigStoreInterfaceMock(t)
	store.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{Writable: json.RawMessage(`{"allowedOrigins":["https://runtime.example.com"]}`)}, nil)
	store.EXPECT().UpsertServerConfig(mock.Anything, mock.MatchedBy(func(cfg ServerConfig) bool {
		return cfg.Name == ConfigNameCORS &&
			assert.JSONEq(t, `{"allowedOrigins":["https://app.example.com","https://runtime.example.com"]}`,
				string(cfg.Value))
	})).Return(nil)

	err := seedMutableCORSFromDeploymentConfig(store,
		map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: cors.OriginHandler{}})
	require.NoError(t, err)
}

func TestSeedMutableCORSFromDeploymentConfig_GetServerConfigError(t *testing.T) {
	defer setDeploymentCORSConfig(t, "  - https://app.example.com\n")()

	store := newServerConfigStoreInterfaceMock(t)
	store.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).Return(storeLayers{}, errors.New("db down"))

	err := seedMutableCORSFromDeploymentConfig(store,
		map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: cors.OriginHandler{}})
	assert.Error(t, err)
}

func TestSeedMutableCORSFromDeploymentConfig_DecodeDeploymentValueFails(t *testing.T) {
	defer setDeploymentCORSConfig(t, "  - https://app.example.com\n")()

	handler := NewServerConfigHandlerInterfaceMock(t)
	handler.EXPECT().Decode(mock.Anything).Return(nil, errors.New("bad deployment value"))

	store := newServerConfigStoreInterfaceMock(t)
	err := seedMutableCORSFromDeploymentConfig(store,
		map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: handler})
	assert.Error(t, err)
}

func TestSeedMutableCORSFromDeploymentConfig_ValidateDeploymentValueFails(t *testing.T) {
	defer setDeploymentCORSConfig(t, "  - https://app.example.com\n")()

	handler := NewServerConfigHandlerInterfaceMock(t)
	handler.EXPECT().Decode(mock.Anything).Return("decoded", nil)
	handler.EXPECT().Validate("decoded", nil, nil).Return(errors.New("rejected"))

	store := newServerConfigStoreInterfaceMock(t)
	err := seedMutableCORSFromDeploymentConfig(store,
		map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: handler})
	assert.Error(t, err)
}

func TestSeedMutableCORSFromDeploymentConfig_DecodeExistingWritableFails(t *testing.T) {
	defer setDeploymentCORSConfig(t, "  - https://app.example.com\n")()

	existingWritable := json.RawMessage(`{"allowedOrigins":["https://existing.example.com"]}`)

	handler := NewServerConfigHandlerInterfaceMock(t)
	handler.EXPECT().Decode(mock.MatchedBy(func(raw json.RawMessage) bool {
		return string(raw) != string(existingWritable)
	})).Return("deployment-cfg", nil)
	handler.EXPECT().Validate("deployment-cfg", nil, nil).Return(nil)
	handler.EXPECT().Decode(existingWritable).Return(nil, errors.New("corrupt existing value"))

	store := newServerConfigStoreInterfaceMock(t)
	store.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{Writable: existingWritable}, nil)

	err := seedMutableCORSFromDeploymentConfig(store,
		map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: handler})
	assert.Error(t, err)
}

func TestSeedMutableCORSFromDeploymentConfig_MergedValueMarshalFails(t *testing.T) {
	defer setDeploymentCORSConfig(t, "  - https://app.example.com\n")()

	handler := NewServerConfigHandlerInterfaceMock(t)
	handler.EXPECT().Decode(mock.Anything).Return("decoded", nil)
	handler.EXPECT().Validate("decoded", nil, nil).Return(nil)
	// A channel value cannot be JSON-marshaled, forcing the post-Merge json.Marshal call to fail.
	handler.EXPECT().Merge("decoded", "decoded").Return(make(chan int))

	store := newServerConfigStoreInterfaceMock(t)
	store.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).Return(storeLayers{}, nil)

	err := seedMutableCORSFromDeploymentConfig(store,
		map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: handler})
	assert.Error(t, err)
}

func TestSeedMutableCORSFromDeploymentConfig_UpsertServerConfigFails(t *testing.T) {
	defer setDeploymentCORSConfig(t, "  - https://app.example.com\n")()

	store := newServerConfigStoreInterfaceMock(t)
	store.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).Return(storeLayers{}, nil)
	store.EXPECT().UpsertServerConfig(mock.Anything, mock.Anything).Return(errors.New("db write failed"))

	err := seedMutableCORSFromDeploymentConfig(store,
		map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: cors.OriginHandler{}})
	assert.Error(t, err)
}
