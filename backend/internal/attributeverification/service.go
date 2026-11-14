// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package attributeverification pins a contact attribute before a challenge and records it as verified
// once the challenge is passed.
package attributeverification

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	entitytypemodel "github.com/thunder-id/thunderid/internal/entitytype/model"
	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// AttributeVerificationServiceInterface pins and records contact attribute verification.
type AttributeVerificationServiceInterface interface {
	// PrepareAttributeVerification pins the requested contact without writing.
	PrepareAttributeVerification(ctx context.Context, request providers.AttributeVerificationRequest) (
		*providers.AttributeVerificationPin, *tidcommon.ServiceError)
	// RecordAttributeVerification records the pinned contact as verified if it is still current.
	RecordAttributeVerification(ctx context.Context, pin providers.AttributeVerificationPin) *tidcommon.ServiceError
	// GetVerifiedAttributes returns the verified attributes of a loaded entity without reading storage.
	GetVerifiedAttributes(ctx context.Context, loaded providers.Entity) map[string]providers.VerifiedAttribute
}

// attributeVerificationService is the default implementation of AttributeVerificationServiceInterface.
type attributeVerificationService struct {
	entityService     entity.EntityServiceInterface
	entityTypeService entitytype.EntityTypeServiceInterface
	observability     providers.ObservabilityProvider
	authorizer        sysauthz.SystemAuthorizationServiceInterface
	bus               eventbus.EventBusInterface
	logger            *log.Logger
}

// newAttributeVerificationService creates the attribute verification service.
func newAttributeVerificationService(entityService entity.EntityServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface, observability providers.ObservabilityProvider,
	authorizer sysauthz.SystemAuthorizationServiceInterface,
	bus eventbus.EventBusInterface) *attributeVerificationService {
	return &attributeVerificationService{
		entityService:     entityService,
		entityTypeService: entityTypeService,
		observability:     observability,
		authorizer:        authorizer,
		bus:               bus,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AttributeVerificationService")),
	}
}

// PrepareAttributeVerification pins the stored contact of an existing user, or for a user pending
// creation the given value checked against the user type, without a subject and without an event. A
// declarative user, an entity that is not a user and an attribute that is not a non-empty email or
// mobile contact of its type are refused.
func (s *attributeVerificationService) PrepareAttributeVerification(ctx context.Context,
	request providers.AttributeVerificationRequest) (*providers.AttributeVerificationPin, *tidcommon.ServiceError) {
	if request.PendingCreation {
		// Only users are created after verification.
		return s.prepareContact(ctx, providers.EntityCategoryUser, request.EntityType, request.Attribute,
			request.Value)
	}

	contact, svcErr := s.readStoredContact(ctx, request.EntityID, request.Attribute)
	// The attribute is typed by the caller, so it is recorded only once it resolved to a contact.
	var auditedAttribute string
	if svcErr == nil {
		auditedAttribute = request.Attribute
	}
	s.publishAuditEvent(ctx, audit.EventUserVerificationRequest, audit.ActionRequest, request.EntityID,
		auditedAttribute, svcErr)
	if svcErr != nil {
		return nil, svcErr
	}

	actor := security.GetSubject(ctx)
	if actor == "" {
		actor = contact.pin.Subject
	}
	s.publishEvent(ctx, event.EventTypeAttributeVerificationRequested, contact.pin, actor, providers.StatusSuccess)
	return &contact.pin, nil
}

// RecordAttributeVerification records a pinned contact as verified. A user deleted since the pin, or a
// contact that changed or is no longer eligible, is refused.
func (s *attributeVerificationService) RecordAttributeVerification(ctx context.Context,
	pin providers.AttributeVerificationPin) *tidcommon.ServiceError {
	svcErr := s.recordAttributeVerification(ctx, pin)
	// The pin comes from the evidence of an earlier successful prepare, so its attribute is server-pinned.
	s.publishAuditEvent(ctx, audit.EventUserVerificationVerify, audit.ActionVerify, pin.Subject, pin.Attribute,
		svcErr)
	return svcErr
}

