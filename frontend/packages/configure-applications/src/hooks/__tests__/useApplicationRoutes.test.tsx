// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@testing-library/react';
import {RoutesProvider} from '@thunderid/contexts';
import type {ReactNode} from 'react';
import {describe, expect, it} from 'vitest';
import useApplicationRoutes from '../useApplicationRoutes';

function withPaths(paths: Record<string, Record<string, () => string>>) {
  return function Wrapper({children}: {children: ReactNode}) {
    return <RoutesProvider paths={paths}>{children}</RoutesProvider>;
  };
}

describe('useApplicationRoutes', () => {
  it("uses the host's path to add a user", () => {
    const {result} = renderHook(() => useApplicationRoutes(), {
      wrapper: withPaths({users: {add: () => '/people/new'}}),
    });

    expect(result.current.users.add()).toBe('/people/new');
  });

  it('adds users at its own path when the host has no path to add one', () => {
    const {result} = renderHook(() => useApplicationRoutes(), {
      wrapper: withPaths({users: {list: () => '/people'}}),
    });

    expect(result.current.users.add()).toBe('/users/add');
  });
});
