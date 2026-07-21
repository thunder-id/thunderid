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
	"fmt"
	"strings"

	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
)

var (
	// QueryCreateConsent is the query to create a new consent record.
	QueryCreateConsent = dbmodel.DBQuery{
		ID: "CNQ-CONSENT_MGT-01",
		Query: `INSERT INTO "CONSENT" ` +
			`(ID, GROUP_ID, STATUS, VALIDITY_TIME, PURPOSES, DEPLOYMENT_ID, CREATED_AT, UPDATED_AT) ` +
			`VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
	}

	// QueryGetConsentByID is the query to get a consent record by id.
	QueryGetConsentByID = dbmodel.DBQuery{
		ID: "CNQ-CONSENT_MGT-02",
		Query: `SELECT ID, GROUP_ID, STATUS, VALIDITY_TIME, PURPOSES FROM "CONSENT" ` +
			`WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// QueryUpdateConsent is the query to update an existing consent record.
	QueryUpdateConsent = dbmodel.DBQuery{
		ID: "CNQ-CONSENT_MGT-03",
		Query: `UPDATE "CONSENT" SET STATUS = $2, VALIDITY_TIME = $3, PURPOSES = $4, UPDATED_AT = $5 ` +
			`WHERE ID = $1 AND DEPLOYMENT_ID = $6`,
	}

	// QueryDeleteConsentAuthorizations is the query to delete all authorization records of a consent.
	QueryDeleteConsentAuthorizations = dbmodel.DBQuery{
		ID:    "CNQ-CONSENT_MGT-04",
		Query: `DELETE FROM "CONSENT_AUTHORIZATION" WHERE CONSENT_ID = $1 AND DEPLOYMENT_ID = $2`,
	}
)

// buildInsertConsentAuthorizationsQuery constructs a single multi-row INSERT for a consent's
// authorization records, along with the flattened args. The caller must ensure authorizations is
// non-empty.
func buildInsertConsentAuthorizationsQuery(
	consentID string, authorizations []ConsentAuthorization, deploymentID string,
) (dbmodel.DBQuery, []interface{}) {
	valuePlaceholders := make([]string, 0, len(authorizations))
	args := make([]interface{}, 0, len(authorizations)*7)
	paramIndex := 1
	for _, authorization := range authorizations {
		valuePlaceholders = append(valuePlaceholders,
			fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d)",
				paramIndex, paramIndex+1, paramIndex+2, paramIndex+3, paramIndex+4, paramIndex+5, paramIndex+6))
		args = append(args,
			authorization.ID, consentID, authorization.UserID,
			string(authorization.Type), string(authorization.Status),
			unixToNullableTime(authorization.UpdatedTime), deploymentID)
		paramIndex += 7
	}

	query := `INSERT INTO "CONSENT_AUTHORIZATION" ` +
		`(ID, CONSENT_ID, USER_ID, TYPE, STATUS, UPDATED_TIME, DEPLOYMENT_ID) VALUES ` +
		strings.Join(valuePlaceholders, ", ")

	return dbmodel.DBQuery{
		ID:    "CNQ-CONSENT_MGT-05",
		Query: query,
	}, args
}

// buildGetConsentAuthorizationsQuery constructs a query and args to load the authorization records
// for the given set of consent IDs in a single round trip. The caller must ensure consentIDs is
// non-empty. The result row includes CONSENT_ID so callers can group records by their consent.
func buildGetConsentAuthorizationsQuery(
	consentIDs []string, deploymentID string,
) (dbmodel.DBQuery, []interface{}) {
	args := make([]interface{}, 0, len(consentIDs)+1)
	args = append(args, deploymentID)

	placeholders := make([]string, 0, len(consentIDs))
	for _, consentID := range consentIDs {
		args = append(args, consentID)
		placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
	}

	query := `SELECT CONSENT_ID, ID, USER_ID, TYPE, STATUS, UPDATED_TIME FROM "CONSENT_AUTHORIZATION" ` +
		`WHERE DEPLOYMENT_ID = $1 AND CONSENT_ID IN (` + strings.Join(placeholders, ", ") + `) ` +
		`ORDER BY UPDATED_TIME`

	return dbmodel.DBQuery{
		ID:    "CNQ-CONSENT_MGT-06",
		Query: query,
	}, args
}

// buildSearchConsentsQuery constructs the query and args to search consent records by the given filters.
// When a user ID filter is supplied, the query joins the authorization table so consents can be
// matched by the user that authorized them. The status filter is intentionally not applied here: a
// consent's effective status depends on its validity time evaluated at request time, so the service
// layer filters by status after loading the records.
func buildSearchConsentsQuery(
	filters ConsentFilter, deploymentID string,
) (dbmodel.DBQuery, []interface{}) {
	joinAuthorization := filters.UserID != ""

	selectClause := `SELECT C.ID, C.GROUP_ID, C.STATUS, C.VALIDITY_TIME, C.PURPOSES FROM "CONSENT" C`
	if joinAuthorization {
		selectClause = `SELECT DISTINCT C.ID, C.GROUP_ID, C.STATUS, C.VALIDITY_TIME, C.PURPOSES ` +
			`FROM "CONSENT" C ` +
			`INNER JOIN "CONSENT_AUTHORIZATION" A ON A.CONSENT_ID = C.ID AND A.DEPLOYMENT_ID = C.DEPLOYMENT_ID`
	}

	args := []interface{}{deploymentID}
	conditions := []string{fmt.Sprintf("C.DEPLOYMENT_ID = $%d", len(args))}
	if filters.GroupID != "" {
		args = append(args, filters.GroupID)
		conditions = append(conditions, fmt.Sprintf("C.GROUP_ID = $%d", len(args)))
	}
	if joinAuthorization {
		args = append(args, filters.UserID)
		conditions = append(conditions, fmt.Sprintf("A.USER_ID = $%d", len(args)))
	}

	query := selectClause + " WHERE " + strings.Join(conditions, " AND ") + " ORDER BY C.ID"

	return dbmodel.DBQuery{
		ID:    "CNQ-CONSENT_MGT-07",
		Query: query,
	}, args
}
