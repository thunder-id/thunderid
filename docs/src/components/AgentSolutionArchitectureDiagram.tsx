// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Typography} from '@wso2/oxygen-ui';
import {Bot, KeyRound, Server, ShieldCheck, User} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useRef, useState} from 'react';

// Fixed canvas, scaled to fit the available width via the ResizeObserver below.
// Below MIN_SCALE it stops shrinking and scrolls horizontally so labels stay legible.
const W = 1220;
const H = 620;
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

const IDENTITY_SUBTITLE = 'Everything about the agent’s identity, in one place';

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
 * The agent solution architecture diagram, built to match the B2C one.
 * Two accented pieces are what the reader owns and wires together: their agent
 * and the identity layer. The identity layer's box names its real jobs in
 * product terms: it registers the agent as a principal, decides what the agent
 * may reach through roles, and issues the tokens the agent presents. The
 * numbered steps trace one call: the agent asks for a token (1), the identity
 * layer mints one naming who is acting and on whose authority (2), the agent
 * presents it on the tool call (3), and the API verifies it and authorizes
 * against what it carries (4). A person appears only when the agent is acting
 * for someone, which is what puts a second name in the token. Built with
 * Oxygen UI + Lucide icons; scales to fit the width, scrolls below MIN_SCALE.
 *
 * Why this is not a Mermaid block: Mermaid cannot place a first-class artifact
 * node between two boxes on parallel request and response lanes, nor accent the
 * pieces the reader owns, nor hold a fixed canvas that scales to the container.
 * The process and sequence diagrams elsewhere in this section remain Mermaid.
 */
export function AgentSolutionArchitectureDiagram() {
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
      aria-label="Agent solution architecture, as a numbered flow. The identity layer owns everything about the agent's identity in one place: it registers the agent as a principal with its own credential, decides what the agent may reach through the roles you assign it, and issues the tokens the agent presents. Step 1: the agent asks the identity layer for a token. Step 2: the identity layer mints a signed token naming who is acting and on whose authority, and returns it to the agent. Step 3: the agent presents that token on the tool call to your API. Step 4: the API verifies the token and authorizes the call against what it carries. A person appears only when the agent is acting on someone's behalf, and their sign-in is what adds a second name to the token."
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
              <marker id="asa-arrow" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
                <path d="M0,0 L10,5 L0,10 z" fill="var(--ifm-color-emphasis-500)" />
              </marker>
            </defs>
            {/* person -> agent (only when the agent acts for someone) */}
            <path d="M940,150 V276" style={lineSx} markerEnd="url(#asa-arrow)" />
            {/* 1: agent -> identity layer, asks for a token (top lane, above the token) */}
            <path d="M820,282 H574" style={lineSx} markerEnd="url(#asa-arrow)" />
            {/* 2: identity layer -> token -> agent (bottom lane: minted here, handed over) */}
            <path d="M570,344 H611" style={lineSx} markerEnd="url(#asa-arrow)" />
            <path d="M790,344 H816" style={lineSx} markerEnd="url(#asa-arrow)" />
            {/* 3 and 4: agent <-> your APIs, down the spine */}
            <path d="M940,355 V466" style={lineSx} markerEnd="url(#asa-arrow)" markerStart="url(#asa-arrow)" />
          </Box>

          {/* The person (top of the spine, optional participant) */}
          <Box sx={{...cardSx, left: 865, top: 66, width: 150, height: 84, borderRadius: '999px'}}>
            <Box aria-hidden sx={iconSx}><User size={26} /></Box>
            <Typography component="span" sx={labelSx}>A person</Typography>
          </Box>
          <Typography component="span" sx={{position: 'absolute', left: 1000, top: 176, width: 200, fontSize: '0.72rem', fontWeight: 600, lineHeight: 1.4, color: 'var(--ifm-color-emphasis-700)'}}>
            Only when the agent acts on someone&rsquo;s behalf
          </Typography>

          {/* The agent (owned by the reader, accented) */}
          <Box sx={{...cardSx, ...highlightSx, left: 820, top: 277, width: 240, height: 78}}>
            <Box sx={{display: 'flex', alignItems: 'center', gap: '0.5rem'}}>
              <Box aria-hidden sx={iconSx}><Bot size={22} /></Box>
              <Typography component="span" sx={labelSx}>Your AI agent</Typography>
            </Box>
          </Box>

          {/* Identity layer: the centrepiece, accented. */}
          <Box sx={{...cardSx, ...highlightSx, left: 50, top: 126, width: 520, height: 286, alignItems: 'stretch', justifyContent: 'flex-start', gap: '0.45rem', padding: '1.1rem 1.25rem'}}>
            <Box sx={{display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '0.5rem'}}>
              <Box aria-hidden sx={iconSx}><ShieldCheck size={26} /></Box>
              <Typography component="span" sx={{...labelSx, fontSize: '1rem'}}>Identity layer</Typography>
            </Box>
            <Typography component="span" sx={{...subLabelSx, marginBottom: '0.2rem'}}>{IDENTITY_SUBTITLE}</Typography>
            <Box sx={stepSx}>
              <Typography component="span" sx={stepTitleSx}>Registers the agent</Typography>
              <Typography component="span" sx={subLabelSx}>a principal with its own credential and owner</Typography>
            </Box>
            <Box sx={stepSx}>
              <Typography component="span" sx={stepTitleSx}>Decides what it may reach</Typography>
              <Typography component="span" sx={subLabelSx}>through the roles you assign it</Typography>
            </Box>
            <Box sx={stepSx}>
              <Typography component="span" sx={stepTitleSx}>Issues its tokens</Typography>
              <Typography component="span" sx={subLabelSx}>naming the agent, the person, or both</Typography>
            </Box>
          </Box>
          <StepLabel n={1} left={590} top={226} width={240} text="Asks for a token for this call" />

          {/* Signed token: minted by the identity layer, carries who is acting */}
          <StepLabel n={2} left={600} top={384} width={220} text="Minted by the identity layer" accent />
          <Box sx={{...cardSx, left: 615, top: 311, width: 175, height: 66, gap: '0.15rem', border: '1.5px solid var(--ifm-color-primary)', background: 'color-mix(in srgb, var(--ifm-color-primary) 10%, var(--ifm-background-surface-color))'}}>
            <Box sx={{display: 'flex', alignItems: 'center', gap: '0.35rem'}}>
              <Box aria-hidden sx={iconSx}><KeyRound size={18} /></Box>
              <Typography component="span" sx={{...labelSx, fontSize: '0.82rem'}}>Signed token</Typography>
            </Box>
            <Typography component="span" sx={subLabelSx}>who is acting, and on whose authority</Typography>
          </Box>

          {/* Your APIs (bottom of the spine) */}
          <StepLabel n={3} left={985} top={366} width={200} text="Sent with the tool call" />
          <StepLabel n={4} left={985} top={414} width={210} text="Verified, then authorized against what it carries" />
          <Box sx={{...cardSx, left: 815, top: 470, width: 250, height: 92}}>
            <Box sx={{display: 'flex', alignItems: 'center', gap: '0.5rem'}}>
              <Box aria-hidden sx={iconSx}><Server size={22} /></Box>
              <Typography component="span" sx={labelSx}>Your APIs</Typography>
            </Box>
            <Typography component="span" sx={subLabelSx}>the tools the agent calls</Typography>
          </Box>

        </Box>
        </Box>
      </Box>
    </Box>
  );
}
