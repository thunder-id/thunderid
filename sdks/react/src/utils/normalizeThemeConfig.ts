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

import {RecursivePartial, ThemeConfig} from '@thunderid/browser';

interface ColorWithMain {
  [key: string]: unknown;
  main: string;
}

/**
 * Normalizes a single color value that may have been supplied as a shorthand
 * CSS color string (`'#2563eb'`) instead of the expected object form
 * (`{ main: '#2563eb' }`).
 *
 * This makes the `preferences.theme.overrides.colors.*` API forgiving for
 * JavaScript callers who don't have TypeScript's type-checker to catch the
 * mismatch at compile time.
 */
const normalizeColorValue = (color: string | ColorWithMain): ColorWithMain =>
  typeof color === 'string' ? {main: color} : color;

/**
 * Normalizes a `RecursivePartial<ThemeConfig>` so that color fields which are
 * supplied as plain CSS color strings are coerced into `{ main: string }`
 * objects before being handed to `createTheme`.
 *
 * Only the color groups that `toCssVariables` in `createTheme` actually reads
 * individual sub-keys from are normalized here (`primary`, `secondary`,
 * `error`, `success`, `warning`, `info`).  `border` is left alone because it
 * IS a plain string in `ThemeConfig`.
 */
const normalizeThemeConfig = (
  config: RecursivePartial<ThemeConfig> | undefined,
): RecursivePartial<ThemeConfig> | undefined => {
  if (!config?.colors) {
    return config;
  }

  const {primary, secondary, error, success, warning, info, ...restColors} = config.colors as any;

  return {
    ...config,
    colors: {
      ...restColors,
      ...(primary !== undefined ? {primary: normalizeColorValue(primary)} : {}),
      ...(secondary !== undefined ? {secondary: normalizeColorValue(secondary)} : {}),
      ...(error !== undefined ? {error: normalizeColorValue(error)} : {}),
      ...(success !== undefined ? {success: normalizeColorValue(success)} : {}),
      ...(warning !== undefined ? {warning: normalizeColorValue(warning)} : {}),
      ...(info !== undefined ? {info: normalizeColorValue(info)} : {}),
    },
  };
};

export default normalizeThemeConfig;
