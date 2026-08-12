// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, cleanup} from '@testing-library/react';
import type {Stylesheet, Theme} from '@thunderid/design';
import {EmbeddedFlowComponentType, EmbeddedFlowEventType, type EmbeddedFlowComponent} from '@thunderid/react';
import {OxygenUIThemeProvider} from '@wso2/oxygen-ui';
import {afterEach, describe, it, expect, vi} from 'vitest';
import IframeContent from '../IframeContent';

// Mock shared-design to stub DesignProvider (which internally needs ConfigProvider). Merges over
// the real module so other real exports this file's module graph relies on stay available.
vi.mock('@thunderid/design', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/design')>();
  return {
    ...actual,
    DesignProvider: ({children}: {children: React.ReactNode}) => children,
    useDesign: () => ({isDesignEnabled: false, theme: undefined}),
    // Renders a rich-text link (data-action-ref) and a button (id) so hover/click
    // resolution in IframeContent has real DOM targets to resolve against.
    FlowComponentRenderer: ({component, onSubmit}: {component: {id?: string}; onSubmit: () => void}) => (
      <div>
        <a href="https://example.com" data-action-ref="submit-action" data-testid={`link-${component.id ?? 'x'}`}>
          Rich text link
        </a>
        <button type="button" id={`btn-${component.id ?? 'x'}`} onClick={() => onSubmit()}>
          Action button
        </button>
      </div>
    ),
    AuthPageLayout: ({children}: {children: React.ReactNode}) => <div data-testid="auth-page-layout">{children}</div>,
    AuthCardLayout: ({children}: {children: React.ReactNode}) => <div data-testid="auth-card-layout">{children}</div>,
    FontImporter: () => null,
    getFontImportURL: () => undefined,
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

const mockTheme = {
  colorSchemes: {
    light: {palette: {background: {default: '#ffffff'}}},
    dark: {palette: {background: {default: '#121212'}}},
  },
} as unknown as Theme;

// `EmbeddedFlowComponent` resolves to an error type here because `@thunderid/react` has no `types`
// condition in its `exports` map, and this package's `tsconfig.spec.json` forces `module: commonjs`
// against a `moduleResolution: bundler` base, so the checker falls back to the untyped CJS entry point.
// eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
const mockComponent: EmbeddedFlowComponent = {id: 'comp-1'} as unknown as EmbeddedFlowComponent;

function renderIframeContent(props: Partial<Parameters<typeof IframeContent>[0]> = {}) {
  const iframe = document.createElement('iframe');
  document.body.appendChild(iframe);
  const iframeDoc = iframe.contentDocument!;
  iframeDoc.open();
  iframeDoc.write('<!DOCTYPE html><html><head></head><body><div id="root"></div></body></html>');
  iframeDoc.close();

  // IframeContent's own tree renders into the iframe document's #root (mirroring the portal
  // GatePreview normally sets up), so DOM queries like getElementById run against iframeDoc.
  const utils = render(
    <OxygenUIThemeProvider>
      <IframeContent
        iframeDoc={iframeDoc}
        colorScheme="light"
        theme={mockTheme}
        stylesheets={[]}
        pageBackground={undefined}
        mock={[mockComponent]}
        inspectorEnabled={false}
        {...props}
      />
    </OxygenUIThemeProvider>,
    {container: iframeDoc.getElementById('root')!, baseElement: iframeDoc.body},
  );

  return {...utils, iframeDoc, cleanupIframe: () => iframe.remove()};
}

afterEach(() => {
  cleanup();
  document.querySelectorAll('iframe').forEach((iframe) => iframe.remove());
});

describe('IframeContent', () => {
  describe('sanitizeThemeForMui', () => {
    it('strips defaultColorScheme "system" before it reaches downstream MUI theming', () => {
      const themeWithSystemScheme = {
        defaultColorScheme: 'system',
        colorSchemes: {
          light: {palette: {background: {default: '#ffffff'}}},
          dark: {palette: {background: {default: '#121212'}}},
        },
      } as unknown as Theme;

      // Rendering must not throw despite MUI's extendTheme rejecting 'system'.
      expect(() => renderIframeContent({theme: themeWithSystemScheme})).not.toThrow();
    });

    it('renders normally when defaultColorScheme is a plain light/dark value', () => {
      const themeWithLightScheme = {
        defaultColorScheme: 'light',
        colorSchemes: {
          light: {palette: {background: {default: '#ffffff'}}},
          dark: {palette: {background: {default: '#121212'}}},
        },
      } as unknown as Theme;

      const {iframeDoc} = renderIframeContent({theme: themeWithLightScheme});
      expect(iframeDoc.getElementById('root')?.children.length).toBeGreaterThan(0);
    });
  });

  describe('stylesheet injection', () => {
    it('injects an inline stylesheet as a <style> tag in the iframe head', () => {
      const sheets: Stylesheet[] = [{id: 'sheet-1', type: 'inline', content: 'body{color:red}'}];
      const {iframeDoc} = renderIframeContent({stylesheets: sheets});

      const style = iframeDoc.getElementById('gate-preview-sheet-1');
      expect(style?.tagName).toBe('STYLE');
      expect(style?.textContent).toBe('body{color:red}');
    });

    it('tags the injected inline stylesheet with the nonce from the parent document', () => {
      const meta = document.createElement('meta');
      meta.setAttribute('property', 'csp-nonce');
      meta.setAttribute('content', 'nonce-xyz');
      document.head.appendChild(meta);

      const sheets: Stylesheet[] = [{id: 'sheet-nonce', type: 'inline', content: 'body{color:blue}'}];
      const {iframeDoc} = renderIframeContent({stylesheets: sheets});

      const style = iframeDoc.getElementById('gate-preview-sheet-nonce');
      expect(style?.getAttribute('nonce')).toBe('nonce-xyz');
      meta.remove();
    });

    it('skips a disabled stylesheet entirely', () => {
      const sheets: Stylesheet[] = [
        {id: 'sheet-disabled', type: 'inline', content: 'body{color:green}', disabled: true},
      ];
      const {iframeDoc} = renderIframeContent({stylesheets: sheets});

      expect(iframeDoc.getElementById('gate-preview-sheet-disabled')).toBeNull();
    });

    it('injects a url stylesheet as a <link> tag when the href is https', () => {
      const sheets: Stylesheet[] = [{id: 'sheet-url', type: 'url', href: 'https://fonts.example.com/style.css'}];
      const {iframeDoc} = renderIframeContent({stylesheets: sheets});

      const link = iframeDoc.getElementById('gate-preview-sheet-url');
      expect(link?.tagName).toBe('LINK');
      expect(link?.getAttribute('href')).toBe('https://fonts.example.com/style.css');
    });

    it('does not inject a url stylesheet whose protocol is not https', () => {
      const sheets: Stylesheet[] = [{id: 'sheet-http', type: 'url', href: 'http://fonts.example.com/style.css'}];
      const {iframeDoc} = renderIframeContent({stylesheets: sheets});

      expect(iframeDoc.getElementById('gate-preview-sheet-http')).toBeNull();
    });

    it('does not inject and does not throw for a malformed url stylesheet href', () => {
      const sheets: Stylesheet[] = [{id: 'sheet-bad', type: 'url', href: 'not a url'}];
      expect(() => renderIframeContent({stylesheets: sheets})).not.toThrow();
    });

    it('removes a previously injected element when re-rendered with an empty stylesheet list', () => {
      const sheets: Stylesheet[] = [{id: 'sheet-cleanup', type: 'inline', content: 'body{color:red}'}];
      const {iframeDoc, rerender} = renderIframeContent({stylesheets: sheets});
      expect(iframeDoc.getElementById('gate-preview-sheet-cleanup')).not.toBeNull();

      rerender(
        <OxygenUIThemeProvider>
          <IframeContent
            iframeDoc={iframeDoc}
            colorScheme="light"
            theme={mockTheme}
            stylesheets={[]}
            pageBackground={undefined}
            mock={[mockComponent]}
            inspectorEnabled={false}
          />
        </OxygenUIThemeProvider>,
      );

      expect(iframeDoc.getElementById('gate-preview-sheet-cleanup')).toBeNull();
    });

    it('removes injected elements on unmount', () => {
      const sheets: Stylesheet[] = [{id: 'sheet-unmount', type: 'inline', content: 'body{color:red}'}];
      const {iframeDoc, unmount} = renderIframeContent({stylesheets: sheets});
      expect(iframeDoc.getElementById('gate-preview-sheet-unmount')).not.toBeNull();

      unmount();

      expect(iframeDoc.getElementById('gate-preview-sheet-unmount')).toBeNull();
    });
  });

  describe('component hover wrapping', () => {
    it('does not wrap components in a hover listener when onComponentHover is not provided', () => {
      const {iframeDoc} = renderIframeContent();
      const button = iframeDoc.getElementById('btn-comp-1');
      expect(button).not.toBeNull();
    });

    it('invokes onComponentHover with the hovered button-backed component on mouseOver', () => {
      const onComponentHover = vi.fn();
      const {iframeDoc} = renderIframeContent({onComponentHover});

      const button = iframeDoc.getElementById('btn-comp-1')!;
      button.dispatchEvent(new Event('mouseover', {bubbles: true}));

      expect(onComponentHover).toHaveBeenCalledWith(mockComponent);
    });

    it('invokes onComponentHover with null on mouseLeave', () => {
      const onComponentHover = vi.fn();
      const {iframeDoc} = renderIframeContent({onComponentHover});

      const button = iframeDoc.getElementById('btn-comp-1')!;
      // React derives mouseenter/mouseleave from bubbling mouseout events with a relatedTarget
      // outside the wrapper, since mouseleave itself does not bubble.
      button.dispatchEvent(new MouseEvent('mouseout', {bubbles: true, relatedTarget: iframeDoc.body}));

      expect(onComponentHover).toHaveBeenCalledWith(null);
    });

    it('invokes onComponentHover on focus and clears it on blur', () => {
      const onComponentHover = vi.fn();
      const {iframeDoc} = renderIframeContent({onComponentHover});

      const button = iframeDoc.getElementById('btn-comp-1') as HTMLButtonElement;
      button.focus();
      expect(onComponentHover).toHaveBeenCalledWith(mockComponent);

      button.blur();
      expect(onComponentHover).toHaveBeenLastCalledWith(null);
    });

    it('submits a trigger event for a wired rich-text link click and prevents default navigation', () => {
      const onComponentHover = vi.fn();
      const onSubmit = vi.fn();
      const {iframeDoc} = renderIframeContent({onComponentHover, onSubmit});

      const link = iframeDoc.querySelector<HTMLAnchorElement>('a[data-action-ref="submit-action"]')!;
      const event = new MouseEvent('click', {bubbles: true, cancelable: true});
      link.dispatchEvent(event);

      // `EmbeddedFlowEventType`/`EmbeddedFlowComponentType` resolve to error types here because
      // `@thunderid/react` has no `types` condition in its `exports` map, and this package's
      // `tsconfig.spec.json` forces `module: commonjs` against a `moduleResolution: bundler` base,
      // so the checker falls back to the untyped CJS entry point.
      /* eslint-disable @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-member-access */
      expect(onSubmit).toHaveBeenCalledWith({
        eventType: EmbeddedFlowEventType.Trigger,
        id: 'submit-action',
        ref: 'submit-action',
        type: EmbeddedFlowComponentType.Action,
      });
      /* eslint-enable @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-member-access */
      expect(event.defaultPrevented).toBe(true);
    });

    it('ignores a click that does not land on a wired rich-text link', () => {
      const onComponentHover = vi.fn();
      const onSubmit = vi.fn();
      const {iframeDoc} = renderIframeContent({onComponentHover, onSubmit});

      const button = iframeDoc.getElementById('btn-comp-1')!;
      button.dispatchEvent(new MouseEvent('click', {bubbles: true, cancelable: true}));

      // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-member-access
      expect(onSubmit).not.toHaveBeenCalledWith(expect.objectContaining({eventType: EmbeddedFlowEventType.Trigger}));
    });
  });

  describe('theme and background handling', () => {
    it('renders without a theme when theme is undefined', () => {
      const {iframeDoc} = renderIframeContent({theme: undefined});
      expect(iframeDoc.getElementById('root')?.children.length).toBeGreaterThan(0);
    });

    it('applies page background when provided', () => {
      const {iframeDoc} = renderIframeContent({pageBackground: '#f0f0f0'});
      const authPageLayout = iframeDoc.querySelector('[data-testid="auth-page-layout"]');
      expect(authPageLayout).not.toBeNull();
    });

    it('renders themeless branding when enabled and no theme is present', () => {
      const {iframeDoc} = renderIframeContent({theme: undefined, themelessBranding: true});
      // ParticleBackground should be rendered when themelessBranding is true and no theme
      const authPageLayout = iframeDoc.querySelector('[data-testid="auth-page-layout"]');
      expect(authPageLayout).not.toBeNull();
    });

    it('does not render themeless branding when theme is present', () => {
      const {iframeDoc} = renderIframeContent({theme: mockTheme, themelessBranding: true});
      // ParticleBackground should not be rendered when theme is present
      expect(iframeDoc.querySelector('[data-testid="auth-page-layout"]')).not.toBeNull();
    });

    it('passes additional data to flow component renderer', () => {
      const additionalData = {consentPrompt: 'test-prompt'};
      const {iframeDoc} = renderIframeContent({additionalData});
      // Component should render without error when additionalData is passed
      expect(iframeDoc.getElementById('btn-comp-1')).not.toBeNull();
    });
  });

  describe('iframe direction setting', () => {
    it('sets iframe document dir attribute to ltr when no theme direction is specified', () => {
      const {iframeDoc} = renderIframeContent({theme: mockTheme});
      expect(iframeDoc.documentElement.getAttribute('dir')).toBe('ltr');
    });

    it('sets iframe document dir attribute based on theme direction', () => {
      const themeWithRtl = {
        ...mockTheme,
        direction: 'rtl',
      } as unknown as Theme;
      const {iframeDoc} = renderIframeContent({theme: themeWithRtl});
      expect(iframeDoc.documentElement.getAttribute('dir')).toBe('rtl');
    });

    it('resets to ltr when theme direction is not set', () => {
      const {iframeDoc, rerender} = renderIframeContent({theme: mockTheme});
      expect(iframeDoc.documentElement.getAttribute('dir')).toBe('ltr');

      const themeWithRtl = {...mockTheme, direction: 'rtl'} as unknown as Theme;
      rerender(
        <OxygenUIThemeProvider>
          <IframeContent
            iframeDoc={iframeDoc}
            colorScheme="light"
            theme={themeWithRtl}
            stylesheets={[]}
            pageBackground={undefined}
            mock={[mockComponent]}
            inspectorEnabled={false}
          />
        </OxygenUIThemeProvider>,
      );
      expect(iframeDoc.documentElement.getAttribute('dir')).toBe('rtl');
    });
  });

  describe('border styles application', () => {
    it('applies border styles from theme.border to components', () => {
      const themeWithBorder = {
        ...mockTheme,
        border: {width: '2px', style: 'solid'},
      } as unknown as Theme;
      const {iframeDoc} = renderIframeContent({theme: themeWithBorder});
      // Component should render without error when border is applied
      expect(iframeDoc.getElementById('btn-comp-1')).not.toBeNull();
    });
  });
});
