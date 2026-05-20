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
import {getValidAccessToken} from '../../../utils/token-refresh';

/**
 * GET /api/auth/token
 *
 * Returns a valid access token for the current session.
 * Proactively refreshes the token if it is within 60 seconds of expiry
 * (requires a refresh token stored in the session JWT).
 * Returns 401 if there is no active session or the token cannot be refreshed.
 */
export default defineEventHandler(async (event: H3Event): Promise<{accessToken: string}> => {
  const accessToken: string = await getValidAccessToken(event);
  return {accessToken};
});
