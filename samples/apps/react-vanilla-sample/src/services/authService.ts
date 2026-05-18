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

import config from '../config';

export const NativeAuthSubmitType = {
    INPUT: 'INPUT',
    SOCIAL: 'SOCIAL',
    OTP: 'OTP',
} as const;

export type NativeAuthSubmitType = (typeof NativeAuthSubmitType)[keyof typeof NativeAuthSubmitType];

// WebAuthn/Passkey helper types
export interface PasskeyCreationOptions {
    challenge: string;
    rp: {
        name: string;
        id: string;
    };
    user: {
        name: string;
        displayName: string;
        id: string;
    };
    pubKeyCredParams: Array<{
        type: string;
        alg: number;
    }>;
    authenticatorSelection?: {
        authenticatorAttachment?: string;
        residentKey?: string;
        userVerification?: string;
    };
    timeout?: number;
    attestation?: string;
}

/**
 * Response data from passkey credential creation (registration).
 * Contains the encoded credential data to be sent to the server for verification.
 */
export interface PasskeyCredentialResponse {
    credentialId: string;
    clientDataJSON: string;
    attestationObject: string;
}

/**
 * Converts an ArrayBuffer to a base64url-encoded string.
 * This is the standard encoding for WebAuthn data.
 * 
 * @param {ArrayBuffer} buffer - The buffer to encode.
 * @returns {string} - The base64url-encoded string.
 */
export const bufferToBase64Url = (buffer: ArrayBuffer): string => {
    const bytes = new Uint8Array(buffer);
    const binary = Array.from(bytes, b => String.fromCharCode(b)).join('');
    return btoa(binary)
        .replace(/\+/g, '-')
        .replace(/\//g, '_')
        .replace(/=+$/, '');
};

/**
 * Converts a base64url-encoded string to an ArrayBuffer.
 * 
 * @param {string} base64url - The base64url string to decode.
 * @returns {ArrayBuffer} - The decoded ArrayBuffer.
 */
export const base64UrlToBuffer = (base64url: string): ArrayBuffer => {
    const padding = '='.repeat((4 - (base64url.length % 4)) % 4);
    const base64 = (base64url + padding)
        .replace(/-/g, '+')
        .replace(/_/g, '/');
    const binary = atob(base64);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) {
        bytes[i] = binary.charCodeAt(i);
    }
    return bytes.buffer;
};

/**
 * Creates a passkey credential using the WebAuthn API.
 * 
 * @param {PasskeyCreationOptions} options - The passkey creation options from the server.
 * @returns {Promise<PasskeyCredentialResponse>} - The encoded credential response.
 */
export const createPasskeyCredential = async (
    options: PasskeyCreationOptions
): Promise<PasskeyCredentialResponse> => {
    // Convert base64url-encoded challenge and user.id to ArrayBuffer
    const publicKeyOptions: PublicKeyCredentialCreationOptions = {
        challenge: base64UrlToBuffer(options.challenge),
        rp: options.rp,
        user: {
            name: options.user.name,
            displayName: options.user.displayName,
            id: base64UrlToBuffer(options.user.id),
        },
        pubKeyCredParams: options.pubKeyCredParams.map(param => ({
            type: param.type as PublicKeyCredentialType,
            alg: param.alg,
        })),
        authenticatorSelection: options.authenticatorSelection ? {
            authenticatorAttachment: options.authenticatorSelection.authenticatorAttachment as AuthenticatorAttachment | undefined,
            residentKey: options.authenticatorSelection.residentKey as ResidentKeyRequirement | undefined,
            userVerification: options.authenticatorSelection.userVerification as UserVerificationRequirement | undefined,
        } : undefined,
        timeout: options.timeout,
        attestation: options.attestation as AttestationConveyancePreference | undefined,
    };

    // Call WebAuthn API
    const credential = await navigator.credentials.create({
        publicKey: publicKeyOptions,
    }) as PublicKeyCredential | null;

    // Check if credential creation was successful
    if (!credential) {
        throw new Error('Passkey creation was cancelled or failed. No credential was returned.');
    }

    const response = credential.response as AuthenticatorAttestationResponse;

    // Encode the response data as base64url strings
    return {
        credentialId: credential.id,
        clientDataJSON: bufferToBase64Url(response.clientDataJSON),
        attestationObject: bufferToBase64Url(response.attestationObject),
    };
};

