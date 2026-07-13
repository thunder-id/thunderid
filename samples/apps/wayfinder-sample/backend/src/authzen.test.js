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
    THUNDERID_AUTHZEN_CLIENT_ID: "authzen-client",
    THUNDERID_AUTHZEN_CLIENT_SECRET: "authzen-client-secret",
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

test("requests and reuses a client token for access evaluations", async () => {
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

        if (url.endsWith("/oauth2/token")) {
            return Response.json({
                access_token: "authzen-access-token",
                expires_in: 3600,
            });
        }

        return Response.json({ decision: true });
    };
    const evaluateAccess = createAuthzenAuthorizer({
        env,
        fetchImpl,
        logger,
    });
    const request = {
        subject: { id: "user-1" },
        resource: { type: "wayfinder" },
        action: { name: "wayfinder:booking:read" },
    };

    assert.equal((await evaluateAccess(request)).decision, true);
    assert.equal((await evaluateAccess(request)).decision, true);
    assert.equal(requests.length, 3);
    assert.equal(requests[0].url, "https://localhost:8090/oauth2/token");
    assert.match(String(requests[0].options.body), /scope=system/);
    assert.ok(requests[0].options.signal instanceof AbortSignal);
    assert.equal(
        requests[1].options.headers.Authorization,
        "Bearer authzen-access-token",
    );
    assert.ok(requests[1].options.signal instanceof AbortSignal);
    assert.deepEqual(JSON.parse(requests[1].options.body), request);
    assert.match(logs[0], /Client token acquired client=authzen-client/);
    assert.match(
        logs[1],
        /ALLOW subject=user-1 resource=wayfinder action=wayfinder:booking:read/,
    );
    assert.doesNotMatch(logs.join(" "), /authzen-access-token/);
    assert.doesNotMatch(logs.join(" "), /authzen-client-secret/);
});

test("shares an in-flight client token request across concurrent evaluations", async () => {
    let tokenRequests = 0;
    let evaluationRequests = 0;
    const fetchImpl = async (url) => {
        if (url.endsWith("/oauth2/token")) {
            tokenRequests += 1;
            await new Promise((resolve) => setTimeout(resolve, 10));
            return Response.json({
                access_token: "authzen-access-token",
                expires_in: 3600,
            });
        }

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
        resource: { type: "wayfinder" },
        action: { name: "wayfinder:booking:read" },
    };

    await Promise.all([
        evaluateAccess(request),
        evaluateAccess(request),
        evaluateAccess(request),
    ]);

    assert.equal(tokenRequests, 1);
    assert.equal(evaluationRequests, 3);
});

test("fails closed when the PDP response has no decision", async () => {
    const fetchImpl = async (url) =>
        url.endsWith("/oauth2/token")
            ? Response.json({ access_token: "authzen-access-token" })
            : Response.json({});
    const evaluateAccess = createAuthzenAuthorizer({
        env,
        fetchImpl,
        logger: silentLogger,
    });

    await assert.rejects(
        evaluateAccess({
            subject: { id: "user-1" },
            resource: { type: "wayfinder" },
            action: { name: "wayfinder:booking:read" },
        }),
        /returned no decision/,
    );
});

test("fails closed when the PDP denies the permission", async () => {
    const fetchImpl = async (url) =>
        url.endsWith("/oauth2/token")
            ? Response.json({ access_token: "authzen-access-token" })
            : Response.json({ decision: false });
    const evaluateAccess = createAuthzenAuthorizer({
        env,
        fetchImpl,
        logger: silentLogger,
    });

    await assert.rejects(
        evaluateAccess({
            subject: { id: "agent-1" },
            resource: { type: "wayfinder" },
            action: { name: "wayfinder:booking:create" },
        }),
        (error) => {
            assert.equal(error.code, "insufficient_permission");
            assert.equal(
                error.requiredPermission,
                "wayfinder:booking:create",
            );
            return true;
        },
    );
});
