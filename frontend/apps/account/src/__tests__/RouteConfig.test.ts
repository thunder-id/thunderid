// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import RouteConfig, {ACCOUNT_PAGES} from '../configs/RouteConfig';

describe('RouteConfig', () => {
  it('builds the path of the home page and every Account App page', () => {
    expect(ACCOUNT_PAGES).toEqual(['profile', 'security']);
    expect(RouteConfig.home()).toBe('/');
    expect(RouteConfig.profile()).toBe('/profile');
    expect(RouteConfig.security()).toBe('/security');
  });
});
