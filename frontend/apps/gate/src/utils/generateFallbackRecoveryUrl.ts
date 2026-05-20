/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import ROUTES from '../constants/routes';

/**
 * Generate a fallback recovery URL preserving the current query string.
 *
 * Used when the flow meta does not provide an explicit
 * `application.forgot_password_url`. The generated URL preserves whatever query
 * parameters are currently in the address bar so the auth flow context
 * (e.g. `client_id`, `redirect_uri`) is carried over.
 *
 * @param searchParams - The current {@link URLSearchParams} from the page URL.
 * @returns An absolute-path URL string pointing to the recovery route.
 */
export default function generateFallbackRecoveryUrl(searchParams: URLSearchParams): string {
  const base = import.meta.env.BASE_URL.replace(/\/$/, '');
  const recoveryPath = `${base}${ROUTES.AUTH.RECOVERY}`;
  const currentParams: string = searchParams.toString();

  return currentParams ? `${recoveryPath}?${currentParams}` : recoveryPath;
}
