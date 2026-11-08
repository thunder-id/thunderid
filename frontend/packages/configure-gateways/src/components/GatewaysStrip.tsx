// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, IconButton, Link, Stack, Typography} from '@wso2/oxygen-ui';
import {ChevronLeft, ChevronRight} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useEffect, useRef, useState, type JSX, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';

/** How far one press of a scroll button moves the gateways: about one card. */
const STEP = 300;

export interface GatewaysStripProps {
  /** How many gateways there are, to say so and to know whether to offer scrolling. */
  count: number;
  /** The gateway cards, with what joins them. */
  children: ReactNode;
  /** Where the gateways and the pipeline between them are edited, opened in a new tab. */
  editUrl?: string;
}

const Fade = ({side}: {side: 'left' | 'right'}): JSX.Element => (
  <Box
    aria-hidden
    data-testid={`strip-fade-${side}`}
    sx={{
      position: 'absolute',
      top: 1,
      bottom: 1,
      [side]: 1,
      width: 40,
      pointerEvents: 'none',
      borderRadius: side === 'left' ? '12px 0 0 12px' : '0 12px 12px 0',
      background: `linear-gradient(to ${side === 'left' ? 'right' : 'left'}, var(--oxygen-palette-background-default), transparent)`,
    }}
  />
);

/**
 * The gateways, in pipeline order, on one row that scrolls sideways when they do not fit, with
 * buttons to step through them and a fade on the side there is more to see.
 */
export default function GatewaysStrip({count, children, editUrl = undefined}: GatewaysStripProps): JSX.Element {
  const {t} = useTranslation('deploy');
  const track = useRef<HTMLDivElement>(null);
  const [atStart, setAtStart] = useState(true);
  const [atEnd, setAtEnd] = useState(true);

  const measure = useCallback((): void => {
    const element = track.current;
    if (!element) return;
    setAtStart(element.scrollLeft < 4);
    setAtEnd(element.scrollLeft + element.clientWidth >= element.scrollWidth - 4);
  }, []);

  useEffect(() => {
    measure();
    window.addEventListener('resize', measure);
    return () => window.removeEventListener('resize', measure);
  }, [measure, count]);

  const scroll = (by: number): void => track.current?.scrollBy({left: by, behavior: 'smooth'});

  return (
    <Stack spacing={1.25} sx={{flex: '999 1 640px', minWidth: 0}}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" spacing={1.5}>
        <Typography variant="overline" color="text.secondary" sx={{fontWeight: 600, letterSpacing: '0.08em'}}>
          {t('pipeline.gatewaysLabel', 'Gateways · where it runs')}
        </Typography>
        <Stack direction="row" alignItems="center" spacing={0.5}>
          <Typography variant="caption" color="text.secondary" sx={{mr: 0.5}}>
            {t('pipeline.gatewayCount', {
              count,
              defaultValue_one: '{{count}} gateway',
              defaultValue_other: '{{count}} gateways',
            })}
          </Typography>
          {editUrl && (
            <Link
              href={editUrl}
              target="_blank"
              rel="noopener noreferrer"
              variant="caption"
              sx={{fontWeight: 500, mr: 1}}
              data-testid="pipeline-edit"
            >
              {t('pipeline.editPipeline', 'Edit pipeline')}
            </Link>
          )}
          {count > 1 && (
            <>
              <IconButton
                size="small"
                onClick={() => scroll(-STEP)}
                disabled={atStart}
                aria-label={t('pipeline.scrollLeft', 'Scroll gateways left')}
              >
                <ChevronLeft size={16} />
              </IconButton>
              <IconButton
                size="small"
                onClick={() => scroll(STEP)}
                disabled={atEnd}
                aria-label={t('pipeline.scrollRight', 'Show more gateways')}
              >
                <ChevronRight size={16} />
              </IconButton>
            </>
          )}
        </Stack>
      </Stack>
      <Box sx={{position: 'relative', flex: 1, display: 'flex', minWidth: 0}}>
        <Box
          ref={track}
          onScroll={measure}
          tabIndex={0}
          role="region"
          aria-label={t('pipeline.gatewaysRegion', 'Gateways, scroll sideways')}
          data-testid="gateways-strip"
          sx={{
            flex: 1,
            minWidth: 0,
            display: 'flex',
            flexWrap: 'nowrap',
            alignItems: 'stretch',
            gap: 1.5,
            // A dashed frame as tall as the versions beside it, with the cards inside it.
            p: 2,
            overflowX: 'auto',
            scrollSnapType: 'x proximity',
            border: '1px dashed',
            borderColor: 'divider',
            borderRadius: '12px',
          }}
        >
          {children}
        </Box>
        {!atStart && <Fade side="left" />}
        {!atEnd && <Fade side="right" />}
      </Box>
    </Stack>
  );
}
