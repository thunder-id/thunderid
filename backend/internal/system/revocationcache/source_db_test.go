// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocationcache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

const testDeploymentID = "test-deployment-id"

type DBSourceTestSuite struct {
	suite.Suite
	mockDBProvider *providermock.DBProviderInterfaceMock
	mockDBClient   *providermock.DBClientInterfaceMock
	source         *dbSource
}

func TestDBSourceTestSuite(t *testing.T) {
	suite.Run(t, new(DBSourceTestSuite))
}

func (suite *DBSourceTestSuite) SetupTest() {
	suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
	suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
	suite.source = &dbSource{
		dbProvider:   suite.mockDBProvider,
		deploymentID: testDeploymentID,
	}
}

// expectTokensAndFamilies answers the two queries that run before the bounded criteria, with no rows.
func (suite *DBSourceTestSuite) expectTokensAndFamilies() {
	suite.mockDBProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, querySnapshotRevokedTokens,
		mock.Anything, testDeploymentID).Return([]map[string]interface{}{}, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, querySnapshotRevokedTokenFamilies,
		criterionTypeTokenFamily, mock.Anything, testDeploymentID).Return([]map[string]interface{}{}, nil)
}

// expectBounded answers the one query reading every bounded criterion type.
func (suite *DBSourceTestSuite) expectBounded(rows []map[string]interface{}, err error) {
	args := make([]interface{}, 0, len(boundedCriterionTypes)+4)
	args = append(args, mock.Anything, querySnapshotBoundedCriteria)
	for _, criterionType := range boundedCriterionTypes {
		args = append(args, criterionType)
	}
	suite.mockDBClient.On("QueryContext", append(args, mock.Anything, testDeploymentID)...).Return(rows, err).Once()
}

// boundedRow is one bounded criterion row of the given type.
func boundedRow(criterionType, value, reason string, revokedAt, expiry time.Time) map[string]interface{} {
	return map[string]interface{}{"criterion_type": criterionType, "criterion_value": value, "reason": reason,
		"revoked_at": revokedAt, "expiry_time": expiry}
}

func (suite *DBSourceTestSuite) TestSnapshot_Success() {
	expiry := time.Now().Add(time.Hour).UTC()
	revokedAt := time.Now().UTC()
	suite.mockDBProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, querySnapshotRevokedTokens,
		mock.Anything, testDeploymentID).
		Return([]map[string]interface{}{
			{"jti": "jti-1", "expiry_time": expiry},
			{"jti": "jti-2", "expiry_time": expiry},
		}, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, querySnapshotRevokedTokenFamilies,
		criterionTypeTokenFamily, mock.Anything, testDeploymentID).
		Return([]map[string]interface{}{
			{"criterion_value": "tfid-1", "expiry_time": expiry},
		}, nil)
	suite.expectBounded([]map[string]interface{}{
		boundedRow(criterionTypeSubject, "user-1", "role_assignment_removed", revokedAt, expiry),
		boundedRow(criterionTypeAppKey, "client-1", "application_secret_regenerated", revokedAt, expiry),
		boundedRow(criterionTypeAppKey, "client-2", "user_deleted", revokedAt, expiry),
		boundedRow(criterionTypeSubjectOU, "subject-in-ou", "resource_unshared", revokedAt, expiry),
		boundedRow(criterionTypeAppKeyOU, "client-in-ou", "resource_unshared", revokedAt, expiry),
	}, nil)

	snapshot, err := suite.source.Snapshot(context.Background())

	suite.Require().NoError(err)
	suite.mockDBClient.AssertNumberOfCalls(suite.T(), "QueryContext", 3)
	assert.Len(suite.T(), snapshot.Tokens, 2)
	assert.Equal(suite.T(), "jti-1", snapshot.Tokens[0].Value)
	assert.Equal(suite.T(), expiry, snapshot.Tokens[0].ExpiryTime)
	assert.Len(suite.T(), snapshot.Families, 1)
	assert.Equal(suite.T(), "tfid-1", snapshot.Families[0].Value)
	suite.Require().Len(snapshot.Subjects, 1)
	assert.Equal(suite.T(), "user-1", snapshot.Subjects[0].Value)
	assert.Equal(suite.T(), revokedAt, snapshot.Subjects[0].RevokedAt)
	assert.True(suite.T(), snapshot.Subjects[0].Boundary)

	// Secret regeneration is a boundary reason; application deletion is terminal. The classification
	// must come through on app-key entries exactly as it does on subjects.
	suite.Require().Len(snapshot.AppKeys, 2)
	assert.Equal(suite.T(), "client-1", snapshot.AppKeys[0].Value)
	assert.True(suite.T(), snapshot.AppKeys[0].Boundary)
	assert.Equal(suite.T(), "client-2", snapshot.AppKeys[1].Value)
	assert.False(suite.T(), snapshot.AppKeys[1].Boundary)

	suite.Require().Len(snapshot.SubjectOUs, 1)
	assert.Equal(suite.T(), "subject-in-ou", snapshot.SubjectOUs[0].Value)
	assert.True(suite.T(), snapshot.SubjectOUs[0].Boundary)
	suite.Require().Len(snapshot.AppKeyOUs, 1)
	assert.Equal(suite.T(), "client-in-ou", snapshot.AppKeyOUs[0].Value)
	assert.Empty(suite.T(), snapshot.EntityScopes)
	assert.Empty(suite.T(), snapshot.EntityScopeOUs)
	assert.Empty(suite.T(), snapshot.EntityScopeSubtreeOUs)
}

