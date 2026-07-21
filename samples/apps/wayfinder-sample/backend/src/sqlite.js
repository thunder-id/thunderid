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

class NodeSqliteDatabase {
  constructor(DatabaseSync, path) {
    this.database = new DatabaseSync(path);
  }

  close() {
    return this.database.close();
  }

  exec(sql) {
    return this.database.exec(sql);
  }

  prepare(sql) {
    return this.database.prepare(sql);
  }

  transaction(callback) {
    return (...args) => {
      this.exec("BEGIN TRANSACTION");

      try {
        const result = callback(...args);
        this.exec("COMMIT");

        return result;
      } catch (error) {
        this.exec("ROLLBACK");

        throw error;
      }
    };
  }
}

export function isNodeSqlitePublicVersion(version = process.versions.node) {
  const [major = 0, minor = 0] = version.split(".").map(Number);

  return major > 23 || (major === 23 && minor >= 4) || (major === 22 && minor >= 13);
}

async function loadDatabaseSync() {
  const provider = process.env.WAYFINDER_SQLITE_PROVIDER || "auto";

  if (provider !== "auto" && provider !== "node:sqlite" && provider !== "better-sqlite3") {
    throw new Error(`Unsupported WAYFINDER_SQLITE_PROVIDER: ${provider}`);
  }

  if (provider === "node:sqlite" || (provider === "auto" && isNodeSqlitePublicVersion())) {
    try {
      const { DatabaseSync } = await import("node:sqlite");

      return class Database extends NodeSqliteDatabase {
        constructor(path) {
          super(DatabaseSync, path);
        }
      };
    } catch (error) {
      if (provider === "node:sqlite" || error.code !== "ERR_UNKNOWN_BUILTIN_MODULE") {
        throw error;
      }
    }
  }

  try {
    const { default: Database } = await import("better-sqlite3");

    return Database;
  } catch (error) {
    if (provider === "better-sqlite3") {
      throw error;
    }

    throw new Error(
      "SQLite requires Node.js with node:sqlite support or an installed better-sqlite3 fallback.",
      { cause: error },
    );
  }
}

export const DatabaseSync = await loadDatabaseSync();
