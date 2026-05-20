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

package flowmgt

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

type FlowStoreTestSuite struct {
	suite.Suite
	mockDBProvider *providermock.DBProviderInterfaceMock
	mockDBClient   *providermock.DBClientInterfaceMock
	store          *flowStore
}

func TestFlowStoreTestSuite(t *testing.T) {
	suite.Run(t, new(FlowStoreTestSuite))
}

func (s *FlowStoreTestSuite) SetupTest() {
	_ = config.InitializeServerRuntime("test", &config.Config{
		Server: config.ServerConfig{Identifier: "test-deployment"},
		Flow:   config.FlowConfig{MaxVersionHistory: 5},
	})

	s.mockDBProvider = providermock.NewDBProviderInterfaceMock(s.T())
	s.mockDBClient = providermock.NewDBClientInterfaceMock(s.T())
	s.store = &flowStore{
		dbProvider:        s.mockDBProvider,
		deploymentID:      "test-deployment",
		maxVersionHistory: 5,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowStore")),
	}
}

// ListFlows Tests

func (s *FlowStoreTestSuite) TestListFlowsDBClientError() {
	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(nil, errors.New("connection error"))

	flows, count, err := s.store.ListFlows(context.Background(), 10, 0, "")

	s.Error(err)
	s.Contains(err.Error(), "failed to get database client")
	s.Equal(0, count)
	s.Nil(flows)
}

func (s *FlowStoreTestSuite) TestListFlowsCountQueryError() {
	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryCountFlows, "test-deployment").
		Return(nil, errors.New("query error")).Once()

	flows, count, err := s.store.ListFlows(context.Background(), 10, 0, "")

	s.Error(err)
	s.Contains(err.Error(), "failed to count flows")
	s.Equal(0, count)
	s.Nil(flows)
}

func (s *FlowStoreTestSuite) TestListFlowsQueryError() {
	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryCountFlows, "test-deployment").
		Return([]map[string]interface{}{{colCount: int64(1)}}, nil).Once()
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryListFlows, "test-deployment", 10, 0).
		Return(nil, errors.New("query error")).Once()

	flows, count, err := s.store.ListFlows(context.Background(), 10, 0, "")

	s.Error(err)
	s.Contains(err.Error(), "failed to list flows")
	s.Equal(0, count)
	s.Nil(flows)
}

func (s *FlowStoreTestSuite) TestListFlowsSuccess() {
	flowsData := []map[string]interface{}{
		{
			colFlowID:        "flow-1",
			colHandle:        "handle-1",
			colName:          "Flow 1",
			colFlowType:      string(common.FlowTypeAuthentication),
			colActiveVersion: int64(1),
			colCreatedAt:     "2025-01-01T00:00:00Z",
			colUpdatedAt:     "2025-01-01T00:00:00Z",
		},
		{
			colFlowID:        "flow-2",
			colHandle:        "handle-2",
			colName:          "Flow 2",
			colFlowType:      string(common.FlowTypeRegistration),
			colActiveVersion: int64(2),
			colCreatedAt:     "2025-01-02T00:00:00Z",
			colUpdatedAt:     "2025-01-02T00:00:00Z",
		},
	}

	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryCountFlows, "test-deployment").
		Return([]map[string]interface{}{{colCount: int64(2)}}, nil).Once()
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryListFlows, "test-deployment", 10, 0).
		Return(flowsData, nil).Once()

	flows, count, err := s.store.ListFlows(context.Background(), 10, 0, "")

	s.NoError(err)
	s.Equal(2, count)
	s.Len(flows, 2)
	s.Equal("flow-1", flows[0].ID)
	s.Equal("Flow 1", flows[0].Name)
	s.Equal("flow-2", flows[1].ID)
	s.Equal("Flow 2", flows[1].Name)
}

func (s *FlowStoreTestSuite) TestListFlowsWithTypeSuccess() {
	flowsData := []map[string]interface{}{
		{
			colFlowID:        "flow-1",
			colHandle:        "auth-handle",
			colName:          "Auth Flow",
			colFlowType:      string(common.FlowTypeAuthentication),
			colActiveVersion: int64(1),
			colCreatedAt:     "2025-01-01T00:00:00Z",
			colUpdatedAt:     "2025-01-01T00:00:00Z",
		},
	}

	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryCountFlowsWithType,
		string(common.FlowTypeAuthentication), "test-deployment").
		Return([]map[string]interface{}{{colCount: int64(1)}}, nil).Once()
	s.mockDBClient.EXPECT().
		QueryContext(mock.Anything, queryListFlowsWithType,
			string(common.FlowTypeAuthentication), "test-deployment", 10, 0).
		Return(flowsData, nil).Once()

	flows, count, err := s.store.ListFlows(context.Background(), 10, 0, string(common.FlowTypeAuthentication))

	s.NoError(err)
	s.Equal(1, count)
	s.Len(flows, 1)
	s.Equal("flow-1", flows[0].ID)
	s.Equal(common.FlowTypeAuthentication, flows[0].FlowType)
}

