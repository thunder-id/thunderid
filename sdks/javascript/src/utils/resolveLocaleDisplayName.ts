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
 * Resolves a BCP 47 locale tag to a human-readable display name using the
 * `Intl.DisplayNames` API.
 *
 * Falls back to the raw locale code if the runtime does not support
 * `Intl.DisplayNames` or if resolution returns `undefined`.
 *
 * @param locale - BCP 47 locale tag to resolve (e.g. "en", "fr", "zh-Hant")
 * @param displayLocale - Locale used for the display name language (defaults to "en")
 * @returns Human-readable language name (e.g. "English", "French")
 */
export default function resolveLocaleDisplayName(locale: string, displayLocale: string): string {
  try {
    const displayNames: Intl.DisplayNames = new Intl.DisplayNames([displayLocale], {type: 'language'});
    return displayNames.of(locale) ?? locale;
  } catch {
    return locale;
  }
}
