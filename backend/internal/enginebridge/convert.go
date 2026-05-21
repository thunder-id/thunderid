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
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/idp"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

func providerError(err error) *serviceerror.ServiceError {
	if err == nil {
		return nil
	}
	return &serviceerror.InternalServerError
}

func toInboundOAuthClient(client *thunderidengine.OAuthClient) *inboundmodel.OAuthClient {
	if client == nil {
		return nil
	}
	grantTypes := make([]oauth2const.GrantType, len(client.GrantTypes))
	for i, g := range client.GrantTypes {
		grantTypes[i] = oauth2const.GrantType(g)
	}
	responseTypes := make([]oauth2const.ResponseType, len(client.ResponseTypes))
	for i, r := range client.ResponseTypes {
		responseTypes[i] = oauth2const.ResponseType(r)
	}
	return &inboundmodel.OAuthClient{
		ID:                                 client.ID,
		OUID:                               client.OUID,
		ClientID:                           client.ClientID,
		RedirectURIs:                       client.RedirectURIs,
		GrantTypes:                         grantTypes,
		ResponseTypes:                      responseTypes,
		TokenEndpointAuthMethod:            oauth2const.TokenEndpointAuthMethod(client.TokenEndpointAuthMethod),
		PKCERequired:                       client.PKCERequired,
		PublicClient:                       client.PublicClient,
		RequirePushedAuthorizationRequests: client.RequirePushedAuthorizationRequests,
		Scopes:                             client.Scopes,
		AcrValues:                          client.AcrValues,
	}
}

func toEntityGroups(groups []thunderidengine.EntityGroup) []entityprovider.EntityGroup {
	if len(groups) == 0 {
		return nil
	}
	out := make([]entityprovider.EntityGroup, len(groups))
	for i, g := range groups {
		out[i] = entityprovider.EntityGroup{
			ID:   g.ID,
			Name: g.Name,
			OUID: g.OUID,
		}
	}
	return out
}

func toInternalResource(res *thunderidengine.Resource) *resource.Resource {
	if res == nil {
		return nil
	}
	var parent *string
	if res.ParentID != "" {
		parent = &res.ParentID
	}
	return &resource.Resource{
		ID:          res.ID,
		Name:        res.Name,
		Handle:      res.Handle,
		Description: res.Description,
		Parent:      parent,
		Permission:  res.Permission,
	}
}

func toOrganizationUnit(ouModel *thunderidengine.OrganizationUnit) ou.OrganizationUnit {
	if ouModel == nil {
		return ou.OrganizationUnit{}
	}
	var parent *string
	if ouModel.ParentID != "" {
		parent = &ouModel.ParentID
	}
	return ou.OrganizationUnit{
		ID:          ouModel.ID,
		Handle:      ouModel.Handle,
		Name:        ouModel.Name,
		Description: ouModel.Description,
		Parent:      parent,
		ThemeID:     ouModel.ThemeID,
		LayoutID:    ouModel.LayoutID,
	}
}

func toIDPDTO(idpModel *thunderidengine.IdentityProvider) *idp.IDPDTO {
	if idpModel == nil {
		return nil
	}
	props := make([]cmodels.Property, 0, len(idpModel.Properties))
	for key, value := range idpModel.Properties {
		prop, propErr := cmodels.NewProperty(key, value, false)
		if propErr != nil {
			continue
		}
		props = append(props, *prop)
	}
	return &idp.IDPDTO{
		ID:          idpModel.ID,
		Name:        idpModel.Name,
		Description: idpModel.Description,
		Type:        idp.IDPType(idpModel.Type),
		Properties:  props,
	}
}

func attributesToResponse(values map[string]interface{}) *authnprovidercm.AttributesResponse {
	if len(values) == 0 {
		return &authnprovidercm.AttributesResponse{}
	}
	attrs := make(map[string]*authnprovidercm.AttributeResponse, len(values))
	for key, value := range values {
		attrs[key] = &authnprovidercm.AttributeResponse{Value: value}
	}
	return &authnprovidercm.AttributesResponse{Attributes: attrs}
}

func toPublicEvent(evt *event.Event) *thunderidengine.Event {
	if evt == nil {
		return nil
	}
	return &thunderidengine.Event{
		TraceID:   evt.TraceID,
		EventID:   evt.EventID,
		Type:      evt.Type,
		Timestamp: evt.Timestamp,
		Component: evt.Component,
		Status:    evt.Status,
		Data:      evt.Data,
	}
}
