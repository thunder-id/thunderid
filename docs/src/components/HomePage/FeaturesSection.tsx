// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import {Box, ButtonBase, Container, Typography, type SxProps, type Theme} from '@wso2/oxygen-ui';
import {
  Bot,
  Check,
  Cloud,
  Container as ContainerIcon,
  Package,
  Server,
  User,
  Wallet,
} from '@wso2/oxygen-ui-icons-react';
import {type JSX, type ReactNode, useMemo, useState} from 'react';
import {ACCENTS, cardSx, DISPLAY_FONT, type HomeTokens, sxList, useHomeTokens, HOME_MAX_WIDTH} from './homeTheme';
import SDKShowcase from './SDKShowcase';
import {Reveal, Section, SectionHeading} from './shared';
import type {DocusaurusProductConfig} from '@site/docusaurus.product.config';

type Category = 'agents' | 'trust' | 'platform' | 'build';
type Filter = Category | 'all';

const CATEGORY_LABELS: Record<Category, string> = {
  agents: 'Agents',
  trust: 'Trust & standards',
  platform: 'Platform',
  build: 'Build',
};

const MONO = 'var(--ifm-font-family-monospace)';

/* ─── Visual helpers ─────────────────────────────────────────────────────── */

/** Colours for the small UI chips drawn inside card visuals. */
function chip(t: HomeTokens): {bg: string; bgSolid: string} {
  return t.isDark ? {bg: 'rgba(4,12,26,0.7)', bgSolid: 'rgba(3,8,18,0.88)'} : {bg: '#ffffff', bgSolid: '#ffffff'};
}

function Visual({children, sx = undefined}: {children: ReactNode; sx?: SxProps<Theme>}): JSX.Element {
  const t = useHomeTokens();
  return (
    <Box
      aria-hidden
      sx={[
        {
          position: 'relative',
          flex: '1 1 auto',
          minHeight: 136,
          borderRadius: '12px',
          overflow: 'hidden',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          fontFamily: MONO,
          fontSize: '0.75rem',
          color: 'text.secondary',
          bgcolor: t.inset,
          border: '1px solid',
          borderColor: t.line,
          transition: 'border-color 0.3s',
          '&::before': {
            content: '""',
            position: 'absolute',
            inset: 0,
            backgroundImage: `linear-gradient(${t.line} 1px, transparent 1px), linear-gradient(90deg, ${t.line} 1px, transparent 1px)`,
            backgroundSize: '16px 16px',
            maskImage: 'radial-gradient(closest-side, #000, transparent)',
            WebkitMaskImage: 'radial-gradient(closest-side, #000, transparent)',
            pointerEvents: 'none',
          },
          '& > *': {position: 'relative'},
          '& pre': {
            m: 0,
            p: 0,
            bgcolor: 'transparent',
            border: 0,
            fontFamily: MONO,
            fontSize: '0.75rem',
            lineHeight: 1.6,
          },
        },
        ...sxList(sx),
      ]}
    >
      {children}
    </Box>
  );
}

function AgentChainVisual(): JSX.Element {
  const t = useHomeTokens();
  const node = (icon: ReactNode, label: string, hot = false): JSX.Element => (
    <Box sx={{flex: 'none', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 0.75, mt: 1.75}}>
      <Box
        sx={{
          width: 44,
          height: 44,
          borderRadius: '12px',
          display: 'grid',
          placeItems: 'center',
          color: hot ? (t.isDark ? '#fff' : ACCENTS.violet) : t.link,
          bgcolor: hot ? 'rgba(123,92,246,0.25)' : 'rgba(54,136,255,0.1)',
          border: '1px solid',
          borderColor: hot ? 'rgba(123,92,246,0.7)' : 'rgba(54,136,255,0.3)',
        }}
      >
        {icon}
      </Box>
      <span>{label}</span>
    </Box>
  );
  const link = (label: string): JSX.Element => (
    <Box
      sx={{
        position: 'relative',
        flex: 1,
        height: 2,
        mx: 0.75,
        background: `repeating-linear-gradient(90deg, ${t.lineStrong} 0 6px, transparent 6px 11px)`,
        '&::after': {
          content: '""',
          position: 'absolute',
          top: '50%',
          right: -2,
          width: 7,
          height: 7,
          mt: '-3.5px',
          borderTop: `2px solid ${t.cyan}`,
          borderRight: `2px solid ${t.cyan}`,
          transform: 'rotate(45deg)',
        },
      }}
    >
      <Box
        component="em"
        sx={{
          position: 'absolute',
          left: '50%',
          top: 12,
          transform: 'translateX(-50%)',
          fontStyle: 'normal',
          fontSize: {xs: '0.625rem', sm: '0.6875rem'},
          whiteSpace: 'nowrap',
          color: t.muted2,
        }}
      >
        {label}
      </Box>
    </Box>
  );

  return (
    <Visual sx={{px: {xs: 1.5, sm: 3.25}}}>
      <Box
        component="span"
        sx={{
          position: 'absolute !important',
          top: 12,
          left: 14,
          display: 'inline-flex',
          alignItems: 'center',
          gap: 0.875,
          px: 1.25,
          py: 0.375,
          borderRadius: '999px',
          fontSize: '0.6875rem',
          color: ACCENTS.green,
          bgcolor: 'rgba(63,185,80,0.08)',
          border: '1px solid rgba(63,185,80,0.3)',
        }}
      >
        <Box component="span" sx={{width: 6, height: 6, borderRadius: '50%', bgcolor: ACCENTS.green}} />
        CIBA · approved
      </Box>
      {node(<User size={22} strokeWidth={1.6} />, 'user')}
      {link('delegation')}
      {node(<Bot size={22} strokeWidth={1.6} />, 'agent', true)}
      {link('token exchange')}
      {node(<Server size={22} strokeWidth={1.6} />, 'API')}
    </Visual>
  );
}

