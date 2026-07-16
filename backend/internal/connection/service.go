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
	"sort"
	"strings"

	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// service delegates connection operations to the underlying identity-provider and
// notification-sender services, scoping each operation to a connection type so a vendor
// endpoint only ever acts on its own instances.
type service struct {
	idpService          idp.IDPServiceInterface
	notificationService notification.NotificationSenderMgtSvcInterface
}

// newService creates a connection service over the given identity-provider and
// notification-sender services.
func newService(idpService idp.IDPServiceInterface,
	notificationService notification.NotificationSenderMgtSvcInterface) *service {
	return &service{idpService: idpService, notificationService: notificationService}
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

// idpVendorName returns the connection vendor name for an identity-provider type, or false
// when the type has no registered vendor (such instances are not exposed by /connections).
func idpVendorName(idpType providers.IDPType) (string, bool) {
	for _, vendor := range idpBackedVendors {
		if vendor.idpType == idpType {
			return vendor.name, true
		}
	}
	return "", false
}

// validatePaginationParams validates the limit and offset pagination parameters.
func validatePaginationParams(limit, offset int) *tidcommon.ServiceError {
	if limit < 1 || limit > serverconst.MaxPageSize {
		return &ErrorInvalidLimit
	}
	if offset < 0 {
		return &ErrorInvalidOffset
	}
	return nil
}

// listInstances returns a page of the configured connection instances across the IdP- and
// sender-backed services, optionally filtered to a single category (empty means no filter).
// The merged list is sorted by type, then name (case-insensitive), then ID, so the listing —
// and therefore pagination — is deterministic regardless of the underlying stores' iteration
// order. Both backing services return full lists, so the page is sliced in memory.
func (s *service) listInstances(ctx context.Context, category connectionCategory,
	limit, offset int) (*connectionListResponse, *tidcommon.ServiceError) {
	if svcErr := validatePaginationParams(limit, offset); svcErr != nil {
		return nil, svcErr
	}

	instances := make([]connectionInstance, 0)

	// Skip the identity-provider fetch entirely when only sms-provider instances were
	// requested. GetIdentityProviderList has no category-scoped variant (every idp.IDPType is
	// vendor-backed, so there is nothing to filter server-side the way notification senders
	// are), but the category check still avoids an unnecessary store call in that case.
	if category == "" || category == categoryIdentityProvider {
		idps, svcErr := s.idpService.GetIdentityProviderList(ctx)
		if svcErr != nil {
			return nil, svcErr
		}
		for _, instance := range idps {
			vendor, ok := idpVendorName(instance.Type)
			if !ok {
				continue
			}
			instances = append(instances, connectionInstance{
				ID:          instance.ID,
				Name:        instance.Name,
				Description: instance.Description,
				Type:        vendor,
				Categories:  []connectionCategory{categoryIdentityProvider},
			})
		}
	}

	if category == "" || category == categorySMSProvider {
		senders, svcErr := s.notificationService.ListSendersByType(ctx, ncommon.NotificationSenderTypeMessage)
		if svcErr != nil {
			return nil, svcErr
		}
		for _, sender := range senders {
			instances = append(instances, connectionInstance{
				ID:          sender.ID,
				Name:        sender.Name,
				Description: sender.Description,
				Type:        string(sender.Provider),
				Categories:  []connectionCategory{categorySMSProvider},
			})
		}
	}

	sort.SliceStable(instances, func(i, j int) bool {
		if instances[i].Type != instances[j].Type {
			return instances[i].Type < instances[j].Type
		}
		nameI, nameJ := strings.ToLower(instances[i].Name), strings.ToLower(instances[j].Name)
		if nameI != nameJ {
			return nameI < nameJ
		}
		return instances[i].ID < instances[j].ID
	})

	total := len(instances)
	page := make([]connectionInstance, 0)
	if offset < total {
		end := offset + limit
		if end > total {
			end = total
		}
		page = instances[offset:end]
	}

	extraQuery := ""
	if category != "" {
		extraQuery = "&category=" + string(category)
	}
	return &connectionListResponse{
		TotalResults: total,
		StartIndex:   offset + 1,
		Count:        len(page),
		Connections:  page,
		Links:        sysutils.BuildPaginationLinks("/connections", limit, offset, total, extraQuery),
	}, nil
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

// listSMSByProvider returns the configured message senders of the given provider.
func (s *service) listSMSByProvider(ctx context.Context, provider ncommon.MessageProviderType) (
	[]ncommon.NotificationSenderDTO, *tidcommon.ServiceError) {
	all, svcErr := s.notificationService.ListSendersByType(ctx, ncommon.NotificationSenderTypeMessage)
	if svcErr != nil {
		return nil, svcErr
	}
	instances := make([]ncommon.NotificationSenderDTO, 0)
	for _, instance := range all {
		if instance.Provider == provider {
			instances = append(instances, instance)
		}
	}
	return instances, nil
}

// getSMSByProvider fetches a single message sender and verifies it is of the expected provider,
// returning a not-found error on a mismatch so a vendor endpoint cannot read another provider.
func (s *service) getSMSByProvider(ctx context.Context, provider ncommon.MessageProviderType, id string) (
	*ncommon.NotificationSenderDTO, *tidcommon.ServiceError) {
	dto, svcErr := s.notificationService.GetSender(ctx, id)
	if svcErr != nil {
		return nil, svcErr
	}
	if dto.Type != ncommon.NotificationSenderTypeMessage || dto.Provider != provider {
		return nil, &notification.ErrorSenderNotFound
	}
	return dto, nil
}

// createSMS delegates creation to the notification-sender service.
func (s *service) createSMS(ctx context.Context, dto ncommon.NotificationSenderDTO) (
	*ncommon.NotificationSenderDTO, *tidcommon.ServiceError) {
	return s.notificationService.CreateSender(ctx, dto)
}

// updateSMS verifies the sender is of the expected provider, preserves any secret the request
// omits (keeping the stored value), then delegates the update.
func (s *service) updateSMS(ctx context.Context, provider ncommon.MessageProviderType, id string,
	dto ncommon.NotificationSenderDTO) (*ncommon.NotificationSenderDTO, *tidcommon.ServiceError) {
	existing, svcErr := s.getSMSByProvider(ctx, provider, id)
	if svcErr != nil {
		return nil, svcErr
	}
	dto.Properties = mergeStoredSecrets(dto.Properties, existing.Properties)
	return s.notificationService.UpdateSender(ctx, id, dto)
}

// deleteSMSByProvider verifies the sender is of the expected provider, then deletes it.
func (s *service) deleteSMSByProvider(ctx context.Context, provider ncommon.MessageProviderType,
	id string) *tidcommon.ServiceError {
	if _, svcErr := s.getSMSByProvider(ctx, provider, id); svcErr != nil {
		return svcErr
	}
	return s.notificationService.DeleteSender(ctx, id)
}

// usagesByType verifies the instance is of the expected type, then returns the resources that
// reference it. Drives the pre-delete confirmation dialog.
func (s *service) usagesByType(ctx context.Context, idpType providers.IDPType, id string) (
	*resourcedependency.DependenciesResponse, *tidcommon.ServiceError) {
	if _, svcErr := s.getByType(ctx, idpType, id); svcErr != nil {
		return nil, svcErr
	}
	return s.idpService.GetIDPUsages(ctx, id)
}
