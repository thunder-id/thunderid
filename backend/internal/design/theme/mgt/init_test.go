// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package thememgt

import (
	"net/http"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
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
		getThemeListFunc: func(limit, offset int) (*ThemeList, *tidcommon.ServiceError) {
			return &ThemeList{Themes: []Theme{}, Links: []Link{}}, nil
		},
		createThemeFunc: func(theme CreateThemeRequestWithID) (*Theme, *tidcommon.ServiceError) {
			return &Theme{}, nil
		},
		getThemeFunc: func(id string) (*Theme, *tidcommon.ServiceError) {
			return &Theme{}, nil
		},
		updateThemeFunc: func(id string, theme UpdateThemeRequest) (*Theme, *tidcommon.ServiceError) {
			return &Theme{}, nil
		},
		deleteThemeFunc: func(id string) *tidcommon.ServiceError {
			return nil
		},
		isThemeExistFunc: func(id string) (bool, *tidcommon.ServiceError) {
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

func (suite *ThemeInitTestSuite) TestInitialize_RefusesANilEventBus() {
	service, exporter, err := Initialize(http.NewServeMux(), nil, nil)

	suite.ErrorIs(err, errNilEventBus)
	suite.Nil(service)
	suite.Nil(exporter)
}
