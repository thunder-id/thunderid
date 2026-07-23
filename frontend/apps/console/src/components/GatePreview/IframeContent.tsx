/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import createCache from '@emotion/cache';
import {CacheProvider} from '@emotion/react';
import {
  FlowComponentRenderer,
  DesignProvider,
  AuthPageLayout,
  AuthCardLayout,
  GoogleFontLoader,
  type Theme,
  type DesignResolveResponse,
  type Stylesheet,
} from '@thunderid/design';
import {useTemplateLiteralResolver} from '@thunderid/hooks';
import {EmbeddedFlowComponentType, EmbeddedFlowEventType, type EmbeddedFlowComponent} from '@thunderid/react';
import {TemplateLiteralType} from '@thunderid/utils';
import {Box, ParticleBackground, useTheme} from '@wso2/oxygen-ui';
import {useEffect, useMemo, type JSX} from 'react';
import PreviewThemeProvider from './PreviewThemeProvider';
import {resolveAnchorActionRef, resolveHoverTarget} from './richTextClickResolution';
import ElementInspector from '../../features/design/components/layouts/ElementInspector';

/**
 * MUI's extendTheme only accepts 'light' or 'dark' for defaultColorScheme.
 * Product stores 'system' as a runtime concept — strip it before passing to MUI.
 */
function sanitizeThemeForMui(theme: Theme): Theme {
  const raw = theme as unknown as Record<string, unknown>;
  if (raw.defaultColorScheme === 'system') {
    const copy = {...raw};
    delete copy.defaultColorScheme;
    return copy as unknown as Theme;
  }
  return theme;
}

/**
 * Applies the merged theme's default background to the iframe body — the same
 * effect the gate gets from its global CssBaseline. Uses the theme's CSS-var
 * reference so the background tracks the preview's light/dark scheme.
 */
function IframeBodyBackground({iframeDoc}: {iframeDoc: Document}): null {
  const muiTheme = useTheme();

  useEffect(() => {
    // Prefer the CSS-var reference (like MUI's CssBaseline does) so the
    // background resolves against the iframe's active light/dark scheme.
    const themed = muiTheme as {
      vars?: {palette?: {background?: {default?: string}}};
      palette?: {background?: {default?: string}};
    };
    const background = themed.vars?.palette?.background?.default ?? themed.palette?.background?.default;
    const {body} = iframeDoc;
    body.style.setProperty('background', background ?? '');
  }, [iframeDoc, muiTheme]);

  return null;
}

/** No-op handlers for preview mode — the form is purely visual. */
const noopSubmit = (): void => {
  /* no-op */
};
const noopInputChange = (): void => {
  /* no-op */
};

export interface IframeContentProps {
  iframeDoc: Document;
  colorScheme: 'light' | 'dark';
  theme: Theme | undefined;
  stylesheets: Stylesheet[];
  pageBackground: string | undefined;
  mock: EmbeddedFlowComponent[];
  inspectorEnabled: boolean;
  onSelectSelector?: (selector: string) => void;
  /** Callback for action submissions. Defaults to a no-op (purely visual preview). */
  onSubmit?: (component: EmbeddedFlowComponent, data?: Record<string, unknown>) => void;
  /** Callback fired when the pointer enters (component) or leaves (null) a top-level component. */
  onComponentHover?: (component: EmbeddedFlowComponent | null) => void;
  /**
   * Additional runtime data handed to the flow component renderer, mirroring what
   * the gate receives during flow execution (e.g. `consentPrompt`).
   */
  additionalData?: Record<string, unknown>;
  /** Base theme the resolved design is merged over. Defaults to Acrylic Orange. */
  baseTheme?: Theme;
  /**
   * When true and no effective theme is configured, mirrors the gate's
   * design-disabled branding: particle background and the product logo.
   */
  themelessBranding?: boolean;
}

/**
 * Wraps preview content with an emotion CacheProvider that injects MUI styles
 * into the iframe's <head>, and injects custom stylesheets into the iframe document.
 */
