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

package inboundclient

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

const (
	testServerID = "test-server-id"
	testEntityID = "entity-1"
)

type mockTransactioner struct{}

func (m *mockTransactioner) Transact(ctx context.Context, operation func(txCtx context.Context) error) error {
	return operation(ctx)
}

// InboundClientStoreTestSuite contains tests for the inboundclient store helper functions and CRUD operations.
type InboundClientStoreTestSuite struct {
	suite.Suite
	mockDBProvider *providermock.DBProviderInterfaceMock
	mockDBClient   *providermock.DBClientInterfaceMock
	store          *store
}

func TestInboundClientStoreTestSuite(t *testing.T) {
	suite.Run(t, new(InboundClientStoreTestSuite))
}

func (suite *InboundClientStoreTestSuite) SetupTest() {
	_ = config.InitializeServerRuntime("test", &config.Config{})
	suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
	suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
	suite.store = &store{
		dbProvider:   suite.mockDBProvider,
		deploymentID: testServerID,
	}
}

func (suite *InboundClientStoreTestSuite) TestNewStore() {
	mockClient := providermock.NewDBClientInterfaceMock(suite.T())
	mockClient.On("QueryContext", mock.Anything, queryGetInboundClientCount, mock.Anything).
		Return([]map[string]interface{}{{"total": int64(0)}}, nil)
	mockProvider := providermock.NewDBProviderInterfaceMock(suite.T())
	mockProvider.On("GetConfigDBClient").Return(mockClient, nil)
	mockProvider.On("GetConfigDBTransactioner").Return(&mockTransactioner{}, nil)
	originalGetDBProvider := getDBProvider
	getDBProvider = func() provider.DBProviderInterface { return mockProvider }
	defer func() { getDBProvider = originalGetDBProvider }()

	st, _, err := newStore()

	suite.NoError(err)
	suite.NotNil(st)
	suite.IsType(&store{}, st)
}

// --- Tests for buildInboundClientFromRow ---

func (suite *InboundClientStoreTestSuite) TestBuildInboundClientFromRow_Success() {
	blob := inboundClientJSONBlob{
		Assertion: &inboundmodel.AssertionConfig{
			ValidityPeriod: 3600,
			UserAttributes: []string{"email", "name"},
		},
		LoginConsent: &inboundmodel.LoginConsentConfig{
			ValidityPeriod: 5400,
		},
		AllowedUserTypes: []string{"admin", "user"},
		Properties:       map[string]interface{}{"template": "spa"},
	}
	blobBytes, _ := json.Marshal(blob)

	row := map[string]interface{}{
		"entity_id":                    "app1",
		"auth_flow_id":                 "auth_flow_1",
		"registration_flow_id":         "reg_flow_1",
		"is_registration_flow_enabled": "1",
		"recovery_flow_id":             "recovery_flow_1",
		"is_recovery_flow_enabled":     "1",
		"theme_id":                     "theme-123",
		"layout_id":                    "layout-456",
		"properties":                   string(blobBytes),
	}

	result, err := buildInboundClientFromRow(row)

	suite.NoError(err)
	suite.NotNil(result)
	suite.Equal("app1", result.ID)
	suite.Equal("auth_flow_1", result.AuthFlowID)
	suite.Equal("reg_flow_1", result.RegistrationFlowID)
	suite.True(result.IsRegistrationFlowEnabled)
	suite.Equal("recovery_flow_1", result.RecoveryFlowID)
	suite.True(result.IsRecoveryFlowEnabled)
	suite.Equal("theme-123", result.ThemeID)
	suite.Equal("layout-456", result.LayoutID)
	suite.NotNil(result.Assertion)
	suite.Equal(int64(3600), result.Assertion.ValidityPeriod)
	suite.NotNil(result.LoginConsent)
	suite.Equal(int64(5400), result.LoginConsent.ValidityPeriod)
	suite.Equal([]string{"admin", "user"}, result.AllowedUserTypes)
	suite.NotNil(result.Properties)
	suite.Equal("spa", result.Properties["template"])
}

func (suite *InboundClientStoreTestSuite) TestBuildInboundClientFromRow_InvalidID() {
	row := map[string]interface{}{
		"entity_id": 123, // Invalid type
	}

	result, err := buildInboundClientFromRow(row)

	suite.Error(err)
	suite.Nil(result)
	suite.Contains(err.Error(), "failed to parse entity_id as string")
}

