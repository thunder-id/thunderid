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

import type {ColorSchemeOption, Stylesheet, Theme} from '@thunderid/design';
import type {EmbeddedFlowComponent} from '@thunderid/react';
import {Box, CircularProgress, Typography, useColorScheme} from '@wso2/oxygen-ui';
import {useCallback, useLayoutEffect, useRef, useState, type JSX, type ReactNode} from 'react';
import {createPortal} from 'react-dom';
import {useTranslation} from 'react-i18next';
import IframeContent from './IframeContent';
import buildPreviewMock from './mocks/buildPreviewMock';
import PreviewToolbar from '../../features/design/components/PreviewToolbar';
import {VIEWPORT_WIDTHS, VIEWPORT_HEIGHTS} from '../../features/design/components/viewportConstants';

// ── Constants ────────────────────────────────────────────────────────────────

const ZOOM_STEPS = [25, 50, 75, 100, 125, 150];

/** Minimum width (px) the content needs so the 450px sign-in card + padding renders without clipping. */
const MIN_CONTENT_WIDTH = 520;

/** Minimum height (px) the content needs so a typical sign-in form renders without clipping. */
const MIN_CONTENT_HEIGHT = 700;

/**
 * Initial HTML written into the preview iframe. Sets up the full height chain
 * so AuthPageLayout's minHeight: 100% resolves correctly.
 */
const IFRAME_INITIAL_HTML = [
  '<!DOCTYPE html><html style="height:100%"><head>',
  '<link rel="preconnect" href="https://fonts.googleapis.com">',
  '<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>',
  '<style>body{margin:0;height:100%}#root,#root>*{height:100%}</style>',
  '</head><body><div id="root"></div></body></html>',
].join('');

// ── Types & Props ────────────────────────────────────────────────────────────

export type Viewport = 'desktop' | 'tablet' | 'mobile';

export interface GatePreviewProps {
  /** The theme to render. Null shows a loading spinner; undefined shows an empty prompt. */
  theme: Theme | null | undefined;
  displayName?: string;
  showToolbar?: boolean;
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
}

// ── Main component ───────────────────────────────────────────────────────────

export default function GatePreview({
  theme,
  displayName = '',
  showToolbar = true,
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
  baseTheme = undefined,
  themelessBranding = false,
  toolbarStart = undefined,
  toolbarEnd = undefined,
  toolbarPortal = undefined,
}: GatePreviewProps): JSX.Element {
  const {t} = useTranslation('design');
  const {mode, systemMode} = useColorScheme();
  const resolvedSystemMode: 'light' | 'dark' = (mode === 'system' ? systemMode : mode) === 'dark' ? 'dark' : 'light';
  const [previewEffectiveScheme, setPreviewEffectiveScheme] = useState<'light' | 'dark'>(resolvedSystemMode);
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
    doc.write(IFRAME_INITIAL_HTML);
    doc.close();
    setIframeDoc(doc);
  }, []);

  const activeScheme = colorScheme !== 'system' ? colorScheme : undefined;
  let effectiveScheme: 'light' | 'dark';
  if (activeScheme) {
    effectiveScheme = activeScheme;
  } else if (syncColorSchemeWithSystem) {
    effectiveScheme = resolvedSystemMode;
  } else {
    effectiveScheme = previewEffectiveScheme;
  }

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
    <Box sx={{height: '100%', display: 'flex', flexDirection: 'column'}}>
      {/* Toolbar — portaled to external container when toolbarPortal is set, otherwise rendered inline */}
      {showToolbar &&
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
          alignItems: 'flex-start',
          p: frameless ? 0 : 2,
        }}
      >
        <Box
          sx={{
            backgroundColor: 'background.paper',
            borderRadius: frameless ? 0 : 1,
            width: frameless ? '100%' : (viewport?.width ?? VIEWPORT_WIDTHS[viewportState]),
            height: frameless ? '100%' : (viewport?.height ?? VIEWPORT_HEIGHTS[viewportState]),
            transition: 'width 0.2s ease, height 0.2s ease',
            display: 'flex',
            flexDirection: 'column',
          }}
        >
          {/* Browser chrome */}
          {!frameless && (
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

          {/* Canvas — fills the browser chrome frame like a real viewport */}
          <Box
            ref={canvasRef}
            sx={{
              flex: 1,
              overflow: 'hidden',
              position: 'relative',
            }}
          >
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
                  pageBackground={pageBackground}
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
        </Box>
      </Box>
    </Box>
  );
}
