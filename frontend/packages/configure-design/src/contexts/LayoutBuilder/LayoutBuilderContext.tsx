// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {createContext, type Context} from 'react';

/**
 * Layout configuration type
 */
export interface LayoutConfig {
  screens?: Record<string, Record<string, unknown>>;
  [key: string]: unknown;
}

/**
 * Layout builder context state interface
 *
 * Provides centralized state management for the layout builder page.
 * This interface defines all the state needed for editing and previewing layouts.
 *
 * @public
 */
export interface LayoutBuilderContextType {
  /**
   * The ID of the layout being edited
   */
  layoutId: string | null;

  /**
   * The handle (unique kebab-case identifier) of the layout
   */
  handle: string | null;

  /**
   * The project the layout belongs to
   */
  projectId: string | null;

  /**
   * The original layout configuration from the API
   */
  originalLayout: LayoutConfig | null;

  /**
   * The display name of the layout
   */
  displayName: string | null;

  /**
   * The draft layout configuration (live changes not yet saved)
   */
  draftLayout: LayoutConfig | null;

  /**
   * Sets the draft layout configuration
   */
  setDraftLayout: (layout: LayoutConfig | null) => void;

  /**
   * Whether there are unsaved changes
   */
  isDirty: boolean;

  /**
   * Sets the dirty state
   */
  setIsDirty: (dirty: boolean) => void;

  /**
   * The currently selected screen in the builder
   */
  selectedScreen: string | null;

  /**
   * Sets the selected screen
   */
  setSelectedScreen: (screen: string | null) => void;

  /**
   * Draft changes for the selected screen (before committing to draftLayout)
   */
  screenDraft: Record<string, unknown> | null;

  /**
   * Sets the screen draft
   */
  setScreenDraft: (draft: Record<string, unknown> | null) => void;

  /**
   * Additional screens added during this session (not yet saved)
   */
  extraScreens: Record<string, Record<string, unknown>>;

  /**
   * Sets extra screens
   */
  setExtraScreens: (screens: Record<string, Record<string, unknown>>) => void;

  /**
   * Whether the layout is currently being saved
   */
  isSaving: boolean;

  /**
   * Sets the saving state
   */
  setIsSaving: (saving: boolean) => void;

  /**
   * Resets the draft to match the original layout
   */
  resetDraft: () => void;

  /**
   * Adds a new screen to the layout
   */
  addScreen: (name: string, extendsBase: string) => void;

  /**
   * Updates a specific path in the draft layout
   */
  updateDraftLayout: (path: string[], value: unknown) => void;

  /**
   * Gets all screens (original + extra)
   */
  getAllScreens: () => Record<string, Record<string, unknown>>;

  /**
   * Gets base screen names (screens without extends property)
   */
  getBaseScreenNames: () => string[];
}

/**
 * React context for layout builder state management
 *
 * @public
 */
const LayoutBuilderContext: Context<LayoutBuilderContextType | undefined> = createContext<
  LayoutBuilderContextType | undefined
>(undefined);

export default LayoutBuilderContext;
