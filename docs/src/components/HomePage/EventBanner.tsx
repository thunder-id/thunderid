// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Link from '@docusaurus/Link';
import {useBaseUrlUtils} from '@docusaurus/useBaseUrl';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import {Box, keyframes, type SxProps, type Theme} from '@wso2/oxygen-ui';
import {ArrowRight, Sparkles} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useEffect, useState} from 'react';
import {ACCENTS, sxList, useHomeTokens} from './homeTheme';
import type {DocusaurusProductConfig} from '@site/docusaurus.product.config';
import {linkColor, REDUCED_MOTION} from '@site/src/components/Events/eventsTheme';

interface EventItem {
  endDate: string;
  id: string;
  startDate: string;
  title: string;
  url: string;
}

const MONTHS_SHORT = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

function formatBannerDate(startDate: string, endDate: string): string {
  const start = new Date(`${startDate}T00:00:00`);
  const end = new Date(`${endDate}T00:00:00`);
  const month = MONTHS_SHORT[start.getMonth()];
  if (startDate === endDate) return `${month} ${start.getDate()}`;
  if (start.getMonth() === end.getMonth()) return `${month} ${start.getDate()}–${end.getDate()}`;
  return `${month} ${start.getDate()}–${MONTHS_SHORT[end.getMonth()]} ${end.getDate()}`;
}

/** Blue into violet, the agentic AI accent. Both ends keep white text above 4.5:1. */
const AI_GRADIENT = 'linear-gradient(120deg, #2c6fe0 0%, #6d4ee6 100%)';

/** A soft light band that sweeps across the banner now and then. */
const sheen = keyframes`
  0%, 70% { transform: translateX(-120%) skewX(-20deg); }
  100% { transform: translateX(320%) skewX(-20deg); }
`;

