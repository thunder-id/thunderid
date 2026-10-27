// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Typography} from '@wso2/oxygen-ui';
import {useLayoutEffect, useRef, type JSX, type ReactNode, type RefObject} from 'react';
import {VIEWPORT_HEIGHTS, VIEWPORT_WIDTHS} from './viewportConstants';
import type {Viewport} from '../GatePreview/GatePreview';

/** Default minimum width (px) the staged content needs so a 450px sign-in card + padding renders unclipped. */
const DEFAULT_MIN_CONTENT_WIDTH = 520;

/** Default minimum height (px) the staged content needs so a typical sign-in form renders unclipped. */
const DEFAULT_MIN_CONTENT_HEIGHT = 700;

export interface PreviewFrameProps {
  /** Text shown in the fake address bar of the browser chrome. */
  label: string;
  viewport: Viewport;
  /** User zoom in percent, applied on top of the fit-to-frame scale. */
  zoom: number;
  /**
   * The element the frame sizes and scales so it fills the canvas: rendered at (canvas / scale)
   * and shrunk back with a CSS transform, so its content lays out like a real, wider viewport.
   * It must be one of `children`, absolutely positioned at the canvas origin.
   */
  stageRef: RefObject<HTMLElement | null>;
  /** Changes whenever the staged element is replaced, so the frame re-measures it. */
  stageKey?: unknown;
  children: ReactNode;
  /** Fixed frame size, overriding the viewport presets. */
  size?: {height: string | number; width: string | number};
  /** Hides the chrome and insets so the stage fills its container edge to edge. */
  frameless?: boolean;
  /** `'browser'` draws a desktop window; `'phone'` a rounded bezel with a status-bar notch. */
  frameStyle?: 'browser' | 'phone';
  /** Background of the frame behind the chrome and the stage. */
  frameBackground?: string;
  /** Content rendered inside the frame, below the canvas. */
  footer?: ReactNode;
  minContentWidth?: number;
  minContentHeight?: number;
}

/**
 * A device frame for builder previews: browser or phone chrome around a canvas that scales its
 * stage to fit the frame at the chosen zoom, with the stage's effective size shown in a corner.
 */
export default function PreviewFrame({
  label,
  viewport,
  zoom,
  stageRef,
  stageKey = undefined,
  children,
  size = undefined,
  frameless = false,
  frameStyle = 'browser',
  frameBackground = undefined,
  footer = undefined,
  minContentWidth = DEFAULT_MIN_CONTENT_WIDTH,
  minContentHeight = DEFAULT_MIN_CONTENT_HEIGHT,
}: PreviewFrameProps): JSX.Element {
  const canvasRef = useRef<HTMLDivElement>(null);
  const dimensionsRef = useRef<HTMLSpanElement>(null);
  const isPhone: boolean = !frameless && frameStyle === 'phone';

  // Imperatively size & scale the stage to fit the canvas: no React state, no re-renders.
  useLayoutEffect(() => {
    const canvas = canvasRef.current;
    const stage = stageRef.current;
    if (!canvas || !stage) return undefined;

    const update = (): void => {
      const cw = canvas.clientWidth;
      const ch = canvas.clientHeight;
      if (!cw || !ch) return;

      // Scale down to fit both dimensions so the content never clips, then apply the user zoom.
      const fitScale = Math.min(1, cw / minContentWidth, ch / minContentHeight);
      const totalScale = fitScale * (zoom / 100);

      // Inverse-scale: render the stage at (canvas / totalScale) so after
      // transform: scale(totalScale) it visually fills the canvas exactly.
      const stageW = Math.round(cw / totalScale);
      const stageH = Math.round(ch / totalScale);
      stage.style.width = `${stageW}px`;
      stage.style.height = `${stageH}px`;
      stage.style.transform = `scale(${totalScale})`;

      if (dimensionsRef.current) {
        dimensionsRef.current.textContent = `${stageW} × ${stageH}`;
      }
    };

    const observer = new ResizeObserver(update);
    observer.observe(canvas);
    update();

    return () => observer.disconnect();
  }, [zoom, stageRef, stageKey, minContentWidth, minContentHeight]);

  return (
    <Box
      sx={{
        flex: 1,
        minHeight: 0,
        overflow: 'hidden',
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
        p: frameless ? 0 : 2,
      }}
    >
      <Box
        sx={{
          backgroundColor: isPhone ? '#0a0a0a' : (frameBackground ?? 'background.paper'),
          borderRadius: frameless ? 0 : isPhone ? '32px' : 1,
          p: isPhone ? '10px' : 0,
          width: frameless ? '100%' : (size?.width ?? VIEWPORT_WIDTHS[viewport]),
          height: frameless ? '100%' : (size?.height ?? VIEWPORT_HEIGHTS[viewport]),
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
                {label}
              </Typography>
            </Box>
          </Box>
        )}

        {/* Phone chrome: a status-bar notch instead of a browser address bar */}
        {isPhone && (
          <Box sx={{display: 'flex', justifyContent: 'center', pb: 1, pt: 0.5, flexShrink: 0}}>
            <Box sx={{width: 56, height: 5, borderRadius: 3, bgcolor: 'rgba(255,255,255,0.25)'}} />
          </Box>
        )}

        {/* Canvas: fills the chrome frame like a real viewport */}
        <Box
          ref={canvasRef}
          sx={{
            flex: 1,
            overflow: 'hidden',
            position: 'relative',
            borderRadius: isPhone ? '22px' : 0,
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
          {children}
        </Box>

        {footer && <Box sx={{borderTop: '1px solid', borderColor: 'divider', flexShrink: 0}}>{footer}</Box>}
      </Box>
    </Box>
  );
}
