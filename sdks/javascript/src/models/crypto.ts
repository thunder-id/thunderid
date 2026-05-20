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
 * JWK Model
 */
export interface JWKInterface {
  alg: string;
  e: string;
  kid: string;
  kty: string;
  n: string;
  use: string;
}

/**
 * Cryptographic utility interface for OIDC operations.
 * Provides methods for encoding, decoding, hashing, and JWT verification
 * used in OAuth2/OIDC flows.
 *
 * @remarks
 * This interface abstracts cryptographic operations needed for:
 * - PKCE challenge/verifier generation
 * - JWT token validation
 * - Base64URL encoding/decoding
 * - Secure random number generation
 *
 * @example
 * ```typescript
 * class MyCrypto implements Crypto<Uint8Array> {
 *   base64URLEncode(value: Uint8Array): string {
 *     // Implementation
 *   }
 *   // ... other implementations
 * }
 * ```
 */
export interface Crypto<T = any> {
  /**
   * Decode the provided data encoded in base64url format.
   *
   * @param value - Data to be decoded.
   *
   * @returns Decoded data.
   */
  base64URLDecode(value: string): string;

  /**
   * Encode the provided data in base64url format.
   *
   * @param value - Data to be encoded.
   *
   * @returns Encoded data.
   */
  base64URLEncode(value: T): string;

  /**
   * Generate random bytes.
   *
   * @param length - Length of the random bytes to be generated.
   *
   * @returns Random bytes.
   */
  generateRandomBytes(length: number): T;

  /**
   * Hash the provided data using SHA-256.
   *
   * @param data - Data to be hashed.
   *
   * @returns Hashed data.
   */
  hashSha256(data: string): T | Promise<T>;

  /**
   * Verify the provided JWT.
   *
   * @param idToken - ID Token to be verified.
   * @param jwk - JWK to be used for verification.
   * @param algorithms - Algorithms to be used for verification.
   * @param clientId - Client ID to be used for verification.
   * @param issuer - Issuer to be used for verification.
   * @param subject - Subject to be used for verification.
   * @param clockTolerance - Clock tolerance to be used for verification.
   *
   * @returns True if the ID Token is valid.
   *
   * @throws if the id_token is invalid.
   */
  verifyJwt(
    idToken: string,
    jwk: JWKInterface,
    algorithms: string[],
    clientId: string,
    issuer: string,
    subject: string,
    clockTolerance?: number,
    validateJwtIssuer?: boolean,
  ): Promise<boolean>;
}
