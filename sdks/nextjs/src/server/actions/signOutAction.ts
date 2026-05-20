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

import {cookies} from 'next/headers';
import getSessionId from './getSessionId';
import getClient from '../getClient';
import logger from '../../utils/logger';
import SessionManager from '../../utils/SessionManager';

type RequestCookies = Awaited<ReturnType<typeof cookies>>;

/**
 * Server action for signing out a user.
 * Clears both JWT and legacy session cookies.
 *
 * @returns Promise that resolves with success status and optional after sign-out URL
 */
const signOutAction = async (): Promise<{data?: {afterSignOutUrl?: string}; error?: unknown; success: boolean}> => {
  logger.debug('[signOutAction] Initiating sign out process from the server action.');

  const clearSessionCookies = async (): Promise<void> => {
    const cookieStore: RequestCookies = await cookies();

    cookieStore.delete(SessionManager.getSessionCookieName());
    cookieStore.delete(SessionManager.getTempSessionCookieName());
  };

  try {
    const client = getClient();
    const sessionId: string | undefined = await getSessionId();

    let afterSignOutUrl = '/';

    if (sessionId) {
      logger.debug('[signOutAction] Session ID found, invoking the `signOut` to obtain the `afterSignOutUrl`.');

      afterSignOutUrl = await client.signOut({}, sessionId);
    }

    await clearSessionCookies();

    return {data: {afterSignOutUrl}, success: true};
  } catch (error) {
    logger.error('[signOutAction] Error during sign out from the server action:', error);

    logger.debug('[signOutAction] Clearing session cookies due to error as a fallback.');

    await clearSessionCookies();

    let errorMessage: unknown;
    if (typeof error === 'string') {
      errorMessage = error;
    } else if (error instanceof Error) {
      errorMessage = error.message;
    } else {
      errorMessage = JSON.stringify(error);
    }

    return {
      error: errorMessage,
      success: false,
    };
  }
};

export default signOutAction;
