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
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/cors"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

type ServiceTestSuite struct {
	suite.Suite
	ctx         context.Context
	mockStore   *serverConfigStoreInterfaceMock
	mockHandler *ServerConfigHandlerInterfaceMock
	service     ServerConfigService
}

func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

func (suite *ServiceTestSuite) SetupTest() {
	suite.ctx = context.Background()
	suite.mockStore = newServerConfigStoreInterfaceMock(suite.T())
	suite.mockHandler = NewServerConfigHandlerInterfaceMock(suite.T())
	suite.service = newServerConfigService(suite.mockStore,
		map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: suite.mockHandler})
}

// serviceWithoutHandlers builds a service with no registered handlers, sharing the suite store mock.
func (suite *ServiceTestSuite) serviceWithoutHandlers() ServerConfigService {
	return newServerConfigService(suite.mockStore, map[ConfigName]ServerConfigHandlerInterface{})
}

// Raw (byte) layers shared across the store tests, plus an incoming PUT value.
var (
	corsValue   = json.RawMessage(`["https://app.example.com"]`)
	declarative = json.RawMessage(`["https://static.example.com"]`)
	mergedValue = json.RawMessage(`["https://static.example.com","https://app.example.com"]`)
	incomingRaw = json.RawMessage(`["https://new.example.com"]`)
)

// Decoded sentinels the mocked handler yields for the raw layers above; they flow through the service
// as opaque values, so simple strings suffice.
const (
	readOnlyVal = "decoded-readonly"
	writableVal = "decoded-writable"
	incomingVal = "decoded-incoming"
	mergedVal   = "decoded-merged"
	patchedVal  = "patched"
)

// --- ListConfigNames ---

func (suite *ServiceTestSuite) TestListConfigNames() {
	names, svcErr := suite.service.ListConfigNames(suite.ctx)
	assert.Nil(suite.T(), svcErr)
	assert.Equal(suite.T(), supportedConfigNames, names)
}

// --- GetConfig ---

func (suite *ServiceTestSuite) TestGetConfig_UnsupportedName() {
	layers, svcErr := suite.service.GetConfig(suite.ctx, ConfigName("bogus"))
	assert.Equal(suite.T(), ServerConfigLayers{}, layers)
	assert.Same(suite.T(), &ErrorUnsupportedConfigName, svcErr)
}

func (suite *ServiceTestSuite) TestGetConfig_NoHandlerRegistered_FailClosed() {
	layers, svcErr := suite.serviceWithoutHandlers().GetConfig(suite.ctx, ConfigNameCORS)
	assert.Equal(suite.T(), ServerConfigLayers{}, layers)
	assert.Same(suite.T(), &common.InternalServerError, svcErr)
}

func (suite *ServiceTestSuite) TestGetConfig_StoreError() {
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{}, errors.New("db error"))
	layers, svcErr := suite.service.GetConfig(suite.ctx, ConfigNameCORS)
	assert.Equal(suite.T(), ServerConfigLayers{}, layers)
	assert.Same(suite.T(), &common.InternalServerError, svcErr)
}

func (suite *ServiceTestSuite) TestGetConfig_DecodeError() {
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{ReadOnly: declarative, Writable: corsValue}, nil)
	suite.mockHandler.EXPECT().Decode(declarative).Return(nil, errors.New("corrupt stored value"))
	layers, svcErr := suite.service.GetConfig(suite.ctx, ConfigNameCORS)
	assert.Equal(suite.T(), ServerConfigLayers{}, layers)
	assert.Same(suite.T(), &common.InternalServerError, svcErr)
}

func (suite *ServiceTestSuite) TestGetConfig_OK() {
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{ReadOnly: declarative, Writable: corsValue}, nil)
	suite.mockHandler.EXPECT().Decode(declarative).Return(readOnlyVal, nil)
	suite.mockHandler.EXPECT().Decode(corsValue).Return(writableVal, nil)
	suite.mockHandler.EXPECT().Merge(readOnlyVal, writableVal).Return(mergedVal)

	layers, svcErr := suite.service.GetConfig(suite.ctx, ConfigNameCORS)
	assert.Nil(suite.T(), svcErr)
	assert.Equal(suite.T(), readOnlyVal, layers.ReadOnly)
	assert.Equal(suite.T(), writableVal, layers.Writable)
	assert.Equal(suite.T(), mergedVal, layers.Merged)
}

