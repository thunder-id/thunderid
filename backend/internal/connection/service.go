// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"context"
	"sort"
	"strings"

	"github.com/thunder-id/thunderid/internal/connection/authzenpdp"
	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/filter"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/managedresource"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	"github.com/thunder-id/thunderid/internal/system/outboundauth/httpauth"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
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
	resourceService     resource.ResourceServiceInterface
	authZENPDPService   authzenpdp.AuthZENPDPServiceInterface
	httpService         httpconnection.HTTPConnectionServiceInterface
	valueCapturer       declarativeresource.ValueCapturer
	sharingService      sharing.SharingServiceInterface
	authzService        sysauthz.SystemAuthorizationServiceInterface
	bus                 eventbus.EventBusInterface
	logger              *log.Logger
	// managed says which connections a control plane deployed, which stay read only here. Nil where
	// nothing is deployed from one.
	managed managedresource.StoreInterface
}

// newService creates a connection service over the given identity-provider and
// notification-sender services, publishing the audit events of sharing policy changes on bus.
func newService(idpService idp.IDPServiceInterface,
	notificationService notification.NotificationSenderMgtSvcInterface,
	resourceService resource.ResourceServiceInterface,
	authZENPDPService authzenpdp.AuthZENPDPServiceInterface,
	httpService httpconnection.HTTPConnectionServiceInterface, bus eventbus.EventBusInterface) *service {
	return &service{
		idpService:          idpService,
		notificationService: notificationService,
		resourceService:     resourceService,
		authZENPDPService:   authZENPDPService,
		httpService:         httpService,
		bus:                 bus,
		logger:              log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ConnectionService")),
	}
}

// refuseManaged refuses a change to a connection a control plane deployed, unless the change is the
// managed import's: only a deploy from the control plane changes or removes it.
func (s *service) refuseManaged(ctx context.Context, id string) *tidcommon.ServiceError {
	managed, err := managedresource.IsReadOnly(ctx, s.managed, id, managedresource.TypeConnection)
	if err != nil {
		log.GetLogger().Error(ctx, "Failed to read whether a control plane deployed the connection",
			log.String("id", id), log.Error(err))
		return &tidcommon.InternalServerError
	}
	if managed {
		return &ErrorManagedConnection
	}
	return nil
}

// listByType returns the configured instances of the given identity-provider type.
func (s *service) listByType(ctx context.Context, idpType providers.IDPType) ([]connectionInstance,
	*tidcommon.ServiceError) {
	vendor, _ := idpVendorName(idpType)
	return s.listVendorInstances(ctx, categoryIdentityProvider, vendor)
}

// listVendorInstances returns the configured connections of one vendor the caller may list, as the organization unit
// the caller lists sees them.
func (s *service) listVendorInstances(ctx context.Context, category connectionCategory, vendor string) (
	[]connectionInstance, *tidcommon.ServiceError) {
	listOU, visible, svcErr := sysauthz.ResolveListOU(ctx, s.authzService, s.logger, security.ActionListConnections, "")
	if svcErr != nil {
		return nil, svcErr
	}
	if !visible {
		return []connectionInstance{}, nil
	}
	instances, svcErr := s.gatherInstances(ctx, category)
	if svcErr != nil {
		return nil, svcErr
	}
	ofVendor := make([]connectionInstance, 0, len(instances))
	for _, instance := range instances {
		if instance.Type == vendor {
			ofVendor = append(ofVendor, instance)
		}
	}
	return s.instancesSeenBy(ctx, ofVendor, listOU)
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
// order. Both backing services return full lists, so the page is sliced in memory. With an
// organization unit, the list is the connections it owns, in that order, followed by the ones
// shared into it in ID order. A caller confined to organization units lists its own.
func (s *service) listInstances(ctx context.Context, category connectionCategory, ouID string,
	limit, offset int) (*connectionListResponse, *tidcommon.ServiceError) {
	if svcErr := validatePaginationParams(limit, offset); svcErr != nil {
		return nil, svcErr
	}
	listOU, visible, svcErr := sysauthz.ResolveListOU(ctx, s.authzService, s.logger, security.ActionListConnections,
		ouID)
	if svcErr != nil {
		return nil, svcErr
	}
	if !visible {
		return emptyInstancesPage(category, ouID, limit, offset), nil
	}
	// Without a filter, a caller confined to an organization unit lists what that unit sees: the connections
	// it owns, then the ones shared into it.
	return s.collectInstances(ctx, category, listOU, limit, offset)
}

// collectInstances is listInstances for a caller already allowed to list the organization unit's
// connections, or every connection when ouID is empty.
func (s *service) collectInstances(ctx context.Context, category connectionCategory, ouID string,
	limit, offset int) (*connectionListResponse, *tidcommon.ServiceError) {
	instances, svcErr := s.gatherInstances(ctx, category)
	if svcErr != nil {
		return nil, svcErr
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

	extraQuery := ""
	if category != "" {
		extraQuery = "&category=" + string(category)
	}
	if ouID != "" {
		extraQuery = sysutils.FilterQueryParam(map[string]interface{}{filter.AttributeOUID: ouID}) + extraQuery
	}
	instances, svcErr = s.instancesSeenBy(ctx, instances, ouID)
	if svcErr != nil {
		return nil, svcErr
	}
	return pageOfInstances(instances, limit, offset, extraQuery), nil
}

// gatherInstances returns every configured connection of the category, or of every category when it is empty, in
// the order the backing services hold them.
func (s *service) gatherInstances(ctx context.Context, category connectionCategory) ([]connectionInstance,
	*tidcommon.ServiceError) {
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
				OUID:         instance.OUID,
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
				OUID:        sender.OUID,
				Type:        vendor,
				Categories:  []connectionCategory{categorySMSProvider},
			})
		}
	}

	if category == "" || category == categoryAuthorizationPDP {
		connections, svcErr := s.allAuthZENPDPs(ctx)
		if svcErr != nil {
			return nil, svcErr
		}
		for _, connection := range connections {
			instances = append(instances, connectionInstance{
				ID:          connection.ID,
				Name:        connection.Name,
				Description: connection.Description,
				OUID:        connection.OUID,
				Type:        "authzen-pdp",
				Categories:  []connectionCategory{categoryAuthorizationPDP},
			})
		}
	}

	if category == "" || category == categoryEmailProvider {
		senders, svcErr := s.notificationService.ListSendersByType(ctx, ncommon.NotificationSenderTypeEmail)
		if svcErr != nil {
			return nil, svcErr
		}
		for _, sender := range senders {
			vendor, ok := emailVendorName(sender.Provider)
			if !ok {
				continue
			}
			instances = append(instances, connectionInstance{
				ID:          sender.ID,
				Name:        sender.Name,
				Description: sender.Description,
				OUID:        sender.OUID,
				Type:        vendor,
				Categories:  []connectionCategory{categoryEmailProvider},
			})
		}
	}

	if category == "" || category == categoryHTTP {
		connections, svcErr := s.allHTTP(ctx)
		if svcErr != nil {
			return nil, svcErr
		}
		for _, connection := range connections {
			instances = append(instances, connectionInstance{
				ID:          connection.ID,
				Name:        connection.Name,
				Description: connection.Description,
				OUID:        connection.OUID,
				Type:        httpVendorName,
				Categories:  []connectionCategory{categoryHTTP},
			})
		}
	}

	return instances, nil
}

