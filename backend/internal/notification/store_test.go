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

package notification

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"

	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

const testDeploymentID = "test-deployment-id"

type mockTransactioner struct{}

func (m *mockTransactioner) Transact(ctx context.Context, operation func(txCtx context.Context) error) error {
	return operation(ctx)
}

type StoreTestSuite struct {
	suite.Suite
	mockDBProvider *providermock.DBProviderInterfaceMock
	mockDBClient   *providermock.DBClientInterfaceMock
	store          *notificationStore
}

func TestStoreTestSuite(t *testing.T) {
	suite.Run(t, new(StoreTestSuite))
}

func (suite *StoreTestSuite) SetupSuite() {
	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			Encryption: config.EncryptionConfig{
				Key: "0579f866ac7c9273580d0ff163fa01a7b2401a7ff3ddc3e3b14ae3136fa6025e",
			},
		},
	}
	err := config.InitializeServerRuntime("", testConfig)
	if err != nil {
		suite.T().Fatalf("Failed to initialize server runtime: %v", err)
	}
}

func (suite *StoreTestSuite) SetupTest() {
	suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
	suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
	suite.store = &notificationStore{
		dbProvider:   suite.mockDBProvider,
		deploymentID: testDeploymentID,
	}
}

func (suite *StoreTestSuite) TestNewNotificationStore() {
	mockClient := providermock.NewDBClientInterfaceMock(suite.T())
	mockClient.On("GetTransactioner").Return(&mockTransactioner{}, nil)
	mockProvider := providermock.NewDBProviderInterfaceMock(suite.T())
	mockProvider.On("GetConfigDBClient").Return(mockClient, nil)
	originalGetDBProvider := getDBProvider
	getDBProvider = func() provider.DBProviderInterface { return mockProvider }
	defer func() { getDBProvider = originalGetDBProvider }()

	store, _, err := newNotificationStore()

	suite.NoError(err)
	suite.NotNil(store)
	suite.Implements((*notificationStoreInterface)(nil), store)
}

func (suite *StoreTestSuite) TestCreateSender() {
	p, err := cmodels.NewProperty("k", "v", false)
	suite.NoError(err)
	sender := common.NotificationSenderDTO{
		ID:          "sender-123",
		Name:        "Test Sender",
		Description: "Test Description",
		Type:        common.NotificationSenderTypeMessage,
		Provider:    common.MessageProviderTypeTwilio,
		Properties:  []cmodels.Property{*p},
	}

	suite.mockDBProvider.EXPECT().GetConfigDBClient().Return(suite.mockDBClient, nil).Once()
	propsJSON, err := cmodels.SerializePropertiesToJSONArray([]cmodels.Property{*p})
	suite.NoError(err)
	suite.mockDBClient.EXPECT().ExecuteContext(
		context.Background(),
		queryCreateNotificationSender, sender.Name, sender.ID, sender.Description,
		string(sender.Type), string(sender.Provider), propsJSON, testDeploymentID,
	).Return(int64(1), nil).Once()

	err = suite.store.createSender(context.Background(), sender)
	suite.NoError(err)
}

func (suite *StoreTestSuite) TestCreateSender_GetConfigDBClientError() {
	suite.mockDBProvider.EXPECT().GetConfigDBClient().Return(nil, errors.New("db err")).Once()
	sender := common.NotificationSenderDTO{ID: "s1"}
	err := suite.store.createSender(context.Background(), sender)
	suite.Error(err)
	suite.Contains(err.Error(), "failed to get database client")
}

func (suite *StoreTestSuite) TestCreateSender_DBExecuteError() {
	p, err := cmodels.NewProperty("k", "v", false)
	suite.NoError(err)
	sender := common.NotificationSenderDTO{
		ID:         "sender-err",
		Name:       "Test",
		Properties: []cmodels.Property{*p},
	}

	suite.mockDBProvider.EXPECT().GetConfigDBClient().Return(suite.mockDBClient, nil).Once()
	propsJSON, err := cmodels.SerializePropertiesToJSONArray([]cmodels.Property{*p})
	suite.NoError(err)
	suite.mockDBClient.EXPECT().ExecuteContext(context.Background(), queryCreateNotificationSender,
		sender.Name, sender.ID,
		sender.Description, string(sender.Type), string(sender.Provider), propsJSON, testDeploymentID).Return(
		int64(0), errors.New("exec fail")).Once()

	err = suite.store.createSender(context.Background(), sender)
	suite.Error(err)
	suite.Contains(err.Error(), "failed to execute query")
}

