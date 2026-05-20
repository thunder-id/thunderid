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
 * OAuth 2.0 client authentication method used at the token endpoint.
 * Corresponds to the `token_endpoint_auth_method` parameter in OIDC Discovery.
 *
 * - `client_secret_basic` — HTTP Basic authentication: credentials are sent in the
 *   `Authorization: Basic base64(client_id:client_secret)` header (RFC 6749 §2.3.1).
 *   Required for ThunderIDV2 (Thunder) by default.
 * - `client_secret_post` — Credentials are sent as `client_id` / `client_secret`
 *   parameters in the POST body (RFC 6749 §2.3.1). Default for all other platforms.
 * - `none` — No client authentication (public clients that have no client secret).
 */
export type TokenEndpointAuthMethod = 'client_secret_basic' | 'client_secret_post' | 'none';
