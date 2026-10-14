// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ResourceAvatar} from '@thunderid/components';
import {Box, IconButton, Tooltip, useTheme} from '@wso2/oxygen-ui';
import {AppWindow, ChevronLeft, ChevronRight} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useEffect, useRef, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {SessionParticipant} from '../models/session';

export interface SessionApplicationsProps {
  participants: SessionParticipant[];
  /** Icon size in pixels. */
  size: number;
}

const appLabel = (participant: SessionParticipant): string => participant.appName ?? participant.appId;

// How many icons one press of a scroll arrow moves the strip by.
const ICONS_PER_SCROLL = 3;

/**
 * The applications that joined a session, as a strip of overlapping icons named by tooltips. When the
 * icons don't fit, the strip scrolls sideways, and each side that has more fades out and offers an arrow.
 */
export default function SessionApplications({participants, size}: SessionApplicationsProps): JSX.Element | null {
  const {t} = useTranslation('sessions');
  const theme = useTheme();
  const stripRef = useRef<HTMLDivElement>(null);
  const [more, setMore] = useState({left: false, right: false});

  const updateMore = useCallback((): void => {
    const strip = stripRef.current;
    if (!strip) {
      return;
    }
    // A pixel of slack absorbs the rounding of fractional scroll positions.
    setMore({
      left: strip.scrollLeft > 1,
      right: strip.scrollLeft + strip.clientWidth < strip.scrollWidth - 1,
    });
  }, []);

  useEffect(() => {
    const strip = stripRef.current;
    if (!strip) {
      return undefined;
    }
    updateMore();
    const observer = new ResizeObserver(updateMore);
    observer.observe(strip);
    return (): void => observer.disconnect();
  }, [updateMore, participants.length]);

  if (participants.length === 0) {
    return null;
  }

  const fade = theme.spacing(3);
  const arrow = (direction: 1 | -1): JSX.Element => (
    <IconButton
      size="small"
      onClick={() => stripRef.current?.scrollBy({left: direction * size * ICONS_PER_SCROLL, behavior: 'smooth'})}
      aria-label={
        direction < 0
          ? t('applications.scrollLeft', 'Scroll applications left')
          : t('applications.scrollRight', 'Scroll applications right')
      }
      sx={{
        position: 'absolute',
        top: '50%',
        transform: 'translateY(-50%)',
        zIndex: 2,
        p: 0.25,
        border: 1,
        borderColor: 'divider',
        bgcolor: 'background.paper',
        ...(direction < 0 ? {left: 0} : {right: 0}),
      }}
    >
      {direction < 0 ? <ChevronLeft size={14} /> : <ChevronRight size={14} />}
    </IconButton>
  );

  return (
    <Box sx={{position: 'relative', display: 'flex', minWidth: 0}}>
      {more.left && arrow(-1)}
      <Box
        ref={stripRef}
        onScroll={updateMore}
        role="region"
        aria-label={t('details.applications', 'Active applications')}
        tabIndex={more.left || more.right ? 0 : undefined}
        sx={{
          display: 'flex',
          alignItems: 'center',
          minWidth: 0,
          overflowX: 'auto',
          scrollbarWidth: 'none',
          '&::-webkit-scrollbar': {display: 'none'},
          // Padding keeps the icons' rings and focus outlines inside the scroll area, which would clip them.
          p: 0.5,
          // Fades each side that has more to scroll to. A mask, so it works on any background.
          maskImage: `linear-gradient(to right, ${more.left ? 'transparent' : 'black'}, black ${fade}, black calc(100% - ${fade}), ${more.right ? 'transparent' : 'black'})`,
        }}
      >
        {participants.map((participant, index) => (
          <Tooltip key={`${participant.appId}:${participant.ouId ?? ''}`} title={appLabel(participant)}>
            <Box
              role="img"
              tabIndex={0}
              aria-label={appLabel(participant)}
              sx={{
                display: 'flex',
                flexShrink: 0,
                ml: index === 0 ? 0 : -0.1,
                borderRadius: 1,
                '&:hover, &:focus-visible': {zIndex: 1},
              }}
            >
              <ResourceAvatar
                variant="rounded"
                value={participant.appLogoUrl}
                size={size}
                fallback={<AppWindow size={size * 0.6} />}
                // The surface color at full opacity, since the theme's own is translucent and would let the icon show through.
                sx={{boxShadow: (th) => `0 0 0 2px rgb(from ${th.vars?.palette.background.paper} r g b / 1)`}}
              />
            </Box>
          </Tooltip>
        ))}
      </Box>
      {more.right && arrow(1)}
    </Box>
  );
}
