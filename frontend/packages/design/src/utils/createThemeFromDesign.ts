// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {isEmpty, merge} from '@thunderid/utils';
import {CssVarsThemeOptions, extendTheme} from '@wso2/oxygen-ui';
import type {Theme} from '../models/theme';

/**
 * Builds an Oxygen UI theme from a Design theme as the server stores it.
 *
 * @param designTheme - The Design theme JSON.
 * @returns The theme, or `undefined` when there is no Design theme to build from.
 *
 * @public
 */
export default function createThemeFromDesign(designTheme: unknown): Theme | undefined {
  if (!designTheme || isEmpty(designTheme)) {
    return undefined;
  }

  const themeOptions = merge({...(designTheme as object)} as CssVarsThemeOptions, {
    colorSchemeSelector: 'data-color-scheme',
  });

  // MUI's extendTheme only accepts 'light' or 'dark' for defaultColorScheme.
  // 'system' is a runtime concept — remove it so MUI falls
  // back to its default.
  if ((themeOptions as Record<string, unknown>)['defaultColorScheme'] === 'system') {
    delete (themeOptions as Record<string, unknown>)['defaultColorScheme'];
  }

  return extendTheme(themeOptions) as unknown as Theme;
}
