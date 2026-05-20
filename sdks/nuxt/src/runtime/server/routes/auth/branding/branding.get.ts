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

import type {BrandingPreference} from '@thunderid/node';
import {defineEventHandler, createError} from 'h3';
import type {H3Event} from 'h3';
import type {ThunderIDNuxtConfig} from '../../../../types';
import ThunderIDNuxtClient from '../../../ThunderIDNuxtClient';
import {verifyAndRehydrateSession} from '../../../utils/serverSession';
import {useRuntimeConfig} from '#imports';

/**
 * GET /api/auth/branding
 *
 * Returns the branding preference for the current tenant / organisation context.
 * Resolves the correct `baseUrl` (org-scoped if the session is inside an org).
 * Does not require an authenticated session — unauthenticated callers receive
 * the root-tenant branding.
 *
 * Used by `ThunderIDRoot.revalidateBranding` to refresh client-side branding
 * state without a full page reload.
 */
export default defineEventHandler(async (event: H3Event): Promise<BrandingPreference | null> => {
  const config: ReturnType<typeof useRuntimeConfig> = useRuntimeConfig(event);
  const publicConfig: ThunderIDNuxtConfig = config.public.thunderid as ThunderIDNuxtConfig;
  const sessionSecret: string | undefined = config.thunderid?.sessionSecret;

  const baseUrl: string = (publicConfig?.baseUrl ?? '') as string;
  let resolvedBaseUrl: string = baseUrl;

  // Attempt to resolve the org-scoped base URL from the session, if present.
  try {
    const session: Awaited<ReturnType<typeof verifyAndRehydrateSession>> = await verifyAndRehydrateSession(
      event,
      sessionSecret,
    );
    if (session) {
      if (session.organizationId) {
        resolvedBaseUrl = `${baseUrl}/o`;
      } else {
        const client: ThunderIDNuxtClient = ThunderIDNuxtClient.getInstance();
        const idToken: Awaited<ReturnType<ThunderIDNuxtClient['getDecodedIdToken']>> = await client.getDecodedIdToken(
          session.sessionId,
        );
        if (idToken?.['user_org']) {
          resolvedBaseUrl = `${baseUrl}/o`;
        }
      }
    }
  } catch {
    // Non-fatal — fall back to the root tenant base URL
  }

  try {
    const client: ThunderIDNuxtClient = ThunderIDNuxtClient.getInstance();
    return await client.getBrandingPreference({baseUrl: resolvedBaseUrl});
  } catch (err) {
    throw createError({
      statusCode: 500,
      statusMessage: `Failed to retrieve branding preference: ${err instanceof Error ? err.message : String(err)}`,
    });
  }
});
