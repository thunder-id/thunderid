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

import {I18nBundle} from '@thunderid/browser';
import {Context, createContext} from 'react';

export interface I18nContextValue {
  /**
   * All available i18n bundles (default + user provided)
   */
  bundles: Record<string, I18nBundle>;

  /**
   * The current language code (e.g., 'en-US', 'fr-FR')
   */
  currentLanguage: string;

  /**
   * The fallback language code
   */
  fallbackLanguage: string;

  /**
   * Injects additional bundles into the i18n system (e.g., from flow metadata).
   * Injected bundles take precedence over defaults but are overridden by prop-provided bundles.
   */
  injectBundles: (bundles: Record<string, I18nBundle>) => void;

  /**
   * Function to change the current language
   */
  setLanguage: (language: string) => void;

  /**
   * Function to get a translation by key with optional parameters
   */
  t: (key: string, params?: Record<string, string | number>) => string;
}

const I18nContext: Context<I18nContextValue | null> = createContext<I18nContextValue | null>(null);

I18nContext.displayName = 'I18nContext';

export default I18nContext;
