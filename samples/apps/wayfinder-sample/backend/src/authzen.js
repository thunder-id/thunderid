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

const TOKEN_EXPIRY_SKEW_MS = 30_000;
const AUTHZEN_REQUEST_TIMEOUT_MS = 5_000;

let authorizationMode;

function readAuthorizationMode(env) {
    const mode = (env.AUTHORIZATION_MODE || "scope").toLowerCase();

    if (mode !== "scope" && mode !== "authzen") {
        throw new Error(
            `Unsupported AUTHORIZATION_MODE: ${env.AUTHORIZATION_MODE}`,
        );
    }

    return mode;
}

export function getAuthorizationMode(env = process.env) {
    if (env !== process.env) {
        return readAuthorizationMode(env);
    }

    if (!authorizationMode) {
        authorizationMode = readAuthorizationMode(env);
    }

    return authorizationMode;
}

export function createAuthzenAuthorizer({
    env = process.env,
    fetchImpl = globalThis.fetch,
    logger = console,
} = {}) {
    let cachedToken = null;
    let tokenExpiresAt = 0;
    let tokenRequest = null;

    async function getAuthzenToken() {
        if (cachedToken && Date.now() < tokenExpiresAt) {
            return cachedToken;
        }

        if (!tokenRequest) {
            tokenRequest = requestAuthzenToken().finally(() => {
                tokenRequest = null;
            });
        }

        return tokenRequest;
    }

    async function requestAuthzenToken() {
        const baseUrl = env.THUNDER_BASE_URL;
        const clientId = env.THUNDERID_AUTHZEN_CLIENT_ID;
        const clientSecret = env.THUNDERID_AUTHZEN_CLIENT_SECRET;

        if (!baseUrl || !clientId || !clientSecret) {
            throw new Error(
                "THUNDER_BASE_URL, THUNDERID_AUTHZEN_CLIENT_ID, and THUNDERID_AUTHZEN_CLIENT_SECRET are required in AuthZEN mode",
            );
        }

        const credentials = Buffer.from(
            `${clientId}:${clientSecret}`,
        ).toString("base64");
        const response = await fetchImpl(`${baseUrl}/oauth2/token`, {
            method: "POST",
            headers: {
                Authorization: `Basic ${credentials}`,
                "Content-Type": "application/x-www-form-urlencoded",
            },
            signal: AbortSignal.timeout(AUTHZEN_REQUEST_TIMEOUT_MS),
            body: new URLSearchParams({
                grant_type: "client_credentials",
                scope: "system",
            }),
        });

        if (!response.ok) {
            logger.error(
                `[authzen] Client token request failed client=${clientId} status=${response.status}`,
            );
            throw new Error(
                `AuthZEN client token request failed with status ${response.status}`,
            );
        }

        const tokenResponse = await response.json();

        if (!tokenResponse.access_token) {
            throw new Error(
                "AuthZEN client token response has no access token",
            );
        }

        const expiresIn = Number(tokenResponse.expires_in || 3600);
        cachedToken = tokenResponse.access_token;
        tokenExpiresAt =
            Date.now() +
            Math.max(expiresIn * 1000 - TOKEN_EXPIRY_SKEW_MS, 0);

        logger.log(
            `[authzen] Client token acquired client=${clientId} expiresIn=${expiresIn}s`,
        );

        return cachedToken;
    }

    return async function evaluateAccess({ subject, resource, action }) {
        const subjectLabel = subject.type
            ? `${subject.type}:${subject.id}`
            : subject.id;
        const accessToken = await getAuthzenToken();
        const response = await fetchImpl(
            `${env.THUNDER_BASE_URL}/access/v1/evaluation`,
            {
                method: "POST",
                headers: {
                    Authorization: `Bearer ${accessToken}`,
                    "Content-Type": "application/json",
                },
                signal: AbortSignal.timeout(AUTHZEN_REQUEST_TIMEOUT_MS),
                body: JSON.stringify({ subject, resource, action }),
            },
        );

        if (!response.ok) {
            logger.error(
                `[authzen] Evaluation request failed subject=${subjectLabel} resource=${resource.type} action=${action.name} status=${response.status}`,
            );
            throw new Error(
                `AuthZEN access evaluation failed with status ${response.status}`,
            );
        }

        const evaluation = await response.json();

        if (typeof evaluation.decision !== "boolean") {
            logger.error(
                `[authzen] Evaluation returned no decision subject=${subjectLabel} resource=${resource.type} action=${action.name}`,
            );
            throw new Error("AuthZEN access evaluation returned no decision");
        }

        if (!evaluation.decision) {
            logger.warn(
                `[authzen] DENY subject=${subjectLabel} resource=${resource.type} action=${action.name}`,
            );
            const error = new Error(
                `AuthZEN denied permission ${action.name}`,
            );
            error.code = "insufficient_permission";
            error.requiredPermission = action.name;
            throw error;
        }

        logger.log(
            `[authzen] ALLOW subject=${subjectLabel} resource=${resource.type} action=${action.name}`,
        );

        return evaluation;
    };
}
