// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sso

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// schemaTestDeployment keeps the rows the schema tests write apart from the server's own.
const schemaTestDeployment = "session-schema-test"

// insertSessionRow writes a session row straight into the runtime database, with the named columns NULL. The row
// is deleted when the test ends.
func (ts *SSOLogoutTestSuite) insertSessionRow(id, sessionType string, nullColumns ...string) error {
	ts.T().Helper()

	values := map[string]string{
		"SESSION_ID": "'" + id + "'", "TYPE": "'" + sessionType + "'", "SUBJECT_ID": "'user-1'",
		"FLOW_ID": "'flow-1'", "FLOW_VERSION": "1", "FLOW_EXECUTION_ID": "'exec-" + id + "'",
		"HANDLE_ID": "'handle-" + id + "'", "AUTHENTICATED_AT": "CURRENT_TIMESTAMP",
		"LAST_ACTIVE_AT": "CURRENT_TIMESTAMP", "IDLE_EXPIRES_AT": "CURRENT_TIMESTAMP",
		"ABSOLUTE_EXPIRES_AT": "CURRENT_TIMESTAMP", "STATE": "'ACTIVE'", "VERSION": "1",
		"DEPLOYMENT_ID": "'" + schemaTestDeployment + "'",
	}
	for _, column := range nullColumns {
		values[column] = "NULL"
	}
	columns := slices.Sorted(maps.Keys(values))
	literals := make([]string, 0, len(columns))
	for _, column := range columns {
		literals = append(literals, values[column])
	}

	ts.T().Cleanup(func() {
		if _, err := testutils.QueryRuntimeDB(fmt.Sprintf(`DELETE FROM "SESSION" WHERE SESSION_ID = '%s' `+
			`AND DEPLOYMENT_ID = '%s';`, id, schemaTestDeployment)); err != nil {
			ts.T().Logf("Failed to delete session row %s: %v", id, err)
		}
	})
	_, err := testutils.QueryRuntimeDB(fmt.Sprintf(`INSERT INTO "SESSION" (%s) VALUES (%s);`,
		strings.Join(columns, ", "), strings.Join(literals, ", ")))
	return err
}

func (ts *SSOLogoutTestSuite) TestSessionSchema_EverySessionNeedsItsFlowAndDeadlines() {
	for _, column := range []string{"FLOW_ID", "FLOW_VERSION", "IDLE_EXPIRES_AT", "ABSOLUTE_EXPIRES_AT"} {
		ts.Error(ts.insertSessionRow("sso-no-"+column, "SSO", column), "an SSO session needs %s", column)
		ts.Error(ts.insertSessionRow("direct-no-"+column, "DIRECT", "FLOW_EXECUTION_ID", "HANDLE_ID", column),
			"a direct session needs %s", column)
	}
}

func (ts *SSOLogoutTestSuite) TestSessionSchema_AnSSOSessionNeedsItsExecutionAndHandle() {
	ts.Error(ts.insertSessionRow("sso-no-execution", "SSO", "FLOW_EXECUTION_ID"))
	ts.Error(ts.insertSessionRow("sso-no-handle", "SSO", "HANDLE_ID"))
	ts.NoError(ts.insertSessionRow("sso-complete", "SSO"))
}

// Direct sessions hold no flow execution or handle, so any number of them fit beside each other.
func (ts *SSOLogoutTestSuite) TestSessionSchema_DirectSessionsNeedNoExecutionOrHandle() {
	ts.NoError(ts.insertSessionRow("direct-1", "DIRECT", "FLOW_EXECUTION_ID", "HANDLE_ID"))
	ts.NoError(ts.insertSessionRow("direct-2", "DIRECT", "FLOW_EXECUTION_ID", "HANDLE_ID"))
}

func (ts *SSOLogoutTestSuite) TestSessionSchema_AParticipantNeedsATokenFamily() {
	_, err := testutils.QueryRuntimeDB(fmt.Sprintf(`INSERT INTO "SESSION_PARTICIPANT" (SESSION_ID, `+
		`DEPLOYMENT_ID, APP_ID, OU_ID, TFID, FIRST_JOINED_AT, LAST_ACTIVE_AT) VALUES ('sess-1', '%s', 'app-1', '', `+
		`NULL, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);`, schemaTestDeployment))
	ts.Error(err)
}
