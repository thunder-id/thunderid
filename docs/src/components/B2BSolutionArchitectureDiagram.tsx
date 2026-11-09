// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Typography} from '@wso2/oxygen-ui';
import {Building2, KeyRound, Server, ShieldCheck, Smartphone, User} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useRef, useState} from 'react';

// Fixed canvas, scaled to fit the available width via the ResizeObserver below.
// Below MIN_SCALE it stops shrinking and scrolls horizontally so labels stay legible.
const W = 1220;
const H = 730;
const MIN_SCALE = 0.45;

const lineSx = {stroke: 'var(--ifm-color-emphasis-400)', strokeWidth: 1.6, fill: 'none'};

const iconSx = {
  color: 'var(--ifm-color-primary)',
  '& svg': {fill: 'none', stroke: 'currentColor', strokeLinecap: 'round', strokeLinejoin: 'round', strokeWidth: 1.7},
};

const cardSx = {
  position: 'absolute',
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'center',
  justifyContent: 'center',
  gap: '0.3rem',
  borderRadius: '14px',
  border: '1px solid var(--ifm-color-emphasis-200)',
  background: 'var(--ifm-background-surface-color)',
  boxShadow: '0 6px 18px color-mix(in srgb, var(--ifm-color-emphasis-900) 6%, transparent)',
  textAlign: 'center',
  padding: '0.5rem',
} as const;

// The two pieces the reader actually owns and wires together get an accent.
const highlightSx = {
  border: '1.5px solid var(--ifm-color-primary)',
  background: 'color-mix(in srgb, var(--ifm-color-primary) 6%, var(--ifm-background-surface-color))',
} as const;

const labelSx = {fontSize: '0.85rem', fontWeight: 700, color: 'var(--ifm-font-color-base)'};
const subLabelSx = {fontSize: '0.68rem', fontWeight: 500, color: 'var(--ifm-color-emphasis-700)', lineHeight: 1.3, textAlign: 'center'} as const;
// One of the identity layer's jobs: a titled inner card.
const stepSx = {
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'center',
  gap: '0.1rem',
  border: '1px solid var(--ifm-color-emphasis-300)',
  borderRadius: '10px',
  background: 'var(--ifm-background-surface-color)',
  padding: '0.5rem 0.75rem',
  textAlign: 'center',
} as const;
const stepTitleSx = {fontSize: '0.82rem', fontWeight: 700, color: 'var(--ifm-font-color-base)'} as const;

const IDENTITY_SUBTITLE = 'Every customer company, its people, and their access, in one place';

/** One company glyph inside the company identity providers card. */
function CompanyChip() {
  return (
    <Box aria-hidden sx={{...iconSx, display: 'inline-flex', color: 'var(--ifm-color-emphasis-700)'}}>
      <Building2 size={20} />
    </Box>
  );
}

/** A numbered flow-step label: a small primary circle carrying the step number, then the text. */
function StepLabel({n, left, top, width, text, accent = false}: {n: number; left: number; top: number; width: number; text: string; accent?: boolean}) {
  return (
    <Box sx={{position: 'absolute', left, top, width, display: 'flex', gap: '0.4rem', alignItems: 'flex-start'}}>
      <Box
        component="span"
        aria-hidden
        sx={{flexShrink: 0, display: 'inline-flex', alignItems: 'center', justifyContent: 'center', width: 20, height: 20, borderRadius: '999px', background: 'var(--ifm-color-primary)', color: '#fff', fontSize: '0.68rem', fontWeight: 700}}
      >
        {n}
      </Box>
      <Typography component="span" sx={{fontSize: '0.72rem', fontWeight: 600, lineHeight: 1.35, color: accent ? 'var(--ifm-color-primary)' : 'var(--ifm-color-emphasis-700)'}}>
        {text}
      </Typography>
    </Box>
  );
}

/**
 * The B2B solution architecture diagram, built to match the B2C one. Two
 * accented pieces are what the reader owns and wires together: their B2B
 * application and the identity layer. The identity layer's box names its real
 * jobs in this solution: it holds one organization per customer company under
 * a platform organization that shares the application, roles, and flows; it
 * runs the company journeys (sign-up, invitations, sign-in) as flows; and it
 * lets each company assign the shared roles to its own people. The numbered
 * steps trace one request: an employee signs in or signs their company up
 * through the app (1), the app hands the journey to the identity layer (2),
 * which mints one signed token for one organization (3), and the app presents
 * that token to the backend APIs (4), which scope every record to that
 * organization. A company's own identity provider is an optional way to verify
 * its employees and never mints the token. Built with Oxygen UI + Lucide icons;
 * scales to fit the width, scrolls below MIN_SCALE.
 */
