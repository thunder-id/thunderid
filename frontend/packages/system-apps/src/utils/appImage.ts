// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AppColorScheme} from './appTheme';
import type {AppImage} from '../models/app-config';

/**
 * The URL of a branding image for a color scheme: its dark variant in the dark scheme when it has
 * one, else its light one. An empty URL counts as none.
 *
 * @param image - The branding image, if any.
 * @param scheme - The color scheme the app shows.
 * @returns The URL, or `undefined` when the image sets none.
 *
 * @public
 */
const resolveAppImage = (image: AppImage | undefined, scheme: AppColorScheme): string | undefined => {
  if (scheme === 'dark' && image?.dark) {
    return image.dark;
  }
  if (image?.light) {
    return image.light;
  }

  return undefined;
};

export default resolveAppImage;