// Passkey Authentication (Assertion) types
export interface PasskeyRequestOptions {
    challenge: string;
    rpId: string;
    allowCredentials?: Array<{
        type: string;
        id: string;
    }>;
    userVerification?: string;
    timeout?: number;
}

/**
 * Response data from passkey authentication (assertion).
 * Contains the encoded assertion data to be sent to the server for verification.
 */
export interface PasskeyAssertionResponse {
    credentialId: string;
    clientDataJSON: string;
    authenticatorData: string;
    signature: string;
    userHandle: string;
}

/**
 * Authenticates with a passkey using the WebAuthn API (assertion/get).
 * 
 * @param {PasskeyRequestOptions} options - The passkey request options from the server.
 * @returns {Promise<PasskeyAssertionResponse>} - The encoded assertion response.
 */
export const authenticateWithPasskey = async (
    options: PasskeyRequestOptions
): Promise<PasskeyAssertionResponse> => {
    // Convert base64url-encoded values to ArrayBuffer
    const publicKeyOptions: PublicKeyCredentialRequestOptions = {
        challenge: base64UrlToBuffer(options.challenge),
        rpId: options.rpId,
        allowCredentials: options.allowCredentials?.map(cred => ({
            type: cred.type as PublicKeyCredentialType,
            id: base64UrlToBuffer(cred.id),
        })),
        userVerification: options.userVerification as UserVerificationRequirement | undefined,
        timeout: options.timeout,
    };

    // Call WebAuthn API for assertion
    const credential = await navigator.credentials.get({
        publicKey: publicKeyOptions,
    }) as PublicKeyCredential | null;

    // Check if credential retrieval was successful
    if (!credential) {
        throw new Error('Passkey authentication was cancelled or failed. No credential was returned.');
    }

    const response = credential.response as AuthenticatorAssertionResponse;

    // Encode the response data as base64url strings
    return {
        credentialId: credential.id,
        clientDataJSON: bufferToBase64Url(response.clientDataJSON),
        authenticatorData: bufferToBase64Url(response.authenticatorData),
        signature: bufferToBase64Url(response.signature),
        userHandle: response.userHandle ? bufferToBase64Url(response.userHandle) : '',
    };
};

type NativeAuthSubmitPayload =
  | { type: typeof NativeAuthSubmitType.INPUT; [key: string]: string }
  | { type: typeof NativeAuthSubmitType.SOCIAL; code: string }
  | { type: typeof NativeAuthSubmitType.OTP; otp: string };

const { applicationID, flowEndpoint } = config;

/**
 * Initiates the native authentication or registration flow by sending a POST request to the flow endpoint.
 * 
 * @param {string} flowType - The type of flow to initiate. Defaults to 'LOGIN'.
 * @returns {Promise<object>} - A promise that resolves to the response data from the server.
 */
export const initiateNativeAuthFlow = async (flowType: 'LOGIN' | 'REGISTRATION' = 'LOGIN') => {
    const headers = {
        'Content-Type': 'application/json'
    };

    const data: Record<string, string> = {
        "applicationId": applicationID
    };

    if (flowType === 'REGISTRATION') {
        data.flowType = 'REGISTRATION';
    } else {
        data.flowType = 'AUTHENTICATION';
    }

    const response = await fetch(`${flowEndpoint}/execute`, {
        method: 'POST',
        headers,
        body: JSON.stringify(data),
    });

    if (!response.ok) {
        const errorData = await response.json().catch(() => ({})) as { message?: { defaultValue?: string } };
        const flowTypeName = flowType === 'REGISTRATION' ? 'registration' : 'authentication';
        const message = response.status === 400
            ? `Error initiating native ${flowTypeName} request.`
            : errorData?.message?.defaultValue || 'Server error occurred.';
        throw new Error(message);
    }

    return { data: await response.json() };
};

/**
 * Initiates the native authentication or registration flow with additional data.
 * 
 * @param {string} flowType - The type of flow to initiate. Defaults to 'LOGIN'.
 * @param {string} actionId - The ID of the action to execute.
 * @param {object} inputs - Optional input data to include in the request.
 * @returns {Promise<object>} - A promise that resolves to the response data from the server.
 */
