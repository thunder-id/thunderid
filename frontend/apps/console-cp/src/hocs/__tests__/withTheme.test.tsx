// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';

const mockUseConfig = vi.hoisted(() => vi.fn());
vi.mock('@thunderid/contexts', () => ({
  useConfig: mockUseConfig,
}));

vi.mock('../../components/Head', () => ({
  default: () => <div data-testid="head" />,
}));

let capturedThemes: unknown;
let capturedInitialTheme: unknown;
let capturedNonce: unknown;

function MockChild() {
  return <div data-testid="mock-child">Child</div>;
}

vi.mock('@wso2/oxygen-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui')>();
  return {
    ...actual,
    createOxygenTheme: actual.createOxygenTheme ?? ((theme: unknown) => theme),
    HighContrastTheme: actual.HighContrastTheme ?? {},
    OxygenUIThemeProvider: ({
      children,
      themes = undefined,
      initialTheme = undefined,
      nonce = undefined,
    }: {
      children: React.ReactNode;
      themes?: unknown;
      initialTheme?: unknown;
      nonce?: unknown;
    }) => {
      capturedThemes = themes;
      capturedInitialTheme = initialTheme;
      capturedNonce = nonce;
      return <div data-testid="theme-provider">{children}</div>;
    },
  };
});

const {default: withTheme} = await import('../withTheme');
const WithThemeComponent = withTheme(MockChild);

describe('withTheme (console)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    capturedThemes = undefined;
    capturedInitialTheme = undefined;
    capturedNonce = undefined;
    document.querySelector('meta[property="csp-nonce"]')?.remove();
    mockUseConfig.mockReturnValue({
      config: {
        brand: {
          product_name: 'ThunderID',
          favicon: {light: 'assets/images/favicon.ico', dark: 'assets/images/favicon-inverted.ico'},
        },
      },
    });
  });

  it('renders without crashing', () => {
    const {container} = render(<WithThemeComponent />);
    expect(container).toBeInTheDocument();
  });

  it('renders the wrapped component', () => {
    render(<WithThemeComponent />);
    expect(screen.getByTestId('mock-child')).toBeInTheDocument();
  });

  it('wraps with OxygenUIThemeProvider', () => {
    render(<WithThemeComponent />);
    expect(screen.getByTestId('theme-provider')).toBeInTheDocument();
  });

  it('provides the expected theme collection', () => {
    render(<WithThemeComponent />);
    expect(Array.isArray(capturedThemes)).toBe(true);
    expect(capturedThemes).toEqual(
      expect.arrayContaining([
        expect.objectContaining({key: 'highContrast'}),
        expect.objectContaining({key: 'default'}),
      ]),
    );
    expect(capturedInitialTheme).toBe('default');
  });

  it('wraps different components correctly', () => {
    function AnotherChild() {
      return <div data-testid="another-child">Another</div>;
    }
    const AnotherWrapped = withTheme(AnotherChild);

    render(<AnotherWrapped />);
    expect(screen.getByTestId('another-child')).toBeInTheDocument();
    expect(screen.getByTestId('theme-provider')).toBeInTheDocument();
  });

  it('passes props through to the wrapped component', () => {
    function PropsChild({label}: {label: string}) {
      return <div data-testid="props-child">{label}</div>;
    }
    const WrappedWithProps = withTheme(PropsChild);

    render(<WrappedWithProps label="test-label" />);
    expect(screen.getByTestId('props-child')).toHaveTextContent('test-label');
  });

  it('renders Head', () => {
    render(<WithThemeComponent />);
    expect(screen.getByTestId('head')).toBeInTheDocument();
  });

  it('passes undefined nonce to OxygenUIThemeProvider when no csp-nonce meta tag is present', () => {
    render(<WithThemeComponent />);
    expect(capturedNonce).toBeUndefined();
  });

  it("forwards the page's csp-nonce meta tag to OxygenUIThemeProvider", () => {
    const meta = document.createElement('meta');
    meta.setAttribute('property', 'csp-nonce');
    meta.setAttribute('content', 'abc123');
    document.head.appendChild(meta);

    render(<WithThemeComponent />);
    expect(capturedNonce).toBe('abc123');
  });

  it('includes custom object themes from config in the theme list', () => {
    const objectTheme = {palette: {primary: {main: '#aabbcc'}}};
    mockUseConfig.mockReturnValue({
      config: {
        brand: {
          design: {
            themes: [{key: 'custom', label: 'Custom Theme', theme: objectTheme}],
          },
        },
      },
    });

    render(<WithThemeComponent />);
    expect(capturedThemes).toEqual(
      expect.arrayContaining([expect.objectContaining({key: 'custom', label: 'Custom Theme'})]),
    );
  });

  it('includes custom string themes from config in the theme list', () => {
    mockUseConfig.mockReturnValue({
      config: {
        brand: {
          design: {
            themes: [{key: 'external', label: 'External Theme', theme: 'https://example.com/theme.json'}],
          },
        },
      },
    });

    render(<WithThemeComponent />);
    expect(capturedThemes).toEqual(
      expect.arrayContaining([
        expect.objectContaining({key: 'external', label: 'External Theme', theme: 'https://example.com/theme.json'}),
      ]),
    );
  });
});
