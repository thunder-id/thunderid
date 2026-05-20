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

import type {H3Event} from 'h3';
import type {ThunderIDSessionPayload, ThunderIDSSRData} from '../../types';

/**
 * The typed shape of `event.context.thunderid` set by the ThunderID Nitro plugin
 * on every SSR request.
 */
export interface ThunderIDEventContext {
  /** Convenience boolean derived from the session presence. */
  isSignedIn: boolean;
  /** The decoded session payload, or null when the user is not signed in. */
  session: ThunderIDSessionPayload | null;
  /** SSR-prefetched data (user profile, orgs, branding). Present only after the SSR plugin runs. */
  ssr?: ThunderIDSSRData;
}

/**
 * Typed accessor for `event.context.thunderid`.
 *
 * Returns null when called before the ThunderID SSR plugin has populated
 * the context (e.g. in non-Nuxt Nitro routes that run before the plugin).
 *
 * @example
 * ```ts
 * import { getThunderIDContext } from '@thunderid/nuxt/server';
 *
 * export default defineEventHandler((event) => {
 *   const ctx = getThunderIDContext(event);
 *   if (!ctx?.isSignedIn) throw createError({ statusCode: 401 });
 *   return { userId: ctx.session!.sub };
 * });
 * ```
 */
export function getThunderIDContext(event: H3Event): ThunderIDEventContext | null {
  return (event.context.thunderid as ThunderIDEventContext | undefined) ?? null;
}
