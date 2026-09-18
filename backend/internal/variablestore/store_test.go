// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package variablestore

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

const testDeploymentID = "test-deployment-id"

type StoreTestSuite struct {
	suite.Suite
	providerMock *providermock.DBProviderInterfaceMock
	dbClientMock *providermock.DBClientInterfaceMock
	store        *store
}

func TestStoreSuite(t *testing.T) {
	suite.Run(t, new(StoreTestSuite))
}

func (s *StoreTestSuite) SetupTest() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("", &config.Config{
		Server: engineconfig.ServerConfig{Identifier: testDeploymentID},
	})
	s.providerMock = providermock.NewDBProviderInterfaceMock(s.T())
	s.dbClientMock = providermock.NewDBClientInterfaceMock(s.T())
	s.store = &store{dbProvider: s.providerMock}
}

func (s *StoreTestSuite) expectDBClient() {
	s.providerMock.On("GetConfigDBClient").Return(s.dbClientMock, nil)
}

func (s *StoreTestSuite) TestGetVariableReadsTheDeploymentScopedRow() {
	s.expectDBClient()
	s.dbClientMock.On("QueryContext", mock.Anything, queryGetVariable, "API_URL", testDeploymentID).
		Return([]map[string]interface{}{{
			"name": "API_URL", "value": "https://example.test", "description": "",
		}}, nil).Once()

	variable, err := s.store.GetVariable(context.Background(), "API_URL")

	s.Require().NoError(err)
	s.Require().NotNil(variable)
	s.Equal("https://example.test", variable.Value)
}

// A name that is not stored is not an error: the caller asked whether it exists.
func (s *StoreTestSuite) TestGetVariableReturnsNilForAMissingRow() {
	s.expectDBClient()
	s.dbClientMock.On("QueryContext", mock.Anything, queryGetVariable, "ABSENT", testDeploymentID).
		Return([]map[string]interface{}{}, nil).Once()

	variable, err := s.store.GetVariable(context.Background(), "ABSENT")

	s.Require().NoError(err)
	s.Nil(variable)
}

func (s *StoreTestSuite) TestGetVariableWrapsAQueryFailure() {
	s.expectDBClient()
	s.dbClientMock.On("QueryContext", mock.Anything, queryGetVariable, "API_URL", testDeploymentID).
		Return(nil, errors.New("connection reset")).Once()

	_, err := s.store.GetVariable(context.Background(), "API_URL")

	s.Require().Error(err)
	s.Contains(err.Error(), "failed to read variable")
}

func (s *StoreTestSuite) TestInsertVariablePassesDeploymentLast() {
	s.expectDBClient()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryInsertVariable,
		"API_URL", "https://example.test", "a url", testDeploymentID).
		Return(int64(1), nil).Once()

	inserted, err := s.store.InsertVariable(context.Background(), Variable{
		Name: "API_URL", Value: "https://example.test", Description: "a url"})

	s.Require().NoError(err)
	s.True(inserted)
}

func (s *StoreTestSuite) TestDeleteVariableIsDeploymentScoped() {
	s.expectDBClient()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryDeleteVariable, "API_URL", testDeploymentID).
		Return(int64(1), nil).Once()

	s.Require().NoError(s.store.DeleteVariable(context.Background(), "API_URL"))
}

// The query that reads a secret must not select its value. This asserts the SQL itself, because the
// guarantee is only as good as the statement.
func (s *StoreTestSuite) TestSecretQueriesNeverSelectTheValue() {
	for _, query := range []struct {
		name string
		sql  string
	}{
		{"get", querySecretExists.Query},
		{"list", queryListSecrets.Query},
	} {
		s.NotContains(query.sql, "VALUE", "the %s query must not read a secret's value", query.name)
	}
}

func (s *StoreTestSuite) TestInsertSecretStoresWhateverItIsGiven() {
	s.expectDBClient()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryInsertSecret,
		"DB_PASSWORD", "sealed-bytes", "", testDeploymentID).
		Return(int64(1), nil).Once()

	inserted, err := s.store.InsertSecret(context.Background(), "DB_PASSWORD", "sealed-bytes", "")

	s.Require().NoError(err)
	s.True(inserted)
}

