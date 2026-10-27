// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {APP_THEME_COLOR, createAppSdkThemes, DEFAULT_APP_THEME, resolveAppTheme} from '../appTheme';

/** The parts of an SDK theme these tests read. */
interface SdkThemeShape {
  borderRadius?: Record<string, string>;
  colors?: {primary?: {contrastText?: string; main?: string}};
  typography?: {fontFamily?: string};
}

describe('appTheme', () => {
  it("fills what an app theme leaves out with the app's own look", () => {
    const resolved = resolveAppTheme({colorSchemes: {dark: {primary: '#2dd4bf'}}});

    expect(resolved.colorSchemes.dark.primary).toBe('#2dd4bf');
    expect(resolved.colorSchemes.dark.surface).toBe(DEFAULT_APP_THEME.colorSchemes.dark.surface);
    expect(resolved.colorSchemes.light).toEqual(DEFAULT_APP_THEME.colorSchemes.light);
    expect(resolved.borderRadius).toBe(DEFAULT_APP_THEME.borderRadius);
  });

  it('keeps square corners', () => {
    expect(resolveAppTheme({borderRadius: 0}).borderRadius).toBe(0);
  });

  it('gives the SDK components the colors, corners and font of each color scheme', () => {
    const sdk = createAppSdkThemes({
      borderRadius: 12,
      colorSchemes: {light: {primary: '#ffeb3b'}},
      fontFamily: 'Georgia, serif',
    }) as Record<'dark' | 'light', SdkThemeShape>;

    expect(sdk.light.colors?.primary?.main).toBe('#ffeb3b');
    // A light primary gets dark text on it.
    expect(sdk.light.colors?.primary?.contrastText).toBe('#111111');
    expect(sdk.dark.colors?.primary?.main).toBe(DEFAULT_APP_THEME.colorSchemes.dark.primary);
    expect(sdk.light.borderRadius).toEqual({large: '24px', medium: '12px', small: '9px'});
    expect(sdk.dark.typography?.fontFamily).toBe('Georgia, serif');
  });

  it('accepts the colors the server accepts, and nothing else', () => {
    ['#36f', '#3688ff', '#3688ffcc', 'rgb(54, 136, 255)', 'rgba(15, 23, 42, 0.08)', 'rgb(232 234 237 / 68%)'].forEach(
      (color: string) => expect(APP_THEME_COLOR.test(color)).toBe(true),
    );
    ['teal', 'red; color: blue', 'url(x)', '#12345'].forEach((color: string) =>
      expect(APP_THEME_COLOR.test(color)).toBe(false),
    );
  });
});
