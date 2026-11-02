// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
)

type InitTestSuite struct {
	suite.Suite
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (s *InitTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

// TestSelectStoreDeclarative verifies declarative mode builds a read-only file store seeded from the
// declarations on disk. This mode needs no database, so selectStore is exercised without a DB
// provider. The read cache is unused in this mode, so a nil cache is passed.
func (s *InitTestSuite) TestSelectStoreDeclarative() {
	serverHome := s.T().TempDir()
	dir := filepath.Join(serverHome, "config", "resources", "notification_templates")
	s.Require().NoError(os.MkdirAll(dir, 0750))
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "welcome.yaml"), []byte(declaredForDBCheck), 0600))

	config.ResetServerRuntime()
	cfg := &config.Config{}
	cfg.Notification.Template.Store = "declarative"
	s.Require().NoError(config.InitializeServerRuntime(serverHome, cfg))

	store, err := selectStore(nil)
	s.Require().NoError(err)

	got, err := store.GetTemplateByHandle(context.Background(), ChannelTypeEmail, "welcome")
	s.Require().NoError(err)
	s.Require().Equal("welcome-email", got.ID)

	// The declarative store is read-only.
	s.Require().ErrorIs(store.CreateTemplate(context.Background(), got), errDeclarativeTemplate)
}

// TestRegisterRoutesWiresEndpoints verifies registerRoutes registers a handler for each
// collection and item route (and their OPTIONS preflight) on the mux.
func (s *InitTestSuite) TestRegisterRoutesWiresEndpoints() {
	mux := http.NewServeMux()
	// Route matching does not invoke the service, so a nil-service handler is sufficient here.
	registerRoutes(mux, newNotificationTemplateHandler(nil))

	routes := []struct{ method, target string }{
		{http.MethodGet, "/notification-templates/email/templates"},
		{http.MethodPost, "/notification-templates/email/templates"},
		{http.MethodOptions, "/notification-templates/email/templates"},
		{http.MethodGet, "/notification-templates/email/templates/abc"},
		{http.MethodPut, "/notification-templates/email/templates/abc"},
		{http.MethodDelete, "/notification-templates/email/templates/abc"},
		{http.MethodOptions, "/notification-templates/email/templates/abc"},
	}
	for _, r := range routes {
		req := httptest.NewRequest(r.method, r.target, nil)
		_, pattern := mux.Handler(req)
		s.Require().NotEmpty(pattern, "%s %s should be routed", r.method, r.target)
	}

	// An OPTIONS preflight is handled inline and returns 204 without touching the service.
	req := httptest.NewRequest(http.MethodOptions, "/notification-templates/email/templates", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	s.Require().Equal(http.StatusNoContent, rec.Code)
}
