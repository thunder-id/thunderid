// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// ResolveBoundResourceServer determines the single resource server a request binds to. An explicit
// identifier resolves by identifier only, with no fallback. Without an identifier, the requested
// permissions are used to derive the resource server; when they do not resolve one, the configured
// default resource server is used. Ambiguous permissions and server errors are returned as is and
// never fall back to the default.
func ResolveBoundResourceServer(
	ctx context.Context,
	provider providers.ResourceServerProvider,
	identifier string,
	permissions []string,
) (*providers.ResourceServer, *tidcommon.ServiceError) {
	if identifier != "" {
		return provider.GetResourceServerByIdentifier(ctx, identifier)
	}
	if len(permissions) > 0 {
		rs, svcErr := provider.GetResourceServerByPermissions(ctx, "", permissions)
		if svcErr == nil && rs != nil {
			return rs, nil
		}
		if svcErr != nil && (svcErr.Type == tidcommon.ServerErrorType ||
			svcErr.Code == tidcommon.ErrorAmbiguousPermissions.Code) {
			return nil, svcErr
		}
	}
	return provider.GetResourceServerByIdentifier(ctx, "")
}