func (suite *ServiceTestSuite) TestGetConfig_Unset() {
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).Return(storeLayers{}, nil)
	suite.mockHandler.EXPECT().Decode(json.RawMessage(nil)).Return(nil, nil)
	suite.mockHandler.EXPECT().Merge(nil, nil).Return(mergedVal)

	layers, svcErr := suite.service.GetConfig(suite.ctx, ConfigNameCORS)
	assert.Nil(suite.T(), svcErr)
	assert.Nil(suite.T(), layers.ReadOnly)
	assert.Nil(suite.T(), layers.Writable)
	assert.Equal(suite.T(), mergedVal, layers.Merged)
}

// --- GetReadOnlyConfig / GetWritableConfig ---

func (suite *ServiceTestSuite) TestGetReadOnlyConfig() {
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{ReadOnly: declarative, Writable: corsValue}, nil)
	// Only the read-only layer is decoded; the absence of a Decode(corsValue) expectation asserts the
	// writable layer is never touched (mockery fails on the unexpected call).
	suite.mockHandler.EXPECT().Decode(declarative).Return(readOnlyVal, nil)

	value, svcErr := suite.service.GetReadOnlyConfig(suite.ctx, string(ConfigNameCORS))
	assert.Nil(suite.T(), svcErr)
	assert.Equal(suite.T(), readOnlyVal, value)
}

func (suite *ServiceTestSuite) TestGetReadOnlyConfig_UnsupportedName() {
	value, svcErr := suite.service.GetReadOnlyConfig(suite.ctx, "bogus")
	assert.Nil(suite.T(), value)
	assert.Same(suite.T(), &ErrorUnsupportedConfigName, svcErr)
	suite.mockStore.AssertNotCalled(suite.T(), "GetServerConfig", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestGetReadOnlyConfig_StoreError() {
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{}, errors.New("db error"))
	value, svcErr := suite.service.GetReadOnlyConfig(suite.ctx, string(ConfigNameCORS))
	assert.Nil(suite.T(), value)
	assert.Same(suite.T(), &common.InternalServerError, svcErr)
}

func (suite *ServiceTestSuite) TestGetReadOnlyConfig_DecodeError() {
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{ReadOnly: declarative, Writable: corsValue}, nil)
	suite.mockHandler.EXPECT().Decode(declarative).Return(nil, errors.New("corrupt stored value"))
	value, svcErr := suite.service.GetReadOnlyConfig(suite.ctx, string(ConfigNameCORS))
	assert.Nil(suite.T(), value)
	assert.Same(suite.T(), &common.InternalServerError, svcErr)
}

func (suite *ServiceTestSuite) TestGetWritableConfig() {
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{ReadOnly: declarative, Writable: corsValue}, nil)
	// Only the writable layer is decoded; the absence of a Decode(declarative) expectation guards against
	// a regression to decoding both layers (decodeLayers) on the writable path.
	suite.mockHandler.EXPECT().Decode(corsValue).Return(writableVal, nil)

	value, svcErr := suite.service.GetWritableConfig(suite.ctx, string(ConfigNameCORS))
	assert.Nil(suite.T(), svcErr)
	assert.Equal(suite.T(), writableVal, value)
}

