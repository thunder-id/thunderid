// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package presentation

import (
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
)

// DBQuery definitions for the presentation-definition config store.
var (
	queryCreateDefinition = dbmodel.DBQuery{
		ID: "OVPQ-PD_MGT-01",
		Query: `INSERT INTO "PRESENTATION_DEFINITION" ` +
			`(ID, HANDLE, OU_ID, NAME, DESCRIPTION, VCT, FORMAT, CLAIMS, ` +
			`ENFORCE_TRUSTED_ISSUER, TRUSTED_AUTHORITIES, PROJECT_ID, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
	}
	queryGetDefinitionByID = dbmodel.DBQuery{
		ID: "OVPQ-PD_MGT-02",
		Query: `SELECT ID, HANDLE, OU_ID, NAME, DESCRIPTION, VCT, FORMAT, CLAIMS, ` +
			`ENFORCE_TRUSTED_ISSUER, TRUSTED_AUTHORITIES, PROJECT_ID ` +
			`FROM "PRESENTATION_DEFINITION" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	queryGetDefinitionByHandle = dbmodel.DBQuery{
		ID: "OVPQ-PD_MGT-03",
		Query: `SELECT ID, HANDLE, OU_ID, NAME, DESCRIPTION, VCT, FORMAT, CLAIMS, ` +
			`ENFORCE_TRUSTED_ISSUER, TRUSTED_AUTHORITIES, PROJECT_ID ` +
			`FROM "PRESENTATION_DEFINITION" WHERE HANDLE = $1 AND DEPLOYMENT_ID = $2`,
	}
	queryListDefinitions = dbmodel.DBQuery{
		ID: "OVPQ-PD_MGT-04",
		Query: `SELECT ID, HANDLE, OU_ID, NAME, DESCRIPTION, VCT, FORMAT, CLAIMS, ` +
			`ENFORCE_TRUSTED_ISSUER, TRUSTED_AUTHORITIES, PROJECT_ID ` +
			`FROM "PRESENTATION_DEFINITION" WHERE DEPLOYMENT_ID = $1`,
	}
	queryListDefinitionSummaries = dbmodel.DBQuery{
		ID: "OVPQ-PD_MGT-07",
		Query: `SELECT ID, HANDLE, OU_ID, PROJECT_ID, NAME, VCT, FORMAT ` +
			`FROM "PRESENTATION_DEFINITION" WHERE DEPLOYMENT_ID = $1`,
	}
	queryUpdateDefinition = dbmodel.DBQuery{
		ID: "OVPQ-PD_MGT-05",
		Query: `UPDATE "PRESENTATION_DEFINITION" SET HANDLE = $2, OU_ID = $3, NAME = $4, DESCRIPTION = $5, ` +
			`VCT = $6, FORMAT = $7, CLAIMS = $8, ` +
			`ENFORCE_TRUSTED_ISSUER = $9, TRUSTED_AUTHORITIES = $10, PROJECT_ID = $11, ` +
			`UPDATED_AT = CURRENT_TIMESTAMP WHERE ID = $1 AND DEPLOYMENT_ID = $12`,
	}
	queryDeleteDefinition = dbmodel.DBQuery{
		ID:    "OVPQ-PD_MGT-06",
		Query: `DELETE FROM "PRESENTATION_DEFINITION" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	queryListDefinitionSummariesByOUID = dbmodel.DBQuery{
		ID: "OVPQ-PD_MGT-08",
		Query: `SELECT ID, HANDLE, OU_ID, PROJECT_ID, NAME, VCT, FORMAT ` +
			`FROM "PRESENTATION_DEFINITION" WHERE OU_ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	queryListDefinitionSummariesByProject = dbmodel.DBQuery{
		ID: "OVPQ-PD_MGT-09",
		Query: `SELECT ID, HANDLE, OU_ID, PROJECT_ID, NAME, VCT, FORMAT ` +
			`FROM "PRESENTATION_DEFINITION" WHERE PROJECT_ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	queryCountDefinitionsByProject = dbmodel.DBQuery{
		ID: "OVPQ-PD_MGT-10",
		Query: `SELECT COUNT(*) AS total FROM "PRESENTATION_DEFINITION" ` +
			`WHERE PROJECT_ID = $1 AND DEPLOYMENT_ID = $2`,
	}
)
