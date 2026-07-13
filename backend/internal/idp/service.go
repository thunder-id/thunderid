/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

// Package idp provides the implementation for identity provider management operations.
package idp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/entitytype"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	"github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// IDPServiceInterface defines the interface for the IdP service.
type IDPServiceInterface interface {
	CreateIdentityProvider(ctx context.Context, idp *providers.IDPDTO) (*providers.IDPDTO, *tidcommon.ServiceError)
	GetIdentityProviderList(ctx context.Context) ([]BasicIDPDTO, *tidcommon.ServiceError)
	GetIdentityProvider(ctx context.Context, idpID string) (*providers.IDPDTO, *tidcommon.ServiceError)
	GetIdentityProviderByName(ctx context.Context, idpName string) (*providers.IDPDTO, *tidcommon.ServiceError)
	GetIdentityProvidersByProperty(ctx context.Context, propertyKey,
		propertyValue string) ([]providers.IDPDTO, *tidcommon.ServiceError)
	UpdateIdentityProvider(
		ctx context.Context,
		idpID string,
		idp *providers.IDPDTO,
	) (*providers.IDPDTO, *tidcommon.ServiceError)
	DeleteIdentityProvider(ctx context.Context, idpID string) *tidcommon.ServiceError
	GetIDPUsages(ctx context.Context, idpID string) (*resourcedependency.DependenciesResponse, *tidcommon.ServiceError)
	SetDependencyRegistry(r resourcedependency.Registry)
}

// idpService is the default implementation of the IdPServiceInterface.
type idpService struct {
	idpStore           idpStoreInterface
	entityTypeService  entitytype.EntityTypeServiceInterface
	transactioner      transaction.Transactioner
	dependencyRegistry resourcedependency.Registry
	logger             *log.Logger
	uuidGenerator      func() (string, error)
}

// newIDPService creates a new instance of IdPService.
func newIDPService(idpStore idpStoreInterface, entityTypeService entitytype.EntityTypeServiceInterface,
	transactioner transaction.Transactioner) IDPServiceInterface {
	return &idpService{
		idpStore:          idpStore,
		entityTypeService: entityTypeService,
		transactioner:     transactioner,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "IdPService")),
		uuidGenerator:     utils.GenerateUUIDv7,
	}
}

// CreateIdentityProvider creates a new Identity Provider.
func (is *idpService) CreateIdentityProvider(
	ctx context.Context, idp *providers.IDPDTO) (*providers.IDPDTO, *tidcommon.ServiceError) {
	logger := is.logger
	if isDeclarativeModeEnabled() {
		return nil, &declarativeresource.ErrorDeclarativeResourceCreateOperation
	}

	if svcErr := validateIDP(ctx, idp, logger); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := is.validateAttributeConfiguration(ctx, idp); svcErr != nil {
		return nil, svcErr
	}

	if idp.ID == "" {
		id, genErr := is.uuidGenerator()
		if genErr != nil {
			logger.Error(ctx, "failed to generate ID for identity provider", log.Error(genErr))
			return nil, &tidcommon.InternalServerError
		}
		idp.ID = id
	}

	var (
		err    error
		svcErr *tidcommon.ServiceError
	)
	err = is.transactioner.Transact(ctx, func(txCtx context.Context) error {
		// Check if an identity provider with the same name already exists
		existingIDP, err := is.idpStore.GetIdentityProviderByName(txCtx, idp.Name)
		if err != nil && !errors.Is(err, ErrIDPNotFound) {
			return err
		}
		if existingIDP != nil {
			svcErr = &ErrorIDPAlreadyExists
			return errors.New("identity provider already exists")
		}

		// Create the IdP in the database.
		err = is.idpStore.CreateIdentityProvider(txCtx, *idp)
		if err != nil {
			return err
		}
		return nil
	})

	if svcErr != nil {
		return nil, svcErr
	}
	if err != nil {
		logger.Error(ctx, "Failed to create identity provider",
			log.Error(err), log.String("idpName", idp.Name))
		return nil, &tidcommon.InternalServerError
	}

	return idp, nil
}