// GetFlowByID Tests

func (s *FlowStoreTestSuite) TestGetFlowByIDNotFound() {
	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryGetFlow, "non-existent", "test-deployment").
		Return([]map[string]interface{}{}, nil).Once()

	flow, err := s.store.GetFlowByID(context.Background(), "non-existent")

	s.Error(err)
	s.ErrorIs(err, errFlowNotFound)
	s.Nil(flow)
}

func (s *FlowStoreTestSuite) TestGetFlowByIDSuccess() {
	flowData := map[string]interface{}{
		colFlowID:        "flow-123",
		colHandle:        "test-handle",
		colName:          "Test Flow",
		colFlowType:      string(common.FlowTypeAuthentication),
		colActiveVersion: int64(1),
		colNodes:         `[{"id":"START","type":"START"}]`,
		colCreatedAt:     "2025-01-01T00:00:00Z",
		colUpdatedAt:     "2025-01-01T00:00:00Z",
	}

	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryGetFlow, "flow-123", "test-deployment").
		Return([]map[string]interface{}{flowData}, nil).Once()

	flow, err := s.store.GetFlowByID(context.Background(), "flow-123")

	s.NoError(err)
	s.NotNil(flow)
	s.Equal("flow-123", flow.ID)
	s.Equal("test-handle", flow.Handle)
	s.Equal("Test Flow", flow.Name)
	s.Equal(common.FlowTypeAuthentication, flow.FlowType)
	s.Equal(1, flow.ActiveVersion)
	s.Len(flow.Nodes, 1)
}

func (s *FlowStoreTestSuite) TestGetFlowByIDDBError() {
	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(nil, errors.New("connection error"))

	flow, err := s.store.GetFlowByID(context.Background(), "flow-1")

	s.Error(err)
	s.Contains(err.Error(), "failed to get database client")
	s.Nil(flow)
}

func (s *FlowStoreTestSuite) TestGetFlowByIDQueryError() {
	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryGetFlow, "flow-1", "test-deployment").
		Return(nil, errors.New("query error")).Once()

	flow, err := s.store.GetFlowByID(context.Background(), "flow-1")

	s.Error(err)
	s.Contains(err.Error(), "failed to get flow")
	s.Nil(flow)
}

// DeleteFlow Tests

func (s *FlowStoreTestSuite) TestDeleteFlowSuccess() {
	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)
	s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryDeleteFlow, "flow-1", "test-deployment").
		Return(int64(1), nil).Once()

	err := s.store.DeleteFlow(context.Background(), "flow-1")

	s.NoError(err)
}

func (s *FlowStoreTestSuite) TestDeleteFlowDBError() {
	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(nil, errors.New("connection error"))

	err := s.store.DeleteFlow(context.Background(), "flow-1")

	s.Error(err)
	s.Contains(err.Error(), "failed to get database client")
}

func (s *FlowStoreTestSuite) TestDeleteFlowExecuteError() {
	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)
	s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryDeleteFlow, "flow-1", "test-deployment").
		Return(int64(0), errors.New("delete failed")).Once()

	err := s.store.DeleteFlow(context.Background(), "flow-1")

	s.Error(err)
	s.Contains(err.Error(), "failed to delete flow")
}

// GetFlowByHandle Tests

func (s *FlowStoreTestSuite) TestGetFlowByHandleSuccess() {
	flowData := map[string]interface{}{
		colFlowID:        "flow-123",
		colHandle:        "test-handle",
		colName:          "Test Flow",
		colFlowType:      string(common.FlowTypeAuthentication),
		colActiveVersion: int64(1),
		colNodes:         `[{"id":"START","type":"START"}]`,
		colCreatedAt:     time.Now().Format(time.RFC3339),
		colUpdatedAt:     time.Now().Format(time.RFC3339),
	}

	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryGetFlowByHandle, "test-handle",
		string(common.FlowTypeAuthentication), "test-deployment").Return(
		[]map[string]interface{}{flowData}, nil).Once()

	flow, err := s.store.GetFlowByHandle(context.Background(), "test-handle", common.FlowTypeAuthentication)

	s.NoError(err)
	s.NotNil(flow)
	s.Equal("flow-123", flow.ID)
	s.Equal("test-handle", flow.Handle)
	s.Equal("Test Flow", flow.Name)
}

func (s *FlowStoreTestSuite) TestGetFlowByHandleNotFound() {
	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryGetFlowByHandle, "non-existent",
		string(common.FlowTypeAuthentication), "test-deployment").Return(
		[]map[string]interface{}{}, nil).Once()

	flow, err := s.store.GetFlowByHandle(context.Background(), "non-existent", common.FlowTypeAuthentication)

	s.Error(err)
	s.ErrorIs(err, errFlowNotFound)
	s.Nil(flow)
}

