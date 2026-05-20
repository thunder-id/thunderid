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
	"strings"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// IDPServiceInterface defines the interface for the IdP service.
type IDPServiceInterface interface {
	CreateIdentityProvider(ctx context.Context, idp *IDPDTO) (*IDPDTO, *serviceerror.ServiceError)
	GetIdentityProviderList(ctx context.Context) ([]BasicIDPDTO, *serviceerror.ServiceError)
	GetIdentityProvider(ctx context.Context, idpID string) (*IDPDTO, *serviceerror.ServiceError)
	GetIdentityProviderByName(ctx context.Context, idpName string) (*IDPDTO, *serviceerror.ServiceError)
	GetIdentityProviderByIssuer(ctx context.Context, issuer string) (*IDPDTO, *serviceerror.ServiceError)
	UpdateIdentityProvider(ctx context.Context, idpID string, idp *IDPDTO) (*IDPDTO, *serviceerror.ServiceError)
	DeleteIdentityProvider(ctx context.Context, idpID string) *serviceerror.ServiceError
}

// idpService is the default implementation of the IdPServiceInterface.
type idpService struct {
	idpStore      idpStoreInterface
	transactioner transaction.Transactioner
	logger        *log.Logger
}

// newIDPService creates a new instance of IdPService.
func newIDPService(idpStore idpStoreInterface, transactioner transaction.Transactioner) IDPServiceInterface {
	return &idpService{
		idpStore:      idpStore,
		transactioner: transactioner,
		logger:        log.GetLogger().With(log.String(log.LoggerKeyComponentName, "IdPService")),
	}
}

// CreateIdentityProvider creates a new Identity Provider.
func (is *idpService) CreateIdentityProvider(
	ctx context.Context, idp *IDPDTO) (*IDPDTO, *serviceerror.ServiceError) {
	logger := is.logger
	if isDeclarativeModeEnabled() {
		return nil, &declarativeresource.ErrorDeclarativeResourceCreateOperation
	}

	if svcErr := validateIDP(idp, logger); svcErr != nil {
		return nil, svcErr
	}

	id, err := utils.GenerateUUIDv7()
	if err != nil {
		logger.Error("failed to generate ID for identity provider", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	idp.ID = id

	var svcErr *serviceerror.ServiceError
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
		logger.Error("Failed to create identity provider", log.Error(err), log.String("idpName", idp.Name))
		return nil, &serviceerror.InternalServerError
	}

	return idp, nil
}

// GetIdentityProviderList retrieves the list of all Identity Providers.
func (is *idpService) GetIdentityProviderList(ctx context.Context) ([]BasicIDPDTO, *serviceerror.ServiceError) {
	logger := is.logger
	idps, err := is.idpStore.GetIdentityProviderList(ctx)
	if err != nil {
		if errors.Is(err, ErrResultLimitExceededInCompositeMode) {
			return nil, &ErrorResultLimitExceededInCompositeMode
		}
		logger.Error("Failed to get identity provider list", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	return idps, nil
}

// GetIdentityProvider retrieves an identity provider by its ID.
func (is *idpService) GetIdentityProvider(ctx context.Context, idpID string) (*IDPDTO, *serviceerror.ServiceError) {
	logger := is.logger
	if strings.TrimSpace(idpID) == "" {
		return nil, &ErrorInvalidIDPID
	}

	idp, err := is.idpStore.GetIdentityProvider(ctx, idpID)
	if err != nil {
		if errors.Is(err, ErrIDPNotFound) {
			return nil, &ErrorIDPNotFound
		}
		logger.Error("Failed to get identity provider", log.String("idpID", idpID), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	return idp, nil
}

// GetIdentityProviderByName retrieves an identity provider by its name.
func (is *idpService) GetIdentityProviderByName(ctx context.Context,
	idpName string) (*IDPDTO, *serviceerror.ServiceError) {
	logger := is.logger
	if strings.TrimSpace(idpName) == "" {
		return nil, &ErrorInvalidIDPName
	}

	idp, err := is.idpStore.GetIdentityProviderByName(ctx, idpName)
	if err != nil {
		if errors.Is(err, ErrIDPNotFound) {
			return nil, &ErrorIDPNotFound
		}
		logger.Error("Failed to get identity provider by name", log.String("idpName", idpName), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	return idp, nil
}

// GetIdentityProviderByIssuer retrieves an identity provider by its issuer property.
func (is *idpService) GetIdentityProviderByIssuer(ctx context.Context,
	issuer string) (*IDPDTO, *serviceerror.ServiceError) {
	logger := is.logger
	if strings.TrimSpace(issuer) == "" {
		return nil, &ErrorInvalidIDPID
	}

	idp, err := is.idpStore.GetIdentityProviderByIssuer(ctx, issuer)
	if err != nil {
		if errors.Is(err, ErrIDPNotFound) {
			return nil, &ErrorIDPNotFound
		}
		logger.Error("Failed to get identity provider by issuer", log.String("issuer", issuer), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	return idp, nil
}

// UpdateIdentityProvider updates an existing Identity Provider.
func (is *idpService) UpdateIdentityProvider(ctx context.Context, idpID string, idp *IDPDTO) (*IDPDTO,
	*serviceerror.ServiceError) {
	logger := is.logger
	// Block updates only in declarative-only mode; allow in composite and mutable modes
	// In composite mode, the store will check if the resource is immutable and return appropriate error
	if isDeclarativeModeEnabled() {
		return nil, &declarativeresource.ErrorDeclarativeResourceUpdateOperation
	}

	if strings.TrimSpace(idpID) == "" {
		return nil, &ErrorInvalidIDPID
	}
	if svcErr := validateIDP(idp, logger); svcErr != nil {
		return nil, svcErr
	}

	idp.ID = idpID
	var svcErr *serviceerror.ServiceError
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
		logger.Error("Failed to update identity provider", log.Error(err), log.String("idpID", idpID))
		return nil, &serviceerror.InternalServerError
	}

	return idp, nil
}

// DeleteIdentityProvider deletes an identity provider.
func (is *idpService) DeleteIdentityProvider(ctx context.Context, idpID string) *serviceerror.ServiceError {
	logger := is.logger
	// Block deletes only in declarative-only mode; allow in composite and mutable modes
	// In composite mode, the store will check if the resource is immutable and return appropriate error
	if isDeclarativeModeEnabled() {
		return &declarativeresource.ErrorDeclarativeResourceDeleteOperation
	}

	if strings.TrimSpace(idpID) == "" {
		return &ErrorInvalidIDPID
	}

	var svcErr *serviceerror.ServiceError
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
		logger.Error("Failed to delete identity provider", log.Error(err), log.String("idpID", idpID))
		return &serviceerror.InternalServerError
	}

	return nil
}
