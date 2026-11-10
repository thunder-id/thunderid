// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package authzdetailmgt manages the registry of RFC 9396 authorization details types: the management
// API, declarative types, and import and export. It provides the registered types to runtime processing
// through providers.AuthorizationDetailTypeProvider.
package authzdetailmgt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/thunder-id/thunderid/internal/authzdetail/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/managedresource"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	"github.com/thunder-id/thunderid/internal/system/outboundauth/httpauth"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	// maxTypeValueLength bounds a type value, typically a URI (RFC 9396 section 2.1).
	maxTypeValueLength = 512
	// maxNameLength bounds the display name of a type, in characters.
	maxNameLength = 100
	// maxConsentTemplateLength bounds a consent template.
	maxConsentTemplateLength = 1000
	// maxSchemaLength bounds a type's schema, in bytes.
	maxSchemaLength = 64 << 10
	// maxTypeIDLength bounds a supplied type id to the width of the store's ID column.
	maxTypeIDLength = 36
)

// templateBraces matches any text between double braces in a consent template.
var templateBraces = regexp.MustCompile(`\{\{[^{}]*\}\}`)

// AuthorizationDetailMgtServiceInterface defines the service for managing authorization details types.
type AuthorizationDetailMgtServiceInterface interface {
	providers.AuthorizationDetailTypeProvider

	// CreateAuthorizationDetailType registers a type.
	CreateAuthorizationDetailType(ctx context.Context, t providers.AuthorizationDetailType) (
		*providers.AuthorizationDetailType, *tidcommon.ServiceError)
	// GetAuthorizationDetailType retrieves a type by id.
	GetAuthorizationDetailType(ctx context.Context, id string) (*providers.AuthorizationDetailType,
		*tidcommon.ServiceError)
	// ListAuthorizationDetailTypes lists the registered types, a page at a time.
	ListAuthorizationDetailTypes(ctx context.Context, limit, offset int) (*AuthorizationDetailTypePage,
		*tidcommon.ServiceError)
	// UpdateAuthorizationDetailType updates a type.
	UpdateAuthorizationDetailType(ctx context.Context, id string, t providers.AuthorizationDetailType) (
		*providers.AuthorizationDetailType, *tidcommon.ServiceError)
	// DeleteAuthorizationDetailType deletes a type.
	DeleteAuthorizationDetailType(ctx context.Context, id string) *tidcommon.ServiceError
	// ListAllAuthorizationDetailTypes lists every registered type.
	ListAllAuthorizationDetailTypes(ctx context.Context) ([]providers.AuthorizationDetailType,
		*tidcommon.ServiceError)
	// ValidateAuthorizationDetailType checks a type's definition and processor credential as create and update
	// do, without saving it. The credential is checked against the stored type when one has the type's id.
	ValidateAuthorizationDetailType(ctx context.Context, t providers.AuthorizationDetailType) *tidcommon.ServiceError
	// IsAuthorizationDetailTypeDeclarative reports whether a type is defined in declarative configuration.
	IsAuthorizationDetailTypeDeclarative(ctx context.Context, id string) (bool, *tidcommon.ServiceError)
}

// authorizationDetailMgtService implements AuthorizationDetailMgtServiceInterface.
type authorizationDetailMgtService struct {
	authzService  sysauthz.SystemAuthorizationServiceInterface
	typeStore     typeStoreInterface
	valueCapturer declarativeresource.ValueCapturer
	bus           eventbus.EventBusInterface
	logger        *log.Logger
	// managed says which types a control plane deployed, which stay read only here. Nil where nothing
	// is deployed from one.
	managed managedresource.StoreInterface
}

// newAuthorizationDetailMgtService creates a new authorizationDetailMgtService.
func newAuthorizationDetailMgtService(typeStore typeStoreInterface,
	valueCapturer declarativeresource.ValueCapturer,
	authzService sysauthz.SystemAuthorizationServiceInterface,
	bus eventbus.EventBusInterface) AuthorizationDetailMgtServiceInterface {
	return &authorizationDetailMgtService{
		authzService:  authzService,
		typeStore:     typeStore,
		valueCapturer: valueCapturer,
		bus:           bus,
		logger: log.GetLogger().With(
			log.String(log.LoggerKeyComponentName, "AuthorizationDetailMgtService")),
	}
}