func (s *FlowStoreTestSuite) TestGetFlowByHandleDBError() {
	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(nil, errors.New("connection error"))

	flow, err := s.store.GetFlowByHandle(context.Background(), "test-handle", common.FlowTypeAuthentication)

	s.Error(err)
	s.Contains(err.Error(), "failed to get database client")
	s.Nil(flow)
}

func (s *FlowStoreTestSuite) TestGetFlowByHandleQueryError() {
	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryGetFlowByHandle, "test-handle",
		string(common.FlowTypeAuthentication), "test-deployment").Return(
		nil, errors.New("query error")).Once()

	flow, err := s.store.GetFlowByHandle(context.Background(), "test-handle", common.FlowTypeAuthentication)

	s.Error(err)
	s.Contains(err.Error(), "failed to get flow by handle")
	s.Nil(flow)
}

// IsFlowExistsByHandle Tests

func (s *FlowStoreTestSuite) TestIsFlowExistsByHandleSuccess() {
	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryCheckFlowExistsByHandle, "test-handle",
		string(common.FlowTypeAuthentication), "test-deployment").Return(
		[]map[string]interface{}{{"exists": 1}}, nil).Once()

	exists, err := s.store.IsFlowExistsByHandle(context.Background(), "test-handle", common.FlowTypeAuthentication)

	s.NoError(err)
	s.True(exists)
}

func (s *FlowStoreTestSuite) TestIsFlowExistsByHandleNotFound() {
	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryCheckFlowExistsByHandle, "non-existent",
		string(common.FlowTypeAuthentication), "test-deployment").Return(
		[]map[string]interface{}{}, nil).Once()

	exists, err := s.store.IsFlowExistsByHandle(context.Background(), "non-existent", common.FlowTypeAuthentication)

	s.NoError(err)
	s.False(exists)
}

func (s *FlowStoreTestSuite) TestIsFlowExistsByHandleDBError() {
	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(nil, errors.New("connection error"))

	exists, err := s.store.IsFlowExistsByHandle(context.Background(), "test-handle", common.FlowTypeAuthentication)

	s.Error(err)
	s.Contains(err.Error(), "failed to get database client")
	s.False(exists)
}

func (s *FlowStoreTestSuite) TestIsFlowExistsByHandleQueryError() {
	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryCheckFlowExistsByHandle, "test-handle",
		string(common.FlowTypeAuthentication), "test-deployment").Return(
		nil, errors.New("query error")).Once()

	exists, err := s.store.IsFlowExistsByHandle(context.Background(), "test-handle", common.FlowTypeAuthentication)

	s.Error(err)
	s.Contains(err.Error(), "failed to check flow existence by handle")
	s.False(exists)
}

// ListFlowVersions Tests

func (s *FlowStoreTestSuite) TestListFlowVersionsDBError() {
	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(nil, errors.New("connection error"))

	versions, err := s.store.ListFlowVersions(context.Background(), "flow-1")

	s.Error(err)
	s.Contains(err.Error(), "failed to get database client")
	s.Nil(versions)
}

func (s *FlowStoreTestSuite) TestListFlowVersionsSuccess() {
	versionData := []map[string]interface{}{
		{
			colVersion:       int64(2),
			colCreatedAt:     "2025-01-02T00:00:00Z",
			colActiveVersion: int64(2),
		},
		{
			colVersion:       int64(1),
			colCreatedAt:     "2025-01-01T00:00:00Z",
			colActiveVersion: int64(2),
		},
	}

	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryListFlowVersions, "flow-123", s.store.deploymentID).
		Return(versionData, nil).Once()

	versions, err := s.store.ListFlowVersions(context.Background(), "flow-123")

	s.NoError(err)
	s.Len(versions, 2)
	s.Equal(2, versions[0].Version)
	s.True(versions[0].IsActive)
	s.Equal(1, versions[1].Version)
	s.False(versions[1].IsActive)
}

// GetFlowVersion Tests

func (s *FlowStoreTestSuite) TestGetFlowVersionNotFound() {
	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryGetFlowVersionWithMetadata,
		"flow-1", 99, "test-deployment").
		Return([]map[string]interface{}{}, nil).Once()

	version, err := s.store.GetFlowVersion(context.Background(), "flow-1", 99)

	s.Error(err)
	s.ErrorIs(err, errVersionNotFound)
	s.Nil(version)
}

func (s *FlowStoreTestSuite) TestGetFlowVersionSuccess() {
	versionData := map[string]interface{}{
		colFlowID:        "flow-123",
		colHandle:        "test-handle",
		colName:          "Test Flow",
		colFlowType:      string(common.FlowTypeAuthentication),
		colVersion:       int64(2),
		colActiveVersion: int64(3),
		colNodes:         `[{"id":"node-1","type":"basic-auth"}]`,
		colCreatedAt:     "2025-01-02T00:00:00Z",
	}

	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryGetFlowVersionWithMetadata,
		"flow-123", 2, "test-deployment").
		Return([]map[string]interface{}{versionData}, nil).Once()

	version, err := s.store.GetFlowVersion(context.Background(), "flow-123", 2)

	s.NoError(err)
	s.NotNil(version)
	s.Equal("flow-123", version.ID)
	s.Equal(2, version.Version)
	s.False(version.IsActive) // Version 2, but active is 3
	s.Len(version.Nodes, 1)
}

