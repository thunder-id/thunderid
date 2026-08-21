// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package presentation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
)

func setupDefinitionConfig(t *testing.T, store string, declarativeEnabled bool) {
	t.Helper()
	config.ResetServerRuntime()
	t.Cleanup(config.ResetServerRuntime)
	require.NoError(t, config.InitializeServerRuntime("", &config.Config{
		OpenID4VP:            config.OpenID4VPConfig{Store: store},
		DeclarativeResources: config.DeclarativeResources{Enabled: declarativeEnabled},
	}))
}

func TestGetDefinitionStoreMode_ExplicitMutable(t *testing.T) {
	setupDefinitionConfig(t, "mutable", false)
	mode, err := getDefinitionStoreMode()
	require.NoError(t, err)
	assert.Equal(t, serverconst.StoreModeMutable, mode)
}

func TestGetDefinitionStoreMode_ExplicitDeclarative(t *testing.T) {
	setupDefinitionConfig(t, "declarative", false)
	mode, err := getDefinitionStoreMode()
	require.NoError(t, err)
	assert.Equal(t, serverconst.StoreModeDeclarative, mode)
}

func TestGetDefinitionStoreMode_ExplicitComposite(t *testing.T) {
	setupDefinitionConfig(t, "composite", false)
	mode, err := getDefinitionStoreMode()
	require.NoError(t, err)
	assert.Equal(t, serverconst.StoreModeComposite, mode)
}

func TestGetDefinitionStoreMode_CaseInsensitive(t *testing.T) {
	setupDefinitionConfig(t, "  Composite  ", false)
	mode, err := getDefinitionStoreMode()
	require.NoError(t, err)
	assert.Equal(t, serverconst.StoreModeComposite, mode)
}

func TestGetDefinitionStoreMode_InvalidIsError(t *testing.T) {
	setupDefinitionConfig(t, "bogus", true)
	_, err := getDefinitionStoreMode()
	assert.Error(t, err)
}

func TestGetDefinitionStoreMode_FallbackDeclarativeEnabled(t *testing.T) {
	setupDefinitionConfig(t, "", true)
	mode, err := getDefinitionStoreMode()
	require.NoError(t, err)
	assert.Equal(t, serverconst.StoreModeDeclarative, mode)
}

func TestGetDefinitionStoreMode_FallbackDeclarativeDisabled(t *testing.T) {
	setupDefinitionConfig(t, "", false)
	mode, err := getDefinitionStoreMode()
	require.NoError(t, err)
	assert.Equal(t, serverconst.StoreModeMutable, mode)
}

func TestRegisterRoutesRegistersEndpoints(t *testing.T) {
	mux := http.NewServeMux()
	handler := newDefinitionHandler(NewPresentationDefinitionServiceInterfaceMock(t))
	registerRoutes(mux, handler)

	// The OPTIONS preflight handlers respond without invoking the service.
	for _, target := range []string{definitionsPath, definitionsPath + "/some-id"} {
		req := httptest.NewRequest(http.MethodOptions, target, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	}
}

// TestDeclarativeModeRejectsCreate verifies the management API cannot create a definition
// when the store is declarative-only. The write would otherwise land in the in-memory
// declarative store and disappear on restart.
func TestDeclarativeModeRejectsCreate(t *testing.T) {
	setupDefinitionConfig(t, "declarative", false)

	ouSvc := newOUServiceMock(t, map[string]bool{"ou-1": true},
		map[string]string{"root": "ou-1"}, map[string]string{"ou-1": "root"})
	store, err := initializeStore(ouSvc)
	require.NoError(t, err)

	svc := newPresentationDefinitionService(store, ouSvc)
	_, svcErr := svc.CreatePresentationDefinition(context.Background(), &PresentationDefinitionDTO{
		Handle: "decl-mode-create",
		OUID:   "ou-1",
		VCT:    "urn:example:vct",
	})

	require.NotNil(t, svcErr, "a create must be rejected in declarative-only mode")
	assert.Equal(t, ErrorDefinitionDeclarativeModeCreateNotAllowed.Code, svcErr.Code)
}
