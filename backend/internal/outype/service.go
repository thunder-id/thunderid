// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package outype manages organization unit types. It wraps the entity type service, which stores the
// types and runs the checks every entity type shares, and adds the checks only organization unit types
// have and errors of their own.
package outype

import (
	"context"
	"encoding/json"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/entitytype/model"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// OUTypeServiceInterface defines the interface for the organization unit type service.
type OUTypeServiceInterface interface {
	GetOUTypeList(ctx context.Context, limit, offset int, includeDisplay bool) (
		*OUTypeListResponse, *tidcommon.ServiceError)
	CreateOUType(ctx context.Context, request CreateOUTypeRequest) (*OUType, *tidcommon.ServiceError)
	GetOUType(ctx context.Context, id string, includeDisplay bool) (*OUType, *tidcommon.ServiceError)
	UpdateOUType(ctx context.Context, id string, request UpdateOUTypeRequest) (*OUType, *tidcommon.ServiceError)
	DeleteOUType(ctx context.Context, id string) *tidcommon.ServiceError
}

// ouTypeService is the default implementation of OUTypeServiceInterface.
type ouTypeService struct {
	entityTypeService entitytype.EntityTypeServiceInterface
	logger            *log.Logger
}

// newOUTypeService creates a new organization unit type service over the entity type service.
func newOUTypeService(entityTypeService entitytype.EntityTypeServiceInterface) OUTypeServiceInterface {
	return &ouTypeService{
		entityTypeService: entityTypeService,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "OUTypeService")),
	}
}

// GetOUTypeList returns a page of the organization unit types the caller may view.
func (s *ouTypeService) GetOUTypeList(ctx context.Context, limit, offset int, includeDisplay bool) (
	*OUTypeListResponse, *tidcommon.ServiceError) {
	list, svcErr := s.entityTypeService.GetEntityTypeList(
		ctx, entitytype.TypeCategoryOU, limit, offset, includeDisplay)
	if svcErr != nil {
		return nil, s.toOUTypeError(ctx, svcErr)
	}

	types := make([]OUTypeListItem, 0, len(list.Types))
	for _, item := range list.Types {
		types = append(types, OUTypeListItem{
			ID:               item.ID,
			Handle:           item.Handle,
			DisplayName:      item.DisplayName,
			OUID:             item.OUID,
			OUHandle:         item.OUHandle,
			SystemAttributes: toOUTypeSystemAttributes(item.SystemAttributes),
			IsReadOnly:       item.IsReadOnly,
		})
	}
	return &OUTypeListResponse{
		TotalResults: list.TotalResults,
		StartIndex:   list.StartIndex,
		Count:        list.Count,
		Types:        types,
		Links:        list.Links,
	}, nil
}

// CreateOUType creates an organization unit type.
func (s *ouTypeService) CreateOUType(ctx context.Context, request CreateOUTypeRequest) (
	*OUType, *tidcommon.ServiceError) {
	if svcErr := s.validateNoCredentialAttributes(ctx, request.Schema); svcErr != nil {
		return nil, svcErr
	}

	created, svcErr := s.entityTypeService.CreateEntityType(ctx, entitytype.TypeCategoryOU,
		entitytype.CreateEntityTypeRequestWithID{
			Handle:           request.Handle,
			DisplayName:      request.DisplayName,
			OUID:             request.OUID,
			SystemAttributes: toEntityTypeSystemAttributes(request.SystemAttributes),
			Schema:           request.Schema,
		})
	if svcErr != nil {
		return nil, s.toOUTypeError(ctx, svcErr)
	}
	return toOUType(created), nil
}

// GetOUType returns an organization unit type.
func (s *ouTypeService) GetOUType(ctx context.Context, id string, includeDisplay bool) (
	*OUType, *tidcommon.ServiceError) {
	entityType, svcErr := s.entityTypeService.GetEntityType(ctx, entitytype.TypeCategoryOU, id, includeDisplay)
	if svcErr != nil {
		return nil, s.toOUTypeError(ctx, svcErr)
	}
	return toOUType(entityType), nil
}

// UpdateOUType replaces the display name, organization unit and schema of an organization unit type.
func (s *ouTypeService) UpdateOUType(ctx context.Context, id string, request UpdateOUTypeRequest) (
	*OUType, *tidcommon.ServiceError) {
	if svcErr := s.validateNoCredentialAttributes(ctx, request.Schema); svcErr != nil {
		return nil, svcErr
	}

	updated, svcErr := s.entityTypeService.UpdateEntityType(ctx, entitytype.TypeCategoryOU, id,
		entitytype.UpdateEntityTypeRequest{
			Handle:           request.Handle,
			DisplayName:      request.DisplayName,
			OUID:             request.OUID,
			SystemAttributes: toEntityTypeSystemAttributes(request.SystemAttributes),
			Schema:           request.Schema,
		})
	if svcErr != nil {
		return nil, s.toOUTypeError(ctx, svcErr)
	}
	return toOUType(updated), nil
}

