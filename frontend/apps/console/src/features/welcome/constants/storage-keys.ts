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

/**
 * Storage key suffix constants for the welcome feature.
 * The product name prefix is prepended at runtime via the utility functions.
 *
 * Utility functions in the `utils` directory generate the full storage keys by replacing the `{{productName}}`
 * placeholder with the actual product name at runtime.
 */
const WelcomeStorageKeys = {
  DISMISSED: '{{productName}}:welcome:dismissed',
  SESSION_CHECKED: '{{productName}}:welcome:session-checked',
  WAYFINDER_CONFIGURED: '{{productName}}:wayfinder-config-imported',
  WAYFINDER_SETUP_EXPANDED: '{{productName}}:wayfinder-setup-expanded',
} as const;

export default WelcomeStorageKeys;