func (suite *ServiceTestSuite) TestGetWritableConfig_UnsupportedName() {
	value, svcErr := suite.service.GetWritableConfig(suite.ctx, "bogus")
	assert.Nil(suite.T(), value)
	assert.Same(suite.T(), &ErrorUnsupportedConfigName, svcErr)
	suite.mockStore.AssertNotCalled(suite.T(), "GetServerConfig", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestGetWritableConfig_StoreError() {
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{}, errors.New("db error"))
	value, svcErr := suite.service.GetWritableConfig(suite.ctx, string(ConfigNameCORS))
	assert.Nil(suite.T(), value)
	assert.Same(suite.T(), &common.InternalServerError, svcErr)
}

func (suite *ServiceTestSuite) TestGetWritableConfig_DecodeError() {
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{ReadOnly: declarative, Writable: corsValue}, nil)
	suite.mockHandler.EXPECT().Decode(corsValue).Return(nil, errors.New("corrupt stored value"))
	value, svcErr := suite.service.GetWritableConfig(suite.ctx, string(ConfigNameCORS))
	assert.Nil(suite.T(), value)
	assert.Same(suite.T(), &common.InternalServerError, svcErr)
}

// --- GetMergedConfig ---

func (suite *ServiceTestSuite) TestGetMergedConfig_OK() {
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{ReadOnly: declarative, Writable: corsValue}, nil)
	suite.mockHandler.EXPECT().Decode(declarative).Return(readOnlyVal, nil)
	suite.mockHandler.EXPECT().Decode(corsValue).Return(writableVal, nil)
	suite.mockHandler.EXPECT().Merge(readOnlyVal, writableVal).Return(mergedVal)

	merged, svcErr := suite.service.GetMergedConfig(suite.ctx, string(ConfigNameCORS))
	assert.Nil(suite.T(), svcErr)
	assert.Equal(suite.T(), mergedVal, merged)
}

func (suite *ServiceTestSuite) TestGetMergedConfig_UnsupportedName() {
	merged, svcErr := suite.service.GetMergedConfig(suite.ctx, "bogus")
	assert.Nil(suite.T(), merged)
	assert.Same(suite.T(), &ErrorUnsupportedConfigName, svcErr)
}

// --- SetConfig ---

