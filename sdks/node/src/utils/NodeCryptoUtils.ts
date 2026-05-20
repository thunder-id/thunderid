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

import {Crypto, JWKInterface} from '@thunderid/javascript';
import base64url from 'base64url';
import sha256 from 'fast-sha256';
import * as jose from 'jose';
import randombytes from 'secure-random-bytes';

/**
 * Node.js crypto utilities implementing the `Crypto` interface.
 * Uses `jose`, `base64url`, `fast-sha256`, and `secure-random-bytes`.
 */
class NodeCryptoUtils implements Crypto<Buffer | string> {
  public base64URLEncode(value: Buffer | string): string {
    return base64url.encode(value).replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '');
  }

  public base64URLDecode(value: string): string {
    return base64url.decode(value).toString();
  }

  public hashSha256(data: string): string | Buffer {
    return Buffer.from(sha256(new TextEncoder().encode(data)));
  }

  public generateRandomBytes(length: number): string | Buffer {
    return randombytes(length);
  }

  public async verifyJwt(
    idToken: string,
    jwk: Partial<JWKInterface>,
    algorithms: string[],
    clientId: string,
    issuer: string,
    subject: string,
    clockTolerance?: number,
  ): Promise<boolean> {
    const key: jose.KeyLike | Uint8Array = await jose.importJWK(jwk);
    return jose
      .jwtVerify(idToken, key, {
        algorithms,
        audience: [clientId],
        clockTolerance,
        issuer,
        subject,
      })
      .then(() => Promise.resolve(true));
  }
}

export default NodeCryptoUtils;
