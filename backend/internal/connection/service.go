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

	"github.com/thunder-id/thunderid/internal/idp"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// service delegates connection operations to the underlying identity-provider service,
// scoping each operation to a connection type so a vendor endpoint only ever acts on its
// own instances.
type service struct {
	idpService idp.IDPServiceInterface
}

// newService creates a connection service over the given identity-provider service.
func newService(idpService idp.IDPServiceInterface) *service {
	return &service{idpService: idpService}
}

// listByType returns the configured instances of the given identity-provider type.
func (s *service) listByType(ctx context.Context, idpType providers.IDPType) ([]idp.BasicIDPDTO,
	*tidcommon.ServiceError) {
	all, svcErr := s.idpService.GetIdentityProviderList(ctx)
	if svcErr != nil {
		return nil, svcErr
	}
	instances := make([]idp.BasicIDPDTO, 0)
	for _, instance := range all {
		if instance.Type == idpType {
			instances = append(instances, instance)
		}
	}
	return instances, nil
}

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

// getByType fetches a single instance and verifies it is of the expected type, returning
// a not-found error on a type mismatch so a vendor endpoint cannot read another type.
func (s *service) getByType(ctx context.Context, idpType providers.IDPType, id string) (*providers.IDPDTO,
	*tidcommon.ServiceError) {
	dto, svcErr := s.idpService.GetIdentityProvider(ctx, id)
	if svcErr != nil {
		return nil, svcErr
	}
	if dto.Type != idpType {
		return nil, &idp.ErrorIDPNotFound
	}
	return dto, nil
}

// create delegates creation to the identity-provider service.
func (s *service) create(ctx context.Context, dto *providers.IDPDTO) (*providers.IDPDTO, *tidcommon.ServiceError) {
	return s.idpService.CreateIdentityProvider(ctx, dto)
}

// update verifies the instance is of the expected type, preserves any secret the request
// omits (keeping the stored value), then delegates the update.
func (s *service) update(ctx context.Context, idpType providers.IDPType, id string,
	dto *providers.IDPDTO) (*providers.IDPDTO, *tidcommon.ServiceError) {
	existing, svcErr := s.getByType(ctx, idpType, id)
	if svcErr != nil {
		return nil, svcErr
	}
	dto.Properties = mergeStoredSecrets(dto.Properties, existing.Properties)
	return s.idpService.UpdateIdentityProvider(ctx, id, dto)
}

// deleteByType verifies the instance is of the expected type, then deletes it.
func (s *service) deleteByType(ctx context.Context, idpType providers.IDPType, id string) *tidcommon.ServiceError {
	if _, svcErr := s.getByType(ctx, idpType, id); svcErr != nil {
		return svcErr
	}
	return s.idpService.DeleteIdentityProvider(ctx, id)
}
