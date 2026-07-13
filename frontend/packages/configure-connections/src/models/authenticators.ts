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
 * Authenticator Types
 *
 * Supported authenticator types in the platform.
 * These represent different authentication methods and protocols.
 *
 * @public
 */
export const AuthenticatorTypes = {
  CREDENTIALS_AUTH: 'credentials_auth',
  PASSKEY: 'passkey',
} as const;

/**
 * Authenticator Type
 *
 * Type alias that derives all supported authenticator values
 * from {@link AuthenticatorTypes}.
 *
 * @public
 * @example
 * ```ts
 * const authenticator: AuthenticatorType = AuthenticatorTypes.CREDENTIALS_AUTH;
 * ```
 */
export type AuthenticatorType = (typeof AuthenticatorTypes)[keyof typeof AuthenticatorTypes];
