// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"

	resourcepkg "github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// permissionDelimiterResolver resolves the resource server's current matching mode.
type permissionDelimiterResolver struct {
	resourceService resourcepkg.ResourceServiceInterface
}

// newPermissionDelimiterResolver creates a resolver over the resource service.
func newPermissionDelimiterResolver(
	resourceService resourcepkg.ResourceServiceInterface,
) *permissionDelimiterResolver {
	return &permissionDelimiterResolver{resourceService: resourceService}
}

// resolve uses defaultDelimiter only for servers with hierarchy disabled.
// Missing or unreadable metadata preserves exact matching.
func (c *permissionDelimiterResolver) resolve(ctx context.Context, resourceServerID, defaultDelimiter string) string {
	if c == nil || c.resourceService == nil || resourceServerID == "" {
		return ""
	}
	delimiter := ""
	resourceServer, svcErr := c.resourceService.GetResourceServer(security.WithRuntimeContext(ctx), resourceServerID)
	if svcErr != nil || resourceServer == nil {
		log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)).Debug(ctx,
			"Could not resolve the resource server delimiter; matching permissions exactly",
			log.String("resourceServerID", resourceServerID))
	} else if resourceServer.HierarchicalPermissions {
		delimiter = resourceServer.Delimiter
	} else {
		delimiter = defaultDelimiter
	}
	return delimiter
}

// resolveAll returns administrative delimiters, using legacy colon matching for disabled servers.
func (c *permissionDelimiterResolver) resolveAll(
	ctx context.Context, resourceServerIDs []string,
) map[string]string {
	if len(resourceServerIDs) == 0 {
		return nil
	}
	delimiters := make(map[string]string, len(resourceServerIDs))
	for _, resourceServerID := range resourceServerIDs {
		if delimiter := c.resolve(ctx, resourceServerID, security.DefaultPermissionDelimiter); delimiter != "" {
			delimiters[resourceServerID] = delimiter
		}
	}
	return delimiters
}
