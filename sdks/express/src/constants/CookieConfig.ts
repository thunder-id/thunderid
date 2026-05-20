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

/** Session cookie name used across the Express SDK. */
export const SESSION_COOKIE_NAME = 'THUNDERID_SESSION_ID';

/** Default cookie configuration values. */
const CookieConfig = {
  defaultExpirySeconds: 86400,
  defaultHttpOnly: true,
  defaultSameSite: 'lax' as const,
  defaultSecure: false,
};

export default CookieConfig;
