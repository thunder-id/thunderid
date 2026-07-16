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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package importer

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/serverconfig"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

type fakeServerConfigService struct {
	set       map[string]json.RawMessage
	returnErr *common.ServiceError
}

func (f *fakeServerConfigService) SetConfig(
	_ context.Context, name serverconfig.ConfigName, value json.RawMessage,
) *common.ServiceError {
	if f.returnErr != nil {
		return f.returnErr
	}
	if f.set == nil {
		f.set = map[string]json.RawMessage{}
	}
	f.set[string(name)] = value
	return nil
}

func newServerConfigImportService(sc serverConfigAdapter) ImportServiceInterface {
	return newImportService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, sc)
}

const serverConfigImportDoc = `resource_type: server_config
name: cors
value:
  - "https://app.example.com"
  - regex: "^https://x$"
`

func TestImportResources_ServerConfig_SetsWritable(t *testing.T) {
	scSvc := &fakeServerConfigService{}
	svc := newServerConfigImportService(scSvc)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: serverConfigImportDoc})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, resourceTypeServerConfig, resp.Results[0].ResourceType)
	assert.Equal(t, "cors", resp.Results[0].ResourceName)
	assert.JSONEq(t, `["https://app.example.com", {"regex":"^https://x$"}]`, string(scSvc.set["cors"]))
}

func TestImportResources_ServerConfig_DryRunDoesNotWrite(t *testing.T) {
	scSvc := &fakeServerConfigService{}
	svc := newServerConfigImportService(scSvc)

	resp, err := svc.ImportResources(context.Background(),
		&ImportRequest{Content: serverConfigImportDoc, DryRun: true})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Empty(t, scSvc.set)
}

func TestImportResources_ServerConfig_ServiceErrorReported(t *testing.T) {
	scSvc := &fakeServerConfigService{returnErr: &common.ServiceError{
		Type:  common.ClientErrorType,
		Code:  "SCF-1003",
		Error: common.I18nMessage{DefaultValue: "Invalid server configuration value"},
	}}
	svc := newServerConfigImportService(scSvc)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: serverConfigImportDoc})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Equal(t, "SCF-1003", resp.Results[0].Code)
}

func TestImportResources_ServerConfig_AdapterNotConfigured(t *testing.T) {
	svc := newServerConfigImportService(nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: serverConfigImportDoc})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
}
