// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act} from 'react';
import {createRoot, type Root} from 'react-dom/client';
import {beforeEach, afterEach, describe, expect, it} from 'vitest';
import type {AdministrationConfig} from '../AdministrationContext';
import AdministrationProvider from '../AdministrationProvider';
import {AdministrationModes} from '../constants';
import {
  DEFAULT_ADMINISTRATION_MODE,
  resolveAdministrationOperation,
  useAdministrationOperation,
} from '../useAdministration';

describe('resolveAdministrationOperation', () => {
  // The whole contract is the order, so each level is pinned against a configuration that sets
  // every level at once: a test that set only one level would pass whatever the order was.
  const everyLevel: AdministrationConfig = {
    mode: AdministrationModes.NATIVE,
    features: {
      users: {
        mode: AdministrationModes.FLOW,
        operations: {add: AdministrationModes.NATIVE},
      },
    },
  };

  it('prefers the operation over the feature and the console', () => {
    expect(resolveAdministrationOperation(everyLevel, 'users', 'add')).toBe(AdministrationModes.NATIVE);
  });

  it('falls back to the feature when the operation names nothing', () => {
    expect(resolveAdministrationOperation(everyLevel, 'users', 'delete')).toBe(AdministrationModes.FLOW);
  });

  it('falls back to the console when the feature names nothing', () => {
    expect(resolveAdministrationOperation(everyLevel, 'applications', 'delete')).toBe(AdministrationModes.NATIVE);
  });

  it('falls back to the default when nothing names anything', () => {
    expect(resolveAdministrationOperation({}, 'users', 'delete')).toBe(DEFAULT_ADMINISTRATION_MODE);
  });

  // Defaulting to native would silently stop revoking tokens and ending sessions on deployments
  // that never asked for the change, so the default has to be what they did before the option
  // existed.
  it('defaults to running through the flow', () => {
    expect(DEFAULT_ADMINISTRATION_MODE).toBe(AdministrationModes.FLOW);
  });

  // A distributor can replace one operation without forking the package that owns it.
  it('returns a configured function as it stands', () => {
    const custom = (): Promise<void> => Promise.resolve();

    const resolved = resolveAdministrationOperation(
      {features: {users: {operations: {delete: custom}}}},
      'users',
      'delete',
    );

    expect(resolved).toBe(custom);
  });
});

function DeleteMode(): React.JSX.Element {
  const how = useAdministrationOperation('users', 'delete');

  return <span data-testid="mode">{typeof how === 'function' ? 'custom' : how}</span>;
}

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
});

describe('useAdministrationOperation', () => {
  it('reads the mode the provider declares', () => {
    act(() => {
      root.render(
        <AdministrationProvider administration={{mode: AdministrationModes.NATIVE}}>
          <DeleteMode />
        </AdministrationProvider>,
      );
    });

    expect(container.querySelector('[data-testid="mode"]')?.textContent).toBe(AdministrationModes.NATIVE);
  });

  // A package has to render in a test or a story without a provider above it.
  it('falls back to the default without a provider', () => {
    act(() => {
      root.render(<DeleteMode />);
    });

    expect(container.querySelector('[data-testid="mode"]')?.textContent).toBe(DEFAULT_ADMINISTRATION_MODE);
  });
});
