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

import {Theme} from '@thunderid/browser';
import {Context, createContext} from 'react';

export interface ThemeContextValue {
  /**
   * Error from branding theme fetch, if any
   */
  brandingError?: Error | null;
  colorScheme: 'light' | 'dark';
  /**
   * The text direction for the UI.
   */
  direction: 'ltr' | 'rtl';
  /**
   * Whether branding inheritance is enabled
   */
  inheritFromBranding?: boolean;
  /**
   * Whether branding theme is currently loading
   */
  isBrandingLoading?: boolean;
  theme: Theme;
  toggleTheme: () => void;
}

const ThemeContext: Context<ThemeContextValue | null> = createContext<ThemeContextValue | null>(null);

ThemeContext.displayName = 'ThemeContext';

export default ThemeContext;
