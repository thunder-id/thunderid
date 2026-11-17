// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package variablestore

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	_ "modernc.org/sqlite"

	"github.com/thunder-id/thunderid/internal/system/config"
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

// SQLiteListTestSuite runs the list and count statements against the real schema on SQLite, so the
// narrowing and paging are checked as the database does them rather than as a mock returns them.
type SQLiteListTestSuite struct {
	suite.Suite
	db    *sql.DB
	store *store
}

func TestSQLiteListTestSuite(t *testing.T) {
	suite.Run(t, new(SQLiteListTestSuite))
}

func (s *SQLiteListTestSuite) SetupTest() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("", &config.Config{
		Server: engineconfig.ServerConfig{Identifier: testDeploymentID},
	})

	schema, err := os.ReadFile("../../dbscripts/configdb/sqlite.sql")
	s.Require().NoError(err)
	db, err := sql.Open("sqlite", ":memory:")
	s.Require().NoError(err)
	// One connection, or each would see its own empty in-memory database.
	db.SetMaxOpenConns(1)
	s.T().Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(string(schema))
	s.Require().NoError(err)
	s.db = db

	client := providermock.NewDBClientInterfaceMock(s.T())
	// The count takes five parameters and the page seven; both are answered by the database.
	for _, arity := range []int{5, 7} {
		matchers := []interface{}{mock.Anything, mock.Anything}
		for i := 0; i < arity; i++ {
			matchers = append(matchers, mock.Anything)
		}
		client.On("QueryContext", matchers...).Maybe().Return(
			func(ctx context.Context, q dbmodel.DBQuery, args ...interface{}) []map[string]interface{} {
				rows, _ := s.query(ctx, q, args...)
				return rows
			}, func(ctx context.Context, q dbmodel.DBQuery, args ...interface{}) error {
				_, qErr := s.query(ctx, q, args...)
				return qErr
			})
	}
	provider := providermock.NewDBProviderInterfaceMock(s.T())
	provider.On("GetConfigDBClient").Return(client, nil).Maybe()
	s.store = &store{dbProvider: provider}

	for _, name := range []string{"API_URL", "DB_HOST", "DB_PORT", "DBX", "db_lower", "DB%X", "B", "A"} {
		s.insert("VARIABLE", name, testDeploymentID)
	}
	// Another deployment's rows must never be seen or counted.
	s.insert("VARIABLE", "DB_OTHER", "other-deployment")
	for _, name := range []string{"S_ONE", "S_TWO", "T_ONE"} {
		s.insert("SECRET", name, testDeploymentID)
	}
}

func (s *SQLiteListTestSuite) insert(table, name, deploymentID string) {
	_, err := s.db.Exec(fmt.Sprintf(
		`INSERT INTO %q (NAME, VALUE, DESCRIPTION, DEPLOYMENT_ID) VALUES ($1, 'v', '', $2)`, table),
		name, deploymentID)
	s.Require().NoError(err)
}