/** Deterministic hex backdrop, so server and client renders match. */
const PQ_HEX = ((): string => {
  const H = '0123456789abcdef';
  let seed = 7;
  const rnd = (): number => {
    seed = (seed * 1103515245 + 12345) % 2147483648;
    return seed / 2147483648;
  };
  return Array.from({length: 20}, () => Array.from({length: 34}, () => H[(rnd() * 16) | 0]).join('')).join('\n');
})();

function PostQuantumVisual(): JSX.Element {
  const t = useHomeTokens();
  const algos = [
    {name: 'ML-DSA', kind: 'signatures'},
    {name: 'ML-KEM', kind: 'key exchange'},
    {name: 'SLH-DSA', kind: 'hash-based'},
    {name: 'X25519MLKEM768', kind: 'hybrid TLS', tls: true},
  ];
  return (
    <Visual>
      <Box
        component="pre"
        sx={{
          position: 'absolute !important',
          inset: '12px',
          overflow: 'hidden',
          whiteSpace: 'pre',
          fontSize: '0.72rem !important',
          lineHeight: '1.55 !important',
          letterSpacing: '0.06em',
          color: t.isDark ? 'rgba(124,180,255,0.28)' : 'rgba(54,136,255,0.25)',
        }}
      >
        {PQ_HEX}
      </Box>
      <Box
        component="ul"
        sx={{
          listStyle: 'none',
          m: 0,
          p: 0,
          display: 'flex',
          flexDirection: 'column',
          gap: {xs: 0.625, lg: 1},
          width: 'min(236px, 84%)',
        }}
      >
        {algos.map((a) => (
          <Box
            component="li"
            key={a.name}
            sx={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              gap: 1.25,
              px: 1.5,
              py: {xs: 0.5, lg: 0.875},
              borderRadius: '9px',
              bgcolor: chip(t).bgSolid,
              border: '1px solid',
              borderColor: a.tls ? 'rgba(123,92,246,0.55)' : 'rgba(0,213,255,0.4)',
            }}
          >
            <Box
              component="b"
              sx={{
                fontFamily: DISPLAY_FONT,
                fontSize: a.tls ? '0.75rem' : '0.84rem',
                fontWeight: 700,
                color: 'text.primary',
              }}
            >
              {a.name}
            </Box>
            <Box component="span" sx={{fontSize: '0.65rem', color: t.muted2}}>
              {a.kind}
            </Box>
          </Box>
        ))}
      </Box>
    </Visual>
  );
}

