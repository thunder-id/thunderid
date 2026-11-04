// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package deviceauth

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/discovery"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/discoverymock"
)

type InitTestSuite struct {
	suite.Suite
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (s *InitTestSuite) TestRegistersDeviceAuthorizationEndpoints() {
	discoveryService := discoverymock.NewDiscoveryServiceInterfaceMock(s.T())
	discoveryService.EXPECT().GetOAuth2AuthorizationServerMetadata(mock.Anything).
		Return(&discovery.OAuth2AuthorizationServerMetadata{Issuer: "https://id.example.com"})

	mux := http.NewServeMux()
	registerRoutes(mux, NewDeviceAuthHandlerInterfaceMock(s.T()), nil, nil, nil, discoveryService, nil, nil,
		engineconfig.ClientAssertionConfig{}, 0)

	for _, tc := range []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/oauth2/device-authorize"},
		{method: http.MethodOptions, path: "/oauth2/device-authorize"},
	} {
		request, err := http.NewRequestWithContext(
			context.Background(), tc.method, "https://id.example.com"+tc.path, nil)
		s.Require().NoError(err)
		_, pattern := mux.Handler(request)
		s.NotEmpty(pattern, "%s %s route was not registered", tc.method, tc.path)
	}
}
