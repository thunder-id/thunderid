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

import {DEFAULT_THEME, ThemeMode} from '@thunderid/javascript';
import {BrowserThemeDetection, detectThemeMode} from './themeDetection';

/**
 * Gets the active theme based on the theme mode preference
 * @param mode - The theme mode preference ('light', 'dark', 'system', or 'class')
 * @param config - Additional configuration for theme detection
 * @returns 'light' or 'dark' based on the resolved theme
 */
const getActiveTheme = (mode: ThemeMode, config: BrowserThemeDetection = {}): ThemeMode => {
  if (mode === 'dark') {
    return 'dark';
  }

  if (mode === 'light') {
    return 'light';
  }

  if (mode === 'system') {
    if (typeof window !== 'undefined' && window.matchMedia) {
      return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
    }

    // Default to light mode if system detection is not available
    return DEFAULT_THEME;
  }

  if (mode === 'class') {
    return detectThemeMode(mode, config);
  }

  // Default to light mode for any unknown mode
  return DEFAULT_THEME;
};

export default getActiveTheme;
