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

import {EmbeddedFlowStatus} from '@thunderid/node';
import {defineEventHandler, readBody, createError} from 'h3';
import type {H3Event} from 'h3';
import ThunderIDNuxtClient from '../../../ThunderIDNuxtClient';
import {useRuntimeConfig} from '#imports';

function hasFlowStatus(value: unknown): value is {flowStatus?: EmbeddedFlowStatus} {
  return typeof value === 'object' && value !== null && 'flowStatus' in value;
}

/**
 * POST /api/auth/signup
 *
 * Handles embedded (app-native) sign-up flow steps.
 *
 * Request body:
 * - `payload` — the embedded sign-up flow step payload (`EmbeddedFlowExecuteRequestPayload`).
 *   When omitted, returns an empty `signUpUrl` (caller should redirect to the sign-up page).
 *
 * Response shape:
 * ```json
 * { "data": { ... }, "success": true }
 * ```
 */
export default defineEventHandler(async (event: H3Event) => {
  const config: ReturnType<typeof useRuntimeConfig> = useRuntimeConfig();
  // Mirror Next.js: after-sign-up redirect reuses the configured `afterSignInUrl`
  // (the user typically signs in immediately after registering).
  const afterSignUpUrl: string = ((config.public.thunderid as any)?.afterSignInUrl as string | undefined) || '/';

  // ── Parse request body ────────────────────────────────────────────────────
  const body: {payload?: Record<string, unknown>} = await readBody(event);
  const payload: Record<string, unknown> | undefined = body?.payload;

  // No payload — return an empty signUpUrl so the client can redirect.
  if (!payload) {
    return {data: {signUpUrl: ''}, success: true};
  }

  // ── Execute embedded sign-up flow step ────────────────────────────────────
  const client: ThunderIDNuxtClient = ThunderIDNuxtClient.getInstance();

  let response: unknown;
  try {
    response = await client.signUp(payload as any);
  } catch (err: any) {
    throw createError({
      statusCode: 502,
      statusMessage: `Embedded sign-up step failed: ${err?.message ?? String(err)}`,
    });
  }

  // ── Flow complete ─────────────────────────────────────────────────────────
  if (hasFlowStatus(response) && response.flowStatus === EmbeddedFlowStatus.Complete) {
    return {data: {afterSignUpUrl}, success: true};
  }

  // ── Flow incomplete — return step data to the client ──────────────────────
  return {data: response, success: true};
});
