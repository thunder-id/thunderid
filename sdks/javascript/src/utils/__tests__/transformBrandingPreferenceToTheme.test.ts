/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import type {BrandingPreference, ThemeVariant} from '../../models/branding-preference';

import createTheme from '../../theme/createTheme';
import {transformBrandingPreferenceToTheme} from '../transformBrandingPreferenceToTheme';

vi.mock('../../theme/createTheme', () => ({
  default: vi.fn((config: any, isDark: boolean) => ({__config: config, __isDark: isDark})),
}));

const lightVariant = (overrides?: Partial<ThemeVariant>): ThemeVariant =>
  ({
    buttons: undefined,
    colors: {
      background: {
        body: {main: '#fbfbfb'},
        surface: {main: '#ffffff'},
      },
      primary: {contrastText: '#fff', main: '#FF7300'},
      secondary: {contrastText: '#000', main: '#E0E1E2'},
      text: {primary: '#000000de', secondary: '#00000066'},
    },
    images: undefined,
    inputs: undefined,
    ...(overrides || {}),
  }) as any;

const darkVariant = (overrides?: Partial<ThemeVariant>): ThemeVariant =>
  ({
    colors: {
      background: {
        body: {main: '#17191a'},
        surface: {main: '#242627'},
      },
      primary: {contrastText: '#fff', dark: '#222222', main: '#111111'},
      text: {primary: '#EBEBEF', secondary: '#B9B9C6'},
    },
    ...(overrides || {}),
  }) as any;

const basePref = (pref: Partial<BrandingPreference['preference']>): BrandingPreference => ({
  locale: 'en-US',
  name: 'dxlab',
  preference: pref as any,
  type: 'ORG',
});