func (s *StoreTestSuite) TestListVariablesPagesAndReportsTheTotal() {
	s.expectDBClient()
	rows := []map[string]interface{}{
		{"name": "C"}, {"name": "A"}, {"name": "B"}, {"name": "D"},
	}
	s.dbClientMock.On("QueryContext", mock.Anything, queryListVariables, testDeploymentID).
		Return(rows, nil).Once()

	page, total, err := s.store.ListVariables(context.Background(), listQuery{limit: 2, offset: 1})

	s.Require().NoError(err)
	s.Equal(4, total)
	s.Require().Len(page, 2)
	// Sorted before paging, so a page is stable across requests.
	s.Equal("B", page[0].Name)
	s.Equal("C", page[1].Name)
}

func (s *StoreTestSuite) TestListVariablesNarrowsByName() {
	s.expectDBClient()
	rows := []map[string]interface{}{{"name": "DB_HOST"}, {"name": "DB_PORT"}, {"name": "API_URL"}}
	s.dbClientMock.On("QueryContext", mock.Anything, queryListVariables, testDeploymentID).
		Return(rows, nil).Once()

	page, total, err := s.store.ListVariables(context.Background(),
		listQuery{limit: 10, namePrefix: "DB_"})

	s.Require().NoError(err)
	// The total counts what the query matched, not what the table holds.
	s.Equal(2, total)
	s.Len(page, 2)
}

func (s *StoreTestSuite) TestListSecretsNarrowsByNamesRequested() {
	s.expectDBClient()
	rows := []map[string]interface{}{{"name": "A"}, {"name": "B"}, {"name": "C"}}
	s.dbClientMock.On("QueryContext", mock.Anything, queryListSecrets, testDeploymentID).
		Return(rows, nil).Once()

	page, total, err := s.store.ListSecrets(context.Background(),
		listQuery{limit: 10, names: []string{"A", "C", "NOT_STORED"}})

	s.Require().NoError(err)
	// A requested name with nothing stored is simply absent, not an error.
	s.Equal(2, total)
	s.Require().Len(page, 2)
	s.True(page[0].Exists)
}

func (s *StoreTestSuite) TestAnOffsetPastTheEndIsAnEmptyPage() {
	s.expectDBClient()
	s.dbClientMock.On("QueryContext", mock.Anything, queryListVariables, testDeploymentID).
		Return([]map[string]interface{}{{"name": "A"}}, nil).Once()

	page, total, err := s.store.ListVariables(context.Background(), listQuery{limit: 10, offset: 500})

	s.Require().NoError(err)
	s.Equal(1, total)
	s.Empty(page)
}

// The insert decides. One row inserted means the name was free, so this created it.
func (s *StoreTestSuite) TestUpsertCreatesWhenTheInsertTakesTheName() {
	s.expectDBClient()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryUpsertVariable,
		"NEW", "v", "", testDeploymentID).Return(int64(1), nil).Once()

	created, err := s.store.UpsertVariable(context.Background(), Variable{Name: "NEW", Value: "v"})

	s.Require().NoError(err)
	s.True(created)
	// No update ran, and no read was taken beforehand: neither is expected on the mock.
}

// No row inserted means the name was taken, so the update replaces what is there.
func (s *StoreTestSuite) TestUpsertReplacesWhenTheInsertFindsTheNameTaken() {
	s.expectDBClient()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryUpsertVariable,
		"API_URL", "v", "", testDeploymentID).Return(int64(0), nil).Once()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryUpdateVariable,
		"API_URL", "v", "", testDeploymentID).Return(int64(1), nil).Once()

	created, err := s.store.UpsertVariable(context.Background(), Variable{Name: "API_URL", Value: "v"})

	s.Require().NoError(err)
	s.False(created)
}

// Drivers differ on whether text arrives as a string or as bytes, and column names arrive in either
// case. A row must read the same either way.
func TestRowStringToleratesDriverShapes(t *testing.T) {
	row := map[string]interface{}{"NAME": []byte("API_URL"), "value": "v", "description": nil}
	if got := rowString(row, "name"); got != "API_URL" {
		t.Fatalf("uppercase bytes column: got %q", got)
	}
	if got := rowString(row, "value"); got != "v" {
		t.Fatalf("string column: got %q", got)
	}
	if got := rowString(row, "description"); got != "" {
		t.Fatalf("null column: got %q", got)
	}
	if got := rowString(row, "absent"); got != "" {
		t.Fatalf("missing column: got %q", got)
	}
}

