// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"

// A template and its design are stored in a single NOTIFICATION_TEMPLATE row: the content is a JSON
// column (CONTENT) and the design is a JSON column (DESIGN), an empty JSON object ({}) when a channel
// carries no design.
var (
	// queryCreateTemplate inserts a new notification template row.
	queryCreateTemplate = dbmodel.DBQuery{
		ID: "NTQ-NOTIF_TMPL-01",
		Query: `INSERT INTO "NOTIFICATION_TEMPLATE" ` +
			`(ID, CHANNEL, HANDLE, DISPLAY_NAME, DESCRIPTION, CONTENT, DESIGN, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
	}

	// queryGetTemplateByID retrieves a template by channel and id.
	queryGetTemplateByID = dbmodel.DBQuery{
		ID: "NTQ-NOTIF_TMPL-02",
		Query: `SELECT ID, CHANNEL, HANDLE, DISPLAY_NAME, DESCRIPTION, CONTENT, DESIGN ` +
			`FROM "NOTIFICATION_TEMPLATE" ` +
			`WHERE ID = $1 AND CHANNEL = $2 AND DEPLOYMENT_ID = $3`,
	}

	// queryListTemplates retrieves a page of templates of a channel. The list response is a summary
	// (id, handle, displayName, description), so CONTENT and DESIGN are not selected.
	queryListTemplates = dbmodel.DBQuery{
		ID: "NTQ-NOTIF_TMPL-03",
		Query: `SELECT ID, CHANNEL, HANDLE, DISPLAY_NAME, DESCRIPTION ` +
			`FROM "NOTIFICATION_TEMPLATE" ` +
			`WHERE CHANNEL = $1 AND DEPLOYMENT_ID = $4 ORDER BY CREATED_AT DESC, ID DESC LIMIT $2 OFFSET $3`,
	}

	// queryUpdateTemplate updates a template's mutable fields. The handle is immutable and is not
	// touched here.
	queryUpdateTemplate = dbmodel.DBQuery{
		ID: "NTQ-NOTIF_TMPL-04",
		PostgresQuery: `UPDATE "NOTIFICATION_TEMPLATE" SET DISPLAY_NAME = $1, DESCRIPTION = $2, ` +
			`CONTENT = $3, DESIGN = $4, UPDATED_AT = NOW() WHERE ID = $5 AND CHANNEL = $6 AND DEPLOYMENT_ID = $7`,
		SQLiteQuery: `UPDATE "NOTIFICATION_TEMPLATE" SET DISPLAY_NAME = $1, DESCRIPTION = $2, ` +
			`CONTENT = $3, DESIGN = $4, UPDATED_AT = datetime('now') ` +
			`WHERE ID = $5 AND CHANNEL = $6 AND DEPLOYMENT_ID = $7`,
	}

	// queryDeleteTemplate deletes a template row by channel and id.
	queryDeleteTemplate = dbmodel.DBQuery{
		ID:    "NTQ-NOTIF_TMPL-05",
		Query: `DELETE FROM "NOTIFICATION_TEMPLATE" WHERE ID = $1 AND CHANNEL = $2 AND DEPLOYMENT_ID = $3`,
	}

	// queryCountTemplates counts the templates of a channel, for pagination totals.
	queryCountTemplates = dbmodel.DBQuery{
		ID: "NTQ-NOTIF_TMPL-06",
		Query: `SELECT COUNT(*) as total FROM "NOTIFICATION_TEMPLATE" ` +
			`WHERE CHANNEL = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryCheckHandleExists checks whether a template in the channel already uses a handle.
	queryCheckHandleExists = dbmodel.DBQuery{
		ID: "NTQ-NOTIF_TMPL-07",
		Query: `SELECT COUNT(*) as total FROM "NOTIFICATION_TEMPLATE" ` +
			`WHERE CHANNEL = $1 AND HANDLE = $2 AND DEPLOYMENT_ID = $3`,
	}

	// queryGetTemplateByHandle retrieves a template by channel and handle (the runtime lookup key).
	queryGetTemplateByHandle = dbmodel.DBQuery{
		ID: "NTQ-NOTIF_TMPL-08",
		Query: `SELECT ID, CHANNEL, HANDLE, DISPLAY_NAME, DESCRIPTION, CONTENT, DESIGN ` +
			`FROM "NOTIFICATION_TEMPLATE" ` +
			`WHERE HANDLE = $1 AND CHANNEL = $2 AND DEPLOYMENT_ID = $3`,
	}
)
