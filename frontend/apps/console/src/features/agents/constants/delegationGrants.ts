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

import {OAuth2GrantTypes} from '../../applications/models/oauth';

/**
 * authorization_code, ciba, and refresh_token only make sense once the agent can act on behalf
 * of a user; they stay visible in grant-type pickers but are locked (unselectable) until
 * Delegated mode is turned on.
 */
export const DELEGATED_ONLY_GRANTS: string[] = [
  OAuth2GrantTypes.AUTHORIZATION_CODE,
  OAuth2GrantTypes.CIBA,
  OAuth2GrantTypes.REFRESH_TOKEN,
];
