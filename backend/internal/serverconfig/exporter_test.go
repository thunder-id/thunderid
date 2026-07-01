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
	exporter := newServerConfigExporter(NewServerConfigServiceMock(t))
	assert.Equal(t, "server_config", exporter.GetResourceType())
	assert.Equal(t, "ServerConfig", exporter.GetParameterizerType())
}

func TestServerConfigExporter_GetAllResourceIDs(t *testing.T) {
	service := NewServerConfigServiceMock(t)
	service.EXPECT().ListConfigNames(mock.Anything).Return([]ConfigName{ConfigNameCORS}, nil)

	ids, svcErr := newServerConfigExporter(service).GetAllResourceIDs(context.Background())
	assert.Nil(t, svcErr)
	assert.Equal(t, []string{"cors"}, ids)
}

func TestServerConfigExporter_GetAllResourceIDs_Error(t *testing.T) {
	service := NewServerConfigServiceMock(t)
	service.EXPECT().ListConfigNames(mock.Anything).Return(nil, &common.InternalServerError)

	_, svcErr := newServerConfigExporter(service).GetAllResourceIDs(context.Background())
	assert.Same(t, &common.InternalServerError, svcErr)
}

func TestServerConfigExporter_GetResourceByID_ExportsEffectiveValue(t *testing.T) {
	service := NewServerConfigServiceMock(t)
	service.EXPECT().GetConfig(mock.Anything, ConfigNameCORS).
		Return(ServerConfigLayers{Merged: mergedValue}, nil)

	resource, name, svcErr := newServerConfigExporter(service).GetResourceByID(context.Background(), "cors")
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

	_, _, svcErr := newServerConfigExporter(service).GetResourceByID(context.Background(), "cors")
	assert.Same(t, &common.InternalServerError, svcErr)
}

func TestServerConfigExporter_ValidateResource(t *testing.T) {
	exporter := newServerConfigExporter(NewServerConfigServiceMock(t))

	name, exportErr := exporter.ValidateResource(context.Background(),
		&serverConfigExportDoc{Name: "cors"}, "cors", log.GetLogger())
	assert.Nil(t, exportErr)
	assert.Equal(t, "cors", name)
}

func TestServerConfigExporter_ValidateResource_WrongType(t *testing.T) {
	exporter := newServerConfigExporter(NewServerConfigServiceMock(t))

	_, exportErr := exporter.ValidateResource(context.Background(), "not a doc", "cors", log.GetLogger())
	assert.NotNil(t, exportErr)
}

func TestServerConfigExporter_ValidateResource_EmptyName(t *testing.T) {
	exporter := newServerConfigExporter(NewServerConfigServiceMock(t))

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
