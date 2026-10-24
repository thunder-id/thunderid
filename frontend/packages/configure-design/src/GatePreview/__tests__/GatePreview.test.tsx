// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type {Theme} from '@thunderid/design';
import {OxygenUIThemeProvider} from '@wso2/oxygen-ui';
import {describe, it, expect, vi, afterEach} from 'vitest';
import GatePreview from '../GatePreview';

// Mock shared-design to stub DesignProvider (which internally needs ConfigProvider). Merges over
// the real module (rather than fully replacing it) so other exports real code in this package's
// module graph relies on (e.g. useGetLayouts, pulled in via DesignPage/LayoutBuilderPage) stay available.
vi.mock('@thunderid/design', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/design')>();
  return {
    ...actual,
    DesignProvider: ({children}: {children: React.ReactNode}) => children,
    useDesign: () => ({isDesignEnabled: false, theme: undefined}),
    FlowComponentRenderer: () => <div data-testid="flow-component" />,
    AuthPageLayout: ({children}: {children: React.ReactNode}) => <div data-testid="auth-page-layout">{children}</div>,
    AuthCardLayout: ({children}: {children: React.ReactNode}) => <div data-testid="auth-card-layout">{children}</div>,
    FontImporter: () => null,
    getFontImportURL: () => undefined,
    AcrylicOrangeTheme: {},
    getCspNonce: () => document.querySelector('meta[property="csp-nonce"]')?.getAttribute('content') ?? undefined,
  };
});

vi.mock('@emotion/cache', async (importActual) => {
  const actual = await importActual<typeof import('@emotion/cache')>();
  return {
    ...actual,
    default: (options: Parameters<typeof actual.default>[0]) => actual.default({...options, container: document.head}),
  };
});

vi.mock('@emotion/react', () => ({
  CacheProvider: ({children}: {children: React.ReactNode}) => children,
}));

// A minimal valid theme for rendering the preview (cast to avoid full type scaffolding)
const mockTheme = {
  colorSchemes: {
    light: {palette: {background: {default: '#ffffff'}}},
    dark: {palette: {background: {default: '#121212'}}},
  },
} as unknown as Theme;

function renderWithThemeProvider(ui: React.ReactElement) {
  return render(<OxygenUIThemeProvider>{ui}</OxygenUIThemeProvider>);
}

describe('GatePreview', () => {
  describe('Loading state', () => {
    it('should render a CircularProgress spinner when theme is null', () => {
      renderWithThemeProvider(<GatePreview theme={null} />);

      expect(screen.getByRole('progressbar')).toBeInTheDocument();
    });

    it('should not render the preview canvas when theme is null', () => {
      renderWithThemeProvider(<GatePreview theme={null} />);

      // When loading, the iframe/canvas area is not rendered
      expect(screen.queryByTitle('Gate preview')).not.toBeInTheDocument();
    });
  });

  describe('Rendering with a valid theme', () => {
    it('should render the preview iframe when a theme is provided', () => {
      renderWithThemeProvider(<GatePreview theme={mockTheme} />);

      expect(screen.getByTitle('Gate preview')).toBeInTheDocument();
    });

    it('should not show a progress spinner when theme is provided', () => {
      renderWithThemeProvider(<GatePreview theme={mockTheme} />);

      expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
    });
  });

  describe('Toolbar visibility', () => {
    it('should render toolbar viewport controls by default (showToolbar=true)', () => {
      renderWithThemeProvider(<GatePreview theme={mockTheme} />);

      // PreviewToolbar contains icon buttons for mobile/tablet/desktop viewports
      const buttons = screen.getAllByRole('button');
      expect(buttons.length).toBeGreaterThan(0);
    });

    it('should not render toolbar buttons when showToolbar is false', () => {
      renderWithThemeProvider(<GatePreview theme={mockTheme} showToolbar={false} />);

      const buttons = screen.queryAllByRole('button');
      expect(buttons).toHaveLength(0);
    });
  });

  describe('Display name', () => {
    it('should show "Preview" in the browser chrome when displayName is not set', () => {
      renderWithThemeProvider(<GatePreview theme={mockTheme} displayName="" />);

      expect(screen.getByText('Preview')).toBeInTheDocument();
    });

    it('should include displayName and "Preview" in the browser chrome', () => {
      renderWithThemeProvider(<GatePreview theme={mockTheme} displayName="My App" />);

      expect(screen.getByText('My App — preview')).toBeInTheDocument();
    });
  });

  describe('Color scheme', () => {
    it('should render without errors when colorScheme is explicitly set to light', () => {
      renderWithThemeProvider(<GatePreview theme={mockTheme} colorScheme="light" />);

      expect(screen.getByTitle('Gate preview')).toBeInTheDocument();
    });

    it('should render without errors when colorScheme is explicitly set to dark', () => {
      renderWithThemeProvider(<GatePreview theme={mockTheme} colorScheme="dark" />);

      expect(screen.getByTitle('Gate preview')).toBeInTheDocument();
    });

    it('should render without errors when syncColorSchemeWithSystem is true', () => {
      renderWithThemeProvider(<GatePreview theme={mockTheme} syncColorSchemeWithSystem />);

      expect(screen.getByTitle('Gate preview')).toBeInTheDocument();
    });
  });

  describe('CSP nonce', () => {
    afterEach(() => {
      document.querySelector('meta[property="csp-nonce"]')?.remove();
    });

    it("tags the iframe's initial inline <style> with the parent document's csp nonce", () => {
      const meta = document.createElement('meta');
      meta.setAttribute('property', 'csp-nonce');
      meta.setAttribute('content', 'abc123');
      document.head.appendChild(meta);

      renderWithThemeProvider(<GatePreview theme={mockTheme} />);

      const iframe = screen.getByTitle<HTMLIFrameElement>('Gate preview');
      const style = iframe.contentDocument?.querySelector('style');
      expect(style?.getAttribute('nonce')).toBe('abc123');
    });

    it('renders the iframe without a nonce attribute when none is available', () => {
      renderWithThemeProvider(<GatePreview theme={mockTheme} />);

      const iframe = screen.getByTitle<HTMLIFrameElement>('Gate preview');
      const style = iframe.contentDocument?.querySelector('style');
      expect(style?.hasAttribute('nonce')).toBe(false);
    });
  });

  describe('Toolbar interactions', () => {
    it('should not crash when a viewport toggle button is clicked', async () => {
      renderWithThemeProvider(<GatePreview theme={mockTheme} />);

      const buttons = screen.getAllByRole('button');
      // Click each toolbar button to exercise viewport and zoom controls
      await Promise.all(
        buttons.map((button) =>
          userEvent.click(button).catch(() => {
            // Some buttons may be disabled; ignore errors
          }),
        ),
      );

      // Preview iframe is still rendered after interactions
      expect(screen.getByTitle('Gate preview')).toBeInTheDocument();
    });
  });
});
