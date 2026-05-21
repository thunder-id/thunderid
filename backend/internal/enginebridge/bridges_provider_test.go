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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/authz"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

type stubClientProvider struct {
	client *thunderidengine.OAuthClient
	groups []thunderidengine.EntityGroup
}

func (s *stubClientProvider) GetOAuthClientByClientID(
	_ context.Context, _ string,
) (*thunderidengine.OAuthClient, error) {
	return s.client, nil
}

func (s *stubClientProvider) GetTransitiveEntityGroups(
	_ context.Context, _ string,
) ([]thunderidengine.EntityGroup, error) {
	return s.groups, nil
}

func (s *stubClientProvider) GetApplicationByID(_ context.Context, _ string) (*thunderidengine.Application, error) {
	return nil, nil
}

type stubAuthzProvider struct {
	granted map[string]bool
}

func (s *stubAuthzProvider) IsAuthorized(_ context.Context, subjectID, action, _ string) (bool, error) {
	key := subjectID + ":" + action
	return s.granted[key], nil
}

func TestClientBridgeGetOAuthClientByClientID(t *testing.T) {
	bridge := newClientBridge(&stubClientProvider{client: &thunderidengine.OAuthClient{
		ID:         "ent-1",
		ClientID:   "client-1",
		GrantTypes: []string{"authorization_code"},
	}})
	got, err := bridge.GetOAuthClientByClientID(context.Background(), "client-1")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "ent-1", got.ID)
	require.Equal(t, "client-1", got.ClientID)
}

func TestEntityBridgeGetTransitiveEntityGroups(t *testing.T) {
	bridge := newEntityBridge(&stubClientProvider{groups: []thunderidengine.EntityGroup{{ID: "g1", Name: "admins"}}})
	groups, err := bridge.GetTransitiveEntityGroups("ent-1")
	require.Nil(t, err)
	require.Len(t, groups, 1)
	require.Equal(t, "g1", groups[0].ID)
}

func TestEntityBridgeIdentifyEntityByClientID(t *testing.T) {
	bridge := newEntityBridge(&stubClientProvider{client: &thunderidengine.OAuthClient{ID: "ent-1", ClientID: "cid"}})
	id, err := bridge.IdentifyEntity(map[string]interface{}{"clientId": "cid"})
	require.Nil(t, err)
	require.NotNil(t, id)
	require.Equal(t, "ent-1", *id)
}

func TestAuthzBridgeGetAuthorizedPermissions(t *testing.T) {
	bridge := newAuthzBridge(&stubAuthzProvider{granted: map[string]bool{
		"user-1:read":   true,
		"user-1:write":  false,
		"group-1:write": true,
	}})
	resp, svcErr := bridge.GetAuthorizedPermissions(context.Background(), authz.GetAuthorizedPermissionsRequest{
		EntityID:             "user-1",
		GroupIDs:             []string{"group-1"},
		RequestedPermissions: []string{"read", "write"},
	})
	require.Nil(t, svcErr)
	require.Equal(t, []string{"read", "write"}, resp.AuthorizedPermissions)
}

func TestResourceBridgeGetResource(t *testing.T) {
	bridge := newResourceBridge(resourceProviderFunc(func(
		_ context.Context, uri string,
	) (*thunderidengine.Resource, error) {
		require.Equal(t, "rs-1/res-1", uri)
		return &thunderidengine.Resource{ID: "res-1", Name: "Resource"}, nil
	}))
	got, svcErr := bridge.GetResource(context.Background(), "rs-1", "res-1")
	require.Nil(t, svcErr)
	require.Equal(t, "res-1", got.ID)
}

func TestOUBridgeGetOrganizationUnit(t *testing.T) {
	bridge := newOUBridge(ouProviderFunc(func(
		_ context.Context, ouID string,
	) (*thunderidengine.OrganizationUnit, error) {
		return &thunderidengine.OrganizationUnit{ID: ouID, Name: "Root"}, nil
	}))
	got, svcErr := bridge.GetOrganizationUnit(context.Background(), "ou-1")
	require.Nil(t, svcErr)
	require.Equal(t, "ou-1", got.ID)
}

func TestIDPBridgeGetIdentityProvider(t *testing.T) {
	bridge := newIDPBridge(idpProviderFunc(func(
		_ context.Context, id string,
	) (*thunderidengine.IdentityProvider, error) {
		return &thunderidengine.IdentityProvider{ID: id, Name: "default", Type: "LOCAL"}, nil
	}))
	got, svcErr := bridge.GetIdentityProvider(context.Background(), "idp-1")
	require.Nil(t, svcErr)
	require.Equal(t, "idp-1", got.ID)
	require.Equal(t, idp.IDPType("LOCAL"), got.Type)
}

type resourceProviderFunc func(context.Context, string) (*thunderidengine.Resource, error)

func (f resourceProviderFunc) GetResource(ctx context.Context, resourceURI string) (*thunderidengine.Resource, error) {
	return f(ctx, resourceURI)
}

type ouProviderFunc func(context.Context, string) (*thunderidengine.OrganizationUnit, error)

func (f ouProviderFunc) GetOU(ctx context.Context, ouID string) (*thunderidengine.OrganizationUnit, error) {
	return f(ctx, ouID)
}

func (f ouProviderFunc) GetOUAncestors(context.Context, string) ([]thunderidengine.OrganizationUnit, error) {
	return nil, nil
}

type idpProviderFunc func(context.Context, string) (*thunderidengine.IdentityProvider, error)

func (f idpProviderFunc) GetIDPByID(ctx context.Context, id string) (*thunderidengine.IdentityProvider, error) {
	return f(ctx, id)
}

func (f idpProviderFunc) GetIDPByName(context.Context, string) (*thunderidengine.IdentityProvider, error) {
	return nil, nil
}

var (
	_ entityprovider.EntityProviderInterface = (*entityBridge)(nil)
	_ resource.ResourceServiceInterface      = (*resourceBridge)(nil)
)
