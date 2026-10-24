// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/ou"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// TestIDPBackedConnectionsCarryOU checks the organization unit goes into the stored model on create and comes back out
// on read, for every identity-provider-backed connection type.
func TestIDPBackedConnectionsCarryOU(t *testing.T) {
	const ouID = "ou-1"
	cases := []struct {
		name   string
		toDTO  func() (*providers.IDPDTO, error)
		fromFn func(providers.IDPDTO) (string, error)
	}{
		{"google", func() (*providers.IDPDTO, error) {
			return googleToIDPDTO(googleConnectionRequest{Name: "n", OUID: ouID, ClientID: "c",
				RedirectURI: "https://app/cb"})
		}, func(d providers.IDPDTO) (string, error) { r, err := googleFromIDPDTO(d); return r.OUID, err }},
		{"github", func() (*providers.IDPDTO, error) {
			return githubToIDPDTO(githubConnectionRequest{Name: "n", OUID: ouID, ClientID: "c",
				RedirectURI: "https://app/cb"})
		}, func(d providers.IDPDTO) (string, error) { r, err := githubFromIDPDTO(d); return r.OUID, err }},
		{"oidc", func() (*providers.IDPDTO, error) {
			return oidcToIDPDTO(oidcConnectionRequest{Name: "n", OUID: ouID, ClientID: "c",
				RedirectURI: "https://app/cb", AuthorizationEndpoint: "https://idp/auth",
				TokenEndpoint: "https://idp/token"})
		}, func(d providers.IDPDTO) (string, error) { r, err := oidcFromIDPDTO(d); return r.OUID, err }},
		{"oauth", func() (*providers.IDPDTO, error) {
			return oauthToIDPDTO(oauthConnectionRequest{Name: "n", OUID: ouID, ClientID: "c",
				RedirectURI: "https://app/cb", AuthorizationEndpoint: "https://idp/auth",
				TokenEndpoint:    "https://idp/token",
				UserInfoEndpoint: "https://idp/userinfo"})
		}, func(d providers.IDPDTO) (string, error) { r, err := oauthFromIDPDTO(d); return r.OUID, err }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dto, err := tc.toDTO()
			require.NoError(t, err)
			require.Equal(t, ouID, dto.OUID)

			got, err := tc.fromFn(*dto)
			require.NoError(t, err)
			require.Equal(t, ouID, got)
		})
	}
}

// TestConnectionModelCarriesOU checks the declarative and export model keeps the organization unit both ways.
func TestConnectionModelCarriesOU(t *testing.T) {
	dto := providers.IDPDTO{ID: "google-1", Name: "My Google", Type: providers.IDPTypeGoogle, OUID: "ou-1"}

	model, err := connectionModelFromIDPDTO(dto)
	require.NoError(t, err)
	require.Equal(t, "ou-1", model.OUID)

	back, sender, err := connectionModelToDTO(model)
	require.NoError(t, err)
	require.Nil(t, sender)
	require.Equal(t, "ou-1", back.OUID)
}

// TestListInstancesCarriesOU checks an identity provider's organization unit is listed with it.
func (s *ServiceTestSuite) TestListInstancesCarriesOU() {
	s.mockIDP.On("GetIdentityProviderList", mock.Anything).Return([]idp.BasicIDPDTO{
		{ID: "1", Name: "Owned", Type: providers.IDPTypeOIDC, OUID: "ou-1"},
		{ID: "2", Name: "Unowned", Type: providers.IDPTypeOIDC},
	}, (*tidcommon.ServiceError)(nil))

	got, svcErr := s.svc.listInstances(context.Background(), categoryIdentityProvider, serverconst.DefaultPageSize, 0)

	s.Nil(svcErr)
	s.Require().Len(got.Connections, 2)
	byID := map[string]connectionInstance{}
	for _, c := range got.Connections {
		byID[c.ID] = c
	}
	s.Equal("ou-1", byID["1"].OUID)
	s.Empty(byID["2"].OUID)
}

// ouResolverStub resolves organization unit handles from a fixed map.
type ouResolverStub struct {
	byHandle map[string]string
}

func (r ouResolverStub) GetOrganizationUnitByPath(
	_ context.Context, handlePath string,
) (ou.OrganizationUnit, *tidcommon.ServiceError) {
	if id, ok := r.byHandle[handlePath]; ok {
		return ou.OrganizationUnit{ID: id, Handle: handlePath}, nil
	}
	return ou.OrganizationUnit{}, &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "OU-1003"}
}

// TestResolveOUHandle resolves a handle named by a declarative connection to its id.
func TestResolveOUHandle(t *testing.T) {
	resolver := ouResolverStub{byHandle: map[string]string{"acme": "ou-acme"}}

	t.Run("resolves the handle", func(t *testing.T) {
		dto := &providers.IDPDTO{ID: "idp-1", OUHandle: "acme"}
		require.NoError(t, resolveOUHandle(dto, resolver))
		require.Equal(t, "ou-acme", dto.OUID)
	})
	t.Run("keeps the id when both are given", func(t *testing.T) {
		dto := &providers.IDPDTO{ID: "idp-1", OUID: "ou-given", OUHandle: "acme"}
		require.NoError(t, resolveOUHandle(dto, resolver))
		require.Equal(t, "ou-given", dto.OUID)
	})
	t.Run("does nothing without a handle", func(t *testing.T) {
		dto := &providers.IDPDTO{ID: "idp-1"}
		require.NoError(t, resolveOUHandle(dto, nil))
		require.Empty(t, dto.OUID)
	})
	t.Run("fails for a handle that does not exist", func(t *testing.T) {
		dto := &providers.IDPDTO{ID: "idp-1", OUHandle: "missing"}
		require.Error(t, resolveOUHandle(dto, resolver))
	})
	t.Run("fails for a handle when no resolver is available", func(t *testing.T) {
		dto := &providers.IDPDTO{ID: "idp-1", OUHandle: "acme"}
		require.Error(t, resolveOUHandle(dto, nil))
	})
}

// TestConnectionDocumentOUHandle reads ouHandle from a connection document, and exports the owner by id.
func TestConnectionDocumentOUHandle(t *testing.T) {
	var node yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(
		"id: idp-1\ntype: google\nname: G\nouHandle: acme\nclientId: c\nredirectUri: https://app/cb\n"),
		&node))

	dto, _, err := ParseConnectionFromNode(&node)
	require.NoError(t, err)
	require.NotNil(t, dto)
	require.Equal(t, "acme", dto.OUHandle)
	require.Empty(t, dto.OUID)

	exported, err := connectionModelFromIDPDTO(providers.IDPDTO{
		ID: "idp-1", Name: "G", Type: providers.IDPTypeGoogle, OUID: "ou-acme", OUHandle: "acme",
	})
	require.NoError(t, err)
	require.Equal(t, "ou-acme", exported.OUID)
	require.Empty(t, exported.OUHandle)
}