function WalletVisual(): JSX.Element {
  const t = useHomeTokens();
  return (
    <Visual>
      <Box
        sx={{
          width: 'min(196px, 80%)',
          px: 1.75,
          py: 1.5,
          borderRadius: '12px',
          background: 'linear-gradient(140deg, rgba(123,92,246,0.4), rgba(54,136,255,0.32) 55%, rgba(0,213,255,0.22))',
          border: '1px solid rgba(200,225,255,0.22)',
          boxShadow: '0 18px 30px -16px rgba(0,0,0,0.6)',
          transform: 'rotate(-4deg)',
          color: t.isDark ? '#f0f8ff' : '#0b2447',
        }}
      >
        <Box sx={{display: 'flex', alignItems: 'center', gap: 0.875, fontSize: '0.72rem', mb: 1.25}}>
          <Wallet size={16} strokeWidth={1.6} />
          <span>Credential</span>
        </Box>
        {['72%', '48%'].map((w) => (
          <Box
            key={w}
            sx={{
              width: w,
              height: 5,
              borderRadius: '3px',
              bgcolor: t.isDark ? 'rgba(255,255,255,0.28)' : 'rgba(11,36,71,0.22)',
              mb: 0.75,
            }}
          />
        ))}
        <Box sx={{display: 'flex', justifyContent: 'space-between', mt: 1.25, fontSize: '0.65rem'}}>
          <span>OpenID4VP</span>
          <Box component="b" sx={{fontWeight: 600, color: t.isDark ? '#8ff0a4' : '#1a7f37'}}>
            ✓ verified
          </Box>
        </Box>
      </Box>
    </Visual>
  );
}

interface FlowPageProps {
  title?: string;
  children?: ReactNode;
  selected?: boolean;
  dashed?: boolean;
  sx?: SxProps<Theme>;
}

function FlowPage({
  title = undefined,
  children = undefined,
  selected = false,
  dashed = false,
  sx = undefined,
}: FlowPageProps): JSX.Element {
  const t = useHomeTokens();
  return (
    <Box
      sx={[
        {
          width: {xs: 84, sm: 124, lg: 136},
          borderRadius: '10px',
          overflow: 'hidden',
          fontFamily: 'inherit',
          bgcolor: t.isDark ? '#0b1526' : '#fff',
          border: '1px solid',
          borderStyle: dashed ? 'dashed' : 'solid',
          borderColor: selected ? ACCENTS.electric : t.lineStrong,
          boxShadow: selected ? '0 0 0 3px rgba(54,136,255,0.18)' : '0 12px 24px -14px rgba(0,0,0,0.4)',
        },
        ...sxList(sx),
      ]}
    >
      <Box sx={{display: 'flex', gap: '3px', px: 0.875, py: 0.625, borderBottom: '1px solid', borderColor: t.line}}>
        {[0, 1, 2].map((i) => (
          <Box key={i} component="i" sx={{width: 4, height: 4, borderRadius: '50%', bgcolor: t.lineStrong}} />
        ))}
      </Box>
      <Box
        sx={{
          display: 'flex',
          flexDirection: 'column',
          gap: 0.75,
          px: {xs: 0.875, lg: 1.5},
          py: {xs: 0.75, lg: 1.125},
          minHeight: {xs: 60, lg: 74},
        }}
      >
        {title && (
          <Box
            component="b"
            sx={{fontSize: {xs: '0.65rem', lg: '0.8rem'}, fontWeight: 600, lineHeight: 1.1, color: 'text.primary'}}
          >
            {title}
          </Box>
        )}
        {children}
      </Box>
    </Box>
  );
}

/** Verification-code boxes in the flow designer visual; the first two are filled in. */
const OTP_DIGITS = [
  {id: 'd1', on: true},
  {id: 'd2', on: true},
  {id: 'd3', on: false},
  {id: 'd4', on: false},
];

