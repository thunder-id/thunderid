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
import logger from '../../utils/logger';
import SessionManager from '../../utils/SessionManager';

type RequestCookies = Awaited<ReturnType<typeof cookies>>;

/**
 * Deletes all ThunderID session cookies from the browser without contacting the
 * identity server.
 *
 * Use this for error-recovery scenarios where the local session must be wiped
 * immediately: refresh token failures, corrupt sessions, or forced local sign-out
 * when the identity server is unreachable.
 *
 * For a complete sign-out that also revokes the server-side session and obtains the
 * after-sign-out redirect URL, use `signOutAction` instead.
 *
 * @example
 * ```typescript
 * import { clearSession } from '@thunderid/nextjs/server';
 *
 * // Inside a Server Action or Route Handler:
 * await clearSession();
 * redirect('/sign-in');
 * ```
 */
const clearSession = async (): Promise<void> => {
  const cookieStore: RequestCookies = await cookies();
  cookieStore.delete(SessionManager.getSessionCookieName());
  cookieStore.delete(SessionManager.getTempSessionCookieName());
  logger.debug('[clearSession] Session cookies cleared.');
};

export default clearSession;
