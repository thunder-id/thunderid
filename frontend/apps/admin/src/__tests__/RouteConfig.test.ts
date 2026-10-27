// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import RouteConfig, {MANAGED_RESOURCES} from '../configs/RouteConfig';

describe('RouteConfig', () => {
  it('builds the list, create and details paths of every managed resource', () => {
    expect(MANAGED_RESOURCES).toEqual(['users', 'groups', 'roles', 'agents']);
    expect(RouteConfig.users.list()).toBe('/users');
    expect(RouteConfig.groups.create()).toBe('/groups/new');
    expect(RouteConfig.roles.details('r-1')).toBe('/roles/r-1');
  });

  it('encodes identifiers in details paths', () => {
    expect(RouteConfig.agents.details('a/b')).toBe('/agents/a%2Fb');
  });
});