func (suite *InboundClientStoreTestSuite) TestBuildInboundClientFromRow_MinimalRow() {
	row := map[string]interface{}{
		"entity_id":                    "app1",
		"auth_flow_id":                 nil,
		"registration_flow_id":         nil,
		"is_registration_flow_enabled": nil,
		"recovery_flow_id":             nil,
		"is_recovery_flow_enabled":     nil,
		"theme_id":                     nil,
		"layout_id":                    nil,
		"properties":                   nil,
	}

	result, err := buildInboundClientFromRow(row)

	suite.NoError(err)
	suite.NotNil(result)
	suite.Equal("app1", result.ID)
	suite.Equal("", result.AuthFlowID)
	suite.Equal("", result.RegistrationFlowID)
	suite.False(result.IsRegistrationFlowEnabled)
	suite.Equal("", result.RecoveryFlowID)
	suite.False(result.IsRecoveryFlowEnabled)
	suite.Nil(result.Assertion)
	suite.Nil(result.LoginConsent)
	suite.Nil(result.AllowedUserTypes)
	suite.Nil(result.Properties)
}

// --- Tests for buildOAuthProfileFromRow ---

func (suite *InboundClientStoreTestSuite) TestBuildOAuthProfileFromRow_Success() {
	cfg := inboundmodel.OAuthProfile{
		RedirectURIs:            []string{"https://example.com/callback"},
		GrantTypes:              []string{"authorization_code"},
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: "client_secret_post",
		PKCERequired:            true,
	}
	cfgBytes, _ := json.Marshal(cfg)

	row := map[string]interface{}{
		"entity_id":    testEntityID,
		"oauth_config": string(cfgBytes),
	}

	result, err := buildOAuthProfileFromRow(row)

	suite.NoError(err)
	suite.NotNil(result)
	suite.Equal([]string{"https://example.com/callback"}, result.RedirectURIs)
	suite.True(result.PKCERequired)
}

func (suite *InboundClientStoreTestSuite) TestBuildOAuthProfileFromRow_NilOAuthConfig() {
	row := map[string]interface{}{
		"entity_id":    testEntityID,
		"oauth_config": nil,
	}

	result, err := buildOAuthProfileFromRow(row)

	suite.NoError(err)
	suite.Nil(result)
}

func (suite *InboundClientStoreTestSuite) TestBuildOAuthProfileFromRow_MalformedJSON() {
	row := map[string]interface{}{
		"entity_id":    testEntityID,
		"oauth_config": "{invalid json",
	}

	result, err := buildOAuthProfileFromRow(row)

	suite.Error(err)
	suite.Nil(result)
	suite.Contains(err.Error(), "failed to unmarshal OAuth profile JSON")
}

func (suite *InboundClientStoreTestSuite) TestMarshalOAuthProfile_WithAcrValues() {
	profile := &inboundmodel.OAuthProfile{
		RedirectURIs: []string{"https://example.com/callback"},
		GrantTypes:   []string{"authorization_code"},
		AcrValues:    []string{"urn:thunder:acr:password", "urn:thunder:acr:generated-code"},
	}

	data, err := marshalOAuthProfile(profile)

	suite.NoError(err)
	suite.NotNil(data)

	var result map[string]interface{}
	suite.NoError(json.Unmarshal(data, &result))

	acrRaw, ok := result["acrValues"].([]interface{})
	suite.True(ok, "acrValues should be present in JSON")
	suite.Len(acrRaw, 2)
	suite.Equal("urn:thunder:acr:password", acrRaw[0])
	suite.Equal("urn:thunder:acr:generated-code", acrRaw[1])
}

func (suite *InboundClientStoreTestSuite) TestMarshalOAuthProfile_WithEmptyAcrValues() {
	profile := &inboundmodel.OAuthProfile{
		RedirectURIs: []string{"https://example.com/callback"},
		GrantTypes:   []string{"authorization_code"},
		AcrValues:    []string{},
	}

	data, err := marshalOAuthProfile(profile)

	suite.NoError(err)
	var result map[string]interface{}
	suite.NoError(json.Unmarshal(data, &result))

	suite.Nil(result["acrValues"])
}

func (suite *InboundClientStoreTestSuite) TestMarshalOAuthProfile_WithNilAcrValues() {
	profile := &inboundmodel.OAuthProfile{
		RedirectURIs: []string{"https://example.com/callback"},
		AcrValues:    nil,
	}

	data, err := marshalOAuthProfile(profile)

	suite.NoError(err)
	var result map[string]interface{}
	suite.NoError(json.Unmarshal(data, &result))

	suite.Nil(result["acrValues"])
}