func (s *FlowStoreTestSuite) TestGetFlowVersionDBError() {
	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(nil, errors.New("connection error"))

	version, err := s.store.GetFlowVersion(context.Background(), "flow-1", 1)

	s.Error(err)
	s.Contains(err.Error(), "failed to get database client")
	s.Nil(version)
}

func (s *FlowStoreTestSuite) TestListFlowsWithTypeCountQueryError() {
	expectedError := errors.New("count query failed")
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryCountFlowsWithType,
		"authentication", s.store.deploymentID).Return(
		nil, expectedError)
	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)

	flows, count, err := s.store.ListFlows(context.Background(), 10, 0, "authentication")

	s.Error(err)
	s.Nil(flows)
	s.Equal(0, count)
	s.Contains(err.Error(), "failed to count flows")
}

func (s *FlowStoreTestSuite) TestListFlowsWithTypeQueryError() {
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryCountFlowsWithType,
		"authentication", s.store.deploymentID).Return(
		[]map[string]interface{}{{colCount: int64(5)}}, nil)
	expectedError := errors.New("list query failed")
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryListFlowsWithType,
		"authentication", s.store.deploymentID, 10, 0).Return(
		nil, expectedError)
	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)

	flows, count, err := s.store.ListFlows(context.Background(), 10, 0, "authentication")

	s.Error(err)
	s.Nil(flows)
	s.Equal(0, count)
	s.Contains(err.Error(), "failed to list flows")
}

func (s *FlowStoreTestSuite) TestListFlowsBuildFlowError() {
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryCountFlows, s.store.deploymentID).Return(
		[]map[string]interface{}{{colCount: int64(1)}}, nil)
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryListFlows, s.store.deploymentID, 10, 0).Return(
		[]map[string]interface{}{
			{colFlowID: "flow-1"}, // Missing name field
		}, nil)
	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)

	flows, count, err := s.store.ListFlows(context.Background(), 10, 0, "")

	s.Error(err)
	s.Nil(flows)
	s.Equal(0, count)
	s.Contains(err.Error(), "failed to build flow")
}

func (s *FlowStoreTestSuite) TestListFlowVersionsQueryError() {
	expectedError := errors.New("query failed")
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryListFlowVersions, "flow-123", s.store.deploymentID).Return(
		nil, expectedError)
	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)

	versions, err := s.store.ListFlowVersions(context.Background(), "flow-123")

	s.Error(err)
	s.Nil(versions)
	s.Contains(err.Error(), "failed to list")
}

func (s *FlowStoreTestSuite) TestListFlowVersionsBuildVersionError() {
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryListFlowVersions, "flow-123", s.store.deploymentID).Return(
		[]map[string]interface{}{
			{colVersion: "invalid"}, // Invalid version type
		}, nil)
	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)

	versions, err := s.store.ListFlowVersions(context.Background(), "flow-123")

	s.Error(err)
	s.Empty(versions) // Returns empty slice on error, not nil
	s.Contains(err.Error(), "version field")
}

func (s *FlowStoreTestSuite) TestGetFlowVersionQueryError() {
	expectedError := errors.New("query failed")
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryGetFlowVersionWithMetadata,
		"flow-123", 5, s.store.deploymentID).Return(
		nil, expectedError)
	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)

	version, err := s.store.GetFlowVersion(context.Background(), "flow-123", 5)

	s.Error(err)
	s.Nil(version)
	s.Contains(err.Error(), "failed to get")
}

func (s *FlowStoreTestSuite) TestGetFlowVersionBuildError() {
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryGetFlowVersionWithMetadata,
		"flow-123", 5, s.store.deploymentID).Return(
		[]map[string]interface{}{
			{colFlowID: 123}, // Invalid type - should be string
		}, nil)
	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)

	version, err := s.store.GetFlowVersion(context.Background(), "flow-123", 5)

	s.Error(err)
	s.Nil(version)
	s.Contains(err.Error(), "id field")
}

func (s *FlowStoreTestSuite) TestBuildBasicFlowDefinitionFromRowInvalidActiveVersion() {
	row := map[string]interface{}{
		colFlowID:        "flow-1",
		colHandle:        "test-handle",
		colName:          "Test Flow",
		colFlowType:      "authentication",
		colActiveVersion: "not-an-int", // Invalid type
		colCreatedAt:     "2024-01-01T00:00:00Z",
		colUpdatedAt:     "2024-01-02T00:00:00Z",
	}

	flow, err := s.store.buildBasicFlowDefinitionFromRow(row)

	s.Error(err)
	s.Equal(BasicFlowDefinition{}, flow)
	s.Contains(err.Error(), "active_version field is missing or invalid")
}