describe('transformBrandingPreferenceToTheme', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should return default light theme when theme config is missing', () => {
    const bp: BrandingPreference = basePref({} as any);

    const out: Record<string, unknown> = transformBrandingPreferenceToTheme(bp);

    expect(createTheme).toHaveBeenCalledWith({}, false);

    expect(out).toEqual({__config: {}, __isDark: false});
  });

  it('should use activeTheme from branding preference when forceTheme is not provided', () => {
    const bp: BrandingPreference = basePref({
      theme: {
        DARK: darkVariant(),
        LIGHT: lightVariant(),
        activeTheme: 'LIGHT',
      },
    });

    const out: Record<string, unknown> = transformBrandingPreferenceToTheme(bp);

    expect((createTheme as any).mock.calls[0][1]).toBe(false);

    const cfg: Record<string, any> = (out as any).__config;
    expect(cfg.colors.primary.main).toBe('#FF7300');
    expect(cfg.colors.secondary.main).toBe('#E0E1E2');
    expect(cfg.colors.background.surface).toBe('#ffffff');
    expect(cfg.colors.background.body.main).toBe('#fbfbfb');
  });

  it('should respect forceTheme=dark and passes isDark=true', () => {
    const bp: BrandingPreference = basePref({
      theme: {
        DARK: darkVariant(),
        LIGHT: lightVariant(),
        activeTheme: 'LIGHT',
      },
    });

    const out: Record<string, unknown> = transformBrandingPreferenceToTheme(bp, 'dark');

    expect((createTheme as any).mock.calls[0][1]).toBe(true);

    const cfg: Record<string, any> = (out as any).__config;

    expect(cfg.colors.primary.main).toBe('#222222');
    expect(cfg.colors.background.surface).toBe('#242627');
  });

  it('should fall back to LIGHT config when requested variant missing, but preserves isDark from activeTheme', () => {
    const bp: BrandingPreference = basePref({
      theme: {
        LIGHT: lightVariant(),
        activeTheme: 'DARK',
      } as any,
    });

    const out: Record<string, unknown> = transformBrandingPreferenceToTheme(bp);

    expect((createTheme as any).mock.calls[0][1]).toBe(true);

    const cfg: Record<string, any> = (out as any).__config;

    expect(cfg.colors.primary.main).toBe('#FF7300');
  });

  it('should return default light theme when no variants exist', () => {
    const bp: BrandingPreference = basePref({
      theme: {} as any,
    });

    const out: Record<string, unknown> = transformBrandingPreferenceToTheme(bp);

    expect(createTheme).toHaveBeenCalledWith({}, false);

    expect(out).toEqual({__config: {}, __isDark: false});
  });

  it('should map images (logo & favicon) into config correctly', () => {
    const bp: BrandingPreference = basePref({
      theme: {
        LIGHT: lightVariant({
          images: {
            favicon: {
              altText: 'App Icon',
              imgURL: 'https://example.com/favicon.ico',
              title: 'My App Favicon',
            },
            logo: {
              altText: 'Company Brand Logo',
              imgURL: 'https://example.com/logo.png',
              title: 'Company Logo',
            },
          },
        }),
        activeTheme: 'LIGHT',
      },
    });

    const out: Record<string, unknown> = transformBrandingPreferenceToTheme(bp);

    const cfg: Record<string, any> = (out as any).__config;

    expect(cfg.images.favicon).toEqual({
      alt: 'App Icon',
      title: 'My App Favicon',
      url: 'https://example.com/favicon.ico',
    });

    expect(cfg.images.logo).toEqual({
      alt: 'Company Brand Logo',
      title: 'Company Logo',
      url: 'https://example.com/logo.png',
    });
  });

  it('should apply component borderRadius overrides for Button and Field when present', () => {
    const bp: BrandingPreference = basePref({
      theme: {
        LIGHT: lightVariant({
          buttons: {
            primary: {
              base: {
                border: {borderRadius: 12},
              },
            },
          } as any,
          inputs: {
            base: {
              border: {borderRadius: 6},
            },
          } as any,
        }),
        activeTheme: 'LIGHT',
      },
    });

    const out: Record<string, unknown> = transformBrandingPreferenceToTheme(bp);

    const cfg: Record<string, any> = (out as any).__config;

    expect(cfg.components.Button.styleOverrides.root.borderRadius).toBe(12);
    expect(cfg.components.Field.styleOverrides.root.borderRadius).toBe(6);
  });

  it('should omit components section when no button/field borderRadius provided', () => {
    const bp: BrandingPreference = basePref({
      theme: {
        LIGHT: lightVariant(),
        activeTheme: 'LIGHT',
      },
    });

    const out: Record<string, unknown> = transformBrandingPreferenceToTheme(bp);

    const cfg: Record<string, any> = (out as any).__config;

    expect(cfg.components).toBeUndefined();
  });

  it('should resolve dark color selection correctly for primary when both main and dark are provided', () => {
    const bp: BrandingPreference = basePref({
      theme: {
        DARK: darkVariant({
          colors: {
            background: {
              body: {main: '#111'},
              surface: {main: '#222'},
            },
            primary: {contrastText: '#fff', dark: '#010101', main: '#999999'},
            text: {primary: '#eee', secondary: '#aaa'},
          } as any,
        }),
        activeTheme: 'DARK',
      },
    });

    const out: Record<string, unknown> = transformBrandingPreferenceToTheme(bp);

    const cfg: Record<string, any> = (out as any).__config;

    expect(cfg.colors.primary.main).toBe('#010101');
    expect(cfg.colors.primary.dark).toBe('#010101');
    expect((createTheme as any).mock.calls[0][1]).toBe(true);
  });

  it('should use contrastText if provided on color variants', () => {
    const bp: BrandingPreference = basePref({
      theme: {
        LIGHT: lightVariant({
          colors: {
            alerts: {
              error: {contrastText: '#fff', main: '#ff0000'},
              info: {contrastText: '#111', main: '#0000ff'},
              neutral: {contrastText: '#222', main: '#00ff00'},
              warning: {contrastText: '#333', main: '#ffff00'},
            } as any,
            primary: {contrastText: '#abcdef', main: '#123456'},
            secondary: {contrastText: '#fefefe', main: '#222222'},
          } as any,
        }),
        activeTheme: 'LIGHT',
      },
    });

    const out: Record<string, unknown> = transformBrandingPreferenceToTheme(bp);

    const cfg: Record<string, any> = (out as any).__config;

    expect(cfg.colors.primary.contrastText).toBe('#abcdef');
    expect(cfg.colors.secondary.contrastText).toBe('#fefefe');
    expect(cfg.colors.error.contrastText).toBe('#fff');
    expect(cfg.colors.info.contrastText).toBe('#111');
    expect(cfg.colors.success.contrastText).toBe('#222');
    expect(cfg.colors.warning.contrastText).toBe('#333');
  });
});
