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

package consent

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

const testDeploymentID = "test-server-id"

type ConsentStoreTestSuite struct {
	suite.Suite
	mockDBProvider *providermock.DBProviderInterfaceMock
	mockDBClient   *providermock.DBClientInterfaceMock
	store          *consentStore
}

func TestConsentStoreTestSuite(t *testing.T) {
	suite.Run(t, new(ConsentStoreTestSuite))
}

func (s *ConsentStoreTestSuite) SetupTest() {
	s.mockDBProvider = providermock.NewDBProviderInterfaceMock(s.T())
	s.mockDBClient = providermock.NewDBClientInterfaceMock(s.T())
	s.store = &consentStore{
		dbProvider:   s.mockDBProvider,
		deploymentID: testDeploymentID,
	}
}

// queryWithID matches a dbmodel.DBQuery by its ID, used for queries built dynamically at call time.
func queryWithID(id string) interface{} {
	return mock.MatchedBy(func(q dbmodel.DBQuery) bool { return q.ID == id })
}

// anyArgs returns a []interface{} of n mock.Anything matchers prefixed by ctx + query matchers, so
// the unrolled variadic ExecuteContext/QueryContext expectation matches a call with n variadic args.
func anyArgs(queryMatcher interface{}, nVariadic int) []interface{} {
	args := []interface{}{mock.Anything, queryMatcher}
	for i := 0; i < nVariadic; i++ {
		args = append(args, mock.Anything)
	}
	return args
}

const samplePurposesJSON = `[{"name":"attributes:app1","elements":` +
	`[{"name":"email","namespace":"attribute","isUserApproved":true}]}]`

func sampleConsentRow(id string) map[string]interface{} {
	return map[string]interface{}{
		"id":            id,
		"group_id":      "app1",
		"status":        "ACTIVE",
		"validity_time": nil,
		"purposes":      samplePurposesJSON,
	}
}

func sampleAuthRow(consentID, id string) map[string]interface{} {
	return map[string]interface{}{
		"consent_id":   consentID,
		"id":           id,
		"user_id":      "user1",
		"type":         "AUTHORIZATION",
		"status":       "APPROVED",
		"updated_time": nil,
	}
}

// CreateConsent

func (s *ConsentStoreTestSuite) TestCreateConsent_Success() {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", anyArgs(QueryCreateConsent, 8)...).Return(int64(1), nil).Once()
	s.mockDBClient.On("ExecuteContext", anyArgs(queryWithID("CNQ-CONSENT_MGT-05"), 7)...).
		Return(int64(1), nil).Once()

	consent := &Consent{
		ID:      "c1",
		GroupID: "app1",
		Status:  ConsentStatusActive,
		Authorizations: []ConsentAuthorization{
			{ID: "a1", UserID: "user1", Type: AuthorizationTypeAuthorization, Status: AuthorizationStatusApproved},
		},
	}

	s.NoError(s.store.CreateConsent(context.Background(), consent))
}

func (s *ConsentStoreTestSuite) TestCreateConsent_NoAuthorizations_SkipsInsert() {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", anyArgs(QueryCreateConsent, 8)...).Return(int64(1), nil).Once()

	consent := &Consent{ID: "c1", GroupID: "app1", Status: ConsentStatusActive}

	s.NoError(s.store.CreateConsent(context.Background(), consent))
	s.mockDBClient.AssertNotCalled(s.T(), "ExecuteContext", anyArgs(queryWithID("CNQ-CONSENT_MGT-05"), 7)...)
}

// GetConsent

func (s *ConsentStoreTestSuite) TestGetConsent_Found() {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", anyArgs(QueryGetConsentByID, 2)...).
		Return([]map[string]interface{}{sampleConsentRow("c1")}, nil).Once()
	s.mockDBClient.On("QueryContext", anyArgs(queryWithID("CNQ-CONSENT_MGT-06"), 2)...).
		Return([]map[string]interface{}{sampleAuthRow("c1", "a1")}, nil).Once()

	consent, err := s.store.GetConsent(context.Background(), "c1")

	s.NoError(err)
	s.Equal("c1", consent.ID)
	s.Equal("app1", consent.GroupID)
	s.Equal(ConsentStatusActive, consent.Status)
	s.Len(consent.Purposes, 1)
	s.Equal("attributes:app1", consent.Purposes[0].Name)
	s.Len(consent.Authorizations, 1)
	s.Equal("a1", consent.Authorizations[0].ID)
	s.Equal("user1", consent.Authorizations[0].UserID)
}

