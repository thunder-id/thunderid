// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act, type ReactNode} from 'react';
import {createRoot, type Root} from 'react-dom/client';
import {describe, it, expect, beforeEach, afterEach} from 'vitest';
import ConfigProvider from '../../Config/ConfigProvider';
import type {ProductConfig} from '../../Config/types';
import RuntimeProvider from '../RuntimeProvider';
import useRuntimeUrl from '../useRuntimeUrl';

const serverConfig: ProductConfig = {
  brand: {
    product_name: 'Test Product',
    favicon: {light: 'assets/images/favicon.ico', dark: 'assets/images/favicon-inverted.ico'},
  },
  client: {base: '/console', client_id: 'CONSOLE'},
  server: {public_url: 'https://cp.example.com:8090'},
};

function TokenEndpoint() {
  return <span data-testid="token-endpoint">{`${useRuntimeUrl()}/oauth2/token`}</span>;
}

let container: HTMLDivElement;
let root: Root;
let originalConfig: ProductConfig | undefined;

function render(children: ReactNode) {
  act(() => {
    root.render(<ConfigProvider>{children}</ConfigProvider>);
  });
}

function tokenEndpoint(): string {
  return container.querySelector('[data-testid="token-endpoint"]')?.textContent ?? '';
}

beforeEach(() => {
  originalConfig = window.__THUNDERID_RUNTIME_CONFIG__;
  window.__THUNDERID_RUNTIME_CONFIG__ = serverConfig;
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => {
    root.unmount();
  });
  container.remove();
  window.__THUNDERID_RUNTIME_CONFIG__ = originalConfig;
});

describe('useRuntimeUrl', () => {
  it('builds endpoints from the supplied runtime URL rather than the server URL', () => {
    render(
      <RuntimeProvider url="https://gateway.example.com:8090">
        <TokenEndpoint />
      </RuntimeProvider>,
    );

    expect(tokenEndpoint()).toBe('https://gateway.example.com:8090/oauth2/token');
  });

  it('falls back to the server URL when no provider is present', () => {
    render(<TokenEndpoint />);

    expect(tokenEndpoint()).toBe('https://cp.example.com:8090/oauth2/token');
  });

  // A host application that has not resolved a runtime URL yet, or that found none registered,
  // renders the provider with nothing. That must read the same as no provider at all rather than
  // producing endpoints with no origin.
  it('falls back to the server URL when the provider has no URL', () => {
    render(
      <RuntimeProvider>
        <TokenEndpoint />
      </RuntimeProvider>,
    );

    expect(tokenEndpoint()).toBe('https://cp.example.com:8090/oauth2/token');
  });

  it('drops a trailing slash so endpoints are not built with a doubled separator', () => {
    render(
      <RuntimeProvider url="https://gateway.example.com:8090/">
        <TokenEndpoint />
      </RuntimeProvider>,
    );

    expect(tokenEndpoint()).toBe('https://gateway.example.com:8090/oauth2/token');
  });
});
