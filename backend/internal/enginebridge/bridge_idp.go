/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package enginebridge

import (
	"context"

	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

type idpBridge struct {
	provider thunderidengine.IDPProvider
}

func newIDPBridge(provider thunderidengine.IDPProvider) *idpBridge {
	return &idpBridge{provider: provider}
}

func (b *idpBridge) CreateIdentityProvider(_ context.Context, _ *idp.IDPDTO) (*idp.IDPDTO, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *idpBridge) GetIdentityProviderList(_ context.Context) ([]idp.BasicIDPDTO, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *idpBridge) GetIdentityProvider(ctx context.Context, id string) (*idp.IDPDTO, *serviceerror.ServiceError) {
	if b.provider == nil {
		return nil, &serviceerror.InternalServerError
	}
	idpModel, err := b.provider.GetIDPByID(ctx, id)
	if err != nil {
		return nil, providerError(err)
	}
	if idpModel == nil {
		return nil, &idp.ErrorIDPNotFound
	}
	return toIDPDTO(idpModel), nil
}

func (b *idpBridge) GetIdentityProviderByName(
	ctx context.Context, name string,
) (*idp.IDPDTO, *serviceerror.ServiceError) {
	if b.provider == nil {
		return nil, &serviceerror.InternalServerError
	}
	idpModel, err := b.provider.GetIDPByName(ctx, name)
	if err != nil {
		return nil, providerError(err)
	}
	if idpModel == nil {
		return nil, &idp.ErrorIDPNotFound
	}
	return toIDPDTO(idpModel), nil
}

func (b *idpBridge) GetIdentityProviderByIssuer(_ context.Context, _ string) (*idp.IDPDTO, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *idpBridge) UpdateIdentityProvider(
	_ context.Context, _ string, _ *idp.IDPDTO,
) (*idp.IDPDTO, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *idpBridge) DeleteIdentityProvider(_ context.Context, _ string) *serviceerror.ServiceError {
	return &serviceerror.InternalServerError
}

var _ idp.IDPServiceInterface = (*idpBridge)(nil)
