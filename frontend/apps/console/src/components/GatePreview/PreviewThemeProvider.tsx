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

import {useDesign, type Theme} from '@thunderid/design';
import {AcrylicOrangeTheme, ThemeProvider} from '@wso2/oxygen-ui';
import type {JSX, ReactNode} from 'react';
import ColorSchemeSync from './ColorSchemeSync';

/**
 * Wraps children in a ThemeProvider scoped to the preview iframe.
 *
 * Key: `colorSchemeNode` must point to the iframe's `<html>` element so MUI
 * sets `data-color-scheme` inside the iframe (not on the parent document).
 * Without this, the CSS-vars selectors like `[data-color-scheme="dark"]`
 * never match and the theme doesn't switch.
 */
export default function PreviewThemeProvider({
  colorScheme,
  colorSchemeNode = undefined,
  baseTheme = undefined,
  children,
}: {
  colorScheme: 'light' | 'dark';
  colorSchemeNode?: HTMLElement | null;
  /** Base theme the resolved design is merged over. Defaults to Acrylic Orange. */
  baseTheme?: Theme;
  children: ReactNode;
}): JSX.Element {
  const effectiveBaseTheme = baseTheme ?? (AcrylicOrangeTheme as Theme);
  const {theme} = useDesign(effectiveBaseTheme);

  // MUI's ThemeProvider supports CSS-vars-specific props (colorSchemeNode,
  // disableNestedContext, storageManager) at runtime, but the TypeScript types
  // only expose them when the module-augmentation `CssThemeVariables` is set to
  // `{ enabled: true }`.  We need these props to isolate the preview iframe's
  // color-scheme attribute and prevent localStorage conflicts.
  const cssVarsProps = {
    storageManager: null,
    disableNestedContext: true,
    ...(colorSchemeNode ? {colorSchemeNode} : {}),
  } as Record<string, unknown>;

  return (
    <ThemeProvider theme={theme ?? effectiveBaseTheme} defaultMode={colorScheme} {...cssVarsProps}>
      <ColorSchemeSync mode={colorScheme} />
      {children}
    </ThemeProvider>
  );
}