func (s *ConsentStoreTestSuite) TestGetConsent_NotFound() {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", anyArgs(QueryGetConsentByID, 2)...).
		Return([]map[string]interface{}{}, nil).Once()

	consent, err := s.store.GetConsent(context.Background(), "c1")

	s.Nil(consent)
	s.ErrorIs(err, errConsentNotFound)
}

// UpdateConsent

func (s *ConsentStoreTestSuite) TestUpdateConsent_Success() {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", anyArgs(QueryUpdateConsent, 6)...).Return(int64(1), nil).Once()
	s.mockDBClient.On("ExecuteContext", anyArgs(QueryDeleteConsentAuthorizations, 2)...).
		Return(int64(1), nil).Once()
	s.mockDBClient.On("ExecuteContext", anyArgs(queryWithID("CNQ-CONSENT_MGT-05"), 7)...).
		Return(int64(1), nil).Once()

	consent := &Consent{
		ID:      "c1",
		GroupID: "app1",
		Status:  ConsentStatusActive,
		Authorizations: []ConsentAuthorization{
			{ID: "a1", UserID: "user1", Type: AuthorizationTypeAuthorization, Status: AuthorizationStatusApproved},
		},
	}

	s.NoError(s.store.UpdateConsent(context.Background(), consent))
}

func (s *ConsentStoreTestSuite) TestUpdateConsent_NotFound() {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", anyArgs(QueryUpdateConsent, 6)...).Return(int64(0), nil).Once()

	consent := &Consent{ID: "c1", GroupID: "app1", Status: ConsentStatusActive}

	err := s.store.UpdateConsent(context.Background(), consent)
	s.ErrorIs(err, errConsentNotFound)
}

// SearchConsents

func (s *ConsentStoreTestSuite) TestSearchConsents_ReturnsResultsWithAuthorizations() {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", anyArgs(queryWithID("CNQ-CONSENT_MGT-07"), 2)...).
		Return([]map[string]interface{}{sampleConsentRow("c1"), sampleConsentRow("c2")}, nil).Once()
	s.mockDBClient.On("QueryContext", anyArgs(queryWithID("CNQ-CONSENT_MGT-06"), 3)...).
		Return([]map[string]interface{}{
			sampleAuthRow("c1", "a1"),
			sampleAuthRow("c2", "a2"),
		}, nil).Once()

	consents, err := s.store.SearchConsents(context.Background(), ConsentFilter{GroupID: "app1"})

	s.NoError(err)
	s.Len(consents, 2)
	byID := map[string]*Consent{}
	for _, c := range consents {
		byID[c.ID] = c
	}
	s.Len(byID["c1"].Authorizations, 1)
	s.Equal("a1", byID["c1"].Authorizations[0].ID)
	s.Len(byID["c2"].Authorizations, 1)
	s.Equal("a2", byID["c2"].Authorizations[0].ID)
}

func (s *ConsentStoreTestSuite) TestSearchConsents_NoResults() {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", anyArgs(queryWithID("CNQ-CONSENT_MGT-07"), 2)...).
		Return([]map[string]interface{}{}, nil).Once()

	consents, err := s.store.SearchConsents(context.Background(), ConsentFilter{GroupID: "app1"})

	s.NoError(err)
	s.Empty(consents)
	s.mockDBClient.AssertNotCalled(s.T(), "QueryContext", anyArgs(queryWithID("CNQ-CONSENT_MGT-06"), 2)...)
}

// Query builders

func (s *ConsentStoreTestSuite) TestBuildSearchConsentsQuery() {
	s.Run("group only", func() {
		q, args := buildSearchConsentsQuery(ConsentFilter{GroupID: "app1"}, testDeploymentID)
		s.NotContains(q.Query, "INNER JOIN")
		s.Equal([]interface{}{testDeploymentID, "app1"}, args)
	})

	s.Run("with user joins authorization", func() {
		q, args := buildSearchConsentsQuery(ConsentFilter{GroupID: "app1", UserID: "user1"}, testDeploymentID)
		s.Contains(q.Query, "INNER JOIN")
		s.Equal([]interface{}{testDeploymentID, "app1", "user1"}, args)
	})

	s.Run("deployment only", func() {
		_, args := buildSearchConsentsQuery(ConsentFilter{}, testDeploymentID)
		s.Equal([]interface{}{testDeploymentID}, args)
	})
}

