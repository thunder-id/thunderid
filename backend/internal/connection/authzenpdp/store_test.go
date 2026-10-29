// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzenpdp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

const testDeploymentID = "test-deployment-id"

func TestAuthZENPDPConnectionSettingsRoundTripThroughProperties(t *testing.T) {
	expected := AuthZENPDPConnection{
		ID: "pdp-1", Name: "PDP", Description: "Test connection",
		Endpoint:      "https://pdp.example.com/evaluation",
		BatchEndpoint: "https://pdp.example.com/evaluations",
		TimeoutMS:     1500, RetryCount: 0,
		SubjectAttributeMappings: []SubjectAttributeMapping{{
			EntityType: "Customer",
			Attributes: []SubjectAttributeRow{{Attribute: "status", PDPAttribute: "account_status"}},
		}},
	}
	expected.AuthenticationProperties = testAuthProperties(t, outboundauth.Authentication{
		Type: string(outboundauth.TypeBearer), Properties: map[string]string{"token": "secret"},
	})
	properties, err := encodeAuthZENPDPProperties(expected)
	require.NoError(t, err)
	require.NotContains(t, properties, `"authenticationScheme"`)
	require.Contains(t, properties, `"authentication_type"`)
	actual, err := (&authZENPDPStore{}).buildAuthZENPDPConnection(map[string]interface{}{
		"id": []byte(expected.ID), "name": expected.Name, "description": expected.Description,
		"properties": []byte(properties),
	})
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestBuildAuthZENPDPConnectionDoesNotApplyDefaults(t *testing.T) {
	store := &authZENPDPStore{}
	for _, raw := range []string{`{}`, `{"timeoutMs":-1,"retryCount":-1}`} {
		t.Run(raw, func(t *testing.T) {
			connection, err := store.buildAuthZENPDPConnection(map[string]interface{}{"properties": raw})
			require.NoError(t, err)
			if raw == `{}` {
				require.Zero(t, connection.TimeoutMS)
				require.Zero(t, connection.RetryCount)
				return
			}
			require.Equal(t, -1, connection.TimeoutMS)
			require.Equal(t, -1, connection.RetryCount)
		})
	}
}

func TestBuildAuthZENPDPConnectionRejectsMalformedProperties(t *testing.T) {
	for _, raw := range []string{"", "{", `{"timeoutMs":"invalid"}`} {
		_, err := (&authZENPDPStore{}).buildAuthZENPDPConnection(map[string]interface{}{"properties": raw})
		require.Error(t, err)
	}
}

func TestStringValueSupportsDatabaseValueTypes(t *testing.T) {
	require.Equal(t, "value", stringValue("value"))
	require.Equal(t, "value", stringValue([]byte("value")))
	require.Empty(t, stringValue(42))
}

type AuthZENPDPStoreTestSuite struct {
	suite.Suite
	ctx        context.Context
	mockClient *providermock.DBClientInterfaceMock
	store      *authZENPDPStore
}

func TestAuthZENPDPStoreTestSuite(t *testing.T) {
	suite.Run(t, new(AuthZENPDPStoreTestSuite))
}

func (s *AuthZENPDPStoreTestSuite) SetupTest() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime("", &config.Config{
		Server: engineconfig.ServerConfig{Identifier: testDeploymentID},
	}))
	s.T().Cleanup(config.ResetServerRuntime)
	s.ctx = context.Background()
	s.mockClient = providermock.NewDBClientInterfaceMock(s.T())
	mockProvider := providermock.NewDBProviderInterfaceMock(s.T())
	mockProvider.On("GetConfigDBClient").Return(s.mockClient, nil)
	s.store = &authZENPDPStore{dbProvider: mockProvider}
}

// row is a stored connection owned by "ou-1".
func (s *AuthZENPDPStoreTestSuite) row() map[string]interface{} {
	return map[string]interface{}{
		"id": "pdp-1", "name": "PDP", "description": "Test connection", "ou_id": "ou-1",
		"properties": `{"endpoint":"https://pdp.example.com/evaluation"}`,
	}
}

func (s *AuthZENPDPStoreTestSuite) TestCreatePassesOUID() {
	s.mockClient.On("ExecuteContext", s.ctx, queryCreateAuthZENPDPConnection,
		"pdp-1", "PDP", "Test connection", authZENPDPConnectionType, "ou-1", mock.Anything, testDeploymentID).
		Return(int64(1), nil)

	err := s.store.CreateAuthZENPDP(s.ctx, AuthZENPDPConnection{
		ID: "pdp-1", Name: "PDP", Description: "Test connection", OUID: "ou-1",
		Endpoint: "https://pdp.example.com/evaluation",
	})

	s.Require().NoError(err)
}

func (s *AuthZENPDPStoreTestSuite) TestGetReadsOUID() {
	s.mockClient.On("QueryContext", s.ctx, queryGetAuthZENPDPConnectionByID,
		"pdp-1", authZENPDPConnectionType, testDeploymentID).
		Return([]map[string]interface{}{s.row()}, nil)

	connection, err := s.store.GetAuthZENPDP(s.ctx, "pdp-1")

	s.Require().NoError(err)
	s.Require().NotNil(connection)
	s.Equal("ou-1", connection.OUID)
}

func (s *AuthZENPDPStoreTestSuite) TestGetByNameReadsOUID() {
	s.mockClient.On("QueryContext", s.ctx, queryGetAuthZENPDPConnectionByName,
		"PDP", authZENPDPConnectionType, testDeploymentID).
		Return([]map[string]interface{}{s.row()}, nil)

	connection, err := s.store.GetAuthZENPDPByName(s.ctx, "PDP")

	s.Require().NoError(err)
	s.Require().NotNil(connection)
	s.Equal("ou-1", connection.OUID)
}

func (s *AuthZENPDPStoreTestSuite) TestListReadsOUID() {
	s.mockClient.On("QueryContext", s.ctx, queryListAuthZENPDPConnections,
		authZENPDPConnectionType, testDeploymentID).
		Return([]map[string]interface{}{s.row()}, nil)

	connections, err := s.store.ListAuthZENPDPs(s.ctx)

	s.Require().NoError(err)
	s.Require().Len(connections, 1)
	s.Equal("ou-1", connections[0].OUID)
}