// CreateAuthorizationDetailType registers a type. Its processor credential is taken as the caller sent it.
func (s *authorizationDetailMgtService) CreateAuthorizationDetailType(ctx context.Context,
	t providers.AuthorizationDetailType) (*providers.AuthorizationDetailType, *tidcommon.ServiceError) {
	created, svcErr := s.createAuthorizationDetailType(ctx, t)
	var typeID string
	if svcErr == nil {
		typeID = created.ID
	}
	s.publishTypeEvent(ctx, audit.EventAuthorizationDetailTypeCreate, audit.ActionCreate, typeID, svcErr)
	return created, svcErr
}

// createAuthorizationDetailType registers a type without publishing its audit event.
func (s *authorizationDetailMgtService) createAuthorizationDetailType(ctx context.Context,
	t providers.AuthorizationDetailType) (*providers.AuthorizationDetailType, *tidcommon.ServiceError) {
	if svcErr := s.checkWriteAccess(ctx, security.ActionCreateAuthzDetail); svcErr != nil {
		return nil, svcErr
	}
	if IsDeclarativeModeEnabled() {
		return nil, &ErrorDeclarativeModeCreateNotAllowed
	}
	if svcErr := validateTypeDefinition(t); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := setProcessorAuthentication(&t, nil); svcErr != nil {
		return nil, svcErr
	}

	if _, err := s.typeStore.GetTypeByValue(ctx, t.Type); err == nil {
		return nil, &ErrorTypeConflict
	} else if !errors.Is(err, errTypeNotFound) {
		s.logger.Error(ctx, "Failed to check authorization details type", log.String("type", t.Type),
			log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	// An id is supplied only on import, so the type keeps its exported id.
	if t.ID == "" {
		id, err := sysutils.GenerateUUIDv7()
		if err != nil {
			s.logger.Error(ctx, "Failed to generate authorization details type id", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		t.ID = id
	} else if len(t.ID) > maxTypeIDLength {
		return nil, &ErrorInvalidRequestFormat
	} else if _, err := s.typeStore.GetTypeByID(ctx, t.ID); err == nil {
		return nil, &ErrorTypeIDConflict
	} else if !errors.Is(err, errTypeNotFound) {
		s.logger.Error(ctx, "Failed to check authorization details type id", log.String("id", t.ID),
			log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	t.Version = 1

	if err := s.typeStore.CreateType(ctx, t); err != nil {
		s.logger.Error(ctx, "Failed to create authorization details type", log.String("id", t.ID),
			log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	s.captureValues(ctx, &t)
	return &t, nil
}

// GetAuthorizationDetailType retrieves a type by id.
func (s *authorizationDetailMgtService) GetAuthorizationDetailType(ctx context.Context,
	id string) (*providers.AuthorizationDetailType, *tidcommon.ServiceError) {
	t, err := s.typeStore.GetTypeByID(ctx, id)
	if err != nil {
		if errors.Is(err, errTypeNotFound) {
			return nil, &ErrorTypeNotFound
		}
		s.logger.Error(ctx, "Failed to get authorization details type", log.String("id", id), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if !t.IsReadOnly {
		deployed, err := managedresource.IsDeployed(ctx, s.managed, id, managedresource.TypeAuthorizationDetailType)
		if err != nil {
			// Every write to such a type is refused whatever the read says, so it is served without the flag.
			s.logger.Error(ctx, "Failed to read whether a control plane deployed the authorization details type",
				log.String("id", id), log.Error(err))
		}
		t.IsReadOnly = deployed
	}
	return &t, nil
}

// getType reads a type, and whether a control plane deployed it. The type's own IsReadOnly says only
// whether it is declared in a file.
func (s *authorizationDetailMgtService) getType(ctx context.Context,
	id string) (*providers.AuthorizationDetailType, bool, *tidcommon.ServiceError) {
	t, err := s.typeStore.GetTypeByID(ctx, id)
	if err != nil {
		if errors.Is(err, errTypeNotFound) {
			return nil, false, &ErrorTypeNotFound
		}
		s.logger.Error(ctx, "Failed to get authorization details type", log.String("id", id), log.Error(err))
		return nil, false, &tidcommon.InternalServerError
	}
	if t.IsReadOnly {
		return &t, false, nil
	}
	deployed, err := managedresource.IsDeployed(ctx, s.managed, id, managedresource.TypeAuthorizationDetailType)
	if err != nil {
		s.logger.Error(ctx, "Failed to read whether a control plane deployed the authorization details type",
			log.String("id", id), log.Error(err))
		return nil, false, &tidcommon.InternalServerError
	}
	return &t, deployed, nil
}

// refuseReadOnly refuses a change to a type declared in a file, or to one a control plane deployed unless
// the change is the managed import's.
func refuseReadOnly(ctx context.Context, t *providers.AuthorizationDetailType,
	deployed bool) *tidcommon.ServiceError {
	if t.IsReadOnly {
		return &ErrorImmutableType
	}
	if deployed && !managedresource.IsManagedImport(ctx) {
		return &ErrorManagedType
	}
	return nil
}

// ListAuthorizationDetailTypes lists the registered types, a page at a time.
func (s *authorizationDetailMgtService) ListAuthorizationDetailTypes(ctx context.Context,
	limit, offset int) (*AuthorizationDetailTypePage, *tidcommon.ServiceError) {
	if limit < 1 || limit > serverconst.MaxPageSize || offset < 0 {
		return nil, &ErrorInvalidPagination
	}
	total, err := s.typeStore.CountTypes(ctx)
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, &ErrorResultLimitExceededInCompositeMode
		}
		s.logger.Error(ctx, "Failed to count authorization details types", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	types, err := s.typeStore.ListTypePage(ctx, limit, offset)
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, &ErrorResultLimitExceededInCompositeMode
		}
		s.logger.Error(ctx, "Failed to list authorization details types", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if err := managedresource.MarkDeployed(ctx, s.managed, types,
		func(t *providers.AuthorizationDetailType) string { return t.ID },
		func(t *providers.AuthorizationDetailType) { t.IsReadOnly = true },
		managedresource.TypeAuthorizationDetailType); err != nil {
		// Every write to such a type is refused whatever the list says, so it is served without the flag.
		s.logger.Error(ctx, "Failed to read which authorization details types a control plane deployed",
			log.Error(err))
	}
	return &AuthorizationDetailTypePage{TotalResults: total, Types: types}, nil
}

// UpdateAuthorizationDetailType updates a type. Its processor credential is taken as the caller sent it: an
// omitted or masked one keeps the stored credential while the endpoint is unchanged.
func (s *authorizationDetailMgtService) UpdateAuthorizationDetailType(ctx context.Context, id string,
	t providers.AuthorizationDetailType) (*providers.AuthorizationDetailType, *tidcommon.ServiceError) {
	updated, svcErr := s.updateAuthorizationDetailType(ctx, id, t)
	s.publishTypeEvent(ctx, audit.EventAuthorizationDetailTypeUpdate, audit.ActionUpdate, id, svcErr)
	return updated, svcErr
}

// updateAuthorizationDetailType updates a type without publishing its audit event.
func (s *authorizationDetailMgtService) updateAuthorizationDetailType(ctx context.Context, id string,
	t providers.AuthorizationDetailType) (*providers.AuthorizationDetailType, *tidcommon.ServiceError) {
	if svcErr := s.checkWriteAccess(ctx, security.ActionUpdateAuthzDetail); svcErr != nil {
		return nil, svcErr
	}
	existing, deployed, svcErr := s.getType(ctx, id)
	if svcErr != nil {
		return nil, svcErr
	}
	if svcErr := refuseReadOnly(ctx, existing, deployed); svcErr != nil {
		return nil, svcErr
	}
	if t.Type != existing.Type {
		return nil, &ErrorTypeValueImmutable
	}
	if svcErr := validateTypeDefinition(t); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := setProcessorAuthentication(&t, existing.Processor); svcErr != nil {
		return nil, svcErr
	}

	t.ID = existing.ID
	// The update applies only while the type is at the version read here, so it never overwrites a change that
	// moved the version, which approvals given on that version rely on.
	t.Version = existing.Version
	// Version bumps invalidate earlier approvals; adding or removing a processor, or changing reuse, counts as a
	// grant change.
	changesGrant := !common.SchemaEquivalent(t.Schema, existing.Schema) ||
		strings.TrimSpace(t.ConsentTemplate) != strings.TrimSpace(existing.ConsentTemplate) ||
		(t.Processor == nil) != (existing.Processor == nil) || t.Reuse != existing.Reuse
	// An equivalent but reordered schema is still stored (it drives consent field order), without a version bump.
	if !changesGrant && sameJSONText(t.Schema, existing.Schema) && t.ConsentTemplate == existing.ConsentTemplate &&
		t.Name == existing.Name &&
		t.Description == existing.Description &&
		t.Reuse == existing.Reuse && reflect.DeepEqual(t.Processor, existing.Processor) {
		return existing, nil
	}
	if err := s.typeStore.UpdateType(ctx, t, changesGrant); err != nil {
		if errors.Is(err, errTypeNotFound) {
			return nil, s.updateMissedError(ctx, id)
		}
		s.logger.Error(ctx, "Failed to update authorization details type", log.String("id", id), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	updated, svcErr := s.GetAuthorizationDetailType(ctx, id)
	if svcErr != nil {
		return nil, svcErr
	}
	s.captureValues(ctx, updated)
	return updated, nil
}

// updateMissedError tells apart a type deleted during an update from one whose version moved meanwhile.
func (s *authorizationDetailMgtService) updateMissedError(ctx context.Context, id string) *tidcommon.ServiceError {
	if _, svcErr := s.GetAuthorizationDetailType(ctx, id); svcErr != nil {
		return svcErr
	}
	return &ErrorTypeChanged
}

// checkWriteAccess refuses a caller who may not write an authorization details type.
func (s *authorizationDetailMgtService) checkWriteAccess(ctx context.Context,
	action security.Action) *tidcommon.ServiceError {
	allowed, svcErr := s.authzService.IsActionAllowed(ctx, action,
		&sysauthz.ActionContext{ResourceType: security.ResourceTypeAuthorizationDetailType})
	if svcErr != nil {
		return &tidcommon.InternalServerError
	}
	if !allowed {
		return &tidcommon.ErrorUnauthorized
	}
	return nil
}

// ValidateAuthorizationDetailType checks a type's definition and processor credential as create and update do,
// without saving it. The credential is checked against the stored type when one has the type's id.
func (s *authorizationDetailMgtService) ValidateAuthorizationDetailType(ctx context.Context,
	t providers.AuthorizationDetailType) *tidcommon.ServiceError {
	if svcErr := validateTypeDefinition(t); svcErr != nil {
		return svcErr
	}
	if t.Processor == nil {
		return nil
	}
	var stored *providers.AuthorizationDetailProcessor
	if t.ID != "" {
		existing, svcErr := s.GetAuthorizationDetailType(ctx, t.ID)
		if svcErr != nil && svcErr.Code != ErrorTypeNotFound.Code {
			return svcErr
		}
		if existing != nil {
			stored = existing.Processor
		}
	}
	return setProcessorAuthentication(&t, stored)
}

// DeleteAuthorizationDetailType deletes a type. Deleting a type that does not exist succeeds.
func (s *authorizationDetailMgtService) DeleteAuthorizationDetailType(ctx context.Context,
	id string) *tidcommon.ServiceError {
	svcErr := s.deleteAuthorizationDetailType(ctx, id)
	// A delete that found no type succeeds without an event, so no caller can record a deletion of an ID
	// it made up.
	if svcErr == errTypeNotFoundNoop {
		return nil
	}
	s.publishTypeEvent(ctx, audit.EventAuthorizationDetailTypeDelete, audit.ActionDelete, id, svcErr)
	return svcErr
}

// deleteAuthorizationDetailType deletes a type without publishing its audit event.
func (s *authorizationDetailMgtService) deleteAuthorizationDetailType(ctx context.Context,
	id string) *tidcommon.ServiceError {
	if svcErr := s.checkWriteAccess(ctx, security.ActionDeleteAuthzDetail); svcErr != nil {
		return svcErr
	}
	existing, deployed, svcErr := s.getType(ctx, id)
	if svcErr != nil {
		if svcErr.Code == ErrorTypeNotFound.Code {
			return errTypeNotFoundNoop
		}
		return svcErr
	}
	if svcErr := refuseReadOnly(ctx, existing, deployed); svcErr != nil {
		return svcErr
	}
	if err := s.typeStore.DeleteType(ctx, id); err != nil {
		s.logger.Error(ctx, "Failed to delete authorization details type", log.String("id", id), log.Error(err))
		return &tidcommon.InternalServerError
	}
	return nil
}

// publishTypeEvent publishes the audit event of an attempt on the authorization details type with typeID,
// after the change was stored or refused. Neither the definition nor the processor credential is recorded.
func (s *authorizationDetailMgtService) publishTypeEvent(ctx context.Context, name string, action audit.Action,
	typeID string, svcErr *tidcommon.ServiceError) {
	// A failed attempt keeps the ID the caller asked for, even one that matches no type, as forensic
	// evidence of what was probed; the payload caps it and the sink escapes it.
	target := audit.Ref{Type: audit.TargetTypeAuthorizationDetailType, ID: typeID}
	// Access is checked before the type is looked up, so a denied attempt may name an ID no type has: only a
	// succeeded one names its type as subject.
	var subject audit.Ref
	if svcErr == nil {
		subject = target
	}
	eventbus.TryPublish(ctx, s.bus,
		eventbus.NewEvent(ctx, name).With(audit.ChangePayloadWithSubject(ctx, target, subject, action, svcErr)))
}

// ListAllAuthorizationDetailTypes implements providers.AuthorizationDetailTypeProvider.
func (s *authorizationDetailMgtService) ListAllAuthorizationDetailTypes(
	ctx context.Context) ([]providers.AuthorizationDetailType, *tidcommon.ServiceError) {
	types, err := s.typeStore.ListTypes(ctx)
	if err != nil {
		s.logger.Error(ctx, "Failed to list authorization details types", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return types, nil
}

// IsAuthorizationDetailTypeDeclarative reports whether a type is defined in declarative configuration.
func (s *authorizationDetailMgtService) IsAuthorizationDetailTypeDeclarative(ctx context.Context,
	id string) (bool, *tidcommon.ServiceError) {
	isDeclarative, err := s.typeStore.IsTypeDeclarative(ctx, id)
	if err != nil {
		s.logger.Error(ctx, "Failed to check whether the authorization details type is declarative",
			log.String("id", id), log.Error(err))
		return false, &tidcommon.InternalServerError
	}
	return isDeclarative, nil
}

// GetAuthorizationDetailTypeByValue implements providers.AuthorizationDetailTypeProvider.
func (s *authorizationDetailMgtService) GetAuthorizationDetailTypeByValue(ctx context.Context,
	value string) (*providers.AuthorizationDetailType, *tidcommon.ServiceError) {
	t, err := s.typeStore.GetTypeByValue(ctx, value)
	if err != nil {
		if errors.Is(err, errTypeNotFound) {
			return nil, nil
		}
		s.logger.Error(ctx, "Failed to get authorization details type", log.String("type", value), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return &t, nil
}

// sameJSONText reports whether two JSON documents are written alike, apart from insignificant whitespace.
func sameJSONText(a, b json.RawMessage) bool {
	var compactA, compactB bytes.Buffer
	if json.Compact(&compactA, a) != nil || json.Compact(&compactB, b) != nil {
		return false
	}
	return bytes.Equal(compactA.Bytes(), compactB.Bytes())
}

// validateTypeDefinition checks a type definition at registration.
func validateTypeDefinition(t providers.AuthorizationDetailType) *tidcommon.ServiceError {
	if !isValidTypeValue(t.Type) {
		return &ErrorInvalidTypeValue
	}
	if strings.TrimSpace(t.Name) == "" || utf8.RuneCountInString(t.Name) > maxNameLength {
		return &ErrorInvalidName
	}
	if err := checkSchema(t); err != nil {
		params := map[string]string{"reason": err.Error()}
		var fieldErr *common.FieldError
		if errors.As(err, &fieldErr) {
			params["path"] = fieldErr.Path
		}
		return ErrorInvalidSchema.WithParams(params)
	}
	if err := validateConsentTemplate(t.ConsentTemplate, t.Schema); err != nil {
		return ErrorInvalidConsentTemplate.WithParams(map[string]string{"reason": err.Error()})
	}
	if !isValidProcessorConfig(t.Processor) {
		return &ErrorInvalidProcessor
	}
	if t.Processor == nil && common.Policies(t.Schema).UsesProcessor() {
		return ErrorInvalidSchema.WithParams(map[string]string{
			"reason": "fields are set or changed by the processor, but the type has no processor"})
	}
	return nil
}

// validateConsentTemplate checks the template length and that each placeholder names a schema field.
// checkSchema compiles a type's schema and checks that it allows the type's own value.
func checkSchema(t providers.AuthorizationDetailType) error {
	if len(t.Schema) > maxSchemaLength {
		return fmt.Errorf("the schema must be at most %d bytes", maxSchemaLength)
	}
	if _, err := common.CompileSchema(t.Schema); err != nil {
		return err
	}
	return common.CheckTypeValue(t.Schema, t.Type)
}

func validateConsentTemplate(template string, schema json.RawMessage) error {
	if len(template) > maxConsentTemplateLength {
		return fmt.Errorf("it must be at most %d bytes", maxConsentTemplateLength)
	}
	if !common.IsSafeText(template) {
		return errors.New("it must not contain control or bidirectional formatting characters")
	}
	paths := common.SchemaFieldPaths(schema)
	// Text between braces that is no placeholder, such as a name with a space, would be shown as written.
	if braced := templateBraces.FindAllString(template, -1); len(braced) !=
		len(common.TemplatePlaceholder.FindAllString(template, -1)) {
		return fmt.Errorf("every {{...}} must name a field of the schema, without spaces inside the name")
	}
	for _, match := range common.TemplatePlaceholder.FindAllStringSubmatch(template, -1) {
		if !paths[match[1]] {
			return fmt.Errorf("{{%s}} is not a field of the schema", match[1])
		}
	}
	return nil
}

// isValidTypeValue reports whether a type value is non-empty printable ASCII without spaces.
func isValidTypeValue(v string) bool {
	if v == "" || len(v) > maxTypeValueLength {
		return false
	}
	for i := 0; i < len(v); i++ {
		if v[i] < 0x21 || v[i] > 0x7e {
			return false
		}
	}
	return true
}

// isValidProcessorConfig reports whether a processor endpoint is HTTPS, or HTTP on a loopback host.
func isValidProcessorConfig(cfg *providers.AuthorizationDetailProcessor) bool {
	if cfg == nil {
		return true
	}
	endpoint, ok := httpauth.ParseEndpointURL(cfg.Endpoint)
	return ok && (endpoint.Scheme == "https" || httpauth.IsLoopbackHost(endpoint.Hostname()))
}

// sentAuthenticationProperties returns a processor credential as the caller sent it, as the properties a type
// carries to the service, or nil when none was sent. The service resolves them into the credential it stores.
func sentAuthenticationProperties(authentication *outboundauth.Authentication) ([]cmodels.Property,
	*tidcommon.ServiceError) {
	if authentication == nil {
		return nil, nil
	}
	properties, err := outboundauth.ToProperties(authentication.Config())
	if err != nil {
		return nil, &ErrorInvalidProcessorAuthentication
	}
	return properties, nil
}

// setProcessorAuthentication replaces the processor credential a type carries as the caller sent it with the
// credential to store, keeping an omitted or masked one from the stored processor while the endpoint is
// unchanged. It works on a copy of the processor, so the caller's type is left as it was.
func setProcessorAuthentication(t *providers.AuthorizationDetailType,
	existing *providers.AuthorizationDetailProcessor) *tidcommon.ServiceError {
	if t.Processor == nil {
		return nil
	}
	processor := *t.Processor
	t.Processor = &processor
	var sent *outboundauth.Authentication
	// A stored credential passed back unchanged was not sent again, so it cannot follow a new endpoint; one that
	// is sent is encrypted afresh and never equals the stored properties.
	if len(t.Processor.AuthenticationProperties) > 0 && (existing == nil ||
		!slices.Equal(t.Processor.AuthenticationProperties, existing.AuthenticationProperties)) {
		config, err := outboundauth.FromProperties(t.Processor.AuthenticationProperties)
		if err != nil {
			return &ErrorInvalidProcessorAuthentication
		}
		sent = &outboundauth.Authentication{Type: string(config.Type), Properties: config.Properties}
	}
	properties, svcErr := processorAuthenticationProperties(sent, t.Processor, existing)
	if svcErr != nil {
		return svcErr
	}
	t.Processor.AuthenticationProperties = properties
	return nil
}

// processorAuthenticationProperties returns the processor authentication to store, keeping omitted or
// masked credentials only while the endpoint is unchanged.
func processorAuthenticationProperties(request *outboundauth.Authentication,
	processor, existing *providers.AuthorizationDetailProcessor) ([]cmodels.Property, *tidcommon.ServiceError) {
	if existing == nil {
		config, err := httpauth.Canonicalize(request.Config())
		if err != nil {
			return nil, &ErrorInvalidProcessorAuthentication
		}
		properties, err := outboundauth.ToProperties(config)
		if err != nil {
			return nil, &ErrorInvalidProcessorAuthentication
		}
		return properties, nil
	}
	properties, err := outboundauth.UpdateProperties(request, existing.AuthenticationProperties,
		[]string{existing.Endpoint}, []string{processor.Endpoint},
		httpauth.SupportedTypes(), httpauth.CanonicalizeForUpdate, httpauth.Canonicalize)
	if err != nil {
		if errors.Is(err, outboundauth.ErrAuthenticationRequiredForTargetChange) {
			return nil, &ErrorProcessorAuthenticationRequired
		}
		return nil, &ErrorInvalidProcessorAuthentication
	}
	return properties, nil
}
