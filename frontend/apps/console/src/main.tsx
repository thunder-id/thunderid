// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/// <reference types="./vite-env.d.ts" />

import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {ReactQueryDevtools} from '@tanstack/react-query-devtools';
import {ConfigProvider} from '@thunderid/contexts';
import {LoggerProvider, LogLevel} from '@thunderid/logger/react';
import {setCnPrefix} from '@thunderid/utils';
import {StrictMode} from 'react';
import * as ReactDOM from 'react-dom/client';
import AppWithDecorators from './AppWithDecorators';

// Initialize the class name prefix from runtime config (e.g., "<PRODUCT_NAME>" -> "<PRODUCT_NAME>SignIn--root")
if (typeof window !== 'undefined') {
  setCnPrefix(window.__THUNDERID_RUNTIME_CONFIG__?.brand?.product_name ?? '');
}

// Seed the runtime config with the dev backend and gate URLs when unset (dev only).
if (import.meta.env.DEV && typeof window !== 'undefined') {
  const runtimeConfig = window.__THUNDERID_RUNTIME_CONFIG__;
  if (runtimeConfig && !runtimeConfig.server) {
    runtimeConfig.server = {public_url: __DEV_SERVER_URL__};
  }
  if (runtimeConfig && !runtimeConfig.gate_client) {
    runtimeConfig.gate_client = {public_url: __DEV_GATE_URL__};
  }
}

const queryClient: QueryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: (failureCount, error) => {
        const status = (error as {response?: {status?: number}})?.response?.status;
        if (status && status >= 400 && status < 500) return false;

        return failureCount < 3;
      },
    },
  },
});

ReactDOM.createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ConfigProvider>
      <LoggerProvider
        logger={{
          level: import.meta.env.DEV ? LogLevel.DEBUG : LogLevel.INFO,
        }}
      >
        <QueryClientProvider client={queryClient}>
          <AppWithDecorators />
          {import.meta.env.DEV && <ReactQueryDevtools initialIsOpen={false} />}
        </QueryClientProvider>
      </LoggerProvider>
    </ConfigProvider>
  </StrictMode>,
);

