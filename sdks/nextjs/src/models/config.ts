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

import {ThunderIDNodeConfig} from '@thunderid/node';

/**
 * Configuration type for the ThunderID Next.js SDK.
 * Extends ThunderIDNodeConfig to provide Next.js-specific authentication configuration.
 *
 * @remarks
 * Configuration options include:
 * - Authentication endpoints and parameters
 * - Next.js specific redirects and middleware settings
 * - Session configuration for Next.js apps
 * - Environment variable integration
 */
export type ThunderIDNextConfig = ThunderIDNodeConfig;
