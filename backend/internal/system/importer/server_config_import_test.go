// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
	return newImportService(nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, sc, nil)
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
