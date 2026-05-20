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
 * @thunderid/nuxt/server
 *
 * Public server-only barrel. Import from this subpath to access
 * server utilities without bundling them into the client.
 *
 * @example
 * ```ts
 * import { useServerSession, requireServerSession } from '@thunderid/nuxt/server';
 * ```
 */

export {useServerSession, requireServerSession} from './utils/serverSession';
export {getValidAccessToken} from './utils/token-refresh';
export {getThunderIDContext} from './utils/event-context';
export type {ThunderIDEventContext} from './utils/event-context';

export type {ThunderIDSessionPayload, ThunderIDNuxtConfig} from '../types';
