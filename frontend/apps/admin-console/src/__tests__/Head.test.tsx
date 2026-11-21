// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render} from '@testing-library/react';
import {DEFAULT_ADMIN_APP_CONFIG, type AppConfig} from '@thunderid/system-apps';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import Head from '../components/Head';

const {appConfig} = vi.hoisted(() => ({appConfig: {current: undefined as unknown as AppConfig}}));

vi.mock('../hooks/useAdminAppConfig', () => ({default: () => ({config: appConfig.current, isLoading: false})}));

vi.mock('@thunderid/contexts', () => ({
  useConfig: () => ({
    config: {brand: {favicon: {dark: 'assets/images/favicon-inverted.ico', light: 'assets/images/favicon.ico'}}},
  }),
}));

const iconFor = (scheme: 'dark' | 'light'): string | null =>
  document.head.querySelector(`link[rel="icon"][media="(prefers-color-scheme: ${scheme})"]`)?.getAttribute('href') ??
  null;

describe('Head', () => {
  beforeEach(() => {
    appConfig.current = DEFAULT_ADMIN_APP_CONFIG;
  });

  it("shows the app's own favicons", () => {
    render(<Head />);

    expect(iconFor('light')).toMatch(/assets\/images\/favicon\.ico$/);
    expect(iconFor('dark')).toMatch(/assets\/images\/favicon-inverted\.ico$/);
  });

  it('shows the favicon the Admin App sets, its light one in both schemes without a dark one', () => {
    appConfig.current = {
      ...DEFAULT_ADMIN_APP_CONFIG,
      branding: {...DEFAULT_ADMIN_APP_CONFIG.branding, favicon: {light: 'https://acme.example/icon.png'}},
    };
    render(<Head />);

    expect(iconFor('light')).toBe('https://acme.example/icon.png');
    expect(iconFor('dark')).toBe('https://acme.example/icon.png');
  });

  it('shows the dark favicon the Admin App sets in the dark scheme', () => {
    appConfig.current = {
      ...DEFAULT_ADMIN_APP_CONFIG,
      branding: {
        ...DEFAULT_ADMIN_APP_CONFIG.branding,
        favicon: {dark: 'https://acme.example/icon-dark.png', light: 'https://acme.example/icon.png'},
      },
    };
    render(<Head />);

    expect(iconFor('dark')).toBe('https://acme.example/icon-dark.png');
  });
});
