// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ProductConfig} from '@thunderid/contexts';
import {describe, it, expect, vi} from 'vitest';

// Mock ReactDOM before importing main
const mockRender = vi.fn();
const mockUnmount = vi.fn();
const mockCreateRoot = vi.fn(() => ({
  render: mockRender,
  unmount: mockUnmount,
}));

vi.mock('react-dom/client', () => ({
  default: {
    createRoot: mockCreateRoot,
  },
  createRoot: mockCreateRoot,
}));

// Mock ConfigProvider
vi.mock('@thunderid/contexts', () => ({
  ConfigProvider: ({children}: {children: React.ReactNode}) => <div data-testid="config-provider">{children}</div>,
}));

// Mock AppWithDecorators
vi.mock('../AppWithDecorators', () => ({
  default: () => <div data-testid="app-with-decorators">AppWithDecorators</div>,
}));

describe('main', () => {
  it('imports without errors', async () => {
    // Set up DOM
    document.body.innerHTML = '<div id="root"></div>';

    // Import main module
    await import('../main');

    // Verify createRoot was called
    expect(mockCreateRoot).toHaveBeenCalled();
  });

  it('calls render on the root', async () => {
    // Set up DOM
    document.body.innerHTML = '<div id="root"></div>';

    // Import main module
    await import('../main');

    // Verify render was called
    expect(mockRender).toHaveBeenCalled();
  });

  it('seeds the dev server and gate URLs on the runtime config when unset', async () => {
    document.body.innerHTML = '<div id="root"></div>';

    const previous = window.__THUNDERID_RUNTIME_CONFIG__;
    window.__THUNDERID_RUNTIME_CONFIG__ = {
      brand: {product_name: 'ThunderID', favicon: {light: '', dark: ''}},
      client: {base: '/console', client_id: 'CONSOLE'},
    } as ProductConfig;

    vi.resetModules();
    await import('../main');

    expect(window.__THUNDERID_RUNTIME_CONFIG__?.server).toEqual({public_url: __DEV_SERVER_URL__});
    expect(window.__THUNDERID_RUNTIME_CONFIG__?.gate_client).toEqual({public_url: __DEV_GATE_URL__});

    window.__THUNDERID_RUNTIME_CONFIG__ = previous;
  });
});
