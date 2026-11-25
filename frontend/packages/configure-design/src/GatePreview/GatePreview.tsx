// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {getCspNonce, type ColorSchemeOption, type Stylesheet, type Theme} from '@thunderid/design';
import type {EmbeddedFlowComponent} from '@thunderid/react';
import {Box, CircularProgress, IconButton, Tooltip, useColorScheme} from '@wso2/oxygen-ui';
import {useCallback, useRef, useState, type JSX, type ReactNode} from 'react';
import {createPortal} from 'react-dom';
import {useTranslation} from 'react-i18next';
import IframeContent from './IframeContent';
import buildPreviewMock from './mocks/buildPreviewMock';
import PreviewFrame from '../components/PreviewFrame';
import PreviewToolbar from '../components/PreviewToolbar';
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
  /**
   * The viewport preset to show, for a host that renders its own toolbar (with `showToolbar`
   * off). Defaults to the toolbar's own choice.
   */
  viewportPreset?: Viewport;
  /** The zoom in percent, for a host that renders its own toolbar. Defaults to the toolbar's own choice. */
  zoom?: number;
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
  viewportPreset = undefined,
  zoom: controlledZoom = undefined,
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
  const [ownViewport, setViewport] = useState<Viewport>('desktop');
  const [ownZoom, setZoom] = useState(75);
  const viewportState: Viewport = viewportPreset ?? ownViewport;
  const zoom: number = controlledZoom ?? ownZoom;
  const iframeRef = useRef<HTMLIFrameElement | null>(null);
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

      <PreviewFrame
        label={
          displayName
            ? t('themes.builder.preview.title_with_name', '{{name}} — preview', {name: displayName})
            : t('themes.builder.preview.label', 'Preview')
        }
        viewport={viewportState}
        zoom={zoom}
        stageRef={iframeRef}
        stageKey={iframeDoc}
        size={viewport}
        frameless={frameless}
        frameStyle={frameStyle}
        frameBackground={toolbarVariant === 'minimal' ? resolvedPageBackground : undefined}
        footer={footer}
      >
        <iframe
          ref={iframeCallbackRef}
          title={t('themes.builder.preview.iframe_title', 'Gate preview')}
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
      </PreviewFrame>
    </Box>
  );
}