func (suite *StoreTestSuite) TestCreateSender_SerializeError() {
	// patch serializePropertiesToJSONArray to return error
	orig := serializePropertiesToJSONArray
	serializePropertiesToJSONArray = func(props []cmodels.Property) (string, error) {
		return "", errors.New("serialize fail")
	}
	defer func() { serializePropertiesToJSONArray = orig }()

	p, err := cmodels.NewProperty("k", "v", false)
	suite.NoError(err)
	sender := common.NotificationSenderDTO{ID: "s-ser", Name: "n", Properties: []cmodels.Property{*p}}

	suite.mockDBProvider.EXPECT().GetConfigDBClient().Return(suite.mockDBClient, nil).Once()

	err = suite.store.createSender(context.Background(), sender)
	suite.Error(err)
	suite.Contains(err.Error(), "failed to serialize properties to JSON")
}

func (suite *StoreTestSuite) TestListSenders_WithPropertiesStringAndBytes() {
	// prepare properties JSON
	p2, err := cmodels.NewProperty("k", "v", false)
	suite.NoError(err)
	props := []cmodels.Property{*p2}
	propsJSON, err := cmodels.SerializePropertiesToJSONArray(props)
	suite.NoError(err)

	// two rows: one with string properties, one with []byte properties
	row1 := map[string]interface{}{
		"id":          "s1",
		"name":        "n1",
		"description": "d1",
		"type":        "message",
		"provider":    "twilio",
		"properties":  propsJSON,
	}
	row2 := map[string]interface{}{
		"id":          "s2",
		"name":        "n2",
		"description": "d2",
		"type":        "message",
		"provider":    "twilio",
		"properties":  []byte(propsJSON),
	}

	suite.mockDBProvider.EXPECT().GetConfigDBClient().Return(suite.mockDBClient, nil).Once()
	suite.mockDBClient.EXPECT().QueryContext(context.Background(), queryGetAllNotificationSenders, testDeploymentID).
		Return(

			[]map[string]interface{}{row1, row2}, nil).Once()

	senders, err := suite.store.listSenders(context.Background())
	suite.NoError(err)
	suite.Len(senders, 2)
	suite.Len(senders[0].Properties, 1)
	suite.Len(senders[1].Properties, 1)
}

func (suite *StoreTestSuite) TestListSenders_GetConfigDBClientError() {
	suite.mockDBProvider.EXPECT().GetConfigDBClient().Return(nil, errors.New("db err")).Once()
	res, err := suite.store.listSenders(context.Background())
	suite.Error(err)
	suite.Nil(res)
	suite.Contains(err.Error(), "failed to get database client")
}

