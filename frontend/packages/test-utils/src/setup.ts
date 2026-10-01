// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import '@testing-library/jest-dom/vitest';
import {cleanup} from '@testing-library/react';
import enUS from '@thunderid/i18n/locales/en-US';
import i18n from 'i18next';
import {afterEach, beforeAll, vi} from 'vitest';

// Initialize i18n for tests
// `react-i18next` is imported lazily so a test file's `vi.mock('react-i18next')` still takes effect in
// browser mode, where a module already evaluated by this setup file can't be swapped for its mock.
beforeAll(async () => {
  const reactI18next = await import('react-i18next');
  // A test file's react-i18next mock may leave out or stub the plugin, in which case there's nothing to bind.
  const plugin: Partial<typeof reactI18next.initReactI18next> | undefined =
    'initReactI18next' in reactI18next ? reactI18next.initReactI18next : undefined;
  if (plugin?.type) {
    i18n.use(reactI18next.initReactI18next);
  }

  await i18n.init({
    resources: {
      'en-US': {
        ...enUS,
      },
    },
    lng: 'en-US',
    fallbackLng: 'en-US',
    defaultNS: 'common',
    keySeparator: false,
    interpolation: {
      escapeValue: false,
    },
    // Disable Suspense in tests for faster execution
    react: {
      useSuspense: false,
    },
  });
});

// Cleanup after each test
afterEach(() => {
  cleanup();
});

// Patch CSSStyleDeclaration.setProperty to handle cssstyle errors with CSS variables in shorthand properties
// This is a known issue with jsdom/cssstyle when using CSS variables like `var(--rowBorderColor)` in border shorthand
// eslint-disable-next-line @typescript-eslint/unbound-method
const originalSetProperty = window.CSSStyleDeclaration.prototype.setProperty;

window.CSSStyleDeclaration.prototype.setProperty = function (
  this: CSSStyleDeclaration,
  property: string,
  value: string | null,
  priority?: string,
) {
  try {
    originalSetProperty.call(this, property, value, priority ?? '');
  } catch {
    // Silently ignore cssstyle errors for CSS variables in shorthand properties
  }
};

// Mock HTMLMediaElement methods that don't exist in jsdom
Object.defineProperty(window.HTMLMediaElement.prototype, 'play', {
  configurable: true,
  value: () => Promise.resolve(),
});

Object.defineProperty(window.HTMLMediaElement.prototype, 'pause', {
  configurable: true,
  value: () => {
    // Intentionally empty
  },
});

Object.defineProperty(window.HTMLMediaElement.prototype, 'load', {
  configurable: true,
  value: () => {
    // Intentionally empty
  },
});

// Mock IntersectionObserver
globalThis.IntersectionObserver = class IntersectionObserver {
  readonly root = null;

  readonly rootMargin = '';

  readonly thresholds = [];

  observe() {
    return this;
  }

  disconnect() {
    return this;
  }

  unobserve() {
    return this;
  }

  takeRecords() {
    return [];
  }
} as unknown as typeof IntersectionObserver;

// Mock ResizeObserver
globalThis.ResizeObserver = class ResizeObserver {
  observe() {
    return this;
  }

  disconnect() {
    return this;
  }

  unobserve() {
    return this;
  }
} as unknown as typeof ResizeObserver;

// Mock global for Node.js built-ins used by @thunderid packages
if (typeof window !== 'undefined') {
  (window as unknown as {global: Window}).global = window;
}

// Mock @thunderid/react to avoid buffer import issues in tests
vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal();
  return {
    ...(actual as object),
    useThunderID: vi.fn(() => ({
      http: {
        request: vi.fn(),
      },
      signIn: vi.fn(),
      signOut: vi.fn(),
      getAccessToken: vi.fn(),
      getIDToken: vi.fn(),
      getDecodedIDToken: vi.fn(),
      isAuthenticated: false,
      isLoading: false,
    })),
    ThunderIDProvider: ({children}: {children: React.ReactNode}) => children,
    InviteUser: ({children}: {children: (props: unknown) => React.ReactNode}) =>
      children({
        components: [],
        values: {},
        fieldErrors: {},
        touched: {},
        error: null,
        isLoading: false,
        handleInputChange: vi.fn(),
        handleInputBlur: vi.fn(),
        handleSubmit: vi.fn().mockResolvedValue(undefined),
        resetFlow: vi.fn(),
        isValid: false,
        meta: null,
        additionalData: undefined,
      }),
  };
});

// Mock MUI transition components to prevent RAF-based animation hangs in tests.
vi.mock('@mui/material/Fade', () => ({
  default: ({children, in: inProp}: {children: React.ReactNode; in?: boolean}) => (inProp !== false ? children : null),
}));

vi.mock('@mui/material/Grow', () => ({
  default: ({children, in: inProp}: {children: React.ReactNode; in?: boolean}) => (inProp !== false ? children : null),
}));

vi.mock('@mui/material/Collapse', () => ({
  default: ({children, in: inProp}: {children: React.ReactNode; in?: boolean}) => (inProp !== false ? children : null),
}));

vi.mock('@mui/material/Slide', () => ({
  default: ({children, in: inProp}: {children: React.ReactNode; in?: boolean}) => (inProp !== false ? children : null),
}));

vi.mock('@mui/material/Zoom', () => ({
  default: ({children, in: inProp}: {children: React.ReactNode; in?: boolean}) => (inProp !== false ? children : null),
}));