func (suite *InboundClientStoreTestSuite) TestBuildOAuthProfileFromRow_WithAcrValues() {
	cfg := inboundmodel.OAuthProfile{
		RedirectURIs:            []string{"https://example.com/callback"},
		GrantTypes:              []string{"authorization_code"},
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: "client_secret_basic",
		AcrValues:               []string{"urn:thunder:acr:password", "urn:thunder:acr:generated-code"},
	}
	cfgBytes, _ := json.Marshal(cfg)

	row := map[string]interface{}{
		"entity_id":    testEntityID,
		"oauth_config": string(cfgBytes),
	}

	result, err := buildOAuthProfileFromRow(row)

	suite.NoError(err)
	suite.Require().NotNil(result)
	suite.Equal(
		[]string{"urn:thunder:acr:password", "urn:thunder:acr:generated-code"},
		result.AcrValues,
	)
}

func (suite *InboundClientStoreTestSuite) TestBuildOAuthProfileFromRow_WithSingleAcrValue() {
	cfg := inboundmodel.OAuthProfile{
		RedirectURIs: []string{"https://example.com/callback"},
		GrantTypes:   []string{"authorization_code"},
		AcrValues:    []string{"urn:thunder:acr:password"},
	}
	cfgBytes, _ := json.Marshal(cfg)

	row := map[string]interface{}{
		"entity_id":    testEntityID,
		"oauth_config": string(cfgBytes),
	}

	result, err := buildOAuthProfileFromRow(row)

	suite.NoError(err)
	suite.Require().NotNil(result)
	suite.Equal([]string{"urn:thunder:acr:password"}, result.AcrValues)
}

func (suite *InboundClientStoreTestSuite) TestBuildOAuthProfileFromRow_WithoutAcrValues() {
	cfg := inboundmodel.OAuthProfile{
		RedirectURIs: []string{"https://example.com/callback"},
		GrantTypes:   []string{"authorization_code"},
	}
	cfgBytes, _ := json.Marshal(cfg)

	row := map[string]interface{}{
		"entity_id":    testEntityID,
		"oauth_config": string(cfgBytes),
	}

	result, err := buildOAuthProfileFromRow(row)

	suite.NoError(err)
	suite.Require().NotNil(result)
	suite.Nil(result.AcrValues)
}

func (suite *InboundClientStoreTestSuite) TestAcrValues_RoundTrip() {
	acrs := []string{"urn:thunder:acr:password", "urn:thunder:acr:generated-code"}

	profile := &inboundmodel.OAuthProfile{
		RedirectURIs:            []string{"https://example.com/callback"},
		GrantTypes:              []string{"authorization_code"},
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: "client_secret_basic",
		AcrValues:               acrs,
	}

	data, err := marshalOAuthProfile(profile)
	suite.NoError(err)

	row := map[string]interface{}{
		"entity_id":    testEntityID,
		"oauth_config": string(data),
	}

	result, err := buildOAuthProfileFromRow(row)
	suite.NoError(err)
	suite.Equal(acrs, result.AcrValues)
}

// --- Helper tests ---

func (suite *InboundClientStoreTestSuite) TestParseBoolFromCount() {
	suite.Run("returns true when count is greater than zero", func() {
		results := []map[string]interface{}{{"count": int64(1)}}
		result, err := parseBoolFromCount(results)
		suite.NoError(err)
		suite.True(result)
	})
	suite.Run("returns false when count is zero", func() {
		results := []map[string]interface{}{{"count": int64(0)}}
		result, err := parseBoolFromCount(results)
		suite.NoError(err)
		suite.False(result)
	})
	suite.Run("returns false when results are empty", func() {
		result, err := parseBoolFromCount([]map[string]interface{}{})
		suite.NoError(err)
		suite.False(result)
	})
	suite.Run("returns error when count is invalid type", func() {
		results := []map[string]interface{}{{"count": "invalid"}}
		result, err := parseBoolFromCount(results)
		suite.Error(err)
		suite.False(result)
		suite.Contains(err.Error(), "failed to parse count from query result")
	})
}

