// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act, useEffect, type ReactNode} from 'react';
import {createRoot, type Root} from 'react-dom/client';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import type {EnvironmentContextType} from '../EnvironmentContext';
import EnvironmentProvider from '../EnvironmentProvider';
import useEnvironment from '../useEnvironment';

const environments = [
  {id: 'gw-1', name: 'Dev', isDefault: true},
  {id: 'gw-2', name: 'Prod'},
];

let container: HTMLDivElement;
let root: Root;
let read: EnvironmentContextType | undefined;

function Reader(): null {
  const environment = useEnvironment();
  useEffect(() => {
    read = environment;
  });
  return null;
}

function render(node: ReactNode): void {
  act(() => root.render(node));
}

beforeEach(() => {
  read = undefined;
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
});

describe('EnvironmentProvider', () => {
  it('shows the configuration without a selection', () => {
    render(
      <EnvironmentProvider environments={environments} onSelect={vi.fn()}>
        <Reader />
      </EnvironmentProvider>,
    );
    expect(read?.selected).toBeUndefined();
    expect(read?.environments).toEqual(environments);
  });

  it('shows the environment selected', () => {
    render(
      <EnvironmentProvider environments={environments} selectedId="gw-2" onSelect={vi.fn()}>
        <Reader />
      </EnvironmentProvider>,
    );
    expect(read?.selected?.name).toBe('Prod');
  });

  it('shows the configuration for a selection that is not an environment', () => {
    render(
      <EnvironmentProvider environments={environments} selectedId="gw-9" onSelect={vi.fn()}>
        <Reader />
      </EnvironmentProvider>,
    );
    expect(read?.selected).toBeUndefined();
  });

  it('hands a choice to the host', () => {
    const onSelect = vi.fn();
    render(
      <EnvironmentProvider environments={environments} onSelect={onSelect}>
        <Reader />
      </EnvironmentProvider>,
    );
    read?.select('gw-1');
    read?.select(undefined);
    expect(onSelect).toHaveBeenNthCalledWith(1, 'gw-1');
    expect(onSelect).toHaveBeenNthCalledWith(2, undefined);
  });

  it('is the configuration with no environments without a provider', () => {
    render(<Reader />);
    expect(read?.environments).toEqual([]);
    expect(read?.selected).toBeUndefined();
    expect(() => read?.select('gw-1')).not.toThrow();
  });
});