func (suite *DBSourceTestSuite) TestSnapshot_Empty() {
	suite.expectTokensAndFamilies()
	suite.expectBounded([]map[string]interface{}{}, nil)

	snapshot, err := suite.source.Snapshot(context.Background())

	suite.Require().NoError(err)
	assert.Empty(suite.T(), snapshot.Tokens)
	assert.Empty(suite.T(), snapshot.Families)
	assert.Empty(suite.T(), snapshot.Subjects)
	assert.Empty(suite.T(), snapshot.AppKeys)
	assert.Empty(suite.T(), snapshot.SubjectOUs)
}

// A row of a type the snapshot does not read is left out rather than filed under another type.
func (suite *DBSourceTestSuite) TestSnapshot_RowOfAnUnreadTypeIsLeftOut() {
	expiry := time.Now().Add(time.Hour)
	suite.expectTokensAndFamilies()
	suite.expectBounded([]map[string]interface{}{
		boundedRow("token_family", "tfid-1", "explicit", time.Now(), expiry),
	}, nil)

	snapshot, err := suite.source.Snapshot(context.Background())

	suite.Require().NoError(err)
	assert.Empty(suite.T(), snapshot.Subjects)
	assert.Empty(suite.T(), snapshot.SubjectOUs)
}

func (suite *DBSourceTestSuite) TestSnapshot_BoundedQueryError() {
	suite.expectTokensAndFamilies()
	suite.expectBounded(nil, errors.New("query error"))

	_, err := suite.source.Snapshot(context.Background())

	suite.Require().Error(err)
	suite.Contains(err.Error(), "error reading revoked criteria snapshot")
}

// Each bounded type's rows are parsed the same way, so a malformed row of any of them fails the snapshot.
func (suite *DBSourceTestSuite) TestSnapshot_BoundedParseError() {
	expiry := time.Now().Add(time.Hour)
	for _, criterionType := range boundedCriterionTypes {
		suite.Run(criterionType+"/value", func() {
			suite.SetupTest()
			suite.expectTokensAndFamilies()
			suite.expectBounded([]map[string]interface{}{
				boundedRow(criterionType, "", "resource_unshared", time.Now(), expiry)}, nil)

			_, err := suite.source.Snapshot(context.Background())

			suite.Require().Error(err)
			suite.Contains(err.Error(), "criterion_value")
		})
		suite.Run(criterionType+"/reason", func() {
			suite.SetupTest()
			suite.expectTokensAndFamilies()
			suite.expectBounded([]map[string]interface{}{boundedRow(criterionType, "value", "", time.Now(), expiry)},
				nil)

			_, err := suite.source.Snapshot(context.Background())

			suite.Require().Error(err)
			suite.Contains(err.Error(), "reason")
		})
	}
}

func (suite *DBSourceTestSuite) TestSnapshot_DBClientError() {
	suite.mockDBProvider.On("GetRuntimePersistentDBClient").Return(nil, errors.New("db client error"))

	snapshot, err := suite.source.Snapshot(context.Background())

	assert.Error(suite.T(), err)
	assert.Empty(suite.T(), snapshot.Tokens)
	assert.Contains(suite.T(), err.Error(), "db client error")
}

func (suite *DBSourceTestSuite) TestSnapshot_QueryError() {
	suite.mockDBProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, querySnapshotRevokedTokens,
		mock.Anything, testDeploymentID).
		Return(nil, errors.New("query error"))

	snapshot, err := suite.source.Snapshot(context.Background())

	assert.Error(suite.T(), err)
	assert.Empty(suite.T(), snapshot.Tokens)
	assert.Contains(suite.T(), err.Error(), "error reading revoked token snapshot")
}

func (suite *DBSourceTestSuite) TestSnapshot_TokenFamilyQueryError() {
	suite.mockDBProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, querySnapshotRevokedTokens,
		mock.Anything, testDeploymentID).
		Return([]map[string]interface{}{}, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, querySnapshotRevokedTokenFamilies,
		criterionTypeTokenFamily, mock.Anything, testDeploymentID).
		Return(nil, errors.New("query error"))

	snapshot, err := suite.source.Snapshot(context.Background())

	assert.Error(suite.T(), err)
	assert.Empty(suite.T(), snapshot.Families)
	assert.Contains(suite.T(), err.Error(), "error reading revoked token family snapshot")
}

func (suite *DBSourceTestSuite) TestSnapshot_InvalidJTI() {
	suite.mockDBProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, querySnapshotRevokedTokens,
		mock.Anything, testDeploymentID).
		Return([]map[string]interface{}{
			{"jti": "", "expiry_time": time.Now().Add(time.Hour)},
		}, nil)

	snapshot, err := suite.source.Snapshot(context.Background())

	assert.Error(suite.T(), err)
	assert.Empty(suite.T(), snapshot.Tokens)
	assert.Contains(suite.T(), err.Error(), "jti")
}

func (suite *DBSourceTestSuite) TestSnapshot_InvalidExpiryTime() {
	suite.mockDBProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, querySnapshotRevokedTokens,
		mock.Anything, testDeploymentID).
		Return([]map[string]interface{}{
			{"jti": "jti-1", "expiry_time": 12345},
		}, nil)

	snapshot, err := suite.source.Snapshot(context.Background())

	assert.Error(suite.T(), err)
	assert.Empty(suite.T(), snapshot.Tokens)
	assert.Contains(suite.T(), err.Error(), "error parsing revocation snapshot")
}
