// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notification

import dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"

var (
	// queryCreateNotificationSender is the query to create a new notification sender.
	queryCreateNotificationSender = dbmodel.DBQuery{
		ID: "NMQ-SM-01",
		Query: `INSERT INTO "NOTIFICATION_SENDER" ` +
			`(NAME, ID, DESCRIPTION, TYPE, OU_ID, PROVIDER, PROPERTIES, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
	}

	// queryGetNotificationSenderByID is the query to get a notification sender by its ID.
	queryGetNotificationSenderByID = dbmodel.DBQuery{
		ID: "NMQ-SM-03",
		Query: `SELECT ID, NAME, DESCRIPTION, TYPE, OU_ID, PROVIDER, PROPERTIES ` +
			`FROM "NOTIFICATION_SENDER" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryGetAllNotificationSenders is the query to get all notification senders.
	queryGetAllNotificationSenders = dbmodel.DBQuery{
		ID: "NMQ-SM-05",
		Query: `SELECT ID, NAME, DESCRIPTION, TYPE, OU_ID, PROVIDER, PROPERTIES ` +
			`FROM "NOTIFICATION_SENDER" WHERE DEPLOYMENT_ID = $1`,
	}

	// queryGetNotificationSendersByType is the query to get all notification senders of a given type.
	queryGetNotificationSendersByType = dbmodel.DBQuery{
		ID: "NMQ-SM-10",
		Query: `SELECT ID, NAME, DESCRIPTION, TYPE, OU_ID, PROVIDER, PROPERTIES ` +
			`FROM "NOTIFICATION_SENDER" WHERE TYPE = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryUpdateNotificationSender is the query to update a notification sender.
	queryUpdateNotificationSender = dbmodel.DBQuery{
		ID: "NMQ-SM-06",
		PostgresQuery: `UPDATE "NOTIFICATION_SENDER" ` +
			`SET NAME = $1, DESCRIPTION = $2, PROVIDER = $3, PROPERTIES = $4, ` +
			`UPDATED_AT = NOW() WHERE ID = $5 AND TYPE = $6 AND DEPLOYMENT_ID = $7`,
		SQLiteQuery: `UPDATE "NOTIFICATION_SENDER" SET NAME = $1, DESCRIPTION = $2, PROVIDER = $3, PROPERTIES = $4, ` +
			`UPDATED_AT = datetime('now') WHERE ID = $5 AND TYPE = $6 AND DEPLOYMENT_ID = $7`,
		Query: `UPDATE "NOTIFICATION_SENDER" SET NAME = $1, DESCRIPTION = $2, PROVIDER = $3, PROPERTIES = $4, ` +
			`UPDATED_AT = datetime('now') WHERE ID = $5 AND TYPE = $6 AND DEPLOYMENT_ID = $7`,
	}

	// queryDeleteNotificationSender is the query to delete a notification sender
	queryDeleteNotificationSender = dbmodel.DBQuery{
		ID:    "NMQ-SM-08",
		Query: `DELETE FROM "NOTIFICATION_SENDER" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryGetNotificationSenderByName is the query to get a notification sender by name
	queryGetNotificationSenderByName = dbmodel.DBQuery{
		ID: "NMQ-SM-09",
		Query: `SELECT ID, NAME, DESCRIPTION, TYPE, OU_ID, PROVIDER, PROPERTIES ` +
			`FROM "NOTIFICATION_SENDER" WHERE NAME = $1 AND DEPLOYMENT_ID = $2`,
	}
)
