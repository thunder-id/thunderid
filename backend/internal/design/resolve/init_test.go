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

package resolve

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/design/common"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/tests/mocks/applicationmock"
	"github.com/thunder-id/thunderid/tests/mocks/design/layoutmock"
	"github.com/thunder-id/thunderid/tests/mocks/design/thememock"
)

// Test Suite
type InitTestSuite struct {
	suite.Suite
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

// Test Initialize returns a non-nil service
func (suite *InitTestSuite) TestInitialize() {
	mux := http.NewServeMux()
	mockTheme := thememock.NewThemeMgtServiceInterfaceMock(suite.T())
	mockLayout := layoutmock.NewLayoutMgtServiceInterfaceMock(suite.T())
	mockApp := applicationmock.NewApplicationServiceInterfaceMock(suite.T())

	service := Initialize(mux, mockTheme, mockLayout, mockApp)

	assert.NotNil(suite.T(), service)
	assert.Implements(suite.T(), (*DesignResolveServiceInterface)(nil), service)
}

// Test registerRoutes does not panic
func (suite *InitTestSuite) TestRegisterRoutes() {
	mux := http.NewServeMux()
	mockService := &mockDesignResolveService{
		resolveDesignFn: func(
			ctx context.Context,
			resolveType common.DesignResolveType,
			id string,
		) (*common.DesignResponse, *serviceerror.ServiceError) {
			return nil, nil
		},
	}

	handler := newDesignResolveHandler(mockService)

	// Verify registerRoutes does not panic
	assert.NotPanics(suite.T(), func() {
		registerRoutes(mux, handler)
	})
}