// query runs a statement the way the database client does, with column names lowercased.
func (s *SQLiteListTestSuite) query(
	ctx context.Context, q dbmodel.DBQuery, args ...interface{},
) ([]map[string]interface{}, error) {
	rows, err := s.db.QueryContext(ctx, q.GetQuery("sqlite"), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var out []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		pointers := make([]interface{}, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}
		row := map[string]interface{}{}
		for i, column := range columns {
			row[strings.ToLower(column)] = values[i]
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func variableNames(variables []Variable) []string {
	names := make([]string, 0, len(variables))
	for _, v := range variables {
		names = append(names, v.Name)
	}
	return names
}

func (s *SQLiteListTestSuite) TestListsEverythingInByteOrder() {
	page, total, err := s.store.ListVariables(context.Background(), listQuery{limit: 100})

	s.Require().NoError(err)
	s.Equal(8, total)
	s.Equal([]string{"A", "API_URL", "B", "DB%X", "DBX", "DB_HOST", "DB_PORT", "db_lower"}, variableNames(page))
	s.Equal("v", page[0].Value)
}

func (s *SQLiteListTestSuite) TestPagesWithTheFullTotal() {
	page, total, err := s.store.ListVariables(context.Background(), listQuery{limit: 3, offset: 2})

	s.Require().NoError(err)
	s.Equal(8, total)
	s.Equal([]string{"B", "DB%X", "DBX"}, variableNames(page))
}

func (s *SQLiteListTestSuite) TestTheLastPageIsShort() {
	page, total, err := s.store.ListVariables(context.Background(), listQuery{limit: 3, offset: 6})

	s.Require().NoError(err)
	s.Equal(8, total)
	s.Equal([]string{"DB_PORT", "db_lower"}, variableNames(page))
}

func (s *SQLiteListTestSuite) TestAnOffsetPastTheEndIsAnEmptyPage() {
	page, total, err := s.store.ListVariables(context.Background(), listQuery{limit: 3, offset: 8})

	s.Require().NoError(err)
	s.Equal(8, total)
	s.Empty(page)
}

// An underscore in the prefix is a character, not a wildcard, and case is not ignored.
func (s *SQLiteListTestSuite) TestPrefixMatchesLiterally() {
	page, total, err := s.store.ListVariables(context.Background(), listQuery{limit: 100, namePrefix: "DB_"})

	s.Require().NoError(err)
	s.Equal(2, total)
	s.Equal([]string{"DB_HOST", "DB_PORT"}, variableNames(page))
}

// A percent sign in the prefix is a character too. Names cannot hold one today; the escaping holds
// regardless, so a widened name format cannot widen a filter.
func (s *SQLiteListTestSuite) TestPrefixPercentIsLiteral() {
	page, total, err := s.store.ListVariables(context.Background(), listQuery{limit: 100, namePrefix: "DB%"})

	s.Require().NoError(err)
	s.Equal(1, total)
	s.Equal([]string{"DB%X"}, variableNames(page))
}

func (s *SQLiteListTestSuite) TestPrefixPagesWithTheMatchedTotal() {
	page, total, err := s.store.ListVariables(context.Background(),
		listQuery{limit: 1, offset: 1, namePrefix: "DB"})

	s.Require().NoError(err)
	// db_lower is not counted: SQLite's LIKE would match it, the exact comparison does not.
	s.Equal(4, total)
	s.Equal([]string{"DBX"}, variableNames(page))
}

func (s *SQLiteListTestSuite) TestExactNameIsCaseSensitive() {
	page, total, err := s.store.ListVariables(context.Background(), listQuery{limit: 100, nameEquals: "DBX"})

	s.Require().NoError(err)
	s.Equal(1, total)
	s.Equal([]string{"DBX"}, variableNames(page))

	page, total, err = s.store.ListVariables(context.Background(), listQuery{limit: 100, nameEquals: "dbx"})
	s.Require().NoError(err)
	s.Equal(0, total)
	s.Empty(page)
}

func (s *SQLiteListTestSuite) TestNamesSkipsWhatIsNotStored() {
	page, total, err := s.store.ListVariables(context.Background(),
		listQuery{limit: 100, names: []string{"DB_PORT", "A", "NOT_STORED", "DB_OTHER"}})

	s.Require().NoError(err)
	s.Equal(2, total)
	s.Equal([]string{"A", "DB_PORT"}, variableNames(page))
}

func (s *SQLiteListTestSuite) TestNamesAndPrefixNarrowTogether() {
	page, total, err := s.store.ListVariables(context.Background(),
		listQuery{limit: 100, names: []string{"DB_PORT", "A", "API_URL"}, namePrefix: "A"})

	s.Require().NoError(err)
	s.Equal(2, total)
	s.Equal([]string{"A", "API_URL"}, variableNames(page))
}

func (s *SQLiteListTestSuite) TestSecretsListWithoutValues() {
	page, total, err := s.store.ListSecrets(context.Background(), listQuery{limit: 1, namePrefix: "S_"})

	s.Require().NoError(err)
	s.Equal(2, total)
	s.Require().Len(page, 1)
	s.Equal("S_ONE", page[0].Name)
	s.True(page[0].Exists)
}