func (suite *InboundClientStoreTestSuite) TestMarshalNullableJSON() {
	suite.Run("returns nil for nil input", func() {
		result, err := marshalNullableJSON(nil)
		suite.NoError(err)
		suite.Nil(result)
	})
	suite.Run("marshals non-nil value", func() {
		result, err := marshalNullableJSON(map[string]string{"key": "value"})
		suite.NoError(err)
		suite.NotNil(result)
	})
}

func (suite *InboundClientStoreTestSuite) TestParseStringColumn() {
	suite.Run("returns string value", func() {
		suite.Equal("value", parseStringColumn(map[string]interface{}{"key": "value"}, "key"))
	})
	suite.Run("returns empty string for nil", func() {
		suite.Equal("", parseStringColumn(map[string]interface{}{"key": nil}, "key"))
	})
	suite.Run("returns empty string for missing key", func() {
		suite.Equal("", parseStringColumn(map[string]interface{}{}, "key"))
	})
	suite.Run("returns empty string for non-string type", func() {
		suite.Equal("", parseStringColumn(map[string]interface{}{"key": 123}, "key"))
	})
}

func (suite *InboundClientStoreTestSuite) TestParseStringOrBytesColumn() {
	suite.Run("returns string value", func() {
		suite.Equal("value", parseStringOrBytesColumn(map[string]interface{}{"key": "value"}, "key"))
	})
	suite.Run("returns string from bytes", func() {
		suite.Equal("value", parseStringOrBytesColumn(map[string]interface{}{"key": []byte("value")}, "key"))
	})
	suite.Run("returns empty string for nil", func() {
		suite.Equal("", parseStringOrBytesColumn(map[string]interface{}{"key": nil}, "key"))
	})
	suite.Run("returns empty string for other types", func() {
		suite.Equal("", parseStringOrBytesColumn(map[string]interface{}{"key": 123}, "key"))
	})
}

func (suite *InboundClientStoreTestSuite) TestParseJSONColumnString() {
	suite.Run("returns string value", func() {
		suite.Equal(`{"key":"value"}`, parseJSONColumnString(map[string]interface{}{"col": `{"key":"value"}`}, "col"))
	})
	suite.Run("returns string from bytes", func() {
		row := map[string]interface{}{"col": []byte(`{"key":"value"}`)}
		suite.Equal(`{"key":"value"}`, parseJSONColumnString(row, "col"))
	})
	suite.Run("returns empty string for nil", func() {
		suite.Equal("", parseJSONColumnString(map[string]interface{}{"col": nil}, "col"))
	})
	suite.Run("returns empty string for missing key", func() {
		suite.Equal("", parseJSONColumnString(map[string]interface{}{}, "col"))
	})
}

// --- CRUD tests ---

func (suite *InboundClientStoreTestSuite) TestInboundClientExists() {
	suite.Run("returns true when profile exists", func() {
		suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil).Once()
		suite.mockDBClient.On("QueryContext", mock.Anything, queryCheckInboundClientExistsByEntityID,
			"existing-app", testServerID).
			Return([]map[string]interface{}{{"count": int64(1)}}, nil).Once()

		exists, err := suite.store.InboundClientExists(context.Background(), "existing-app")
		suite.NoError(err)
		suite.True(exists)
	})

	suite.Run("returns false when profile not found", func() {
		suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil).Once()
		suite.mockDBClient.On("QueryContext", mock.Anything, queryCheckInboundClientExistsByEntityID,
			"non-existent-app", testServerID).
			Return([]map[string]interface{}{{"count": int64(0)}}, nil).Once()

		exists, err := suite.store.InboundClientExists(context.Background(), "non-existent-app")
		suite.NoError(err)
		suite.False(exists)
	})

	suite.Run("returns error when database query fails", func() {
		suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil).Once()
		suite.mockDBClient.On("QueryContext", mock.Anything, queryCheckInboundClientExistsByEntityID,
			"test-app", testServerID).
			Return(nil, errors.New("database connection error")).Once()

		exists, err := suite.store.InboundClientExists(context.Background(), "test-app")
		suite.False(exists)
		suite.Error(err)
	})
}

func (suite *InboundClientStoreTestSuite) TestDeleteProfile() {
	suite.Run("successfully deletes", func() {
		suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil).Once()
		suite.mockDBClient.On("ExecuteContext", mock.Anything, queryDeleteInboundClientByEntityID,
			"app-to-delete", testServerID).Return(int64(1), nil).Once()

		err := suite.store.DeleteInboundClient(context.Background(), "app-to-delete")
		suite.NoError(err)
	})

	suite.Run("returns error when db provider fails", func() {
		suite.mockDBProvider.On("GetConfigDBClient").
			Return(nil, errors.New("db provider unavailable")).Once()

		err := suite.store.DeleteInboundClient(context.Background(), "app-to-delete")
		suite.Error(err)
	})
}

