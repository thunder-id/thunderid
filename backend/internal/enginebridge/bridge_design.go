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

	"github.com/thunder-id/thunderid/internal/design/common"
	"github.com/thunder-id/thunderid/internal/design/resolve"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

type designBridge struct {
	provider thunderidengine.DesignProvider
}

func newDesignBridge(provider thunderidengine.DesignProvider) *designBridge {
	return &designBridge{provider: provider}
}

func (b *designBridge) ResolveDesign(
	ctx context.Context, resolveType common.DesignResolveType, id string,
) (*common.DesignResponse, *serviceerror.ServiceError) {
	if b.provider == nil {
		return nil, &serviceerror.InternalServerError
	}
	resp, err := b.provider.ResolveDesign(ctx, string(resolveType), id)
	if err != nil {
		return nil, &serviceerror.InternalServerError
	}
	if resp == nil {
		return nil, nil
	}
	return &common.DesignResponse{
		Theme:  resp.Theme,
		Layout: resp.Layout,
	}, nil
}

var _ resolve.DesignResolveServiceInterface = (*designBridge)(nil)
