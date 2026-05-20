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

/**
 * Utility to check if `code` and `session_state` are available in the URL as search params.
 *
 * @param params - The URL search params to check. Defaults to `window.location.search`.
 * @return `true` if the URL contains `code` and `session_state` search params, otherwise `false`.
 */
const hasAuthParamsInUrl = (params: string = window.location.search): boolean => {
  const MATCHER = /[?&]code=[^&]+/;

  return MATCHER.test(params);
};

export default hasAuthParamsInUrl;
