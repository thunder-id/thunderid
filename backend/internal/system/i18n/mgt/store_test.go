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

package mgt

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/mocks/database/modelmock"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

const testDeploymentID = "test-deployment-id"

// mockResult is a simple mock implementation of sql.Result.
type mockResult struct {
	lastInsertID int64
	rowsAffected int64
}

func (m *mockResult) LastInsertId() (int64, error) {
	return m.lastInsertID, nil
}

func (m *mockResult) RowsAffected() (int64, error) {
	return m.rowsAffected, nil
}

var _ sql.Result = (*mockResult)(nil)

type I18nStoreTestSuite struct {
	suite.Suite
	mockDBProvider *providermock.DBProviderInterfaceMock
	mockDBClient   *providermock.DBClientInterfaceMock
	mockTx         *modelmock.TxInterfaceMock
	store          *i18nStore
}

func TestI18nStoreTestSuite(t *testing.T) {
	suite.Run(t, new(I18nStoreTestSuite))
}

func (suite *I18nStoreTestSuite) SetupTest() {
	suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
	suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
	suite.mockTx = modelmock.NewTxInterfaceMock(suite.T())
	suite.store = &i18nStore{
		dbProvider:   suite.mockDBProvider,
		deploymentID: testDeploymentID,
	}
}

// GetDistinctLanguages Tests
func (suite *I18nStoreTestSuite) TestGetDistinctLanguages_Success() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", queryGetDistinctLanguages, testDeploymentID).Return([]map[string]interface{}{
		{"language_code": "en-US"},
		{"language_code": "fr-FR"},
	}, nil)

	langs, err := suite.store.GetDistinctLanguages()

	suite.NoError(err)
	suite.Len(langs, 2)
	suite.Contains(langs, "en-US")
	suite.Contains(langs, "fr-FR")
}

func (suite *I18nStoreTestSuite) TestGetDistinctLanguages_QueryError() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", queryGetDistinctLanguages, testDeploymentID).Return(nil, errors.New("db error"))

	langs, err := suite.store.GetDistinctLanguages()

	suite.Error(err)
	suite.Nil(langs)
}

// GetTranslations Tests
func (suite *I18nStoreTestSuite) TestGetTranslations_Success() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", queryGetTranslations, testDeploymentID).Return([]map[string]interface{}{
		{
			"message_key": "k1", "language_code": "en-US", "namespace": "ns1", "value": "v1",
		},
	}, nil)

	trans, err := suite.store.GetTranslations()

	suite.NoError(err)
	suite.NotNil(trans)
	suite.Equal("v1", trans["ns1|k1"]["en-US"].Value)
}

// GetTranslationsByNamespace Tests
func (suite *I18nStoreTestSuite) TestGetTranslationsByNamespace_Success() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", queryGetTranslationsByNamespace, "ns1", testDeploymentID).
		Return([]map[string]interface{}{
			{
				"message_key": "k1", "language_code": "en-US", "namespace": "ns1", "value": "v1",
			},
		}, nil)

	trans, err := suite.store.GetTranslationsByNamespace("ns1")

	suite.NoError(err)
	suite.NotNil(trans)
	suite.Equal("v1", trans["ns1|k1"]["en-US"].Value)
}

// GetTranslationsByKey Tests
func (suite *I18nStoreTestSuite) TestGetTranslationsByKey_Success() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", queryGetTranslation, "k1", "ns1", testDeploymentID).Return([]map[string]interface{}{
		{
			"message_key": "k1", "language_code": "en-US", "namespace": "ns1", "value": "v1",
		},
	}, nil)

	trans, err := suite.store.GetTranslationsByKey("k1", "ns1")

	suite.NoError(err)
	suite.NotNil(trans)
	suite.Equal("v1", trans["en-US"].Value)
}

func (suite *I18nStoreTestSuite) TestGetTranslationsByKey_NotFound() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", queryGetTranslation, "k1", "ns1", testDeploymentID).
		Return([]map[string]interface{}{}, nil)

	trans, err := suite.store.GetTranslationsByKey("k1", "ns1")
	suite.NoError(err)
	suite.NotNil(trans)
	suite.Empty(trans)
}