export const initiateNativeAuthFlowWithData = async (flowType: 'LOGIN' | 'REGISTRATION' = 'LOGIN', 
    actionId: string | null, inputs?: Record<string, unknown>) => {
    const headers = {
        'Content-Type': 'application/json'
    };

    const data: Record<string, unknown> = {
        "applicationId": applicationID,
    };

    if (actionId) {
        data.action = actionId;
    }

    if (flowType === 'REGISTRATION') {
        data.flowType = 'REGISTRATION';
    } else {
        data.flowType = 'AUTHENTICATION';
    }

    // Include inputs if provided
    if (inputs && Object.keys(inputs).length > 0) {
        data.inputs = inputs;
    }

    const response = await fetch(`${flowEndpoint}/execute`, {
        method: 'POST',
        headers,
        body: JSON.stringify(data),
    });

    if (!response.ok) {
        const errorData = await response.json().catch(() => ({})) as { message?: { defaultValue?: string } };
        const flowTypeName = flowType === 'REGISTRATION' ? 'registration' : 'authentication';
        const message = response.status === 400
            ? `Error initiating native ${flowTypeName} request.`
            : errorData?.message?.defaultValue || 'Server error occurred.';
        throw new Error(message);
    }

    return { data: await response.json() };
};

/**
 * Submits the user's selected authentication option when multiple options are available.
 * 
 * @param {string} executionId - The flow ID received from the initiateNativeAuth response.
 * @param {string} actionId - The ID of the selected authentication action.
 * @param {object} inputs - Optional input data to submit with the decision.
 * @param {string} challengeToken - Optional challenge token for the current step, if required by the server.
 * @returns {Promise<object>} - A promise that resolves to the response data from the server.
 */
export const submitAuthDecision = async (executionId: string, actionId: string, inputs?: Record<string, unknown>, challengeToken?: string) => {
    const headers = {
        'Content-Type': 'application/json'
    };

    const data: Record<string, unknown> = {
        executionId: executionId,
        action: actionId
    };

    if (challengeToken) {
        data.challengeToken = challengeToken;
    }

    // Include inputs if provided
    if (inputs && Object.keys(inputs).length > 0) {
        data.inputs = inputs;
    }

    const response = await fetch(`${flowEndpoint}/execute`, {
        method: 'POST',
        headers,
        body: JSON.stringify(data),
    });

    if (!response.ok) {
        const errorData = await response.json().catch(() => ({})) as { message?: { defaultValue?: string } };
        const message = response.status === 400
            ? 'Error processing authentication option.'
            : errorData?.message?.defaultValue || 'Server error occurred.';
        throw new Error(message);
    }

    return { data: await response.json() };
};

/**
 * Submits the native authentication form data to the server.
 * 
 * @param {string} executionId - The flow ID received from the initiateNativeAuth response.
 * @param {object} payload - The payload containing the form data or other required information.
 * @param {string} action - Optional action ref to include in the request.
 * @param {string} challengeToken - Optional challenge token for the current step, if required by the server.
 * @returns {Promise<object>} - A promise that resolves to the response data from the server.
 */
export const submitNativeAuth = async (
    executionId: string,
    payload: Record<string, unknown> | NativeAuthSubmitPayload,
    action?: string,
    challengeToken?: string
) => {
    const headers = {
        'Content-Type': 'application/json'
    };

    const data: Record<string, unknown> = {
        executionId: executionId
    };

    // Include action if provided
    if (action) {
        data.action = action;
    }

    if (challengeToken) {
        data.challengeToken = challengeToken;
    }

    if ('type' in payload) {
        if (payload.type === NativeAuthSubmitType.INPUT) {
            // For input type, include all fields except 'type'
            const { ...inputValues } = payload;
            data.inputs = inputValues;
        } else if (payload.type === NativeAuthSubmitType.SOCIAL) {
            data.inputs = {
                code: payload.code
            };
        } else if (payload.type === NativeAuthSubmitType.OTP) {
            data.inputs = {
                otp: payload.otp
            };
        }
    } else {
        // Handle as generic payload
        data.inputs = payload;
    }

    const response = await fetch(`${flowEndpoint}/execute`, {
        method: 'POST',
        headers,
        body: JSON.stringify(data),
    });

    if (!response.ok) {
        const errorData = await response.json().catch(() => ({})) as { message?: { defaultValue?: string } };
        const message = response.status === 400
            ? 'Login failed. Please check your credentials.'
            : errorData?.message?.defaultValue || 'Server error occurred.';
        throw new Error(message);
    }

    return { data: await response.json() };
}

