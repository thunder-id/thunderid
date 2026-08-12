// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Theme} from '@thunderid/design';
import {describe, it, expect} from 'vitest';
import {applyBorderStylesToTheme} from '../applyBorderStylesToTheme';

function makeBaseTheme(overrides: Partial<Record<string, unknown>> = {}): Theme {
  return {
    shape: {borderRadius: 8},
    colorSchemes: {
      light: {palette: {primary: {main: '#3688FF'}}},
      dark: {palette: {primary: {main: '#3688FF'}}},
    },
    ...overrides,
  } as unknown as Theme;
}

describe('applyBorderStylesToTheme', () => {
  describe('handles undefined and null', () => {
    it('returns undefined when theme is undefined', () => {
      expect(applyBorderStylesToTheme(undefined)).toBeUndefined();
    });

    it('returns theme unchanged when border property is missing', () => {
      const theme = makeBaseTheme();
      const result = applyBorderStylesToTheme(theme);
      expect(result).toBe(theme);
    });

    it('returns theme unchanged when border object is empty', () => {
      const theme = makeBaseTheme({border: {}});
      const result = applyBorderStylesToTheme(theme);
      expect(result).toBe(theme);
    });
  });

  describe('stores border in shape property', () => {
    it('adds borderWidth to theme.shape', () => {
      const theme = makeBaseTheme({border: {width: '2px', style: 'solid'}});
      const result = applyBorderStylesToTheme(theme);
      expect((result?.shape as unknown as Record<string, unknown>)?.['borderWidth']).toBe('2px');
    });

    it('adds borderStyle to theme.shape', () => {
      const theme = makeBaseTheme({border: {width: '1px', style: 'dashed'}});
      const result = applyBorderStylesToTheme(theme);
      expect((result?.shape as unknown as Record<string, unknown>)?.['borderStyle']).toBe('dashed');
    });

    it('preserves existing borderRadius in shape', () => {
      const theme = makeBaseTheme({
        shape: {borderRadius: 12},
        border: {width: '2px', style: 'dotted'},
      });
      const result = applyBorderStylesToTheme(theme);
      expect((result?.shape as unknown as Record<string, unknown>)?.['borderRadius']).toBe(12);
    });

    it('only stores provided border properties', () => {
      const theme = makeBaseTheme({border: {width: '3px'}});
      const result = applyBorderStylesToTheme(theme);
      const shape = result?.shape as unknown as Record<string, unknown>;
      expect(shape?.['borderWidth']).toBe('3px');
      expect(shape?.['borderStyle']).toBeUndefined();
    });
  });

  describe('applies border to MuiButton', () => {
    it('applies border to outlined variant (object override)', () => {
      const theme = makeBaseTheme({
        border: {width: '2px', style: 'solid'},
        components: {
          MuiButton: {
            styleOverrides: {
              outlined: {color: 'red'},
            },
          },
        },
      });
      const result = applyBorderStylesToTheme(theme);
      const components = result?.components as Record<string, unknown>;
      const buttonOverrides = (components?.['MuiButton'] as Record<string, unknown>)?.['styleOverrides'] as Record<
        string,
        unknown
      >;
      const outlinedOverride = buttonOverrides?.['outlined'] as Record<string, unknown>;
      expect(outlinedOverride?.['border']).toBe('2px solid');
      expect(outlinedOverride?.['color']).toBe('red');
    });

    it('applies border to outlinedPrimary variant (object override)', () => {
      const theme = makeBaseTheme({
        border: {width: '1px', style: 'dashed'},
        components: {
          MuiButton: {
            styleOverrides: {
              outlinedPrimary: {color: 'blue'},
            },
          },
        },
      });
      const result = applyBorderStylesToTheme(theme);
      const components = result?.components as Record<string, unknown>;
      const buttonOverrides = (components?.['MuiButton'] as Record<string, unknown>)?.['styleOverrides'] as Record<
        string,
        unknown
      >;
      const outlinedPrimaryOverride = buttonOverrides?.['outlinedPrimary'] as Record<string, unknown>;
      expect(outlinedPrimaryOverride?.['border']).toBe('1px dashed');
      expect(outlinedPrimaryOverride?.['color']).toBe('blue');
    });

    it('applies border to outlinedSecondary variant (object override)', () => {
      const theme = makeBaseTheme({
        border: {width: '3px', style: 'dotted'},
        components: {
          MuiButton: {
            styleOverrides: {
              outlinedSecondary: {color: 'green'},
            },
          },
        },
      });
      const result = applyBorderStylesToTheme(theme);
      const components = result?.components as Record<string, unknown>;
      const buttonOverrides = (components?.['MuiButton'] as Record<string, unknown>)?.['styleOverrides'] as Record<
        string,
        unknown
      >;
      const outlinedSecondaryOverride = buttonOverrides?.['outlinedSecondary'] as Record<string, unknown>;
      expect(outlinedSecondaryOverride?.['border']).toBe('3px dotted');
      expect(outlinedSecondaryOverride?.['color']).toBe('green');
    });

    it('handles function-based button overrides', () => {
      const theme = makeBaseTheme({
        border: {width: '2px', style: 'solid'},
        components: {
          MuiButton: {
            styleOverrides: {
              outlined: () => ({color: 'red'}),
            },
          },
        },
      });
      const result = applyBorderStylesToTheme(theme);
      const components = result?.components as Record<string, unknown>;
      const buttonOverrides = (components?.['MuiButton'] as Record<string, unknown>)?.['styleOverrides'] as Record<
        string,
        unknown
      >;
      const outlinedOverride = buttonOverrides?.['outlined'] as (props: unknown) => Record<string, unknown>;
      const overrideResult = outlinedOverride({});
      expect(overrideResult['border']).toBe('2px solid');
      expect(overrideResult['color']).toBe('red');
    });

    it('creates outlined override when absent', () => {
      const theme = makeBaseTheme({border: {width: '2px', style: 'solid'}});
      const result = applyBorderStylesToTheme(theme);
      const components = result?.components as Record<string, unknown>;
      const buttonOverrides = (components?.['MuiButton'] as Record<string, unknown>)?.['styleOverrides'] as Record<
        string,
        unknown
      >;
      const outlinedOverride = buttonOverrides?.['outlined'] as Record<string, unknown>;
      expect(outlinedOverride?.['borderWidth']).toBe('2px');
      expect(outlinedOverride?.['borderStyle']).toBe('solid');
    });

    it('creates outlinedPrimary override when absent', () => {
      const theme = makeBaseTheme({border: {width: '1px', style: 'dashed'}});
      const result = applyBorderStylesToTheme(theme);
      const components = result?.components as Record<string, unknown>;
      const buttonOverrides = (components?.['MuiButton'] as Record<string, unknown>)?.['styleOverrides'] as Record<
        string,
        unknown
      >;
      const outlinedPrimaryOverride = buttonOverrides?.['outlinedPrimary'] as Record<string, unknown>;
      expect(outlinedPrimaryOverride?.['borderWidth']).toBe('1px');
      expect(outlinedPrimaryOverride?.['borderStyle']).toBe('dashed');
    });

    it('creates outlinedSecondary override when absent', () => {
      const theme = makeBaseTheme({border: {width: '3px', style: 'dotted'}});
      const result = applyBorderStylesToTheme(theme);
      const components = result?.components as Record<string, unknown>;
      const buttonOverrides = (components?.['MuiButton'] as Record<string, unknown>)?.['styleOverrides'] as Record<
        string,
        unknown
      >;
      const outlinedSecondaryOverride = buttonOverrides?.['outlinedSecondary'] as Record<string, unknown>;
      expect(outlinedSecondaryOverride?.['borderWidth']).toBe('3px');
      expect(outlinedSecondaryOverride?.['borderStyle']).toBe('dotted');
    });
  });

  describe('applies border to MuiOutlinedInput', () => {
    it('applies borderWidth and borderStyle to notchedOutline', () => {
      const theme = makeBaseTheme({border: {width: '2px', style: 'dashed'}});
      const result = applyBorderStylesToTheme(theme);
      const components = result?.components as Record<string, unknown>;
      const inputOverrides = (components?.['MuiOutlinedInput'] as Record<string, unknown>)?.[
        'styleOverrides'
      ] as Record<string, unknown>;
      const notchedOutline = inputOverrides?.['notchedOutline'] as Record<string, unknown>;
      expect(notchedOutline?.['borderWidth']).toBe('2px');
      expect(notchedOutline?.['borderStyle']).toBe('dashed');
    });

    it('preserves existing notchedOutline overrides', () => {
      const theme = makeBaseTheme({
        border: {width: '1px', style: 'solid'},
        components: {
          MuiOutlinedInput: {
            styleOverrides: {
              notchedOutline: {borderColor: 'blue'},
            },
          },
        },
      });
      const result = applyBorderStylesToTheme(theme);
      const components = result?.components as Record<string, unknown>;
      const inputOverrides = (components?.['MuiOutlinedInput'] as Record<string, unknown>)?.[
        'styleOverrides'
      ] as Record<string, unknown>;
      const notchedOutline = inputOverrides?.['notchedOutline'] as Record<string, unknown>;
      expect(notchedOutline?.['borderColor']).toBe('blue');
      expect(notchedOutline?.['borderWidth']).toBe('1px');
    });

    it('preserves callback-based notchedOutline overrides', () => {
      const theme = makeBaseTheme({
        border: {width: '2px', style: 'dashed'},
        components: {
          MuiOutlinedInput: {
            styleOverrides: {
              notchedOutline: () => ({borderColor: 'orange'}),
            },
          },
        },
      });
      const result = applyBorderStylesToTheme(theme);
      const components = result?.components as Record<string, unknown>;
      const inputOverrides = (components?.['MuiOutlinedInput'] as Record<string, unknown>)?.[
        'styleOverrides'
      ] as Record<string, unknown>;
      const notchedOutline = inputOverrides?.['notchedOutline'] as (props: unknown) => Record<string, unknown>;
      const overrideResult = notchedOutline({});
      expect(overrideResult['borderColor']).toBe('orange');
      expect(overrideResult['borderWidth']).toBe('2px');
      expect(overrideResult['borderStyle']).toBe('dashed');
    });
  });

  describe('applies border to MuiChip', () => {
    it('applies borderWidth and borderStyle to outlined variant', () => {
      const theme = makeBaseTheme({border: {width: '2px', style: 'dotted'}});
      const result = applyBorderStylesToTheme(theme);
      const components = result?.components as Record<string, unknown>;
      const chipOverrides = (components?.['MuiChip'] as Record<string, unknown>)?.['styleOverrides'] as Record<
        string,
        unknown
      >;
      const outlined = chipOverrides?.['outlined'] as Record<string, unknown>;
      expect(outlined?.['borderWidth']).toBe('2px');
      expect(outlined?.['borderStyle']).toBe('dotted');
    });

    it('preserves existing outlined overrides', () => {
      const theme = makeBaseTheme({
        border: {width: '1px', style: 'solid'},
        components: {
          MuiChip: {
            styleOverrides: {
              outlined: {borderColor: 'green'},
            },
          },
        },
      });
      const result = applyBorderStylesToTheme(theme);
      const components = result?.components as Record<string, unknown>;
      const chipOverrides = (components?.['MuiChip'] as Record<string, unknown>)?.['styleOverrides'] as Record<
        string,
        unknown
      >;
      const outlined = chipOverrides?.['outlined'] as Record<string, unknown>;
      expect(outlined?.['borderColor']).toBe('green');
      expect(outlined?.['borderWidth']).toBe('1px');
    });

    it('preserves callback-based outlined overrides', () => {
      const theme = makeBaseTheme({
        border: {width: '2px', style: 'dotted'},
        components: {
          MuiChip: {
            styleOverrides: {
              outlined: () => ({borderColor: 'purple'}),
            },
          },
        },
      });
      const result = applyBorderStylesToTheme(theme);
      const components = result?.components as Record<string, unknown>;
      const chipOverrides = (components?.['MuiChip'] as Record<string, unknown>)?.['styleOverrides'] as Record<
        string,
        unknown
      >;
      const outlined = chipOverrides?.['outlined'] as (props: unknown) => Record<string, unknown>;
      const overrideResult = outlined({});
      expect(overrideResult['borderColor']).toBe('purple');
      expect(overrideResult['borderWidth']).toBe('2px');
      expect(overrideResult['borderStyle']).toBe('dotted');
    });
  });

  describe('uses default values', () => {
    it('uses 1px as default borderWidth', () => {
      const theme = makeBaseTheme({border: {style: 'solid'}});
      const result = applyBorderStylesToTheme(theme);
      // Default is applied in component overrides, not in shape
      const components = result?.components as Record<string, unknown>;
      const inputOverrides = (components?.['MuiOutlinedInput'] as Record<string, unknown>)?.[
        'styleOverrides'
      ] as Record<string, unknown>;
      const notchedOutline = inputOverrides?.['notchedOutline'] as Record<string, unknown>;
      expect(notchedOutline?.['borderWidth']).toBe('1px');
    });

    it('uses solid as default borderStyle', () => {
      const theme = makeBaseTheme({border: {width: '2px'}});
      const result = applyBorderStylesToTheme(theme);
      const components = result?.components as Record<string, unknown>;
      const inputOverrides = (components?.['MuiOutlinedInput'] as Record<string, unknown>)?.[
        'styleOverrides'
      ] as Record<string, unknown>;
      const notchedOutline = inputOverrides?.['notchedOutline'] as Record<string, unknown>;
      expect(notchedOutline?.['borderStyle']).toBe('solid');
    });
  });

  describe('immutability', () => {
    it('modifies the theme in-place (as intended)', () => {
      const theme = makeBaseTheme({border: {width: '2px', style: 'dashed'}});
      const result = applyBorderStylesToTheme(theme);
      expect(result).toBe(theme);
    });

    it('replaces outlined override object with new one', () => {
      const theme = makeBaseTheme({
        border: {width: '2px', style: 'solid'},
        components: {
          MuiButton: {
            styleOverrides: {
              outlined: {color: 'red'},
            },
          },
        },
      });
      const originalOverride = (
        (theme.components as Record<string, unknown>)?.['MuiButton'] as Record<string, unknown>
      )?.['styleOverrides'] as Record<string, unknown>;
      const originalOutlined = originalOverride?.['outlined'];

      const result = applyBorderStylesToTheme(theme);
      const resultOverride = (
        (result?.components as Record<string, unknown>)?.['MuiButton'] as Record<string, unknown>
      )?.['styleOverrides'] as Record<string, unknown>;
      const resultOutlined = resultOverride?.['outlined'];

      // The outlined override object should have border property added
      expect(typeof resultOutlined).toBe('object');
      expect((resultOutlined as Record<string, unknown>)?.['border']).toBe('2px solid');
      expect(resultOutlined).not.toBe(originalOutlined);
    });
  });

  describe('edge cases and branch coverage', () => {
    it('handles border with only width (no style)', () => {
      const theme = makeBaseTheme({border: {width: '2px'}});
      const result = applyBorderStylesToTheme(theme);
      const components = result?.components as Record<string, unknown>;
      const buttonOverrides = (components?.['MuiButton'] as Record<string, unknown>)?.['styleOverrides'] as Record<
        string,
        unknown
      >;
      const outlinedOverride = buttonOverrides?.['outlined'] as Record<string, unknown>;
      expect(outlinedOverride?.['borderWidth']).toBe('2px');
      expect(outlinedOverride?.['borderStyle']).toBe('solid');
    });

    it('handles border with only style (no width)', () => {
      const theme = makeBaseTheme({border: {style: 'dashed'}});
      const result = applyBorderStylesToTheme(theme);
      const components = result?.components as Record<string, unknown>;
      const buttonOverrides = (components?.['MuiButton'] as Record<string, unknown>)?.['styleOverrides'] as Record<
        string,
        unknown
      >;
      const outlinedOverride = buttonOverrides?.['outlined'] as Record<string, unknown>;
      expect(outlinedOverride?.['borderWidth']).toBe('1px');
      expect(outlinedOverride?.['borderStyle']).toBe('dashed');
    });

    it('applies all three button variants independently', () => {
      const theme = makeBaseTheme({
        border: {width: '2px', style: 'solid'},
        components: {
          MuiButton: {
            styleOverrides: {
              outlined: {color: 'red'},
              outlinedPrimary: {color: 'blue'},
              outlinedSecondary: {color: 'green'},
            },
          },
        },
      });
      const result = applyBorderStylesToTheme(theme);
      const components = result?.components as Record<string, unknown>;
      const buttonOverrides = (components?.['MuiButton'] as Record<string, unknown>)?.['styleOverrides'] as Record<
        string,
        unknown
      >;
      const outlined = buttonOverrides?.['outlined'] as Record<string, unknown>;
      const outlinedPrimary = buttonOverrides?.['outlinedPrimary'] as Record<string, unknown>;
      const outlinedSecondary = buttonOverrides?.['outlinedSecondary'] as Record<string, unknown>;

      expect(outlined?.['border']).toBe('2px solid');
      expect(outlinedPrimary?.['border']).toBe('2px solid');
      expect(outlinedSecondary?.['border']).toBe('2px solid');
    });

    it('applies border to MuiChip outlined variant', () => {
      const theme = makeBaseTheme({
        border: {width: '1px', style: 'solid'},
        components: {
          MuiChip: {
            styleOverrides: {
              outlined: {borderColor: 'red'},
            },
          },
        },
      });
      const result = applyBorderStylesToTheme(theme);
      const components = result?.components as Record<string, unknown>;
      const chipOverrides = (components?.['MuiChip'] as Record<string, unknown>)?.['styleOverrides'] as Record<
        string,
        unknown
      >;
      const outlined = chipOverrides?.['outlined'] as Record<string, unknown>;
      expect(outlined?.['borderColor']).toBe('red');
      expect(outlined?.['borderWidth']).toBe('1px');
      expect(outlined?.['borderStyle']).toBe('solid');
    });

    it('handles function-based MuiButton outlinedPrimary overrides', () => {
      const theme = makeBaseTheme({
        border: {width: '1px', style: 'dashed'},
        components: {
          MuiButton: {
            styleOverrides: {
              outlinedPrimary: () => ({color: 'blue'}),
            },
          },
        },
      });
      const result = applyBorderStylesToTheme(theme);
      const components = result?.components as Record<string, unknown>;
      const buttonOverrides = (components?.['MuiButton'] as Record<string, unknown>)?.['styleOverrides'] as Record<
        string,
        unknown
      >;
      const outlinedPrimaryOverride = buttonOverrides?.['outlinedPrimary'] as (
        props: unknown,
      ) => Record<string, unknown>;
      const overrideResult = outlinedPrimaryOverride({});
      expect(overrideResult['border']).toBe('1px dashed');
      expect(overrideResult['color']).toBe('blue');
    });

    it('handles function-based MuiButton outlinedSecondary overrides', () => {
      const theme = makeBaseTheme({
        border: {width: '3px', style: 'dotted'},
        components: {
          MuiButton: {
            styleOverrides: {
              outlinedSecondary: () => ({color: 'green'}),
            },
          },
        },
      });
      const result = applyBorderStylesToTheme(theme);
      const components = result?.components as Record<string, unknown>;
      const buttonOverrides = (components?.['MuiButton'] as Record<string, unknown>)?.['styleOverrides'] as Record<
        string,
        unknown
      >;
      const outlinedSecondaryOverride = buttonOverrides?.['outlinedSecondary'] as (
        props: unknown,
      ) => Record<string, unknown>;
      const overrideResult = outlinedSecondaryOverride({});
      expect(overrideResult['border']).toBe('3px dotted');
      expect(overrideResult['color']).toBe('green');
    });

    it('handles function-based MuiChip outlined overrides', () => {
      const theme = makeBaseTheme({
        border: {width: '1px', style: 'solid'},
        components: {
          MuiChip: {
            styleOverrides: {
              outlined: () => ({borderColor: 'red'}),
            },
          },
        },
      });
      const result = applyBorderStylesToTheme(theme);
      const components = result?.components as Record<string, unknown>;
      const chipOverrides = (components?.['MuiChip'] as Record<string, unknown>)?.['styleOverrides'] as Record<
        string,
        unknown
      >;
      const outlinedOverride = chipOverrides?.['outlined'] as (props: unknown) => Record<string, unknown>;
      const overrideResult = outlinedOverride({});
      expect(overrideResult['borderColor']).toBe('red');
      expect(overrideResult['borderWidth']).toBe('1px');
      expect(overrideResult['borderStyle']).toBe('solid');
    });
  });
});
