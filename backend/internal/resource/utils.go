// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// emptyResourceServerList answers a list the caller may see nothing of.
func emptyResourceServerList(limit, offset int, extraQuery string) *ResourceServerList {
	return &ResourceServerList{
		ResourceServers: []providers.ResourceServer{},
		StartIndex:      offset + 1,
		Links:           utils.BuildPaginationLinks("/resource-servers", limit, offset, 0, extraQuery),
	}
}

// GetDeclaredPermissions lists covered resource permissions and resource-owned actions in one batch.
func (rs *resourceService) GetDeclaredPermissions(
	ctx context.Context, resourceServerID string, roots []string,
) ([]string, *tidcommon.ServiceError) {
	if len(roots) == 0 {
		return nil, nil
	}
	server, svcErr := rs.validateAndGetResourceServer(ctx, resourceServerID, security.ActionReadResourceServer)
	if svcErr != nil {
		return nil, svcErr
	}
	delimiter := ""
	if server.HierarchicalPermissions {
		delimiter = server.Delimiter
	}
	permissions, err := rs.resourceStore.GetDeclaredPermissions(ctx, resourceServerID, roots, delimiter)
	if err != nil {
		rs.logger.Error(ctx, "Failed to list declared permissions", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return permissions, nil
}