// UpsertTranslation Tests
func (suite *I18nStoreTestSuite) TestUpsertTranslation_Success() {
	translation := Translation{Key: "k", Namespace: "ns", Language: "en", Value: "v"}

	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Execute", queryUpsertTranslation, "k", "en", "ns", "v", testDeploymentID).
		Return(int64(1), nil)

	err := suite.store.UpsertTranslation(translation)

	suite.NoError(err)
}

// DeleteTranslation Tests
func (suite *I18nStoreTestSuite) TestDeleteTranslation_Success() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Execute", queryDeleteTranslation, "en", "k", "ns", testDeploymentID).
		Return(int64(1), nil)

	err := suite.store.DeleteTranslation("en", "k", "ns")

	suite.NoError(err)
}

// DeleteTranslationsByLanguage Tests
func (suite *I18nStoreTestSuite) TestDeleteTranslationsByLanguage_Success() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Execute", queryDeleteTranslationsByLanguage, "en", testDeploymentID).
		Return(int64(1), nil)

	err := suite.store.DeleteTranslationsByLanguage("en")

	suite.NoError(err)
}

// UpsertTranslationsByLanguage Tests
func (suite *I18nStoreTestSuite) TestUpsertTranslationsByLanguage_Success() {
	translations := []Translation{
		{Key: "k1", Namespace: "ns", Language: "en", Value: "v1"},
	}

	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("BeginTx").Return(suite.mockTx, nil)

	// Expect delete first
	suite.mockTx.On("Exec", queryDeleteTranslationsByLanguage, "en", testDeploymentID).
		Return(&mockResult{}, nil)

	// Expect insert
	suite.mockTx.On("Exec", queryInsertTranslation, "k1", "en", "ns", "v1", testDeploymentID).
		Return(&mockResult{}, nil)

	suite.mockTx.On("Commit").Return(nil)

	err := suite.store.UpsertTranslationsByLanguage("en", translations)

	suite.NoError(err)
}

func (suite *I18nStoreTestSuite) TestUpsertTranslationsByLanguage_RollbackOnInsertError() {
	translations := []Translation{
		{Key: "k1", Namespace: "ns", Language: "en", Value: "v1"},
	}

	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("BeginTx").Return(suite.mockTx, nil)

	suite.mockTx.On("Exec", queryDeleteTranslationsByLanguage, "en", testDeploymentID).
		Return(&mockResult{}, nil)

	suite.mockTx.On("Exec", queryInsertTranslation, "k1", "en", "ns", "v1", testDeploymentID).
		Return(nil, errors.New("insert error"))

	suite.mockTx.On("Rollback").Return(nil)

	err := suite.store.UpsertTranslationsByLanguage("en", translations)

	suite.Error(err)
	suite.Contains(err.Error(), "failed to insert translation")
}

func (suite *I18nStoreTestSuite) TestUpsertTranslationsByLanguage_BeginTxError() {
	translations := []Translation{{Key: "k", Namespace: "n", Language: "l", Value: "v"}}
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("BeginTx").Return(nil, errors.New("begin error"))

	err := suite.store.UpsertTranslationsByLanguage("l", translations)

	suite.Error(err)
	suite.Contains(err.Error(), "failed to begin transaction")
}

func (suite *I18nStoreTestSuite) TestUpsertTranslationsByLanguage_DeleteError() {
	translations := []Translation{{Key: "k", Namespace: "n", Language: "l", Value: "v"}}
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("BeginTx").Return(suite.mockTx, nil)
	suite.mockTx.On("Exec", queryDeleteTranslationsByLanguage, "l", testDeploymentID).
		Return(nil, errors.New("delete error"))
	suite.mockTx.On("Rollback").Return(nil)

	err := suite.store.UpsertTranslationsByLanguage("l", translations)

	suite.Error(err)
	suite.Contains(err.Error(), "failed to delete translations")
}

func (suite *I18nStoreTestSuite) TestUpsertTranslationsByLanguage_RollbackError() {
	translations := []Translation{{Key: "k", Namespace: "n", Language: "l", Value: "v"}}
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("BeginTx").Return(suite.mockTx, nil)
	suite.mockTx.On("Exec", queryDeleteTranslationsByLanguage, "l", testDeploymentID).
		Return(nil, errors.New("delete error"))
	suite.mockTx.On("Rollback").Return(errors.New("rollback error"))

	err := suite.store.UpsertTranslationsByLanguage("l", translations)

	suite.Error(err)
	suite.Contains(err.Error(), "failed to delete translations")
	// errors.Join might not stringify both messages nicely in all go versions or checking implies it contains both?
	// Usually "failed to delete translations: delete error\nfailed to rollback transaction: rollback error"
	suite.Contains(err.Error(), "failed to rollback transaction")
}

