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

import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { mkdtempSync } from "node:fs";
import { rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { promisify } from "node:util";
import test from "node:test";

import { DatabaseSync, isNodeSqlitePublicVersion } from "./sqlite.js";

const execFileAsync = promisify(execFile);

test("identifies Node versions where node:sqlite is public", () => {
    assert.equal(isNodeSqlitePublicVersion("20.19.0"), false);
    assert.equal(isNodeSqlitePublicVersion("22.12.0"), false);
    assert.equal(isNodeSqlitePublicVersion("22.13.0"), true);
    assert.equal(isNodeSqlitePublicVersion("23.3.0"), false);
    assert.equal(isNodeSqlitePublicVersion("23.4.0"), true);
    assert.equal(isNodeSqlitePublicVersion("24.0.0"), true);
});

test("sqlite adapter supports the synchronous database API used by Wayfinder", async () => {
    const tempDir = mkdtempSync(join(tmpdir(), "wayfinder-sqlite-"));
    const db = new DatabaseSync(join(tempDir, "test.sqlite"));

    try {
        db.exec("CREATE TABLE records (id TEXT PRIMARY KEY, value TEXT NOT NULL)");

        const insert = db.prepare("INSERT INTO records (id, value) VALUES (@id, @value)");
        db.transaction(() => {
            insert.run({ id: "record-1", value: "created" });
        })();

        const row = db.prepare("SELECT * FROM records WHERE id = @id").get({ id: "record-1" });

        assert.deepEqual({ ...row }, { id: "record-1", value: "created" });
    } finally {
        db.close();
        await rm(tempDir, { recursive: true, force: true });
    }
});

test("sqlite adapter can force the better-sqlite3 fallback", async () => {
    const { stdout } = await execFileAsync(
        process.execPath,
        [
            "--input-type=module",
            "--eval",
            `
                import { DatabaseSync } from "./src/sqlite.js";

                const db = new DatabaseSync(":memory:");
                db.exec("CREATE TABLE records (id TEXT PRIMARY KEY)");
                db.prepare("INSERT INTO records (id) VALUES (@id)").run({ id: "fallback" });
                console.log(db.prepare("SELECT id FROM records").get().id);
                db.close();
            `,
        ],
        {
            cwd: new URL("..", import.meta.url),
            env: {
                ...process.env,
                WAYFINDER_SQLITE_PROVIDER: "better-sqlite3",
            },
        },
    );

    assert.equal(stdout.trim(), "fallback");
});
