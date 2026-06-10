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

'use server';

import {
  generateSessionId,
  EmbeddedSignInFlowStatus,
  EmbeddedFlowExecuteRequestConfig,
  EmbeddedSignInFlowResponse,
  IdToken,
  isEmpty,
} from '@thunderid/node';
import {cookies} from 'next/headers';
import {ThunderIDNextConfig} from '../../models/config';
import logger from '../../utils/logger';
import SessionManager, {SessionTokenPayload} from '../../utils/SessionManager';
import getClient from '../getClient';

type RequestCookies = Awaited<ReturnType<typeof cookies>>;

/**
 * Server action for signing in a user.
 * Handles the embedded sign-in flow and manages session cookies.
 *
 * @param payload - The embedded sign-in flow payload
 * @param request - The embedded flow execute request config
 * @returns Promise that resolves when sign-in is complete
 */
const signInAction = async (
  payload?: any,
  request?: EmbeddedFlowExecuteRequestConfig,
): Promise<{
  data?: {afterSignInUrl?: string; signInUrl?: string} | EmbeddedSignInFlowResponse;
  error?: string;
  success: boolean;
}> => {
  try {
    const client = getClient();
    const cookieStore: RequestCookies = await cookies();

    let sessionId: string | undefined;

    const existingSessionToken: string | undefined = cookieStore.get(SessionManager.getSessionCookieName())?.value;

    if (existingSessionToken) {
      try {
        const sessionPayload: SessionTokenPayload = await SessionManager.verifySessionToken(existingSessionToken);
        sessionId = sessionPayload.sessionId;
      } catch {
        // Invalid session token, will create new temp session
      }
    }

    if (!sessionId) {
      const tempSessionToken: string | undefined = cookieStore.get(SessionManager.getTempSessionCookieName())?.value;

      if (tempSessionToken) {
        try {
          const tempSession: {sessionId: string} = await SessionManager.verifyTempSession(tempSessionToken);
          sessionId = tempSession.sessionId;
        } catch {
          // Invalid temp session, will create new one
        }
      }
    }

    if (!sessionId) {
      sessionId = generateSessionId();

      const tempSessionToken: string = await SessionManager.createTempSession(sessionId);

      cookieStore.set(
        SessionManager.getTempSessionCookieName(),
        tempSessionToken,
        SessionManager.getTempSessionCookieOptions(),
      );
    }

    // If no payload provided, redirect to sign-in URL for redirect-based sign-in.
    if (!payload || isEmpty(payload)) {
      const defaultSignInUrl: string = await client.getAuthorizeRequestUrl({}, sessionId);
      return {data: {signInUrl: String(defaultSignInUrl)}, success: true};
    }

    // Handle embedded sign-in flow
    const response: any = await client.signIn(payload, request!, sessionId);

    if (response.flowStatus === EmbeddedSignInFlowStatus.Complete) {
      const signInResult: Record<string, unknown> = await client.signIn(
        {
          code: response?.authData?.code,
          session_state: response?.authData?.session_state,
          state: response?.authData?.state,
        } as any,
        {},
        sessionId,
      );

      if (signInResult) {
        const idToken: IdToken = await client.getDecodedIdToken(
          sessionId,
          (signInResult['idToken'] || signInResult['id_token']) as string,
        );
        const userIdFromToken: string = (idToken.sub || signInResult['sub'] || sessionId) as string;
        const {accessToken}: {accessToken: string} = signInResult as {accessToken: string};
        const refreshToken: string = (signInResult['refreshToken'] as string | undefined) ?? '';
        const scopes: string = signInResult['scope'] as string;
        const organizationId: string | undefined = (idToken['user_org'] || idToken['organization_id']) as
          | string
          | undefined;
        const rawExpiresIn: unknown = signInResult['expiresIn'] ?? signInResult['expires_in'];
        const expiresIn = Number(rawExpiresIn);
        if (Number.isNaN(expiresIn)) {
          throw new Error(`[signInAction] Invalid expiresIn value received: ${rawExpiresIn}`);
        }
        const config: ThunderIDNextConfig = await client.getConfiguration();
        const sessionCookieExpiryTime: number = SessionManager.resolveSessionCookieExpiry(
          config.sessionCookie?.expiryTime,
        );

        const sessionToken: string = await SessionManager.createSessionToken(
          accessToken,
          userIdFromToken,
          sessionId,
          scopes,
          expiresIn,
          refreshToken,
          organizationId,
        );

        cookieStore.set(
          SessionManager.getSessionCookieName(),
          sessionToken,
          SessionManager.getSessionCookieOptions(sessionCookieExpiryTime),
        );

        cookieStore.delete(SessionManager.getTempSessionCookieName());
      }

      const afterSignInUrl: string = await (await client.getStorageManager()).getConfigDataParameter('afterSignInUrl');
      return {data: {afterSignInUrl: String(afterSignInUrl)}, success: true};
    }

    return {data: response as EmbeddedSignInFlowResponse, success: true};
  } catch (error) {
    logger.error(`[signInAction] Error during sign-in: ${error instanceof Error ? error.message : String(error)}`);
    return {error: String(error), success: false};
  }
};

export default signInAction;
