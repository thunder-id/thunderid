/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import * as jose from 'jose';
import {Crypto, JWKInterface} from './models/crypto';

/**
 * Default implementation of the Crypto interface using the 'jose' library
 * and the native Web Crypto API.
 */
export class DefaultCrypto implements Crypto<Uint8Array> {
  public base64URLDecode(value: string): string {
    const decodedArray: Uint8Array = jose.base64url.decode(value);
    return new TextDecoder().decode(decodedArray);
  }

  public base64URLEncode(value: Uint8Array): string {
    return jose.base64url.encode(value);
  }

  public generateRandomBytes(length: number): Uint8Array {
    return crypto.getRandomValues(new Uint8Array(length));
  }

  public async hashSha256(data: string): Promise<Uint8Array> {
    const encoder: TextEncoder = new TextEncoder();
    const dataBuffer = encoder.encode(data);
    const hashBuffer: ArrayBuffer = await crypto.subtle.digest('SHA-256', dataBuffer);

    return new Uint8Array(hashBuffer);
  }

  public async verifyJwt(
    idToken: string,
    jwk: JWKInterface,
    algorithms: string[],
    clientId: string,
    issuer: string,
    subject: string,
    clockTolerance?: number,
    validateJwtIssuer = true,
  ): Promise<boolean> {
    const key: jose.KeyLike | Uint8Array = await jose.importJWK(jwk as jose.JWK);

    await jose.jwtVerify(idToken, key, {
      algorithms,
      audience: [clientId],
      clockTolerance,
      issuer: validateJwtIssuer ? issuer : undefined,
      subject,
    });

    return true;
  }
}
