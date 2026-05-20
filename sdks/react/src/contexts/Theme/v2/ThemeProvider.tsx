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

import {createTheme, Theme, ThemeConfig, RecursivePartial, FlowMetaTheme} from '@thunderid/browser';
import {FC, PropsWithChildren, ReactElement, useCallback, useContext, useEffect, useMemo, useState} from 'react';
import applyThemeToDOM from '../../../utils/applyThemeToDOM';
import normalizeThemeConfig from '../../../utils/normalizeThemeConfig';
import buildThemeConfigFromFlowMeta from '../../../utils/v2/buildThemeConfigFromFlowMeta';
import FlowMetaContext, {FlowMetaContextValue} from '../../FlowMeta/FlowMetaContext';
import ThemeContext from '../ThemeContext';

export interface ThemeProviderProps {
  /**
   * Optional theme overrides merged on top of the server-side flow meta theme.
   * User-supplied values take highest precedence.
   */
  theme?: RecursivePartial<ThemeConfig>;
}

/**
 * ThemeProvider is the v2 drop-in replacement for `ThemeProvider`.
 *
 * It reads the design theme from the nearest `FlowMetaContext` (provided by
 * `FlowMetaProvider`) and publishes a resolved `Theme` object through the
 * **same** `ThemeContext` that `useTheme` consumes.  This means all existing
 * components that call `useTheme` continue to work without any changes.
 *
 * The `defaultColorScheme` field returned by the server is used to seed the
 * active color scheme; the user can still toggle it locally via the
 * `toggleTheme` value exposed in the context.
 *
 * @example
 * ```tsx
 * <FlowMetaProvider config={{ baseUrl, type: FlowMetaType.App, id: appId }}>
 *   <ThemeProvider>
 *     <App />   {/* useTheme() works here as usual *\/}
 *   </ThemeProvider>
 * </FlowMetaProvider>
 * ```
 *
 * @example
 * With user theme overrides (user values win over server values):
 * ```tsx
 * <ThemeProvider theme={{ colors: { primary: { main: '#ff0000' } } }}>
 *   <App />
 * </ThemeProvider>
 * ```
 */
const ThemeProvider: FC<PropsWithChildren<ThemeProviderProps>> = ({
  children,
  theme: themeOverrideProp,
}: PropsWithChildren<ThemeProviderProps>): ReactElement => {
  const themeOverride: RecursivePartial<ThemeConfig> | undefined = normalizeThemeConfig(themeOverrideProp);
  const flowMetaContext: FlowMetaContextValue | null = useContext(FlowMetaContext);

  const flowMetaTheme: FlowMetaTheme | null = flowMetaContext?.meta?.design?.theme ?? null;
  const isLoading: boolean = flowMetaContext?.isLoading ?? false;
  const error: Error | null = flowMetaContext?.error ?? null;

  // Seed the color scheme from the server's defaultColorScheme; allow local toggling.
  const [colorScheme, setColorScheme] = useState<'light' | 'dark'>(() => flowMetaTheme?.defaultColorScheme ?? 'light');

  // When meta finishes loading, sync the color scheme with the server default.
  useEffect(() => {
    if (flowMetaTheme?.defaultColorScheme) {
      setColorScheme(flowMetaTheme.defaultColorScheme);
    }
  }, [flowMetaTheme?.defaultColorScheme]);

  const toggleTheme: () => void = useCallback(() => {
    setColorScheme((prev: 'light' | 'dark') => (prev === 'light' ? 'dark' : 'light'));
  }, []);

  // Build the resolved ThemeConfig: flow meta base → user overrides on top.
  const finalThemeConfig: RecursivePartial<ThemeConfig> | undefined = useMemo(() => {
    if (!flowMetaTheme) {
      return themeOverride;
    }

    const metaConfig: RecursivePartial<ThemeConfig> = buildThemeConfigFromFlowMeta(flowMetaTheme, colorScheme);

    if (!themeOverride) {
      return metaConfig;
    }

    return {
      ...metaConfig,
      ...themeOverride,
      borderRadius: {
        ...metaConfig.borderRadius,
        ...themeOverride.borderRadius,
      },
      colors: {
        ...metaConfig.colors,
        ...themeOverride.colors,
      },
      ...(metaConfig.typography || themeOverride.typography
        ? {
            typography: {
              ...(metaConfig as any).typography,
              ...themeOverride.typography,
            },
          }
        : {}),
    };
  }, [flowMetaTheme, colorScheme, themeOverride]);

  const theme: Theme = useMemo(
    () => createTheme(finalThemeConfig, colorScheme === 'dark'),
    [finalThemeConfig, colorScheme],
  );

  const direction: 'ltr' | 'rtl' = flowMetaTheme?.direction ?? 'ltr';

  // Apply CSS variables to the document root.
  useEffect(() => {
    applyThemeToDOM(theme);
  }, [theme]);

  // Apply text direction to the document root.
  useEffect(() => {
    if (typeof document !== 'undefined') {
      document.documentElement.dir = direction;
    }
  }, [direction]);

  const value: any = {
    brandingError: error,
    colorScheme,
    direction,
    inheritFromBranding: false,
    isBrandingLoading: isLoading,
    theme,
    toggleTheme,
  };

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
};

export default ThemeProvider;