func (s *StoreTestSuite) TestGetSecretReportsExistenceWithoutAValue() {
	s.expectDBClient()
	s.dbClientMock.On("QueryContext", mock.Anything, querySecretExists, "DB_PASSWORD", testDeploymentID).
		Return([]map[string]interface{}{{"name": "DB_PASSWORD", "description": "the database"}}, nil).Once()

	secret, err := s.store.GetSecret(context.Background(), "DB_PASSWORD")

	s.Require().NoError(err)
	s.Require().NotNil(secret)
	s.True(secret.Exists)
	s.Equal("the database", secret.Description)
}

func (s *StoreTestSuite) TestGetSecretReturnsNilForAMissingRow() {
	s.expectDBClient()
	s.dbClientMock.On("QueryContext", mock.Anything, querySecretExists, "ABSENT", testDeploymentID).
		Return([]map[string]interface{}{}, nil).Once()

	secret, err := s.store.GetSecret(context.Background(), "ABSENT")

	s.Require().NoError(err)
	s.Nil(secret)
}

func (s *StoreTestSuite) TestUpsertSecretRotatesWhenTheNameIsTaken() {
	s.expectDBClient()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryUpsertSecret,
		"DB_PASSWORD", "sealed", "", testDeploymentID).Return(int64(0), nil).Once()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryUpdateSecret,
		"DB_PASSWORD", "sealed", "", testDeploymentID).Return(int64(1), nil).Once()

	created, err := s.store.UpsertSecret(context.Background(), "DB_PASSWORD", "sealed", "")

	s.Require().NoError(err)
	s.False(created)
}

func (s *StoreTestSuite) TestUpsertSecretStoresWhenTheNameIsFree() {
	s.expectDBClient()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryUpsertSecret,
		"NEW", "sealed", "", testDeploymentID).Return(int64(1), nil).Once()

	created, err := s.store.UpsertSecret(context.Background(), "NEW", "sealed", "")

	s.Require().NoError(err)
	s.True(created)
}

func (s *StoreTestSuite) TestDeleteSecretIsDeploymentScoped() {
	s.expectDBClient()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryDeleteSecret, "DB_PASSWORD", testDeploymentID).
		Return(int64(1), nil).Once()

	s.Require().NoError(s.store.DeleteSecret(context.Background(), "DB_PASSWORD"))
}

func (s *StoreTestSuite) TestAProviderFailureIsReported() {
	s.providerMock.On("GetConfigDBClient").Return(nil, errors.New("no pool")).Once()

	_, err := s.store.GetVariable(context.Background(), "API_URL")

	s.Require().Error(err)
	s.Contains(err.Error(), "failed to get database client")
}

// Every write reports a database failure rather than returning as if it had succeeded. Written as a
// table because the branch is the same shape in each, and a missed one is silent data loss.
func (s *StoreTestSuite) TestEveryWriteReportsADatabaseFailure() {
	failure := errors.New("deadlock detected")

	writes := []struct {
		name    string
		query   dbmodel.DBQuery
		args    []interface{}
		call    func() error
		wantMsg string
	}{
		{
			name: "insert variable", query: queryInsertVariable,
			args: []interface{}{"A", "v", "", testDeploymentID},
			call: func() error {
				_, err := s.store.InsertVariable(context.Background(), Variable{Name: "A", Value: "v"})
				return err
			},
			wantMsg: "failed to insert variable",
		},
		{
			name: "delete variable", query: queryDeleteVariable,
			args: []interface{}{"A", testDeploymentID},
			call: func() error { return s.store.DeleteVariable(context.Background(), "A") },

			wantMsg: "failed to delete variable",
		},
		{
			name: "insert secret", query: queryInsertSecret,
			args: []interface{}{"A", "sealed", "", testDeploymentID},
			call: func() error {
				_, err := s.store.InsertSecret(context.Background(), "A", "sealed", "")
				return err
			},
			wantMsg: "failed to insert secret",
		},
		{
			name: "delete secret", query: queryDeleteSecret,
			args: []interface{}{"A", testDeploymentID},
			call: func() error { return s.store.DeleteSecret(context.Background(), "A") },

			wantMsg: "failed to delete secret",
		},
	}

	for _, write := range writes {
		s.Run(write.name, func() {
			s.SetupTest()
			s.expectDBClient()
			call := []interface{}{"ExecuteContext", mock.Anything, write.query}
			call = append(call, write.args...)
			s.dbClientMock.On(call[0].(string), call[1:]...).Return(int64(0), failure).Once()

			err := write.call()

			s.Require().Error(err)
			s.Contains(err.Error(), write.wantMsg)
		})
	}
}

