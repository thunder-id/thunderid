// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Typography} from '@wso2/oxygen-ui';
import {BadgeCheck, Bot, KeyRound, Server} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useRef, useState} from 'react';

// Fixed canvas, scaled to fit the available width via the ResizeObserver below.
// Below MIN_SCALE it stops shrinking and scrolls horizontally so labels stay legible.
const W = 1100;
const H = 380;
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

// The agent is the subject the three concepts describe, so it carries the accent.
const highlightSx = {
  border: '1.5px solid var(--ifm-color-primary)',
  background: 'color-mix(in srgb, var(--ifm-color-primary) 6%, var(--ifm-background-surface-color))',
} as const;

const labelSx = {fontSize: '0.85rem', fontWeight: 700, color: 'var(--ifm-font-color-base)'};
const subLabelSx = {fontSize: '0.68rem', fontWeight: 500, color: 'var(--ifm-color-emphasis-700)', lineHeight: 1.3, textAlign: 'center'} as const;

/** A relationship label sitting on a connector: plain words, read as subject verb object. */
function EdgeLabel({left, top, width, text}: {left: number; top: number; width: number; text: string}) {
  return (
    <Typography
      component="span"
      sx={{position: 'absolute', left, top, width, textAlign: 'center', fontSize: '0.72rem', fontWeight: 600, lineHeight: 1.35, color: 'var(--ifm-color-emphasis-700)'}}
    >
      {text}
    </Typography>
  );
}

/**
 * The three concepts that describe an agent's authority, as a structural map
 * rather than a runtime flow: the credential it authenticates with, the role it
 * holds, and the resource server that role grants access to. Edge labels read
 * as "subject verb object" so the diagram can be read aloud. Matches the visual
 * system of the solution architecture diagram on the overview. Built with
 * Oxygen UI + Lucide icons; scales to fit the width, scrolls below MIN_SCALE.
 *
 * Why this is not a Mermaid block: it matches the visual system of the B2C
 * solution diagram (accent-highlighted cards, per-card sub-labels, and a fixed
 * canvas that scales to the container). Mermaid gives no control over card
 * styling, label placement, or connector routing, so the two showcase diagrams
 * in this section are hand-built while the process and sequence diagrams stay
 * in Mermaid, where its grammar fits.
 */
export function AgentAuthorityDiagram() {
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
      aria-label="The three concepts that describe an agent's authority. The agent authenticates with a credential, either a client secret or a private key JWT. The agent holds a role. That role grants access to a resource server, the API behind the agent's tools."
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
              <marker id="aad-arrow" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
                <path d="M0,0 L10,5 L0,10 z" fill="var(--ifm-color-emphasis-500)" />
              </marker>
            </defs>
            {/* agent -> credential (curves up) */}
            <path d="M280,168 C350,168 350,96 418,96" style={lineSx} markerEnd="url(#aad-arrow)" />
            {/* agent -> role (curves down) */}
            <path d="M280,208 C350,208 350,272 418,272" style={lineSx} markerEnd="url(#aad-arrow)" />
            {/* role -> resource server */}
            <path d="M638,272 H738" style={lineSx} markerEnd="url(#aad-arrow)" />
          </Box>

          {/* The agent: the subject the three concepts describe */}
          <Box sx={{...cardSx, ...highlightSx, left: 60, top: 148, width: 220, height: 80}}>
            <Box sx={{display: 'flex', alignItems: 'center', gap: '0.5rem'}}>
              <Box aria-hidden sx={iconSx}><Bot size={22} /></Box>
              <Typography component="span" sx={labelSx}>Your AI agent</Typography>
            </Box>
          </Box>

          {/* 1. Credential */}
          <EdgeLabel left={296} top={104} width={126} text="authenticates with" />
          <Box sx={{...cardSx, left: 420, top: 52, width: 260, height: 88}}>
            <Box sx={{display: 'flex', alignItems: 'center', gap: '0.5rem'}}>
              <Box aria-hidden sx={iconSx}><KeyRound size={20} /></Box>
              <Typography component="span" sx={labelSx}>Credential</Typography>
            </Box>
            <Typography component="span" sx={subLabelSx}>a client secret or a private key JWT</Typography>
          </Box>

          {/* 2. Role */}
          <EdgeLabel left={306} top={226} width={106} text="holds" />
          <Box sx={{...cardSx, left: 420, top: 232, width: 218, height: 80}}>
            <Box sx={{display: 'flex', alignItems: 'center', gap: '0.5rem'}}>
              <Box aria-hidden sx={iconSx}><BadgeCheck size={20} /></Box>
              <Typography component="span" sx={labelSx}>Role</Typography>
            </Box>
            <Typography component="span" sx={subLabelSx}>a bundle of permissions</Typography>
          </Box>

          {/* 3. Resource server */}
          <EdgeLabel left={640} top={244} width={96} text="grants access to" />
          <Box sx={{...cardSx, left: 740, top: 226, width: 290, height: 92}}>
            <Box sx={{display: 'flex', alignItems: 'center', gap: '0.5rem'}}>
              <Box aria-hidden sx={iconSx}><Server size={20} /></Box>
              <Typography component="span" sx={labelSx}>Resource server</Typography>
            </Box>
            <Typography component="span" sx={subLabelSx}>the API behind your agent&rsquo;s tools</Typography>
          </Box>

        </Box>
        </Box>
      </Box>
    </Box>
  );
}
