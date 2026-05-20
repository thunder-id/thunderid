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

package layoutmgt

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

// Test Suite
type LayoutInitTestSuite struct {
	suite.Suite
}

func TestLayoutInitTestSuite(t *testing.T) {
	suite.Run(t, new(LayoutInitTestSuite))
}

// Test registerRoutes does not panic
func (suite *LayoutInitTestSuite) TestRegisterRoutes() {
	mux := http.NewServeMux()
	mockSvc := &mockLayoutService{
		getLayoutListFunc: func(limit, offset int) (*LayoutList, *serviceerror.ServiceError) {
			return &LayoutList{Layouts: []Layout{}, Links: []Link{}}, nil
		},
		createLayoutFunc: func(layout CreateLayoutRequest) (*Layout, *serviceerror.ServiceError) {
			return &Layout{}, nil
		},
		getLayoutFunc: func(id string) (*Layout, *serviceerror.ServiceError) {
			return &Layout{}, nil
		},
		updateLayoutFunc: func(id string, layout UpdateLayoutRequest) (*Layout, *serviceerror.ServiceError) {
			return &Layout{}, nil
		},
		deleteLayoutFunc: func(id string) *serviceerror.ServiceError {
			return nil
		},
		isLayoutExistFunc: func(id string) (bool, *serviceerror.ServiceError) {
			return false, nil
		},
	}

	handler := newLayoutMgtHandler(mockSvc)

	// Verify registerRoutes does not panic
	assert.NotPanics(suite.T(), func() {
		registerRoutes(mux, handler)
	})
}

func (suite *LayoutInitTestSuite) TestInitializeStore_CompositeMode() {
	// Initialize runtime with temp home
	tempDir := suite.T().TempDir()
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime(tempDir, &config.Config{})
	suite.Require().NoError(err)

	runtime := config.GetServerRuntime()
	runtime.Config.Layout.Store = "composite"

	store, err := initializeStore()

	suite.NoError(err)
	_, ok := store.(*compositeLayoutStore)
	suite.True(ok)
}

func (suite *LayoutInitTestSuite) TestInitializeStore_DeclarativeMode() {
	// Initialize runtime with temp home
	tempDir := suite.T().TempDir()
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime(tempDir, &config.Config{})
	suite.Require().NoError(err)

	runtime := config.GetServerRuntime()
	runtime.Config.Layout.Store = "declarative"

	store, err := initializeStore()

	suite.NoError(err)
	_, ok := store.(*layoutFileBasedStore)
	suite.True(ok)
}