func (suite *I18nStoreTestSuite) TestUpsertTranslationsByLanguage_CommitError() {
	translations := []Translation{{Key: "k", Namespace: "n", Language: "l", Value: "v"}}
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("BeginTx").Return(suite.mockTx, nil)
	suite.mockTx.On("Exec", queryDeleteTranslationsByLanguage, "l", testDeploymentID).
		Return(&mockResult{}, nil)
	suite.mockTx.On("Exec", queryInsertTranslation, "k", "l", "n", "v", testDeploymentID).
		Return(&mockResult{}, nil)
	suite.mockTx.On("Commit").Return(errors.New("commit error"))

	err := suite.store.UpsertTranslationsByLanguage("l", translations)

	suite.Error(err)
	suite.Contains(err.Error(), "failed to commit transaction")
}

func (suite *I18nStoreTestSuite) TestUpsertTranslation_Error() {
	translation := Translation{Key: "k", Namespace: "ns", Language: "en", Value: "v"}
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Execute", queryUpsertTranslation, "k", "en", "ns", "v", testDeploymentID).
		Return(int64(0), errors.New("exec error"))

	err := suite.store.UpsertTranslation(translation)

	suite.Error(err)
}

func (suite *I18nStoreTestSuite) TestDeleteTranslation_Error() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Execute", queryDeleteTranslation, "en", "k", "ns", testDeploymentID).
		Return(int64(0), errors.New("exec error"))

	err := suite.store.DeleteTranslation("en", "k", "ns")

	suite.Error(err)
}

func (suite *I18nStoreTestSuite) TestDeleteTranslationsByLanguage_Error() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Execute", queryDeleteTranslationsByLanguage, "en", testDeploymentID).
		Return(int64(0), errors.New("exec error"))

	err := suite.store.DeleteTranslationsByLanguage("en")

	suite.Error(err)
}

func (suite *I18nStoreTestSuite) TestGetTranslations_QueryError() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", queryGetTranslations, testDeploymentID).
		Return(nil, errors.New("query error"))

	result, err := suite.store.GetTranslations()

	suite.Nil(result)
	suite.Error(err)
}

func (suite *I18nStoreTestSuite) TestGetTranslationsByNamespace_QueryError() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", queryGetTranslationsByNamespace, "ns", testDeploymentID).
		Return(nil, errors.New("query error"))

	result, err := suite.store.GetTranslationsByNamespace("ns")

	suite.Nil(result)
	suite.Error(err)
}

func (suite *I18nStoreTestSuite) TestGetTranslationsByKey_QueryError() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", queryGetTranslation, "k", "ns", testDeploymentID).
		Return(nil, errors.New("query error"))

	result, err := suite.store.GetTranslationsByKey("k", "ns")

	suite.Nil(result)
	suite.Error(err)
}

// Test Data Parsing Errors
func (suite *I18nStoreTestSuite) TestBuildTranslationFromRow_Errors() {
	// message_key error
	row1 := map[string]interface{}{"message_key": 123}
	_, err := buildTranslationFromRow(row1)
	suite.Error(err)
	suite.Contains(err.Error(), "message_key")

	// language_code error
	row2 := map[string]interface{}{"message_key": "k", "language_code": 123}
	_, err = buildTranslationFromRow(row2)
	suite.Error(err)
	suite.Contains(err.Error(), "language_code")

	// namespace error
	row3 := map[string]interface{}{"message_key": "k", "language_code": "l", "namespace": 123}
	_, err = buildTranslationFromRow(row3)
	suite.Error(err)
	suite.Contains(err.Error(), "namespace")

	// value error
	row4 := map[string]interface{}{"message_key": "k", "language_code": "l", "namespace": "n", "value": 123}
	_, err = buildTranslationFromRow(row4)
	suite.Error(err)
	suite.Contains(err.Error(), "value")
}