func (suite *StoreTestSuite) TestListSenders_WithError() {
	suite.mockDBProvider.EXPECT().GetConfigDBClient().Return(suite.mockDBClient, nil).Once()
	suite.mockDBClient.EXPECT().QueryContext(context.Background(), queryGetAllNotificationSenders, testDeploymentID).
		Return(nil, errors.New("query fail")).Once()

	res, err := suite.store.listSenders(context.Background())
	suite.Error(err)
	suite.Nil(res)
	suite.Contains(err.Error(), "failed to execute query")

	badRow := map[string]interface{}{"name": "n1"}
	suite.mockDBProvider.EXPECT().GetConfigDBClient().Return(suite.mockDBClient, nil).Once()
	suite.mockDBClient.EXPECT().QueryContext(context.Background(), queryGetAllNotificationSenders, testDeploymentID).
		Return(

			[]map[string]interface{}{badRow}, nil).Once()

	res2, err := suite.store.listSenders(context.Background())
	suite.Error(err)
	suite.Nil(res2)
	suite.Contains(err.Error(), "failed to build sender from result row")

	// deserialize properties error (invalid JSON)
	row := map[string]interface{}{
		"id":          "s1",
		"name":        "n1",
		"description": "d1",
		"type":        "message",
		"provider":    "twilio",
		"properties":  "not-json",
	}
	suite.mockDBProvider.EXPECT().GetConfigDBClient().Return(suite.mockDBClient, nil).Once()
	suite.mockDBClient.EXPECT().QueryContext(context.Background(), queryGetAllNotificationSenders, testDeploymentID).
		Return(

			[]map[string]interface{}{row}, nil).Once()

	res3, err := suite.store.listSenders(context.Background())
	suite.Error(err)
	suite.Nil(res3)
	suite.Contains(err.Error(), "failed to deserialize properties from JSON")
}

func (suite *StoreTestSuite) TestGetSenderByID() {
	// success single row
	row := map[string]interface{}{
		"id":          "s1",
		"name":        "n1",
		"description": "d1",
		"type":        "message",
		"provider":    "twilio",
		"properties":  "",
	}
	suite.mockDBProvider.EXPECT().GetConfigDBClient().Return(suite.mockDBClient, nil).Once()
	suite.mockDBClient.EXPECT().QueryContext(context.Background(), queryGetNotificationSenderByID,
		"s1", testDeploymentID).Return(
		[]map[string]interface{}{row}, nil).Once()

	s, err := suite.store.getSenderByID(context.Background(), "s1")
	suite.NoError(err)
	suite.NotNil(s)
	suite.Equal("s1", s.ID)

	// not found
	suite.mockDBProvider.EXPECT().GetConfigDBClient().Return(suite.mockDBClient, nil).Once()
	suite.mockDBClient.EXPECT().QueryContext(context.Background(), queryGetNotificationSenderByID,
		"s-x", testDeploymentID).Return(
		[]map[string]interface{}{}, nil).Once()
	s2, err := suite.store.getSenderByID(context.Background(), "s-x")
	suite.NoError(err)
	suite.Nil(s2)

	// multiple results -> error
	suite.mockDBProvider.EXPECT().GetConfigDBClient().Return(suite.mockDBClient, nil).Once()
	suite.mockDBClient.EXPECT().QueryContext(context.Background(), queryGetNotificationSenderByID,
		"s-multi", testDeploymentID).Return(
		[]map[string]interface{}{row, row}, nil).Once()
	s3, err := suite.store.getSenderByID(context.Background(), "s-multi")
	suite.Error(err)
	suite.Nil(s3)
}

func (suite *StoreTestSuite) TestGetSenderByID_GetConfigDBClientError() {
	suite.mockDBProvider.EXPECT().GetConfigDBClient().Return(nil, errors.New("db err")).Once()
	res, err := suite.store.getSenderByID(context.Background(), "s1")
	suite.Error(err)
	suite.Nil(res)
}

func (suite *StoreTestSuite) TestGetSenderByName_GetConfigDBClientError() {
	suite.mockDBProvider.EXPECT().GetConfigDBClient().Return(nil, errors.New("db err")).Once()
	res, err := suite.store.getSenderByName(context.Background(), "n1")
	suite.Error(err)
	suite.Nil(res)
}

