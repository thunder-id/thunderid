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

package idp

import (
	"context"
	"errors"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type idpFileBasedStore struct {
	*declarativeresource.GenericFileBasedStore
}

// Create implements declarativeresource.Storer interface for resource loader
func (f *idpFileBasedStore) Create(id string, data interface{}) error {
	idp := data.(*providers.IDPDTO)
	return f.CreateIdentityProvider(context.Background(), *idp)
}

// CreateIdentityProvider implements idpStoreInterface.
func (f *idpFileBasedStore) CreateIdentityProvider(ctx context.Context, idp providers.IDPDTO) error {
	return f.GenericFileBasedStore.Create(idp.ID, &idp)
}

// DeleteIdentityProvider implements idpStoreInterface.
func (f *idpFileBasedStore) DeleteIdentityProvider(ctx context.Context, id string) error {
	return errors.New("DeleteIdentityProvider is not supported in file-based store")
}

// GetIdentityProvider implements idpStoreInterface.
func (f *idpFileBasedStore) GetIdentityProvider(ctx context.Context, idpID string) (*providers.IDPDTO, error) {
	data, err := f.GenericFileBasedStore.Get(idpID)
	if err != nil {
		return nil, ErrIDPNotFound
	}
	idp, ok := data.(*providers.IDPDTO)
	if !ok {
		declarativeresource.LogTypeAssertionError("identity provider", idpID)
		return nil, errors.New("identity provider data corrupted")
	}
	return idp, nil
}

// GetIdentityProviderByName implements idpStoreInterface.
func (f *idpFileBasedStore) GetIdentityProviderByName(ctx context.Context, idpName string) (*providers.IDPDTO, error) {
	data, err := f.GenericFileBasedStore.GetByField(idpName, func(d interface{}) string {
		return d.(*providers.IDPDTO).Name
	})
	if err != nil {
		return nil, ErrIDPNotFound
	}
	return data.(*providers.IDPDTO), nil
}

// GetIdentityProviderList implements idpStoreInterface.
func (f *idpFileBasedStore) GetIdentityProviderList(ctx context.Context) ([]BasicIDPDTO, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}

	var idpList []BasicIDPDTO
	for _, item := range list {
		if idp, ok := item.Data.(*providers.IDPDTO); ok {
			basicIDP := BasicIDPDTO{
				ID:           idp.ID,
				Name:         idp.Name,
				Description:  idp.Description,
				Type:         idp.Type,
				IDJagEnabled: idJagEnabledFromProperties(idp.Properties),
			}
			idpList = append(idpList, basicIDP)
		}
	}
	return idpList, nil
}

// GetIdentityProvidersByProperty retrieves identity providers matching a property from the file-based store.
func (f *idpFileBasedStore) GetIdentityProvidersByProperty(ctx context.Context,
	propertyKey, propertyValue string) ([]providers.IDPDTO, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}

	var idps []providers.IDPDTO
	for _, item := range list {
		if idpItem, ok := item.Data.(*providers.IDPDTO); ok {
			if GetPropertyValue(idpItem.Properties, propertyKey) == propertyValue {
				idps = append(idps, *idpItem)
			}
		}
	}
	if len(idps) == 0 {
		return nil, ErrIDPNotFound
	}
	return idps, nil
}

// GetIdentityProviderListCount retrieves the total count of identity providers.
func (f *idpFileBasedStore) GetIdentityProviderListCount(ctx context.Context) (int, error) {
	return f.GenericFileBasedStore.Count()
}

// UpdateIdentityProvider implements idpStoreInterface.
func (f *idpFileBasedStore) UpdateIdentityProvider(ctx context.Context, idp *providers.IDPDTO) error {
	return errors.New("UpdateIdentityProvider is not supported in file-based store")
}

// newIDPFileBasedStore creates a new instance of a file-based store.
func newIDPFileBasedStore() (idpStoreInterface, transaction.Transactioner) {
	genericStore := declarativeresource.NewGenericFileBasedStore(entity.KeyTypeIDP)
	return &idpFileBasedStore{
		GenericFileBasedStore: genericStore,
	}, transaction.NewNoOpTransactioner()
}