// DeleteOUType deletes an organization unit type. Deleting a type that does not exist succeeds.
func (s *ouTypeService) DeleteOUType(ctx context.Context, id string) *tidcommon.ServiceError {
	if svcErr := s.entityTypeService.DeleteEntityType(ctx, entitytype.TypeCategoryOU, id); svcErr != nil {
		return s.toOUTypeError(ctx, svcErr)
	}
	return nil
}

// validateNoCredentialAttributes refuses a schema that declares a credential attribute at any depth. A
// schema that does not compile is left to the entity type service, which reports why.
func (s *ouTypeService) validateNoCredentialAttributes(
	ctx context.Context, schema json.RawMessage) *tidcommon.ServiceError {
	compiled, err := model.CompileSchema(schema)
	if err != nil {
		return nil
	}
	if compiled.HasCredentialAttribute() {
		s.logger.Debug(ctx, "Organization unit type declares a credential attribute")
		return &ErrorCredentialAttributeNotAllowed
	}
	return nil
}

// toOUTypeError maps an error of the entity type service to the organization unit type error that
// describes it. The caller authorization error and server errors keep their own codes.
func (s *ouTypeService) toOUTypeError(
	ctx context.Context, svcErr *tidcommon.ServiceError) *tidcommon.ServiceError {
	if svcErr.Type != tidcommon.ClientErrorType {
		return &tidcommon.InternalServerError
	}

	switch svcErr.Code {
	case tidcommon.ErrorUnauthorized.Code:
		return svcErr
	case entitytype.ErrorEntityTypeNotFound.Code:
		return &ErrorOUTypeNotFound
	case entitytype.ErrorEntityTypeHandleConflict.Code:
		return &ErrorOUTypeHandleConflict
	case entitytype.ErrorInvalidEntityTypeRequest.Code:
		// The entity type service appends the cause to its description after a colon.
		_, detail, _ := strings.Cut(svcErr.ErrorDescription.DefaultValue, ": ")
		return invalidOUTypeRequestErr(detail)
	case entitytype.ErrorInvalidLimit.Code:
		return &ErrorInvalidLimit
	case entitytype.ErrorInvalidOffset.Code:
		return &ErrorInvalidOffset
	case entitytype.ErrorInvalidEntityTypeHandle.Code:
		return &ErrorInvalidOUTypeHandle
	case entitytype.ErrorEntityTypeHandleUpdateNotAllowed.Code:
		return &ErrorOUTypeHandleUpdateNotAllowed
	case entitytype.ErrorCannotModifyDeclarativeResource.Code:
		return &ErrorCannotModifyDeclarativeOUType
	case entitytype.ErrorManagedEntityType.Code:
		return &ErrorCannotModifyManagedOUType
	default:
		// The remaining errors describe an invalid schema or display attribute in their description.
		s.logger.Debug(ctx, "Mapped an entity type error to an invalid organization unit type request",
			log.String("code", svcErr.Code))
		return invalidOUTypeRequestErr(svcErr.ErrorDescription.DefaultValue)
	}
}

// toOUType converts an entity type to an organization unit type.
func toOUType(entityType *entitytype.EntityType) *OUType {
	return &OUType{
		ID:               entityType.ID,
		Handle:           entityType.Handle,
		DisplayName:      entityType.DisplayName,
		OUID:             entityType.OUID,
		OUHandle:         entityType.OUHandle,
		SystemAttributes: toOUTypeSystemAttributes(entityType.SystemAttributes),
		Schema:           entityType.Schema,
		IsReadOnly:       entityType.IsReadOnly,
	}
}

// toOUTypeSystemAttributes keeps the system attributes an organization unit type supports.
func toOUTypeSystemAttributes(attrs *entitytype.SystemAttributes) *SystemAttributes {
	if attrs == nil || attrs.Display == "" {
		return nil
	}
	return &SystemAttributes{Display: attrs.Display}
}

// toEntityTypeSystemAttributes converts the system attributes of a request to those of an entity type.
func toEntityTypeSystemAttributes(attrs *SystemAttributes) *entitytype.SystemAttributes {
	if attrs == nil {
		return nil
	}
	return &entitytype.SystemAttributes{Display: attrs.Display}
}
