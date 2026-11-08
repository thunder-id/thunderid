// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act} from 'react';
import {createRoot, type Root} from 'react-dom/client';
import {beforeEach, afterEach, describe, expect, it} from 'vitest';
import type {Capabilities} from '../CapabilitiesContext';
import CapabilitiesProvider from '../CapabilitiesProvider';
import useCapabilitiesContext from '../useCapabilitiesContext';

function DeleteAppButton() {
  const {can} = useCapabilitiesContext();
  return can('app', 'delete') ? <button type="button">Delete</button> : null;
}

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => {
    root.unmount();
  });
  container.remove();
});

describe('CapabilitiesProvider', () => {
  it('exposes the supplied capabilities to descendants via useCapabilitiesContext', () => {
    const capabilities: Capabilities = {
      can: (resource, action = 'view') => resource === 'app' && action === 'view',
      canManageRoot: false,
      isLoading: false,
    };

    act(() => {
      root.render(
        <CapabilitiesProvider capabilities={capabilities}>
          <DeleteAppButton />
        </CapabilitiesProvider>,
      );
    });

    expect(container.querySelector('button')).toBeNull();
  });

  it('allows everything when rendered without a provider', () => {
    act(() => {
      root.render(<DeleteAppButton />);
    });

    expect(container.querySelector('button')?.textContent).toBe('Delete');
  });
});
