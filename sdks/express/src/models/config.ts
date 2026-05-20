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

import {ThunderIDNodeConfig, ThunderIDRuntimeError, TokenResponse} from '@thunderid/node';
import express from 'express';

/**
 * Express-specific configuration fields.
 */
export interface StrictExpressClientConfig {
  /** Called with the response and token on successful sign-in. */
  onSignIn?: (res: express.Response, tokenResponse: TokenResponse) => void;
  /** Called with the response on successful sign-out. */
  onSignOut?: (res: express.Response) => void;
  /** Called with the response and error on authentication failure. */
  onError?: (res: express.Response, exception: ThunderIDRuntimeError) => void;
  /** Called with the response when a protected route is accessed without a valid session. */
  onUnauthenticated?: (res: express.Response) => void;
}

/**
 * Full configuration type for `ThunderIDExpressClient`.
 * Combines node-level auth config with Express-specific settings.
 *
 * `afterSignInUrl` and `afterSignOutUrl` are optional. When omitted, the SDK
 * infers them from the first incoming request's origin combined with the path
 * derived from those URLs (defaulting to `/login` and `/logout`).
 *
 * Set `mode: 'embedded'` to enable app-native embedded auth via `handleFlow()`.
 * Defaults to `'redirect'` (standard OAuth 2.0 authorization-code flow).
 */
export type ExpressClientConfig = ThunderIDNodeConfig & StrictExpressClientConfig;

/**
 * Configuration type for the ThunderID Express.js SDK.
 */
export type ThunderIDExpressConfig = ExpressClientConfig;