export default function IframeContent({
  iframeDoc,
  colorScheme,
  theme,
  stylesheets,
  pageBackground,
  mock,
  inspectorEnabled,
  onSelectSelector = undefined,
  onSubmit = undefined,
  onComponentHover = undefined,
  additionalData = undefined,
  baseTheme = undefined,
  themelessBranding = false,
}: IframeContentProps): JSX.Element {
  // Strip {{t(key)}} wrappers, returning the bare i18n key so the shared
  // adapters' own t() call performs the actual translation (avoids double-translation).
  const {resolveAll} = useTemplateLiteralResolver();
  const previewResolve = useMemo(
    () =>
      (template: string | undefined): string | undefined =>
        resolveAll(template, {[TemplateLiteralType.TRANSLATION]: (key: string) => key}),
    [resolveAll],
  );

  // Create an emotion cache that injects styles into the iframe's <head>.
  const cache = useMemo(() => createCache({key: 'preview', container: iframeDoc.head}), [iframeDoc]);

  // Whether an actual theme is configured — mirrors the gate's isDesignEnabled
  // check, where an empty theme object counts as "no design".
  const hasTheme = Boolean(theme && Object.keys(theme).length > 0);
  const showThemelessBranding = themelessBranding && !hasTheme;

  const themeTypography = (hasTheme ? theme : baseTheme)?.typography as {fontFamily?: string} | undefined;
  const fontFamily = themeTypography?.fontFamily;

  // Inject custom stylesheets into the iframe document (not the parent).
  const serializedSheets = JSON.stringify(stylesheets);
  useEffect(() => {
    const parsed: Stylesheet[] = JSON.parse(serializedSheets) as Stylesheet[];
    const injectedIds: string[] = [];

    parsed.forEach((sheet) => {
      const elementId = `gate-preview-${sheet.id}`;
      iframeDoc.getElementById(elementId)?.remove();

      if (sheet.disabled) return;

      if (sheet.type === 'inline') {
        const style = iframeDoc.createElement('style');
        style.id = elementId;
        style.textContent = sheet.content;
        iframeDoc.head.appendChild(style);
        injectedIds.push(elementId);
      } else if (sheet.type === 'url') {
        try {
          const url = new URL(sheet.href);
          if (url.protocol !== 'https:') return;
        } catch {
          return;
        }
        const link = iframeDoc.createElement('link');
        link.id = elementId;
        link.rel = 'stylesheet';
        link.href = sheet.href;
        iframeDoc.head.appendChild(link);
        injectedIds.push(elementId);
      }
    });

    return () => {
      injectedIds.forEach((id) => iframeDoc.getElementById(id)?.remove());
    };
  }, [iframeDoc, serializedSheets]);

  return (
    <CacheProvider value={cache}>
      <GoogleFontLoader fontFamily={fontFamily} targetDocument={iframeDoc} />
      <DesignProvider
        shouldResolveDesignInternally={false}
        design={hasTheme ? ({theme: sanitizeThemeForMui(theme!)} as DesignResolveResponse) : undefined}
      >
        <PreviewThemeProvider
          colorScheme={colorScheme}
          colorSchemeNode={iframeDoc.documentElement}
          baseTheme={baseTheme}
        >
          <IframeBodyBackground iframeDoc={iframeDoc} />
          <ElementInspector enabled={inspectorEnabled} onSelectSelector={onSelectSelector}>
            <AuthPageLayout isLoading={false} variant="SignIn" background={pageBackground}>
              {showThemelessBranding && <ParticleBackground opacity={0.5} />}
              <AuthCardLayout
                variant="SignInBox"
                showLogo={showThemelessBranding}
                logo={
                  showThemelessBranding
                    ? {
                        src: {
                          light: `${import.meta.env.BASE_URL}/assets/images/logo.svg`,
                          dark: `${import.meta.env.BASE_URL}/assets/images/logo-inverted.svg`,
                        },
                        alt: {light: '', dark: ''},
                      }
                    : undefined
                }
                logoDisplay={{xs: 'flex'}}
              >
                {/* Matches the gate's component container exactly (no centering) so
                    previews lay out pixel-identical to the real sign-in box. */}
                <Box sx={{display: 'flex', flexDirection: 'column', gap: 2}}>
                  {mock.map((component, index) => {
                    const renderer = (
                      <FlowComponentRenderer
                        key={component.id ?? index}
                        component={component}
                        index={index}
                        values={{}}
                        isLoading={false}
                        resolve={previewResolve}
                        onInputChange={noopInputChange}
                        onSubmit={onSubmit ?? noopSubmit}
                        additionalData={additionalData}
                      />
                    );

                    // Only wrap when a hover listener is provided so the default
                    // preview layout is untouched. mouseover/focus bubble from the
                    // component's controls, so the hover target resolves to the
                    // exact button under the pointer (or keyboard focus) when a
                    // block contains several actions.
                    return onComponentHover ? (
                      <Box
                        key={component.id ?? index}
                        sx={{display: 'contents'}}
                        onMouseOver={(event) => onComponentHover(resolveHoverTarget(component, event.target))}
                        onMouseLeave={() => onComponentHover(null)}
                        onFocus={(event) => onComponentHover(resolveHoverTarget(component, event.target))}
                        onBlur={() => onComponentHover(null)}
                        onClick={(event) => {
                          // Only handle wired rich-text links; buttons dispatch through
                          // their own handler. Prevent the decorative `href="#"` from
                          // scrolling the iframe.
                          const actionRef = resolveAnchorActionRef(event.target);
                          if (actionRef === null) {
                            return;
                          }
                          event.preventDefault();
                          onSubmit?.({
                            eventType: EmbeddedFlowEventType.Trigger,
                            id: actionRef,
                            ref: actionRef,
                            type: EmbeddedFlowComponentType.Action,
                          } as EmbeddedFlowComponent);
                        }}
                      >
                        {renderer}
                      </Box>
                    ) : (
                      renderer
                    );
                  })}
                </Box>
              </AuthCardLayout>
            </AuthPageLayout>
          </ElementInspector>
        </PreviewThemeProvider>
      </DesignProvider>
    </CacheProvider>
  );
}
