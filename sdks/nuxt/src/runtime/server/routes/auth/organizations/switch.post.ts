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

import type {Organization, TokenResponse} from '@thunderid/node';
import {defineEventHandler, readBody, createError} from 'h3';
import type {H3Event} from 'h3';
import ThunderIDNuxtClient from '../../../ThunderIDNuxtClient';
import {verifyAndRehydrateSession} from '../../../utils/serverSession';
import {issueSessionCookie} from '../../../utils/session';
import {useRuntimeConfig} from '#imports';

/**
 * POST /api/auth/organizations/switch
 *
 * Performs an `organization_switch` token exchange for the given organisation,
 * then re-issues the JWT session cookie so subsequent requests carry the new
 * organisation context.
 *
 * Request body: `{ organization: Organization }`
 *
 * Mirrors `switchOrganization` server action in the Next.js SDK.
 */
export default defineEventHandler(async (event: H3Event): Promise<{success: boolean}> => {
  const config: ReturnType<typeof useRuntimeConfig> = useRuntimeConfig();
  const sessionSecret: string | undefined = config.thunderid?.sessionSecret;

  const session: Awaited<ReturnType<typeof verifyAndRehydrateSession>> = await verifyAndRehydrateSession(
    event,
    sessionSecret,
  );
  if (!session) {
    throw createError({statusCode: 401, statusMessage: 'Unauthorized: Invalid or expired session.'});
  }
  const {sessionId} = session;

  let organization: Organization;
  try {
    const body: {organization: Organization} = await readBody<{organization: Organization}>(event);
    organization = body.organization;
  } catch {
    throw createError({statusCode: 400, statusMessage: 'Invalid request body.'});
  }

  if (!organization?.id) {
    throw createError({statusCode: 400, statusMessage: 'organization.id is required.'});
  }

  let tokenResponse: TokenResponse;
  try {
    const client: ThunderIDNuxtClient = ThunderIDNuxtClient.getInstance();
    const response: TokenResponse | Response = await client.switchOrganization(organization, sessionId);
    tokenResponse = response as TokenResponse;
  } catch (err) {
    throw createError({
      statusCode: 500,
      statusMessage: `Organisation switch failed: ${err instanceof Error ? err.message : String(err)}`,
    });
  }

  // Re-issue the session cookie with the new token so subsequent SSR requests
  // pick up the switched organisation context — mirrors callback.get.ts.
  try {
    const runtimeConfig: ReturnType<typeof useRuntimeConfig> = useRuntimeConfig();
    const runtimeSessionSecret: string | undefined = runtimeConfig.thunderid?.sessionSecret;
    await issueSessionCookie(event, sessionId, tokenResponse, runtimeSessionSecret);
  } catch (err) {
    throw createError({
      statusCode: 500,
      statusMessage: `Failed to establish new session after organisation switch: ${
        err instanceof Error ? err.message : String(err)
      }`,
    });
  }

  return {success: true};
});