// pageOfInstances returns a page of the listed connections.
func pageOfInstances(
	instances []connectionInstance, limit, offset int, extraQuery string,
) *connectionListResponse {
	total := len(instances)
	page := make([]connectionInstance, 0)
	if offset < total {
		end := offset + limit
		if end > total {
			end = total
		}
		page = instances[offset:end]
	}
	return &connectionListResponse{
		TotalResults: total,
		StartIndex:   offset + 1,
		Count:        len(page),
		Connections:  page,
		Links:        sysutils.BuildPaginationLinks("/connections", limit, offset, total, extraQuery),
	}
}

// instancesSeenBy returns the connections as an organization unit lists them: the ones it owns, in the given order,
// then the ones shared into it, in ID order, each marked with whether the unit owns it. A shared connection carries
// the unit's ID. Without an organization unit, every connection is listed as owned.
func (s *service) instancesSeenBy(ctx context.Context, instances []connectionInstance, ouID string) (
	[]connectionInstance, *tidcommon.ServiceError) {
	if ouID == "" {
		for i := range instances {
			instances[i].IsOwnedByOU = new(true)
		}
		return instances, nil
	}
	listed := make([]connectionInstance, 0, len(instances))
	others := make(map[string]connectionInstance)
	for _, instance := range instances {
		if instance.OUID == ouID {
			instance.IsOwnedByOU = new(true)
			listed = append(listed, instance)
		} else {
			others[instance.ID] = instance
		}
	}
	if s.sharingService == nil {
		return listed, nil
	}
	sharedIDs, svcErr := s.sharingService.ListVisibleResourceIDs(ctx, ConnectionSharingType, ouID)
	if svcErr != nil {
		return nil, translateSharingError(svcErr)
	}
	for _, id := range sharedIDs {
		if instance, ok := others[id]; ok {
			instance.IsOwnedByOU = new(false)
			instance.OUID = ouID
			listed = append(listed, instance)
		}
	}
	return listed, nil
}

// getByType fetches a single instance and verifies it is of the expected type, returning
// a not-found error on a type mismatch so a vendor endpoint cannot read another type. It also
// reports whether the organization unit reading the instance owns it (see checkConnectionRead).
func (s *service) getByType(ctx context.Context, idpType providers.IDPType, id string) (*providers.IDPDTO, bool,
	*tidcommon.ServiceError) {
	dto, svcErr := s.findByType(ctx, idpType, id)
	if svcErr != nil {
		return nil, false, svcErr
	}
	sharedOUID, svcErr := s.checkConnectionRead(ctx, id, dto.OUID)
	if svcErr != nil {
		return nil, false, svcErr
	}
	if sharedOUID == "" {
		return dto, true, nil
	}
	shared := *dto
	shared.OUID = sharedOUID
	return &shared, false, nil
}