func (s *FlowStoreTestSuite) TestBuildCompleteFlowDefinitionFromRowInvalidActiveVersion() {
	row := map[string]interface{}{
		colFlowID:        "flow-1",
		colHandle:        "test-handle",
		colName:          "Test Flow",
		colFlowType:      "authentication",
		colActiveVersion: "invalid", // Invalid type
		colNodes:         "{}",
		colCreatedAt:     "2024-01-01T00:00:00Z",
		colUpdatedAt:     "2024-01-02T00:00:00Z",
	}

	flow, err := s.store.buildCompleteFlowDefinitionFromRow(row)

	s.Error(err)
	s.Nil(flow)
	s.Contains(err.Error(), "active_version field is missing or invalid")
}

func (s *FlowStoreTestSuite) TestBuildBasicFlowVersionFromRowInvalidVersion() {
	row := map[string]interface{}{
		colVersion:       "not-an-int", // Invalid type
		colCreatedAt:     "2024-01-01T00:00:00Z",
		colActiveVersion: int64(1),
	}

	version, err := s.store.buildBasicFlowVersionFromRow(row)

	s.Error(err)
	s.Equal(BasicFlowVersion{}, version)
	s.Contains(err.Error(), "version field is missing or invalid")
}

func (s *FlowStoreTestSuite) TestBuildBasicFlowVersionFromRowInvalidActiveVersion() {
	row := map[string]interface{}{
		colVersion:       int64(1),
		colCreatedAt:     "2024-01-01T00:00:00Z",
		colActiveVersion: "not-an-int", // Invalid type
	}

	version, err := s.store.buildBasicFlowVersionFromRow(row)

	s.Error(err)
	s.Equal(BasicFlowVersion{}, version)
	s.Contains(err.Error(), "active_version field is missing or invalid")
}

func (s *FlowStoreTestSuite) TestBuildFlowVersionFromRowInvalidVersion() {
	row := map[string]interface{}{
		colFlowID:    "flow-1",
		colHandle:    "test-handle",
		colName:      "Test",
		colFlowType:  "authentication",
		colVersion:   "not-an-int", // Invalid type
		colNodes:     "{}",
		colCreatedAt: "2024-01-01T00:00:00Z",
	}

	version, err := s.store.buildFlowVersionFromRow(row)

	s.Error(err)
	s.Nil(version)
	s.Contains(err.Error(), "version field is missing or invalid")
}

func (s *FlowStoreTestSuite) TestBuildFlowVersionFromRowInvalidFlowID() {
	row := map[string]interface{}{
		colFlowID:    123, // Invalid type - should be string
		colName:      "Test",
		colFlowType:  "authentication",
		colVersion:   int64(1),
		colNodes:     "{}",
		colCreatedAt: "2024-01-01T00:00:00Z",
	}

	version, err := s.store.buildFlowVersionFromRow(row)

	s.Error(err)
	s.Nil(version)
	s.Contains(err.Error(), "id field is missing or invalid")
}

// Write Operation Tests

func (s *FlowStoreTestSuite) TestCreateFlow_ExecError() {
	flowDef := &FlowDefinition{
		Handle:   "login-handle",
		Name:     "Login Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes:    []NodeDefinition{{Type: "start", ID: "node1"}},
	}

	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)
	s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryCreateFlow, "flow-1", "login-handle", "Login Flow",
		common.FlowTypeAuthentication, int64(1), s.store.deploymentID).Return(int64(0), errors.New("insert error"))

	result, err := s.store.CreateFlow(context.Background(), "flow-1", flowDef)

	s.Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "failed to create flow")
}

// Helper Function Tests

func (s *FlowStoreTestSuite) TestParseCountResult() {
	tests := []struct {
		name          string
		results       []map[string]interface{}
		expectedCount int
		expectError   bool
	}{
		{"Parse int", []map[string]interface{}{{colCount: 5}}, 5, false},
		{"Parse int64", []map[string]interface{}{{colCount: int64(10)}}, 10, false},
		{"Parse float64", []map[string]interface{}{{colCount: float64(15)}}, 15, false},
		{"Empty results", []map[string]interface{}{}, 0, false},
		{"Missing count field", []map[string]interface{}{{"other": 5}}, 0, true},
		{"Invalid type", []map[string]interface{}{{colCount: "invalid"}}, 0, true},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			count, err := s.store.parseCountResult(tt.results)

			if tt.expectError {
				s.Error(err)
			} else {
				s.NoError(err)
				s.Equal(tt.expectedCount, count)
			}
		})
	}
}