func (suite *I18nStoreTestSuite) TestGetTranslations_ParsingError() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", queryGetTranslations, testDeploymentID).Return([]map[string]interface{}{
		{"message_key": 123}, // Invalid type
	}, nil)

	result, err := suite.store.GetTranslations()

	suite.Nil(result)
	suite.Error(err)
}

func (suite *I18nStoreTestSuite) TestGetTranslationsByKey_ParsingError() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", queryGetTranslation, "k", "ns", testDeploymentID).Return([]map[string]interface{}{
		{"message_key": 123}, // Invalid type
	}, nil)

	result, err := suite.store.GetTranslationsByKey("k", "ns")

	suite.Nil(result)
	suite.Error(err)
}

func (suite *I18nStoreTestSuite) TestGetDistinctLanguages_ParsingError() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", queryGetDistinctLanguages, testDeploymentID).Return([]map[string]interface{}{
		{"language_code": 123}, // Invalid type
	}, nil)

	result, err := suite.store.GetDistinctLanguages()

	suite.Nil(result)
	suite.Error(err)
}

func (suite *I18nStoreTestSuite) TestGetDBClient_Error() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("config error"))

	var err error

	_, err = suite.store.GetDistinctLanguages()
	suite.Error(err)
	suite.Contains(err.Error(), "failed to get database client")

	_, err = suite.store.GetTranslations()
	suite.Error(err)
	suite.Contains(err.Error(), "failed to get database client")

	_, err = suite.store.GetTranslationsByNamespace("ns")
	suite.Error(err)
	suite.Contains(err.Error(), "failed to get database client")

	_, err = suite.store.GetTranslationsByKey("k", "ns")
	suite.Error(err)
	suite.Contains(err.Error(), "failed to get database client")

	err = suite.store.UpsertTranslationsByLanguage("en", nil)
	suite.Error(err)
	suite.Contains(err.Error(), "failed to get database client")

	err = suite.store.UpsertTranslation(Translation{})
	suite.Error(err)
	suite.Contains(err.Error(), "failed to get database client")

	err = suite.store.DeleteTranslation("en", "k", "ns")
	suite.Error(err)
	suite.Contains(err.Error(), "failed to get database client")

	err = suite.store.DeleteTranslationsByLanguage("en")
	suite.Error(err)
	suite.Contains(err.Error(), "failed to get database client")

	err = suite.store.DeleteTranslationsByNamespace(context.Background(), "app-test")
	suite.Error(err)
	suite.Contains(err.Error(), "failed to get database client")

	err = suite.store.DeleteTranslationsByKey(context.Background(), "custom", "app.test-id.name")
	suite.Error(err)
	suite.Contains(err.Error(), "failed to get database client")
}

// DeleteTranslationsByNamespace Tests

func (suite *I18nStoreTestSuite) TestDeleteTranslationsByNamespace_Success() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything,
		queryDeleteTranslationsByNamespace, "app-test", testDeploymentID).
		Return(int64(1), nil)

	err := suite.store.DeleteTranslationsByNamespace(context.Background(), "app-test")

	suite.NoError(err)
}

func (suite *I18nStoreTestSuite) TestDeleteTranslationsByNamespace_Error() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything,
		queryDeleteTranslationsByNamespace, "app-test", testDeploymentID).
		Return(int64(0), errors.New("exec error"))

	err := suite.store.DeleteTranslationsByNamespace(context.Background(), "app-test")

	suite.Error(err)
	suite.Contains(err.Error(), "failed to delete translations by namespace")
}

// DeleteTranslationsByKey Tests

func (suite *I18nStoreTestSuite) TestDeleteTranslationsByKey_Success() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything,
		queryDeleteTranslationsByKey, "custom", "app.test-id.name", testDeploymentID).
		Return(int64(1), nil)

	err := suite.store.DeleteTranslationsByKey(context.Background(), "custom", "app.test-id.name")

	suite.NoError(err)
}

func (suite *I18nStoreTestSuite) TestDeleteTranslationsByKey_Error() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything,
		queryDeleteTranslationsByKey, "custom", "app.test-id.name", testDeploymentID).
		Return(int64(0), errors.New("exec error"))

	err := suite.store.DeleteTranslationsByKey(context.Background(), "custom", "app.test-id.name")

	suite.Error(err)
	suite.Contains(err.Error(), "failed to delete translations by namespace and key")
}