func (suite *InboundClientStoreTestSuite) TestDeleteOAuthProfile() {
	suite.Run("successfully deletes", func() {
		suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil).Once()
		suite.mockDBClient.On("ExecuteContext", mock.Anything, queryDeleteOAuthProfileByEntityID,
			testEntityID, testServerID).Return(int64(1), nil).Once()

		err := suite.store.DeleteOAuthProfile(context.Background(), testEntityID)
		suite.NoError(err)
	})

	suite.Run("returns error when db provider fails", func() {
		suite.mockDBProvider.On("GetConfigDBClient").
			Return(nil, errors.New("db provider unavailable")).Once()

		err := suite.store.DeleteOAuthProfile(context.Background(), testEntityID)
		suite.Error(err)
	})
}

func (suite *InboundClientStoreTestSuite) TestCreateProfile() {
	client := inboundmodel.InboundClient{
		ID:                        "app1",
		AuthFlowID:                "flow_1",
		RegistrationFlowID:        "reg_flow_1",
		IsRegistrationFlowEnabled: true,
	}

	suite.Run("successfully executes", func() {
		suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil).Once()
		suite.mockDBClient.On("ExecuteContext", mock.Anything, queryCreateInboundClient,
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything).
			Return(int64(1), nil).Once()

		err := suite.store.CreateInboundClient(context.Background(), client)
		suite.NoError(err)
	})

	suite.Run("returns error when db provider fails", func() {
		suite.mockDBProvider.On("GetConfigDBClient").
			Return(nil, errors.New("db provider unavailable")).Once()

		err := suite.store.CreateInboundClient(context.Background(), client)
		suite.Error(err)
	})
}

func (suite *InboundClientStoreTestSuite) TestCreateOAuthProfile() {
	suite.Run("successfully executes", func() {
		suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil).Once()
		suite.mockDBClient.On("ExecuteContext", mock.Anything, queryCreateOAuthProfile,
			testEntityID, mock.Anything, testServerID).Return(int64(1), nil).Once()

		err := suite.store.CreateOAuthProfile(context.Background(), testEntityID, &inboundmodel.OAuthProfile{})
		suite.NoError(err)
	})
}

func (suite *InboundClientStoreTestSuite) TestUpdateProfile() {
	client := inboundmodel.InboundClient{ID: "app1", AuthFlowID: "flow_1"}

	suite.Run("successfully executes", func() {
		suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil).Once()
		suite.mockDBClient.On("ExecuteContext", mock.Anything, queryUpdateInboundClientByEntityID,
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything).
			Return(int64(1), nil).Once()

		err := suite.store.UpdateInboundClient(context.Background(), client)
		suite.NoError(err)
	})
}

func (suite *InboundClientStoreTestSuite) TestUpdateOAuthProfile() {
	suite.Run("successfully executes", func() {
		suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil).Once()
		suite.mockDBClient.On("ExecuteContext", mock.Anything, queryUpdateOAuthProfileByEntityID,
			testEntityID, mock.Anything, testServerID).Return(int64(1), nil).Once()

		err := suite.store.UpdateOAuthProfile(context.Background(), testEntityID, &inboundmodel.OAuthProfile{})
		suite.NoError(err)
	})
}

func (suite *InboundClientStoreTestSuite) TestGetTotalInboundClientCount() {
	suite.Run("returns count successfully", func() {
		suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil).Once()
		suite.mockDBClient.On("QueryContext", mock.Anything, queryGetInboundClientCount, testServerID).
			Return([]map[string]interface{}{{"total": int64(5)}}, nil).Once()

		count, err := suite.store.GetTotalInboundClientCount(context.Background())
		suite.NoError(err)
		suite.Equal(5, count)
	})

	suite.Run("returns zero for empty results", func() {
		suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil).Once()
		suite.mockDBClient.On("QueryContext", mock.Anything, queryGetInboundClientCount, testServerID).
			Return([]map[string]interface{}{}, nil).Once()

		count, err := suite.store.GetTotalInboundClientCount(context.Background())
		suite.NoError(err)
		suite.Equal(0, count)
	})
}