// GetIdentityProviderList retrieves the list of all Identity Providers.
func (is *idpService) GetIdentityProviderList(ctx context.Context) ([]BasicIDPDTO, *tidcommon.ServiceError) {
	logger := is.logger
	idps, err := is.idpStore.GetIdentityProviderList(ctx)
	if err != nil {
		if errors.Is(err, ErrResultLimitExceededInCompositeMode) {
			return nil, &ErrorResultLimitExceededInCompositeMode
		}
		logger.Error(ctx, "Failed to get identity provider list", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return idps, nil
}

// GetIdentityProvider retrieves an identity provider by its ID.
func (is *idpService) GetIdentityProvider(
	ctx context.Context,
	idpID string,
) (*providers.IDPDTO, *tidcommon.ServiceError) {
	logger := is.logger
	if strings.TrimSpace(idpID) == "" {
		return nil, &ErrorInvalidIDPID
	}

	idp, err := is.idpStore.GetIdentityProvider(ctx, idpID)
	if err != nil {
		if errors.Is(err, ErrIDPNotFound) {
			return nil, &ErrorIDPNotFound
		}
		logger.Error(ctx, "Failed to get identity provider", log.String("idpID", idpID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return idp, nil
}

// GetIdentityProviderByName retrieves an identity provider by its name.
func (is *idpService) GetIdentityProviderByName(ctx context.Context,
	idpName string) (*providers.IDPDTO, *tidcommon.ServiceError) {
	logger := is.logger
	if strings.TrimSpace(idpName) == "" {
		return nil, &ErrorInvalidIDPName
	}

	idp, err := is.idpStore.GetIdentityProviderByName(ctx, idpName)
	if err != nil {
		if errors.Is(err, ErrIDPNotFound) {
			return nil, &ErrorIDPNotFound
		}
		logger.Error(ctx, "Failed to get identity provider by name",
			log.String("idpName", idpName), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return idp, nil
}

// GetIdentityProvidersByProperty retrieves identity providers matching a given property key and value.
func (is *idpService) GetIdentityProvidersByProperty(ctx context.Context,
	propertyKey, propertyValue string) ([]providers.IDPDTO, *tidcommon.ServiceError) {
	logger := is.logger
	if strings.TrimSpace(propertyKey) == "" || strings.TrimSpace(propertyValue) == "" {
		return nil, &ErrorInvalidIDPID
	}

	idps, err := is.idpStore.GetIdentityProvidersByProperty(ctx, propertyKey, propertyValue)
	if err != nil {
		if errors.Is(err, ErrIDPNotFound) {
			return nil, &ErrorIDPNotFound
		}
		logger.Error(ctx, "Failed to get identity providers by property",
			log.String("propertyKey", propertyKey),
			log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return idps, nil
}

// UpdateIdentityProvider updates an existing Identity Provider.
func (is *idpService) UpdateIdentityProvider(
	ctx context.Context,
	idpID string,
	idp *providers.IDPDTO,
) (*providers.IDPDTO,
	*tidcommon.ServiceError) {
	logger := is.logger
	// Block updates only in declarative-only mode; allow in composite and mutable modes
	// In composite mode, the store will check if the resource is immutable and return appropriate error
	if isDeclarativeModeEnabled() {
		return nil, &declarativeresource.ErrorDeclarativeResourceUpdateOperation
	}

	if strings.TrimSpace(idpID) == "" {
		return nil, &ErrorInvalidIDPID
	}
	if svcErr := validateIDP(ctx, idp, logger); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := is.validateAttributeConfiguration(ctx, idp); svcErr != nil {
		return nil, svcErr
	}

	idp.ID = idpID
	var svcErr *tidcommon.ServiceError
	err := is.transactioner.Transact(ctx, func(txCtx context.Context) error {
		// Check if the identity provider exists
		existingIDP, err := is.idpStore.GetIdentityProvider(txCtx, idpID)
		if err != nil {
			if errors.Is(err, ErrIDPNotFound) {
				svcErr = &ErrorIDPNotFound
				return err
			}
			return err
		}

		// If the name is being updated, check whether another IdP with the same name exists
		if existingIDP.Name != idp.Name {
			existingIDPByName, err := is.idpStore.GetIdentityProviderByName(txCtx, idp.Name)
			if err != nil && !errors.Is(err, ErrIDPNotFound) {
				return err
			}
			if existingIDPByName != nil {
				svcErr = &ErrorIDPAlreadyExists
				return errors.New("identity provider already exists")
			}
		}

		err = is.idpStore.UpdateIdentityProvider(txCtx, idp)
		if err != nil {
			// Check if it's the immutable error from composite store
			if errors.Is(err, ErrIDPIsImmutable) {
				svcErr = &ErrorIDPDeclarativeReadOnly
				return err
			}
			return err
		}
		return nil
	})

	if svcErr != nil {
		return nil, svcErr
	}
	if err != nil {
		logger.Error(ctx, "Failed to update identity provider", log.Error(err), log.String("idpID", idpID))
		return nil, &tidcommon.InternalServerError
	}

	return idp, nil
}

// DeleteIdentityProvider deletes an identity provider.
func (is *idpService) DeleteIdentityProvider(ctx context.Context, idpID string) *tidcommon.ServiceError {
	logger := is.logger
	// Block deletes only in declarative-only mode; allow in composite and mutable modes
	// In composite mode, the store will check if the resource is immutable and return appropriate error
	if isDeclarativeModeEnabled() {
		return &declarativeresource.ErrorDeclarativeResourceDeleteOperation
	}

	if strings.TrimSpace(idpID) == "" {
		return &ErrorInvalidIDPID
	}

	// Refuse deletion while other resources block it (e.g. flows that reference the identity provider).
	if svcErr := is.ensureNoBlockingDependencies(ctx, idpID); svcErr != nil {
		return svcErr
	}

	var svcErr *tidcommon.ServiceError
	err := is.transactioner.Transact(ctx, func(txCtx context.Context) error {
		// Check if the identity provider exists
		_, err := is.idpStore.GetIdentityProvider(txCtx, idpID)
		if err != nil {
			if errors.Is(err, ErrIDPNotFound) {
				return nil
			}
			return err
		}

		err = is.idpStore.DeleteIdentityProvider(txCtx, idpID)
		if err != nil {
			// Check if it's the immutable error from composite store
			if errors.Is(err, ErrIDPIsImmutable) {
				svcErr = &ErrorIDPDeclarativeReadOnly
				return err
			}
			return err
		}
		return nil
	})

	if svcErr != nil {
		return svcErr
	}
	if err != nil {
		logger.Error(ctx, "Failed to delete identity provider", log.Error(err), log.String("idpID", idpID))
		return &tidcommon.InternalServerError
	}

	return nil
}

// SetDependencyRegistry injects the dependency registry. Called by servicemanager after the
// provider services are initialized to avoid a cyclic import.
func (is *idpService) SetDependencyRegistry(r resourcedependency.Registry) {
	is.dependencyRegistry = r
}

// GetIDPUsages returns the resources that reference this identity provider, such as flows that use
// it. It is informational — it drives the pre-delete confirmation dialog and does not gate deletion
// on the server (deletion is gated separately by ensureNoBlockingDependencies).
func (is *idpService) GetIDPUsages(
	ctx context.Context, idpID string,
) (*resourcedependency.DependenciesResponse, *tidcommon.ServiceError) {
	if strings.TrimSpace(idpID) == "" {
		return nil, &ErrorInvalidIDPID
	}

	if _, err := is.idpStore.GetIdentityProvider(ctx, idpID); err != nil {
		if errors.Is(err, ErrIDPNotFound) {
			return nil, &ErrorIDPNotFound
		}
		is.logger.Error(ctx, "Failed to retrieve identity provider", log.Error(err), log.String("idpID", idpID))
		return nil, &tidcommon.InternalServerError
	}

	if is.dependencyRegistry == nil {
		is.logger.Warn(ctx, "Dependency registry not set; returning unknown dependencies",
			log.String("idpID", idpID))
		return &resourcedependency.DependenciesResponse{
			TotalResults: nil,
			Count:        0,
			Summary:      nil,
			Usages:       []resourcedependency.ResourceDependency{},
		}, nil
	}

	result, err := is.dependencyRegistry.GetDependencies(ctx, resourcedependency.ResourceTypeIDP, idpID)
	if err != nil {
		is.logger.Error(ctx, "Failed to get identity provider usages", log.Error(err), log.String("idpID", idpID))
		return nil, &tidcommon.InternalServerError
	}

	return result, nil
}

// ensureNoBlockingDependencies refuses deletion when other resources depend on the identity provider
// in a way that forbids it (behaviorOnDelete == restrict), such as flows that reference it. Because
// deletion is destructive, it fails closed: if dependency data cannot be determined, the deletion is
// refused rather than allowed.
func (is *idpService) ensureNoBlockingDependencies(ctx context.Context, idpID string) *tidcommon.ServiceError {
	if is.dependencyRegistry == nil {
		is.logger.Error(ctx, "Dependency registry not set; refusing to delete identity provider",
			log.String("idpID", idpID))
		return &tidcommon.InternalServerError
	}

	deps, err := is.dependencyRegistry.GetDependencies(ctx, resourcedependency.ResourceTypeIDP, idpID)
	if err != nil {
		is.logger.Error(ctx, "Failed to evaluate identity provider dependencies",
			log.Error(err), log.String("idpID", idpID))
		return &tidcommon.InternalServerError
	}
	// Fail closed: nil TotalResults means a provider failed to report, so usage is unknown.
	if deps == nil || deps.TotalResults == nil {
		is.logger.Error(ctx, "Identity provider dependency data unavailable; refusing to delete",
			log.String("idpID", idpID))
		return &tidcommon.InternalServerError
	}

	blocking := resourcedependency.BlockingUsages(deps)
	if len(blocking) == 0 {
		return nil
	}

	is.logger.Debug(ctx, "Identity provider has blocking dependencies; deletion refused",
		log.String("idpID", idpID), log.Int("blockingCount", len(blocking)))
	return tidcommon.CustomServiceError(ErrorIDPHasBlockingDependencies, tidcommon.I18nMessage{
		Key: "error.idpservice.idp_has_blocking_dependencies_description",
		DefaultValue: fmt.Sprintf(
			"The identity provider cannot be deleted because %s depend on it. Remove or reassign them first.",
			resourcedependency.SummarizeBlockingUsages(blocking)),
	})
}

// validateAttributeConfiguration validates the IDP's attribute configuration: a default user type is
// required only when user-type attribute mappings are configured (it selects which mapping profile
// applies), and for each user type's attributes a valid claim-mapping shape with every local (target)
// claim a non-credential attribute defined in that user type's schema. No-op when no profile is
// configured.
func (is *idpService) validateAttributeConfiguration(
	ctx context.Context,
	idp *providers.IDPDTO,
) *tidcommon.ServiceError {
	profile := idp.AttributeConfiguration
	if profile == nil {
		return nil
	}
	if len(profile.UserTypeAttributeMappings) > 0 &&
		(profile.UserTypeResolution == nil || strings.TrimSpace(profile.UserTypeResolution.Default) == "") {
		return tidcommon.CustomServiceError(ErrorInvalidAttributeConfiguration, tidcommon.I18nMessage{
			Key:          "error.idpservice.attribute_configuration_user_type_required_description",
			DefaultValue: "attribute configuration requires an user type",
		})
	}

	seenUserTypes := make(map[string]bool, len(profile.UserTypeAttributeMappings))
	for i := range profile.UserTypeAttributeMappings {
		entry := profile.UserTypeAttributeMappings[i]
		if strings.TrimSpace(entry.UserType) == "" {
			return tidcommon.CustomServiceError(ErrorInvalidAttributeConfiguration, tidcommon.I18nMessage{
				Key:          "error.idpservice.attribute_configuration_entry_user_type_required_description",
				DefaultValue: "each user type attributes entry requires an user type",
			})
		}
		if seenUserTypes[entry.UserType] {
			return tidcommon.CustomServiceError(ErrorInvalidAttributeConfiguration, tidcommon.I18nMessage{
				Key:          "error.idpservice.attribute_configuration_duplicate_user_type_description",
				DefaultValue: "user type '{{param(userType)}}' is configured more than once",
				Params:       map[string]string{"userType": entry.UserType},
			})
		}
		seenUserTypes[entry.UserType] = true

		if len(entry.Attributes) > 0 {
			if svcErr := validateAttributeMappingShape(entry.Attributes); svcErr != nil {
				return svcErr
			}
		}

		// Local targets must be non-credential attributes defined in the user type's schema.
		attributes, svcErr := is.entityTypeService.GetAttributes(
			ctx, entitytype.TypeCategoryUser, entry.UserType, false, true, false)
		if svcErr != nil {
			return tidcommon.CustomServiceError(ErrorInvalidAttributeConfiguration, tidcommon.I18nMessage{
				Key: "error.idpservice.attribute_configuration_user_type_invalid_description",
				DefaultValue: fmt.Sprintf("invalid user type '%s' for attribute configuration: %s",
					entry.UserType, svcErr.ErrorDescription.DefaultValue),
			})
		}
		validTargets := make(map[string]bool, len(attributes))
		for _, attr := range attributes {
			validTargets[attr.Attribute] = true
		}
		for _, m := range entry.Attributes {
			if !validTargets[m.LocalAttribute] {
				return tidcommon.CustomServiceError(ErrorInvalidAttributeConfiguration, tidcommon.I18nMessage{
					Key: "error.idpservice.attribute_configuration_target_not_in_schema_description",
					DefaultValue: "local claim '{{param(claim)}}' is not an attribute of " +
						"user type '{{param(userType)}}'",
					Params: map[string]string{"claim": m.LocalAttribute, "userType": entry.UserType},
				})
			}
		}
	}
	return nil
}
