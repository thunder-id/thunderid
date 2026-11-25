// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useContext, useMemo} from 'react';
import DesignContext, {DesignContextType} from './DesignContext';
import type {Theme} from '../../models/theme';
import createThemeFromDesign from '../../utils/createThemeFromDesign';

/**
 * React hook for accessing design configuration throughout the application.
 *
 * This hook provides access to the design data loaded from the server, resolved theme,
 * and layout configuration. It must be used within a component tree wrapped by
 * `DesignProvider`, otherwise it will throw an error.
 *
 * @param baseTheme - Optional base theme to extend from. If provided, overrides the provider's baseTheme
 * @returns The design context containing design data and resolved theme/layout
 *
 * @throws {Error} Throws an error if used outside of DesignProvider
 *
 * @public
 */
export default function useDesign(baseTheme?: Theme): DesignContextType {
  const context = useContext(DesignContext);

  if (context === undefined) {
    throw new Error('useDesign must be used within a DesignProvider');
  }

  // If a baseTheme is provided, override the transformedTheme
  const transformedTheme = useMemo(
    () => (baseTheme ? (createThemeFromDesign(context.design?.theme) ?? baseTheme) : baseTheme),
    [baseTheme, context.design?.theme],
  );

  return useMemo(
    () => ({
      ...context,
      theme: transformedTheme,
    }),
    [context, transformedTheme],
  );
}
