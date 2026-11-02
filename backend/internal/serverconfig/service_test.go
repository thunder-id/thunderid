// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/observabilityprovidermock"
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
		map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: suite.mockHandler}, nil)
}

// serviceWithoutHandlers builds a service with no registered handlers, sharing the suite store mock.
func (suite *ServiceTestSuite) serviceWithoutHandlers() ServerConfigService {
	return newServerConfigService(suite.mockStore, map[ConfigName]ServerConfigHandlerInterface{}, nil)
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

// Only the names a handler is registered for are listed, so a server without a section's service
// does not offer a section every read of would fail.
func (suite *ServiceTestSuite) TestListConfigNames() {
	names, svcErr := suite.service.ListConfigNames(suite.ctx)
	assert.Nil(suite.T(), svcErr)
	assert.Equal(suite.T(), []ConfigName{ConfigNameCORS}, names)
}

func (suite *ServiceTestSuite) TestListConfigNames_NoHandlers() {
	names, svcErr := suite.serviceWithoutHandlers().ListConfigNames(suite.ctx)
	assert.Nil(suite.T(), svcErr)
	assert.Empty(suite.T(), names)
}

// A name registered with no handler is not listed either: every read of it would fail the same way.
func (suite *ServiceTestSuite) TestListConfigNames_NilHandler() {
	service := newServerConfigService(suite.mockStore, map[ConfigName]ServerConfigHandlerInterface{
		ConfigNameCORS: nil,
	}, nil)

	names, svcErr := service.ListConfigNames(suite.ctx)

	assert.Nil(suite.T(), svcErr)
	assert.Empty(suite.T(), names)
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

func (suite *ServiceTestSuite) TestSetConfig_UpsertError() {
	suite.mockHandler.EXPECT().Decode(incomingRaw).Return(incomingVal, nil)
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).Return(storeLayers{}, nil)
	suite.mockHandler.EXPECT().Decode(json.RawMessage(nil)).Return(nil, nil)
	suite.mockHandler.EXPECT().Validate(incomingVal, nil, nil).Return(nil)
	suite.mockStore.EXPECT().UpsertServerConfig(mock.Anything, mock.Anything).Return(errors.New("db error"))
	svcErr := suite.service.SetConfig(suite.ctx, ConfigNameCORS, incomingRaw)
	assert.Same(suite.T(), &common.InternalServerError, svcErr)
}

// auditedService builds a service over the suite mocks whose events land in the returned slice.
func (suite *ServiceTestSuite) auditedService() (ServerConfigService, *[]*providers.Event) {
	published := []*providers.Event{}
	obs := observabilityprovidermock.NewObservabilityProviderMock(suite.T())
	obs.EXPECT().IsEnabled().Return(true)
	obs.EXPECT().PublishEvent(mock.Anything, mock.Anything).
		Run(func(_ context.Context, evt *providers.Event) { published = append(published, evt) })
	return newServerConfigService(suite.mockStore,
		map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: suite.mockHandler}, obs), &published
}

// A write is recorded with the section and the outcome, never the value, which may hold secrets.
func (suite *ServiceTestSuite) TestSetConfig_PublishesTheChange() {
	previous := json.RawMessage(`["https://old.example.com"]`)
	suite.mockHandler.EXPECT().Decode(incomingRaw).Return(incomingVal, nil)
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).
		Return(storeLayers{Writable: previous}, nil)
	suite.mockHandler.EXPECT().Decode(json.RawMessage(nil)).Return(nil, nil)
	suite.mockHandler.EXPECT().Decode(previous).Return(readOnlyVal, nil)
	suite.mockHandler.EXPECT().Validate(incomingVal, nil, readOnlyVal).Return(nil)
	suite.mockStore.EXPECT().UpsertServerConfig(mock.Anything, mock.Anything).Return(nil)
	svc, published := suite.auditedService()

	suite.Nil(svc.SetConfig(suite.ctx, ConfigNameCORS, incomingRaw))

	suite.Require().Len(*published, 1)
	evt := (*published)[0]
	suite.Equal(string(event.EventTypeServerConfigUpdated), evt.Type)
	suite.Equal(providers.StatusSuccess, evt.Status)
	suite.Equal(string(ConfigNameCORS), evt.Data[event.DataKey.ConfigName])
	suite.assertCarriesNoValue(evt, previous, incomingRaw)
}

// A rejected write is recorded as an attempt, without the value that was refused.
func (suite *ServiceTestSuite) TestSetConfig_PublishesARejection() {
	suite.mockHandler.EXPECT().Decode(incomingRaw).Return(nil, errors.New("bad shape"))
	svc, published := suite.auditedService()

	suite.NotNil(svc.SetConfig(suite.ctx, ConfigNameCORS, incomingRaw))

	suite.Require().Len(*published, 1)
	evt := (*published)[0]
	suite.Equal(string(event.EventTypeServerConfigUpdateFailed), evt.Type)
	suite.Equal(ErrorInvalidConfigValue.Code, evt.Data[event.DataKey.Error])
	suite.assertCarriesNoValue(evt, incomingRaw)
}

