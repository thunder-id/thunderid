// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import appThemeFromDesign from '../appThemeFromDesign';

describe('appThemeFromDesign', () => {
  it('keeps the app its own look when there is no Gate theme', () => {
    expect(appThemeFromDesign(undefined)).toBeUndefined();
    expect(appThemeFromDesign({})).toBeUndefined();
  });

  it('reads the colors of each color scheme, the corners and the font from the Gate theme', () => {
    const theme = appThemeFromDesign({
      colorSchemes: {
        dark: {palette: {primary: {main: '#7dd3fc'}}},
        light: {palette: {primary: {main: '#0f766e'}}},
      },
      shape: {borderRadius: 4},
      typography: {fontFamily: 'Georgia, serif'},
    });

    expect(theme?.colorSchemes?.light?.primary).toBe('#0f766e');
    expect(theme?.colorSchemes?.dark?.primary).toBe('#7dd3fc');
    expect(theme?.borderRadius).toBe(4);
    expect(theme?.fontFamily).toBe('Georgia, serif');
  });
});
