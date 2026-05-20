/**
 * Copyright (c) 2022, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
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

import TokenConstants from './constants/TokenConstants';
import {ThunderIDAuthException} from './errors/exception';
import {Crypto, JWKInterface} from './models/crypto';

export class IsomorphicCrypto<T = any> {
  private cryptoUtils: Crypto<T>;

  public constructor(cryptoUtils: Crypto<T>) {
    this.cryptoUtils = cryptoUtils;
  }

  /**
   * Generate code verifier.
   *
   * @returns code verifier.
   */
  public getCodeVerifier(): string {
    return this.cryptoUtils.base64URLEncode(this.cryptoUtils.generateRandomBytes(32));
  }

  /**
   * Derive code challenge from the code verifier.
   *
   * @param verifier - Code verifier.
   *
   * @returns - code challenge.
   */
  public async getCodeChallenge(verifier: string): Promise<string> {
    const hashed: T = await this.cryptoUtils.hashSha256(verifier);
    return this.cryptoUtils.base64URLEncode(hashed);
  }

  /**
   * Get JWK used for the id_token
   *
   * @param jwtHeader - header of the id_token.
   * @param keys - jwks response.
   *
   * @returns public key.
   *
   * @throws
   */
  /* eslint-disable @typescript-eslint/no-explicit-any */
  public getJWKForTheIdToken(jwtHeader: string, keys: JWKInterface[]): JWKInterface {
    const headerJSON: Record<string, string> = JSON.parse(this.cryptoUtils.base64URLDecode(jwtHeader));

    const matchingKey: JWKInterface | undefined = keys.find(
      (key: JWKInterface): boolean => headerJSON['kid'] === key.kid,
    );

    if (matchingKey) {
      return matchingKey;
    }

    throw new ThunderIDAuthException(
      'JS-CRYPTO_UTIL-GJFTIT-IV01',
      'kid not found.',
      `Failed to find the 'kid' specified in the id_token. 'kid' found in the header : ${
        headerJSON['kid']
      }, Expected values: ${keys.map((key: JWKInterface) => key.kid).join(', ')}`,
    );
  }

  /**
   * Verify id token.
   *
   * @param idToken - id_token received from the IdP.
   * @param jwk - public key used for signing.
   * @param clientId - app identification.
   * @param issuer - id_token issuer.
   * @param username - Username.
   * @param clockTolerance - Allowed leeway for id_tokens (in seconds).
   *
   * @returns whether the id_token is valid.
   *
   * @throws
   */
  public isValidIdToken(
    idToken: string,
    jwk: JWKInterface,
    clientId: string,
    issuer: string,
    username: string,
    clockTolerance: number | undefined,
    validateJwtIssuer: boolean | undefined,
  ): Promise<boolean> {
    return this.cryptoUtils
      .verifyJwt(
        idToken,
        jwk,
        TokenConstants.SignatureValidation.SUPPORTED_ALGORITHMS as unknown as string[],
        clientId,
        issuer,
        username,
        clockTolerance,
        validateJwtIssuer,
      )
      .then((response: boolean) => {
        if (response) {
          return Promise.resolve(true);
        }

        return Promise.reject(
          new ThunderIDAuthException(
            'JS-CRYPTO_HELPER-IVIT-IV01',
            'Invalid ID token.',
            'ID token validation returned false',
          ),
        );
      })
      .catch((error: ThunderIDAuthException) => Promise.reject(error));
  }

  public decodeJwtToken<R = Record<string, unknown>>(token: string): R {
    try {
      const utf8String: string = this.cryptoUtils.base64URLDecode(token?.split('.')[1]);
      const payload: R = JSON.parse(utf8String);

      return payload;
    } catch (error: any) {
      throw new ThunderIDAuthException('JS-CRYPTO_UTIL-DIT-IV02', 'Decoding token failed.', error);
    }
  }
}
