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
 * Converts a base64url encoded string back to an ArrayBuffer.
 *
 * This function performs the inverse operation of base64url encoding by:
 * - Replacing URL-safe characters: '-' becomes '+', '_' becomes '/'
 * - Adding back padding '=' characters that were removed during base64url encoding
 * - Decoding the resulting base64 string to binary data
 * - Converting the binary data to an ArrayBuffer
 *
 * This is commonly used for decoding JWT tokens, OAuth2 PKCE code verifiers,
 * and other cryptographic data that was encoded using base64url format.
 *
 * @param base64url - The base64url encoded string to decode
 * @returns The ArrayBuffer containing the decoded binary data
 *
 * @throws {DOMException} Throws an error if the input string is not valid base64url
 *
 * @example
 * ```typescript
 * const encoded = 'SGVsbG8gV29ybGQ';
 * const buffer = base64urlToArrayBuffer(encoded);
 * const text = new TextDecoder().decode(buffer);
 * console.log(text); // "Hello World"
 * ```
 *
 * @example
 * ```typescript
 * // Decoding a JWT payload
 * const jwtPayload = 'eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ';
 * const payloadBuffer = base64urlToArrayBuffer(jwtPayload);
 * const payloadJson = new TextDecoder().decode(payloadBuffer);
 * const payload = JSON.parse(payloadJson);
 * ```
 *
 * @see {@link arrayBufferToBase64url} - The inverse function for encoding ArrayBuffer to base64url
 */
const base64urlToArrayBuffer = (base64url: string): ArrayBuffer => {
  const padding: string = '='.repeat((4 - (base64url.length % 4)) % 4);
  const base64: string = base64url.replace(/-/g, '+').replace(/_/g, '/') + padding;

  const binaryString: string = atob(base64);
  const bytes: Uint8Array<ArrayBuffer> = new Uint8Array(binaryString.length);

  for (let i = 0; i < binaryString.length; i += 1) {
    bytes[i] = binaryString.charCodeAt(i);
  }

  return bytes.buffer;
};

export default base64urlToArrayBuffer;
