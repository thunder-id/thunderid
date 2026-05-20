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

import {en_US} from '../translations';

/**
 * Constants related to internationalization (i18n) translation bundles.
 *
 * @example
 * ```typescript
 * // Using default locale
 * const locale = TranslationBundleConstants.FALLBACK_LOCALE;
 *
 * // Using supported locales
 * const locales = TranslationBundleConstants.DEFAULT_LOCALES;
 * ```
 */
const TranslationBundleConstants: {
  DEFAULT_LOCALES: string[];
  FALLBACK_LOCALE: string;
} = {
  /**
   * List of default locales bundles with the SDKs.
   *
   * Current default locales:
   * - `en-US` - English (United States)
   */
  DEFAULT_LOCALES: [en_US.metadata.localeCode],

  /**
   * Default locale code used as fallback when no specific locale is provided.
   */
  FALLBACK_LOCALE: en_US.metadata.localeCode,
} as const;

export default TranslationBundleConstants;
