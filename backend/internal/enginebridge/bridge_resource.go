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

	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

type resourceBridge struct {
	provider thunderidengine.ResourceProvider
}

func newResourceBridge(provider thunderidengine.ResourceProvider) *resourceBridge {
	return &resourceBridge{provider: provider}
}

func (b *resourceBridge) CreateResourceServer(
	_ context.Context, _ resource.ResourceServer,
) (*resource.ResourceServer, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *resourceBridge) GetResourceServer(
	_ context.Context, _ string,
) (*resource.ResourceServer, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *resourceBridge) GetResourceServerList(
	_ context.Context, _, _ int,
) (*resource.ResourceServerList, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *resourceBridge) UpdateResourceServer(
	_ context.Context, _ string, _ resource.ResourceServer,
) (*resource.ResourceServer, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *resourceBridge) DeleteResourceServer(_ context.Context, _ string) *serviceerror.ServiceError {
	return &serviceerror.InternalServerError
}

func (b *resourceBridge) GetResourceServerByIdentifier(
	_ context.Context, _ string,
) (*resource.ResourceServer, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *resourceBridge) IsResourceServerDeclarative(_ string) bool {
	return false
}

func (b *resourceBridge) CreateResource(
	_ context.Context, _ string, _ resource.Resource,
) (*resource.Resource, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *resourceBridge) GetResource(ctx context.Context, resourceServerID, resourceID string) (
	*resource.Resource, *serviceerror.ServiceError,
) {
	if b.provider == nil {
		return nil, &serviceerror.InternalServerError
	}
	resourceURI := resourceServerID + "/" + resourceID
	res, err := b.provider.GetResource(ctx, resourceURI)
	if err != nil {
		return nil, providerError(err)
	}
	if res == nil {
		return nil, &resource.ErrorResourceNotFound
	}
	return toInternalResource(res), nil
}

func (b *resourceBridge) GetResourceList(
	_ context.Context, _ string, _ *string, _, _ int,
) (*resource.ResourceList, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *resourceBridge) UpdateResource(
	_ context.Context, _, _ string, _ resource.Resource,
) (*resource.Resource, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *resourceBridge) DeleteResource(_ context.Context, _, _ string) *serviceerror.ServiceError {
	return &serviceerror.InternalServerError
}

func (b *resourceBridge) CreateAction(
	_ context.Context, _ string, _ *string, _ resource.Action,
) (*resource.Action, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *resourceBridge) GetAction(
	_ context.Context, _ string, _ *string, _ string,
) (*resource.Action, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *resourceBridge) GetActionList(
	_ context.Context, _ string, _ *string, _, _ int,
) (*resource.ActionList, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *resourceBridge) UpdateAction(
	_ context.Context, _ string, _ *string, _ string, _ resource.Action,
) (*resource.Action, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *resourceBridge) DeleteAction(_ context.Context, _ string, _ *string, _ string) *serviceerror.ServiceError {
	return &serviceerror.InternalServerError
}

func (b *resourceBridge) ValidatePermissions(
	_ context.Context, _ string, _ []string,
) ([]string, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *resourceBridge) FindResourceServersByPermissions(
	_ context.Context, _ []string,
) ([]resource.ResourceServer, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *resourceBridge) ResolveResourceServerOUHandle(
	_ context.Context, _ *resource.ResourceServer,
) *serviceerror.ServiceError {
	return &serviceerror.InternalServerError
}

var _ resource.ResourceServiceInterface = (*resourceBridge)(nil)
