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
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import Head from '../Head';

const mockUseConfig = vi.hoisted(() => vi.fn());
vi.mock('@thunderid/contexts', () => ({
  useConfig: mockUseConfig,
}));

vi.mock('@thunderid/components', () => ({
  Helmet: ({children = undefined}: {children?: ReactNode}) => children,
}));

const defaultFavicon = {
  light: 'assets/images/favicon.ico',
  dark: 'assets/images/favicon-inverted.ico',
};

const withBase = (path: string): string => `${import.meta.env.BASE_URL.replace(/\/$/, '')}/${path.replace(/^\//, '')}`;

const iconFor = (scheme: 'light' | 'dark'): Element | null =>
  document.head.querySelector(`link[rel="icon"][media="(prefers-color-scheme: ${scheme})"]`);

describe('Head', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseConfig.mockReturnValue({config: {brand: {favicon: defaultFavicon}}});
  });

  afterEach(() => {
    cleanup();
    document.head.querySelectorAll('link[rel="icon"]').forEach((el) => el.remove());
  });

  it('renders a light and a dark favicon link, each scoped to a prefers-color-scheme media query', () => {
    render(<Head />);
    expect(document.head.querySelectorAll('link[rel="icon"]')).toHaveLength(2);
    expect(iconFor('light')).not.toBeNull();
    expect(iconFor('dark')).not.toBeNull();
  });

  it('maps the light favicon to the light color scheme and the dark favicon to the dark color scheme', () => {
    render(<Head />);
    expect(iconFor('light')).toHaveAttribute('href', withBase(defaultFavicon.light));
    expect(iconFor('dark')).toHaveAttribute('href', withBase(defaultFavicon.dark));
  });

  it('prefixes the base URL to relative favicon paths', () => {
    render(<Head />);
    const href = iconFor('light')?.getAttribute('href');
    expect(href).toBe(withBase(defaultFavicon.light));
    expect(href?.startsWith(import.meta.env.BASE_URL)).toBe(true);
  });

  it('uses absolute favicon URLs as-is without prefixing the base URL', () => {
    mockUseConfig.mockReturnValue({
      config: {
        brand: {
          favicon: {light: 'https://cdn.example.com/light.ico', dark: 'https://cdn.example.com/dark.ico'},
        },
      },
    });
    render(<Head />);
    expect(iconFor('light')).toHaveAttribute('href', 'https://cdn.example.com/light.ico');
    expect(iconFor('dark')).toHaveAttribute('href', 'https://cdn.example.com/dark.ico');
  });

  it('reflects custom favicon paths from config', () => {
    mockUseConfig.mockReturnValue({
      config: {
        brand: {
          favicon: {light: 'custom/light.ico', dark: 'custom/dark.ico'},
        },
      },
    });
    render(<Head />);
    expect(iconFor('light')).toHaveAttribute('href', withBase('custom/light.ico'));
    expect(iconFor('dark')).toHaveAttribute('href', withBase('custom/dark.ico'));
  });
});
