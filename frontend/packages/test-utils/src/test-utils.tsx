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

/* eslint-disable react-refresh/only-export-components */

import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {
  render,
  renderHook as rtlRenderHook,
  type RenderOptions,
  type RenderHookOptions,
  type RenderResult,
} from '@testing-library/react';
import {ConfigProvider, ToastProvider} from '@thunderid/contexts';
import {LoggerProvider, LogLevel} from '@thunderid/logger';
import {OxygenUIThemeProvider} from '@wso2/oxygen-ui';
import {useMemo, type ReactElement, type ReactNode} from 'react';
import {MemoryRouter} from 'react-router';

/**
 * Configuration options for test utilities
 */
export interface ThunderTestConfig {
  /**
   * Base path for the application (e.g., '/console', '/gate')
   */
  base: string;
  /**
   * Client ID for the application
   */
  clientId: string;
  /**
   * Server hostname
   * @default 'localhost'
   */
  hostname?: string;
  /**
   * Server port
   * @default 8090
   */
  port?: number;
  /**
   * Whether to use HTTP only
   * @default false
   */
  httpOnly?: boolean;
}

interface ProvidersProps {
  children: ReactNode;
  queryClient?: QueryClient;
  config?: ThunderTestConfig;
}

// Default configuration for console (backwards compatibility)
const defaultConfig: ThunderTestConfig = {
  base: '/console',
  clientId: 'CONSOLE',
};

/**
 * The CSS class name prefix used by cn() during tests.
 * Import this instead of hardcoding the product name in test assertions.
 */
export const TEST_CN_PREFIX = 'ThunderID';

// Store the current config
let currentConfig: ThunderTestConfig = defaultConfig;

// Create a new QueryClient for each test to avoid shared state
function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
      mutations: {
        retry: false,
      },
    },
  });
}

// Wrapper component with common providers
function Providers({children, queryClient = undefined, config = undefined}: ProvidersProps) {
  const testConfig = config ?? currentConfig;

  // Setup window runtime configuration for tests
  if (typeof window !== 'undefined' && !window.__THUNDERID_RUNTIME_CONFIG__) {
    // eslint-disable-next-line react-hooks/immutability
    window.__THUNDERID_RUNTIME_CONFIG__ = {
      brand: {
        product_name: 'ThunderID',
        favicon: {light: 'assets/images/favicon.ico', dark: 'assets/images/favicon-inverted.ico'},
      },
      client: {
        base: testConfig.base,
        client_id: testConfig.clientId,
      },
      server: {
        hostname: testConfig.hostname ?? 'localhost',
        port: testConfig.port ?? 8090,
        http_only: testConfig.httpOnly ?? false,
      },
    };
  }

  // Use useMemo to ensure the default QueryClient is only created once per mount,
  // preventing cache reset on re-renders when queryClient prop is not provided
  const client = useMemo(() => queryClient ?? createTestQueryClient(), [queryClient]);

  return (
    <MemoryRouter>
      <QueryClientProvider client={client}>
        <ConfigProvider>
          <LoggerProvider
            logger={{
              level: LogLevel.ERROR,
              transports: [],
            }}
          >
            <ToastProvider>
              <OxygenUIThemeProvider>{children}</OxygenUIThemeProvider>
            </ToastProvider>
          </LoggerProvider>
        </ConfigProvider>
      </QueryClientProvider>
    </MemoryRouter>
  );
}

/**
 * Configure the test utilities with app-specific settings
 * Call this in your test setup file before running tests
 */
export function configureTestUtils(config: ThunderTestConfig): void {
  currentConfig = config;
}

// Custom render function that includes providers
function customRender(ui: ReactElement, options?: Omit<RenderOptions, 'wrapper'>): RenderResult {
  const wrapper = ({children}: {children: ReactNode}) => <Providers config={currentConfig}>{children}</Providers>;
  return render(ui, {wrapper, ...options});
}

/**
 * Alternative render function with providers
 * Alias for customRender to support different naming conventions
 */
export function renderWithProviders(ui: ReactElement, options?: RenderOptions): RenderResult {
  return customRender(ui, options ?? {});
}

interface RenderHookWithQueryClientOptions<Props> extends Omit<RenderHookOptions<Props>, 'wrapper'> {
  queryClient?: QueryClient;
}

/**
 * Custom renderHook function that includes providers
 * Wraps hooks with necessary context providers for testing
 * Optionally accepts a queryClient for tests that need direct access to manipulate cache or spy on methods
 * Returns the queryClient instance for convenience
 */
export function renderHook<Result, Props>(
  hook: (props: Props) => Result,
  options?: RenderHookWithQueryClientOptions<Props>,
) {
  const {queryClient: providedQueryClient, ...restOptions} = options ?? {};
  const queryClient = providedQueryClient ?? createTestQueryClient();

  const wrapper = ({children}: {children: ReactNode}) => (
    <Providers config={currentConfig} queryClient={queryClient}>
      {children}
    </Providers>
  );

  return {
    ...rtlRenderHook(hook, {wrapper, ...restOptions}),
    queryClient,
  };
}

/**
 * Helper to get element by translation key
 * Useful when using mocked translations that return keys
 */
export function getByTranslationKey(container: HTMLElement, key: string) {
  return (
    container.querySelector(`[data-testid="${key}"]`) ??
    Array.from(container.querySelectorAll('*')).find((el) => el.textContent === key)
  );
}

export default customRender;
