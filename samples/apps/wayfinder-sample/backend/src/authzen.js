// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
        const baseUrl = env.THUNDERID_BASE_URL;
        const directAuthSecret = env.THUNDERID_DIRECT_AUTH_SECRET;

        if (!baseUrl || !directAuthSecret) {
            throw new Error(
                "THUNDERID_BASE_URL and THUNDERID_DIRECT_AUTH_SECRET are required in AuthZEN mode",
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
