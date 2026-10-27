// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/// <reference types="./vite-env.d.ts" />

import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {ConfigProvider} from '@thunderid/contexts';
import {LoggerProvider, LogLevel} from '@thunderid/logger/react';
import {setCnPrefix} from '@thunderid/utils';
import {StrictMode} from 'react';
import * as ReactDOM from 'react-dom/client';
import AppWithDecorators from './AppWithDecorators';

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

const queryClient: QueryClient = new QueryClient();

ReactDOM.createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ConfigProvider>
      <LoggerProvider logger={{level: import.meta.env.DEV ? LogLevel.DEBUG : LogLevel.INFO}}>
        <QueryClientProvider client={queryClient}>
          <AppWithDecorators />
        </QueryClientProvider>
      </LoggerProvider>
    </ConfigProvider>
  </StrictMode>,
);