function todayStr(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

/** Promotes the next upcoming event and links to its page. `sx` styles the centring wrapper. */
export default function EventBanner({sx = undefined}: {sx?: SxProps<Theme>}): JSX.Element | null {
  const t = useHomeTokens();
  const {withBaseUrl} = useBaseUrlUtils();
  const {siteConfig} = useDocusaurusContext();
  const productName = (siteConfig.customFields?.product as DocusaurusProductConfig)?.project?.name ?? siteConfig.title;
  const [nextEvent, setNextEvent] = useState<EventItem | null | undefined>(undefined);

  useEffect(() => {
    const today = todayStr();
    fetch(withBaseUrl('/data/events.json'))
      .then((r) => r.json() as Promise<{events: EventItem[]}>)
      .then((data) => {
        const upcoming = data.events
          .filter((e) => e.endDate >= today)
          .sort((a, b) => a.startDate.localeCompare(b.startDate));
        setNextEvent(upcoming[0] ?? null);
      })
      .catch(() => setNextEvent(null));
  }, [withBaseUrl]);

  // undefined = loading (render nothing to avoid flicker), null = no upcoming events
  if (!nextEvent) return null;

  const layer = (color: string): string => `linear-gradient(${color}, ${color})`;
  const soft = (color: string): string => `color-mix(in srgb, ${color} 80%, transparent)`;
  const background = [
    `radial-gradient(70% 220% at 100% 50%, ${t.isDark ? 'rgba(123,92,246,0.34)' : 'rgba(123,92,246,0.16)'}, transparent 70%) padding-box`,
    `radial-gradient(45% 200% at 0% 50%, ${t.isDark ? 'rgba(0,213,255,0.18)' : 'rgba(0,213,255,0.11)'}, transparent 70%) padding-box`,
    `${layer(t.isDark ? 'rgba(54,136,255,0.11)' : 'rgba(54,136,255,0.056)')} padding-box`,
    `${layer(t.surfaceStrong)} padding-box`,
    `linear-gradient(90deg, ${soft(ACCENTS.cyan)}, ${soft(ACCENTS.electric)} 45%, ${soft(ACCENTS.violet)}) border-box`,
  ].join(', ');
  const glow = (strength: number): string =>
    `-14px 12px 32px -16px rgba(0,213,255,${0.5 * strength}), 14px 14px 36px -14px rgba(123,92,246,${0.65 * strength})`;

  return (
    <Box sx={[{display: 'flex', justifyContent: 'center', pt: 2.5, px: 2}, ...sxList(sx)]}>
      <Box
        component={Link}
        to={nextEvent.url}
        sx={{
          display: 'inline-flex',
          flexWrap: {xs: 'wrap', md: 'nowrap'},
          alignItems: 'center',
          gap: 1.5,
          position: 'relative',
          overflow: 'hidden',
          isolation: 'isolate',
          maxWidth: '100%',
          minWidth: 0,
          width: {xs: '100%', md: 'auto'},
          p: {xs: 1.5, md: 1},
          pl: {xs: 2, sm: 1.5, md: 1},
          borderRadius: {xs: '20px', md: '999px'},
          border: '1px solid transparent',
          background,
          boxShadow: glow(0.8),
          color: 'text.primary',
          '&::after': {
            content: '""',
            position: 'absolute',
            inset: 0,
            width: '30%',
            zIndex: -1,
            pointerEvents: 'none',
            background: `linear-gradient(90deg, transparent, ${t.isDark ? 'rgba(255,255,255,0.08)' : 'rgba(255,255,255,0.7)'}, transparent)`,
            animation: `${sheen} 7s ease-in-out 1.5s infinite`,
          },
          textDecoration: 'none',
          transition: 'transform 0.18s ease, box-shadow 0.2s ease',
          '&:hover': {
            color: 'text.primary',
            textDecoration: 'none',
            transform: 'translateY(-1px)',
            boxShadow: glow(1.1),
          },
          '&:hover .event-banner-cta': {filter: 'brightness(1.08)'},
          '&:hover .event-banner-arrow': {transform: 'translateX(2px)'},
          '&:focus-visible': {outline: '2px solid', outlineColor: 'primary.main', outlineOffset: '3px'},
          [REDUCED_MOTION]: {
            transition: 'none',
            '&::after': {animation: 'none', display: 'none'},
            '&:hover': {transform: 'none'},
            '&:hover .event-banner-arrow': {transform: 'none'},
          },
        }}
      >
        <Box
          component="span"
          aria-hidden
          sx={{
            display: {xs: 'none', sm: 'inline-flex'},
            alignItems: 'center',
            justifyContent: 'center',
            width: 36,
            height: 36,
            borderRadius: '50%',
            flex: 'none',
            background: AI_GRADIENT,
            color: 'common.white',
            boxShadow: `0 0 0 4px ${t.isDark ? 'rgba(123,92,246,0.18)' : 'rgba(123,92,246,0.11)'}`,
          }}
        >
          <Sparkles size={17} strokeWidth={2} />
        </Box>
        <Box
          component="span"
          sx={{
            flex: 'none',
            fontFamily: '"JetBrains Mono", monospace',
            fontSize: '0.8125rem',
            fontWeight: 700,
            letterSpacing: '0.04em',
            textTransform: 'uppercase',
            color: linkColor(t),
            whiteSpace: 'nowrap',
          }}
        >
          {formatBannerDate(nextEvent.startDate, nextEvent.endDate)}
        </Box>
        <Box
          component="span"
          aria-hidden
          sx={{display: {xs: 'none', sm: 'block'}, width: '1px', height: 16, flex: 'none', bgcolor: t.lineStrong}}
        />
        <Box
          component="span"
          sx={{
            flex: {xs: '1 1 0', md: '0 1 auto'},
            minWidth: 0,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: {xs: 'normal', md: 'nowrap'},
            lineHeight: 1.35,
            fontSize: {xs: '0.9375rem', sm: '1rem'},
            fontWeight: 600,
            letterSpacing: '-0.005em',
            color: 'text.primary',
          }}
        >
          {nextEvent.title.replaceAll('{{ProductName}}', productName)}
        </Box>
        <Box
          component="span"
          className="event-banner-cta"
          sx={{
            display: 'inline-flex',
            alignItems: 'center',
            justifyContent: 'center',
            gap: 0.75,
            flex: 'none',
            width: {xs: '100%', md: 'auto'},
            ml: {xs: 0, md: 0.5},
            px: 2.25,
            height: 36,
            borderRadius: '999px',
            background: AI_GRADIENT,
            color: 'common.white',
            fontSize: '0.875rem',
            fontWeight: 600,
            whiteSpace: 'nowrap',
            boxShadow: 'inset 0 1px 0 rgba(255,255,255,0.22), 0 6px 16px -6px rgba(109,78,230,0.56)',
            transition: 'filter 0.2s ease',
          }}
        >
          Register now
          <Box
            component="span"
            className="event-banner-arrow"
            aria-hidden
            sx={{display: 'inline-flex', transition: 'transform 0.18s ease'}}
          >
            <ArrowRight size={15} strokeWidth={2.25} />
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
