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

import {useGetTheme, type Theme} from '@thunderid/design';
import {useState, useMemo, useCallback, type PropsWithChildren} from 'react';
import {useParams} from 'react-router';
import ThemeBuilderContext, {type ThemeBuilderContextType} from './ThemeBuilderContext';
import type {ThemeSection, Viewport} from '../../models/theme-builder';

/**
 * Props for the {@link ThemeBuilderProvider} component.
 *
 * @public
 */
export type ThemeBuilderProviderProps = PropsWithChildren;

/**
 * React context provider component that provides theme builder state
 * to all child components.
 *
 * This component manages all the state needed for editing and previewing a theme.
 * It automatically fetches the theme data and maintains a draft copy for live edits.
 *
 * @param props - The component props
 * @param props.themeId - The ID of the theme to edit
 * @param props.children - React children to be wrapped with the theme builder context
 *
 * @returns JSX element that provides theme builder context to children
 *
 * @example
 * ```tsx
 * import ThemeBuilderProvider from './ThemeBuilderProvider';
 * import ThemeBuilderPage from './ThemeBuilderPage';
 *
 * function App() {
 *   return (
 *     <ThemeBuilderProvider themeId="theme-123">
 *       <ThemeBuilderPage />
 *     </ThemeBuilderProvider>
 *   );
 * }
 * ```
 *
 * @public
 */
export default function ThemeBuilderProvider({children}: ThemeBuilderProviderProps) {
  const {themeId = ''} = useParams<{themeId: string}>();
  const {data: themeData, isLoading} = useGetTheme(themeId);

  const [draftTheme, setDraftTheme] = useState<Theme | null>(() => themeData?.theme ?? null);
  const [isDirty, setIsDirty] = useState<boolean>(false);
  const [activeSection, setActiveSection] = useState<ThemeSection>('colors');
  const [previewColorScheme, setPreviewColorScheme] = useState<'light' | 'dark' | 'system'>(() =>
    themeData?.theme?.defaultColorScheme === 'dark' ? 'dark' : 'light',
  );
  const [viewport, setViewport] = useState<Viewport>('desktop');
  const [isSaving, setIsSaving] = useState<boolean>(false);

  const handle = themeData?.handle ?? null;
  const displayName = themeData?.displayName ?? null;
  const isReadOnly = themeData?.isReadOnly ?? false;

  const [prevThemeData, setPrevThemeData] = useState(themeData);

  // When theme data loads, sync previewColorScheme and initialize draft
  if (prevThemeData !== themeData) {
    setPrevThemeData(themeData);
    if (themeData?.theme?.defaultColorScheme === 'dark') {
      setPreviewColorScheme('dark');
    }
    if (themeData?.theme && !draftTheme) {
      setDraftTheme(themeData.theme);
    }
  }

  /**
   * Resets the draft to match the original theme
   */
  const resetDraft = useCallback(() => {
    if (themeData?.theme) {
      setDraftTheme(themeData.theme);
      setIsDirty(false);
    }
  }, [themeData]);

  /**
   * Updates a specific path in the draft theme using dot notation
   * @param path - Array of keys representing the path to update
   * @param value - The new value to set
   */
  const updateDraftTheme = useCallback((path: string[], value: unknown) => {
    setDraftTheme((prev: Theme | null) => {
      if (!prev) return prev;

      const newTheme = JSON.parse(JSON.stringify(prev)) as Theme;
      let current: Record<string, unknown> = newTheme as unknown as Record<string, unknown>;

      // Navigate to the parent of the target property
      for (let i = 0; i < path.length - 1; i += 1) {
        const key = path[i];
        if (!(key in current)) {
          current[key] = {};
        }
        current = current[key] as Record<string, unknown>;
      }

      // Set the value
      const lastKey = path[path.length - 1];
      current[lastKey] = value;

      return newTheme;
    });
    setIsDirty(true);
  }, []);

  const contextValue: ThemeBuilderContextType = useMemo(
    () => ({
      themeId,
      handle,
      originalTheme: themeData?.theme ?? null,
      displayName,
      isReadOnly,
      draftTheme,
      setDraftTheme,
      isDirty,
      setIsDirty,
      activeSection,
      setActiveSection,
      previewColorScheme,
      setPreviewColorScheme,
      viewport,
      setViewport,
      isSaving,
      setIsSaving,
      resetDraft,
      updateDraftTheme,
    }),
    [
      themeId,
      handle,
      themeData?.theme,
      displayName,
      isReadOnly,
      draftTheme,
      isDirty,
      activeSection,
      viewport,
      previewColorScheme,
      isSaving,
      resetDraft,
      updateDraftTheme,
    ],
  );

  if (isLoading) {
    return null; // or a loading spinner
  }

  return <ThemeBuilderContext.Provider value={contextValue}>{children}</ThemeBuilderContext.Provider>;
}
