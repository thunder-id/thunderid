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
 * Query key constants for design feature cache management.
 */
const DesignQueryKeys = {
  /** Key for listing themes */
  THEMES: 'themes',

  /** Key for a specific theme by ID */
  THEME: 'theme',

  /** Key for listing layouts */
  LAYOUTS: 'layouts',

  /** Key for a specific layout by ID */
  LAYOUT: 'layout',

  /** Key for resolving design configuration by type and ID */
  DESIGN_RESOLVE: 'design-resolve',

  /** Key for theme usages (resources referencing a theme) */
  THEME_USAGES: 'theme-usages',
} as const;

export default DesignQueryKeys;
