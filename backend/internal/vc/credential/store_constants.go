// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package credential

import (
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
)

// DBQuery definitions for the credential-configuration config store.
var (
	queryCreateConfiguration = dbmodel.DBQuery{
		ID: "OVCIQ-CC_MGT-01",
		Query: `INSERT INTO "CREDENTIAL_CONFIGURATION" ` +
			`(ID, HANDLE, OU_ID, NAME, DESCRIPTION, FORMAT, VCT, CLAIMS, DISPLAY, VALIDITY_SECONDS, PROJECT_ID, ` +
			`DEPLOYMENT_ID) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
	}
	queryGetConfigurationByID = dbmodel.DBQuery{
		ID: "OVCIQ-CC_MGT-02",
		Query: `SELECT ID, HANDLE, OU_ID, NAME, DESCRIPTION, FORMAT, VCT, CLAIMS, DISPLAY, VALIDITY_SECONDS, ` +
			`PROJECT_ID FROM "CREDENTIAL_CONFIGURATION" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	queryGetConfigurationByHandle = dbmodel.DBQuery{
		ID: "OVCIQ-CC_MGT-03",
		Query: `SELECT ID, HANDLE, OU_ID, NAME, DESCRIPTION, FORMAT, VCT, CLAIMS, DISPLAY, VALIDITY_SECONDS, ` +
			`PROJECT_ID FROM "CREDENTIAL_CONFIGURATION" WHERE HANDLE = $1 AND DEPLOYMENT_ID = $2`,
	}
	queryListConfigurations = dbmodel.DBQuery{
		ID: "OVCIQ-CC_MGT-04",
		Query: `SELECT ID, HANDLE, OU_ID, NAME, DESCRIPTION, FORMAT, VCT, CLAIMS, DISPLAY, VALIDITY_SECONDS, ` +
			`PROJECT_ID FROM "CREDENTIAL_CONFIGURATION" WHERE DEPLOYMENT_ID = $1`,
	}
	queryListConfigurationSummaries = dbmodel.DBQuery{
		ID: "OVCIQ-CC_MGT-07",
		Query: `SELECT ID, HANDLE, OU_ID, PROJECT_ID, NAME, FORMAT, VCT ` +
			`FROM "CREDENTIAL_CONFIGURATION" WHERE DEPLOYMENT_ID = $1`,
	}
	queryUpdateConfiguration = dbmodel.DBQuery{
		ID: "OVCIQ-CC_MGT-05",
		Query: `UPDATE "CREDENTIAL_CONFIGURATION" SET HANDLE = $2, OU_ID = $3, NAME = $4, DESCRIPTION = $5, ` +
			`FORMAT = $6, VCT = $7, CLAIMS = $8, DISPLAY = $9, VALIDITY_SECONDS = $10, PROJECT_ID = $11, ` +
			`UPDATED_AT = CURRENT_TIMESTAMP WHERE ID = $1 AND DEPLOYMENT_ID = $12`,
	}
	queryDeleteConfiguration = dbmodel.DBQuery{
		ID:    "OVCIQ-CC_MGT-06",
		Query: `DELETE FROM "CREDENTIAL_CONFIGURATION" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	queryListConfigurationSummariesByOUID = dbmodel.DBQuery{
		ID: "OVCIQ-CC_MGT-08",
		Query: `SELECT ID, HANDLE, OU_ID, PROJECT_ID, NAME, FORMAT, VCT ` +
			`FROM "CREDENTIAL_CONFIGURATION" WHERE OU_ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	queryListConfigurationSummariesByProject = dbmodel.DBQuery{
		ID: "OVCIQ-CC_MGT-09",
		Query: `SELECT ID, HANDLE, OU_ID, PROJECT_ID, NAME, FORMAT, VCT ` +
			`FROM "CREDENTIAL_CONFIGURATION" WHERE PROJECT_ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	queryCountConfigurationsByProject = dbmodel.DBQuery{
		ID: "OVCIQ-CC_MGT-10",
		Query: `SELECT COUNT(*) AS total FROM "CREDENTIAL_CONFIGURATION" ` +
			`WHERE PROJECT_ID = $1 AND DEPLOYMENT_ID = $2`,
	}
)
