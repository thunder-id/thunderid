// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package layoutmgt

import (
	"net/http"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
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
		getLayoutListFunc: func(limit, offset int) (*LayoutList, *tidcommon.ServiceError) {
			return &LayoutList{Layouts: []Layout{}, Links: []Link{}}, nil
		},
		createLayoutFunc: func(layout CreateLayoutRequestWithID) (*Layout, *tidcommon.ServiceError) {
			return &Layout{}, nil
		},
		getLayoutFunc: func(id string) (*Layout, *tidcommon.ServiceError) {
			return &Layout{}, nil
		},
		updateLayoutFunc: func(id string, layout UpdateLayoutRequest) (*Layout, *tidcommon.ServiceError) {
			return &Layout{}, nil
		},
		deleteLayoutFunc: func(id string) *tidcommon.ServiceError {
			return nil
		},
		isLayoutExistFunc: func(id string) (bool, *tidcommon.ServiceError) {
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

func (suite *LayoutInitTestSuite) TestInitialize_RefusesANilEventBus() {
	service, exporter, err := Initialize(http.NewServeMux(), nil)

	suite.ErrorIs(err, errNilEventBus)
	suite.Nil(service)
	suite.Nil(exporter)
}
