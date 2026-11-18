// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Link from '@docusaurus/Link';
import {useBaseUrlUtils} from '@docusaurus/useBaseUrl';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import {Box} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useEffect, useState} from 'react';
import useIsDarkMode from '../../hooks/useIsDarkMode';
import type {DocusaurusProductConfig} from '@site/docusaurus.product.config';

interface EventItem {
  city?: string;
  endDate: string;
  id: string;
  startDate: string;
  title: string;
}

const MONTHS_SHORT = ['JAN', 'FEB', 'MAR', 'APR', 'MAY', 'JUN', 'JUL', 'AUG', 'SEP', 'OCT', 'NOV', 'DEC'];

function formatBannerDate(startDate: string, endDate: string): string {
  const start = new Date(`${startDate}T00:00:00`);
  const end = new Date(`${endDate}T00:00:00`);
  const month = MONTHS_SHORT[start.getMonth()];
  if (startDate === endDate) return `${month} ${start.getDate()}`;
  if (start.getMonth() === end.getMonth()) return `${month} ${start.getDate()}–${end.getDate()}`;
  return `${month} ${start.getDate()}–${MONTHS_SHORT[end.getMonth()]} ${end.getDate()}`;
}

function todayStr(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

export default function EventBanner(): JSX.Element | null {
  const isLight = !useIsDarkMode();
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

  return (
    <Box sx={{display: 'flex', justifyContent: 'center', pt: 2, px: 4, overflow: 'hidden'}}>
      <Box
        component={Link}
        href="/events"
        sx={{
          display: 'inline-flex',
          alignItems: 'center',
          border: '1px solid',
          borderColor: isLight ? 'rgba(0,0,0,0.1)' : 'rgba(255,255,255,0.1)',
          borderRadius: '999px',
          bgcolor: isLight ? 'rgba(0,0,0,0.02)' : 'rgba(255,255,255,0.025)',
          backdropFilter: 'blur(8px)',
          py: '6px',
          pr: '6px',
          pl: 2,
          fontFamily: '"JetBrains Mono", monospace',
          fontSize: '0.75rem',
          maxWidth: '100%',
          transition: 'border-color 0.2s, background-color 0.2s',
          textDecoration: 'none',
          color: 'text.primary',
          '&:hover': {
            borderColor: 'rgba(54,136,255,0.4)',
            bgcolor: 'rgba(54,136,255,0.06)',
            textDecoration: 'none',
          },
        }}
      >
        <Box component="span" sx={{display: 'flex', alignItems: 'center', flexShrink: 0, mr: '9px'}}>
          <svg fill="none" height="14" viewBox="0 0 24 24" width="14">
            <path d="M13 2L4.5 13.5H11L10 22L20.5 10H14L13 2z" fill="url(#eventBannerBolt)" />
            <defs>
              <linearGradient gradientUnits="userSpaceOnUse" id="eventBannerBolt" x1="4.5" x2="20.5" y1="2" y2="22">
                <stop offset="0%" stopColor="#8bf9fa" />
                <stop offset="100%" stopColor="#3688ff" />
              </linearGradient>
            </defs>
          </svg>
        </Box>
        <Box component="span" sx={{fontWeight: 600, letterSpacing: '0.02em', whiteSpace: 'nowrap', color: 'text.primary', textTransform: 'uppercase'}}>
          {productName} ON THE ROAD
        </Box>
        <Box
          component="span"
          sx={{
            display: {xs: 'none', md: 'inline-block'},
            width: '1px',
            height: '13px',
            bgcolor: isLight ? 'rgba(0,0,0,0.16)' : 'rgba(255,255,255,0.16)',
            mx: 1.75,
            flexShrink: 0,
          }}
        />
        <Box component="span" sx={{display: {xs: 'none', md: 'inline'}, color: 'text.secondary', whiteSpace: 'nowrap', letterSpacing: '0.01em', textTransform: 'uppercase'}}>
          {nextEvent.title.replaceAll('{{ProductName}}', productName)} · {nextEvent.city} · {formatBannerDate(nextEvent.startDate, nextEvent.endDate)}
        </Box>
        <Box
          component="span"
          sx={{
            display: 'inline-flex',
            alignItems: 'center',
            ml: 1.75,
            px: 1.5,
            py: '5px',
            border: '1px solid',
            borderColor: isLight ? 'rgba(0,0,0,0.12)' : 'rgba(255,255,255,0.12)',
            borderRadius: '999px',
            color: 'text.secondary',
            whiteSpace: 'nowrap',
            flexShrink: 0,
            letterSpacing: '0.04em',
          }}
        >
          SEE ALL STOPS ›
        </Box>
      </Box>
    </Box>
  );
}
