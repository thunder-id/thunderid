// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Theme} from '@thunderid/design';

/**
 * Applies border width and style from theme.border to the theme's shape property
 * and component overrides. This mirrors how borderRadius is stored and used.
 *
 * The border settings get propagated to MUI components through the shape property
 * and explicit component overrides where needed.
 */
export function applyBorderStylesToTheme(theme: Theme | undefined): Theme | undefined {
  if (!theme) {
    return theme;
  }

  const themeRecord = theme as unknown as Record<string, unknown>;
  const borderObj = themeRecord['border'] as Record<string, unknown> | undefined;

  if (!borderObj) {
    return theme;
  }

  const borderWidth = borderObj['width'] as string | undefined;
  const borderStyle = borderObj['style'] as string | undefined;

  if (!borderWidth && !borderStyle) {
    return theme;
  }

  // Store border settings in shape property, alongside borderRadius
  // This keeps all shape-related properties together
  themeRecord['shape'] ??= {};
  const shape = themeRecord['shape'] as Record<string, unknown>;
  if (borderWidth) {
    shape['borderWidth'] = borderWidth;
  }
  if (borderStyle) {
    shape['borderStyle'] = borderStyle;
  }

  const borderStr = `${borderWidth ?? '1px'} ${borderStyle ?? 'solid'}`;

  // Ensure components object exists
  themeRecord['components'] ??= {};

  const components = themeRecord['components'] as Record<string, unknown>;

  // Helper to apply border to overrides
  const applyBorderToOverride = (override: unknown) => {
    if (typeof override === 'function') {
      return (props: unknown) => {
        const result = (override as (props: unknown) => Record<string, unknown>)(props);
        return {...result, border: borderStr};
      };
    }
    if (override && typeof override === 'object') {
      return {...(override as Record<string, unknown>), border: borderStr};
    }
    return {border: borderStr};
  };

  // Apply to MuiButton outlined variants
  components['MuiButton'] ??= {styleOverrides: {}};

  const muiButton = components['MuiButton'] as Record<string, unknown>;
  muiButton['styleOverrides'] ??= {};

  const buttonOverrides = muiButton['styleOverrides'] as Record<string, unknown>;

  const outlinedOverride = buttonOverrides['outlined'];
  buttonOverrides['outlined'] = outlinedOverride
    ? applyBorderToOverride(outlinedOverride)
    : {borderWidth: borderWidth ?? '1px', borderStyle: borderStyle ?? 'solid'};

  const outlinedPrimaryOverride = buttonOverrides['outlinedPrimary'];
  buttonOverrides['outlinedPrimary'] = outlinedPrimaryOverride
    ? applyBorderToOverride(outlinedPrimaryOverride)
    : {borderWidth: borderWidth ?? '1px', borderStyle: borderStyle ?? 'solid'};

  const outlinedSecondaryOverride = buttonOverrides['outlinedSecondary'];
  buttonOverrides['outlinedSecondary'] = outlinedSecondaryOverride
    ? applyBorderToOverride(outlinedSecondaryOverride)
    : {borderWidth: borderWidth ?? '1px', borderStyle: borderStyle ?? 'solid'};

  // Apply to MuiOutlinedInput notchedOutline
  components['MuiOutlinedInput'] ??= {styleOverrides: {}};
  const muiInput = components['MuiOutlinedInput'] as Record<string, unknown>;
  muiInput['styleOverrides'] ??= {};
  const inputOverrides = muiInput['styleOverrides'] as Record<string, unknown>;
  const notchedOverride = inputOverrides['notchedOutline'];

  if (typeof notchedOverride === 'function') {
    inputOverrides['notchedOutline'] = (props: unknown) => ({
      ...(notchedOverride as (props: unknown) => Record<string, unknown>)(props),
      borderWidth: borderWidth ?? '1px',
      borderStyle: borderStyle ?? 'solid',
    });
  } else {
    inputOverrides['notchedOutline'] = {
      ...(notchedOverride as Record<string, unknown> | undefined),
      borderWidth: borderWidth ?? '1px',
      borderStyle: borderStyle ?? 'solid',
    };
  }

  // Apply to MuiChip outlined
  components['MuiChip'] ??= {styleOverrides: {}};
  const muiChip = components['MuiChip'] as Record<string, unknown>;
  muiChip['styleOverrides'] ??= {};
  const chipOverrides = muiChip['styleOverrides'] as Record<string, unknown>;
  const chipOutlined = chipOverrides['outlined'];

  if (typeof chipOutlined === 'function') {
    chipOverrides['outlined'] = (props: unknown) => ({
      ...(chipOutlined as (props: unknown) => Record<string, unknown>)(props),
      borderWidth: borderWidth ?? '1px',
      borderStyle: borderStyle ?? 'solid',
    });
  } else {
    chipOverrides['outlined'] = {
      ...(chipOutlined as Record<string, unknown> | undefined),
      borderWidth: borderWidth ?? '1px',
      borderStyle: borderStyle ?? 'solid',
    };
  }

  return theme;
}
