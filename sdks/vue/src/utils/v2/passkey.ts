/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {ThunderIDRuntimeError, arrayBufferToBase64url, base64urlToArrayBuffer} from '@thunderid/browser';

/**
 * Handles WebAuthn/Passkey registration flow for browser environments.
 *
 * @param challengeData - JSON stringified challenge data containing WebAuthn creation options.
 * @returns Promise that resolves to a JSON string containing the WebAuthn registration response.
 */
export const handlePasskeyRegistration = async (challengeData: string): Promise<string> => {
  if (!window.navigator.credentials?.create) {
    throw new ThunderIDRuntimeError(
      'WebAuthn is not supported in this browser.',
      'browser-webauthn-not-supported',
      'browser',
      'WebAuthn/Passkey registration requires a browser that supports the Web Authentication API.',
    );
  }

  try {
    const creationOptions: any = JSON.parse(challengeData);

    const publicKey: any = {
      ...creationOptions,
      challenge: base64urlToArrayBuffer(creationOptions.challenge),
      user: {
        ...creationOptions.user,
        id: base64urlToArrayBuffer(creationOptions.user.id),
      },
      ...(creationOptions.excludeCredentials && {
        excludeCredentials: creationOptions.excludeCredentials.map((cred: any) => ({
          ...cred,
          id: base64urlToArrayBuffer(cred.id),
        })),
      }),
    };

    const credential: PublicKeyCredential = (await navigator.credentials.create({
      publicKey,
    })) as PublicKeyCredential;

    if (!credential) {
      throw new ThunderIDRuntimeError(
        'No credential returned from WebAuthn registration.',
        'browser-webauthn-no-credential',
        'browser',
        'The WebAuthn registration ceremony completed but did not return a valid credential.',
      );
    }

    const response: AuthenticatorAttestationResponse = credential.response as AuthenticatorAttestationResponse;

    const registrationResponse: any = {
      id: credential.id,
      rawId: arrayBufferToBase64url(credential.rawId),
      response: {
        attestationObject: arrayBufferToBase64url(response.attestationObject),
        clientDataJSON: arrayBufferToBase64url(response.clientDataJSON),
        ...(response.getTransports && {
          transports: response.getTransports(),
        }),
      },
      type: credential.type,
      ...(credential.authenticatorAttachment && {
        authenticatorAttachment: credential.authenticatorAttachment,
      }),
    };

    return JSON.stringify(registrationResponse);
  } catch (error) {
    if (error instanceof ThunderIDRuntimeError) {
      throw error;
    }

    if (error instanceof Error) {
      throw new ThunderIDRuntimeError(
        `Passkey registration failed: ${error.message}`,
        'browser-webauthn-registration-error',
        'browser',
        `WebAuthn registration failed with error: ${error.name}`,
      );
    }

    throw new ThunderIDRuntimeError(
      'Passkey registration failed due to an unexpected error.',
      'browser-webauthn-unexpected-error',
      'browser',
      'An unexpected error occurred during WebAuthn registration.',
    );
  }
};

/**
 * Handles WebAuthn/Passkey authentication flow for browser environments.
 *
 * @param challengeData - JSON stringified challenge data containing WebAuthn request options.
 * @returns Promise that resolves to a JSON string containing the WebAuthn authentication response.
 */
export const handlePasskeyAuthentication = async (challengeData: string): Promise<string> => {
  if (!window.navigator.credentials?.get) {
    throw new ThunderIDRuntimeError(
      'WebAuthn is not supported in this browser.',
      'browser-webauthn-not-supported',
      'browser',
      'WebAuthn/Passkey authentication requires a browser that supports the Web Authentication API.',
    );
  }

  try {
    const requestOptions: any = JSON.parse(challengeData);

    const publicKey: any = {
      ...requestOptions,
      challenge: base64urlToArrayBuffer(requestOptions.challenge),
      ...(requestOptions.allowCredentials && {
        allowCredentials: requestOptions.allowCredentials.map((cred: any) => ({
          ...cred,
          id: base64urlToArrayBuffer(cred.id),
        })),
      }),
    };

    const credential: PublicKeyCredential = (await navigator.credentials.get({
      publicKey,
    })) as PublicKeyCredential;

    if (!credential) {
      throw new ThunderIDRuntimeError(
        'No credential returned from WebAuthn authentication.',
        'browser-webauthn-no-credential',
        'browser',
        'The WebAuthn authentication ceremony completed but did not return a valid credential.',
      );
    }

    const response: AuthenticatorAssertionResponse = credential.response as AuthenticatorAssertionResponse;

    const authenticationResponse: any = {
      id: credential.id,
      rawId: arrayBufferToBase64url(credential.rawId),
      response: {
        authenticatorData: arrayBufferToBase64url(response.authenticatorData),
        clientDataJSON: arrayBufferToBase64url(response.clientDataJSON),
        signature: arrayBufferToBase64url(response.signature),
        ...(response.userHandle && {
          userHandle: arrayBufferToBase64url(response.userHandle),
        }),
      },
      type: credential.type,
      ...(credential.authenticatorAttachment && {
        authenticatorAttachment: credential.authenticatorAttachment,
      }),
    };

    return JSON.stringify(authenticationResponse);
  } catch (error) {
    if (error instanceof ThunderIDRuntimeError) {
      throw error;
    }

    if (error instanceof Error) {
      throw new ThunderIDRuntimeError(
        `Passkey authentication failed: ${error.message}`,
        'browser-webauthn-authentication-error',
        'browser',
        `WebAuthn authentication failed with error: ${error.name}`,
      );
    }

    throw new ThunderIDRuntimeError(
      'Passkey authentication failed due to an unexpected error.',
      'browser-webauthn-unexpected-error',
      'browser',
      'An unexpected error occurred during WebAuthn authentication.',
    );
  }
};
