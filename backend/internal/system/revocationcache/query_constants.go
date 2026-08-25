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

// querySnapshotBoundedCriteria reads the criteria of one dimension together with the reason and action
// boundary needed to distinguish permanent revocations from time-bounded ones. The criterion type is a
// bind parameter, so a new dimension reuses this query rather than adding another.
var querySnapshotBoundedCriteria = dbmodel.DBQuery{
	ID: "RVC-SRC-03",
	Query: `SELECT CRITERION_VALUE, REASON, REVOKED_AT, EXPIRY_TIME FROM "REVOCATION_CRITERIA" ` +
		`WHERE CRITERION_TYPE = $1 AND EXPIRY_TIME > $2 AND DEPLOYMENT_ID = $3`,
}