// An unknown section name comes from the request path, so it is refused without an event.
func (suite *ServiceTestSuite) TestSetConfig_UnknownNamePublishesNothing() {
	obs := observabilityprovidermock.NewObservabilityProviderMock(suite.T())
	svc := newServerConfigService(suite.mockStore,
		map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: suite.mockHandler}, obs)

	suite.NotNil(svc.SetConfig(suite.ctx, ConfigName("no-such-section"), incomingRaw))
}

// expectSuccessfulWrite sets up the store and handler for a write that succeeds.
func (suite *ServiceTestSuite) expectSuccessfulWrite() {
	suite.mockHandler.EXPECT().Decode(incomingRaw).Return(incomingVal, nil)
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).Return(storeLayers{}, nil)
	suite.mockHandler.EXPECT().Decode(json.RawMessage(nil)).Return(nil, nil)
	suite.mockHandler.EXPECT().Validate(incomingVal, nil, nil).Return(nil)
	suite.mockStore.EXPECT().UpsertServerConfig(mock.Anything, mock.Anything).Return(nil)
}

// A failed store write is recorded as a failure with the server error code.
func (suite *ServiceTestSuite) TestSetConfig_PublishesAFailedUpsert() {
	suite.mockHandler.EXPECT().Decode(incomingRaw).Return(incomingVal, nil)
	suite.mockStore.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).Return(storeLayers{}, nil)
	suite.mockHandler.EXPECT().Decode(json.RawMessage(nil)).Return(nil, nil)
	suite.mockHandler.EXPECT().Validate(incomingVal, nil, nil).Return(nil)
	suite.mockStore.EXPECT().UpsertServerConfig(mock.Anything, mock.Anything).Return(errors.New("db error"))
	svc, published := suite.auditedService()

	suite.Same(&common.InternalServerError, svc.SetConfig(suite.ctx, ConfigNameCORS, incomingRaw))

	suite.Require().Len(*published, 1)
	evt := (*published)[0]
	suite.Equal(string(event.EventTypeServerConfigUpdateFailed), evt.Type)
	suite.Equal(providers.StatusFailure, evt.Status)
	suite.Equal(common.InternalServerError.Code, evt.Data[event.DataKey.Error])
}

// The writer's subject is recorded when the request has one, and omitted when it has none.
func (suite *ServiceTestSuite) TestSetConfig_PublishesTheActorWhenThereIsOne() {
	cases := []struct {
		name  string
		ctx   context.Context
		actor string
	}{
		{name: "with a subject", ctx: security.WithSecurityContextTest(context.Background(),
			security.NewSecurityContextForTest("admin-1", "", "", nil, nil)), actor: "admin-1"},
		{name: "without a subject", ctx: context.Background()},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			suite.expectSuccessfulWrite()
			svc, published := suite.auditedService()

			suite.Nil(svc.SetConfig(tc.ctx, ConfigNameCORS, incomingRaw))

			suite.Require().Len(*published, 1)
			actor, recorded := (*published)[0].Data[event.DataKey.ActorSub]
			suite.Equal(tc.actor != "", recorded)
			if recorded {
				suite.Equal(tc.actor, actor)
			}
		})
	}
}

// Without an enabled observability provider a write publishes nothing and still succeeds.
func (suite *ServiceTestSuite) TestSetConfig_PublishesNothingWithoutObservability() {
	suite.Run("disabled", func() {
		suite.SetupTest()
		suite.expectSuccessfulWrite()
		obs := observabilityprovidermock.NewObservabilityProviderMock(suite.T())
		obs.EXPECT().IsEnabled().Return(false).Once()
		svc := newServerConfigService(suite.mockStore,
			map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: suite.mockHandler}, obs)

		suite.Nil(svc.SetConfig(suite.ctx, ConfigNameCORS, incomingRaw))
		obs.AssertNotCalled(suite.T(), "PublishEvent", mock.Anything, mock.Anything)
	})
	suite.Run("absent", func() {
		suite.SetupTest()
		suite.expectSuccessfulWrite()

		suite.Nil(suite.service.SetConfig(suite.ctx, ConfigNameCORS, incomingRaw))
	})
}

// assertCarriesNoValue fails if any event field carries a stored or written value.
func (suite *ServiceTestSuite) assertCarriesNoValue(evt *providers.Event, values ...json.RawMessage) {
	for key, field := range evt.Data {
		for _, value := range values {
			suite.NotContains(fmt.Sprint(field), string(value), "field %s carries a config value", key)
		}
	}
}
