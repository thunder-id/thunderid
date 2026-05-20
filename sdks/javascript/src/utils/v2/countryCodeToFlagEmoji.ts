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
 * Converts a two-letter ISO 3166-1 alpha-2 country code to a flag emoji using
 * Unicode Regional Indicator Symbols (U+1F1E6–U+1F1FF).
 *
 * @param countryCode - Two-letter uppercase country code (e.g. "US", "GB")
 * @returns Flag emoji string (e.g. "🇺🇸", "🇬🇧")
 */
export default function countryCodeToFlagEmoji(countryCode: string): string {
  return countryCode
    .toUpperCase()
    .split('')
    .map((char: string) => String.fromCodePoint(0x1f1e6 - 65 + char.charCodeAt(0)))
    .join('');
}
