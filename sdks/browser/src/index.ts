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
 * Entry point for all public APIs of the @thunderid/browser SDK.
 */

// Client
export {default as ThunderIDBrowserClient} from './ThunderIDBrowserClient';
export {default as FetchHttpClient} from './FetchHttpClient';

// Constants
export {default as Hooks, Hooks as BrowserHooks} from './constants/Hooks';
export * from './constants/SPAConstants';

// Models
export {default as BrowserStorage} from './models/BrowserStorage';
export type {BrowserClientConfig, BrowserAuthConfig} from './models/BrowserConfig';
export type {default as SignInConfig} from './models/SignInConfig';
export type {default as SignOutError} from './models/SignOutError';
export type {SPATokenExchangeConfig} from './models/TokenExchangeConfig';
export type {ThunderIDBrowserConfig} from './models/config';

// Stores
export {default as LocalStore} from './stores/LocalStore';
export {default as SessionStore} from './stores/SessionStore';
export {default as MemoryStore} from './stores/MemoryStore';

// Utils
export {default as SPACryptoUtils} from './utils/SPACryptoUtils';
export {default as SPAUtils} from './utils/SPAUtils';
export {default as SPAHelper} from './utils/SPAHelper';
export {default as AuthenticationHelper} from './utils/AuthenticationHelper';
export {default as createSessionManagementHelper} from './utils/SessionManagementHelper';
export type {SessionManagementHelperInterface} from './utils/SessionManagementHelper';
export {default as hasAuthParamsInUrl} from './utils/hasAuthParamsInUrl';
export {default as hasCalledForThisInstanceInUrl} from './utils/hasCalledForThisInstanceInUrl';
export {default as navigate} from './utils/navigate';
export {default as http} from './utils/http';
export {default as handleWebAuthnAuthentication} from './utils/handleWebAuthnAuthentication';
export {default as resolveEmojiUrisInHtml} from './utils/v2/resolveEmojiUrisInHtml';

// Theme
export {detectThemeMode, createClassObserver, createMediaQueryListener} from './theme/themeDetection';
export type {BrowserThemeDetection} from './theme/themeDetection';
export {default as getActiveTheme} from './theme/getActiveTheme';

// Re-export everything from the JavaScript SDK
export * from '@thunderid/javascript';