// findByType is getByType without checking the caller.
func (s *service) findByType(ctx context.Context, idpType providers.IDPType, id string) (*providers.IDPDTO,
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
	if svcErr := s.checkConnectionAccess(ctx, security.ActionCreateConnection, dto.OUID); svcErr != nil {
		return nil, svcErr
	}
	created, svcErr := s.idpService.CreateIdentityProvider(ctx, dto)
	if svcErr == nil {
		s.captureIDPValues(ctx, created)
	}
	return created, svcErr
}

// update verifies the instance is of the expected type, preserves any secret the request
// omits (keeping the stored value), then delegates the update.
func (s *service) update(ctx context.Context, idpType providers.IDPType, id string,
	dto *providers.IDPDTO) (*providers.IDPDTO, *tidcommon.ServiceError) {
	existing, svcErr := s.findByType(ctx, idpType, id)
	if svcErr != nil {
		return nil, svcErr
	}
	if svcErr := s.checkConnectionWrite(ctx, security.ActionUpdateConnection, id, existing.OUID); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := s.refuseManaged(ctx, id); svcErr != nil {
		return nil, svcErr
	}
	dto.Properties = mergeStoredSecrets(dto.Properties, existing.Properties)
	updated, svcErr := s.idpService.UpdateIdentityProvider(ctx, id, dto)
	if svcErr == nil {
		s.captureIDPValues(ctx, updated)
	}
	return updated, svcErr
}

// deleteByType verifies the instance is of the expected type, then deletes it.
func (s *service) deleteByType(ctx context.Context, idpType providers.IDPType, id string) *tidcommon.ServiceError {
	dto, svcErr := s.idpService.GetIdentityProvider(ctx, id)
	if svcErr != nil {
		if svcErr.Code == idp.ErrorIDPNotFound.Code {
			return s.purgeOrphanedPolicies(ctx, id, svcErr)
		}
		return svcErr
	}
	if dto.Type != idpType {
		return &idp.ErrorIDPNotFound
	}
	if svcErr := s.checkConnectionWrite(ctx, security.ActionDeleteConnection, id, dto.OUID); svcErr != nil {
		return svcErr
	}
	if svcErr := s.refuseManaged(ctx, id); svcErr != nil {
		return svcErr
	}
	if svcErr := s.idpService.DeleteIdentityProvider(ctx, id); svcErr != nil {
		return svcErr
	}
	return s.deleteSharingPolicies(ctx, id)
}

// listSendersByProvider returns the configured senders of the given type and provider.
func (s *service) listSendersByProvider(ctx context.Context, senderType ncommon.NotificationSenderType,
	provider ncommon.NotificationProviderType) ([]connectionInstance, *tidcommon.ServiceError) {
	if senderType == ncommon.NotificationSenderTypeEmail {
		vendor, _ := emailVendorName(provider)
		return s.listVendorInstances(ctx, categoryEmailProvider, vendor)
	}
	vendor, _ := smsVendorName(provider)
	return s.listVendorInstances(ctx, categorySMSProvider, vendor)
}

// getSenderByProvider fetches a single sender and verifies it is of the expected type and
// provider, returning a not-found error on a mismatch so a vendor endpoint cannot read a
// sender belonging to another vendor or channel. It also reports whether the organization unit
// reading the sender owns it (see checkConnectionRead).
func (s *service) getSenderByProvider(ctx context.Context, senderType ncommon.NotificationSenderType,
	provider ncommon.NotificationProviderType, id string) (
	*ncommon.NotificationSenderDTO, bool, *tidcommon.ServiceError) {
	dto, svcErr := s.findSenderByProvider(ctx, senderType, provider, id)
	if svcErr != nil {
		return nil, false, svcErr
	}
	sharedOUID, svcErr := s.checkConnectionRead(ctx, id, dto.OUID)
	if svcErr != nil {
		return nil, false, svcErr
	}
	if sharedOUID == "" {
		return dto, true, nil
	}
	shared := *dto
	shared.OUID = sharedOUID
	return &shared, false, nil
}

// findSenderByProvider is getSenderByProvider without checking the caller.
func (s *service) findSenderByProvider(ctx context.Context, senderType ncommon.NotificationSenderType,
	provider ncommon.NotificationProviderType, id string) (
	*ncommon.NotificationSenderDTO, *tidcommon.ServiceError) {
	dto, svcErr := s.notificationService.GetSender(ctx, id)
	if svcErr != nil {
		return nil, svcErr
	}
	if dto.Type != senderType || dto.Provider != provider {
		return nil, &notification.ErrorSenderNotFound
	}
	return dto, nil
}

// createSender delegates creation to the notification-sender service.
func (s *service) createSender(ctx context.Context, dto ncommon.NotificationSenderDTO) (
	*ncommon.NotificationSenderDTO, *tidcommon.ServiceError) {
	if svcErr := s.checkConnectionAccess(ctx, security.ActionCreateConnection, dto.OUID); svcErr != nil {
		return nil, svcErr
	}
	created, svcErr := s.notificationService.CreateSender(ctx, dto)
	if svcErr == nil {
		s.captureSenderValues(ctx, created)
	}
	return created, svcErr
}

