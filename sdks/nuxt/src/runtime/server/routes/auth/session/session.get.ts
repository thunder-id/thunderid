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

import {defineEventHandler} from 'h3';
import type {H3Event} from 'h3';
import type {ThunderIDAuthState} from '../../../../types';
import ThunderIDNuxtClient from '../../../ThunderIDNuxtClient';
import {verifyAndRehydrateSession} from '../../../utils/serverSession';
import {useRuntimeConfig} from '#imports';

/**
 * GET /api/auth/session
 *
 * Returns the current auth state: { isSignedIn, user, isLoading }.
 * Used by the client-side composable to hydrate auth state.
 */
export default defineEventHandler(async (event: H3Event): Promise<ThunderIDAuthState> => {
  const config: ReturnType<typeof useRuntimeConfig> = useRuntimeConfig();
  const sessionSecret: string | undefined = config.thunderid?.sessionSecret;

  const session: Awaited<ReturnType<typeof verifyAndRehydrateSession>> = await verifyAndRehydrateSession(
    event,
    sessionSecret,
  );
  if (!session) {
    return {isLoading: false, isSignedIn: false, user: null};
  }

  try {
    const client: ThunderIDNuxtClient = ThunderIDNuxtClient.getInstance();
    const user: Awaited<ReturnType<ThunderIDNuxtClient['getUser']>> = await client.getUser(session.sessionId);
    return {isLoading: false, isSignedIn: true, user};
  } catch {
    return {isLoading: false, isSignedIn: false, user: null};
  }
});
