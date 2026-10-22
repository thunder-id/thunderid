// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package deviceauth

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/discovery"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/discoverymock"
)

func TestRegisterRoutesRegistersDeviceAuthorizationEndpoints(t *testing.T) {
	discoveryService := discoverymock.NewDiscoveryServiceInterfaceMock(t)
	discoveryService.EXPECT().GetOAuth2AuthorizationServerMetadata(mock.Anything).
		Return(&discovery.OAuth2AuthorizationServerMetadata{Issuer: "https://id.example.com"})

	mux := http.NewServeMux()
	registerRoutes(mux, NewDeviceAuthHandlerInterfaceMock(t), nil, nil, nil, discoveryService, nil,
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
		if err != nil {
			t.Fatal(err)
		}
		_, pattern := mux.Handler(request)
		if pattern == "" {
			t.Fatalf("%s %s route was not registered", tc.method, tc.path)
		}
	}
}
