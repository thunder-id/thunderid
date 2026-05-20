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

import getSessionId from './getSessionId';
import getSessionPayload from './getSessionPayload';
import getClient from '../getClient';
import {SessionTokenPayload} from '../../utils/SessionManager';

/**
 * Check if the user is currently signed in.
 *
 * For JWT-based sessions: the session JWT exp claim is now tied to the access
 * token expiry. A successful jwtVerify (inside getSessionPayload) already proves
 * exp > now, so no separate timestamp comparison is needed here.
 *
 * Falls back to the legacy SDK in-memory check when no JWT session cookie exists.
 *
 * @param sessionId - Optional session ID (used only for the legacy fallback path)
 * @returns True if the user is signed in with a valid, non-expired token
 */
const isSignedIn = async (sessionId?: string): Promise<boolean> => {
  try {
    const sessionPayload: SessionTokenPayload | undefined = await getSessionPayload();

    if (sessionPayload) {
      return true;
    }

    // No JWT session — fall back to the legacy SDK in-memory store check.
    const resolvedSessionId: string | undefined = sessionId || (await getSessionId());

    if (!resolvedSessionId) {
      return false;
    }

    const client = getClient();

    try {
      const accessToken: string = await client.getAccessToken(resolvedSessionId);
      return !!accessToken;
    } catch {
      return false;
    }
  } catch {
    return false;
  }
};

export default isSignedIn;
