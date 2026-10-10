// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func viewServerConfigDocument(t *testing.T, content string) *yaml.Node {
	t.Helper()
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte("resource_type: server_config\n"+content), &root))
	return root.Content[0]
}

// An exported section is shown as its GET returns it: the document as the writable layer over an
// absent read-only layer, merged by the section's handler.
func TestServerConfigExporter_ViewResourceShowsTheDocumentAsItsRead(t *testing.T) {
	exported, err := yaml.Marshal(&serverConfigExportDoc{
		Name:  "cors",
		Value: map[string]interface{}{"allowedOrigins": []string{"https://app.example"}},
	})
	require.NoError(t, err)
	handler := NewServerConfigHandlerInterfaceMock(t)
	handler.EXPECT().Decode(json.RawMessage(nil)).Return([]string{}, nil)
	handler.EXPECT().Decode(json.RawMessage(`{"allowedOrigins":["https://app.example"]}`)).
		Return([]string{"https://app.example"}, nil)
	handler.EXPECT().Merge([]string{}, []string{"https://app.example"}).Return([]string{"https://app.example"})
	exporter := newServerConfigExporter(nil, map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: handler})

	view, err := exporter.ViewResource(context.Background(), viewServerConfigDocument(t, string(exported)))

	require.NoError(t, err)
	assert.Equal(t, ServerConfigLayers{
		ReadOnly: []string{},
		Writable: []string{"https://app.example"},
		Merged:   []string{"https://app.example"},
	}, view)
}

// A document that is not a section, names no served section, or holds a value its handler refuses is
// refused.
func TestServerConfigExporter_ViewResourceRefusesAnUnreadableDocument(t *testing.T) {
	handler := NewServerConfigHandlerInterfaceMock(t)
	handler.EXPECT().Decode(json.RawMessage(nil)).Return(nil, nil)
	handler.EXPECT().Decode(mock.Anything).Return(nil, errors.New("bad value"))
	exporter := newServerConfigExporter(nil, map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: handler})

	_, err := exporter.ViewResource(context.Background(), viewServerConfigDocument(t, "name: [not, a, name]"))
	assert.Error(t, err)

	_, err = exporter.ViewResource(context.Background(), viewServerConfigDocument(t, "name: bogus\nvalue: {}"))
	assert.Error(t, err)

	_, err = exporter.ViewResource(context.Background(), viewServerConfigDocument(t, "name: cors\nvalue: 7"))
	assert.Error(t, err)
}
