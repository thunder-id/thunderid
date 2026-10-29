// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package authzdetailexec validates and processes RFC 9396 authorization details.
package authzdetailexec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/thunder-id/thunderid/internal/authzdetail/common"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// authorizationDetailExecService implements providers.AuthorizationDetailProvider.
type authorizationDetailExecService struct {
	typeProvider providers.AuthorizationDetailTypeProvider
	processor    processorClientInterface
	logger       *log.Logger
}

// newAuthorizationDetailExecService creates a new authorizationDetailExecService.
func newAuthorizationDetailExecService(
	typeProvider providers.AuthorizationDetailTypeProvider,
	processor processorClientInterface,
) providers.AuthorizationDetailProvider {
	return &authorizationDetailExecService{
		typeProvider: typeProvider,
		processor:    processor,
		logger: log.GetLogger().With(
			log.String(log.LoggerKeyComponentName, "AuthorizationDetailExecService")),
	}
}

// ValidateAuthorizationDetails validates requested authorization details (RFC 9396 section 5).
func (s *authorizationDetailExecService) ValidateAuthorizationDetails(ctx context.Context,
	raw string) (json.RawMessage, *tidcommon.ServiceError) {
	objects, err := common.ParseRequestedDetails(raw)
	if err != nil {
		return nil, ErrorMalformedAuthorizationDetails.WithParams(map[string]string{"reason": err.Error()})
	}

	if _, svcErr := s.validateRequestedDetails(ctx, objects); svcErr != nil {
		return nil, svcErr
	}

	validated, err := common.EncodeValues(objects)
	if err != nil {
		s.logger.Error(ctx, "Failed to encode authorization details", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return validated, nil
}

// validateRequestedDetails validates requested details against their registered types.
func (s *authorizationDetailExecService) validateRequestedDetails(ctx context.Context,
	objects []common.Detail) (resolvedTypes, *tidcommon.ServiceError) {
	types, svcErr := s.resolveRequestedTypes(ctx, objects)
	if svcErr != nil {
		return types, svcErr
	}
	for i, obj := range objects {
		if err := common.ValidateDetail(types.schemas[obj.Type], obj); err != nil {
			return types, ErrorAuthorizationDetailNotConformant.WithParams(
				map[string]string{"index": strconv.Itoa(i), "reason": err.Error()})
		}
		if err := common.CheckRequestedFields(types.policies[obj.Type], obj); err != nil {
			return types, ErrorAuthorizationDetailNotAllowed.WithParams(
				map[string]string{"index": strconv.Itoa(i), "reason": err.Error()})
		}
	}
	return types, nil
}

// resolveRequestedTypes looks up and compiles the type of every requested detail.
func (s *authorizationDetailExecService) resolveRequestedTypes(ctx context.Context,
	objects []common.Detail) (resolvedTypes, *tidcommon.ServiceError) {
	types := resolvedTypes{
		byValue:  map[string]providers.AuthorizationDetailType{},
		schemas:  map[string]*jsonschema.Resolved{},
		policies: map[string]common.FieldPolicies{},
	}
	for i, obj := range objects {
		if _, seen := types.byValue[obj.Type]; seen {
			continue
		}
		t, svcErr := s.lookupType(ctx, obj.Type)
		if svcErr != nil {
			return types, &tidcommon.InternalServerError
		}
		if t == nil {
			return types, ErrorUnknownAuthorizationDetailType.WithParams(
				map[string]string{"index": strconv.Itoa(i), "type": obj.Type})
		}
		resolved, err := common.CompileRequestSchema(t.Schema)
		if err != nil {
			s.logger.Error(ctx, "Registered authorization details type schema cannot be compiled",
				log.String("type", obj.Type), log.Error(err))
			return types, &tidcommon.InternalServerError
		}
		types.byValue[obj.Type] = *t
		types.schemas[obj.Type] = resolved
		types.policies[obj.Type] = common.Policies(t.Schema)
	}
	return types, nil
}

// ProcessAuthorizationDetails runs the types' processors with the given subject. In a login the subject is the
// signed-in user or agent, and a detail of a type without a processor is left for consent to decide. When the
// subject is the client itself, as on client_credentials, no one is there to consent, so every type must
// declare a processor.
func (s *authorizationDetailExecService) ProcessAuthorizationDetails(ctx context.Context,
	details []providers.AuthorizationDetail, client providers.EntityReference, clientID string,
	subject providers.EntityReference, resourceServer string,
) ([]providers.AuthorizationDetail, *tidcommon.ServiceError) {
	objects, err := detailObjects(details)
	if err != nil {
		s.logger.Error(ctx, "Failed to read authorization details", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	clientIsSubject := subject.EntityID == client.EntityID
	types := map[string]providers.AuthorizationDetailType{}
	for _, detail := range details {
		if t, seen := types[detail.Type]; seen {
			if t.ID != detail.TypeID || t.Version != detail.TypeVersion {
				return nil, &common.ErrorAuthorizationDetailTypeChanged
			}
			continue
		}
		t, svcErr := common.StartedType(ctx, s.typeProvider, s.logger, detail)
		if svcErr != nil {
			return nil, svcErr
		}
		if clientIsSubject && t.Processor == nil {
			return nil, ErrorAuthorizationDetailNeedsUser.WithParams(map[string]string{"type": detail.Type})
		}
		types[detail.Type] = t
	}
	processed, svcErr := s.processDetails(ctx, objects, types,
		requesterEntity(providers.EntityCategory(client.EntityCategory), clientID), subjectEntity(subject),
		resourceServer)
	if svcErr != nil {
		return nil, svcErr
	}

	byID := make(map[string]providers.AuthorizationDetail, len(details))
	for _, detail := range details {
		byID[detail.ID] = detail
	}
	admitted := make([]providers.AuthorizationDetail, 0, len(processed))
	for _, obj := range processed {
		value, err := obj.Value()
		if err != nil {
			s.logger.Error(ctx, "Failed to encode authorization details", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		detail := byID[obj.ID]
		detail.Value = value
		detail.Admitted = true
		admitted = append(admitted, detail)
	}
	return admitted, nil
}

// PrepareAuthorizationDetails stamps details with identifiers and type versions for a flow. They are validated
// here, so a client's details can be prepared directly, and details validated earlier are checked again since a
// pushed request's types may change before it is used.
func (s *authorizationDetailExecService) PrepareAuthorizationDetails(ctx context.Context,
	validated json.RawMessage) ([]providers.AuthorizationDetail, *tidcommon.ServiceError) {
	objects, err := common.ParseIdentifiedDetails(validated)
	if err != nil {
		return nil, ErrorMalformedAuthorizationDetails.WithParams(map[string]string{"reason": err.Error()})
	}
	types, svcErr := s.validateRequestedDetails(ctx, objects)
	if svcErr != nil {
		return nil, svcErr
	}
	details := make([]providers.AuthorizationDetail, 0, len(objects))
	for _, obj := range objects {
		t := types.byValue[obj.Type]
		value, err := obj.Value()
		if err != nil {
			s.logger.Error(ctx, "Failed to read authorization details", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		details = append(details, providers.AuthorizationDetail{
			ID: obj.ID, Type: obj.Type, TypeID: t.ID, TypeVersion: t.Version,
			Requested: value, Value: value, Admitted: t.Processor == nil,
		})
	}
	return details, nil
}

// detailObjects reads details in the form the comparator and processors work on.
func detailObjects(details []providers.AuthorizationDetail) ([]common.Detail, error) {
	objects := make([]common.Detail, 0, len(details))
	for _, detail := range details {
		obj, err := common.DetailFromValue(detail.ID, detail.Type, detail.Value)
		if err != nil {
			return nil, fmt.Errorf("authorization detail %s is not an object: %w", detail.ID, err)
		}
		objects = append(objects, obj)
	}
	return objects, nil
}

// processDetails runs the types' processors in one call per endpoint and credential, so a credential
// never goes with another type's details. An unavailable processor fails the request.
func (s *authorizationDetailExecService) processDetails(ctx context.Context, objects []common.Detail,
	types map[string]providers.AuthorizationDetailType, client, subject processorEntity,
	resourceServer string) ([]common.Detail, *tidcommon.ServiceError) {
	batches := map[string][]int{}
	keys := []string{}
	for i, obj := range objects {
		t := types[obj.Type]
		if t.Processor == nil {
			continue
		}
		key, err := batchKey(*t.Processor)
		if err != nil {
			s.logger.Error(ctx, "Failed to read the processor authentication", log.String("type", t.Type),
				log.Error(err))
			return nil, &ErrorProcessorUnavailable
		}
		if _, ok := batches[key]; !ok {
			keys = append(keys, key)
		}
		batches[key] = append(batches[key], i)
	}

	refused := map[int]string{}
	for _, key := range keys {
		indexes := batches[key]
		cfg := *types[objects[indexes[0]].Type].Processor
		req, svcErr := processorRequestFor(objects, indexes, types, client, subject, resourceServer)
		if svcErr != nil {
			return nil, svcErr
		}
		resp, err := s.processor.Process(ctx, cfg, req)
		if err != nil {
			s.logger.Warn(ctx, "Authorization details processor unavailable",
				log.String("endpoint", cfg.Endpoint), log.Error(err))
			return nil, &ErrorProcessorUnavailable
		}
		if svcErr := s.applyProcessorAnswer(ctx, objects, cfg, indexes, resp, types, refused); svcErr != nil {
			return nil, svcErr
		}
	}
	if len(refused) == 0 {
		return objects, nil
	}
	// Refused details are dropped (RFC 9396 section 7); the request fails only when none are left.
	kept := make([]common.Detail, 0, len(objects)-len(refused))
	var reasons []string
	for i, obj := range objects {
		reason, ok := refused[i]
		if !ok {
			kept = append(kept, obj)
			continue
		}
		// Each reason is checked on its own, so one that cannot be shown does not hide the others.
		if reason = common.DisplayableReason(reason); reason != "" && !slices.Contains(reasons, reason) {
			reasons = append(reasons, reason)
		}
	}
	if len(kept) == 0 {
		return nil, common.RefusalError(ErrorAuthorizationDetailRefused, errorAuthorizationDetailRefusedReason,
			strings.Join(reasons, "; "))
	}
	return kept, nil
}

// batchKey identifies a processor call by its endpoint and decrypted credential.
func batchKey(processor providers.AuthorizationDetailProcessor) (string, error) {
	config, err := outboundauth.FromProperties(processor.AuthenticationProperties)
	if err != nil {
		return "", err
	}
	credential, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	return processor.Endpoint + "\x00" + string(credential), nil
}

// processorRequestFor builds the request for the details at the given indexes, which share an endpoint.
func processorRequestFor(objects []common.Detail, indexes []int, types map[string]providers.AuthorizationDetailType,
	client, subject processorEntity, resourceServer string) (processorRequest, *tidcommon.ServiceError) {
	req := processorRequest{
		Subject:  subject,
		Client:   client,
		Resource: resourceServer,
		Details:  make([]processorDetail, 0, len(indexes)),
	}
	for _, i := range indexes {
		value, err := objects[i].Value()
		if err != nil {
			return processorRequest{}, &tidcommon.InternalServerError
		}
		req.Details = append(req.Details, processorDetail{
			DetailID:    objects[i].ID,
			Type:        objects[i].Type,
			TypeVersion: types[objects[i].Type].Version,
			Detail:      value,
		})
	}
	return req, nil
}

// applyProcessorAnswer applies a processor's answers in place, recording refusals in refused.
func (s *authorizationDetailExecService) applyProcessorAnswer(ctx context.Context, objects []common.Detail,
	cfg providers.AuthorizationDetailProcessor, indexes []int, resp *processorResponse,
	types map[string]providers.AuthorizationDetailType, refused map[int]string) *tidcommon.ServiceError {
	sent := make(map[string]bool, len(indexes))
	for _, i := range indexes {
		sent[objects[i].ID] = true
	}
	results := make(map[string]processorResult, len(resp.Results))
	for _, result := range resp.Results {
		if !sent[result.DetailID] {
			s.logger.Warn(ctx, "Authorization details processor answered a detail it was not sent",
				log.String("endpoint", cfg.Endpoint))
			return &ErrorProcessorUnavailable
		}
		if _, seen := results[result.DetailID]; seen {
			s.logger.Warn(ctx, "Authorization details processor answered a detail more than once",
				log.String("endpoint", cfg.Endpoint))
			return &ErrorProcessorUnavailable
		}
		results[result.DetailID] = result
	}
	for _, i := range indexes {
		t := types[objects[i].Type]
		result, ok := results[objects[i].ID]
		if !ok || result.Decision == nil {
			s.logger.Warn(ctx, "Authorization details processor answer is missing or undecided",
				log.String("endpoint", cfg.Endpoint), log.String("type", t.Type))
			return &ErrorProcessorUnavailable
		}
		if !*result.Decision {
			s.logger.Debug(ctx, "Authorization detail refused by processor", log.String("type", t.Type),
				log.String("reason", result.Context.ReasonAdmin))
			refused[i] = result.Context.ReasonUser
			continue
		}
		if svcErr := s.applyPatch(ctx, &objects[i], t, result.Patch); svcErr != nil {
			return svcErr
		}
	}
	return nil
}

// applyPatch applies a processor's patch to a detail and checks it still conforms to the type.
func (s *authorizationDetailExecService) applyPatch(ctx context.Context, obj *common.Detail,
	t providers.AuthorizationDetailType, patch map[string]json.RawMessage) *tidcommon.ServiceError {
	changes := make(map[string]json.RawMessage, len(patch))
	for pointer, value := range patch {
		path, err := common.PointerPath(pointer)
		if err != nil {
			s.logger.Warn(ctx, "Authorization details processor patch cannot be applied",
				log.String("type", t.Type), log.Error(err))
			return &ErrorProcessorUnavailable
		}
		changes[path] = value
	}
	patched, err := common.ApplyChanges(*obj, t, changes, common.PartyProcessor)
	if err != nil {
		if !errors.Is(err, common.ErrChangeRefused) {
			s.logger.Error(ctx, "Registered authorization details type schema cannot be compiled", log.Error(err))
			return &tidcommon.InternalServerError
		}
		// The reason can carry the patched values, so only the debug log records it.
		s.logger.Warn(ctx, "Authorization details processor patch refused", log.String("type", t.Type))
		s.logger.Debug(ctx, "Authorization details processor patch refusal reason", log.String("type", t.Type),
			log.Error(err))
		return &ErrorProcessorUnavailable
	}
	*obj = patched
	return nil
}

// lookupType returns the type registered under a value, or nil when no type has it.
func (s *authorizationDetailExecService) lookupType(ctx context.Context,
	value string) (*providers.AuthorizationDetailType, *tidcommon.ServiceError) {
	if s.typeProvider == nil {
		return nil, nil
	}
	t, svcErr := s.typeProvider.GetAuthorizationDetailTypeByValue(ctx, value)
	if svcErr != nil {
		s.logger.Error(ctx, "Failed to get authorization details type", log.String("type", value),
			log.String("code", svcErr.Code))
		return nil, svcErr
	}
	return t, nil
}

// requesterEntity identifies a client as an application or an agent.
func requesterEntity(category providers.EntityCategory, clientID string) processorEntity {
	entityType := oauth2const.SubTypeApp
	if category == providers.EntityCategoryAgent {
		entityType = oauth2const.SubTypeAgent
	}
	return processorEntity{Type: entityType, ID: clientID}
}

// subjectEntity identifies the subject of a request as a user, an agent, or an application.
func subjectEntity(subject providers.EntityReference) processorEntity {
	entityType := string(providers.EntityCategoryUser)
	switch subject.EntityCategory {
	case string(providers.EntityCategoryAgent):
		entityType = oauth2const.SubTypeAgent
	case string(providers.EntityCategoryApp):
		entityType = oauth2const.SubTypeApp
	}
	return processorEntity{Type: entityType, ID: subject.EntityID}
}