func (suite *StoreTestSuite) TestGetSender_WithError() {
	cases := []struct {
		name    string
		setup   func(t *testing.T)
		wantErr string
	}{
		{
			name: "query error",
			setup: func(t *testing.T) {
				suite.mockDBProvider.EXPECT().GetConfigDBClient().Return(suite.mockDBClient, nil).Once()
				suite.mockDBClient.EXPECT().QueryContext(context.Background(), queryGetNotificationSenderByID,
					"s1", testDeploymentID).
					Return(nil, errors.New("query fail")).Once()
			},
			wantErr: "failed to execute query",
		},
		{
			name: "multiple results",
			setup: func(t *testing.T) {
				row := map[string]interface{}{
					"id":          "s1",
					"name":        "n1",
					"description": "d1",
					"type":        "message",
					"provider":    "twilio",
				}
				suite.mockDBProvider.EXPECT().GetConfigDBClient().Return(suite.mockDBClient, nil).Once()
				suite.mockDBClient.EXPECT().QueryContext(context.Background(), queryGetNotificationSenderByID,
					"s1", testDeploymentID).
					Return([]map[string]interface{}{row, row}, nil).Once()
			},
			wantErr: "multiple senders",
		},
		{
			name: "build error",
			setup: func(t *testing.T) {
				badRow := map[string]interface{}{"name": "n1"}
				suite.mockDBProvider.EXPECT().GetConfigDBClient().Return(suite.mockDBClient, nil).Once()
				suite.mockDBClient.EXPECT().QueryContext(context.Background(), queryGetNotificationSenderByID,
					"s1", testDeploymentID).
					Return([]map[string]interface{}{badRow}, nil).Once()
			},
			wantErr: "failed to build sender",
		},
		{
			name: "deserialize error",
			setup: func(t *testing.T) {
				row := map[string]interface{}{
					"id":          "s1",
					"name":        "n1",
					"description": "d1",
					"type":        "message",
					"provider":    "twilio",
					"properties":  "not-json",
				}
				suite.mockDBProvider.EXPECT().GetConfigDBClient().Return(suite.mockDBClient, nil).Once()
				suite.mockDBClient.EXPECT().QueryContext(context.Background(), queryGetNotificationSenderByID,
					"s1", testDeploymentID).
					Return([]map[string]interface{}{row}, nil).Once()
			},
			wantErr: "failed to deserialize properties",
		},
	}

	for _, tc := range cases {
		suite.T().Run(tc.name, func(t *testing.T) {
			tc.setup(t)
			s, err := suite.store.getSenderByID(context.Background(), "s1")
			suite.Error(err)
			suite.Nil(s)
			suite.Contains(err.Error(), tc.wantErr)
		})
	}
}

func (suite *StoreTestSuite) TestGetSender_WithProperties() {
	p, err := cmodels.NewProperty("k", "v", false)
	suite.NoError(err)
	propsJSON, err := cmodels.SerializePropertiesToJSONArray([]cmodels.Property{*p})
	suite.NoError(err)

	row := map[string]interface{}{
		"id":          "s1",
		"name":        "n1",
		"description": "d1",
		"type":        "message",
		"provider":    "twilio",
		"properties":  propsJSON,
	}
	suite.mockDBProvider.EXPECT().GetConfigDBClient().Return(suite.mockDBClient, nil).Once()
	suite.mockDBClient.EXPECT().QueryContext(context.Background(), queryGetNotificationSenderByID,
		"s1", testDeploymentID).Return(
		[]map[string]interface{}{row}, nil).Once()

	s, err := suite.store.getSender(context.Background(), queryGetNotificationSenderByID, "s1")
	suite.NoError(err)
	suite.NotNil(s)
	suite.Len(s.Properties, 1)
}

func (suite *StoreTestSuite) TestUpdateSender() {
	sender := common.NotificationSenderDTO{ID: "s1", Name: "n1", Provider: common.MessageProviderTypeTwilio}

	suite.mockDBProvider.EXPECT().GetConfigDBClient().Return(suite.mockDBClient, nil).Once()
	suite.mockDBClient.EXPECT().ExecuteContext(context.Background(), queryUpdateNotificationSender,
		sender.Name, sender.Description,
		string(sender.Provider), "", "s1", string(sender.Type), testDeploymentID).Return(int64(1), nil).Once()

	err := suite.store.updateSender(context.Background(), "s1", sender)
	suite.NoError(err)
}

