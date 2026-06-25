/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package connection

import (
	"context"
	"net/http"

	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// typeCounts returns the number of configured instances per identity-provider type.
func (s *service) typeCounts(ctx context.Context) (map[providers.IDPType]int, *tidcommon.ServiceError) {
	all, svcErr := s.idpService.GetIdentityProviderList(ctx)
	if svcErr != nil {
		return nil, svcErr
	}
	counts := make(map[providers.IDPType]int)
	for _, instance := range all {
		counts[instance.Type]++
	}
	return counts, nil
}

// handleListConnections handles GET /connections, returning the available connection types
// with their configured status and instance count.
func (h *handler) handleListConnections(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	counts, svcErr := h.svc.typeCounts(ctx)
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}

	summaries := make([]connectionTypeSummary, 0, len(idpBackedVendors))
	for _, vendor := range idpBackedVendors {
		count := counts[vendor.idpType]
		summaries = append(summaries, connectionTypeSummary{
			Type:          vendor.name,
			Configured:    count > 0,
			InstanceCount: count,
		})
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, connectionListResponse{Connections: summaries})
}
