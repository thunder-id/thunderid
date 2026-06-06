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

import {OAuth2GrantTypes} from '../models/oauth';

/**
 * Returns a human-readable label for a given OAuth2 grant type value.
 * For known grant types with long URN identifiers a friendly name is returned.
 * All other grant type values are returned unchanged.
 */
export function getGrantTypeLabel(grant: string, t: (key: string, fallback: string) => string): string {
  if (grant === OAuth2GrantTypes.CIBA) {
    return t('applications:edit.advanced.grantTypes.labels.ciba', 'CIBA (Client-Initiated Backchannel Authentication)');
  }
  return grant;
}
