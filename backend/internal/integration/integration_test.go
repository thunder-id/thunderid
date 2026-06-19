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

// Package integration provides tests for the integrations catalog endpoint.
package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type IntegrationTestSuite struct {
	suite.Suite
	integrations []Descriptor
	service      ServiceInterface
	handler      *handler
}

func TestIntegrationSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

func (suite *IntegrationTestSuite) SetupTest() {
	suite.integrations = []Descriptor{
		{
			Type:        "GOOGLE",
			DisplayName: "Google",
			Category:    CategorySocialLogin,
			Fields: []Field{
				{Name: "client_id", Required: true},
				{Name: "client_secret", Required: true, Secret: true},
			},
		},
		{
			Type:        "twilio",
			DisplayName: "Twilio",
			Category:    CategorySMS,
			Fields:      []Field{{Name: "account_sid", Required: true}},
		},
	}
	suite.service = newService(suite.integrations)
	suite.handler = newHandler(suite.service)
}

func (suite *IntegrationTestSuite) TestGetIntegrationsReturnsAll() {
	catalog := suite.service.GetIntegrations(context.Background())

	assert.Len(suite.T(), catalog.Integrations, 2)
	assert.Equal(suite.T(), "GOOGLE", catalog.Integrations[0].Type)
	assert.Equal(suite.T(), CategorySMS, catalog.Integrations[1].Category)
}

func (suite *IntegrationTestSuite) TestHandleIntegrationsRequest() {
	req := httptest.NewRequest(http.MethodGet, "/integrations", nil)
	w := httptest.NewRecorder()

	suite.handler.HandleIntegrationsRequest(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var catalog ListResponse
	err := json.NewDecoder(w.Body).Decode(&catalog)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), catalog.Integrations, 2)

	google := catalog.Integrations[0]
	assert.Equal(suite.T(), "Google", google.DisplayName)
	assert.Equal(suite.T(), CategorySocialLogin, google.Category)
	assert.True(suite.T(), google.Fields[1].Secret)
}

func (suite *IntegrationTestSuite) TestHandleIntegrationsRequestEmpty() {
	svc := newService(nil)
	h := newHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/integrations", nil)
	w := httptest.NewRecorder()

	h.HandleIntegrationsRequest(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)

	var catalog ListResponse
	err := json.NewDecoder(w.Body).Decode(&catalog)
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), catalog.Integrations)
}
