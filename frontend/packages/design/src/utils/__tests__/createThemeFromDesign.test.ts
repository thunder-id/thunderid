// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import createThemeFromDesign from '../createThemeFromDesign';

describe('createThemeFromDesign', () => {
  it('returns nothing when there is no Design theme', () => {
    expect(createThemeFromDesign(undefined)).toBeUndefined();
    expect(createThemeFromDesign({})).toBeUndefined();
  });

  it('builds a theme with the Design palette', () => {
    const theme = createThemeFromDesign({
      colorSchemes: {light: {palette: {primary: {main: '#0f766e'}}}},
      defaultColorScheme: 'light',
    }) as unknown as {colorSchemes: {light: {palette: {primary: {main: string}}}}};

    expect(theme.colorSchemes.light.palette.primary.main).toBe('#0f766e');
  });

  it('drops a system default color scheme, which the theme builder does not accept', () => {
    expect(() =>
      createThemeFromDesign({
        colorSchemes: {light: {palette: {primary: {main: '#0f766e'}}}},
        defaultColorScheme: 'system',
      }),
    ).not.toThrow();
  });
});
