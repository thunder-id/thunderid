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
 * Utility to check if `state` is available in the URL as a search param and matches the provided instance.
 *
 * @param params - The URL search params to check. Defaults to `window.location.search`.
 * @param instanceId - The instance ID to match against the `state` param.
 * @return `true` if the URL contains a matching `state` search param, otherwise `false`.
 */
const hasCalledForThisInstanceInUrl = (instanceId: number, params: string = window.location.search): boolean => {
  const MATCHER = new RegExp(`[?&]state=instance_${instanceId}_[^&]+`);

  return MATCHER.test(params);
};

export default hasCalledForThisInstanceInUrl;