func (suite *StoreTestSuite) TestUpdateSender_WithProperties() {
	p, err := cmodels.NewProperty("k", "v", false)
	suite.NoError(err)
	sender := common.NotificationSenderDTO{ID: "s1", Name: "n1",
		Provider: common.MessageProviderTypeTwilio, Properties: []cmodels.Property{*p}}

	suite.mockDBProvider.EXPECT().GetConfigDBClient().Return(suite.mockDBClient, nil).Once()
	propsJSON, err := cmodels.SerializePropertiesToJSONArray([]cmodels.Property{*p})
	suite.NoError(err)
	suite.mockDBClient.EXPECT().ExecuteContext(context.Background(), queryUpdateNotificationSender,
		sender.Name, sender.Description,
		string(sender.Provider), propsJSON, "s1", string(sender.Type), testDeploymentID).Return(int64(1), nil).Once()

	err = suite.store.updateSender(context.Background(), "s1", sender)
	suite.NoError(err)
}

func (suite *StoreTestSuite) TestUpdateSender_GetConfigDBClientError() {
	suite.mockDBProvider.EXPECT().GetConfigDBClient().Return(nil, errors.New("db err")).Once()
	err := suite.store.updateSender(context.Background(), "s1", common.NotificationSenderDTO{})
	suite.Error(err)
}

func (suite *StoreTestSuite) TestUpdateSender_WithError() {
	cases := []struct {
		name    string
		setup   func(t *testing.T)
		wantErr string
	}{
		{
			name: "get db client error",
			setup: func(t *testing.T) {
				suite.mockDBProvider.EXPECT().GetConfigDBClient().Return(nil, errors.New("db err")).Once()
			},
			wantErr: "failed to get database client",
		},
		{
			name: "execute error",
			setup: func(t *testing.T) {
				sender := common.NotificationSenderDTO{Name: "n1", Provider: common.MessageProviderTypeTwilio}
				suite.mockDBProvider.EXPECT().GetConfigDBClient().Return(suite.mockDBClient, nil).Once()
				suite.mockDBClient.EXPECT().ExecuteContext(context.Background(), queryUpdateNotificationSender,
					sender.Name,
					sender.Description, string(sender.Provider), "", "s1", string(sender.Type), testDeploymentID).
					Return(int64(0), errors.New("exec fail")).Once()
			},
			wantErr: "failed to execute query",
		},
	}

	for _, tc := range cases {
		suite.T().Run(tc.name, func(t *testing.T) {
			tc.setup(t)
			// use a sender without properties for these failure-case checks
			sender := common.NotificationSenderDTO{ID: "s1", Name: "n1",
				Provider: common.MessageProviderTypeTwilio}
			err := suite.store.updateSender(context.Background(), "s1", sender)
			suite.Error(err)
			suite.Contains(err.Error(), tc.wantErr)
		})
	}
}

func (suite *StoreTestSuite) TestUpdateSender_ExecuteError() {
	sender := common.NotificationSenderDTO{ID: "s1", Name: "n1", Provider: common.MessageProviderTypeTwilio}
	suite.mockDBProvider.EXPECT().GetConfigDBClient().Return(suite.mockDBClient, nil).Once()
	suite.mockDBClient.EXPECT().ExecuteContext(context.Background(), queryUpdateNotificationSender,
		sender.Name, sender.Description,
		string(sender.Provider), "", "s1", string(sender.Type), testDeploymentID).
		Return(int64(0), errors.New("exec fail")).Once()

	err := suite.store.updateSender(context.Background(), "s1", sender)
	suite.Error(err)
	suite.Contains(err.Error(), "failed to execute query")
}

func (suite *StoreTestSuite) TestUpdateSender_SerializeError() {
	orig := serializePropertiesToJSONArray
	serializePropertiesToJSONArray = func(props []cmodels.Property) (string, error) {
		return "", errors.New("serialize fail")
	}
	defer func() { serializePropertiesToJSONArray = orig }()

	p, err := cmodels.NewProperty("k", "v", false)
	suite.NoError(err)
	sender := common.NotificationSenderDTO{ID: "s1", Name: "n1",
		Provider: common.MessageProviderTypeTwilio, Properties: []cmodels.Property{*p}}

	suite.mockDBProvider.EXPECT().GetConfigDBClient().Return(suite.mockDBClient, nil).Once()

	err = suite.store.updateSender(context.Background(), "s1", sender)
	suite.Error(err)
	suite.Contains(err.Error(), "failed to serialize properties to JSON")
}

