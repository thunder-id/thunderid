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
			s.logger.Debug(ctx, "Authorization detail failed schema validation",
				log.String("type", obj.Type), log.Error(err))
			return types, ErrorAuthorizationDetailNotConformant.WithParams(
				map[string]string{"index": strconv.Itoa(i)})
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
	for _, obj := range objects {
		if _, seen := types.byValue[obj.Type]; seen {
			continue
		}
		t, svcErr := s.lookupType(ctx, obj.Type)
		if svcErr != nil {
			return types, &tidcommon.InternalServerError
		}
		if t == nil {
			return types, &ErrorUnknownAuthorizationDetailType
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

// RefreshableAuthorizationDetails drops one-off details and those whose type changed since the grant.
func (s *authorizationDetailExecService) RefreshableAuthorizationDetails(ctx context.Context,
	granted []providers.AuthorizationDetail) ([]providers.AuthorizationDetail, *tidcommon.ServiceError) {
	if len(granted) == 0 {
		return nil, nil
	}
	objects, svcErr := s.parseGrantedDetails(ctx, granted)
	if svcErr != nil {
		return nil, svcErr
	}
	// A deleted, recreated or changed type drops its detail rather than failing the request, since the user
	// approved it on the version it was granted on.
	refreshable := make([]providers.AuthorizationDetail, 0, len(granted))
	for i, detail := range granted {
		obj := objects[i]
		t, svcErr := s.lookupType(ctx, obj.Type)
		if svcErr != nil {
			return nil, &tidcommon.InternalServerError
		}
		switch {
		case t == nil:
			s.logger.Debug(ctx, "Authorization details type deleted; detail is not renewed",
				log.String("type", obj.Type))
		case t.ID != detail.TypeID:
			s.logger.Debug(ctx, "Authorization details type is not the one that granted the detail; "+
				"detail is not renewed", log.String("type", obj.Type))
		case t.Version != detail.TypeVersion:
			s.logger.Debug(ctx, "Authorization details type changed since the grant; detail is not renewed",
				log.String("type", obj.Type))
		case !t.Reuse:
			s.logger.Debug(ctx, "Authorization details type does not allow reuse; detail is not renewed",
				log.String("type", obj.Type))
		default:
			value, err := obj.Value()
			if err != nil {
				s.logger.Error(ctx, "Failed to encode refreshable authorization details", log.Error(err))
				return nil, &tidcommon.InternalServerError
			}
			detail.Value = value
			refreshable = append(refreshable, detail)
		}
	}
	if len(refreshable) == 0 {
		return nil, nil
	}
	return refreshable, nil
}

// parseGrantedDetails parses the values of granted details, in order, as tokens carry them.
func (s *authorizationDetailExecService) parseGrantedDetails(ctx context.Context,
	granted []providers.AuthorizationDetail) ([]common.Detail, *tidcommon.ServiceError) {
	values, err := common.EncodeDetailValues(granted)
	if err != nil {
		s.logger.Error(ctx, "Failed to encode granted authorization details", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	objects, err := common.ParseRequestedDetails(string(values))
	if err != nil {
		s.logger.Error(ctx, "Failed to parse granted authorization details", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return objects, nil
}

// ModifyAuthorizationDetails checks a token request's authorization_details against a grant. Each entry
// must modify a distinct granted detail, so one approval cannot be claimed twice.
func (s *authorizationDetailExecService) ModifyAuthorizationDetails(ctx context.Context,
	granted []providers.AuthorizationDetail, raw string) (json.RawMessage, *tidcommon.ServiceError) {
	if len(granted) == 0 {
		return nil, &ErrorNoAuthorizationDetailsGranted
	}
	grantedObjects, svcErr := s.parseGrantedDetails(ctx, granted)
	if svcErr != nil {
		return nil, svcErr
	}
	grantedTypes := make(map[string]providers.AuthorizationDetail, len(granted))
	for _, detail := range granted {
		grantedTypes[detail.Type] = detail
	}
	requested, err := common.ParseRequestedDetails(raw)
	if err != nil {
		return nil, ErrorMalformedAuthorizationDetails.WithParams(map[string]string{"reason": err.Error()})
	}

	types := map[string]providers.AuthorizationDetailType{}
	for i, obj := range requested {
		if _, ok := types[obj.Type]; ok {
			continue
		}
		grant, isGranted := grantedTypes[obj.Type]
		if !isGranted {
			return nil, ErrorAuthorizationDetailNotGranted.WithParams(map[string]string{"index": strconv.Itoa(i)})
		}
		t, svcErr := s.lookupType(ctx, obj.Type)
		if svcErr != nil {
			return nil, &tidcommon.InternalServerError
		}
		if t == nil || t.ID != grant.TypeID || t.Version != grant.TypeVersion {
			return nil, ErrorAuthorizationDetailTypeChangedSinceGrant.WithParams(
				map[string]string{"index": strconv.Itoa(i)})
		}
		types[obj.Type] = *t
	}

	policies := make(map[string]common.FieldPolicies, len(types))
	for typ, t := range types {
		policies[typ] = common.Policies(t.Schema)
	}
	modifies := make([][]bool, len(requested))
	reasons := make([]error, len(requested))
	for i, obj := range requested {
		modifies[i] = make([]bool, len(grantedObjects))
		for j, base := range grantedObjects {
			if base.Type != obj.Type {
				continue
			}
			if err := common.CheckModification(base, obj, policies[base.Type],
				common.PartyClient); err != nil {
				if reasons[i] == nil {
					reasons[i] = err
				}
				continue
			}
			modifies[i][j] = true
		}
	}
	matched := matchDetails(modifies)
	for i, obj := range requested {
		if matched[i] < 0 {
			if reasons[i] != nil && !slices.Contains(modifies[i], true) {
				return nil, ErrorAuthorizationDetailExceedsGrant.WithParams(
					map[string]string{"index": strconv.Itoa(i), "reason": reasons[i].Error()})
			}
			return nil, ErrorAuthorizationDetailNotGranted.WithParams(map[string]string{"index": strconv.Itoa(i)})
		}
		t := types[obj.Type]
		resolved, err := common.CompileSchema(t.Schema)
		if err != nil {
			s.logger.Error(ctx, "Registered authorization details type schema cannot be compiled", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		if err := common.ValidateDetail(resolved, obj); err != nil {
			return nil, ErrorAuthorizationDetailNotConformant.WithParams(map[string]string{"index": strconv.Itoa(i)})
		}
		if err := common.CheckFieldLimits(policies[obj.Type], obj); err != nil {
			return nil, ErrorAuthorizationDetailNotAllowed.WithParams(
				map[string]string{"index": strconv.Itoa(i), "reason": err.Error()})
		}
	}

	encoded, err := common.EncodeValues(requested)
	if err != nil {
		s.logger.Error(ctx, "Failed to encode modified authorization details", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return encoded, nil
}

// matchDetails pairs each requested detail with a distinct granted detail it modifies, or -1 if none.
func matchDetails(modifies [][]bool) []int {
	matched := make([]int, len(modifies))
	owner := map[int]int{}
	var assign func(i int, visited map[int]bool) bool
	assign = func(i int, visited map[int]bool) bool {
		for j, ok := range modifies[i] {
			if !ok || visited[j] {
				continue
			}
			visited[j] = true
			if current, taken := owner[j]; !taken || assign(current, visited) {
				owner[j] = i
				matched[i] = j
				return true
			}
		}
		return false
	}
	for i := range matched {
		matched[i] = -1
	}
	for i := range modifies {
		assign(i, map[int]bool{})
	}
	return matched
}

// ProcessAuthorizationDetails runs the types' processors with the given subject. In a login the subject is the
// signed-in user or agent, and a detail of a type without a processor is left for consent to decide. When the
// subject is the client itself, as on client_credentials, no one is there to consent, so every type must
// declare a processor.
func (s *authorizationDetailExecService) ProcessAuthorizationDetails(ctx context.Context,
	details []providers.AuthorizationDetail, client *providers.Application, subject providers.EntityReference,
	resourceServer string) ([]providers.AuthorizationDetail, *tidcommon.ServiceError) {
	objects, err := detailObjects(details)
	if err != nil {
		s.logger.Error(ctx, "Failed to read authorization details", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	clientIsSubject := subject.EntityID == client.ID
	types := map[string]providers.AuthorizationDetailType{}
	for _, detail := range details {
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
		requesterEntity(client.EntityCategory, client.OAuthClientID()), subjectEntity(subject),
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

// PrepareAuthorizationDetails stamps validated details with identifiers and type versions for a flow.
// They are validated again since a pushed request's types may change before it is used.
func (s *authorizationDetailExecService) PrepareAuthorizationDetails(ctx context.Context,
	validated json.RawMessage) ([]providers.AuthorizationDetail, *tidcommon.ServiceError) {
	objects, err := common.ParseIdentifiedDetails(validated)
	if err != nil {
		s.logger.Error(ctx, "Failed to prepare validated authorization details", log.Error(err))
		return nil, &tidcommon.InternalServerError
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

// GrantedFromClaim reads the granted authorization details from a flow assertion claim.
func GrantedFromClaim(claim json.RawMessage) ([]providers.AuthorizationDetail, error) {
	granted := []providers.AuthorizationDetail{}
	if err := json.Unmarshal(claim, &granted); err != nil {
		return nil, fmt.Errorf("granted authorization details are malformed: %w", err)
	}
	if granted == nil {
		granted = []providers.AuthorizationDetail{}
	}
	return granted, nil
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
		if reason != "" && !slices.Contains(reasons, reason) {
			reasons = append(reasons, reason)
		}
	}
	if len(kept) == 0 {
		return nil, refusedError(strings.Join(reasons, "; "))
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

// refusedError returns the error for details the processor refused, carrying its reason when it gave one that
// is safe to show the user.
func refusedError(reason string) *tidcommon.ServiceError {
	reason = common.DisplayableReason(reason)
	if reason == "" {
		return &ErrorAuthorizationDetailRefused
	}
	desc := errorAuthorizationDetailRefusedReason
	desc.Params = map[string]string{"reason": reason}
	return tidcommon.CustomServiceError(ErrorAuthorizationDetailRefused, desc)
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
	t, svcErr := s.typeProvider.GetAuthorizationDetailTypeByValue(ctx, value)
	if svcErr != nil {
		s.logger.Error(ctx, "Failed to get authorization details type", log.String("type", value),
			log.String("code", svcErr.Code))
		return nil, svcErr
	}
	return t, nil
}

// noTypes is a type registry that holds no types.
type noTypes struct{}

// NoTypes returns a type registry with no types.
func NoTypes() providers.AuthorizationDetailTypeProvider {
	return noTypes{}
}

// GetAuthorizationDetailTypeByValue returns no type.
func (noTypes) GetAuthorizationDetailTypeByValue(context.Context,
	string) (*providers.AuthorizationDetailType, *tidcommon.ServiceError) {
	return nil, nil
}

// ListAuthorizationDetailTypeValues returns no type values.
func (noTypes) ListAuthorizationDetailTypeValues(context.Context) ([]string, *tidcommon.ServiceError) {
	return []string{}, nil
}

// ListAllAuthorizationDetailTypes returns no types.
func (noTypes) ListAllAuthorizationDetailTypes(context.Context) ([]providers.AuthorizationDetailType,
	*tidcommon.ServiceError) {
	return []providers.AuthorizationDetailType{}, nil
}
