// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Typography} from '@wso2/oxygen-ui';
import {Bot, KeyRound, Server, ShieldCheck, User} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useRef, useState} from 'react';

// Fixed canvas, scaled to fit the available width via the ResizeObserver below.
// Below MIN_SCALE it stops shrinking and scrolls horizontally so labels stay legible.
const W = 1220;
const H = 600;
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

// The identity layer is the piece the reader owns and wires, so it gets the accent.
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

const IDENTITY_SUBTITLE = 'Keeps the assistant tied to the customer who brought it in';

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
      {/* At least as tall as the badge, so a single line centres on the number; longer text still starts at the top. */}
      <Typography component="span" sx={{minHeight: 20, display: 'flex', alignItems: 'center', fontSize: '0.72rem', fontWeight: 600, lineHeight: 1.35, color: accent ? 'var(--ifm-color-primary)' : 'var(--ifm-color-emphasis-700)'}}>
        {text}
      </Typography>
    </Box>
  );
}

/**
 * The bring-your-own-assistant solution diagram, built to match the agent one.
 * Only the identity layer is accented: the assistant belongs to the customer,
 * not the reader. The identity layer's box names its jobs in vendor-neutral
 * terms: it records whose assistant it is, lets it act only for that customer,
 * issues tokens naming both, and lets both sides govern it. The numbered steps
 * trace one assistant from registration to an API call: the customer
 * registers it (1), the assistant asks for access (2), the identity layer asks
 * the customer to sign in and approve, and they do (3), the identity layer mints a token for that customer only
 * (4), and the assistant sends it with each call, where the APIs check it (5).
 * Built with Oxygen UI + Lucide icons; scales to fit the width, scrolls below
 * MIN_SCALE.
 */
