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

import TranslationBundleConstants from '../constants/TranslationBundleConstants';
import {I18nBundle} from '../models/i18n';
import * as translations from '../translations';

/**
 * Get the default i18n bundles.
 * Dynamically builds the bundles collection by iterating through supported locales
 * and importing their corresponding translation modules.
 *
 * @returns The collection of all default i18n bundles
 */
const getDefaultI18nBundles = (): Record<string, I18nBundle> => {
  const bundles: Record<string, I18nBundle> = {};

  // Iterate through supported locales and build bundles dynamically
  TranslationBundleConstants.DEFAULT_LOCALES.forEach((localeCode: string) => {
    // Convert locale code to translation module key (e.g., 'en-US' -> 'en_US')
    const moduleKey: string = localeCode.replace('-', '_') as keyof typeof translations;

    // Get the translation bundle from the translations module
    const bundle: I18nBundle | undefined = translations[moduleKey] as I18nBundle;

    if (bundle && bundle.metadata?.localeCode) {
      bundles[bundle.metadata.localeCode] = bundle;
    }
  });

  return bundles;
};

export default getDefaultI18nBundles;
