// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"github.com/thunder-id/thunderid/internal/system/utils"
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