export function PersonalAssistantSolutionDiagram() {
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
      aria-label="Bring-your-own-assistant solution, as a numbered flow. The identity layer keeps the assistant tied to the customer who brought it in: it records whose assistant it is, lets it act only for that customer, issues tokens naming the customer and the assistant, and lets both the customer and the bank govern it. Step 1: the customer registers the assistant as their own. Step 2: the assistant asks the identity layer for access for its owner. Step 3: the identity layer asks the customer to sign in and approve what the assistant asks for, and the customer does. Step 4: the identity layer issues a signed token for that customer only, naming the customer and the assistant, and returns it to the assistant. Step 5: the assistant sends the token with each call to the bank's APIs, which check it. Separately, the customer uses the assistant for everyday tasks."
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
              <marker id="pas-arrow" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
                <path d="M0,0 L10,5 L0,10 z" fill="var(--ifm-color-emphasis-500)" />
              </marker>
            </defs>
            {/* 1: customer -> identity layer, registers the assistant */}
            <Box component="path" d="M865,98 H574" sx={lineSx} markerEnd="url(#pas-arrow)" />
            {/* 3: identity layer <-> customer, asks them to sign in and approve, and they answer */}
            <Box component="path" d="M574,120 H864" sx={lineSx} markerEnd="url(#pas-arrow)" markerStart="url(#pas-arrow)" />
            {/* customer -> assistant (uses it for everyday tasks) */}
            <Box component="path" d="M940,150 V273" sx={lineSx} markerEnd="url(#pas-arrow)" />
            {/* 2: assistant -> identity layer, asks for access (top lane, above the token) */}
            <Box component="path" d="M820,292 H574" sx={lineSx} markerEnd="url(#pas-arrow)" />
            {/* 4: identity layer -> token -> assistant (bottom lane: minted here, handed over) */}
            <Box component="path" d="M570,344 H611" sx={lineSx} markerEnd="url(#pas-arrow)" />
            <Box component="path" d="M790,344 H818" sx={lineSx} markerEnd="url(#pas-arrow)" />
            {/* 5: assistant <-> Bank's APIs, down the spine */}
            <Box component="path" d="M940,355 V466" sx={lineSx} markerEnd="url(#pas-arrow)" markerStart="url(#pas-arrow)" />
          </Box>

          {/* The customer (top of the spine) */}
          <Box sx={{...cardSx, left: 865, top: 66, width: 150, height: 84, borderRadius: '999px'}}>
            <Box aria-hidden sx={iconSx}><User size={26} /></Box>
            <Typography component="span" sx={labelSx}>A customer</Typography>
          </Box>
          <StepLabel n={1} left={590} top={72} width={270} text="Registers the assistant as their own" />
          <StepLabel n={3} left={590} top={128} width={270} text="Signs in and approves what the assistant asks for" />
          <Typography component="span" sx={{position: 'absolute', left: 950, top: 196, width: 200, fontSize: '0.72rem', fontWeight: 600, lineHeight: 1.4, color: 'var(--ifm-color-emphasis-700)'}}>
            Uses it for everyday tasks
          </Typography>

          {/* The customer's assistant (not the reader's, so not accented) */}
          <Box sx={{...cardSx, left: 820, top: 277, width: 240, height: 78, gap: '0.15rem'}}>
            <Box sx={{display: 'flex', alignItems: 'center', gap: '0.5rem'}}>
              <Box aria-hidden sx={iconSx}><Bot size={22} /></Box>
              <Typography component="span" sx={labelSx}>Their AI assistant</Typography>
            </Box>
            <Typography component="span" sx={subLabelSx}>brought in by the customer</Typography>
          </Box>

          {/* Identity layer: the centrepiece, accented. */}
          <Box sx={{...cardSx, ...highlightSx, left: 50, top: 60, width: 520, height: 380, alignItems: 'stretch', justifyContent: 'flex-start', gap: '0.45rem', padding: '1.1rem 1.25rem'}}>
            <Box sx={{display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '0.5rem'}}>
              <Box aria-hidden sx={iconSx}><ShieldCheck size={26} /></Box>
              <Typography component="span" sx={{...labelSx, fontSize: '1rem'}}>Identity layer</Typography>
            </Box>
            <Typography component="span" sx={{...subLabelSx, marginBottom: '0.2rem'}}>{IDENTITY_SUBTITLE}</Typography>
            <Box sx={stepSx}>
              <Typography component="span" sx={stepTitleSx}>Records whose assistant it is</Typography>
              <Typography component="span" sx={subLabelSx}>an identity of its own, owned by one customer</Typography>
            </Box>
            <Box sx={stepSx}>
              <Typography component="span" sx={stepTitleSx}>Lets it act only for that customer</Typography>
              <Typography component="span" sx={subLabelSx}>after they sign in and approve</Typography>
            </Box>
            <Box sx={stepSx}>
              <Typography component="span" sx={stepTitleSx}>Issues its tokens</Typography>
              <Typography component="span" sx={subLabelSx}>naming the customer and the assistant</Typography>
            </Box>
            <Box sx={stepSx}>
              <Typography component="span" sx={stepTitleSx}>Lets both sides govern it</Typography>
              <Typography component="span" sx={subLabelSx}>the customer and the bank</Typography>
            </Box>
          </Box>
          <StepLabel n={2} left={590} top={248} width={220} text="Asks for access for its owner" />

          {/* Signed token: minted by the identity layer, for the owner only */}
          <StepLabel n={4} left={600} top={384} width={220} text="Issued for that customer only" accent />
          <Box sx={{...cardSx, left: 615, top: 311, width: 175, height: 66, gap: '0.15rem', border: '1.5px solid var(--ifm-color-primary)', background: 'color-mix(in srgb, var(--ifm-color-primary) 10%, var(--ifm-background-surface-color))'}}>
            <Box sx={{display: 'flex', alignItems: 'center', gap: '0.35rem'}}>
              <Box aria-hidden sx={iconSx}><KeyRound size={18} /></Box>
              <Typography component="span" sx={{...labelSx, fontSize: '0.82rem'}}>Signed token</Typography>
            </Box>
            <Typography component="span" sx={subLabelSx}>the customer, through the assistant</Typography>
          </Box>

          {/* Bank's APIs (bottom of the spine) */}
          <StepLabel n={5} left={955} top={398} width={210} text="Sent with each call, and checked by the bank's APIs" />
          <Box sx={{...cardSx, left: 815, top: 470, width: 250, height: 92}}>
            <Box sx={{display: 'flex', alignItems: 'center', gap: '0.5rem'}}>
              <Box aria-hidden sx={iconSx}><Server size={22} /></Box>
              <Typography component="span" sx={labelSx}>{"Bank's APIs"}</Typography>
            </Box>
            <Typography component="span" sx={subLabelSx}>the services the assistant calls</Typography>
          </Box>

        </Box>
        </Box>
      </Box>
    </Box>
  );
}
