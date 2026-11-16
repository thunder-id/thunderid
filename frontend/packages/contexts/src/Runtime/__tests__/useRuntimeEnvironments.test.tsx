// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act, type ReactNode} from 'react';
import {createRoot, type Root} from 'react-dom/client';
import {afterEach, beforeEach, describe, expect, it} from 'vitest';
import RuntimeEnvironmentsContext, {type RuntimeEnvironmentsContextType} from '../RuntimeEnvironmentsContext';
import useRuntimeEnvironments from '../useRuntimeEnvironments';

const environments: RuntimeEnvironmentsContextType = {
  environments: [
    {id: 'env-dev', name: 'Development', runtimeUrl: 'https://dev.example.com'},
    {id: 'env-prod', name: 'Production', runtimeUrl: 'https://prod.example.com'},
  ],
  getValue: () => Promise.resolve({status: 'unset'}),
};

function EnvironmentNames() {
  const runtime = useRuntimeEnvironments();
  return (
    <span data-testid="environments">
      {runtime ? runtime.environments.map((environment) => environment.name).join(', ') : 'none'}
    </span>
  );
}

let container: HTMLDivElement;
let root: Root;

function render(children: ReactNode) {
  act(() => {
    root.render(children);
  });
}

function environmentNames(): string {
  return container.querySelector('[data-testid="environments"]')?.textContent ?? '';
}

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
});

describe('useRuntimeEnvironments', () => {
  it('returns undefined where the console provides no environments', () => {
    render(<EnvironmentNames />);

    expect(environmentNames()).toBe('none');
  });

  it('returns the environments the console provides, in order', () => {
    render(
      <RuntimeEnvironmentsContext.Provider value={environments}>
        <EnvironmentNames />
      </RuntimeEnvironmentsContext.Provider>,
    );

    expect(environmentNames()).toBe('Development, Production');
  });
});
