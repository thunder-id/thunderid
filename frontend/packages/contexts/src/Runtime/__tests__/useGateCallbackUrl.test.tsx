// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act, type ReactNode} from 'react';
import {createRoot, type Root} from 'react-dom/client';
import {describe, it, expect, beforeEach, afterEach} from 'vitest';
import ConfigProvider from '../../Config/ConfigProvider';
import type {ProductConfig} from '../../Config/types';
import RuntimeProvider from '../RuntimeProvider';
import useGateCallbackUrl from '../useGateCallbackUrl';

function buildConfig(overrides?: Partial<ProductConfig>): ProductConfig {
  return {
    brand: {
      product_name: 'Test Product',
      favicon: {light: 'assets/images/favicon.ico', dark: 'assets/images/favicon-inverted.ico'},
    },
    client: {base: '/console', client_id: 'CONSOLE'},
    server: {public_url: 'https://cp.example.com/_api/iam'},
    ...overrides,
  };
}

function GateCallbackUrl() {
  return <span data-testid="gate-callback-url">{useGateCallbackUrl()}</span>;
}

let container: HTMLDivElement;
let root: Root;
let originalConfig: ProductConfig | undefined;

function render(config: ProductConfig, children: ReactNode) {
  window.__THUNDERID_RUNTIME_CONFIG__ = config;
  act(() => {
    root.render(<ConfigProvider>{children}</ConfigProvider>);
  });
}

function gateCallbackUrl(): string {
  return container.querySelector('[data-testid="gate-callback-url"]')?.textContent ?? '';
}

beforeEach(() => {
  originalConfig = window.__THUNDERID_RUNTIME_CONFIG__;
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

describe('useGateCallbackUrl', () => {
  // A console managing a separate runtime must name the gate the identity provider returns the
  // user to, not the server the console talks to.
  it('builds the URL from the runtime URL rather than the server URL', () => {
    render(
      buildConfig(),
      <RuntimeProvider url="https://gateway.example.com">
        <GateCallbackUrl />
      </RuntimeProvider>,
    );

    expect(gateCallbackUrl()).toBe('https://gateway.example.com/gate/callback');
  });

  it('falls back to the server URL when no runtime URL is supplied', () => {
    render(buildConfig({server: {hostname: 'localhost', port: 8090, http_only: false}}), <GateCallbackUrl />);

    expect(gateCallbackUrl()).toBe('https://localhost:8090/gate/callback');
  });

  it('falls back to the served origin when neither a runtime URL nor a server is configured', () => {
    render(buildConfig({server: undefined}), <GateCallbackUrl />);

    expect(gateCallbackUrl()).toBe(`${window.location.origin}/gate/callback`);
  });

  it('prefers gate_client public_url over the runtime URL', () => {
    render(
      buildConfig({gate_client: {public_url: 'https://localhost:5190'}}),
      <RuntimeProvider url="https://gateway.example.com">
        <GateCallbackUrl />
      </RuntimeProvider>,
    );

    expect(gateCallbackUrl()).toBe('https://localhost:5190/gate/callback');
  });

  it('builds the URL from gate_client hostname/port/scheme', () => {
    render(buildConfig({gate_client: {hostname: 'localhost', port: 5190, scheme: 'http'}}), <GateCallbackUrl />);

    expect(gateCallbackUrl()).toBe('http://localhost:5190/gate/callback');
  });

  it('defaults gate_client scheme to https when hostname/port are set without a scheme', () => {
    render(buildConfig({gate_client: {hostname: 'localhost', port: 5190}}), <GateCallbackUrl />);

    expect(gateCallbackUrl()).toBe('https://localhost:5190/gate/callback');
  });

  it('builds the URL from gate_client hostname without a port', () => {
    render(buildConfig({gate_client: {hostname: 'gate.example.com'}}), <GateCallbackUrl />);

    expect(gateCallbackUrl()).toBe('https://gate.example.com/gate/callback');
  });

  it('strips a trailing slash from the base before appending the callback path', () => {
    render(
      buildConfig(),
      <RuntimeProvider url="https://gateway.example.com/">
        <GateCallbackUrl />
      </RuntimeProvider>,
    );

    expect(gateCallbackUrl()).toBe('https://gateway.example.com/gate/callback');
  });
});