func (s *StoreTestSuite) TestListReportsADatabaseFailure() {
	s.expectDBClient()
	s.dbClientMock.On("QueryContext", mock.Anything, queryListVariables, testDeploymentID).
		Return(nil, errors.New("timeout")).Once()

	_, _, err := s.store.ListVariables(context.Background(), listQuery{limit: 10})

	s.Require().Error(err)
	s.Contains(err.Error(), "failed to list variables")
}

func (s *StoreTestSuite) TestListSecretsReportsADatabaseFailure() {
	s.expectDBClient()
	s.dbClientMock.On("QueryContext", mock.Anything, queryListSecrets, testDeploymentID).
		Return(nil, errors.New("timeout")).Once()

	_, _, err := s.store.ListSecrets(context.Background(), listQuery{limit: 10})

	s.Require().Error(err)
	s.Contains(err.Error(), "failed to list secrets")
}

func (s *StoreTestSuite) TestGetSecretReportsADatabaseFailure() {
	s.expectDBClient()
	s.dbClientMock.On("QueryContext", mock.Anything, querySecretExists, "A", testDeploymentID).
		Return(nil, errors.New("timeout")).Once()

	_, err := s.store.GetSecret(context.Background(), "A")

	s.Require().Error(err)
	s.Contains(err.Error(), "failed to read secret")
}

// A concurrent delete can leave the insert with nothing to insert and the update with nothing to
// update. The write has not happened, so the store tries again rather than reporting success.
func (s *StoreTestSuite) TestUpsertRetriesWhenTheRowIsDeletedInBetween() {
	s.expectDBClient()
	// First round: the name was taken when the insert ran, then gone when the update ran.
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryUpsertVariable,
		"A", "v", "", testDeploymentID).Return(int64(0), nil).Once()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryUpdateVariable,
		"A", "v", "", testDeploymentID).Return(int64(0), nil).Once()
	// Second round: the name is free now, so the insert stores it.
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryUpsertVariable,
		"A", "v", "", testDeploymentID).Return(int64(1), nil).Once()

	created, err := s.store.UpsertVariable(context.Background(), Variable{Name: "A", Value: "v"})

	s.Require().NoError(err)
	s.True(created)
}

// If every attempt is beaten, the caller is told the write failed. Reporting success here would
// claim a value was stored when none was.
func (s *StoreTestSuite) TestUpsertReportsFailureWhenEveryAttemptIsBeaten() {
	s.expectDBClient()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryUpsertVariable,
		"A", "v", "", testDeploymentID).Return(int64(0), nil).Times(upsertAttempts)
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryUpdateVariable,
		"A", "v", "", testDeploymentID).Return(int64(0), nil).Times(upsertAttempts)

	_, err := s.store.UpsertVariable(context.Background(), Variable{Name: "A", Value: "v"})

	s.Require().Error(err)
	s.Contains(err.Error(), "deleted during each attempt")
}

// A create does not overwrite. The insert stores only when the name is free, and says which it did.
func (s *StoreTestSuite) TestInsertReportsWhenTheNameIsTaken() {
	s.expectDBClient()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryInsertVariable,
		"A", "v", "", testDeploymentID).Return(int64(0), nil).Once()

	inserted, err := s.store.InsertVariable(context.Background(), Variable{Name: "A", Value: "v"})

	s.Require().NoError(err)
	s.False(inserted)
}
