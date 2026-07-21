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

package resource

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type DefaultResourceServerConfigHandlerTestSuite struct {
	suite.Suite
}

func TestDefaultResourceServerConfigHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(DefaultResourceServerConfigHandlerTestSuite))
}

func (suite *DefaultResourceServerConfigHandlerTestSuite) marshal(v any) string {
	out, err := json.Marshal(v)
	suite.Require().NoError(err)
	return string(out)
}

func (suite *DefaultResourceServerConfigHandlerTestSuite) handler() *DefaultResourceServerConfigHandler {
	return NewDefaultResourceServerConfigHandler(NewResourceServiceInterfaceMock(suite.T()))
}

func (suite *DefaultResourceServerConfigHandlerTestSuite) TestDecodeEmptyYieldsEmptyConfig() {
	decoded, err := suite.handler().Decode(json.RawMessage(nil))
	suite.Require().NoError(err)
	assert.JSONEq(suite.T(), `{"resourceServerId":""}`, suite.marshal(decoded))
}

func (suite *DefaultResourceServerConfigHandlerTestSuite) TestDecodeValidObject() {
	decoded, err := suite.handler().Decode(json.RawMessage(`{"resourceServerId":"abc"}`))
	suite.Require().NoError(err)
	assert.Equal(suite.T(), DefaultResourceServerConfig{ResourceServerID: "abc"}, decoded)
}

func (suite *DefaultResourceServerConfigHandlerTestSuite) TestDecodeMalformedJSON() {
	_, err := suite.handler().Decode(json.RawMessage(`{not json`))
	assert.Error(suite.T(), err)
}

func (suite *DefaultResourceServerConfigHandlerTestSuite) TestValidateUnsetAccepted() {
	h := NewDefaultResourceServerConfigHandler(NewResourceServiceInterfaceMock(suite.T()))
	assert.NoError(suite.T(), h.Validate(DefaultResourceServerConfig{}, nil, nil))
}

func (suite *DefaultResourceServerConfigHandlerTestSuite) TestValidateKnownIDAccepted() {
	mockSvc := NewResourceServiceInterfaceMock(suite.T())
	mockSvc.EXPECT().GetResourceServer(mock.Anything, "rs-1").Return(&providers.ResourceServer{ID: "rs-1"}, nil)
	h := NewDefaultResourceServerConfigHandler(mockSvc)
	assert.NoError(suite.T(), h.Validate(DefaultResourceServerConfig{ResourceServerID: "rs-1"}, nil, nil))
}

func (suite *DefaultResourceServerConfigHandlerTestSuite) TestValidateUnknownIDRejected() {
	mockSvc := NewResourceServiceInterfaceMock(suite.T())
	mockSvc.EXPECT().GetResourceServer(mock.Anything, "missing").Return(nil, &ErrorResourceServerNotFound)
	h := NewDefaultResourceServerConfigHandler(mockSvc)
	assert.Error(suite.T(), h.Validate(DefaultResourceServerConfig{ResourceServerID: "missing"}, nil, nil))
}

func (suite *DefaultResourceServerConfigHandlerTestSuite) TestValidateInternalErrorRejected() {
	mockSvc := NewResourceServiceInterfaceMock(suite.T())
	mockSvc.EXPECT().GetResourceServer(mock.Anything, "rs-1").Return(nil, &tidcommon.InternalServerError)
	h := NewDefaultResourceServerConfigHandler(mockSvc)
	err := h.Validate(DefaultResourceServerConfig{ResourceServerID: "rs-1"}, nil, nil)
	assert.ErrorIs(suite.T(), err, errDefaultResourceServerLookupFailed)
}

func (suite *DefaultResourceServerConfigHandlerTestSuite) TestValidateRejectsWriteWhenDeclarativeSet() {
	h := NewDefaultResourceServerConfigHandler(NewResourceServiceInterfaceMock(suite.T()))
	err := h.Validate(
		DefaultResourceServerConfig{ResourceServerID: "rs-2"},
		DefaultResourceServerConfig{ResourceServerID: "rs-1"},
		DefaultResourceServerConfig{},
	)
	assert.Error(suite.T(), err)
}

func (suite *DefaultResourceServerConfigHandlerTestSuite) TestValidateRejectsClearWhenDeclarativeSet() {
	h := NewDefaultResourceServerConfigHandler(NewResourceServiceInterfaceMock(suite.T()))
	err := h.Validate(
		DefaultResourceServerConfig{},
		DefaultResourceServerConfig{ResourceServerID: "rs-1"},
		DefaultResourceServerConfig{},
	)
	assert.Error(suite.T(), err)
}

func (suite *DefaultResourceServerConfigHandlerTestSuite) TestMergeReadOnlyWins() {
	merged := suite.handler().
		Merge(DefaultResourceServerConfig{ResourceServerID: "ro"}, DefaultResourceServerConfig{ResourceServerID: "w"})
	assert.Equal(suite.T(), DefaultResourceServerConfig{ResourceServerID: "ro"}, merged)
}

func (suite *DefaultResourceServerConfigHandlerTestSuite) TestMergeFallsBackToWritable() {
	merged := suite.handler().
		Merge(DefaultResourceServerConfig{}, DefaultResourceServerConfig{ResourceServerID: "w"})
	assert.Equal(suite.T(), DefaultResourceServerConfig{ResourceServerID: "w"}, merged)
}

func (suite *DefaultResourceServerConfigHandlerTestSuite) TestMergeBothEmpty() {
	merged := suite.handler().
		Merge(DefaultResourceServerConfig{}, DefaultResourceServerConfig{})
	assert.Equal(suite.T(), DefaultResourceServerConfig{}, merged)
}
