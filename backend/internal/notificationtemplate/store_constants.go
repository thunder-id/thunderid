// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"

// Reads join the companion design table so a template and its (optional) design come back in one row.
// The content is stored as a single JSON column (CONTENT); the design lives in a companion table so a
// channel with no design simply has no row.
var (
	// queryCreateTemplate inserts a new notification template row.
	queryCreateTemplate = dbmodel.DBQuery{
		ID: "NTQ-NOTIF_TMPL-01",
		Query: `INSERT INTO "NOTIFICATION_TEMPLATE" ` +
			`(ID, CHANNEL, NAME, DESCRIPTION, CONTENT, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3, $4, $5, $6)`,
	}

	// queryUpsertTemplateDesign inserts the companion design row. Callers delete the existing row
	// first (queryDeleteTemplateDesign) so this is always a plain insert, keeping it dialect-neutral.
	queryUpsertTemplateDesign = dbmodel.DBQuery{
		ID: "NTQ-NOTIF_TMPL-02",
		Query: `INSERT INTO "NOTIFICATION_TEMPLATE_DESIGN" ` +
			`(TEMPLATE_ID, COLOR_SCHEME, DEPLOYMENT_ID) VALUES ($1, $2, $3)`,
	}

	// queryGetTemplateByID retrieves a template and its design by channel and id.
	queryGetTemplateByID = dbmodel.DBQuery{
		ID: "NTQ-NOTIF_TMPL-03",
		Query: `SELECT t.ID, t.CHANNEL, t.NAME, t.DESCRIPTION, t.CONTENT, d.COLOR_SCHEME ` +
			`FROM "NOTIFICATION_TEMPLATE" t ` +
			`LEFT JOIN "NOTIFICATION_TEMPLATE_DESIGN" d ON d.TEMPLATE_ID = t.ID ` +
			`WHERE t.ID = $1 AND t.CHANNEL = $2 AND t.DEPLOYMENT_ID = $3`,
	}

	// queryListTemplates retrieves all templates of a channel with their designs.
	queryListTemplates = dbmodel.DBQuery{
		ID: "NTQ-NOTIF_TMPL-04",
		Query: `SELECT t.ID, t.CHANNEL, t.NAME, t.DESCRIPTION, t.CONTENT, d.COLOR_SCHEME ` +
			`FROM "NOTIFICATION_TEMPLATE" t ` +
			`LEFT JOIN "NOTIFICATION_TEMPLATE_DESIGN" d ON d.TEMPLATE_ID = t.ID ` +
			`WHERE t.CHANNEL = $1 AND t.DEPLOYMENT_ID = $2 ORDER BY t.CREATED_AT DESC`,
	}

	// queryUpdateTemplate updates a template's mutable fields.
	queryUpdateTemplate = dbmodel.DBQuery{
		ID: "NTQ-NOTIF_TMPL-05",
		PostgresQuery: `UPDATE "NOTIFICATION_TEMPLATE" SET NAME = $1, DESCRIPTION = $2, ` +
			`CONTENT = $3, UPDATED_AT = NOW() WHERE ID = $4 AND CHANNEL = $5 AND DEPLOYMENT_ID = $6`,
		SQLiteQuery: `UPDATE "NOTIFICATION_TEMPLATE" SET NAME = $1, DESCRIPTION = $2, ` +
			`CONTENT = $3, UPDATED_AT = datetime('now') WHERE ID = $4 AND CHANNEL = $5 AND DEPLOYMENT_ID = $6`,
		Query: `UPDATE "NOTIFICATION_TEMPLATE" SET NAME = $1, DESCRIPTION = $2, ` +
			`CONTENT = $3, UPDATED_AT = datetime('now') WHERE ID = $4 AND CHANNEL = $5 AND DEPLOYMENT_ID = $6`,
	}

	// queryDeleteTemplateDesign deletes the companion design row for a template.
	queryDeleteTemplateDesign = dbmodel.DBQuery{
		ID: "NTQ-NOTIF_TMPL-06",
		Query: `DELETE FROM "NOTIFICATION_TEMPLATE_DESIGN" ` +
			`WHERE TEMPLATE_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryDeleteTemplate deletes a template row by channel and id.
	queryDeleteTemplate = dbmodel.DBQuery{
		ID:    "NTQ-NOTIF_TMPL-07",
		Query: `DELETE FROM "NOTIFICATION_TEMPLATE" WHERE ID = $1 AND CHANNEL = $2 AND DEPLOYMENT_ID = $3`,
	}

	// queryCheckNameExists checks whether another template in the channel already uses a name.
	queryCheckNameExists = dbmodel.DBQuery{
		ID: "NTQ-NOTIF_TMPL-09",
		Query: `SELECT COUNT(*) as total FROM "NOTIFICATION_TEMPLATE" ` +
			`WHERE CHANNEL = $1 AND NAME = $2 AND DEPLOYMENT_ID = $3 AND ID != $4`,
	}
)
