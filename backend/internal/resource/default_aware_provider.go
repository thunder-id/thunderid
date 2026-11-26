// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"

	"github.com/thunder-id/thunderid/internal/serverconfig"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// defaultAwareResourceServerProvider decorates a providers.ResourceServerProvider so that an empty
// identifier resolves the deployment's configured default resource server. The default resource server
// is a server-side policy: the authentication engine and OAuth layers depend only on
// providers.ResourceServerProvider and never see the server-config store.
type defaultAwareResourceServerProvider struct {
	// The resource service, which also tells whether an organization unit may use a resource server.
	ResourceServiceInterface
	serverConfigService serverconfig.ServerConfigService
}

var _ providers.ResourceServerProvider = (*defaultAwareResourceServerProvider)(nil)

// NewDefaultAwareResourceServerProvider wraps base so that GetResourceServerByIdentifier resolves the
// configured default resource server when the identifier is empty, and passes on whether an organization unit may use
// a resource server. base and serverConfigService must both be non-nil.
func NewDefaultAwareResourceServerProvider(
	base ResourceServiceInterface,
	serverConfigService serverconfig.ServerConfigService,
) providers.ResourceServerProvider {
	if base == nil {
		panic("default-aware resource server provider requires a non-nil base provider")
	}
	if serverConfigService == nil {
		panic("default-aware resource server provider requires a non-nil server config service")
	}
	return &defaultAwareResourceServerProvider{
		ResourceServiceInterface: base,
		serverConfigService:      serverConfigService,
	}
}

// GetResourceServerByIdentifier resolves an explicit identifier through the wrapped provider. When the
// identifier is empty it resolves the deployment's configured default resource server: a client error
// when no default is configured (or the merged config is malformed), and a server error when the
// configuration cannot be read.
func (p *defaultAwareResourceServerProvider) GetResourceServerByIdentifier(
	ctx context.Context, identifier string,
) (*providers.ResourceServer, *tidcommon.ServiceError) {
	if identifier != "" {
		return p.ResourceServiceInterface.GetResourceServerByIdentifier(ctx, identifier)
	}
	merged, svcErr := p.serverConfigService.GetMergedConfig(
		ctx, string(serverconfig.ConfigNameDefaultResourceServer))
	if svcErr != nil {
		return nil, svcErr
	}
	cfg, _ := merged.(DefaultResourceServerConfig)
	if cfg.ResourceServerID == "" {
		return nil, &ErrorResourceServerNotFound
	}
	return p.ResourceServiceInterface.GetResourceServer(ctx, cfg.ResourceServerID)
}
