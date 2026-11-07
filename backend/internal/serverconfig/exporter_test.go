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
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

func TestServerConfigExporter_Type(t *testing.T) {
	exporter := newServerConfigExporter(NewServerConfigServiceMock(t), nil)
	assert.Equal(t, "server_config", exporter.GetResourceType())
	assert.Equal(t, "ServerConfig", exporter.GetParameterizerType())
}

func TestServerConfigExporter_GetAllResourceIDs(t *testing.T) {
	service := NewServerConfigServiceMock(t)
	service.EXPECT().ListConfigNames(mock.Anything).Return([]ConfigName{ConfigNameCORS}, nil)

	ids, svcErr := newServerConfigExporter(service, nil).GetAllResourceIDs(context.Background())
	assert.Nil(t, svcErr)
	assert.Equal(t, []string{"cors"}, ids)
}

func TestServerConfigExporter_GetAllResourceIDs_LeavesOutGateway(t *testing.T) {
	service := NewServerConfigServiceMock(t)
	service.EXPECT().ListConfigNames(mock.Anything).Return([]ConfigName{ConfigNameCORS, ConfigNameGateway}, nil)

	ids, svcErr := newServerConfigExporter(service, nil).GetAllResourceIDs(context.Background())
	assert.Nil(t, svcErr)
	assert.Equal(t, []string{"cors"}, ids)
}

func TestServerConfigExporter_GetAllResourceIDs_Error(t *testing.T) {
	service := NewServerConfigServiceMock(t)
	service.EXPECT().ListConfigNames(mock.Anything).Return(nil, &common.InternalServerError)

	_, svcErr := newServerConfigExporter(service, nil).GetAllResourceIDs(context.Background())
	assert.Same(t, &common.InternalServerError, svcErr)
}

func TestServerConfigExporter_GetResourceByID_ExportsEffectiveValue(t *testing.T) {
	service := NewServerConfigServiceMock(t)
	service.EXPECT().GetConfig(mock.Anything, ConfigNameCORS).
		Return(ServerConfigLayers{Merged: mergedValue}, nil)

	resource, name, svcErr := newServerConfigExporter(service, nil).GetResourceByID(context.Background(), "cors")
	assert.Nil(t, svcErr)
	assert.Equal(t, "cors", name)

	doc := resource.(*serverConfigExportDoc)
	assert.Equal(t, "cors", doc.Name)
	raw, err := json.Marshal(doc.Value)
	require.NoError(t, err)
	assert.JSONEq(t, string(mergedValue), string(raw))
}

func TestServerConfigExporter_GetResourceByID_Error(t *testing.T) {
	service := NewServerConfigServiceMock(t)
	service.EXPECT().GetConfig(mock.Anything, ConfigNameCORS).
		Return(ServerConfigLayers{}, &common.InternalServerError)

	_, _, svcErr := newServerConfigExporter(service, nil).GetResourceByID(context.Background(), "cors")
	assert.Same(t, &common.InternalServerError, svcErr)
}

// An export that names the gateway section is refused as one that does not name a section, the same
// as an export of everything leaves it out; the mock fails the test if the section is read.
func TestServerConfigExporter_GetResourceByID_RefusesGateway(t *testing.T) {
	_, _, svcErr := newServerConfigExporter(NewServerConfigServiceMock(t), nil).
		GetResourceByID(context.Background(), string(ConfigNameGateway))
	assert.Same(t, &ErrorUnsupportedConfigName, svcErr)
}

func TestServerConfigExporter_ValidateResource(t *testing.T) {
	exporter := newServerConfigExporter(NewServerConfigServiceMock(t), nil)

	name, exportErr := exporter.ValidateResource(context.Background(),
		&serverConfigExportDoc{Name: "cors"}, "cors", log.GetLogger())
	assert.Nil(t, exportErr)
	assert.Equal(t, "cors", name)
}

func TestServerConfigExporter_ValidateResource_WrongType(t *testing.T) {
	exporter := newServerConfigExporter(NewServerConfigServiceMock(t), nil)

	_, exportErr := exporter.ValidateResource(context.Background(), "not a doc", "cors", log.GetLogger())
	assert.NotNil(t, exportErr)
}

func TestServerConfigExporter_ValidateResource_EmptyName(t *testing.T) {
	exporter := newServerConfigExporter(NewServerConfigServiceMock(t), nil)

	_, exportErr := exporter.ValidateResource(context.Background(),
		&serverConfigExportDoc{Name: ""}, "cors", log.GetLogger())
	assert.NotNil(t, exportErr)
}

// TestServerConfigExportDoc_YAMLShape confirms the export document marshals to the declarative shape the
// loader parses back (name + a value object carrying allowedOrigins), which is what the export parameterizer
// produces.
func TestServerConfigExportDoc_YAMLShape(t *testing.T) {
	out, err := yaml.Marshal(&serverConfigExportDoc{
		Name:  "cors",
		Value: map[string]interface{}{"allowedOrigins": []interface{}{"https://app.example.com"}},
	})
	require.NoError(t, err)
	assert.Contains(t, string(out), "name: cors")
	assert.Contains(t, string(out), "value:")
	assert.Contains(t, string(out), "allowedOrigins:")
	assert.Contains(t, string(out), "- https://app.example.com")
}
