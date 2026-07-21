/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import {render, cleanup} from '@testing-library/react';
import {OxygenUIThemeProvider} from '@wso2/oxygen-ui';
import {afterEach, describe, expect, it} from 'vitest';
import DefaultTheme from '../DefaultTheme';

// Normalize any CSS color to its `rgb(...)` form so hex and rgb values compare equal.
function toRgb(color: string): string {
  const probe = document.createElement('span');
  probe.style.color = color;
  document.body.appendChild(probe);
  const resolved = getComputedStyle(probe).color;
  probe.remove();
  return resolved;
}

// The "r g b" channel MUI stores for alpha tints is the hex split into decimals.
function channelOf(hex: string): string {
  const n = parseInt(hex.slice(1), 16);
  return `${(n >> 16) & 255} ${(n >> 8) & 255} ${n & 255}`;
}

const PALETTE_COLORS = ['primary', 'secondary', 'error', 'warning', 'info', 'success'] as const;
const CHANNEL_VARIANTS = ['main', 'light', 'dark'] as const;

// Oxygen's frozen orange primary channel — what mainChannel resolves to if override is dropped.
const OXYGEN_ORANGE_CHANNEL = '255 115 0';

interface PaletteColor {
  main: string;
  light?: string;
  dark?: string;
  mainChannel?: string;
  lightChannel?: string;
  darkChannel?: string;
}

function palette(scheme: 'light' | 'dark'): Record<(typeof PALETTE_COLORS)[number], PaletteColor> {
  const {colorSchemes} = DefaultTheme as unknown as {
    colorSchemes: Record<'light' | 'dark', {palette: Record<(typeof PALETTE_COLORS)[number], PaletteColor>}>;
  };
  return colorSchemes[scheme].palette;
}

// createOxygenTheme merges our palette over Oxygen's base but doesn't recompute the baked
// channels, so each must be set explicitly or MUI tints resolve to Oxygen's defaults. Asserting
// each channel against its own hex keeps this robust to future colour changes.
describe('DefaultTheme transparent-tint channels', () => {
  afterEach(() => {
    cleanup();
  });

  it('derives every colour channel from its own hex, not the Oxygen defaults', () => {
    (['light', 'dark'] as const).forEach((scheme) => {
      const colors = palette(scheme);

      PALETTE_COLORS.flatMap((name) =>
        CHANNEL_VARIANTS.filter((variant) => typeof colors[name][variant] === 'string').map((variant) => ({
          actual: colors[name][`${variant}Channel`],
          expected: channelOf(colors[name][variant]!),
        })),
      ).forEach(({actual, expected}) => {
        expect(actual).toBe(expected);
      });

      expect(colors.primary.mainChannel).not.toBe(OXYGEN_ORANGE_CHANNEL);
    });
  });

  it('resolves a primary-tinted surface to primary.main at runtime', () => {
    const {container} = render(
      <OxygenUIThemeProvider
        themes={[{key: 'default', label: 'Default Theme', theme: DefaultTheme}]}
        initialTheme="default"
      >
        <div data-testid="tint" style={{backgroundColor: 'rgba(var(--oxygen-palette-primary-mainChannel) / 1)'}} />
      </OxygenUIThemeProvider>,
    );

    const tint = container.querySelector('[data-testid="tint"]');
    expect(tint).not.toBeNull();

    const bg = toRgb(getComputedStyle(tint!).backgroundColor);
    expect(bg).toBe(toRgb(palette('light').primary.main));
  });
});
