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
 * Re-exports of theme detection utilities from the Browser SDK.
 *
 * - `detectThemeMode` — Detects current theme mode based on system preference or DOM class.
 * - `createClassObserver` — Creates a MutationObserver watching for CSS class changes on a target element.
 * - `createMediaQueryListener` — Creates a media query listener for `prefers-color-scheme` changes.
 * - `BrowserThemeDetection` — Configuration interface for DOM-specific theme detection options.
 *
 * @see {@link @thunderid/browser#detectThemeMode}
 * @see {@link @thunderid/browser#createClassObserver}
 * @see {@link @thunderid/browser#createMediaQueryListener}
 */
export {
  detectThemeMode,
  createClassObserver,
  createMediaQueryListener,
  type BrowserThemeDetection,
} from '@thunderid/browser';
