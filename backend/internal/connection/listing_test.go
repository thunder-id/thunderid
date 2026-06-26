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

package connection

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/idp"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
)

type ListingTestSuite struct {
	suite.Suite
	handler *handler
	mockIDP *idpmock.IDPServiceInterfaceMock
}

func TestListingSuite(t *testing.T) {
	suite.Run(t, new(ListingTestSuite))
}

func (s *ListingTestSuite) SetupTest() {
	s.handler, s.mockIDP = newConnectionTestHandler(s.T())
}

func (s *ListingTestSuite) TestCounts() {
	s.mockIDP.On("GetIdentityProviderList", mock.Anything).Return([]idp.BasicIDPDTO{
		{ID: "1", Type: providers.IDPTypeGoogle},
		{ID: "2", Type: providers.IDPTypeGoogle},
		{ID: "3", Type: providers.IDPTypeOIDC},
	}, (*tidcommon.ServiceError)(nil))

	req := httptest.NewRequest(http.MethodGet, "/connections", nil)
	rr := httptest.NewRecorder()
	s.handler.handleListConnections(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	var resp connectionListResponse
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))

	byType := make(map[string]connectionTypeSummary, len(resp.Connections))
	for _, c := range resp.Connections {
		byType[c.Type] = c
	}
	s.Len(resp.Connections, len(idpBackedVendors))
	s.Equal(2, byType["google"].InstanceCount)
	s.True(byType["google"].Configured)
	s.Equal(1, byType["oidc"].InstanceCount)
	s.Equal(0, byType["github"].InstanceCount)
	s.False(byType["github"].Configured)
}

func (s *ListingTestSuite) TestServiceError() {
	s.mockIDP.On("GetIdentityProviderList", mock.Anything).
		Return(([]idp.BasicIDPDTO)(nil), &tidcommon.InternalServerError)

	req := httptest.NewRequest(http.MethodGet, "/connections", nil)
	rr := httptest.NewRecorder()
	s.handler.handleListConnections(rr, req)

	s.Equal(http.StatusInternalServerError, rr.Code)
}
