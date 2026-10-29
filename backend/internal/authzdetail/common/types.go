// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// StartedType returns a detail's type, failing if the type changed since the request started.
func StartedType(ctx context.Context, provider providers.AuthorizationDetailTypeProvider, logger *log.Logger,
	detail providers.AuthorizationDetail) (providers.AuthorizationDetailType, *tidcommon.ServiceError) {
	var t *providers.AuthorizationDetailType
	var svcErr *tidcommon.ServiceError
	if provider != nil {
		t, svcErr = provider.GetAuthorizationDetailTypeByValue(ctx, detail.Type)
	}
	if svcErr != nil {
		logger.Error(ctx, "Failed to get authorization details type", log.String("type", detail.Type),
			log.String("code", svcErr.Code))
		return providers.AuthorizationDetailType{}, &tidcommon.InternalServerError
	}
	if t == nil || t.ID != detail.TypeID || t.Version != detail.TypeVersion {
		logger.Warn(ctx, "Authorization details type changed while the request was in progress",
			log.String("type", detail.Type))
		return providers.AuthorizationDetailType{}, &ErrorAuthorizationDetailTypeChanged
	}
	return *t, nil
}

// ApplyChanges applies changes keyed by field path to a copy of a detail on behalf of party.
// A refused change wraps ErrChangeRefused; any other error means the schema cannot be compiled.
func ApplyChanges(base Detail, t providers.AuthorizationDetailType,
	changes map[string]json.RawMessage, party string) (Detail, error) {
	fieldPaths := SchemaFieldPaths(t.Schema)
	for path := range changes {
		if !fieldPaths[path] {
			return Detail{}, fmt.Errorf("%w: field %s is not part of the type", ErrChangeRefused, path)
		}
	}
	if err := checkOverlappingPaths(changes); err != nil {
		return Detail{}, fmt.Errorf("%w: %w", ErrChangeRefused, err)
	}
	candidate := Detail{ID: base.ID, Type: base.Type, Fields: maps.Clone(base.Fields)}
	for path, value := range changes {
		if err := checkJSONText(value); err != nil {
			return Detail{}, fmt.Errorf("%w: %w", ErrChangeRefused, err)
		}
		if err := setPath(candidate.Fields, path, value); err != nil {
			return Detail{}, fmt.Errorf("%w: %w", ErrChangeRefused, err)
		}
	}
	resolved, err := CompileSchema(t.Schema)
	if err != nil {
		return Detail{}, err
	}
	if err := ValidateDetail(resolved, candidate); err != nil {
		return Detail{}, fmt.Errorf("%w: %w", ErrChangeRefused, err)
	}
	policies := Policies(t.Schema)
	if err := CheckModification(base, candidate, policies, party); err != nil {
		return Detail{}, fmt.Errorf("%w: %w", ErrChangeRefused, err)
	}
	if err := CheckFieldLimits(policies, candidate); err != nil {
		return Detail{}, fmt.Errorf("%w: %w", ErrChangeRefused, err)
	}
	return candidate, nil
}

// checkOverlappingPaths refuses changes where one path lies inside another, since the order they are applied in
// would decide the result.
func checkOverlappingPaths(changes map[string]json.RawMessage) error {
	for path := range changes {
		for other := range changes {
			if path != other && strings.HasPrefix(other, path+".") {
				return fmt.Errorf("the changes to %s and %s overlap", path, other)
			}
		}
	}
	return nil
}
