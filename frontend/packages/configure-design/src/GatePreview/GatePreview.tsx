// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {getCspNonce, type ColorSchemeOption, type Stylesheet, type Theme} from '@thunderid/design';
import type {EmbeddedFlowComponent} from '@thunderid/react';
import {Box, CircularProgress, IconButton, Tooltip, Typography, useColorScheme} from '@wso2/oxygen-ui';
import {useCallback, useLayoutEffect, useRef, useState, type JSX, type ReactNode} from 'react';
import {createPortal} from 'react-dom';
import {useTranslation} from 'react-i18next';
import IframeContent from './IframeContent';
import buildPreviewMock from './mocks/buildPreviewMock';
import PreviewToolbar from '../components/PreviewToolbar';
import {VIEWPORT_WIDTHS, VIEWPORT_HEIGHTS} from '../components/viewportConstants';
import ColorSchemeOptions from '../constants/ColorSchemeOptions';

// ── Constants ────────────────────────────────────────────────────────────────

const ZOOM_STEPS = [25, 50, 75, 100, 125, 150];

/**
 * Neutral studio backdrop shown behind the preview card in 'minimal' toolbar mode, independent
 * of the previewed theme's own background — keeps the card as the only themed element, like a
 * product screenshot rather than a literal edge-to-edge simulation of the live page.
 */
const NEUTRAL_CANVAS_BACKGROUND: Record<'light' | 'dark', string> = {
  light: '#f4f4f5',
  dark: '#18181b',
};

/** Minimum width (px) the content needs so the 450px sign-in card + padding renders without clipping. */
const MIN_CONTENT_WIDTH = 520;

/** Minimum height (px) the content needs so a typical sign-in form renders without clipping. */
const MIN_CONTENT_HEIGHT = 700;

/**
 * Initial HTML written into the preview iframe. Sets up the full height chain so AuthPageLayout's
 * minHeight: 100% resolves correctly. Its <style> tag carries the parent document's CSP nonce: this
 * iframe has no src (an about:blank doc written to directly), so it inherits the parent's CSP,
 * including this exact nonce value, rather than carrying its own.
 */
function buildIframeInitialHTML(nonce: string | undefined): string {
  return [
    '<!DOCTYPE html><html style="height:100%"><head>',
    `<style${nonce ? ` nonce="${nonce}"` : ''}>*,*::before,*::after{box-sizing:border-box}body{margin:0;height:100%}#root,#root>*{height:100%}</style>`,
    '</head><body><div id="root"></div></body></html>',
  ].join('');
}

// ── Types & Props ────────────────────────────────────────────────────────────

export type Viewport = 'desktop' | 'tablet' | 'mobile';

export interface GatePreviewProps {
  /** The theme to render. Null shows a loading spinner; undefined shows an empty prompt. */
  theme: Theme | null | undefined;
  displayName?: string;
  showToolbar?: boolean;
  /**
   * 'full' renders the pill toolbar (viewport/zoom/color-scheme controls). 'minimal' renders just
   * a single floating icon button that cycles the color scheme, for full-bleed preview panels.
   * Defaults to 'full'.
   */
  toolbarVariant?: 'full' | 'minimal';
  /** Whether the toolbar shows the mobile/tablet/desktop viewport switcher. Defaults to true. */
  showViewportControls?: boolean;
  /** Whether the toolbar shows zoom in/out controls. Defaults to true. */
  showZoomControls?: boolean;
  viewport?: {
    width: string | number;
    height: string | number;
  };
  colorScheme?: ColorSchemeOption;
  /** When true, the preview tracks the host app's color scheme instead of the toolbar toggle. */
  syncColorSchemeWithSystem?: boolean;
  mock?: EmbeddedFlowComponent[];
  /** Optional page background CSS value (color, gradient, or image). Overrides theme background when set. */
  pageBackground?: string;
  /** Custom stylesheets to inject into the isolated preview iframe. */
  stylesheets?: Stylesheet[];
  /** When true, enables the element inspector overlay inside the preview. */
  inspectorEnabled?: boolean;
  /** Callback when a CSS selector is picked via the inspector. */
  onSelectSelector?: (selector: string) => void;
  /** Callback for action submissions inside the preview. Defaults to a no-op. */
  onSubmit?: (component: EmbeddedFlowComponent, data?: Record<string, unknown>) => void;
  /** Callback fired when the pointer enters (component) or leaves (null) a top-level component. */
  onComponentHover?: (component: EmbeddedFlowComponent | null) => void;
  /**
   * Additional runtime data handed to the flow component renderer, mirroring what
   * the gate receives during flow execution (e.g. `consentPrompt`).
   */
  additionalData?: Record<string, unknown>;
  /**
   * When true, hides the built-in browser chrome and insets so the preview fills
   * its container edge to edge. Useful when the host provides its own window chrome.
   */
  frameless?: boolean;
  /**
   * The device chrome drawn around the preview. `'browser'` (default) renders a desktop browser
   * window (traffic-light dots + fake address bar). `'phone'` renders a dark rounded phone bezel
   * with a status-bar notch instead, for previews of app-native (embedded) sign-in flows. Has no
   * effect when `frameless` is set.
   */
  frameStyle?: 'browser' | 'phone';
  /** Base theme the resolved design is merged over. Defaults to Acrylic Orange. */
  baseTheme?: Theme;
  /**
   * When true and no effective theme is configured, mirrors the gate's
   * design-disabled branding: particle background and the product logo.
   */
  themelessBranding?: boolean;
  /** Content rendered to the left of the toolbar (e.g. back button, title). */
  toolbarStart?: ReactNode;
  /** Content rendered inside the toolbar pill on the right (e.g. inspector toggle, theme selector). */
  toolbarEnd?: ReactNode;
  /**
   * When provided, the toolbar is portaled into this DOM element instead of being rendered inline.
   * The parent is responsible for rendering the container and passing it here.
   * Useful for placing the toolbar in a full-width top bar outside the preview area.
   */
  toolbarPortal?: HTMLElement | null;
  /**
   * Content rendered inside the preview frame, below the canvas (e.g. a step changer for
   * multi-screen onboarding previews). Rendered within the same bordered window as the
   * browser chrome, unlike toolbarStart/toolbarEnd which sit outside it.
   */
  footer?: ReactNode;
}

