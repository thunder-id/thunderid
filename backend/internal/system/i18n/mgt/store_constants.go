/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package mgt

import (
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
)

var (
	// queryGetDistinctLanguages retrieves all distinct language codes.
	queryGetDistinctLanguages = dbmodel.DBQuery{
		ID: "I18N-01",
		Query: `SELECT DISTINCT LANGUAGE_CODE FROM "TRANSLATION" ` +
			`WHERE DEPLOYMENT_ID = $1 ORDER BY LANGUAGE_CODE`,
	}

	// queryGetTranslationsByLanguage retrieves all translations for a language and namespace.
	queryGetTranslations = dbmodel.DBQuery{
		ID: "I18N-02",
		Query: `SELECT MESSAGE_KEY, LANGUAGE_CODE, NAMESPACE, VALUE FROM "TRANSLATION" ` +
			`WHERE DEPLOYMENT_ID = $1 ORDER BY MESSAGE_KEY`,
	}

	// queryGetTranslationsByLanguage retrieves all translations for a language and namespace.
	queryGetTranslationsByNamespace = dbmodel.DBQuery{
		ID: "I18N-03",
		Query: `SELECT MESSAGE_KEY, LANGUAGE_CODE, NAMESPACE, VALUE FROM "TRANSLATION" ` +
			`WHERE NAMESPACE = $1 AND DEPLOYMENT_ID = $2 ORDER BY MESSAGE_KEY`,
	}

	// queryGetTranslation retrieves a single translation by key, and namespace.
	queryGetTranslation = dbmodel.DBQuery{
		ID: "I18N-04",
		Query: `SELECT MESSAGE_KEY, LANGUAGE_CODE, NAMESPACE, VALUE FROM "TRANSLATION" ` +
			`WHERE MESSAGE_KEY = $1 AND NAMESPACE = $2 AND DEPLOYMENT_ID = $3`,
	}

	// queryInsertTranslation inserts a new translation.
	queryInsertTranslation = dbmodel.DBQuery{
		ID: "I18N-05",
		Query: `INSERT INTO "TRANSLATION" (MESSAGE_KEY, LANGUAGE_CODE, NAMESPACE, VALUE, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3, $4, $5)`,
	}

	// queryUpsertTranslation inserts or updates a translation.
	queryUpsertTranslation = dbmodel.DBQuery{
		ID: "I18N-06",
		Query: `INSERT INTO "TRANSLATION" (MESSAGE_KEY, LANGUAGE_CODE, NAMESPACE, VALUE, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3, $4, $5) ` +
			`ON CONFLICT (DEPLOYMENT_ID, NAMESPACE, MESSAGE_KEY, LANGUAGE_CODE) ` +
			`DO UPDATE SET VALUE = EXCLUDED.VALUE, UPDATED_AT = NOW()`,
		SQLiteQuery: `INSERT INTO "TRANSLATION" (MESSAGE_KEY, LANGUAGE_CODE, NAMESPACE, VALUE, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3, $4, $5) ` +
			`ON CONFLICT (DEPLOYMENT_ID, NAMESPACE, MESSAGE_KEY, LANGUAGE_CODE) ` +
			`DO UPDATE SET VALUE = excluded.VALUE, UPDATED_AT = datetime('now')`,
	}

	// queryDeleteTranslation deletes a translation by language, key, and namespace.
	queryDeleteTranslation = dbmodel.DBQuery{
		ID: "I18N-07",
		Query: `DELETE FROM "TRANSLATION" ` +
			`WHERE LANGUAGE_CODE = $1 AND MESSAGE_KEY = $2 AND NAMESPACE = $3 AND DEPLOYMENT_ID = $4`,
	}

	// queryDeleteTranslationsByLanguage deletes all translations for a language code.
	queryDeleteTranslationsByLanguage = dbmodel.DBQuery{
		ID:    "I18N-08",
		Query: `DELETE FROM "TRANSLATION" WHERE LANGUAGE_CODE = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryDeleteTranslationsByNamespace deletes all translations for a given namespace.
	queryDeleteTranslationsByNamespace = dbmodel.DBQuery{
		ID:    "I18N-09",
		Query: `DELETE FROM "TRANSLATION" WHERE NAMESPACE = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryDeleteTranslationsByKey deletes all translations for a given namespace and key.
	queryDeleteTranslationsByKey = dbmodel.DBQuery{
		ID:    "I18N-10",
		Query: `DELETE FROM "TRANSLATION" WHERE NAMESPACE = $1 AND MESSAGE_KEY = $2 AND DEPLOYMENT_ID = $3`,
	}
)