func (s *FlowStoreTestSuite) TestGetString() {
	tests := []struct {
		name        string
		row         map[string]interface{}
		key         string
		expected    string
		expectError bool
	}{
		{"Valid string", map[string]interface{}{"key": "value"}, "key", "value", false},
		{"Missing key", map[string]interface{}{}, "key", "", true},
		{"Invalid type", map[string]interface{}{"key": 123}, "key", "", true},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			value, err := s.store.getString(tt.row, tt.key)

			if tt.expectError {
				s.Error(err)
			} else {
				s.NoError(err)
				s.Equal(tt.expected, value)
			}
		})
	}
}

func (s *FlowStoreTestSuite) TestGetTimestamp() {
	testTime := time.Date(2025, 12, 13, 10, 30, 0, 0, time.UTC)
	expectedTimeStr := testTime.Format(time.RFC3339)

	tests := []struct {
		name        string
		row         map[string]interface{}
		key         string
		expected    string
		expectError bool
	}{
		{"Valid string", map[string]interface{}{"key": "2025-12-13T10:30:00Z"}, "key", "2025-12-13T10:30:00Z", false},
		{"Valid time.Time", map[string]interface{}{"key": testTime}, "key", expectedTimeStr, false},
		{"Missing key", map[string]interface{}{}, "key", "", true},
		{"Invalid type", map[string]interface{}{"key": 123}, "key", "", true},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			value, err := s.store.getTimestamp(tt.row, tt.key)

			if tt.expectError {
				s.Error(err)
			} else {
				s.NoError(err)
				s.Equal(tt.expected, value)
			}
		})
	}
}

func (s *FlowStoreTestSuite) TestGetInt64() {
	tests := []struct {
		name        string
		row         map[string]interface{}
		key         string
		expected    int64
		expectError bool
	}{
		{"Valid int64", map[string]interface{}{"key": int64(123)}, "key", int64(123), false},
		{"Missing key", map[string]interface{}{}, "key", 0, true},
		{"Invalid type", map[string]interface{}{"key": "string"}, "key", 0, true},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			value, err := s.store.getInt64(tt.row, tt.key)

			if tt.expectError {
				s.Error(err)
			} else {
				s.NoError(err)
				s.Equal(tt.expected, value)
			}
		})
	}
}

func (s *FlowStoreTestSuite) TestBuildBasicFlowDefinitionFromRow() {
	validRow := map[string]interface{}{
		colFlowID:        "flow-1",
		colHandle:        "test-handle",
		colName:          "Test Flow",
		colFlowType:      string(common.FlowTypeAuthentication),
		colActiveVersion: int64(1),
		colCreatedAt:     "2025-01-01T00:00:00Z",
		colUpdatedAt:     "2025-01-01T00:00:00Z",
	}

	flow, err := s.store.buildBasicFlowDefinitionFromRow(validRow)

	s.NoError(err)
	s.Equal("flow-1", flow.ID)
	s.Equal("Test Flow", flow.Name)
	s.Equal(common.FlowTypeAuthentication, flow.FlowType)
	s.Equal(1, flow.ActiveVersion)
}

func (s *FlowStoreTestSuite) TestBuildBasicFlowDefinitionFromRowMissingField() {
	invalidRow := map[string]interface{}{
		colFlowID: "flow-1",
	}

	flow, err := s.store.buildBasicFlowDefinitionFromRow(invalidRow)

	s.Error(err)
	s.Empty(flow.ID)
}

func (s *FlowStoreTestSuite) TestBuildCompleteFlowDefinitionFromRow() {
	nodesJSON := `[{"id":"node-1","type":"basic-auth"}]`

	validRow := map[string]interface{}{
		colFlowID:        "flow-1",
		colHandle:        "test-handle",
		colName:          "Test Flow",
		colFlowType:      string(common.FlowTypeAuthentication),
		colActiveVersion: int64(1),
		colNodes:         nodesJSON,
		colCreatedAt:     "2025-01-01T00:00:00Z",
		colUpdatedAt:     "2025-01-01T00:00:00Z",
	}

	flow, err := s.store.buildCompleteFlowDefinitionFromRow(validRow)

	s.NoError(err)
	s.NotNil(flow)
	s.Equal("flow-1", flow.ID)
	s.Equal("Test Flow", flow.Name)
	s.Len(flow.Nodes, 1)
	s.Equal("node-1", flow.Nodes[0].ID)
}

func (s *FlowStoreTestSuite) TestBuildCompleteFlowDefinitionFromRowInvalidJSON() {
	invalidRow := map[string]interface{}{
		colFlowID:        "flow-1",
		colHandle:        "test-handle",
		colName:          "Test Flow",
		colFlowType:      string(common.FlowTypeAuthentication),
		colActiveVersion: int64(1),
		colNodes:         "invalid-json",
		colCreatedAt:     "2025-01-01T00:00:00Z",
		colUpdatedAt:     "2025-01-01T00:00:00Z",
	}

	flow, err := s.store.buildCompleteFlowDefinitionFromRow(invalidRow)

	s.Error(err)
	s.Nil(flow)
	s.Contains(err.Error(), "failed to unmarshal nodes")
}

