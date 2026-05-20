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

import {
  arrayBufferToBase64url,
  base64urlToArrayBuffer,
  ThunderIDRuntimeError,
  createPackageComponentLogger,
} from '@thunderid/javascript';

const logger: ReturnType<typeof createPackageComponentLogger> = createPackageComponentLogger(
  '@thunderid/browser',
  'WebAuthn',
);

/**
 * Handles WebAuthn/Passkey authentication flow for browser environments.
 *
 * This function processes a WebAuthn challenge, performs the authentication ceremony,
 * and returns the authentication response that can be sent to the server for verification.
 *
 * The function handles various aspects of WebAuthn authentication including:
 * - Browser compatibility checks for WebAuthn support
 * - HTTPS requirement validation (except for localhost development)
 * - Relying Party ID validation and domain compatibility
 * - Challenge data decoding and credential request options processing
 * - User authentication ceremony via navigator.credentials.get()
 * - Response formatting for server consumption
 *
 * @param challengeData - Base64-encoded challenge data containing WebAuthn request options.
 *                       This data typically includes the challenge, RP ID, allowed credentials,
 *                       user verification requirements, and other authentication parameters.
 *
 * @returns Promise that resolves to a JSON string containing the WebAuthn authentication response.
 *          The response includes the credential ID, authenticator data, client data JSON,
 *          signature, and optional user handle that can be verified by the server.
 *
 * @throws {ThunderIDRuntimeError} When WebAuthn is not supported in the current browser
 * @throws {ThunderIDRuntimeError} When the page is not served over HTTPS (except localhost)
 * @throws {ThunderIDRuntimeError} When the user cancels or times out the authentication
 * @throws {ThunderIDRuntimeError} When there's a domain/RP ID mismatch
 * @throws {ThunderIDRuntimeError} When no valid passkey is found for the account
 * @throws {ThunderIDRuntimeError} When WebAuthn is not supported on the device/browser
 * @throws {ThunderIDRuntimeError} When there's a network error during authentication
 * @throws {ThunderIDRuntimeError} For any other authentication failures
 *
 * @example
 * ```typescript
 * try {
 *   const challengeData = 'eyJwdWJsaWNLZXlDcmVkZW50aWFsUmVxdWVzdE9wdGlvbnMiOi4uLn0=';
 *   const authResponse = await handleWebAuthnAuthentication(challengeData);
 *
 *   // Send the response to your server for verification
 *   const result = await fetch('/api/verify-webauthn', {
 *     method: 'POST',
 *     headers: { 'Content-Type': 'application/json' },
 *     body: authResponse
 *   });
 * } catch (error) {
 *   if (error instanceof ThunderIDRuntimeError) {
 *     console.error('WebAuthn authentication failed:', error.message);
 *   }
 * }
 * ```
 *
 * @example
 * ```typescript
 * // Usage in an authentication flow
 * const authenticateWithPasskey = async (challengeFromServer: string) => {
 *   try {
 *     const response = await handleWebAuthnAuthentication(challengeFromServer);
 *     return JSON.parse(response);
 *   } catch (error) {
 *     // Handle specific error cases
 *     if (error instanceof ThunderIDRuntimeError) {
 *       switch (error.code) {
 *         case 'browser-webauthn-not-supported':
 *           showFallbackAuth();
 *           break;
 *         case 'browser-webauthn-user-cancelled':
 *           showRetryOption();
 *           break;
 *         default:
 *           showGenericError();
 *       }
 *     }
 *   }
 * };
 * ```
 *
 * @see {@link https://webauthn.guide/} - WebAuthn specification guide
 * @see {@link https://developer.mozilla.org/en-US/docs/Web/API/Web_Authentication_API} - MDN WebAuthn API documentation
 */
