// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useColorScheme, useTheme} from '@wso2/oxygen-ui';

/**
 * What a full-screen view needs to look like the rest of the console, in the color scheme the console is in.
 */
export interface ConsoleSurface {
  /**
   * Styles for the background of a full-screen dialog: the console's own page background, the one its theme paints on
   * the body, so the dialog covers the page without turning flatter or darker than it.
   */
  pageSx: Record<string, unknown>;
}

const PAGE_STYLES = ['backgroundAttachment', 'backgroundBlendMode', 'backgroundImage'] as const;

/**
 * Reads what the page paints behind the app, from the element that paints it, so a full-screen view covering the page
 * can paint the same without knowing how the theme does it.
 *
 * @returns The styles of the page background that the theme sets beyond its background color, or none.
 */
const readPageBackground = (): Record<string, string> => {
  const painter = [document.body, document.documentElement].find(
    (element: HTMLElement) => getComputedStyle(element).backgroundImage !== 'none',
  );

  if (!painter) {
    return {};
  }

  const computed = getComputedStyle(painter);

  return Object.fromEntries(PAGE_STYLES.map((key: (typeof PAGE_STYLES)[number]) => [key, computed[key]]));
};

/**
 * Reads the page the console paints, so a full-screen view looks like it.
 *
 * @param open - Whether the full-screen view is open. The page is read as it opens, when it is painted as it is shown.
 * @returns The page background styles.
 */
export default function useConsoleSurface(open: boolean): ConsoleSurface {
  // The page repaints when the theme or its color scheme changes, so the view is drawn again then and reads the page
  // as it is now.
  useTheme();
  useColorScheme();

  return {pageSx: {backgroundColor: 'background.default', ...(open ? readPageBackground() : {})}};
}
