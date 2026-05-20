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
 * Re-export of the active theme resolver from the Browser SDK.
 *
 * Gets the active theme based on the theme mode preference:
 * - `'light'` / `'dark'` → Returns the specified mode directly.
 * - `'system'` → Uses `matchMedia('(prefers-color-scheme: dark)')` to detect system preference.
 * - `'class'` → Inspects DOM element class list for dark/light classes.
 *
 * @see {@link @thunderid/browser#getActiveTheme}
 */
export {getActiveTheme, getActiveTheme as default} from '@thunderid/browser';
