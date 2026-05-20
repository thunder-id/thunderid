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

import {FlowMetaTheme, FlowMetaThemeColorScheme, RecursivePartial, ThemeConfig} from '@thunderid/browser';

/**
 * Converts a v2 `FlowMetaTheme` into a `RecursivePartial<ThemeConfig>` that
 * `createTheme` can consume.
 *
 * Only fields explicitly present in the FlowMeta response are included so that
 * `createTheme` can deep-merge them onto its base (light/dark) defaults without
 * accidentally dropping sibling keys that were not returned by the server.
 */
const buildThemeConfigFromFlowMeta = (
  flowMetaTheme: FlowMetaTheme,
  colorScheme: 'light' | 'dark',
): RecursivePartial<ThemeConfig> => {
  const scheme: FlowMetaThemeColorScheme | undefined = flowMetaTheme.colorSchemes?.[colorScheme];
  const borderRadius: number | undefined = flowMetaTheme.shape?.borderRadius;
  const borderRadiusStr: string | undefined = borderRadius !== undefined ? `${borderRadius}px` : undefined;

  let colors: RecursivePartial<ThemeConfig['colors']> | undefined;

  if (scheme?.palette) {
    colors = {};

    if (scheme.palette.primary) {
      colors.primary = scheme.palette.primary;
    }
    if (scheme.palette.secondary) {
      colors.secondary = scheme.palette.secondary;
    }
    if (scheme.palette.text) {
      colors.text = scheme.palette.text;
    }

    if (scheme.palette.background) {
      const bg: RecursivePartial<ThemeConfig['colors']['background']> = {};

      if (scheme.palette.background.default) {
        bg.body = {main: scheme.palette.background.default};
      }
      if (scheme.palette.background.paper) {
        bg.surface = scheme.palette.background.paper;
      }

      if (Object.keys(bg).length > 0) {
        colors.background = bg;
      }
    }
  }

  return {
    ...(flowMetaTheme.direction ? {direction: flowMetaTheme.direction} : {}),
    ...(borderRadiusStr
      ? {
          borderRadius: {
            large: borderRadiusStr,
            medium: borderRadiusStr,
            small: borderRadiusStr,
          },
        }
      : {}),
    ...(colors && Object.keys(colors).length > 0 ? {colors} : {}),
    ...(flowMetaTheme.typography?.fontFamily ? {typography: {fontFamily: flowMetaTheme.typography.fontFamily}} : {}),
  };
};

export default buildThemeConfigFromFlowMeta;
