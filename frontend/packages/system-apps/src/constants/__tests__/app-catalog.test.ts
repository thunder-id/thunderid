// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import type {AppPageConfig} from '../../models/app-config';
import {APP_PAGES, DEFAULT_ACCOUNT_APP_CONFIG, DEFAULT_ADMIN_APP_CONFIG} from '../app-catalog';

describe('app catalog', () => {
  it('gives the Account App its profile and security pages, beside the content', () => {
    expect(APP_PAGES.ACCOUNT.map((page) => page.key)).toEqual(['profile', 'security']);
    expect(DEFAULT_ACCOUNT_APP_CONFIG.type).toBe('ACCOUNT');
    expect(DEFAULT_ACCOUNT_APP_CONFIG.navigation.layout).toBe('SIDE_BAR');
    expect(DEFAULT_ACCOUNT_APP_CONFIG.pages.map((page: AppPageConfig) => [page.key, page.actions])).toEqual([
      ['profile', {edit: true}],
      ['security', {password: true}],
    ]);
  });

  it('keeps the Admin App on a top bar', () => {
    expect(DEFAULT_ADMIN_APP_CONFIG.navigation.layout).toBe('TOP_BAR');
  });
});
