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

import type {UpdateMeProfileConfig, User} from '@thunderid/node';
import {defineEventHandler, readBody, createError} from 'h3';
import type {H3Event} from 'h3';
import type {ThunderIDNuxtConfig} from '../../../../types';
import ThunderIDNuxtClient from '../../../ThunderIDNuxtClient';
import {verifyAndRehydrateSession} from '../../../utils/serverSession';
import {useRuntimeConfig} from '#imports';

/**
 * PATCH /api/auth/user/profile
 *
 * Updates the SCIM2 /Me profile for the authenticated user.
 * Mirrors the `updateUserProfileAction` Next.js server action.
 *
 * Request body: {@link UpdateMeProfileConfig} (the SCIM patch payload).
 * Response: `{ data: { user: User }; success: boolean; error: string }`
 */
export default defineEventHandler(
  async (event: H3Event): Promise<{data: {user: User}; error: string; success: boolean}> => {
    const config: ReturnType<typeof useRuntimeConfig> = useRuntimeConfig();
    const sessionSecret: string | undefined = config.thunderid?.sessionSecret;
    const publicConfig: ThunderIDNuxtConfig = config.public.thunderid as ThunderIDNuxtConfig;

    const session: Awaited<ReturnType<typeof verifyAndRehydrateSession>> = await verifyAndRehydrateSession(
      event,
      sessionSecret,
    );
    if (!session) {
      throw createError({statusCode: 401, statusMessage: 'Unauthorized: Invalid or expired session.'});
    }

    let payload: UpdateMeProfileConfig;
    try {
      payload = await readBody<UpdateMeProfileConfig>(event);
    } catch {
      throw createError({statusCode: 400, statusMessage: 'Invalid request body.'});
    }

    try {
      const client: ThunderIDNuxtClient = ThunderIDNuxtClient.getInstance();
      const user: User = await client.updateUserProfile(payload, session.sessionId);
      return {data: {user}, error: '', success: true};
    } catch (err) {
      throw createError({
        statusCode: 500,
        statusMessage: `Failed to update user profile: ${err instanceof Error ? err.message : String(err)}`,
      });
    }
  },
);
