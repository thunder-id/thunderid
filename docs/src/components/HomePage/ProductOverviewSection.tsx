// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import {Box, ButtonBase, Container, Typography} from '@wso2/oxygen-ui';
import {Bot, Check, ShieldCheck, Waypoints, Zap} from '@wso2/oxygen-ui-icons-react';
import {type JSX, type KeyboardEvent, type ReactNode, useRef, useState} from 'react';
import {type AccentKey, cardSx, DISPLAY_FONT, EASE, useHomeTokens, HOME_MAX_WIDTH} from './homeTheme';
import {IconTile, Reveal, Section, SectionHeading, TextLink} from './shared';
import type {DocusaurusProductConfig} from '@site/docusaurus.product.config';

interface Pillar {
  accent: AccentKey;
  icon: (size: number) => ReactNode;
  title: string;
  /** One-line problem statement shown on the tab. */
  why: string;
  headline: string;
  body: string;
  supports: string[];
}

const PILLARS: Pillar[] = [
  {
    accent: 'violet',
    icon: (size) => <Bot size={size} strokeWidth={1.6} />,
    title: 'Native agent identity',
    why: "AI agents now act on their own and on others' behalf.",
    headline: 'Agents are identities, not service accounts',
    body: 'Every AI agent is its own first-class identity, with an owning user and a managed lifecycle, and acts with delegated, auditable authority, keeping a human in the loop.',
    supports: ['OAuth 2.1', 'Token exchange', 'ID-JAG', 'CIBA', 'MCP authorization', 'A2A authorization'],
  },
  {
    accent: 'cyan',
    icon: (size) => <ShieldCheck size={size} strokeWidth={1.6} />,
    title: 'Post-quantum safe by design',
    why: 'The digital trust has to hold up against quantum attacks.',
    headline: 'Tokens you sign today stay trustworthy tomorrow',
    body: 'ThunderID is crypto-agile from the core, with algorithms and keys set in configuration, so what you issue today holds up against harvest-now-decrypt-later and trust-now-forge-later attacks.',
    supports: ['ML-DSA', 'ML-KEM', 'SLH-DSA', 'X25519MLKEM768 hybrid TLS', 'Hybrid classical and post-quantum mode'],
  },
  {
    accent: 'electric',
    icon: (size) => <Waypoints size={size} strokeWidth={1.6} />,
    title: 'Integrates with decentralized identity infrastructure',
    why: 'Credentials are moving to wallets.',
    headline: "A bridge between today's identity systems and digital wallets",
    body: 'ThunderID connects the identity systems you run today with decentralized identity infrastructure, including wallets, verifiable credentials, DIDs, and trust registries.',
    supports: ['OpenID4VCI issuance', 'OpenID4VP presentation', 'Standard APIs for issuers, verifiers, and holders'],
  },
  {
    accent: 'violet',
    icon: (size) => <Zap size={size} strokeWidth={1.6} />,
    title: 'High-performance lightweight runtime with GitOps support',
    why: 'Identity infrastructure has to be lightweight and fast.',
    headline: 'A Go-based, high-performance runtime that lives in Git',
    body: 'ThunderID is a lightweight, Go-based, high-performance runtime that runs headless and API-first, with identity flows, policies, and configuration declared in code, versioned in Git, and deployed through your existing pipelines.',
    supports: ['Docker', 'Kubernetes', 'Helm', 'On-premises', 'Cloud'],
  },
];

const num = (i: number): string => String(i + 1).padStart(2, '0');