// updateSender verifies the sender is of the expected type and provider, preserves any secret
// the request omits (keeping the stored value) while its credential target is unchanged, then
// delegates the update.
func (s *service) updateSender(ctx context.Context, senderType ncommon.NotificationSenderType,
	provider ncommon.NotificationProviderType, id string,
	dto ncommon.NotificationSenderDTO) (*ncommon.NotificationSenderDTO, *tidcommon.ServiceError) {
	existing, svcErr := s.findSenderByProvider(ctx, senderType, provider, id)
	if svcErr != nil {
		return nil, svcErr
	}
	if svcErr := s.checkConnectionWrite(ctx, security.ActionUpdateConnection, id, existing.OUID); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := s.refuseManaged(ctx, id); svcErr != nil {
		return nil, svcErr
	}
	storedProperties := existing.Properties
	if senderType == ncommon.NotificationSenderTypeMessage && provider == ncommon.NotificationProviderTypeCustom {
		var err error
		dto.Properties, err = mergeSMSGatewayProperties(dto.Properties, existing.Properties)
		if err != nil {
			return nil, &notification.ErrorInvalidRequestFormat
		}
	}
	if senderType == ncommon.NotificationSenderTypeEmail && provider == ncommon.NotificationProviderTypeSMTP {
		var err error
		dto.Properties, err = mergeSMTPAuthentication(dto.Properties, existing.Properties)
		if err != nil {
			return nil, &notification.ErrorInvalidRequestFormat
		}
	}
	if senderType == ncommon.NotificationSenderTypeMessage && provider == ncommon.NotificationProviderTypeCustom ||
		senderType == ncommon.NotificationSenderTypeEmail && provider == ncommon.NotificationProviderTypeSMTP {
		storedProperties = make([]cmodels.Property, 0, len(existing.Properties))
		for _, property := range existing.Properties {
			if !outboundauth.OwnsPropertyKey(property.GetName()) &&
				property.GetName() != ncommon.CustomPropKeyHTTPHeaders {
				storedProperties = append(storedProperties, property)
			}
		}
	}
	dto.Properties = mergeStoredSecrets(dto.Properties, storedProperties,
		senderCredentialTargetKeys(senderType, provider)...)
	updated, svcErr := s.notificationService.UpdateSender(ctx, id, dto)
	if svcErr == nil {
		s.captureSenderValues(ctx, updated)
	}
	return updated, svcErr
}

// senderCredentialTargetKeys returns the transport properties that identify where a sender's
// outbound authentication credential is presented.
func senderCredentialTargetKeys(senderType ncommon.NotificationSenderType,
	provider ncommon.NotificationProviderType) []string {
	if senderType == ncommon.NotificationSenderTypeMessage && provider == ncommon.NotificationProviderTypeCustom {
		return []string{ncommon.CustomPropKeyURL}
	}
	if senderType != ncommon.NotificationSenderTypeEmail {
		return nil
	}
	for _, vendor := range emailBackedVendors {
		if vendor.provider == provider {
			return vendor.credentialTargetKeys
		}
	}
	return nil
}

// deleteSenderByProvider verifies the sender is of the expected type and provider, then deletes it.
// A sender that does not exist is treated as already deleted, while one belonging to another
// vendor or channel is reported as not found.
func (s *service) deleteSenderByProvider(ctx context.Context, senderType ncommon.NotificationSenderType,
	provider ncommon.NotificationProviderType, id string) *tidcommon.ServiceError {
	dto, svcErr := s.notificationService.GetSender(ctx, id)
	if svcErr != nil {
		if svcErr.Code == notification.ErrorSenderNotFound.Code {
			return s.purgeOrphanedPolicies(ctx, id, nil)
		}
		return svcErr
	}
	if dto.Type != senderType || dto.Provider != provider {
		return &notification.ErrorSenderNotFound
	}
	if svcErr := s.checkConnectionWrite(ctx, security.ActionDeleteConnection, id, dto.OUID); svcErr != nil {
		return svcErr
	}
	if svcErr := s.refuseManaged(ctx, id); svcErr != nil {
		return svcErr
	}
	if svcErr := s.notificationService.DeleteSender(ctx, id); svcErr != nil {
		return svcErr
	}
	return s.deleteSharingPolicies(ctx, id)
}

// createAuthZENPDP validates and stores an AuthZEN PDP connection.
func (s *service) createAuthZENPDP(
	ctx context.Context,
	request authzenpdp.ConnectionRequest,
) (*authzenpdp.AuthZENPDPConnection, *tidcommon.ServiceError) {
	if s.authZENPDPService == nil {
		return nil, &tidcommon.InternalServerError
	}
	if svcErr := s.checkConnectionAccess(ctx, security.ActionCreateConnection, request.OUID); svcErr != nil {
		return nil, svcErr
	}
	return s.authZENPDPService.CreateAuthZENPDPConnection(ctx, request)
}

// listAuthZENPDP returns all AuthZEN PDP connections.
func (s *service) listAuthZENPDP(ctx context.Context) ([]connectionInstance, *tidcommon.ServiceError) {
	return s.listVendorInstances(ctx, categoryAuthorizationPDP, authZENPDPVendorName)
}

