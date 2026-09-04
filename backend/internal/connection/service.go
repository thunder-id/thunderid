// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"context"
	"sort"
	"strings"

	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/resource"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
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
	resourceService     resourceServerLister
	authZENPDPStore     authZENPDPStoreInterface
}

type resourceServerLister interface {
	GetResourceServerList(
		ctx context.Context, limit, offset int,
	) (*resource.ResourceServerList, *tidcommon.ServiceError)
}

// newService creates a connection service over the given identity-provider and
// notification-sender services.
func newService(idpService idp.IDPServiceInterface,
	notificationService notification.NotificationSenderMgtSvcInterface,
	resourceService resourceServerLister,
	authZENPDPStore authZENPDPStoreInterface) *service {
	return &service{
		idpService:          idpService,
		notificationService: notificationService,
		resourceService:     resourceService,
		authZENPDPStore:     authZENPDPStore,
	}
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

// smsVendorName returns the connection vendor name for a message provider, or false when the
// provider has no registered vendor (such instances are not exposed by /connections).
func smsVendorName(provider ncommon.MessageProviderType) (string, bool) {
	for _, vendor := range smsBackedVendors {
		if vendor.provider == provider {
			return vendor.name, true
		}
	}
	return "", false
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
				ID:           instance.ID,
				Name:         instance.Name,
				Description:  instance.Description,
				Type:         vendor,
				Categories:   []connectionCategory{categoryIdentityProvider},
				IDJagEnabled: instance.IDJagEnabled,
			})
		}
	}

	if category == "" || category == categorySMSProvider {
		senders, svcErr := s.notificationService.ListSendersByType(ctx, ncommon.NotificationSenderTypeMessage)
		if svcErr != nil {
			return nil, svcErr
		}
		for _, sender := range senders {
			vendor, ok := smsVendorName(sender.Provider)
			if !ok {
				continue
			}
			instances = append(instances, connectionInstance{
				ID:          sender.ID,
				Name:        sender.Name,
				Description: sender.Description,
				Type:        vendor,
				Categories:  []connectionCategory{categorySMSProvider},
			})
		}
	}

	if category == "" || category == categoryAuthorizationPDP {
		connections, svcErr := s.listAuthZENPDP(ctx)
		if svcErr != nil {
			return nil, svcErr
		}
		for _, connection := range connections {
			instances = append(instances, connectionInstance{
				ID:          connection.ID,
				Name:        connection.Name,
				Description: connection.Description,
				Type:        authZENPDPVendorName,
				Categories:  []connectionCategory{categoryAuthorizationPDP},
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

// createAuthZENPDP validates and stores an external AuthZEN PDP connection.
func (s *service) createAuthZENPDP(
	ctx context.Context,
	connection authZENPDPConnection,
) (*authZENPDPConnection, *tidcommon.ServiceError) {
	if svcErr := declarativeresource.CheckDeclarativeCreate(); svcErr != nil {
		return nil, svcErr
	}
	if s.authZENPDPStore == nil {
		return nil, &tidcommon.InternalServerError
	}
	if svcErr := normalizeAuthZENPDPEndpoints(&connection); svcErr != nil {
		return nil, svcErr
	}
	connection.ID = sysutils.GenerateUUID()
	if err := s.authZENPDPStore.create(ctx, connection); err != nil {
		return nil, &tidcommon.InternalServerError
	}
	return &connection, nil
}

// listAuthZENPDP returns all external AuthZEN PDP connections.
func (s *service) listAuthZENPDP(ctx context.Context) ([]authZENPDPConnection, *tidcommon.ServiceError) {
	if s.authZENPDPStore == nil {
		return nil, &tidcommon.InternalServerError
	}
	connections, err := s.authZENPDPStore.list(ctx)
	if err != nil {
		return nil, &tidcommon.InternalServerError
	}
	return connections, nil
}

// getAuthZENPDP returns an external AuthZEN PDP connection by ID.
func (s *service) getAuthZENPDP(ctx context.Context, id string) (*authZENPDPConnection, *tidcommon.ServiceError) {
	if s.authZENPDPStore == nil {
		return nil, &tidcommon.InternalServerError
	}
	connection, err := s.authZENPDPStore.get(ctx, id)
	if err != nil {
		return nil, &tidcommon.InternalServerError
	}
	if connection == nil {
		return nil, &ErrorConnectionNotFound
	}
	return connection, nil
}

// updateAuthZENPDP validates and updates an external AuthZEN PDP connection by ID.
func (s *service) updateAuthZENPDP(
	ctx context.Context,
	id string,
	connection authZENPDPConnection,
) (*authZENPDPConnection, *tidcommon.ServiceError) {
	if svcErr := declarativeresource.CheckDeclarativeUpdate(); svcErr != nil {
		return nil, svcErr
	}
	if _, svcErr := s.getAuthZENPDP(ctx, id); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := normalizeAuthZENPDPEndpoints(&connection); svcErr != nil {
		return nil, svcErr
	}
	if err := s.authZENPDPStore.update(ctx, id, connection); err != nil {
		return nil, &tidcommon.InternalServerError
	}
	updated, err := s.authZENPDPStore.get(ctx, id)
	if err != nil || updated == nil {
		return nil, &tidcommon.InternalServerError
	}
	return updated, nil
}

// deleteAuthZENPDP deletes an external AuthZEN PDP connection when it has no blocking usages.
func (s *service) deleteAuthZENPDP(ctx context.Context, id string) *tidcommon.ServiceError {
	if svcErr := declarativeresource.CheckDeclarativeDelete(); svcErr != nil {
		return svcErr
	}
	if _, svcErr := s.getAuthZENPDP(ctx, id); svcErr != nil {
		return svcErr
	}
	usages, svcErr := s.usagesAuthZENPDP(ctx, id)
	if svcErr != nil {
		return svcErr
	}
	if len(resourcedependency.BlockingUsages(usages)) > 0 {
		return &ErrorConnectionHasBlockingDependencies
	}
	if err := s.authZENPDPStore.delete(ctx, id); err != nil {
		return &tidcommon.InternalServerError
	}
	return nil
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

// usagesSMSByProvider verifies the sender is of the expected provider, then returns the resources
// that reference it. Drives the pre-delete confirmation dialog.
func (s *service) usagesSMSByProvider(ctx context.Context, provider ncommon.MessageProviderType, id string) (
	*resourcedependency.DependenciesResponse, *tidcommon.ServiceError) {
	if _, svcErr := s.getSMSByProvider(ctx, provider, id); svcErr != nil {
		return nil, svcErr
	}
	return s.notificationService.GetSenderUsages(ctx, id)
}

// usagesAuthZENPDP returns resources that reference an external AuthZEN PDP connection.
func (s *service) usagesAuthZENPDP(ctx context.Context, id string) (
	*resourcedependency.DependenciesResponse, *tidcommon.ServiceError) {
	if _, svcErr := s.getAuthZENPDP(ctx, id); svcErr != nil {
		return nil, svcErr
	}
	if s.resourceService == nil {
		return nil, &tidcommon.InternalServerError
	}

	usages := make([]resourcedependency.ResourceDependency, 0)
	offset := 0
	for {
		list, svcErr := s.resourceService.GetResourceServerList(ctx, serverconst.MaxPageSize, offset)
		if svcErr != nil {
			return nil, svcErr
		}
		if list == nil || list.Count == 0 {
			break
		}
		for _, resourceServer := range list.ResourceServers {
			if resourceServer.AuthorizationEngine.Properties.ExternalPDPConnectionID != id {
				continue
			}
			usages = append(usages, resourcedependency.ResourceDependency{
				ResourceType:     resourcedependency.ResourceTypeResourceServer,
				ID:               resourceServer.ID,
				DisplayName:      resourceServer.Name,
				BehaviorOnDelete: resourcedependency.BehaviorRestrict,
			})
		}
		offset += list.Count
		if offset >= list.TotalResults {
			break
		}
	}

	total := len(usages)
	return &resourcedependency.DependenciesResponse{
		TotalResults: &total,
		Count:        total,
		Summary:      map[string]int{resourcedependency.ResourceTypeResourceServer: total},
		Usages:       usages,
	}, nil
}