function FlowDesignerVisual(): JSX.Element {
  const t = useHomeTokens();
  const input = (
    <Box
      component="span"
      sx={{height: {xs: 9, lg: 11}, borderRadius: '4px', border: '1px solid', borderColor: t.lineStrong}}
    />
  );
  const button = (color: string = ACCENTS.electric): JSX.Element => (
    <Box component="span" sx={{height: {xs: 10, lg: 13}, mt: '2px', borderRadius: '4px', bgcolor: color}} />
  );
  const done = (label: string, color: string): JSX.Element => (
    <Box
      sx={{
        flex: 1,
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        gap: 0.75,
      }}
    >
      <Box
        sx={{
          width: {xs: 24, lg: 30},
          height: {xs: 24, lg: 30},
          borderRadius: '50%',
          display: 'grid',
          placeItems: 'center',
          color,
          bgcolor: `color-mix(in srgb, ${color} 14%, transparent)`,
          border: `1px solid color-mix(in srgb, ${color} 50%, transparent)`,
        }}
      >
        <Check size={13} strokeWidth={2.4} />
      </Box>
      <Box component="b" sx={{fontSize: {xs: '0.65rem', lg: '0.8rem'}, fontWeight: 600, color: 'text.primary'}}>
        {label}
      </Box>
    </Box>
  );
  const edge = (sx: SxProps<Theme>): JSX.Element => (
    <Box
      sx={[
        {
          position: 'relative',
          height: 2,
          width: '100%',
          background: 'linear-gradient(90deg, rgba(54,136,255,0.25), rgba(54,136,255,0.75))',
          '&::after': {
            content: '""',
            position: 'absolute',
            right: 0,
            top: '50%',
            width: 6,
            height: 6,
            mt: '-3px',
            borderTop: `2px solid ${t.link}`,
            borderRight: `2px solid ${t.link}`,
            transform: 'rotate(45deg)',
          },
        },
        ...sxList(sx),
      ]}
    />
  );
  const GREEN = t.isDark ? '#8ff0a4' : '#1a7f37';

  return (
    <Visual
      sx={{flexDirection: 'column', gap: 3, minHeight: {xs: 240, lg: 150}, pt: 5, pb: 2, px: {xs: 1.25, sm: 2.25}}}
    >
      <Box
        component="span"
        sx={{
          position: 'absolute !important',
          top: 9,
          left: 12,
          display: 'flex',
          alignItems: 'center',
          gap: 0.75,
          fontSize: '0.65rem',
          color: t.muted2,
        }}
      >
        <Box component="i" sx={{width: 6, height: 6, borderRadius: '2px', bgcolor: ACCENTS.electric}} />
        Flow designer · login
      </Box>
      <Box
        component="span"
        sx={{
          position: 'absolute !important',
          top: 8,
          right: 10,
          display: 'flex',
          gap: '2px',
          p: '2px',
          borderRadius: '8px',
          border: '1px solid',
          borderColor: t.line,
          bgcolor: chip(t).bg,
        }}
      >
        <Box
          component="b"
          sx={{
            fontWeight: 500,
            fontSize: '0.6875rem',
            px: 1.125,
            py: 0.375,
            borderRadius: '6px',
            color: 'text.primary',
            bgcolor: 'rgba(54,136,255,0.22)',
          }}
        >
          Designer
        </Box>
        <Box component="b" sx={{fontWeight: 500, fontSize: '0.6875rem', px: 1.125, py: 0.375}}>
          Code
        </Box>
      </Box>

      <Box
        sx={{
          display: 'grid',
          alignItems: 'center',
          justifyContent: 'center',
          width: '100%',
          gridTemplateColumns: {
            xs: 'auto minmax(8px,1fr) auto minmax(8px,1fr) auto',
            sm: 'auto minmax(12px,54px) auto minmax(12px,54px) auto minmax(12px,54px) auto',
            lg: 'auto minmax(10px,40px) auto minmax(10px,40px) auto minmax(10px,40px) auto',
          },
          gridTemplateRows: {xs: 'auto 52px auto', lg: 'auto 40px auto'},
        }}
      >
        {/* On phones the start node is dropped and everything shifts one column left. */}
        <Box
          sx={{
            display: {xs: 'none', sm: 'block'},
            gridArea: '1/1',
            width: 10,
            height: 10,
            borderRadius: '50%',
            bgcolor: ACCENTS.cyan,
            boxShadow: '0 0 0 4px rgba(0,213,255,0.15)',
          }}
        />
        {edge({display: {xs: 'none', sm: 'block'}, gridArea: '1/2'})}
        <FlowPage title="Sign in" selected sx={{gridArea: {xs: '1/1', sm: '1/3'}}}>
          {input}
          {input}
          {button()}
          <Box
            component="span"
            sx={{width: '46%', height: 4, mt: '1px', borderRadius: '2px', bgcolor: 'rgba(0,213,255,0.55)'}}
          />
        </FlowPage>
        {edge({gridArea: {xs: '1/2', sm: '1/4'}})}
        <FlowPage title="Verify code" sx={{gridArea: {xs: '1/3', sm: '1/5'}}}>
          <Box sx={{display: 'flex', gap: 0.5}}>
            {OTP_DIGITS.map(({id, on}) => (
              <Box
                key={id}
                component="i"
                sx={{
                  flex: 1,
                  height: {xs: 14, lg: 17},
                  borderRadius: '3px',
                  border: '1px solid',
                  borderColor: on ? 'rgba(0,213,255,0.7)' : t.lineStrong,
                  bgcolor: on ? 'rgba(0,213,255,0.1)' : 'transparent',
                }}
              />
            ))}
          </Box>
          {button()}
        </FlowPage>
        {edge({gridArea: {xs: '1/4', sm: '1/6'}})}
        <FlowPage sx={{gridArea: {xs: '1/5', sm: '1/7'}, '& > div:last-of-type': {justifyContent: 'center'}}}>
          {done('Welcome', GREEN)}
        </FlowPage>

        <Box
          sx={{
            gridArea: {xs: '2/1', sm: '2/3'},
            justifySelf: 'center',
            position: 'relative',
            width: 2,
            height: '100%',
            background: 'repeating-linear-gradient(180deg, rgba(0,213,255,0.7) 0 5px, transparent 5px 9px)',
            '&::after': {
              content: '""',
              position: 'absolute',
              left: '50%',
              bottom: 0,
              width: 6,
              height: 6,
              ml: '-3px',
              borderRight: `2px solid ${t.cyan}`,
              borderBottom: `2px solid ${t.cyan}`,
              transform: 'rotate(45deg)',
            },
          }}
        >
          <Box
            component="em"
            sx={{
              display: {xs: 'none', sm: 'block'},
              position: 'absolute',
              left: 10,
              top: '50%',
              transform: 'translateY(-50%)',
              fontStyle: 'normal',
              fontSize: '0.65rem',
              color: t.muted2,
              whiteSpace: 'nowrap',
            }}
          >
            recovery
          </Box>
        </Box>
        <FlowPage title="Reset password" dashed sx={{gridArea: {xs: '3/1', sm: '3/3'}}}>
          {input}
          {button(ACCENTS.violet)}
        </FlowPage>
        {edge({gridArea: {xs: '3/2', sm: '3/4'}})}
        <FlowPage dashed sx={{gridArea: {xs: '3/3', sm: '3/5'}}}>
          {done('Link sent', t.cyan)}
        </FlowPage>
      </Box>

      <Box
        sx={{
          display: {xs: 'flex', lg: 'none'},
          alignItems: 'center',
          justifyContent: 'center',
          flexWrap: 'wrap',
          gap: 0.75,
          fontSize: '0.6875rem',
        }}
      >
        <Box component="span" sx={{color: t.muted2, mr: 0.5}}>
          Nodes
        </Box>
        {['Page', 'Prompt', 'Decision', 'Action', 'Executor'].map((n) => (
          <Box
            key={n}
            component="b"
            sx={{
              fontWeight: 500,
              px: 1.25,
              py: 0.5,
              borderRadius: '7px',
              border: '1px dashed',
              borderColor: t.lineStrong,
              bgcolor: chip(t).bg,
            }}
          >
            {n}
          </Box>
        ))}
      </Box>
    </Visual>
  );
}