func (s *ConsentStoreTestSuite) TestBuildInsertConsentAuthorizationsQuery() {
	auths := []ConsentAuthorization{
		{ID: "a1", UserID: "u1", Type: AuthorizationTypeAuthorization, Status: AuthorizationStatusApproved},
		{ID: "a2", UserID: "u2", Type: AuthorizationTypeReAuthorization, Status: AuthorizationStatusRejected},
	}
	q, args := buildInsertConsentAuthorizationsQuery("c1", auths, testDeploymentID)

	s.Contains(q.Query, `INSERT INTO "CONSENT_AUTHORIZATION"`)
	s.Contains(q.Query, "($1, $2, $3, $4, $5, $6, $7), ($8, $9, $10, $11, $12, $13, $14)")
	s.Len(args, 14)
	s.Equal("a1", args[0])
	s.Equal("c1", args[1])
}

func (s *ConsentStoreTestSuite) TestBuildGetConsentAuthorizationsQuery() {
	q, args := buildGetConsentAuthorizationsQuery([]string{"c1", "c2"}, testDeploymentID)

	s.Contains(q.Query, "CONSENT_ID IN ($2, $3)")
	s.Equal([]interface{}{testDeploymentID, "c1", "c2"}, args)
}

// Marshaling helpers

func (s *ConsentStoreTestSuite) TestMarshalUnmarshalPurposes_RoundTrip() {
	purposes := []ConsentPurposeItem{
		{Name: "attributes:app1", Elements: []ConsentElementApproval{
			{Name: "email", Namespace: NamespaceAttribute, IsUserApproved: true},
		}},
	}
	data, err := marshalPurposes(purposes)
	s.NoError(err)

	got, err := unmarshalPurposes(data)
	s.NoError(err)
	s.Equal(purposes, got)
}

func (s *ConsentStoreTestSuite) TestUnmarshalPurposes() {
	s.Run("nil", func() {
		got, err := unmarshalPurposes(nil)
		s.NoError(err)
		s.Nil(got)
	})
	s.Run("empty string", func() {
		got, err := unmarshalPurposes("   ")
		s.NoError(err)
		s.Nil(got)
	})
	s.Run("byte slice", func() {
		got, err := unmarshalPurposes([]byte(`[{"name":"p","elements":[]}]`))
		s.NoError(err)
		s.Len(got, 1)
		s.Equal("p", got[0].Name)
	})
	s.Run("invalid json", func() {
		_, err := unmarshalPurposes(`{not json`)
		s.Error(err)
	})
	s.Run("unexpected type", func() {
		_, err := unmarshalPurposes(42)
		s.Error(err)
	})
}

// Column parsers

func (s *ConsentStoreTestSuite) TestParseStringColumn() {
	v, err := parseStringColumn(map[string]interface{}{"id": "x"}, "id")
	s.NoError(err)
	s.Equal("x", v)

	_, err = parseStringColumn(map[string]interface{}{"id": 42}, "id")
	s.Error(err)
}

func (s *ConsentStoreTestSuite) TestParseUnixColumn() {
	s.Run("missing", func() {
		v, err := parseUnixColumn(map[string]interface{}{}, "validity_time")
		s.NoError(err)
		s.Zero(v)
	})
	s.Run("nil", func() {
		v, err := parseUnixColumn(map[string]interface{}{"validity_time": nil}, "validity_time")
		s.NoError(err)
		s.Zero(v)
	})
	s.Run("time value", func() {
		ts := time.Unix(1700000000, 0).UTC()
		v, err := parseUnixColumn(map[string]interface{}{"validity_time": ts}, "validity_time")
		s.NoError(err)
		s.Equal(int64(1700000000), v)
	})
}

func (s *ConsentStoreTestSuite) TestUnixToNullableTime() {
	s.Nil(unixToNullableTime(0))
	s.NotNil(unixToNullableTime(1700000000))
}

// Row builders

func (s *ConsentStoreTestSuite) TestBuildConsentFromResultRow() {
	consent, err := buildConsentFromResultRow(sampleConsentRow("c1"))
	s.NoError(err)
	s.Equal("c1", consent.ID)
	s.Equal("app1", consent.GroupID)
	s.Equal(ConsentStatusActive, consent.Status)
	s.Zero(consent.ValidityTime)
	s.Len(consent.Purposes, 1)
}

func (s *ConsentStoreTestSuite) TestBuildAuthorizationFromResultRow() {
	auth, err := buildAuthorizationFromResultRow(sampleAuthRow("c1", "a1"))
	s.NoError(err)
	s.Equal("a1", auth.ID)
	s.Equal("user1", auth.UserID)
	s.Equal(AuthorizationTypeAuthorization, auth.Type)
	s.Equal(AuthorizationStatusApproved, auth.Status)
}