// allAuthZENPDPs lists every AuthZEN PDP connection, for a caller already allowed to list them.
func (s *service) allAuthZENPDPs(ctx context.Context) ([]authzenpdp.AuthZENPDPConnection, *tidcommon.ServiceError) {
	if s.authZENPDPService == nil {
		return nil, &tidcommon.InternalServerError
	}
	return s.authZENPDPService.ListAuthZENPDPs(ctx)
}

// getAuthZENPDP returns an AuthZEN PDP connection by ID, and whether the organization unit reading it owns it (see
// checkConnectionRead).
func (s *service) getAuthZENPDP(ctx context.Context, id string) (*authzenpdp.AuthZENPDPConnection, bool,
	*tidcommon.ServiceError) {
	connection, svcErr := s.findAuthZENPDP(ctx, id)
	if svcErr != nil {
		return nil, false, svcErr
	}
	sharedOUID, svcErr := s.checkConnectionRead(ctx, id, connection.OUID)
	if svcErr != nil {
		return nil, false, svcErr
	}
	if sharedOUID == "" {
		return connection, true, nil
	}
	shared := *connection
	shared.OUID = sharedOUID
	return &shared, false, nil
}

// findAuthZENPDP is getAuthZENPDP without checking the caller.
func (s *service) findAuthZENPDP(ctx context.Context, id string) (*authzenpdp.AuthZENPDPConnection,
	*tidcommon.ServiceError) {
	if s.authZENPDPService == nil {
		return nil, &tidcommon.InternalServerError
	}
	connection, svcErr := s.authZENPDPService.GetAuthZENPDP(ctx, id)
	if svcErr != nil {
		return nil, svcErr
	}
	if connection == nil {
		return nil, &authzenpdp.ErrorNotFound
	}
	return connection, nil
}

// updateAuthZENPDP validates and updates an AuthZEN PDP connection by ID.
func (s *service) updateAuthZENPDP(
	ctx context.Context,
	id string,
	request authzenpdp.ConnectionRequest,
) (*authzenpdp.AuthZENPDPConnection, *tidcommon.ServiceError) {
	// A connection that is not there is left to the update, which answers and records it.
	existing, svcErr := s.findAuthZENPDP(ctx, id)
	if svcErr == nil {
		if svcErr := s.checkConnectionWrite(ctx, security.ActionUpdateConnection, id, existing.OUID); svcErr != nil {
			return nil, svcErr
		}
		if svcErr := s.refuseManaged(ctx, id); svcErr != nil {
			return nil, svcErr
		}
	} else if svcErr.Code != authzenpdp.ErrorNotFound.Code {
		return nil, svcErr
	}
	return s.authZENPDPService.UpdateAuthZENPDPConnection(ctx, id, request)
}

// deleteAuthZENPDP deletes an AuthZEN PDP connection when it has no blocking usages.
func (s *service) deleteAuthZENPDP(ctx context.Context, id string) *tidcommon.ServiceError {
	existing, svcErr := s.findAuthZENPDP(ctx, id)
	if svcErr != nil {
		if svcErr.Code == authzenpdp.ErrorNotFound.Code {
			return s.purgeOrphanedPolicies(ctx, id, svcErr)
		}
		return svcErr
	}
	if svcErr := s.checkConnectionWrite(ctx, security.ActionDeleteConnection, id, existing.OUID); svcErr != nil {
		return svcErr
	}
	if svcErr := s.refuseManaged(ctx, id); svcErr != nil {
		return svcErr
	}
	usages, svcErr := s.authZENPDPUsages(ctx, id)
	if svcErr != nil {
		return svcErr
	}
	if len(resourcedependency.BlockingUsages(usages)) > 0 {
		return &authzenpdp.ErrorHasBlockingDependencies
	}
	if svcErr := s.authZENPDPService.DeleteAuthZENPDPConnection(ctx, id); svcErr != nil {
		return svcErr
	}
	return s.deleteSharingPolicies(ctx, id)
}

// createHTTP validates and stores an HTTP connection.
func (s *service) createHTTP(ctx context.Context, connection httpconnection.HTTPConnection) (
	*httpconnection.HTTPConnection, *tidcommon.ServiceError) {
	if s.httpService == nil {
		return nil, &tidcommon.InternalServerError
	}
	if svcErr := s.checkConnectionAccess(ctx, security.ActionCreateConnection, connection.OUID); svcErr != nil {
		return nil, svcErr
	}
	created, svcErr := s.httpService.CreateHTTPConnection(ctx, connection)
	if svcErr == nil {
		s.captureHTTPValues(ctx, created)
	}
	return created, svcErr
}

// listHTTP returns all HTTP connections.
func (s *service) listHTTP(ctx context.Context) ([]connectionInstance, *tidcommon.ServiceError) {
	return s.listVendorInstances(ctx, categoryHTTP, httpVendorName)
}

// allHTTP lists every HTTP connection, for a caller already allowed to list them.
func (s *service) allHTTP(ctx context.Context) ([]httpconnection.HTTPConnection, *tidcommon.ServiceError) {
	if s.httpService == nil {
		return nil, &tidcommon.InternalServerError
	}
	return s.httpService.ListHTTPConnections(ctx)
}

