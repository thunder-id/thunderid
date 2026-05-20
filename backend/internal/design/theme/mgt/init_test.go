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

package thememgt

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

// Test Suite
type ThemeInitTestSuite struct {
	suite.Suite
}

func TestThemeInitTestSuite(t *testing.T) {
	suite.Run(t, new(ThemeInitTestSuite))
}

// Test registerRoutes does not panic
func (suite *ThemeInitTestSuite) TestRegisterRoutes() {
	mux := http.NewServeMux()
	mockSvc := &mockThemeService{
		getThemeListFunc: func(limit, offset int) (*ThemeList, *serviceerror.ServiceError) {
			return &ThemeList{Themes: []Theme{}, Links: []Link{}}, nil
		},
		createThemeFunc: func(theme CreateThemeRequestWithID) (*Theme, *serviceerror.ServiceError) {
			return &Theme{}, nil
		},
		getThemeFunc: func(id string) (*Theme, *serviceerror.ServiceError) {
			return &Theme{}, nil
		},
		updateThemeFunc: func(id string, theme UpdateThemeRequest) (*Theme, *serviceerror.ServiceError) {
			return &Theme{}, nil
		},
		deleteThemeFunc: func(id string) *serviceerror.ServiceError {
			return nil
		},
		isThemeExistFunc: func(id string) (bool, *serviceerror.ServiceError) {
			return false, nil
		},
	}

	handler := newThemeMgtHandler(mockSvc)

	// Verify registerRoutes does not panic
	assert.NotPanics(suite.T(), func() {
		registerRoutes(mux, handler)
	})
}

func (suite *ThemeInitTestSuite) TestInitializeStore_CompositeMode() {
	// Initialize runtime with temp home
	tempDir := suite.T().TempDir()
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime(tempDir, &config.Config{})
	suite.Require().NoError(err)

	runtime := config.GetServerRuntime()
	runtime.Config.Theme.Store = "composite"

	store, err := initializeStore()

	suite.NoError(err)
	_, ok := store.(*compositeThemeStore)
	suite.True(ok)
}

func (suite *ThemeInitTestSuite) TestInitializeStore_DeclarativeMode() {
	// Initialize runtime with temp home
	tempDir := suite.T().TempDir()
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime(tempDir, &config.Config{})
	suite.Require().NoError(err)

	runtime := config.GetServerRuntime()
	runtime.Config.Theme.Store = "declarative"

	store, err := initializeStore()

	suite.NoError(err)
	_, ok := store.(*themeFileBasedStore)
	suite.True(ok)
}