// recordAttributeVerification records a pinned contact as verified and publishes its observability event.
func (s *attributeVerificationService) recordAttributeVerification(ctx context.Context,
	pin providers.AttributeVerificationPin) *tidcommon.ServiceError {
	contact, svcErr := s.readStoredContact(ctx, pin.Subject, pin.Attribute)
	if svcErr == nil && contact.pin != pin {
		svcErr = &ErrorAttributeNotVerifiable
	}

	if svcErr != nil {
		if svcErr.Type == tidcommon.ClientErrorType {
			s.publishEvent(ctx, event.EventTypeAttributeVerificationRefused, pin, pin.Subject, providers.StatusFailure)
		}

		return svcErr
	}

	// The entity service compares the canonical value again under the entity lock.
	applied, err := s.entityService.RecordAttributeVerification(ctx, entity.AttributeVerificationRecord{
		EntityID: pin.Subject, EntityCategory: contact.category, EntityType: contact.entityType,
		Attribute: pin.Attribute, Value: pin.Value, VerifiedAt: time.Now()})
	if err != nil {
		return s.serverError(ctx, "Failed to record attribute verification", pin.Subject, err)
	}

	if !applied {
		s.publishEvent(ctx, event.EventTypeAttributeVerificationRefused, pin, pin.Subject, providers.StatusFailure)
		return &ErrorAttributeNotVerifiable
	}

	s.publishEvent(ctx, event.EventTypeAttributeVerificationVerified, pin, pin.Subject, providers.StatusSuccess)
	return nil
}

// GetVerifiedAttributes returns the verifications recorded in the system attributes of a mutable user,
// without reading the user type's schema, and an empty map for any other entity. It logs and returns nil
// when the verification state cannot be read.
func (s *attributeVerificationService) GetVerifiedAttributes(ctx context.Context,
	loaded providers.Entity) map[string]providers.VerifiedAttribute {
	if loaded.Category != providers.EntityCategoryUser || loaded.IsReadOnly {
		return map[string]providers.VerifiedAttribute{}
	}

	entries, err := entity.ReadRecordedAttributeVerifications(loaded.SystemAttributes)
	if err != nil {
		s.logger.Warn(ctx, "Failed to read the verified attributes of an entity",
			log.MaskedString(log.LoggerKeyEntityID, loaded.ID), log.Error(err))
		return nil
	}

	return entries
}

// readStoredContact reads the stored contact of a user and pins its canonical value. An unknown entity,
// an entity that is not a user, a declarative user, a user the caller may not update and an attribute that
// is not a non-empty email or mobile contact of the user's type are refused.
func (s *attributeVerificationService) readStoredContact(ctx context.Context, entityID, attribute string) (
	*storedContact, *tidcommon.ServiceError) {
	profile, err := s.entityService.GetEntityProfile(ctx, entityID)
	if errors.Is(err, entity.ErrEntityNotFound) {
		return nil, &ErrorAttributeNotVerifiable
	}

	if err != nil {
		return nil, s.serverError(ctx, "Failed to read the contact for attribute verification", entityID, err)
	}

	if profile.Category != providers.EntityCategoryUser || profile.IsReadOnly {
		return nil, &ErrorAttributeNotVerifiable
	}

	if svcErr := s.authorize(ctx, profile); svcErr != nil {
		return nil, svcErr
	}

	var values map[string]json.RawMessage
	if len(profile.Attributes) > 0 {
		if err := json.Unmarshal(profile.Attributes, &values); err != nil {
			return nil, s.serverError(ctx, "Failed to read the contact for attribute verification", entityID, err)
		}
	}

	var value string
	if err := json.Unmarshal(values[attribute], &value); err != nil {
		return nil, &ErrorAttributeNotVerifiable
	}

	pin, svcErr := s.prepareContact(ctx, profile.Category, profile.Type, attribute, value)
	if svcErr != nil {
		return nil, svcErr
	}

	pin.Subject = entityID
	return &storedContact{pin: *pin, category: profile.Category, entityType: profile.Type}, nil
}

// authorize requires the caller to be allowed to update the user in the user's organization unit. A runtime
// caller, such as a flow acting for whoever drives it, is allowed by the authorizer.
func (s *attributeVerificationService) authorize(ctx context.Context,
	profile *providers.Entity) *tidcommon.ServiceError {
	allowed, svcErr := s.authorizer.IsActionAllowed(ctx, security.ActionUpdateUser, &sysauthz.ActionContext{
		ResourceType: security.ResourceTypeUser, OUID: profile.OUID, ResourceID: profile.ID,
	})
	if svcErr != nil {
		s.logger.Error(ctx, "Failed to authorize attribute verification",
			log.MaskedString(log.LoggerKeyEntityID, profile.ID), log.String("errorCode", svcErr.Code))
		return &tidcommon.InternalServerError
	}

	if !allowed {
		return &tidcommon.ErrorUnauthorized
	}

	return nil
}