const handleWebAuthnAuthentication = async (challengeData: string): Promise<string> => {
  if (!window.navigator.credentials?.get) {
    throw new ThunderIDRuntimeError(
      'WebAuthn is not supported in this browser. Please use a modern browser or try a different authentication method.',
      'browser-webauthn-not-supported',
      'browser',
      'WebAuthn/Passkey authentication requires a browser that supports the Web Authentication API.',
    );
  }

  if (window.location.protocol !== 'https:' && window.location.hostname !== 'localhost') {
    throw new ThunderIDRuntimeError(
      'Passkey authentication requires a secure connection (HTTPS). Please ensure you are accessing this site over HTTPS.',
      'browser-webauthn-insecure-connection',
      'browser',
      'WebAuthn authentication requires HTTPS for security reasons, except when running on localhost for development.',
    );
  }

  try {
    const decodedChallenge: any = JSON.parse(atob(challengeData));
    const {publicKeyCredentialRequestOptions} = decodedChallenge;

    const currentDomain: string = window.location.hostname;
    const challengeRpId: string = publicKeyCredentialRequestOptions.rpId;

    let rpIdToUse: string = challengeRpId;

    if (challengeRpId && !currentDomain.endsWith(challengeRpId) && challengeRpId !== currentDomain) {
      logger.warn(`RP ID mismatch detected. Challenge RP ID: ${challengeRpId}, Current domain: ${currentDomain}`);
      rpIdToUse = currentDomain;
    }

    const adjustedOptions: any = {
      ...publicKeyCredentialRequestOptions,
      challenge: base64urlToArrayBuffer(publicKeyCredentialRequestOptions.challenge),
      rpId: rpIdToUse,
      ...(publicKeyCredentialRequestOptions.userVerification && {
        userVerification: publicKeyCredentialRequestOptions.userVerification,
      }),
      ...(publicKeyCredentialRequestOptions.allowCredentials && {
        allowCredentials: publicKeyCredentialRequestOptions.allowCredentials.map((cred: any) => ({
          ...cred,
          id: base64urlToArrayBuffer(cred.id),
        })),
      }),
    };

    const credential: PublicKeyCredential = (await navigator.credentials.get({
      publicKey: adjustedOptions,
    })) as PublicKeyCredential;

    if (!credential) {
      throw new ThunderIDRuntimeError(
        'No credential returned from WebAuthn authentication',
        'browser-webauthn-no-credential',
        'browser',
        'The WebAuthn authentication ceremony completed but did not return a valid credential.',
      );
    }

    const authData: AuthenticatorAssertionResponse = credential.response as AuthenticatorAssertionResponse;

    const tokenResponse: {credential: any; requestId: string} = {
      credential: {
        id: credential.id,
        rawId: arrayBufferToBase64url(credential.rawId),
        response: {
          authenticatorData: arrayBufferToBase64url(authData.authenticatorData),
          clientDataJSON: arrayBufferToBase64url(authData.clientDataJSON),
          signature: arrayBufferToBase64url(authData.signature),
          ...(authData.userHandle && {
            userHandle: arrayBufferToBase64url(authData.userHandle),
          }),
        },
        type: credential.type,
      },
      requestId: decodedChallenge.requestId,
    };

    return JSON.stringify(tokenResponse);
  } catch (error) {
    logger.error('WebAuthn authentication failed:');

    if (error instanceof ThunderIDRuntimeError) {
      throw error;
    }

    if (error instanceof Error) {
      switch (error.name) {
        case 'NotAllowedError':
          throw new ThunderIDRuntimeError(
            'Passkey authentication was cancelled or timed out. Please try again.',
            'browser-webauthn-user-cancelled',
            'browser',
            'The user cancelled the WebAuthn authentication request or the request timed out.',
          );

        case 'SecurityError':
          if (error.message.includes('relying party ID') || error.message.includes('RP ID')) {
            throw new ThunderIDRuntimeError(
              'Domain mismatch error. The passkey was registered for a different domain. Please contact support or try a different authentication method.',
              'browser-webauthn-domain-mismatch',
              'browser',
              'The WebAuthn relying party ID does not match the current domain.',
            );
          }
          throw new ThunderIDRuntimeError(
            'Passkey authentication failed due to a security error. Please ensure you are using HTTPS and that your browser supports passkeys.',
            'browser-webauthn-security-error',
            'browser',
            'A security error occurred during WebAuthn authentication.',
          );

        case 'InvalidStateError':
          throw new ThunderIDRuntimeError(
            'No valid passkey found for this account. Please register a passkey first or use a different authentication method.',
            'browser-webauthn-no-passkey',
            'browser',
            'No registered passkey credentials were found for the current user account.',
          );

        case 'NotSupportedError':
          throw new ThunderIDRuntimeError(
            'Passkey authentication is not supported on this device or browser. Please use a different authentication method.',
            'browser-webauthn-not-supported',
            'browser',
            'WebAuthn is not supported on the current device or browser configuration.',
          );

        case 'NetworkError':
          throw new ThunderIDRuntimeError(
            'Network error during passkey authentication. Please check your connection and try again.',
            'browser-webauthn-network-error',
            'browser',
            'A network error occurred while communicating with the authenticator.',
          );

        case 'UnknownError':
          throw new ThunderIDRuntimeError(
            'An unknown error occurred during passkey authentication. Please try again or use a different authentication method.',
            'browser-webauthn-unknown-error',
            'browser',
            'An unidentified error occurred during the WebAuthn authentication process.',
          );

        default:
          throw new ThunderIDRuntimeError(
            `Passkey authentication failed: ${error.message}`,
            'browser-webauthn-general-error',
            'browser',
            `WebAuthn authentication failed with error: ${error.name}`,
          );
      }
    }

    throw new ThunderIDRuntimeError(
      'Passkey authentication failed due to an unexpected error.',
      'browser-webauthn-unexpected-error',
      'browser',
      'An unexpected error occurred during WebAuthn authentication.',
    );
  }
};

export default handleWebAuthnAuthentication;
