// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

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
	handler.EXPECT().Validate(mock.Anything, "decoded", nil, nil).Return(nil)

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
	handler.EXPECT().Validate(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("bad value"))

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
