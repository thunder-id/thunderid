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
    return async function evaluateAccess({ subject, resource, action }) {
        const baseUrl = env.THUNDER_BASE_URL;
        const directAuthSecret = env.THUNDERID_DIRECT_AUTH_SECRET;

        if (!baseUrl || !directAuthSecret) {
            throw new Error(
                "THUNDER_BASE_URL and THUNDERID_DIRECT_AUTH_SECRET are required in AuthZEN mode",
            );
        }

        const subjectLabel = subject.type
            ? `${subject.type}:${subject.id}`
            : subject.id;
        const response = await fetchImpl(
            `${baseUrl}/access/v1/evaluation`,
            {
                method: "POST",
                headers: {
                    "Direct-Auth-Secret": directAuthSecret,
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