func (suite *InboundClientStoreTestSuite) TestGetInboundClientList() {
	suite.Run("returns list of profiles", func() {
		mockRows := []map[string]interface{}{
			{
				"entity_id":                    "app1",
				"auth_flow_id":                 "flow1",
				"registration_flow_id":         "reg1",
				"is_registration_flow_enabled": "1",
				"recovery_flow_id":             "recovery1",
				"is_recovery_flow_enabled":     "1",
				"theme_id":                     nil,
				"layout_id":                    nil,
				"properties":                   nil,
			},
		}
		suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil).Once()
		suite.mockDBClient.On("QueryContext", mock.Anything, queryGetInboundClientList, testServerID, mock.Anything).
			Return(mockRows, nil).Once()

		profiles, err := suite.store.GetInboundClientList(context.Background(), 100)
		suite.NoError(err)
		suite.Len(profiles, 1)
		suite.Equal("app1", profiles[0].ID)
		suite.Equal("recovery1", profiles[0].RecoveryFlowID)
		suite.True(profiles[0].IsRecoveryFlowEnabled)
	})
}

func (suite *InboundClientStoreTestSuite) TestGetInboundClientByEntityID() {
	suite.Run("returns profile when found", func() {
		mockRow := map[string]interface{}{
			"entity_id":                    "app1",
			"auth_flow_id":                 "flow1",
			"registration_flow_id":         "reg1",
			"is_registration_flow_enabled": "1",
			"recovery_flow_id":             "recovery1",
			"is_recovery_flow_enabled":     "1",
			"theme_id":                     nil,
			"layout_id":                    nil,
			"properties":                   nil,
		}
		suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil).Once()
		suite.mockDBClient.On("QueryContext", mock.Anything, queryGetInboundClientByEntityID, "app1", testServerID).
			Return([]map[string]interface{}{mockRow}, nil).Once()

		p, err := suite.store.GetInboundClientByEntityID(context.Background(), "app1")
		suite.NoError(err)
		suite.NotNil(p)
		suite.Equal("app1", p.ID)
		suite.Equal("recovery1", p.RecoveryFlowID)
		suite.True(p.IsRecoveryFlowEnabled)
	})

	suite.Run("returns ErrInboundClientNotFound when not found", func() {
		suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil).Once()
		suite.mockDBClient.On("QueryContext", mock.Anything, queryGetInboundClientByEntityID,
			"non-existent", testServerID).Return([]map[string]interface{}{}, nil).Once()

		p, err := suite.store.GetInboundClientByEntityID(context.Background(), "non-existent")
		suite.Error(err)
		suite.Nil(p)
		suite.ErrorIs(err, ErrInboundClientNotFound)
	})
}

func (suite *InboundClientStoreTestSuite) TestGetOAuthProfileByEntityID() {
	suite.Run("returns OAuth config when found", func() {
		cfg := inboundmodel.OAuthProfile{RedirectURIs: []string{"https://example.com/cb"}, PKCERequired: true}
		cfgBytes, _ := json.Marshal(cfg)
		mockRow := map[string]interface{}{
			"entity_id":    testEntityID,
			"oauth_config": string(cfgBytes),
		}
		suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil).Once()
		suite.mockDBClient.On("QueryContext", mock.Anything, queryGetOAuthProfileByEntityID,
			testEntityID, testServerID).Return([]map[string]interface{}{mockRow}, nil).Once()

		result, err := suite.store.GetOAuthProfileByEntityID(context.Background(), testEntityID)
		suite.NoError(err)
		suite.NotNil(result)
		suite.True(result.PKCERequired)
	})

	suite.Run("returns ErrInboundClientNotFound when not found", func() {
		suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil).Once()
		suite.mockDBClient.On("QueryContext", mock.Anything, queryGetOAuthProfileByEntityID,
			testEntityID, testServerID).Return([]map[string]interface{}{}, nil).Once()

		result, err := suite.store.GetOAuthProfileByEntityID(context.Background(), testEntityID)
		suite.Error(err)
		suite.Nil(result)
		suite.ErrorIs(err, ErrInboundClientNotFound)
	})
}

func (suite *InboundClientStoreTestSuite) TestIsDeclarative_AlwaysFalse() {
	suite.False(suite.store.IsDeclarative(context.Background(), "any-id"))
	suite.False(suite.store.IsDeclarative(context.Background(), ""))
}