func (s *FlowStoreTestSuite) TestBuildBasicFlowVersionFromRow() {
	validRow := map[string]interface{}{
		colVersion:       int64(2),
		colCreatedAt:     "2025-01-02T00:00:00Z",
		colActiveVersion: int64(3),
	}

	version, err := s.store.buildBasicFlowVersionFromRow(validRow)

	s.NoError(err)
	s.Equal(2, version.Version)
	s.Equal("2025-01-02T00:00:00Z", version.CreatedAt)
	s.False(version.IsActive)
}

func (s *FlowStoreTestSuite) TestBuildFlowVersionFromRow() {
	nodesJSON := `[{"id":"node-1","type":"basic-auth"}]`

	validRow := map[string]interface{}{
		colFlowID:        "flow-1",
		colHandle:        "test-handle",
		colName:          "Test Flow",
		colFlowType:      string(common.FlowTypeAuthentication),
		colVersion:       int64(2),
		colActiveVersion: int64(2),
		colNodes:         nodesJSON,
		colCreatedAt:     "2025-01-02T00:00:00Z",
	}

	version, err := s.store.buildFlowVersionFromRow(validRow)

	s.NoError(err)
	s.NotNil(version)
	s.Equal("flow-1", version.ID)
	s.Equal(2, version.Version)
	s.True(version.IsActive)
	s.Len(version.Nodes, 1)
}

func (s *FlowStoreTestSuite) TestGetConfigDBClientError() {
	mockProvider := providermock.NewDBProviderInterfaceMock(s.T())
	mockProvider.EXPECT().GetConfigDBClient().Return(nil, errors.New("database connection failed"))

	s.store.dbProvider = mockProvider

	_, err := s.store.getConfigDBClient()

	s.Error(err)
	s.Contains(err.Error(), "failed to get database client")
}

func (s *FlowStoreTestSuite) TestCreateFlow_InsertFlowVersionError() {
	flowDef := &FlowDefinition{
		Handle:   "login-handle",
		Name:     "Login Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes:    []NodeDefinition{{Type: "start", ID: "node1"}},
	}

	nodesJSON := `[{"id":"node1","type":"start"}]`
	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)
	s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryCreateFlow, "flow-1", "login-handle", "Login Flow",
		common.FlowTypeAuthentication, int64(1), s.store.deploymentID).Return(int64(0), nil)
	s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryInsertFlowVersion, "flow-1", 1, nodesJSON,
		s.store.deploymentID).Return(int64(0), errors.New("version insert error"))

	result, err := s.store.CreateFlow(context.Background(), "flow-1", flowDef)

	s.Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "failed to create flow version")
}

// UpdateFlow - Flow Not Found: QueryContext returns empty results -> errFlowNotFound
func (s *FlowStoreTestSuite) TestUpdateFlow_FlowNotFound() {
	flowDef := &FlowDefinition{
		Handle:   "updated-handle",
		Name:     "Updated Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes:    []NodeDefinition{},
	}

	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryGetFlow, "flow-1", s.store.deploymentID).
		Return([]map[string]interface{}{}, nil)

	result, err := s.store.UpdateFlow(context.Background(), "flow-1", flowDef)

	s.Error(err)
	s.Nil(result)
	s.ErrorIs(err, errFlowNotFound)
}

func (s *FlowStoreTestSuite) TestUpdateFlow_PushToVersionStackError() {
	flowDef := &FlowDefinition{
		Handle:   "updated-handle",
		Name:     "Updated Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes:    []NodeDefinition{},
	}

	flowData := []map[string]interface{}{{
		colFlowID:        "flow-1",
		colHandle:        "updated-handle",
		colName:          "Updated Flow",
		colFlowType:      "authentication",
		colActiveVersion: int64(3),
		colNodes:         "[]",
		colCreatedAt:     "2025-01-01T00:00:00Z",
		colUpdatedAt:     "2025-01-01T00:00:00Z",
	}}

	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryGetFlow, "flow-1", s.store.deploymentID).
		Return(flowData, nil)
	s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryInsertFlowVersion, "flow-1", 4, "[]",
		s.store.deploymentID).Return(int64(0), errors.New("insert version error"))

	result, err := s.store.UpdateFlow(context.Background(), "flow-1", flowDef)

	s.Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "failed to insert flow version")
}

func (s *FlowStoreTestSuite) TestRestoreFlowVersion_FlowNotFound() {
	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryGetFlow, "flow-1", s.store.deploymentID).
		Return([]map[string]interface{}{}, nil)

	result, err := s.store.RestoreFlowVersion(context.Background(), "flow-1", 1)

	s.Error(err)
	s.Nil(result)
	s.ErrorIs(err, errFlowNotFound)
}