function PillarPanel({pillar, index, active}: {pillar: Pillar; index: number; active: boolean}): JSX.Element {
  const t = useHomeTokens();
  return (
    <Box
      role="tabpanel"
      id={`pillar-panel-${index}`}
      aria-labelledby={`pillar-tab-${index}`}
      sx={{
        display: 'flex',
        flexDirection: 'column',
        /* Desktop: panels stack in one grid cell and cross-fade. */
        gridArea: {md: '1 / 1'},
        opacity: {xs: 1, md: active ? 1 : 0},
        visibility: {xs: 'visible', md: active ? 'visible' : 'hidden'},
        transform: {xs: 'none', md: active ? 'none' : 'translateY(10px)'},
        transition: active
          ? 'opacity 0.45s 0.05s, transform 0.45s 0.05s'
          : 'opacity 0.4s, transform 0.4s, visibility 0s 0.4s',
        /* Mobile: each panel is its own card. */
        p: {xs: 3.25, md: 0},
        mb: {xs: 1.75, md: 0},
        borderRadius: {xs: '20px', md: 0},
        bgcolor: {xs: t.isDark ? 'rgba(5,22,44,0.8)' : t.surface, md: 'transparent'},
        border: {xs: '1px solid', md: 'none'},
        borderColor: t.line,
        '@media (prefers-reduced-motion: reduce)': {transition: 'none'},
      }}
    >
      {/* Mobile-only header replacing the tab list. */}
      <Box
        sx={{
          display: {xs: 'flex', md: 'none'},
          gap: 1.25,
          alignItems: 'baseline',
          mb: 2,
          pb: 1.75,
          borderBottom: '1px solid',
          borderColor: t.line,
        }}
      >
        <Typography
          component="span"
          sx={{fontFamily: 'var(--ifm-font-family-monospace)', fontSize: '0.78rem', color: t.cyan}}
        >
          {num(index)}
        </Typography>
        <Typography component="span" sx={{fontSize: '0.875rem', color: 'text.secondary'}}>
          {pillar.why}
        </Typography>
      </Box>

      <Box sx={{display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', mb: {xs: 2, md: 2.75}}}>
        <IconTile accent={pillar.accent} size={54}>
          {pillar.icon(26)}
        </IconTile>
        <Box
          component="span"
          aria-hidden
          sx={{
            fontFamily: DISPLAY_FONT,
            fontSize: {xs: 52, md: 72},
            fontWeight: 800,
            lineHeight: 0.8,
            letterSpacing: '-0.04em',
            background: t.isDark
              ? 'linear-gradient(180deg, rgba(124,180,255,0.35), rgba(124,180,255,0))'
              : 'linear-gradient(180deg, rgba(54,136,255,0.3), rgba(54,136,255,0))',
            WebkitBackgroundClip: 'text',
            backgroundClip: 'text',
            color: 'transparent',
          }}
        >
          {num(index)}
        </Box>
      </Box>

      <Typography
        component="h3"
        sx={{
          m: 0,
          fontFamily: DISPLAY_FONT,
          fontSize: {xs: '1.375rem', md: '1.75rem'},
          fontWeight: 700,
          lineHeight: 1.2,
          letterSpacing: '-0.02em',
        }}
      >
        {pillar.title}
      </Typography>
      <Typography sx={{mt: 1.25, fontSize: '1.0625rem', fontWeight: 600, lineHeight: 1.45, color: t.link}}>
        {pillar.headline}
      </Typography>
      <Typography sx={{mt: 1.5, fontSize: '1rem', lineHeight: 1.65, color: 'text.secondary', maxWidth: '60ch'}}>
        {pillar.body}
      </Typography>

      <Box sx={{mt: 'auto', pt: 3.25}}>
        <Typography
          component="span"
          sx={{
            fontSize: '0.72rem',
            fontWeight: 600,
            letterSpacing: '0.08em',
            textTransform: 'uppercase',
            color: t.muted2,
          }}
        >
          Supports
        </Typography>
        <Box
          component="ul"
          sx={{
            listStyle: 'none',
            display: 'grid',
            gridTemplateColumns: 'repeat(2, minmax(0, 1fr))',
            gap: 1,
            m: 0,
            mt: 1,
            p: 0,
            pt: 1.75,
            borderTop: '1px solid',
            borderColor: t.line,
          }}
        >
          {pillar.supports.map((s) => (
            <Box
              component="li"
              key={s}
              sx={{
                display: 'flex',
                alignItems: 'flex-start',
                gap: 1.125,
                fontSize: {xs: '0.84rem', sm: '0.9rem'},
                lineHeight: 1.4,
                color: 'text.primary',
              }}
            >
              <Check size={16} strokeWidth={2} color={t.cyan} style={{flex: 'none', marginTop: 1}} />
              {s}
            </Box>
          ))}
        </Box>
      </Box>
    </Box>
  );
}

export default function ProductOverviewSection(): JSX.Element {
  const t = useHomeTokens();
  const {siteConfig} = useDocusaurusContext();
  const product = siteConfig.customFields?.product as DocusaurusProductConfig;
  const productName = product.project.name;
  const githubUrl = product.project.source.github.url;
  const [active, setActive] = useState(0);
  const tabRefs = useRef<(HTMLButtonElement | null)[]>([]);

  const select = (i: number, focus = false): void => {
    setActive(i);
    if (focus) tabRefs.current[i]?.focus();
  };

  const onKeyDown = (e: KeyboardEvent): void => {
    const keys = ['ArrowDown', 'ArrowUp', 'ArrowRight', 'ArrowLeft', 'Home', 'End'];
    if (!keys.includes(e.key)) return;
    e.preventDefault();
    const n = PILLARS.length;
    const next =
      e.key === 'Home'
        ? 0
        : e.key === 'End'
          ? n - 1
          : (active + (e.key === 'ArrowDown' || e.key === 'ArrowRight' ? 1 : -1) + n) % n;
    select(next, true);
  };

  return (
    <Section id="why">
      <Container maxWidth={false} sx={{maxWidth: HOME_MAX_WIDTH, px: {xs: 2, sm: 4}}}>
        <Box
          sx={{
            display: 'grid',
            gridTemplateColumns: {xs: '1fr', md: 'minmax(0,1.15fr) minmax(0,0.85fr)'},
            gap: {xs: 3, md: '24px 56px'},
            alignItems: {xs: 'start', md: 'end'},
            mb: {xs: 4.5, md: 7},
          }}
        >
          <SectionHeading
            eyebrow={`What is ${productName}?`}
            title="Open source IAM,"
            titleMuted="rebuilt for the decade ahead"
            sx={{mb: 0}}
          />
          <Reveal delay={160}>
            <Typography sx={{color: 'text.secondary', fontSize: {xs: '1rem', md: '1.125rem'}, lineHeight: 1.6}}>
              {productName} is a neutrally governed, open-source IAM stack built in Go, focused on open standards and
              designed to handle identity for humans, AI agents, and machines with fully orchestratable identity flows.
            </Typography>
            <Box sx={{mt: 2.25, display: 'flex', flexWrap: 'wrap', gap: '10px 28px'}}>
              <TextLink href={`${githubUrl}/milestones`}>See the public roadmap</TextLink>
              <TextLink href="/blog/introducing-thunderid">Read why we built {productName}</TextLink>
            </Box>
          </Reveal>
        </Box>

        <Reveal
          delay={200}
          sx={{
            display: 'grid',
            gridTemplateColumns: {xs: '1fr', md: 'minmax(0,0.95fr) minmax(0,1.25fr)'},
            gap: 2,
            alignItems: 'stretch',
          }}
        >
          <Box
            role="tablist"
            aria-label={`${productName} pillars`}
            aria-orientation="vertical"
            sx={{display: {xs: 'none', md: 'flex'}, flexDirection: 'column', gap: 1.25}}
          >
            {PILLARS.map((p, i) => {
              const on = i === active;
              return (
                <ButtonBase
                  key={p.title}
                  ref={(el: HTMLButtonElement | null) => {
                    tabRefs.current[i] = el;
                  }}
                  role="tab"
                  id={`pillar-tab-${i}`}
                  aria-controls={`pillar-panel-${i}`}
                  aria-selected={on}
                  tabIndex={on ? 0 : -1}
                  onClick={() => select(i)}
                  onKeyDown={onKeyDown}
                  sx={{
                    flex: 1,
                    display: 'grid',
                    gridTemplateColumns: 'auto auto 1fr',
                    alignItems: 'center',
                    justifyContent: 'stretch',
                    gap: 2,
                    px: 2.5,
                    py: 2,
                    textAlign: 'left',
                    borderRadius: '16px',
                    border: '1px solid',
                    borderColor: on ? 'rgba(54,136,255,0.5)' : t.line,
                    bgcolor: t.isDark ? (on ? 'rgba(8,40,74,0.86)' : 'rgba(5,22,44,0.72)') : on ? '#fff' : t.surface,
                    boxShadow: !t.isDark && on ? '0 8px 24px -12px rgba(15,40,80,0.2)' : 'none',
                    backdropFilter: 'blur(16px)',
                    transition: `background-color 0.3s, border-color 0.3s, transform 0.3s ${EASE}`,
                    '&:hover': {
                      borderColor: on ? 'rgba(54,136,255,0.5)' : t.lineStrong,
                      bgcolor: t.isDark ? 'rgba(8,40,74,0.78)' : '#fff',
                    },
                    '&:focus-visible': {outline: '2px solid', outlineColor: 'primary.main', outlineOffset: 3},
                  }}
                >
                  <Typography
                    component="span"
                    sx={{
                      fontFamily: 'var(--ifm-font-family-monospace)',
                      fontSize: '0.78rem',
                      letterSpacing: '0.04em',
                      color: on ? t.cyan : t.muted2,
                    }}
                  >
                    {num(i)}
                  </Typography>
                  <IconTile accent={p.accent} size={50} sx={{opacity: on ? 1 : 0.85}}>
                    {p.icon(26)}
                  </IconTile>
                  <Box sx={{display: 'flex', flexDirection: 'column', gap: 0.375, minWidth: 0}}>
                    <Typography
                      component="b"
                      sx={{
                        fontFamily: DISPLAY_FONT,
                        fontSize: '0.97rem',
                        fontWeight: 600,
                        lineHeight: 1.3,
                        color: 'text.primary',
                      }}
                    >
                      {p.title}
                    </Typography>
                    <Typography
                      component="em"
                      sx={{
                        fontStyle: 'normal',
                        fontSize: '0.84rem',
                        lineHeight: 1.45,
                        color: on ? 'text.secondary' : t.muted2,
                      }}
                    >
                      {p.why}
                    </Typography>
                  </Box>
                </ButtonBase>
              );
            })}
          </Box>

          <Box
            sx={(theme) => ({
              display: 'flex',
              flexDirection: 'column',
              [theme.breakpoints.up('md')]: {...(cardSx(t) as object), display: 'grid', padding: theme.spacing(4.5)},
              [theme.breakpoints.up('lg')]: {padding: theme.spacing(5)},
            })}
          >
            {PILLARS.map((p, i) => (
              <PillarPanel key={p.title} pillar={p} index={i} active={i === active} />
            ))}
          </Box>
        </Reveal>
      </Container>
    </Section>
  );
}