func (suite *ServiceTestSuite) TestSetConfig_UnsupportedName() {
	svcErr := suite.service.SetConfig(suite.ctx, ConfigName("bogus"), incomingRaw)
	assert.Same(suite.T(), &ErrorUnsupportedConfigName, svcErr)
	suite.mockStore.AssertNotCalled(suite.T(), "UpsertServerConfig", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestSetConfig_NoHandlerRegistered_FailClosed() {
	svcErr := suite.serviceWithoutHandlers().SetConfig(suite.ctx, ConfigNameCORS, incomingRaw)
	assert.Same(suite.T(), &common.InternalServerError, svcErr)
	suite.mockStore.AssertNotCalled(suite.T(), "UpsertServerConfig", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestSetConfig_DecodeIncomingError() {
	suite.mockHandler.EXPECT().Decode(incomingRaw).Return(nil, errors.New("bad shape"))
	svcErr := suite.service.SetConfig(suite.ctx, ConfigNameCORS, incomingRaw)
	assert.Same(suite.T(), &ErrorInvalidConfigValue, svcErr)
	suite.mockStore.AssertNotCalled(suite.T(), "GetServerConfig", mock.Anything, mock.Anything)
	suite.mockStore.AssertNotCalled(suite.T(), "UpsertServerConfig", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestSetConfig_ReadError() {
	suite.mockHandler.EXPECT().Decode(incomingRaw).Return(incomingVal, nil)
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{}, errors.New("db error"))
	svcErr := suite.service.SetConfig(suite.ctx, ConfigNameCORS, incomingRaw)
	assert.Same(suite.T(), &common.InternalServerError, svcErr)
	suite.mockStore.AssertNotCalled(suite.T(), "UpsertServerConfig", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestSetConfig_HandlerRejects() {
	suite.mockHandler.EXPECT().Decode(incomingRaw).Return(incomingVal, nil)
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{ReadOnly: declarative}, nil)
	suite.mockHandler.EXPECT().Decode(declarative).Return(readOnlyVal, nil)
	suite.mockHandler.EXPECT().Decode(json.RawMessage(nil)).Return(nil, nil)
	suite.mockHandler.EXPECT().Validate(incomingVal, readOnlyVal, nil).Return(errors.New("bad value"))
	svcErr := suite.service.SetConfig(suite.ctx, ConfigNameCORS, incomingRaw)
	assert.Same(suite.T(), &ErrorInvalidConfigValue, svcErr)
	suite.mockStore.AssertNotCalled(suite.T(), "UpsertServerConfig", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestSetConfig_OK() {
	suite.mockHandler.EXPECT().Decode(incomingRaw).Return(incomingVal, nil)
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{ReadOnly: declarative}, nil)
	suite.mockHandler.EXPECT().Decode(declarative).Return(readOnlyVal, nil)
	suite.mockHandler.EXPECT().Decode(json.RawMessage(nil)).Return(nil, nil)
	suite.mockHandler.EXPECT().Validate(incomingVal, readOnlyVal, nil).Return(nil)
	suite.mockStore.EXPECT().
		UpsertServerConfig(mock.Anything, ServerConfig{Name: ConfigNameCORS, Value: incomingRaw}).Return(nil)
	svcErr := suite.service.SetConfig(suite.ctx, ConfigNameCORS, incomingRaw)
	assert.Nil(suite.T(), svcErr)
}

// --- writable composition ---

// composingHandler adds the optional writable composition to the suite's handler mock, recording the
// layers it was handed.
type composingHandler struct {
	*ServerConfigHandlerInterfaceMock
	readOnly, existing, incoming any
}

func (h *composingHandler) ComposeWritable(readOnly, existing, incoming any) any {
	h.readOnly, h.existing, h.incoming = readOnly, existing, incoming
	return mergedVal
}

// TestSetConfig_RealCORSHandlerSkipsDeclarativeOrigin covers the PUT path (and a replacing import): an
// incoming document may list origins the declarative layer already allows, and storing them in the
// writable layer would leave them allowed after the declarative layer stops declaring them.
func (suite *ServiceTestSuite) TestSetConfig_RealCORSHandlerSkipsDeclarativeOrigin() {
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{ReadOnly: json.RawMessage(`{"allowedOrigins":["https://declared.example.com"]}`)}, nil)
	var stored json.RawMessage
	suite.mockStore.EXPECT().UpsertServerConfig(mock.Anything, mock.Anything).
		Run(func(_ context.Context, cfg ServerConfig) { stored = cfg.Value }).Return(nil)

	svc := newServerConfigService(suite.mockStore,
		map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: cors.OriginHandler{}})
	svcErr := svc.SetConfig(suite.ctx, ConfigNameCORS,
		json.RawMessage(`{"allowedOrigins":["https://declared.example.com","https://db.example.com"]}`), false)

	require.Nil(suite.T(), svcErr)
	assert.JSONEq(suite.T(), `{"allowedOrigins":["https://db.example.com"]}`, string(stored))
}

// TestSetConfig_RealCORSHandlerReplacesWritable pins the default path as replacing: an origin already in
// the writable layer and absent from the incoming document is dropped, the same as the PUT API.
func (suite *ServiceTestSuite) TestSetConfig_RealCORSHandlerReplacesWritable() {
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{Writable: json.RawMessage(`{"allowedOrigins":["https://a.example.com"]}`)}, nil)
	var stored json.RawMessage
	suite.mockStore.EXPECT().UpsertServerConfig(mock.Anything, mock.Anything).
		Run(func(_ context.Context, cfg ServerConfig) { stored = cfg.Value }).Return(nil)

	svc := newServerConfigService(suite.mockStore,
		map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: cors.OriginHandler{}})
	svcErr := svc.SetConfig(suite.ctx, ConfigNameCORS,
		json.RawMessage(`{"allowedOrigins":["https://b.example.com"]}`), false)

	require.Nil(suite.T(), svcErr)
	assert.JSONEq(suite.T(), `{"allowedOrigins":["https://b.example.com"]}`, string(stored))
}

func (suite *ServiceTestSuite) TestSetConfig_MergeTrue_PersistsHandlerComposition() {
	handler := &composingHandler{ServerConfigHandlerInterfaceMock: suite.mockHandler}
	service := newServerConfigService(suite.mockStore,
		map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: handler})
	suite.mockHandler.EXPECT().Decode(incomingRaw).Return(incomingVal, nil)
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{ReadOnly: declarative, Writable: corsValue}, nil)
	suite.mockHandler.EXPECT().Decode(declarative).Return(readOnlyVal, nil)
	suite.mockHandler.EXPECT().Decode(corsValue).Return(writableVal, nil)
	suite.mockHandler.EXPECT().Validate(mergedVal, readOnlyVal, writableVal).Return(nil)
	suite.mockStore.EXPECT().UpsertServerConfig(mock.Anything,
		ServerConfig{Name: ConfigNameCORS, Value: json.RawMessage(`"` + mergedVal + `"`)}).Return(nil)
	svcErr := service.SetConfig(suite.ctx, ConfigNameCORS, incomingRaw, true)
	assert.Nil(suite.T(), svcErr)
	assert.Equal(suite.T(), readOnlyVal, handler.readOnly)
	assert.Equal(suite.T(), writableVal, handler.existing)
	assert.Equal(suite.T(), incomingVal, handler.incoming)
}

// TestSetConfig_MergeTrue_RealCORSHandlerIsAdditive wires the real CORS handler through the service. The
// composition is duck-typed, so a change to either side of the contract would otherwise revert a merging
// import to replacing the writable layer without a compile error; only this test fails.
func (suite *ServiceTestSuite) TestSetConfig_MergeTrue_RealCORSHandlerIsAdditive() {
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{Writable: json.RawMessage(`{"allowedOrigins":["https://a.example.com"]}`)}, nil)
	var stored json.RawMessage
	suite.mockStore.EXPECT().UpsertServerConfig(mock.Anything, mock.Anything).
		Run(func(_ context.Context, cfg ServerConfig) { stored = cfg.Value }).Return(nil)

	svc := newServerConfigService(suite.mockStore,
		map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: cors.OriginHandler{}})
	svcErr := svc.SetConfig(suite.ctx, ConfigNameCORS,
		json.RawMessage(`{"allowedOrigins":["https://b.example.com"]}`), true)

	require.Nil(suite.T(), svcErr)
	assert.JSONEq(suite.T(), `{"allowedOrigins":["https://a.example.com","https://b.example.com"]}`, string(stored))
}

