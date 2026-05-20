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

/**
 * Converts an ArrayBuffer to a base64url encoded string.
 *
 * Base64url encoding is a URL-safe variant of base64 encoding that:
 * - Replaces '+' with '-'
 * - Replaces '/' with '_'
 * - Removes padding '=' characters
 *
 * This encoding is commonly used in JWT tokens, OAuth2 PKCE challenges,
 * and other web standards where the encoded data needs to be safely
 * transmitted in URLs or HTTP headers.
 *
 * @param buffer - The ArrayBuffer to convert to base64url string
 * @returns The base64url encoded string representation of the input buffer
 *
 * @example
 * ```typescript
 * const buffer = new TextEncoder().encode('Hello World');
 * const encoded = arrayBufferToBase64url(buffer);
 * console.log(encoded); // "SGVsbG8gV29ybGQ"
 * ```
 *
 * @example
 * ```typescript
 * // Converting crypto random bytes for PKCE challenge
 * const randomBytes = crypto.getRandomValues(new Uint8Array(32));
 * const codeVerifier = arrayBufferToBase64url(randomBytes.buffer);
 * ```
 */
const arrayBufferToBase64url = (buffer: ArrayBuffer): string => {
  const bytes: Uint8Array<ArrayBuffer> = new Uint8Array(buffer);
  let binary = '';

  for (let i = 0; i < bytes.byteLength; i += 1) {
    binary += String.fromCharCode(bytes[i]);
  }

  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '');
};

export default arrayBufferToBase64url;
