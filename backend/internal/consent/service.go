// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package consent

import (
	"context"
	"errors"
	"slices"
	"time"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// ConsentServiceInterface defines the business operations for managing consents and their purposes.
type ConsentServiceInterface interface {
	ListPurposes(ctx context.Context, filters PurposeFilter) ([]ConsentPurpose, *tidcommon.ServiceError)
	CreateConsent(ctx context.Context, consent *ConsentRequest) (*Consent, *tidcommon.ServiceError)
	UpdateConsent(ctx context.Context, consentID string, consent *ConsentRequest) (
		*Consent, *tidcommon.ServiceError)
	SearchConsents(ctx context.Context, filters ConsentFilter) ([]*Consent, *tidcommon.ServiceError)
}

// InboundClientProvider supplies the inbound client attribute data from which consent purposes are
// derived. It is the narrow subset of the inbound client layer that consent purpose derivation
// depends on, defined here so consent does not couple to that layer.
type InboundClientProvider interface {
	// GetInboundClientAttributes returns the configured user attributes for a single inbound client,
	// or ErrInboundClientNotFound if the inbound client does not exist.
	GetInboundClientAttributes(ctx context.Context, inboundClientID string) (
		*inboundmodel.InboundClientAttributes, error)
	// ListInboundClientAttributes returns the configured user attributes for all inbound clients.
	ListInboundClientAttributes(ctx context.Context) ([]inboundmodel.InboundClientAttributes, error)
}

// consentService is the default implementation of ConsentServiceInterface.
type consentService struct {
	consentStore          consentStoreInterface
	transactioner         providers.Transactioner
	inboundClientProvider InboundClientProvider
	bus                   eventbus.EventBusInterface
	logger                *log.Logger
}

// newConsentService creates a new consent service backed by the database store. The inbound client
// provider supplies the persisted inbound client data from which consent purposes are derived, and the
// bus carries the audit events of the consents it writes.
func newConsentService(
	inboundClientProvider InboundClientProvider, bus eventbus.EventBusInterface,
) (ConsentServiceInterface, error) {
	consentStore, transactioner, err := newConsentStore()
	if err != nil {
		return nil, err
	}

	return &consentService{
		consentStore:          consentStore,
		transactioner:         transactioner,
		inboundClientProvider: inboundClientProvider,
		bus:                   bus,
		logger:                log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ConsentService")),
	}, nil
}

// ListPurposes derives the consent purposes matching the given filters.
func (c *consentService) ListPurposes(
	ctx context.Context, filters PurposeFilter,
) ([]ConsentPurpose, *tidcommon.ServiceError) {
	if filters.GroupID != "" {
		inboundClient, err := c.inboundClientProvider.GetInboundClientAttributes(ctx, filters.GroupID)
		if err != nil {
			c.logger.Error(ctx, "Failed to get inbound client for consent purpose derivation",
				log.String("inboundClientID", filters.GroupID), log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		purpose := buildAttributePurpose(inboundClient.InboundClientID, inboundClient.Attributes)
		if purpose == nil {
			return []ConsentPurpose{}, nil
		}
		return []ConsentPurpose{*purpose}, nil
	}

	inboundClients, err := c.inboundClientProvider.ListInboundClientAttributes(ctx)
	if err != nil {
		c.logger.Error(ctx, "Failed to list inbound clients for consent purpose derivation", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	purposes := make([]ConsentPurpose, 0, len(inboundClients))
	for i := range inboundClients {
		purpose := buildAttributePurpose(inboundClients[i].InboundClientID, inboundClients[i].Attributes)
		if purpose != nil {
			purposes = append(purposes, *purpose)
		}
	}
	return purposes, nil
}

// Purpose-name prefixes identify the namespace a consent purpose belongs to. A purpose name is the
// prefix concatenated with the application ID.
const (
	AttributePurposeNamePrefix           = "attributes:"
	PermissionPurposeNamePrefix          = "permissions:"
	AuthorizationDetailPurposeNamePrefix = "authorization_details:"
)

// AttributePurposeName returns the canonical name identifying the attribute consent purpose of an
// application.
func AttributePurposeName(appID string) string {
	return AttributePurposeNamePrefix + appID
}

// PermissionPurposeName returns the canonical name identifying the permission consent purpose of an
// application.
func PermissionPurposeName(appID string) string {
	return PermissionPurposeNamePrefix + appID
}

// AuthorizationDetailPurposeName returns the authorization details consent purpose name of an application.
func AuthorizationDetailPurposeName(appID string) string {
	return AuthorizationDetailPurposeNamePrefix + appID
}

// buildAttributePurpose constructs the attribute consent purpose from an application's configured
// user attributes. Returns nil when no attributes are configured.
func buildAttributePurpose(appID string, attributes []string) *ConsentPurpose {
	if len(attributes) == 0 {
		return nil
	}
	purposeName := AttributePurposeName(appID)
	return &ConsentPurpose{
		ID:          purposeName,
		Name:        purposeName,
		Description: "Attribute consent purpose for application " + appID,
		GroupID:     appID,
		Elements:    attributesToPurposeElements(attributes),
	}
}

// attributesToPurposeElements maps attribute names to attribute-namespace purpose elements, ordered
// by name for stable output.
func attributesToPurposeElements(attributes []string) []PurposeElement {
	names := slices.Clone(attributes)
	slices.Sort(names)
	elements := make([]PurposeElement, 0, len(names))
	for _, name := range names {
		elements = append(elements, PurposeElement{
			Name:        name,
			Namespace:   NamespaceAttribute,
			IsMandatory: false,
		})
	}
	return elements
}

// CreateConsent creates a new consent record together with its authorization records.
func (c *consentService) CreateConsent(
	ctx context.Context, consent *ConsentRequest,
) (*Consent, *tidcommon.ServiceError) {
	created, svcErr := c.createConsent(ctx, consent)
	var consentID string
	if svcErr == nil {
		consentID = created.ID
	}
	c.publishConsentEvent(ctx, consentCreate, consentID, consent, svcErr)
	return created, svcErr
}

// UpdateConsent updates an existing consent record and replaces its authorization records.
func (c *consentService) UpdateConsent(
	ctx context.Context, consentID string, consent *ConsentRequest,
) (*Consent, *tidcommon.ServiceError) {
	updated, svcErr := c.updateConsent(ctx, consentID, consent)
	c.publishConsentEvent(ctx, consentUpdate, consentID, consent, svcErr)
	return updated, svcErr
}

// SearchConsents retrieves the consent records matching the given filters.
//
// A consent whose validity time has elapsed is expired even if its stored status has not yet been
// updated to reflect that. Because the persisted status can therefore be stale, the status filter is
// evaluated here against each consent's effective status rather than pushed down to the store, and
// the returned records carry their effective status.
func (c *consentService) SearchConsents(
	ctx context.Context, filters ConsentFilter,
) ([]*Consent, *tidcommon.ServiceError) {
	if filters.ConsentStatus != "" && !filters.ConsentStatus.IsValid() {
		c.logger.Debug(ctx, "Invalid consent status filter", log.String("status", string(filters.ConsentStatus)))
		return nil, &ErrorInvalidConsentStatus
	}

	consents, err := c.consentStore.SearchConsents(ctx, filters)
	if err != nil {
		c.logger.Error(ctx, "Failed to search consents", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	now := time.Now().Unix()
	filtered := make([]*Consent, 0, len(consents))
	for _, consent := range consents {
		consent.Status = effectiveStatus(consent.Status, consent.ValidityTime, now)
		if filters.ConsentStatus != "" && consent.Status != filters.ConsentStatus {
			continue
		}
		filtered = append(filtered, consent)
	}

	return filtered, nil
}

// createConsent creates a consent record without publishing its audit event.
func (c *consentService) createConsent(
	ctx context.Context, consent *ConsentRequest,
) (*Consent, *tidcommon.ServiceError) {
	if svcErr := validateConsentRequest(consent); svcErr != nil {
		return nil, svcErr
	}

	id, err := utils.GenerateUUIDv7()
	if err != nil {
		c.logger.Error(ctx, "Failed to generate consent ID", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	authorizations, err := buildAuthorizations(consent.Authorizations)
	if err != nil {
		c.logger.Error(ctx, "Failed to generate consent authorization ID", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	newConsent := &Consent{
		ID:             id,
		GroupID:        consent.GroupID,
		Status:         ConsentStatusActive,
		ValidityTime:   consent.ValidityTime,
		Purposes:       consent.Purposes,
		Authorizations: authorizations,
	}

	err = c.transactioner.Transact(ctx, func(txCtx context.Context) error {
		return c.consentStore.CreateConsent(txCtx, newConsent)
	})
	if err != nil {
		c.logger.Error(ctx, "Failed to create consent", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	c.logger.Debug(ctx, "Successfully created consent", log.String("id", id))
	return newConsent, nil
}

// updateConsent updates a consent record without publishing its audit event.
func (c *consentService) updateConsent(
	ctx context.Context, consentID string, consent *ConsentRequest,
) (*Consent, *tidcommon.ServiceError) {
	if consentID == "" {
		return nil, &ErrorMissingConsentID
	}
	if svcErr := validateConsentRequest(consent); svcErr != nil {
		return nil, svcErr
	}

	var updatedConsent *Consent
	err := c.transactioner.Transact(ctx, func(txCtx context.Context) error {
		existing, err := c.consentStore.GetConsent(txCtx, consentID)
		if err != nil {
			return err
		}

		authorizations, err := buildAuthorizations(consent.Authorizations)
		if err != nil {
			return err
		}

		updatedConsent = &Consent{
			ID:             consentID,
			GroupID:        existing.GroupID,
			Status:         existing.Status,
			ValidityTime:   consent.ValidityTime,
			Purposes:       consent.Purposes,
			Authorizations: authorizations,
		}
		return c.consentStore.UpdateConsent(txCtx, updatedConsent)
	})
	if err != nil {
		if errors.Is(err, errConsentNotFound) {
			c.logger.Debug(ctx, "Consent not found", log.String("id", consentID))
			return nil, &ErrorConsentNotFound
		}
		c.logger.Error(ctx, "Failed to update consent", log.String("id", consentID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	c.logger.Debug(ctx, "Successfully updated consent", log.String("id", consentID))
	return updatedConsent, nil
}

// publishConsentEvent publishes the audit event of an attempt on the consent with consentID, after its
// transaction returned. The subject of a succeeded attempt is the user the request holds decisions for,
// if it names one; a failed attempt has none. The purposes and elements are never recorded.
//
// A runtime flow, such as sign-in, runs with the subject of the request it came in on, which a public
// path lets anyone's bearer token be. Its actor is therefore set explicitly: the user who gave the
// consent, or anonymous when it was not recorded.
func (c *consentService) publishConsentEvent(ctx context.Context, change auditedChange, consentID string,
	consent *ConsentRequest, svcErr *tidcommon.ServiceError) {
	// A failed create has no ID, and a failed update keeps the one it was called with, even one that
	// matches no consent, as forensic evidence of what was probed.
	target := audit.Ref{Type: audit.TargetTypeConsent, ID: consentID}
	var subject audit.Ref
	if svcErr == nil && len(consent.Authorizations) > 0 {
		// The consent enforcer sends one authorization per consent today.
		subject = audit.Ref{Type: audit.TargetTypeUser, ID: consent.Authorizations[0].UserID}
	}
	payload := audit.ChangePayloadWithSubject(ctx, target, subject, change.action, svcErr)
	if security.IsRuntimeContext(ctx) {
		actor := audit.Actor{Type: audit.ActorTypeAnonymous}
		if subject.ID != "" {
			actor = audit.Actor{Type: audit.ActorTypeUser, ID: subject.ID}
		}
		payload = payload.WithActor(actor)
	}
	eventbus.TryPublish(ctx, c.bus, eventbus.NewEvent(ctx, change.event).With(payload))
}

// effectiveStatus returns the status a consent presents at the given Unix time. A consent whose
// validity time has elapsed is reported as expired even if its stored status has not been updated.
// A non-positive validity time means the consent never expires.
func effectiveStatus(status ConsentStatus, validityTime, now int64) ConsentStatus {
	if validityTime <= 0 || now < validityTime {
		return status
	}
	return ConsentStatusExpired
}

// validateConsentRequest validates the fields of a consent create or update request.
func validateConsentRequest(consent *ConsentRequest) *tidcommon.ServiceError {
	if consent == nil || consent.GroupID == "" {
		return &ErrorInvalidRequestFormat
	}
	for _, purpose := range consent.Purposes {
		if svcErr := validatePurposeItem(purpose); svcErr != nil {
			return svcErr
		}
	}
	for _, authorization := range consent.Authorizations {
		if svcErr := validateAuthorizationRequest(authorization); svcErr != nil {
			return svcErr
		}
	}
	return nil
}

// validatePurposeItem validates a single purpose item and its element approvals.
func validatePurposeItem(purpose ConsentPurposeItem) *tidcommon.ServiceError {
	if purpose.Name == "" {
		return &ErrorInvalidRequestFormat
	}
	for _, element := range purpose.Elements {
		if element.Name == "" {
			return &ErrorInvalidRequestFormat
		}
		if !element.Namespace.IsValid() {
			return &ErrorInvalidNamespace
		}
	}
	return nil
}

// validateAuthorizationRequest validates a single authorization payload of a consent request.
func validateAuthorizationRequest(authorization ConsentAuthorizationRequest) *tidcommon.ServiceError {
	if authorization.UserID == "" {
		return &ErrorInvalidRequestFormat
	}
	if !authorization.Type.IsValid() {
		return &ErrorInvalidAuthorizationType
	}
	if !authorization.Status.IsValid() {
		return &ErrorInvalidAuthorizationStatus
	}
	return nil
}

// buildAuthorizations converts the authorization payloads into persistable authorization records,
// generating an identifier and stamping the current time on each.
func buildAuthorizations(requests []ConsentAuthorizationRequest) ([]ConsentAuthorization, error) {
	now := time.Now().Unix()
	authorizations := make([]ConsentAuthorization, 0, len(requests))
	for _, request := range requests {
		id, err := utils.GenerateUUIDv7()
		if err != nil {
			return nil, err
		}
		authorizations = append(authorizations, ConsentAuthorization{
			ID:          id,
			UserID:      request.UserID,
			Type:        request.Type,
			Status:      request.Status,
			UpdatedTime: now,
		})
	}
	return authorizations, nil
}
