// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {TypographyProps} from '@wso2/oxygen-ui';
import {TypographyVariants, TypographyColors} from '../models/elements';

/**
 * Maps flow typography variant names to Material UI typography variant names.
 */
export const VARIANT_TO_MUI_MAP: Record<string, TypographyProps['variant']> = {
  [TypographyVariants.H1]: 'h1',
  [TypographyVariants.H2]: 'h2',
  [TypographyVariants.H3]: 'h3',
  [TypographyVariants.H4]: 'h4',
  [TypographyVariants.H5]: 'h5',
  [TypographyVariants.H6]: 'h6',
  [TypographyVariants.Body1]: 'body1',
  [TypographyVariants.Body2]: 'body2',
};

/**
 * Maps flow typography color names to Material UI color values.
 */
export const COLOR_ENUM_TO_MUI_MAP: Record<string, TypographyProps['color']> = {
  [TypographyColors.Error]: 'error',
  [TypographyColors.Warning]: 'warning',
  [TypographyColors.Success]: 'success',
  [TypographyColors.Info]: 'info',
  [TypographyColors.Primary]: 'primary',
  [TypographyColors.Secondary]: 'secondary',
};
