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
    createAuthzenAuthorizer,
    getAuthorizationMode,
} from "./authzen.js";

const env = {
    THUNDER_BASE_URL: "https://localhost:8090",
    THUNDERID_DIRECT_AUTH_SECRET: "authzen-direct-auth-secret",
};
const silentLogger = {
    error() {},
    log() {},
    warn() {},
};

test("scope is the default authorization mode", () => {
    assert.equal(getAuthorizationMode({}), "scope");
});

test("accepts authzen authorization mode", () => {
    assert.equal(getAuthorizationMode({ AUTHORIZATION_MODE: "authzen" }), "authzen");
});

test("rejects unsupported authorization modes", () => {
    assert.throws(
        () => getAuthorizationMode({ AUTHORIZATION_MODE: "other" }),
        /Unsupported AUTHORIZATION_MODE/,
    );
});

test("sends the direct auth secret for access evaluations", async () => {
    const requests = [];
    const logs = [];
    const logger = {
        error(message) {
            logs.push(message);
        },
        log(message) {
            logs.push(message);
        },
        warn(message) {
            logs.push(message);
        },
    };
    const fetchImpl = async (url, options) => {
        requests.push({ url, options });

        return Response.json({ decision: true });
    };
    const evaluateAccess = createAuthzenAuthorizer({
        env,
        fetchImpl,
        logger,
    });
    const request = {
        subject: { id: "user-1" },
        resource: { type: "http://localhost:8787/mcp" },
        action: { name: "booking:read" },
    };

    assert.equal((await evaluateAccess(request)).decision, true);
    assert.equal((await evaluateAccess(request)).decision, true);
    assert.equal(requests.length, 2);
    assert.equal(
        requests[0].url,
        "https://localhost:8090/access/v1/evaluation",
    );
    assert.equal(
        requests[0].options.headers["Direct-Auth-Secret"],
        "authzen-direct-auth-secret",
    );
    assert.ok(requests[0].options.signal instanceof AbortSignal);
    assert.deepEqual(JSON.parse(requests[0].options.body), request);
    assert.match(
        logs[0],
        /ALLOW subject=user-1 resource=http:\/\/localhost:8787\/mcp action=booking:read/,
    );
    assert.doesNotMatch(logs.join(" "), /authzen-direct-auth-secret/);
});

test("requires the direct auth secret in authzen mode", async () => {
    const evaluateAccess = createAuthzenAuthorizer({
        env: { THUNDER_BASE_URL: "https://localhost:8090" },
        fetchImpl: async () => Response.json({ decision: true }),
        logger: silentLogger,
    });

    await assert.rejects(
        evaluateAccess({
            subject: { id: "user-1" },
            resource: { type: "http://localhost:8787/mcp" },
            action: { name: "booking:read" },
        }),
        /THUNDER_BASE_URL and THUNDERID_DIRECT_AUTH_SECRET are required/,
    );
});

test("sends one PDP request per concurrent evaluation", async () => {
    let evaluationRequests = 0;
    const fetchImpl = async () => {
        evaluationRequests += 1;
        return Response.json({ decision: true });
    };
    const evaluateAccess = createAuthzenAuthorizer({
        env,
        fetchImpl,
        logger: silentLogger,
    });
    const request = {
        subject: { id: "user-1" },
        resource: { type: "http://localhost:8787/mcp" },
        action: { name: "booking:read" },
    };

    await Promise.all([
        evaluateAccess(request),
        evaluateAccess(request),
        evaluateAccess(request),
    ]);

    assert.equal(evaluationRequests, 3);
});

test("fails closed when the PDP response has no decision", async () => {
    const fetchImpl = async () => Response.json({});
    const evaluateAccess = createAuthzenAuthorizer({
        env,
        fetchImpl,
        logger: silentLogger,
    });

    await assert.rejects(
        evaluateAccess({
            subject: { id: "user-1" },
            resource: { type: "http://localhost:8787/mcp" },
            action: { name: "booking:read" },
        }),
        /returned no decision/,
    );
});

test("fails closed when the PDP denies the permission", async () => {
    const fetchImpl = async () => Response.json({ decision: false });
    const evaluateAccess = createAuthzenAuthorizer({
        env,
        fetchImpl,
        logger: silentLogger,
    });

    await assert.rejects(
        evaluateAccess({
            subject: { id: "agent-1" },
            resource: { type: "http://localhost:8787/mcp" },
            action: { name: "booking:create" },
        }),
        (error) => {
            assert.equal(error.code, "insufficient_permission");
            assert.equal(
                error.requiredPermission,
                "booking:create",
            );
            return true;
        },
    );
});
