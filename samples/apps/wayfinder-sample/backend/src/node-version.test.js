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
import test from "node:test";

import {
  nodeSqliteVersionMessage,
  requiredNodeVersion,
  supportsNodeSqlite
} from "./node-version.js";

test("identifies Node versions where node:sqlite is available without the experimental flag", () => {
  assert.equal(supportsNodeSqlite("20.19.0"), false);
  assert.equal(supportsNodeSqlite("22.12.0"), false);
  assert.equal(supportsNodeSqlite("22.13.0"), true);
  assert.equal(supportsNodeSqlite("23.3.0"), false);
  assert.equal(supportsNodeSqlite("23.4.0"), true);
  assert.equal(supportsNodeSqlite("24.0.0"), true);
});

test("builds the node:sqlite version guidance", () => {
  const message = nodeSqliteVersionMessage("20.19.0");

  assert.match(message, /Node\.js v20\.19\.0 detected/);
  assert.match(message, new RegExp(`Node\\.js v${requiredNodeVersion}`));
  assert.match(message, /nvm install 22\.13\.0/);
  assert.match(message, /nodejs\.org\/en\/download/);
});