// A handler that does not compose its writable value stores the incoming value regardless of merge.
func (suite *ServiceTestSuite) TestSetConfig_MergeTrue_HandlerWithoutComposition() {
	suite.mockHandler.EXPECT().Decode(incomingRaw).Return(incomingVal, nil)
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).Return(storeLayers{}, nil)
	suite.mockHandler.EXPECT().Decode(json.RawMessage(nil)).Return(nil, nil)
	suite.mockHandler.EXPECT().Validate(incomingVal, nil, nil).Return(nil)
	suite.mockStore.EXPECT().UpsertServerConfig(mock.Anything,
		ServerConfig{Name: ConfigNameCORS, Value: incomingRaw}).Return(nil)
	svcErr := suite.service.SetConfig(suite.ctx, ConfigNameCORS, incomingRaw, true)
	assert.Nil(suite.T(), svcErr)
}

func (suite *ServiceTestSuite) TestSetConfig_UpsertError() {
	suite.mockHandler.EXPECT().Decode(incomingRaw).Return(incomingVal, nil)
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).Return(storeLayers{}, nil)
	suite.mockHandler.EXPECT().Decode(json.RawMessage(nil)).Return(nil, nil)
	suite.mockHandler.EXPECT().Validate(incomingVal, nil, nil).Return(nil)
	suite.mockStore.EXPECT().UpsertServerConfig(mock.Anything, mock.Anything).Return(errors.New("db error"))
	svcErr := suite.service.SetConfig(suite.ctx, ConfigNameCORS, incomingRaw)
	assert.Same(suite.T(), &common.InternalServerError, svcErr)
}