func (suite *StoreTestSuite) TestDeleteSender_NoRows() {
	// delete with 0 rows affected (should not return error)
	suite.mockDBProvider.EXPECT().GetConfigDBClient().Return(suite.mockDBClient, nil).Once()
	suite.mockDBClient.EXPECT().ExecuteContext(context.Background(), queryDeleteNotificationSender, "s1",
		testDeploymentID).
		Return(int64(0), nil).Once()

	err := suite.store.deleteSender(context.Background(), "s1")
	suite.NoError(err)
}

func (suite *StoreTestSuite) TestDeleteSender_GetConfigDBClientError() {
	suite.mockDBProvider.EXPECT().GetConfigDBClient().Return(nil, errors.New("db err")).Once()
	err := suite.store.deleteSender(context.Background(), "s1")
	suite.Error(err)
}

func (suite *StoreTestSuite) TestDeleteSender_ExecuteError() {
	suite.mockDBProvider.EXPECT().GetConfigDBClient().Return(suite.mockDBClient, nil).Once()
	suite.mockDBClient.EXPECT().ExecuteContext(context.Background(), queryDeleteNotificationSender,
		"s1", testDeploymentID).Return(
		int64(0), errors.New("exec fail")).Once()

	err := suite.store.deleteSender(context.Background(), "s1")
	suite.Error(err)
	suite.Contains(err.Error(), "failed to execute delete query")
}

func (suite *StoreTestSuite) TestBuildSenderFromResultRow_WithError() {
	s := &notificationStore{
		deploymentID: testDeploymentID,
	}

	// missing sender_id
	row := map[string]interface{}{"name": "n1", "description": "d1", "type": "message", "provider": "p"}
	_, err := s.buildSenderFromResultRow(row)
	suite.Error(err)

	// wrong type for sender_id
	row2 := map[string]interface{}{"id": 123, "name": "n1", "description": "d1",
		"type": "message", "provider": "p"}
	_, err = s.buildSenderFromResultRow(row2)
	suite.Error(err)
}

func (suite *StoreTestSuite) TestBuildSenderFromResultRow_MissingFields() {
	s := &notificationStore{
		deploymentID: testDeploymentID,
	}

	// missing name
	row := map[string]interface{}{"id": "s1", "description": "d1", "type": "message", "provider": "p"}
	_, err := s.buildSenderFromResultRow(row)
	suite.Error(err)
	suite.Contains(err.Error(), "failed to parse name")

	// missing description
	row2 := map[string]interface{}{"id": "s1", "name": "n1", "type": "message", "provider": "p"}
	_, err = s.buildSenderFromResultRow(row2)
	suite.Error(err)
	suite.Contains(err.Error(), "failed to parse description")

	// missing type
	row3 := map[string]interface{}{"id": "s1", "name": "n1", "description": "d1", "provider": "p"}
	_, err = s.buildSenderFromResultRow(row3)
	suite.Error(err)
	suite.Contains(err.Error(), "failed to parse type")

	// missing provider
	row4 := map[string]interface{}{"id": "s1", "name": "n1", "description": "d1", "type": "message"}
	_, err = s.buildSenderFromResultRow(row4)
	suite.Error(err)
	suite.Contains(err.Error(), "failed to parse provider")
}

func (suite *StoreTestSuite) TestBuildSenderFromResultRow() {
	s := &notificationStore{
		deploymentID: testSenderID,
	}
	row := map[string]interface{}{
		"id":          "sid",
		"name":        "name",
		"description": "desc",
		"type":        "message",
		"provider":    "twilio",
	}
	sender, err := s.buildSenderFromResultRow(row)
	suite.NoError(err)
	suite.NotNil(sender)
	suite.Equal("sid", sender.ID)
	suite.Equal("name", sender.Name)
}
