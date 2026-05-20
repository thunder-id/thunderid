/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
          <ReactQueryDevtools initialIsOpen={false} />
        </QueryClientProvider>
      </LoggerProvider>
    </ConfigProvider>
  </StrictMode>,
);
