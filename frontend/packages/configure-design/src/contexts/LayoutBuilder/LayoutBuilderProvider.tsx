// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice} from '@thunderid/components';
import {useGetLayout} from '@thunderid/design';
import {useState, useMemo, useCallback, type PropsWithChildren} from 'react';
import {useTranslation} from 'react-i18next';
import {useParams} from 'react-router';
import LayoutBuilderContext, {type LayoutBuilderContextType, type LayoutConfig} from './LayoutBuilderContext';

/**
 * Props for the {@link LayoutBuilderProvider} component.
 *
 * @public
 */
export type LayoutBuilderProviderProps = PropsWithChildren;

/**
 * React context provider component that provides layout builder state
 * to all child components.
 *
 * This component manages all the state needed for editing and previewing a layout.
 * It automatically fetches the layout data and maintains a draft copy for live edits.
 *
 * @param props - The component props
 * @param props.layoutId - The ID of the layout to edit
 * @param props.children - React children to be wrapped with the layout builder context
 *
 * @returns JSX element that provides layout builder context to children
 *
 * @example
 * ```tsx
 * import LayoutBuilderProvider from './LayoutBuilderProvider';
 * import LayoutBuilderPage from './LayoutBuilderPage';
 *
 * function App() {
 *   return (
 *     <LayoutBuilderProvider layoutId="layout-123">
 *       <LayoutBuilderPage />
 *     </LayoutBuilderProvider>
 *   );
 * }
 * ```
 *
 * @public
 */
export default function LayoutBuilderProvider({children}: LayoutBuilderProviderProps) {
  const {t} = useTranslation('design');
  const {layoutId = ''} = useParams<{layoutId: string}>();
  const {data: layoutData, isLoading, error, refetch} = useGetLayout(layoutId);

  const [draftLayout, setDraftLayout] = useState<LayoutConfig | null>(
    () => (layoutData?.layout as LayoutConfig) ?? null,
  );
  const [isDirty, setIsDirty] = useState<boolean>(false);
  const [selectedScreen, setSelectedScreen] = useState<string | null>(null);
  const [screenDraft, setScreenDraft] = useState<Record<string, unknown> | null>(null);
  const [extraScreens, setExtraScreens] = useState<Record<string, Record<string, unknown>>>({});
  const [isSaving, setIsSaving] = useState<boolean>(false);

  const handle = layoutData?.handle ?? null;
  const displayName = layoutData?.displayName ?? null;
  const projectId = layoutData?.projectId ?? null;

  // Initialize draft when layout data loads (update during render to avoid effect-driven cascades)
  const [prevLayoutData, setPrevLayoutData] = useState(layoutData);
  if (prevLayoutData !== layoutData) {
    setPrevLayoutData(layoutData);
    if (layoutData?.layout && !draftLayout) {
      setDraftLayout(layoutData.layout as LayoutConfig);
    }
  }

  // Derive effective selected screen — falls back to first screen when nothing is explicitly selected
  const effectiveSelectedScreen = useMemo(() => {
    if (selectedScreen) return selectedScreen;
    if (draftLayout?.screens) {
      const screenNames = Object.keys(draftLayout.screens);
      return screenNames.length > 0 ? screenNames[0] : null;
    }
    return null;
  }, [selectedScreen, draftLayout]);

  /**
   * Resets the draft to match the original layout
   */
  const resetDraft = useCallback(() => {
    if (layoutData?.layout) {
      setDraftLayout(layoutData.layout as LayoutConfig);
      setExtraScreens({});
      setScreenDraft(null);
      setIsDirty(false);
    }
  }, [layoutData]);

  /**
   * Adds a new screen to the layout
   */
  const addScreen = useCallback((name: string, extendsBase: string) => {
    const newScreen: Record<string, unknown> = {extends: extendsBase};
    setExtraScreens((prev) => ({...prev, [name]: newScreen}));
    setSelectedScreen(name);
    setIsDirty(true);
  }, []);

  /**
   * Updates a specific path in the draft layout using dot notation
   * @param path - Array of keys representing the path to update
   * @param value - The new value to set
   */
  const updateDraftLayout = useCallback((path: string[], value: unknown) => {
    setDraftLayout((prev) => {
      if (!prev) return prev;

      const newLayout = JSON.parse(JSON.stringify(prev)) as LayoutConfig;
      let current: Record<string, unknown> = newLayout as unknown as Record<string, unknown>;

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

      return newLayout;
    });
    setIsDirty(true);
  }, []);

  /**
   * Gets all screens (original + extra)
   */
  const getAllScreens = useCallback(() => {
    const serverScreens = draftLayout?.screens ?? {};
    return {...serverScreens, ...extraScreens};
  }, [draftLayout, extraScreens]);

  /**
   * Gets base screen names (screens without extends property)
   */
  const getBaseScreenNames = useCallback(() => {
    const allScreens = getAllScreens();
    return Object.keys(allScreens).filter((name) => !allScreens[name]?.['extends']);
  }, [getAllScreens]);

  const contextValue: LayoutBuilderContextType = useMemo(
    () => ({
      layoutId,
      handle,
      projectId,
      originalLayout: layoutData?.layout as LayoutConfig | null,
      displayName,
      draftLayout,
      setDraftLayout,
      isDirty,
      setIsDirty,
      selectedScreen: effectiveSelectedScreen,
      setSelectedScreen,
      screenDraft,
      setScreenDraft,
      extraScreens,
      setExtraScreens,
      isSaving,
      setIsSaving,
      resetDraft,
      addScreen,
      updateDraftLayout,
      getAllScreens,
      getBaseScreenNames,
    }),
    [
      layoutId,
      handle,
      projectId,
      layoutData?.layout,
      displayName,
      draftLayout,
      isDirty,
      effectiveSelectedScreen,
      screenDraft,
      extraScreens,
      isSaving,
      resetDraft,
      addScreen,
      updateDraftLayout,
      getAllScreens,
      getBaseScreenNames,
    ],
  );

  if (error) {
    return (
      <QueryErrorNotice
        error={error}
        t={t}
        variant="block"
        title={t('layouts.builder.errors.load.title', 'Failed to load layout')}
        onRetry={() => void refetch()}
      />
    );
  }

  if (isLoading) {
    return null; // or a loading spinner
  }

  return <LayoutBuilderContext.Provider value={contextValue}>{children}</LayoutBuilderContext.Provider>;
}
