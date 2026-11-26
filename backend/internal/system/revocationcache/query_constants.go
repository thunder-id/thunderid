// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocationcache

import dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"

// querySnapshotRevokedTokens reads the full set of non-expired single-token deny-list entries for this
// deployment. It is read-only: this package holds no insert/update/delete query against the deny list.
var querySnapshotRevokedTokens = dbmodel.DBQuery{
	ID:    "RVC-SRC-01",
	Query: `SELECT JTI, EXPIRY_TIME FROM "REVOKED_TOKEN" WHERE EXPIRY_TIME > $1 AND DEPLOYMENT_ID = $2`,
}

// querySnapshotRevokedTokenFamilies reads the full set of non-expired token-family revocation entries for
// this deployment from the criteria deny list. It is read-only.
var querySnapshotRevokedTokenFamilies = dbmodel.DBQuery{
	ID: "RVC-SRC-02",
	Query: `SELECT CRITERION_VALUE, EXPIRY_TIME FROM "REVOCATION_CRITERIA" ` +
		`WHERE CRITERION_TYPE = $1 AND EXPIRY_TIME > $2 AND DEPLOYMENT_ID = $3`,
}

// querySnapshotBoundedCriteria reads the criteria of every bounded type, in the order of boundedCriterionTypes, with
// the type, the reason and the action boundary, so permanent revocations can be told from time-bounded ones.
var querySnapshotBoundedCriteria = dbmodel.DBQuery{
	ID: "RVC-SRC-03",
	Query: `SELECT CRITERION_TYPE, CRITERION_VALUE, REASON, REVOKED_AT, EXPIRY_TIME FROM "REVOCATION_CRITERIA" ` +
		`WHERE CRITERION_TYPE IN ($1, $2, $3, $4, $5, $6, $7, $8, $9) AND EXPIRY_TIME > $10 AND DEPLOYMENT_ID = $11`,
}