func (s *FlowStoreTestSuite) TestRestoreFlowVersion_GetVersionQueryError() {
	flowData := []map[string]interface{}{{
		colFlowID:        "flow-2",
		colHandle:        "some-handle",
		colName:          "Some Flow",
		colFlowType:      "authentication",
		colActiveVersion: int64(2),
		colNodes:         "[]",
		colCreatedAt:     "2025-01-01T00:00:00Z",
		colUpdatedAt:     "2025-01-01T00:00:00Z",
	}}

	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryGetFlow, "flow-2", s.store.deploymentID).
		Return(flowData, nil)
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryGetFlowVersion, "flow-2", 2, s.store.deploymentID).
		Return(nil, errors.New("version query error"))

	result, err := s.store.RestoreFlowVersion(context.Background(), "flow-2", 2)

	s.Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "failed to get version to restore")
}

func (s *FlowStoreTestSuite) TestRestoreFlowVersion_PushToVersionStackError() {
	flowData := []map[string]interface{}{{
		colFlowID:        "flow-3",
		colHandle:        "some-handle",
		colName:          "Some Flow",
		colFlowType:      "authentication",
		colActiveVersion: int64(1),
		colNodes:         "[]",
		colCreatedAt:     "2025-01-01T00:00:00Z",
		colUpdatedAt:     "2025-01-01T00:00:00Z",
	}}
	versionData := []map[string]interface{}{{
		colNodes: "[]",
	}}

	s.mockDBProvider.EXPECT().GetConfigDBClient().Return(s.mockDBClient, nil)
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryGetFlow, "flow-3", s.store.deploymentID).
		Return(flowData, nil)
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryGetFlowVersion, "flow-3", 1, s.store.deploymentID).
		Return(versionData, nil)
	s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryInsertFlowVersion, "flow-3", 2, "[]",
		s.store.deploymentID).Return(int64(0), errors.New("insert error"))

	result, err := s.store.RestoreFlowVersion(context.Background(), "flow-3", 1)

	s.Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "failed to insert flow version")
}

func (s *FlowStoreTestSuite) TestPushToVersionStack_CountVersionsQueryError() {
	mockDBClient := providermock.NewDBClientInterfaceMock(s.T())

	mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryInsertFlowVersion,
		"flow-1", 2, `[]`, s.store.deploymentID).
		Return(int64(0), nil)
	mockDBClient.EXPECT().QueryContext(mock.Anything, queryCountFlowVersions, "flow-1", s.store.deploymentID).
		Return(nil, errors.New("count query error"))

	err := s.store.pushToVersionStack(context.Background(), mockDBClient, "flow-1", 2, `[]`)

	s.Error(err)
	s.Contains(err.Error(), "failed to count versions")
}

func (s *FlowStoreTestSuite) TestPushToVersionStack_DeleteOldestVersionError() {
	mockDBClient := providermock.NewDBClientInterfaceMock(s.T())

	countResults := []map[string]interface{}{{"count": int64(6)}}

	mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryInsertFlowVersion,
		"flow-1", 2, `[]`, s.store.deploymentID).
		Return(int64(0), nil)
	mockDBClient.EXPECT().QueryContext(mock.Anything, queryCountFlowVersions, "flow-1", s.store.deploymentID).
		Return(countResults, nil)
	mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryDeleteOldestVersion, "flow-1", s.store.deploymentID).
		Return(int64(0), errors.New("delete error"))

	err := s.store.pushToVersionStack(context.Background(), mockDBClient, "flow-1", 2, `[]`)

	s.Error(err)
	s.Contains(err.Error(), "failed to delete oldest version")
}

func (s *FlowStoreTestSuite) TestPushToVersionStack_InsertVersionError() {
	mockDBClient := providermock.NewDBClientInterfaceMock(s.T())

	mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryInsertFlowVersion,
		"flow-1", 2, `[]`, s.store.deploymentID).
		Return(int64(0), errors.New("insert error"))

	err := s.store.pushToVersionStack(context.Background(), mockDBClient, "flow-1", 2, `[]`)

	s.Error(err)
	s.Contains(err.Error(), "failed to insert flow version")
}

func (s *FlowStoreTestSuite) TestGetMaxVersionHistory() {
	tests := []struct {
		name     string
		config   int
		expected int
	}{
		{"Default value", 0, defaultVersionHistory},
		{"Valid value", 20, 20},
		{"Exceeds max", 200, maxAllowedVersionHistory},
		{"Negative value", -5, defaultVersionHistory},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			// Reset before reinitializing config for each test case
			config.ResetServerRuntime()
			err := config.InitializeServerRuntime("test", &config.Config{
				Server: config.ServerConfig{Identifier: "test-deployment"},
				Flow:   config.FlowConfig{MaxVersionHistory: tt.config},
			})
			s.NoError(err)

			result := getMaxVersionHistory()

			s.Equal(tt.expected, result)
		})
	}
}
