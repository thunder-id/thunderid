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

// fakeServerConfigService records the value and merge flag passed to SetConfig, so tests can assert the
// importer routed a document's importBehavior to the right merge value.
type fakeServerConfigService struct {
	value     json.RawMessage
	merge     bool
	returnErr *common.ServiceError
}

func (f *fakeServerConfigService) SetConfig(
	_ context.Context, _ serverconfig.ConfigName, value json.RawMessage, merge ...bool,
) *common.ServiceError {
	if f.returnErr != nil {
		return f.returnErr
	}
	f.value = value
	f.merge = len(merge) > 0 && merge[0]
	return nil
}

func newServerConfigImportService(sc serverConfigAdapter) ImportServiceInterface {
	return newImportService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, sc)
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
	assert.JSONEq(t, `["https://app.example.com", {"regex":"^https://x$"}]`, string(scSvc.value))
}

// serverConfigBehaviorDoc builds a cors import document with the given importBehavior line (empty, or
// "importBehavior: merge\n") inserted before value.
func serverConfigBehaviorDoc(behaviorYAML string) string {
	return "resource_type: server_config\nname: cors\n" + behaviorYAML +
		"value:\n  - \"https://app.example.com\"\n"
}

func TestImportResources_ServerConfig_BehaviorOmittedDoesNotMerge(t *testing.T) {
	scSvc := &fakeServerConfigService{}
	svc := newServerConfigImportService(scSvc)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: serverConfigBehaviorDoc("")})

	require.Nil(t, err)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.False(t, scSvc.merge)
}

func TestImportResources_ServerConfig_BehaviorMergeMerges(t *testing.T) {
	scSvc := &fakeServerConfigService{}
	svc := newServerConfigImportService(scSvc)

	resp, err := svc.ImportResources(context.Background(),
		&ImportRequest{Content: serverConfigBehaviorDoc("importBehavior: merge\n")})

	require.Nil(t, err)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.True(t, scSvc.merge)
}

func TestImportResources_ServerConfig_UnknownBehaviorRejected(t *testing.T) {
	scSvc := &fakeServerConfigService{}
	svc := newServerConfigImportService(scSvc)

	resp, err := svc.ImportResources(context.Background(),
		&ImportRequest{Content: serverConfigBehaviorDoc("importBehavior: Merge\n")})

	require.Nil(t, err)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Equal(t, ErrorInvalidYAMLContent.Code, resp.Results[0].Code)
	assert.Nil(t, scSvc.value)
}

func TestImportResources_ServerConfig_DryRunDoesNotWrite(t *testing.T) {
	scSvc := &fakeServerConfigService{}
	svc := newServerConfigImportService(scSvc)

	resp, err := svc.ImportResources(context.Background(),
		&ImportRequest{Content: serverConfigImportDoc, DryRun: true})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Nil(t, scSvc.value)
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
