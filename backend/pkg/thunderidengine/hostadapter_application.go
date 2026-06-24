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

package thunderidengine

import (
	"context"
	"errors"

	"github.com/thunder-id/thunderid/internal/application"
	"github.com/thunder-id/thunderid/internal/application/model"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/host"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/runtime"
)

// applicationAdapter implements application.ApplicationServiceInterface for design resolution
// by delegating GetApplication to a host.ActorProvider. Other methods are unsupported.
type applicationAdapter struct {
	h host.ActorProvider
}

func newApplicationAdapter(h host.ActorProvider) application.ApplicationServiceInterface {
	return &applicationAdapter{h: h}
}

func (a *applicationAdapter) GetApplication(
	ctx context.Context, appID string,
) (*model.Application, *serviceerror.ServiceError) {
	if appID == "" {
		return nil, &application.ErrorInvalidApplicationID
	}
	app, err := a.h.GetApplication(ctx, appID)
	if err != nil {
		if errors.Is(err, runtime.ErrNotFound) {
			return nil, &application.ErrorApplicationNotFound
		}
		return nil, &serviceerror.InternalServerError
	}
	if app == nil {
		return nil, &application.ErrorApplicationNotFound
	}
	return &model.Application{
		ID:   app.ID,
		Name: app.Name,
		OUID: app.OUID,
		InboundAuthProfile: inboundmodel.InboundAuthProfile{
			ThemeID:  app.ThemeID,
			LayoutID: app.LayoutID,
		},
	}, nil
}

func (*applicationAdapter) CreateApplication(context.Context, *model.ApplicationDTO) (
	*model.ApplicationDTO, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (*applicationAdapter) ValidateApplication(context.Context, *model.ApplicationDTO) (
	*model.ApplicationProcessedDTO, *inboundmodel.InboundAuthConfigWithSecret, *serviceerror.ServiceError) {
	return nil, nil, &serviceerror.InternalServerError
}

func (*applicationAdapter) GetApplicationList(context.Context) (
	*model.ApplicationListResponse, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (*applicationAdapter) GetOAuthApplication(context.Context, string) (
	*inboundmodel.OAuthClient, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (*applicationAdapter) UpdateApplication(context.Context, string, *model.ApplicationDTO) (
	*model.ApplicationDTO, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (*applicationAdapter) DeleteApplication(context.Context, string) *serviceerror.ServiceError {
	return &serviceerror.InternalServerError
}
