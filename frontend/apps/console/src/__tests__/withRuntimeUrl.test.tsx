// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {render, screen, waitFor} from '@testing-library/react';
import {ConfigProvider, useRuntimeUrl} from '@thunderid/contexts';
import type {ProductConfig} from '@thunderid/contexts';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import withRuntimeUrl from '../hocs/withRuntimeUrl';

const request = vi.fn();
let isSignedIn = true;

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request}, isSignedIn}),
}));

const config: ProductConfig = {
  brand: {
    product_name: 'Test Product',
    favicon: {light: 'assets/images/favicon.ico', dark: 'assets/images/favicon-inverted.ico'},
  },
  client: {base: '/console', client_id: 'CONSOLE'},
  server: {public_url: 'https://cp.example.com:8090'},
  mode: 'control_plane',
};

function TokenEndpoint() {
  return <span data-testid="token-endpoint">{`${useRuntimeUrl()}/oauth2/token`}</span>;
}

const WithRuntimeUrl = withRuntimeUrl(TokenEndpoint);

function tree(queryClient: QueryClient) {
  return (
    <QueryClientProvider client={queryClient}>
      <ConfigProvider>
        <WithRuntimeUrl />
      </ConfigProvider>
    </QueryClientProvider>
  );
}

function renderProvider(queryClient = new QueryClient({defaultOptions: {queries: {retry: false}}})) {
  return {queryClient, ...render(tree(queryClient))};
}

beforeEach(() => {
  window.__THUNDERID_RUNTIME_CONFIG__ = config;
  isSignedIn = true;
  request.mockReset();
});

afterEach(() => {
  window.__THUNDERID_RUNTIME_CONFIG__ = undefined;
});

describe('withRuntimeUrl', () => {
  it('builds runtime endpoints from the registered gateway rather than the console server', async () => {
    request.mockResolvedValue({
      data: [{id: 'gw-1', name: 'production', baseUrl: 'https://gateway.example.com:8090', isDefault: true}],
    });

    renderProvider();

    await waitFor(() => {
      expect(screen.getByTestId('token-endpoint').textContent).toBe('https://gateway.example.com:8090/oauth2/token');
    });
    expect(request).toHaveBeenCalledWith(expect.objectContaining({url: 'https://cp.example.com:8090/gateways'}));
  });

  // Only the default gateway is where a developer's application points, wherever it sits in the list.
  it('uses the default gateway rather than the first one registered', async () => {
    request.mockResolvedValue({
      data: [
        {id: 'gw-1', name: 'staging', baseUrl: 'https://staging.example.com:8090'},
        {id: 'gw-2', name: 'production', baseUrl: 'https://gateway.example.com:8090', isDefault: true},
      ],
    });

    renderProvider();

    await waitFor(() => {
      expect(screen.getByTestId('token-endpoint').textContent).toBe('https://gateway.example.com:8090/oauth2/token');
    });
  });

  // Gateways that only receive applied configuration are not a guess at the runtime.
  it('falls back to the server URL when no gateway is the default', async () => {
    request.mockResolvedValue({
      data: [{id: 'gw-1', name: 'staging', baseUrl: 'https://staging.example.com:8090'}],
    });

    renderProvider();

    await waitFor(() => {
      expect(request).toHaveBeenCalled();
    });
    expect(screen.getByTestId('token-endpoint').textContent).toBe('https://cp.example.com:8090/oauth2/token');
  });

  // A deployment that serves its own runtime has no gateways registered, and must keep showing
  // its own endpoints.
  it('falls back to the server URL when nothing is registered', async () => {
    request.mockResolvedValue({data: []});

    renderProvider();

    await waitFor(() => {
      expect(screen.getByTestId('token-endpoint').textContent).toBe('https://cp.example.com:8090/oauth2/token');
    });
  });

  // A deployment with no gateway API at all answers with a 404, which must read the same way.
  it('falls back to the server URL when the request fails', async () => {
    request.mockRejectedValue(new Error('404'));

    renderProvider();

    await waitFor(() => {
      expect(screen.getByTestId('token-endpoint').textContent).toBe('https://cp.example.com:8090/oauth2/token');
    });
  });

  // A standalone deployment serves its own runtime, so it has no gateway to look for.
  it('does not ask for gateways outside control-plane mode', async () => {
    window.__THUNDERID_RUNTIME_CONFIG__ = {...config, mode: 'standalone'};

    renderProvider();

    await waitFor(() => {
      expect(screen.getByTestId('token-endpoint').textContent).toBe('https://cp.example.com:8090/oauth2/token');
    });
    expect(request).not.toHaveBeenCalled();
  });

  it('does not ask for gateways before the user is signed in', async () => {
    isSignedIn = false;

    renderProvider();

    await waitFor(() => {
      expect(screen.getByTestId('token-endpoint').textContent).toBe('https://cp.example.com:8090/oauth2/token');
    });
    expect(request).not.toHaveBeenCalled();
  });

  // A failure is kept as an error rather than cached as an empty list, so the next mount asks again
  // and picks up a gateway that has since become reachable.
  it('asks again on the next mount after a failed request', async () => {
    request.mockRejectedValueOnce(new Error('503'));

    const {queryClient, unmount} = renderProvider();

    await waitFor(() => {
      expect(queryClient.getQueryState(['gateways'])?.status).toBe('error');
    });
    expect(queryClient.getQueryData(['gateways'])).toBeUndefined();
    unmount();

    request.mockResolvedValue({
      data: [{id: 'gw-1', name: 'production', baseUrl: 'https://gateway.example.com:8090', isDefault: true}],
    });
    renderProvider(queryClient);

    await waitFor(() => {
      expect(screen.getByTestId('token-endpoint').textContent).toBe('https://gateway.example.com:8090/oauth2/token');
    });
  });

  // The query client outlives a session, so what the previous session listed must not be shown
  // once it has signed out.
  it('drops the previous session gateways on sign-out', async () => {
    request.mockResolvedValue({
      data: [{id: 'gw-1', name: 'production', baseUrl: 'https://gateway.example.com:8090', isDefault: true}],
    });

    const {queryClient, rerender} = renderProvider();

    await waitFor(() => {
      expect(screen.getByTestId('token-endpoint').textContent).toBe('https://gateway.example.com:8090/oauth2/token');
    });

    isSignedIn = false;
    rerender(tree(queryClient));

    await waitFor(() => {
      expect(screen.getByTestId('token-endpoint').textContent).toBe('https://cp.example.com:8090/oauth2/token');
    });
    expect(queryClient.getQueryData(['gateways'])).toBeUndefined();
  });
});