export function B2BSolutionArchitectureDiagram() {
  const wrapperRef = useRef<HTMLDivElement>(null);
  const [scale, setScale] = useState(1);

  useEffect(() => {
    const el = wrapperRef.current;
    if (!el) {
      return undefined;
    }
    const updateScale = () => {
      const next = Math.min(1, el.clientWidth / W);
      setScale(Math.max(next, MIN_SCALE));
    };
    updateScale();
    const observer = new ResizeObserver(updateScale);
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  const needsScroll = scale <= MIN_SCALE;

  return (
    <Box
      component="figure"
      aria-label="B2B solution architecture, as a numbered flow. The identity layer holds every customer company, its people, and their access in one place: it keeps one organization per customer company under a platform organization that shares the application, roles, and flows with each of them; it runs the company journeys (sign-up, invitations, and sign-in) as flows; and it lets each company assign the shared roles to its own people. Step 1: an employee signs in, signs their company up, or accepts an invitation through the B2B application. Step 2: the application hands the journey to the identity layer. Step 3: the identity layer mints one signed token for one organization, naming it and the roles held there, and returns it to the application. Step 4: the application presents that token to the backend APIs, which trust it and scope every record to that organization. A company's own identity provider is an optional way to verify its employees and never mints the token."
      sx={{margin: '2rem 0 2.5rem', border: 0, padding: 0, overflow: 'visible'}}
    >
      <Box
        ref={wrapperRef}
        sx={{width: '100%', height: H * scale, overflowX: needsScroll ? 'auto' : 'hidden', overflowY: 'hidden', WebkitOverflowScrolling: 'touch'}}
      >
        <Box sx={{width: W * scale, height: H * scale, overflow: 'hidden'}}>
        <Box sx={{position: 'relative', width: W, height: H, transform: `scale(${scale})`, transformOrigin: 'top left'}}>

          {/* Connector layer */}
          <Box component="svg" viewBox={`0 0 ${W} ${H}`} width={W} height={H} aria-hidden sx={{position: 'absolute', inset: 0, pointerEvents: 'none'}}>
            <defs>
              <marker id="b2b-sa-arrow" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
                <path d="M0,0 L10,5 L0,10 z" fill="var(--ifm-color-emphasis-500)" />
              </marker>
            </defs>
            {/* 1: employee -> application */}
            <path d="M940,154 V301" style={lineSx} markerEnd="url(#b2b-sa-arrow)" />
            {/* 2: application -> identity layer, hands off the journey (top lane, above the token) */}
            <path d="M820,306 H574" style={lineSx} markerEnd="url(#b2b-sa-arrow)" />
            {/* 3: identity layer -> token -> application (bottom lane: minted here, handed over) */}
            <path d="M570,368 H611" style={lineSx} markerEnd="url(#b2b-sa-arrow)" />
            <path d="M790,368 H816" style={lineSx} markerEnd="url(#b2b-sa-arrow)" />
            {/* 4: application <-> backend APIs (present token / data back), down the spine */}
            <path d="M940,379 V536" style={lineSx} markerEnd="url(#b2b-sa-arrow)" markerStart="url(#b2b-sa-arrow)" />
            {/* identity layer -> company identity providers (optional delegation) */}
            <path d="M310,436 V586" style={lineSx} markerEnd="url(#b2b-sa-arrow)" />
          </Box>

          {/* Employee of a customer company (top of the spine) */}
          <Box sx={{...cardSx, left: 855, top: 70, width: 170, height: 84, borderRadius: '999px'}}>
            <Box aria-hidden sx={iconSx}><User size={26} /></Box>
            <Typography component="span" sx={labelSx}>Company employee</Typography>
          </Box>
          <StepLabel n={1} left={1000} top={200} width={195} text="Signs in, signs a company up, or accepts an invitation" />

          {/* B2B application (owned by the reader, accented) */}
          <Box sx={{...cardSx, ...highlightSx, left: 820, top: 301, width: 240, height: 78}}>
            <Box sx={{display: 'flex', alignItems: 'center', gap: '0.5rem'}}>
              <Box aria-hidden sx={iconSx}><Smartphone size={22} /></Box>
              <Typography component="span" sx={labelSx}>Your B2B application</Typography>
            </Box>
            <Typography component="span" sx={subLabelSx}>one deployment for every company</Typography>
          </Box>

          {/* Identity layer: the centrepiece, accented. */}
          <Box sx={{...cardSx, ...highlightSx, left: 50, top: 150, width: 520, height: 286, alignItems: 'stretch', justifyContent: 'flex-start', gap: '0.45rem', padding: '1.1rem 1.25rem'}}>
            <Box sx={{display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '0.5rem'}}>
              <Box aria-hidden sx={iconSx}><ShieldCheck size={26} /></Box>
              <Typography component="span" sx={{...labelSx, fontSize: '1rem'}}>Identity layer</Typography>
            </Box>
            <Typography component="span" sx={{...subLabelSx, marginBottom: '0.2rem'}}>{IDENTITY_SUBTITLE}</Typography>
            <Box sx={stepSx}>
              <Typography component="span" sx={stepTitleSx}>Holds one organization per company</Typography>
              <Typography component="span" sx={subLabelSx}>your platform organization shares the application, roles, and flows with each</Typography>
            </Box>
            <Box sx={stepSx}>
              <Typography component="span" sx={stepTitleSx}>Runs the company journeys</Typography>
              <Typography component="span" sx={subLabelSx}>sign-up, invitations, and sign-in, built as flows</Typography>
            </Box>
            <Box sx={stepSx}>
              <Typography component="span" sx={stepTitleSx}>Assigns roles per organization</Typography>
              <Typography component="span" sx={subLabelSx}>each company grants the shared roles to its own people</Typography>
            </Box>
          </Box>
          <StepLabel n={2} left={590} top={250} width={240} text="Hands off sign-in, sign-up, and invitations" />

          {/* Signed token: minted for one organization at sign-in */}
          <StepLabel n={3} left={600} top={408} width={220} text="Minted for one organization at sign-in" accent />
          <Box sx={{...cardSx, left: 615, top: 335, width: 175, height: 66, gap: '0.15rem', border: '1.5px solid var(--ifm-color-primary)', background: 'color-mix(in srgb, var(--ifm-color-primary) 10%, var(--ifm-background-surface-color))'}}>
            <Box sx={{display: 'flex', alignItems: 'center', gap: '0.35rem'}}>
              <Box aria-hidden sx={iconSx}><KeyRound size={18} /></Box>
              <Typography component="span" sx={{...labelSx, fontSize: '0.82rem'}}>Signed token</Typography>
            </Box>
            <Typography component="span" sx={subLabelSx}>names the organization and its roles</Typography>
          </Box>

          {/* Backend APIs (bottom of the spine) */}
          <StepLabel n={4} left={985} top={430} width={200} text="Presents the token, gets that company's data" />
          <Box sx={{...cardSx, left: 815, top: 540, width: 250, height: 92}}>
            <Box sx={{display: 'flex', alignItems: 'center', gap: '0.5rem'}}>
              <Box aria-hidden sx={iconSx}><Server size={22} /></Box>
              <Typography component="span" sx={labelSx}>Backend APIs</Typography>
            </Box>
            <Typography component="span" sx={subLabelSx}>Trust the token · key every record by organization</Typography>
          </Box>

          {/* Company identity providers: one optional way to verify at sign-in */}
          <Typography component="span" sx={{position: 'absolute', left: 330, top: 486, width: 220, fontSize: '0.72rem', fontWeight: 600, lineHeight: 1.4, color: 'var(--ifm-color-emphasis-700)'}}>
            Optionally delegate to the company&rsquo;s own identity provider
          </Typography>
          <Box sx={{...cardSx, left: 160, top: 590, width: 300, height: 110}}>
            <Typography component="span" sx={labelSx}>Company identity providers</Typography>
            <Typography component="span" sx={subLabelSx}>one per organization, verification only</Typography>
            <Box sx={{display: 'flex', alignItems: 'center', gap: '0.9rem'}}>
              <CompanyChip />
              <CompanyChip />
              <CompanyChip />
            </Box>
          </Box>

        </Box>
        </Box>
      </Box>
    </Box>
  );
}