// prepareContact checks the entity type's schema and pins a canonical contact value without writing.
func (s *attributeVerificationService) prepareContact(ctx context.Context, category providers.EntityCategory,
	entityType, attribute, value string) (*providers.AttributeVerificationPin, *tidcommon.ServiceError) {
	if entityType == "" || s.entityTypeService == nil {
		return nil, &ErrorAttributeNotVerifiable
	}

	attributes, svcErr := s.entityTypeService.GetAttributes(ctx, entitytype.TypeCategory(category), entityType,
		entitytype.AttributeFilter{AllowNonCredential: true})
	if svcErr != nil {
		if svcErr.Code == entitytype.ErrorEntityTypeNotFound.Code {
			return nil, &ErrorAttributeNotVerifiable
		}

		s.logger.Error(ctx, "Failed to read the contact schema for attribute verification",
			log.String("errorCode", svcErr.Code))
		return nil, &tidcommon.InternalServerError
	}

	for _, contact := range attributes {
		if contact.Attribute != attribute || !entitytypemodel.IsContactType(contact.Type) {
			continue
		}

		value = entitytypemodel.CanonicalContactValue(contact.Type, value)
		if strings.TrimSpace(value) == "" {
			return nil, &ErrorAttributeNotVerifiable
		}

		return &providers.AttributeVerificationPin{Attribute: attribute, Value: value,
			Channel: entitytypemodel.ContactChannel(contact.Type)}, nil
	}

	return nil, &ErrorAttributeNotVerifiable
}

// publishEvent publishes an attribute verification event about the pinned contact, naming the actor.
// It names the attribute, never its value.
func (s *attributeVerificationService) publishEvent(ctx context.Context, eventType providers.EventType,
	pin providers.AttributeVerificationPin, actor, status string) {
	if s.observability == nil || !s.observability.IsEnabled() {
		return
	}

	evt := event.NewEvent(syscontext.GetTraceID(ctx), string(eventType), event.ComponentAttributeVerification).
		WithStatus(status).
		WithData(event.DataKey.Subject, pin.Subject).
		WithData(event.DataKey.SubjectType, event.PrincipalTypeUser).
		WithData(event.DataKey.Attribute, pin.Attribute).
		WithData(event.DataKey.ActorSub, actor)
	s.observability.PublishEvent(ctx, evt)
}

// publishAuditEvent publishes the audit event of an attempt on a user's attribute that ended with svcErr,
// setting the attribute detail only when one is given, never its value. A flow on the server's authority
// may carry any bearer token sent to a public path, so under a runtime context the actor is set explicitly:
// the user for a verification that succeeded, anonymous for every other outcome and for a request.
func (s *attributeVerificationService) publishAuditEvent(ctx context.Context, name string, action audit.Action,
	entityID, attribute string, svcErr *tidcommon.ServiceError) {
	target := audit.Ref{Type: audit.TargetTypeUser, ID: entityID}
	payload := audit.ChangePayload(ctx, target, action, svcErr)
	if attribute != "" {
		payload = payload.WithDetails(map[string]string{audit.DetailAttribute: attribute})
	}

	if security.IsRuntimeContext(ctx) {
		actor := audit.Actor{Type: audit.ActorTypeAnonymous}
		if action == audit.ActionVerify && svcErr == nil {
			actor = audit.Actor{Type: audit.ActorTypeUser, ID: entityID}
		}
		payload = payload.WithActor(actor)
	}

	eventbus.TryPublish(ctx, s.bus, eventbus.NewEvent(ctx, name).With(payload))
}

// serverError logs an attribute verification server failure and returns the server error.
func (s *attributeVerificationService) serverError(ctx context.Context, message, entityID string,
	err error) *tidcommon.ServiceError {
	s.logger.Error(ctx, message, log.MaskedString(log.LoggerKeyEntityID, entityID), log.Error(err))
	return &tidcommon.InternalServerError
}