function BootVisual(): JSX.Element {
  const t = useHomeTokens();
  return (
    <Visual sx={{flexDirection: 'column', alignItems: 'stretch', gap: 1.5, px: 2.25}}>
      <Box sx={{display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
        <span>
          <Box component="span" sx={{color: t.cyan}}>
            $
          </Box>{' '}
          thunderid
        </span>
        <Box
          component="span"
          sx={{
            fontSize: '0.65rem',
            px: 0.875,
            borderRadius: '999px',
            color: t.isDark ? '#79d4fd' : '#00799e',
            border: '1px solid',
            borderColor: 'rgba(121,212,253,0.5)',
          }}
        >
          Go
        </Box>
      </Box>
      <Box sx={{height: 6, borderRadius: '3px', bgcolor: t.line, overflow: 'hidden'}}>
        <Box sx={{height: '100%', borderRadius: '3px', background: 'linear-gradient(90deg, #2d78e0, #3688FF)'}} />
      </Box>
      <Box sx={{display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
        <Box component="span" sx={{color: ACCENTS.green}}>
          ● ready
        </Box>
        <span>&lt; 50 ms</span>
      </Box>
    </Visual>
  );
}

function DeployVisual(): JSX.Element {
  const t = useHomeTokens();
  const items = [
    {label: 'Docker', icon: <ContainerIcon size={15} />, on: true},
    {label: 'Kubernetes', icon: <Cloud size={15} />},
    {label: 'Helm', icon: <Package size={15} />},
  ];
  return (
    <Visual>
      <Box sx={{display: 'flex', flexDirection: 'column', gap: 0.875, width: '70%'}}>
        {items.map((i) => (
          <Box
            key={i.label}
            sx={{
              display: 'flex',
              alignItems: 'center',
              gap: 1.125,
              px: 1.25,
              py: 0.75,
              borderRadius: '8px',
              border: '1px solid',
              borderColor: i.on ? 'rgba(123,92,246,0.7)' : t.line,
              bgcolor: chip(t).bg,
              color: i.on ? 'text.primary' : 'text.secondary',
              transform: i.on ? 'translateX(6px)' : 'none',
            }}
          >
            {i.icon}
            {i.label}
          </Box>
        ))}
      </Box>
    </Visual>
  );
}

function OrgTreeVisual(): JSX.Element {
  const t = useHomeTokens();
  const tag = (label: string, color: string): JSX.Element => (
    <Box
      component="i"
      sx={{
        fontStyle: 'normal',
        px: 0.75,
        borderRadius: '5px',
        fontSize: '0.72rem',
        color,
        bgcolor: `color-mix(in srgb, ${color} 14%, transparent)`,
        border: `1px solid color-mix(in srgb, ${color} 45%, transparent)`,
      }}
    >
      {label}
    </Box>
  );
  const rows: ReactNode[] = [
    <Box key="r" component="span" sx={{color: t.cyan}}>
      acme
    </Box>,
    <>├─ {tag('workforce', t.isDark ? '#b7a6ff' : ACCENTS.violet)}</>,
    '│  ├─ engineering',
    '│  ├─ sales',
    '│  └─ agents',
    <>└─ {tag('B2B customers', t.isDark ? '#7fe6ff' : '#00799e')}</>,
    '   ├─ globex',
    '   │  ├─ users',
    '   │  └─ apps',
    '   └─ initech',
  ];
  return (
    <Visual sx={{minHeight: 200}}>
      <Box
        component="pre"
        sx={{
          fontSize: {xs: '0.78rem !important', lg: '0.8125rem !important'},
          lineHeight: {xs: '1.7 !important', lg: '2.2 !important'},
        }}
      >
        {rows.map((r, i) => (
          // eslint-disable-next-line react/no-array-index-key -- static tree rows
          <Box key={i} component="span" sx={{display: 'block', whiteSpace: 'pre'}}>
            {r}
          </Box>
        ))}
      </Box>
    </Visual>
  );
}

function GitOpsVisual(): JSX.Element {
  const t = useHomeTokens();
  const commits = [
    {sha: 'a1f3', file: 'flows/login.yaml', msg: 'add passkey step'},
    {sha: '9c2e', file: 'policies/agents.yaml', msg: 'scope orders:read'},
    {sha: '4b7d', file: 'merge → main', msg: 'deployed'},
  ];
  return (
    <Visual sx={{justifyContent: 'flex-start', gap: 1.25, px: 2}}>
      <Box
        component="svg"
        viewBox="0 0 40 96"
        sx={{
          flex: 'none',
          width: 30,
          height: 96,
          fill: t.isDark ? '#0a1628' : '#fff',
          stroke: ACCENTS.electric,
          strokeWidth: 2,
        }}
      >
        <path d="M14 6v84" fill="none" stroke="rgba(54,136,255,0.55)" />
        <path d="M14 30c0 10 14 8 14 18s-14 8-14 18" fill="none" stroke="rgba(54,136,255,0.55)" />
        <circle cx="14" cy="12" r="4" />
        <circle cx="28" cy="48" r="4" />
        <circle cx="14" cy="84" r="4" />
      </Box>
      <Box
        component="ul"
        sx={{
          listStyle: 'none',
          m: 0,
          p: 0,
          display: 'flex',
          flexDirection: 'column',
          gap: 1.75,
          fontSize: '0.72rem',
          minWidth: 0,
        }}
      >
        {commits.map((c, i) => (
          <Box
            component="li"
            key={c.sha}
            sx={{
              whiteSpace: 'nowrap',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              color: i === commits.length - 1 ? 'text.primary' : 'text.secondary',
            }}
          >
            <Box
              component="code"
              sx={{color: ACCENTS.violet, mr: 0.75, p: 0, bgcolor: 'transparent', border: 0, fontSize: 'inherit'}}
            >
              {c.sha}
            </Box>
            {c.file}{' '}
            <Box component="span" sx={{color: t.muted2}}>
              · {c.msg}
            </Box>
          </Box>
        ))}
      </Box>
    </Visual>
  );
}

/* ─── Feature data ───────────────────────────────────────────────────────── */

interface Feature {
  id: string;
  category: Category;
  title: string;
  description: string;
  visual?: () => JSX.Element;
  /** Grid placement on wide screens (4-column bento). */
  area: string;
  /** Spans two columns on tablet widths. */
  wide?: boolean;
}

const FEATURES: Feature[] = [
  {
    id: 'agent',
    category: 'agents',
    title: 'Native agent identity',
    description: 'Agents as their own identity type, with delegation, token exchange, and CIBA',
    visual: AgentChainVisual,
    area: '1 / 1 / 3 / 3',
    wide: true,
  },
  {
    id: 'pq',
    category: 'trust',
    title: 'Post-quantum safe',
    description: 'ML-DSA, ML-KEM, SLH-DSA, and hybrid post-quantum TLS',
    visual: PostQuantumVisual,
    area: '1 / 3 / 4 / 4',
  },
  {
    id: 'wallet',
    category: 'trust',
    title: 'Wallet integration',
    description: 'OpenID4VCI issuance and OpenID4VP presentation',
    visual: WalletVisual,
    area: '1 / 4 / 3 / 5',
  },
  {
    id: 'mcp',
    category: 'agents',
    title: 'MCP and A2A authorization',
    description: 'Secure agent-to-tool and agent-to-agent calls',
    area: '3 / 4 / 4 / 5',
  },
  {
    id: 'flow',
    category: 'build',
    title: 'Composable flows',
    description: 'Login, registration, recovery, and step-up as flows, in a visual designer or as code',
    visual: FlowDesignerVisual,
    area: '3 / 1 / 6 / 3',
    wide: true,
  },
  {
    id: 'ext',
    category: 'build',
    title: 'Extensible',
    description: 'Extend flows and authenticators in Go',
    area: '4 / 3 / 5 / 4',
  },
  {
    id: 'deploy',
    category: 'platform',
    title: 'Deploy anywhere',
    description: 'Docker, Kubernetes, and Helm',
    visual: DeployVisual,
    area: '4 / 4 / 6 / 5',
  },
  {
    id: 'org',
    category: 'platform',
    title: 'Organization units',
    description: 'Organize users, agents, and applications in a hierarchy',
    visual: OrgTreeVisual,
    area: '5 / 3 / 9 / 4',
    wide: true,
  },
  {
    id: 'git',
    category: 'platform',
    title: 'GitOps',
    description: 'Declarative, versioned identity configuration',
    visual: GitOpsVisual,
    area: '6 / 1 / 8 / 3',
    wide: true,
  },
  {
    id: 'con',
    category: 'platform',
    title: 'Console',
    description: 'Optional web console for the control plane, to configure identities, applications, and flows',
    area: '6 / 4 / 7 / 5',
    wide: true,
  },
  {
    id: 'boot',
    category: 'platform',
    title: 'Lightweight runtime',
    description: 'Written in Go, headless, API-first, and starts in under 50 ms',
    visual: BootVisual,
    area: '7 / 4 / 9 / 5',
  },
  {
    id: 'proto',
    category: 'trust',
    title: 'Standard protocols',
    description: 'OAuth 2.1, OpenID Connect, SAML, and SCIM',
    area: '8 / 1 / 9 / 2',
  },
  {
    id: 'authz',
    category: 'trust',
    title: 'Authorization',
    description: 'Roles, groups, and AuthZEN-compatible policy evaluation',
    area: '8 / 2 / 9 / 3',
  },
];

const SDK_CATEGORY: Category = 'build';
const TOTAL = FEATURES.length + 1;

/* ─── Section ────────────────────────────────────────────────────────────── */

function CategoryLabel({category}: {category: Category}): JSX.Element {
  const t = useHomeTokens();
  return (
    <Typography
      component="span"
      sx={{
        display: 'block',
        fontFamily: MONO,
        fontSize: '0.72rem',
        letterSpacing: '0.06em',
        textTransform: 'uppercase',
        color: t.cyan,
      }}
    >
      {CATEGORY_LABELS[category]}
    </Typography>
  );
}

export default function FeaturesSection(): JSX.Element {
  const t = useHomeTokens();
  const [filter, setFilter] = useState<Filter>('all');
  const {siteConfig} = useDocusaurusContext();
  const productName = (siteConfig.customFields?.product as DocusaurusProductConfig).project.name;

  const counts = useMemo(() => {
    const c: Record<Filter, number> = {all: TOTAL, agents: 0, trust: 0, platform: 0, build: 1};
    FEATURES.forEach((f) => {
      c[f.category] += 1;
    });
    return c;
  }, []);

  const tileSx = (category: Category) => {
    const match = filter === 'all' || filter === category;
    return {
      ...cardSx(t),
      display: 'flex',
      flexDirection: 'column',
      '&:hover': match ? {transform: 'translateY(-4px)', bgcolor: t.surfaceStrong} : {},
      ...(match
        ? filter !== 'all' && {'& > [aria-hidden]': {borderColor: 'rgba(54,136,255,0.4)'}}
        : {opacity: '0.18 !important', filter: 'saturate(0.2)'}),
    };
  };

  return (
    <Section id="features">
      <Container maxWidth={false} sx={{maxWidth: HOME_MAX_WIDTH, px: {xs: 2, sm: 4}}}>
        <SectionHeading eyebrow="Features" title={`${productName} features`} />

        <Reveal delay={120} sx={{mt: {xs: -2, md: -3}, mb: 3.5}}>
          <Box
            role="toolbar"
            aria-label="Filter features"
            sx={{
              display: 'inline-flex',
              maxWidth: '100%',
              overflowX: 'auto',
              scrollbarWidth: 'none',
              gap: 0.75,
              p: 0.625,
              borderRadius: '14px',
              bgcolor: t.isDark ? 'rgba(4,10,22,0.6)' : 'rgba(255,255,255,0.8)',
              border: '1px solid',
              borderColor: t.isDark ? '#1f3350' : t.lineStrong,
              backdropFilter: 'blur(12px)',
              '&::-webkit-scrollbar': {display: 'none'},
            }}
          >
            {(['all', 'agents', 'trust', 'platform', 'build'] as Filter[]).map((f) => {
              const on = filter === f;
              return (
                <ButtonBase
                  key={f}
                  aria-pressed={on}
                  onClick={() => setFilter(f)}
                  sx={{
                    flex: 'none',
                    gap: 1,
                    px: 1.75,
                    py: 1,
                    borderRadius: '10px',
                    border: '1px solid',
                    borderColor: on ? 'rgba(54,136,255,0.45)' : 'transparent',
                    bgcolor: on ? 'rgba(54,136,255,0.16)' : 'transparent',
                    fontSize: '0.875rem',
                    fontWeight: 600,
                    whiteSpace: 'nowrap',
                    color: on ? 'text.primary' : 'text.secondary',
                    transition: 'color 0.2s, background-color 0.2s, border-color 0.2s',
                    '&:hover': {color: 'text.primary'},
                  }}
                >
                  {f === 'all' ? 'All' : CATEGORY_LABELS[f]}
                  <Box
                    component="i"
                    sx={{
                      fontStyle: 'normal',
                      fontFamily: MONO,
                      fontSize: '0.6875rem',
                      px: 0.875,
                      borderRadius: '999px',
                      border: '1px solid',
                      borderColor: t.lineStrong,
                      color: 'text.secondary',
                    }}
                  >
                    {counts[f]}
                  </Box>
                </ButtonBase>
              );
            })}
          </Box>
        </Reveal>

        <Box
          sx={(theme) => ({
            display: 'grid',
            gap: 1.75,
            gridTemplateColumns: '1fr',
            [theme.breakpoints.up('sm')]: {gridTemplateColumns: 'repeat(2, 1fr)', gridAutoFlow: 'dense'},
            [theme.breakpoints.up('lg')]: {gridTemplateColumns: 'repeat(4, 1fr)', gridAutoRows: 'minmax(140px, auto)'},
          })}
        >
          {FEATURES.map((f, i) => (
            <Reveal
              key={f.id}
              component="article"
              delay={(i % 4) * 60}
              sx={[
                tileSx(f.category),
                (theme: Theme) => ({
                  p: f.visual ? '14px 14px 22px' : '22px 22px 24px',
                  [theme.breakpoints.up('sm')]: {
                    gridColumn: f.wide ? 'span 2' : 'auto',
                    order: f.visual ? 0 : 1,
                  },
                  [theme.breakpoints.up('lg')]: {gridArea: f.area, order: 0},
                }),
              ]}
            >
              {f.visual && <f.visual />}
              <Box sx={{px: f.visual ? 1 : 0, mt: f.visual ? 2 : 0}}>
                <CategoryLabel category={f.category} />
                <Typography
                  component="h3"
                  sx={{
                    m: 0,
                    mt: 0.75,
                    fontFamily: DISPLAY_FONT,
                    fontSize: '1.0625rem',
                    fontWeight: 600,
                    letterSpacing: '-0.01em',
                  }}
                >
                  {f.title}
                </Typography>
                <Typography sx={{mt: 0.75, fontSize: '0.9rem', lineHeight: 1.55, color: 'text.secondary'}}>
                  {f.description}
                </Typography>
              </Box>
            </Reveal>
          ))}

          <Reveal
            component="article"
            sx={[
              tileSx(SDK_CATEGORY),
              (theme: Theme) => ({
                p: '14px',
                gridColumn: '1 / -1',
                [theme.breakpoints.up('sm')]: {order: 2},
                [theme.breakpoints.up('lg')]: {gridRow: 9, order: 0},
              }),
            ]}
          >
            <SDKShowcase />
          </Reveal>
        </Box>
      </Container>
    </Section>
  );
}
