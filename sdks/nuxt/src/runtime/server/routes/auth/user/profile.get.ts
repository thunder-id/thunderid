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

import type {UserProfile} from '@thunderid/node';
import {defineEventHandler, createError} from 'h3';
import type {H3Event} from 'h3';
import ThunderIDNuxtClient from '../../../ThunderIDNuxtClient';
import {verifyAndRehydrateSession} from '../../../utils/serverSession';
import {useRuntimeConfig} from '#imports';

/**
 * GET /api/auth/user/profile
 *
 * Returns the full SCIM2 {@link UserProfile} (with `flattenedProfile` and
 * `schemas`) for the authenticated user.  Used by `ThunderIDRoot.revalidateProfile`
 * to refresh client-side state after a profile update.
 *
 * Mirrors `getUserProfileAction` in the Next.js SDK.
 */
export default defineEventHandler(async (event: H3Event): Promise<UserProfile> => {
  const config: ReturnType<typeof useRuntimeConfig> = useRuntimeConfig();
  const sessionSecret: string | undefined = config.thunderid?.sessionSecret;

  const session: Awaited<ReturnType<typeof verifyAndRehydrateSession>> = await verifyAndRehydrateSession(
    event,
    sessionSecret,
  );
  if (!session) {
    throw createError({statusCode: 401, statusMessage: 'Unauthorized: Invalid or expired session.'});
  }

  try {
    const client: ThunderIDNuxtClient = ThunderIDNuxtClient.getInstance();
    return await client.getUserProfile(session.sessionId);
  } catch (err) {
    throw createError({
      statusCode: 500,
      statusMessage: `Failed to retrieve user profile: ${err instanceof Error ? err.message : String(err)}`,
    });
  }
});
