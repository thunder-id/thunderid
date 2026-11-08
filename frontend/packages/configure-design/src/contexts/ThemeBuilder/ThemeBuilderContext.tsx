// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Theme} from '@thunderid/design';
import {createContext, type Context} from 'react';
import type {ThemeSection, Viewport} from '../../models/theme-builder';

/**
 * Theme builder context state interface
 *
 * Provides centralized state management for the theme builder page.
 * This interface defines all the state needed for editing and previewing themes.
 *
 * @public
 */
export interface ThemeBuilderContextType {
  /**
   * The ID of the theme being edited
   */
  themeId: string | null;

  /**
   * The handle (unique kebab-case identifier) of the theme
   */
  handle: string | null;

  /**
   * The ID of the project the theme belongs to, or null when it belongs to none
   */
  projectId: string | null;

  /**
   * The original theme configuration from the API
   */
  originalTheme: Theme | null;

  /**
   * The display name of the theme
   */
  displayName: string | null;

  /**
   * The draft theme configuration (live changes not yet saved)
   */
  draftTheme: Theme | null;

  /**
   * Sets the draft theme configuration
   */
  setDraftTheme: (theme: Theme | null) => void;

  /**
   * Whether there are unsaved changes
   */
  isDirty: boolean;

  /**
   * Sets the dirty state
   */
  setIsDirty: (dirty: boolean) => void;

  /**
   * The currently active section in the builder
   */
  activeSection: ThemeSection;

  /**
   * Sets the active section
   */
  setActiveSection: (section: ThemeSection) => void;

  /**
   * The color scheme used for the live preview (independent of the draft's defaultColorScheme).
   * 'system' follows the OS/app color scheme.
   */
  previewColorScheme: 'light' | 'dark' | 'system';

  /**
   * Sets the preview color scheme to flip the preview between light, dark, or system
   */
  setPreviewColorScheme: (scheme: 'light' | 'dark' | 'system') => void;

  /**
   * The current viewport for preview
   */
  viewport: Viewport;

  /**
   * Sets the viewport
   */
  setViewport: (viewport: Viewport) => void;

  /**
   * Whether the theme is currently being saved
   */
  isSaving: boolean;

  /**
   * Sets the saving state
   */
  setIsSaving: (saving: boolean) => void;

  /**
   * Whether the theme is read-only (declarative) and cannot be deleted or modified
   */
  isReadOnly: boolean;

  /**
   * Resets the draft to match the original theme
   */
  resetDraft: () => void;

  /**
   * Updates a specific path in the draft theme
   */
  updateDraftTheme: (path: string[], value: unknown) => void;
}

/**
 * React context for theme builder state management
 *
 * @public
 */
const ThemeBuilderContext: Context<ThemeBuilderContextType | undefined> = createContext<
  ThemeBuilderContextType | undefined
>(undefined);

export default ThemeBuilderContext;
