// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ColorSchemeOption} from '@thunderid/design';
import {Box, IconButton, Tooltip, Typography, useColorScheme} from '@wso2/oxygen-ui';
import {Minus, Monitor, Plus, Smartphone, Tablet} from '@wso2/oxygen-ui-icons-react';
import {type JSX, type ReactNode, useEffect, useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import type {Viewport} from './ThemePreviewPanel';
import {VIEWPORT_WIDTHS, VIEWPORT_HEIGHTS} from './viewportConstants';
import ColorSchemeOptions from '../constants/ColorSchemeOptions';

export interface PreviewToolbarProps {
  viewport: Viewport;
  setViewport: (v: Viewport) => void;
  previewColorScheme?: ColorSchemeOption;
  setPreviewColorScheme?: (cs: ColorSchemeOption) => void;
  /** Called whenever the color scheme changes (controlled or uncontrolled). */
  onColorSchemeChange?: (cs: ColorSchemeOption) => void;
  /** Called whenever the resolved effective scheme changes ('light' | 'dark'). */
  onEffectiveSchemeChange?: (scheme: 'light' | 'dark') => void;
  zoom: number;
  setZoom: (z: number) => void;
  zoomIdx: number;
  showDimensions?: boolean;
  /** Extra content rendered at the end of the toolbar (after a divider). */
  extraContent?: ReactNode;
  /** Whether to show viewport switching controls. Defaults to true. */
  showViewportControls?: boolean;
  /** Whether to show zoom controls. Defaults to true. */
  showZoomControls?: boolean;
}

const ZOOM_STEPS = [25, 50, 75, 100, 125, 150];

function ToolbarDivider(): JSX.Element {
  return <Box sx={{width: '1px', height: 16, bgcolor: 'divider', mx: 0.5, flexShrink: 0}} />;
}

const ASPECT_RATIO_LABELS: Record<Viewport, string> = {
  desktop: '16:9',
  tablet: '3:4',
  mobile: '9:19',
};

export default function PreviewToolbar({
  viewport,
  setViewport,
  previewColorScheme: controlledScheme = undefined,
  setPreviewColorScheme: setControlledScheme = undefined,
  onColorSchemeChange = undefined,
  onEffectiveSchemeChange = undefined,
  showDimensions = false,
  zoom,
  setZoom,
  zoomIdx,
  extraContent = undefined,
  showViewportControls = true,
  showZoomControls = true,
}: PreviewToolbarProps): JSX.Element {
  const {t} = useTranslation('design');
  const {mode, systemMode} = useColorScheme();
  const [internalScheme, setInternalScheme] = useState<ColorSchemeOption>(
    (mode as ColorSchemeOption | undefined) ?? 'light',
  );

  const isControlled = controlledScheme !== undefined && setControlledScheme !== undefined;
  const previewColorScheme = isControlled ? controlledScheme : internalScheme;

  const resolvedSystemMode = useMemo<'light' | 'dark'>(
    () => ((mode === 'system' ? systemMode : mode) === 'dark' ? 'dark' : 'light'),
    [mode, systemMode],
  );

  const effectiveScheme = useMemo<'light' | 'dark'>(
    () => (previewColorScheme !== 'system' ? previewColorScheme : resolvedSystemMode),
    [previewColorScheme, resolvedSystemMode],
  );

  useEffect(() => {
    onEffectiveSchemeChange?.(effectiveScheme);
  }, [effectiveScheme, onEffectiveSchemeChange]);

  const handleColorSchemeChange = (cs: ColorSchemeOption): void => {
    if (isControlled) {
      setControlledScheme(cs);
    } else {
      setInternalScheme(cs);
    }
    onColorSchemeChange?.(cs);
  };

  const viewportOptions: {id: Viewport; label: string; icon: JSX.Element}[] = [
    {
      id: 'mobile',
      label: t('common.preview.toolbar.viewports.mobile.label', 'Mobile (390px)'),
      icon: <Smartphone size={14} />,
    },
    {
      id: 'tablet',
      label: t('common.preview.toolbar.viewports.tablet.label', 'Tablet (768px)'),
      icon: <Tablet size={14} />,
    },
    {
      id: 'desktop',
      label: t('common.preview.toolbar.viewports.desktop.label', 'Desktop (1440px)'),
      icon: <Monitor size={14} />,
    },
  ];

  return (
    <Box
      sx={{
        zIndex: 10,
        display: 'flex',
        alignItems: 'center',
        gap: 0.5,
        px: 2,
        py: 1,
        bgcolor: 'background.paper',
        borderRadius: 1,
        boxShadow: '0 8px 32px rgba(0,0,0,0.18), 0 2px 8px rgba(0,0,0,0.08)',
        border: '1px solid',
        borderColor: 'divider',
        whiteSpace: 'nowrap',
      }}
    >
      {/* Viewport */}
      {showViewportControls && (
        <>
          {viewportOptions.map((vp) => (
            <Tooltip key={vp.id} title={vp.label}>
              <IconButton
                size="small"
                onClick={() => setViewport(vp.id)}
                sx={{
                  borderRadius: 1,
                  color: viewport === vp.id ? 'primary.main' : 'text.secondary',
                  bgcolor: viewport === vp.id ? 'primary.50' : 'transparent',
                  '&:hover': {bgcolor: viewport === vp.id ? 'primary.100' : 'action.hover'},
                }}
              >
                {vp.icon}
              </IconButton>
            </Tooltip>
          ))}

          <ToolbarDivider />
        </>
      )}

      {/* Dimensions */}
      {showDimensions && (
        <>
          <Box sx={{display: 'flex', alignItems: 'center', gap: 0.5, px: 0.5}}>
            <Typography
              variant="caption"
              sx={{color: 'text.secondary', fontSize: '0.7rem', fontVariantNumeric: 'tabular-nums'}}
            >
              {VIEWPORT_WIDTHS[viewport]}
            </Typography>
            <Typography variant="caption" sx={{color: 'text.disabled', fontSize: '0.7rem', lineHeight: 1}}>
              ×
            </Typography>
            <Typography
              variant="caption"
              sx={{color: 'text.secondary', fontSize: '0.7rem', fontVariantNumeric: 'tabular-nums'}}
            >
              {VIEWPORT_HEIGHTS[viewport]}
            </Typography>
            <Typography variant="caption" sx={{color: 'text.disabled', fontSize: '0.65rem', lineHeight: 1, pl: 0.25}}>
              {ASPECT_RATIO_LABELS[viewport]}
            </Typography>
          </Box>
          <ToolbarDivider />
        </>
      )}

      {/* Color scheme 3-way switch */}
      <Box sx={{display: 'flex', alignItems: 'center', gap: 0.75}}>
        <Typography
          variant="caption"
          sx={{fontSize: '0.7rem', color: 'text.secondary', userSelect: 'none', lineHeight: 1}}
        >
          {t('common.preview.toolbar.fields.color_scheme.label', 'Color scheme')}
        </Typography>
        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            border: '1px solid',
            borderColor: 'divider',
            borderRadius: 1,
            overflow: 'hidden',
          }}
        >
          {ColorSchemeOptions.map((cs) => (
            <Tooltip key={cs.id} title={t(`common.color_scheme.options.${cs.id}.label`, cs.label)}>
              <Box
                onClick={() => handleColorSchemeChange(cs.id)}
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  px: 0.75,
                  py: 0.5,
                  cursor: 'pointer',
                  color: previewColorScheme === cs.id ? 'primary.main' : 'text.secondary',
                  bgcolor: previewColorScheme === cs.id ? 'primary.50' : 'transparent',
                  '&:hover': {bgcolor: previewColorScheme === cs.id ? 'primary.100' : 'action.hover'},
                }}
              >
                {cs.icon}
              </Box>
            </Tooltip>
          ))}
        </Box>
      </Box>

      {showZoomControls && (
        <>
          <ToolbarDivider />

          {/* Zoom out */}
          <Tooltip title={t('common.preview.toolbar.actions.zoom_out.tooltip', 'Zoom out')}>
            <span>
              <IconButton
                size="small"
                disabled={zoomIdx <= 0}
                onClick={() => setZoom(ZOOM_STEPS[zoomIdx - 1] ?? zoom)}
                sx={{borderRadius: 1, color: 'text.secondary'}}
              >
                <Minus size={12} />
              </IconButton>
            </span>
          </Tooltip>

          <Typography
            variant="caption"
            sx={{
              fontSize: '0.7rem',
              minWidth: 36,
              textAlign: 'center',
              color: 'text.secondary',
              fontVariantNumeric: 'tabular-nums',
            }}
          >
            {zoom}%
          </Typography>

          {/* Zoom in */}
          <Tooltip title={t('common.preview.toolbar.actions.zoom_in.tooltip', 'Zoom in')}>
            <span>
              <IconButton
                size="small"
                disabled={zoomIdx >= ZOOM_STEPS.length - 1}
                onClick={() => setZoom(ZOOM_STEPS[zoomIdx + 1] ?? zoom)}
                sx={{borderRadius: 1, color: 'text.secondary'}}
              >
                <Plus size={12} />
              </IconButton>
            </span>
          </Tooltip>
        </>
      )}

      {extraContent && (
        <>
          <ToolbarDivider />
          {extraContent}
        </>
      )}
    </Box>
  );
}