// ── Main component ───────────────────────────────────────────────────────────

export default function GatePreview({
  theme,
  displayName = '',
  showToolbar = true,
  toolbarVariant = 'full',
  showViewportControls = true,
  showZoomControls = true,
  viewport = undefined,
  mock = buildPreviewMock(),
  colorScheme = undefined,
  syncColorSchemeWithSystem = false,
  pageBackground = undefined,
  stylesheets = [],
  inspectorEnabled = false,
  onSelectSelector = undefined,
  onSubmit = undefined,
  onComponentHover = undefined,
  additionalData = undefined,
  frameless = false,
  frameStyle = 'browser',
  baseTheme = undefined,
  themelessBranding = false,
  toolbarStart = undefined,
  toolbarEnd = undefined,
  toolbarPortal = undefined,
  footer = undefined,
}: GatePreviewProps): JSX.Element {
  const {t} = useTranslation('design');
  const {mode, systemMode} = useColorScheme();
  const resolvedSystemMode: 'light' | 'dark' = (mode === 'system' ? systemMode : mode) === 'dark' ? 'dark' : 'light';
  const [previewEffectiveScheme, setPreviewEffectiveScheme] = useState<'light' | 'dark'>(resolvedSystemMode);
  const [minimalColorScheme, setMinimalColorScheme] = useState<ColorSchemeOption>('system');
  const [viewportState, setViewport] = useState<Viewport>('desktop');
  const [zoom, setZoom] = useState(75);
  const canvasRef = useRef<HTMLDivElement>(null);
  const iframeRef = useRef<HTMLIFrameElement | null>(null);
  const dimensionsRef = useRef<HTMLSpanElement>(null);
  const [iframeDoc, setIframeDoc] = useState<Document | null>(null);

  // Callback ref: initializes the iframe document whenever the <iframe> mounts.
  // This handles the case where theme starts as null (loading spinner), so the
  // iframe doesn't exist on first render — the callback fires when it appears.
  // We skip re-initialization if #root already exists (React Strict Mode calls
  // the callback ref twice; re-writing the document would destroy the portal
  // target without triggering a re-render since the doc reference is the same).
  const iframeCallbackRef = useCallback((iframe: HTMLIFrameElement | null) => {
    iframeRef.current = iframe;
    if (!iframe) return;
    const doc = iframe.contentDocument;
    if (!doc) return;
    if (doc.getElementById('root')) {
      setIframeDoc(doc);
      return;
    }
    doc.open();
    doc.write(buildIframeInitialHTML(getCspNonce()));
    doc.close();
    setIframeDoc(doc);
  }, []);

  const activeScheme = colorScheme !== 'system' ? colorScheme : undefined;
  let effectiveScheme: 'light' | 'dark';
  if (activeScheme) {
    effectiveScheme = activeScheme;
  } else if (syncColorSchemeWithSystem) {
    effectiveScheme = resolvedSystemMode;
  } else if (toolbarVariant === 'minimal') {
    effectiveScheme = minimalColorScheme === 'system' ? resolvedSystemMode : minimalColorScheme;
  } else {
    effectiveScheme = previewEffectiveScheme;
  }

  const handleCycleMinimalColorScheme = (): void => {
    const currentIdx = ColorSchemeOptions.findIndex((option) => option.id === minimalColorScheme);
    const next = ColorSchemeOptions[(currentIdx + 1) % ColorSchemeOptions.length];
    setMinimalColorScheme(next.id);
  };

  const resolvedPageBackground =
    pageBackground ?? (toolbarVariant === 'minimal' ? NEUTRAL_CANVAS_BACKGROUND[effectiveScheme] : undefined);

  const zoomIdx = ZOOM_STEPS.indexOf(zoom);

  // Imperatively size & scale the iframe to fit the canvas — no React state, no re-renders.
  useLayoutEffect(() => {
    const canvas = canvasRef.current;
    const iframe = iframeRef.current;
    if (!canvas || !iframe) return undefined;

    const update = (): void => {
      const cw = canvas.clientWidth;
      const ch = canvas.clientHeight;
      if (!cw || !ch) return;

      const userScale = zoom / 100;
      // Scale down to fit both dimensions so the card never clips.
      const fitScaleW = Math.min(1, cw / MIN_CONTENT_WIDTH);
      const fitScaleH = Math.min(1, ch / MIN_CONTENT_HEIGHT);
      const fitScale = Math.min(fitScaleW, fitScaleH);
      const totalScale = fitScale * userScale;

      // Inverse-scale: render iframe at (canvas / totalScale) so after
      // transform: scale(totalScale) it visually fills the canvas exactly.
      const iframeW = Math.round(cw / totalScale);
      const iframeH = Math.round(ch / totalScale);
      iframe.style.width = `${iframeW}px`;
      iframe.style.height = `${iframeH}px`;
      iframe.style.transform = `scale(${totalScale})`;

      // Update dimensions label without triggering a React re-render.
      if (dimensionsRef.current) {
        dimensionsRef.current.textContent = `${iframeW} × ${iframeH}`;
      }
    };

    const observer = new ResizeObserver(update);
    observer.observe(canvas);
    update();

    return () => observer.disconnect();
  }, [zoom, iframeDoc]);

  if (theme === null) {
    return (
      <Box sx={{height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center'}}>
        <CircularProgress size={32} />
      </Box>
    );
  }

  return (
    <Box sx={{height: '100%', display: 'flex', flexDirection: 'column', position: 'relative'}}>
      {/* Minimal toolbar — a single floating icon that cycles the color scheme, for full-bleed previews */}
      {showToolbar && toolbarVariant === 'minimal' && (
        <Tooltip title={t('common.preview.toolbar.actions.cycle_color_scheme.tooltip', 'Toggle color scheme')}>
          <IconButton
            onClick={handleCycleMinimalColorScheme}
            size="small"
            sx={{
              position: 'absolute',
              top: 16,
              right: 16,
              zIndex: 10,
              bgcolor: 'background.paper',
              boxShadow: '0 8px 32px rgba(0,0,0,0.18), 0 2px 8px rgba(0,0,0,0.08)',
              border: '1px solid',
              borderColor: 'divider',
              '&:hover': {bgcolor: 'action.hover'},
            }}
          >
            {ColorSchemeOptions.find((option) => option.id === minimalColorScheme)?.icon}
          </IconButton>
        </Tooltip>
      )}

      {/* Toolbar — portaled to external container when toolbarPortal is set, otherwise rendered inline */}
      {showToolbar &&
        toolbarVariant === 'full' &&
        (() => {
          const toolbar = (
            <PreviewToolbar
              viewport={viewportState}
              setViewport={setViewport}
              onEffectiveSchemeChange={setPreviewEffectiveScheme}
              zoom={zoom}
              setZoom={setZoom}
              zoomIdx={zoomIdx}
              extraContent={toolbarEnd}
              showViewportControls={showViewportControls}
              showZoomControls={showZoomControls}
            />
          );

          if (toolbarPortal) {
            return createPortal(toolbar, toolbarPortal);
          }

          return (
            <Box sx={{display: 'flex', alignItems: 'center', py: 1.5, flexShrink: 0, px: 1}}>
              {toolbarStart}
              <Box sx={{flex: 1, display: 'flex', justifyContent: 'center'}}>{toolbar}</Box>
            </Box>
          );
        })()}

      {/* Viewport container */}
      <Box
        sx={{
          flex: 1,
          overflow: 'hidden',
          display: 'flex',
          justifyContent: 'center',
          alignItems: 'center',
          p: frameless ? 0 : 2,
        }}
      >
        <Box
          sx={{
            backgroundColor:
              !frameless && frameStyle === 'phone'
                ? '#0a0a0a'
                : toolbarVariant === 'minimal'
                  ? resolvedPageBackground
                  : 'background.paper',
            borderRadius: frameless ? 0 : frameStyle === 'phone' ? '32px' : 1,
            p: !frameless && frameStyle === 'phone' ? '10px' : 0,
            width: frameless ? '100%' : (viewport?.width ?? VIEWPORT_WIDTHS[viewportState]),
            height: frameless ? '100%' : (viewport?.height ?? VIEWPORT_HEIGHTS[viewportState]),
            maxHeight: '100%',
            transition: 'width 0.2s ease, height 0.2s ease',
            display: 'flex',
            flexDirection: 'column',
          }}
        >
          {/* Browser chrome */}
          {!frameless && frameStyle === 'browser' && (
            <Box
              sx={{
                px: 3,
                py: 1.5,
                borderBottom: '1px solid',
                borderColor: 'divider',
                display: 'flex',
                alignItems: 'center',
                gap: 1,
                flexShrink: 0,
              }}
            >
              <Box sx={{width: 8, height: 8, borderRadius: '50%', bgcolor: '#fc5c57'}} />
              <Box sx={{width: 8, height: 8, borderRadius: '50%', bgcolor: '#febc2e'}} />
              <Box sx={{width: 8, height: 8, borderRadius: '50%', bgcolor: '#29c840'}} />
              <Box
                sx={{
                  flex: 1,
                  mx: 2,
                  height: 22,
                  bgcolor: 'action.hover',
                  borderRadius: 1,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                }}
              >
                <Typography variant="caption" color="text.disabled" sx={{fontSize: 10}}>
                  {displayName
                    ? t('themes.builder.preview.title_with_name', '{{name}} — Preview', {name: displayName})
                    : t('themes.builder.preview.label', 'Preview')}
                </Typography>
              </Box>
            </Box>
          )}

          {/* Phone chrome — a status-bar notch instead of a browser address bar */}
          {!frameless && frameStyle === 'phone' && (
            <Box sx={{display: 'flex', justifyContent: 'center', pb: 1, pt: 0.5, flexShrink: 0}}>
              <Box sx={{width: 56, height: 5, borderRadius: 3, bgcolor: 'rgba(255,255,255,0.25)'}} />
            </Box>
          )}

          {/* Canvas — fills the chrome frame like a real viewport */}
          <Box
            ref={canvasRef}
            sx={{
              flex: 1,
              overflow: 'hidden',
              position: 'relative',
              borderRadius: !frameless && frameStyle === 'phone' ? '22px' : 0,
            }}
          >
            {!frameless && (
              <Typography
                component="span"
                ref={dimensionsRef}
                variant="caption"
                sx={{
                  position: 'absolute',
                  top: 4,
                  right: 6,
                  zIndex: 1,
                  fontSize: 9,
                  fontFamily: 'monospace',
                  color: 'text.disabled',
                  opacity: 0.7,
                  pointerEvents: 'none',
                }}
              />
            )}
            <iframe
              ref={iframeCallbackRef}
              title={t('themes.builder.preview.iframe_title', 'Gate Preview')}
              style={{border: 'none', transformOrigin: 'top left', position: 'absolute', top: 0, left: 0}}
            />
            {iframeDoc?.getElementById('root') &&
              createPortal(
                <IframeContent
                  iframeDoc={iframeDoc}
                  colorScheme={effectiveScheme}
                  theme={theme}
                  stylesheets={stylesheets}
                  pageBackground={resolvedPageBackground}
                  mock={mock}
                  inspectorEnabled={inspectorEnabled}
                  onSelectSelector={onSelectSelector}
                  onSubmit={onSubmit}
                  onComponentHover={onComponentHover}
                  additionalData={additionalData}
                  baseTheme={baseTheme}
                  themelessBranding={themelessBranding}
                />,
                iframeDoc.getElementById('root')!,
              )}
          </Box>

          {footer && <Box sx={{borderTop: '1px solid', borderColor: 'divider', flexShrink: 0}}>{footer}</Box>}
        </Box>
      </Box>
    </Box>
  );
}