// getHTTP returns an HTTP connection by ID, and whether the organization unit reading it owns it (see
// checkConnectionRead).
func (s *service) getHTTP(ctx context.Context, id string) (*httpconnection.HTTPConnection, bool,
	*tidcommon.ServiceError) {
	if s.httpService == nil {
		return nil, false, &tidcommon.InternalServerError
	}
	connection, svcErr := s.httpService.GetHTTPConnection(ctx, id)
	if svcErr != nil {
		return nil, false, svcErr
	}
	sharedOUID, svcErr := s.checkConnectionRead(ctx, id, connection.OUID)
	if svcErr != nil {
		return nil, false, svcErr
	}
	if sharedOUID == "" {
		return connection, true, nil
	}
	shared := *connection
	shared.OUID = sharedOUID
	return &shared, false, nil
}

// getOwnedHTTP returns an HTTP connection by ID for a caller that may read it in its owning organization unit. It
// keeps the stored owner, so a write that follows is checked against it.
func (s *service) getOwnedHTTP(ctx context.Context, id string) (*httpconnection.HTTPConnection,
	*tidcommon.ServiceError) {
	if s.httpService == nil {
		return nil, &tidcommon.InternalServerError
	}
	connection, svcErr := s.httpService.GetHTTPConnection(ctx, id)
	if svcErr != nil {
		return nil, svcErr
	}
	if svcErr := s.checkConnectionAccess(ctx, security.ActionReadConnection, connection.OUID); svcErr != nil {
		return nil, svcErr
	}
	return connection, nil
}

// updateHTTP resolves the request's authentication against the stored connection under the shared
// retention rule, then delegates the update.
func (s *service) updateHTTP(ctx context.Context, id string, connection httpconnection.HTTPConnection) (
	*httpconnection.HTTPConnection, *tidcommon.ServiceError) {
	if s.httpService == nil {
		return nil, &tidcommon.InternalServerError
	}
	existing, svcErr := s.httpService.GetHTTPConnection(ctx, id)
	if svcErr != nil {
		return nil, svcErr
	}
	return s.updateStoredHTTP(ctx, existing, connection)
}

// updateStoredHTTP updates existing as updateHTTP does, for a caller that has already read it.
// TODO: A concurrent write of the same connection can keep a stored secret the newer write discarded (#5840).
func (s *service) updateStoredHTTP(ctx context.Context, existing *httpconnection.HTTPConnection,
	connection httpconnection.HTTPConnection) (*httpconnection.HTTPConnection, *tidcommon.ServiceError) {
	if svcErr := s.checkConnectionWrite(ctx, security.ActionUpdateConnection, existing.ID,
		existing.OUID); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := s.refuseManaged(ctx, existing.ID); svcErr != nil {
		return nil, svcErr
	}
	var svcErr *tidcommon.ServiceError
	if connection.AuthProperties, svcErr = keepStoredHTTPSecrets(connection, *existing); svcErr != nil {
		return nil, svcErr
	}
	updated, svcErr := s.httpService.UpdateHTTPConnection(ctx, existing.ID, connection)
	if svcErr == nil {
		s.captureHTTPValues(ctx, updated)
	}
	return updated, svcErr
}

// keepStoredHTTPSecrets returns the authentication properties of an update to an HTTP connection,
// resolved against the stored connection under the shared retention rule. The credential target of an
// HTTP connection is its URL origin, so pointing a connection at another server needs each secret
// again.
func keepStoredHTTPSecrets(update, existing httpconnection.HTTPConnection) ([]cmodels.Property,
	*tidcommon.ServiceError) {
	request, err := outboundauth.FromProperties(update.AuthProperties)
	if err != nil {
		return nil, &tidcommon.InternalServerError
	}
	previousTarget := []string{existing.URL}
	nextTarget := []string{update.URL}
	if httpconnection.SameOrigin(update.URL, existing.URL) {
		nextTarget = previousTarget
	}
	properties, err := outboundauth.UpdateProperties(
		&outboundauth.Authentication{Type: string(request.Type), Properties: request.Properties},
		existing.AuthProperties, previousTarget, nextTarget, httpauth.SupportedTypes(),
		httpauth.CanonicalizeForUpdate, httpauth.Canonicalize)
	if err != nil {
		return nil, &httpconnection.ErrorInvalidAuthentication
	}
	return properties, nil
}

// deleteHTTP deletes an HTTP connection by ID.
func (s *service) deleteHTTP(ctx context.Context, id string) *tidcommon.ServiceError {
	if s.httpService == nil {
		return &tidcommon.InternalServerError
	}
	// The HTTP connection service treats deleting a connection it does not hold as success, so the lookup has to
	// come first: a delete naming a connection of another kind must not reach the policy cleanup.
	existing, svcErr := s.httpService.GetHTTPConnection(ctx, id)
	if svcErr != nil {
		if svcErr.Code == httpconnection.ErrorNotFound.Code {
			return s.purgeOrphanedPolicies(ctx, id, nil)
		}
		return svcErr
	}
	if svcErr := s.checkConnectionWrite(ctx, security.ActionDeleteConnection, id, existing.OUID); svcErr != nil {
		return svcErr
	}
	if svcErr := s.refuseManaged(ctx, id); svcErr != nil {
		return svcErr
	}
	if svcErr := s.httpService.DeleteHTTPConnection(ctx, id); svcErr != nil {
		return svcErr
	}
	return s.deleteSharingPolicies(ctx, id)
}

