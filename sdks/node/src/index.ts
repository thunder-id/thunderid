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

// Add Ponyfills for Fetch API
import fetch, {Headers, Request, Response} from 'cross-fetch';

if (!globalThis.fetch) {
  globalThis.fetch = fetch;
  globalThis.Headers = Headers;
  globalThis.Request = Request;
  globalThis.Response = Response;
}

/**
 * Entry point for all public APIs of the @thunderid/node SDK.
 */

// Client
export {default as ThunderIDNodeClient} from './ThunderIDNodeClient';

// Constants
export {default as CookieConfig} from './constants/CookieConfig';

// Models
export type {ThunderIDNodeConfig, SessionCookieConfig} from './models/config';
export type {default as AuthURLCallback} from './models/AuthURLCallback';

// Stores
export {default as MemoryCacheStore} from './stores/MemoryCacheStore';

// Utils
export {default as NodeCryptoUtils} from './utils/NodeCryptoUtils';
export {default as SessionUtils} from './utils/SessionUtils';
export {default as generateSessionId} from './utils/generateSessionId';
export {default as getSessionCookieOptions} from './utils/getSessionCookieOptions';

// Re-export everything from the JavaScript SDK
export * from '@thunderid/javascript';