// usagesHTTP returns the resources that use an HTTP connection.
func (s *service) usagesHTTP(ctx context.Context, id string) (
	*resourcedependency.DependenciesResponse, *tidcommon.ServiceError) {
	if _, svcErr := s.getOwnedHTTP(ctx, id); svcErr != nil {
		return nil, svcErr
	}
	return s.httpService.GetHTTPConnectionUsages(ctx, id)
}

// HTTPConnectionWriterInterface writes HTTP connections under the rules the connection API applies:
// the retention rule on update, and capturing the values an export refers to.
type HTTPConnectionWriterInterface interface {
	CreateHTTPConnection(ctx context.Context, connection httpconnection.HTTPConnection) (
		*httpconnection.HTTPConnection, *tidcommon.ServiceError)
	GetHTTPConnection(ctx context.Context, id string) (*httpconnection.HTTPConnection, *tidcommon.ServiceError)
	UpdateHTTPConnection(ctx context.Context, existing *httpconnection.HTTPConnection,
		connection httpconnection.HTTPConnection) (*httpconnection.HTTPConnection, *tidcommon.ServiceError)
	DeleteHTTPConnection(ctx context.Context, id string) *tidcommon.ServiceError
	ResolveConnectionOU(ctx context.Context, connection *httpconnection.HTTPConnection) *tidcommon.ServiceError
	ValidateHTTPConnectionCreate(ctx context.Context, connection *httpconnection.HTTPConnection) *tidcommon.ServiceError
	ValidateHTTPConnectionUpdate(ctx context.Context, existing *httpconnection.HTTPConnection,
		connection httpconnection.HTTPConnection) *tidcommon.ServiceError
}

// httpConnectionWriter implements HTTPConnectionWriterInterface over the connection service.
type httpConnectionWriter struct {
	svc *service
}

var _ HTTPConnectionWriterInterface = (*httpConnectionWriter)(nil)

// CreateHTTPConnection creates a connection as the create route does.
func (w *httpConnectionWriter) CreateHTTPConnection(ctx context.Context,
	connection httpconnection.HTTPConnection) (*httpconnection.HTTPConnection, *tidcommon.ServiceError) {
	return w.svc.createHTTP(ctx, connection)
}

// GetHTTPConnection returns a connection by ID.
func (w *httpConnectionWriter) GetHTTPConnection(ctx context.Context, id string) (
	*httpconnection.HTTPConnection, *tidcommon.ServiceError) {
	return w.svc.getOwnedHTTP(ctx, id)
}

// UpdateHTTPConnection updates existing as the update route does, under the retention rule.
func (w *httpConnectionWriter) UpdateHTTPConnection(ctx context.Context, existing *httpconnection.HTTPConnection,
	connection httpconnection.HTTPConnection) (*httpconnection.HTTPConnection, *tidcommon.ServiceError) {
	return w.svc.updateStoredHTTP(ctx, existing, connection)
}

// DeleteHTTPConnection deletes a connection as the delete route does.
func (w *httpConnectionWriter) DeleteHTTPConnection(ctx context.Context, id string) *tidcommon.ServiceError {
	return w.svc.deleteHTTP(ctx, id)
}

// ResolveConnectionOU sets the organization unit the connection is owned by, resolving a handle to its id.
func (w *httpConnectionWriter) ResolveConnectionOU(ctx context.Context,
	connection *httpconnection.HTTPConnection) *tidcommon.ServiceError {
	return w.svc.httpService.ResolveConnectionOU(ctx, connection)
}

// ValidateHTTPConnectionCreate reports what CreateHTTPConnection would refuse, without writing.
func (w *httpConnectionWriter) ValidateHTTPConnectionCreate(ctx context.Context,
	connection *httpconnection.HTTPConnection) *tidcommon.ServiceError {
	return w.svc.httpService.ValidateHTTPConnectionCreate(ctx, connection)
}

// ValidateHTTPConnectionUpdate reports what UpdateHTTPConnection would refuse, without writing.
func (w *httpConnectionWriter) ValidateHTTPConnectionUpdate(ctx context.Context,
	existing *httpconnection.HTTPConnection, connection httpconnection.HTTPConnection) *tidcommon.ServiceError {
	var svcErr *tidcommon.ServiceError
	if connection.AuthProperties, svcErr = keepStoredHTTPSecrets(connection, *existing); svcErr != nil {
		return svcErr
	}
	return w.svc.httpService.ValidateHTTPConnectionUpdate(ctx, existing, &connection)
}

// usagesByType verifies the instance is of the expected type, then returns the resources that
// reference it. Drives the pre-delete confirmation dialog.
func (s *service) usagesByType(ctx context.Context, idpType providers.IDPType, id string) (
	*resourcedependency.DependenciesResponse, *tidcommon.ServiceError) {
	dto, svcErr := s.findByType(ctx, idpType, id)
	if svcErr != nil {
		return nil, svcErr
	}
	if svcErr := s.checkConnectionAccess(ctx, security.ActionReadConnection, dto.OUID); svcErr != nil {
		return nil, svcErr
	}
	return s.idpService.GetIDPUsages(ctx, id)
}

// usagesSenderByProvider verifies the sender is of the expected type and provider, then returns
// the resources that reference it. Drives the pre-delete confirmation dialog.
func (s *service) usagesSenderByProvider(ctx context.Context, senderType ncommon.NotificationSenderType,
	provider ncommon.NotificationProviderType, id string) (
	*resourcedependency.DependenciesResponse, *tidcommon.ServiceError) {
	dto, svcErr := s.findSenderByProvider(ctx, senderType, provider, id)
	if svcErr != nil {
		return nil, svcErr
	}
	if svcErr := s.checkConnectionAccess(ctx, security.ActionReadConnection, dto.OUID); svcErr != nil {
		return nil, svcErr
	}
	return s.notificationService.GetSenderUsages(ctx, id)
}

// usagesAuthZENPDP returns resources that reference an AuthZEN PDP connection.
func (s *service) usagesAuthZENPDP(ctx context.Context, id string) (
	*resourcedependency.DependenciesResponse, *tidcommon.ServiceError) {
	connection, svcErr := s.findAuthZENPDP(ctx, id)
	if svcErr != nil {
		return nil, svcErr
	}
	if svcErr := s.checkConnectionAccess(ctx, security.ActionReadConnection, connection.OUID); svcErr != nil {
		return nil, svcErr
	}
	return s.authZENPDPUsages(ctx, id)
}

// authZENPDPUsages returns the resource servers that use an AuthZEN PDP connection. It reads every
// resource server, whichever organization unit owns it, because a use by any of them blocks the
// connection's delete.
func (s *service) authZENPDPUsages(ctx context.Context, id string) (
	*resourcedependency.DependenciesResponse, *tidcommon.ServiceError) {
	ctx = security.WithRuntimeContext(ctx)
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
			if strings.TrimSpace(resourceServer.AuthorizationEngine.Properties.PDPConnectionID) != id {
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

// checkConnectionAccess checks the caller may perform the action on a connection of the organization
// unit.
func (s *service) checkConnectionAccess(ctx context.Context, action security.Action,
	ouID string) *tidcommon.ServiceError {
	allowed, svcErr := s.authzService.IsActionAllowed(ctx, action,
		&sysauthz.ActionContext{ResourceType: security.ResourceTypeConnection, OUID: ouID})
	if svcErr != nil {
		s.logger.Error(ctx, "Failed to check authorization for action", log.String("action", string(action)),
			log.Any("error", svcErr))
		return &tidcommon.InternalServerError
	}
	if !allowed {
		return &tidcommon.ErrorUnauthorized
	}
	return nil
}

// checkConnectionRead checks the caller may read a connection of the owning organization unit. A caller refused
// there may still read the connection when it is shared into the organization unit the caller is confined to, and
// that unit is returned. It returns empty for a read in the owning organization unit.
func (s *service) checkConnectionRead(ctx context.Context, id, ownerOUID string) (string,
	*tidcommon.ServiceError) {
	refusal := s.checkConnectionAccess(ctx, security.ActionReadConnection, ownerOUID)
	if refusal == nil {
		return "", nil
	}
	ouID, svcErr := s.sharedIntoCallerOU(ctx, id, ownerOUID, refusal)
	if svcErr != nil {
		return "", svcErr
	}
	if ouID == "" {
		return "", refusal
	}
	if svcErr := s.checkConnectionAccess(ctx, security.ActionReadConnection, ouID); svcErr != nil {
		return "", svcErr
	}
	return ouID, nil
}

// checkConnectionWrite checks the caller may change or delete a connection of the owning organization unit. A
// connection shared into the organization unit the caller is confined to is refused with
// ErrorSharedConnectionReadOnly, as a connection declares nothing a sharee may change.
func (s *service) checkConnectionWrite(ctx context.Context, action security.Action, id,
	ownerOUID string) *tidcommon.ServiceError {
	refusal := s.checkConnectionAccess(ctx, action, ownerOUID)
	if refusal == nil {
		return nil
	}
	ouID, svcErr := s.sharedIntoCallerOU(ctx, id, ownerOUID, refusal)
	if svcErr != nil {
		return svcErr
	}
	if ouID != "" {
		return &ErrorSharedConnectionReadOnly
	}
	return refusal
}

// sharedIntoCallerOU returns the organization unit the caller is confined to when a refused connection is shared
// into it, and empty otherwise.
func (s *service) sharedIntoCallerOU(ctx context.Context, id, ownerOUID string,
	refusal *tidcommon.ServiceError) (string, *tidcommon.ServiceError) {
	if s.sharingService == nil {
		return "", nil
	}
	return sharing.SharedIntoCallerOU(ctx, s.authzService, s.sharingService, s.logger, ConnectionSharingType, id,
		ownerOUID, refusal)
}
