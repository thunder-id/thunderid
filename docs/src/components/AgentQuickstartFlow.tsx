/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {Box, Typography} from '@wso2/oxygen-ui';
import {BookOpen, Bot, GraduationCap, ShieldCheck, Sparkles, User, Zap} from '@wso2/oxygen-ui-icons-react';
import React from 'react';

// Theme-aware building blocks shared by both quickstart flow figures. Every
// colour is a CSS variable, so the diagrams follow the light/dark toggle.
const surfaceCardSx = {
  position: 'absolute',
  borderRadius: '14px',
  border: '1px solid var(--ifm-color-emphasis-200)',
  background: 'var(--ifm-background-surface-color)',
  boxShadow: '0 6px 18px color-mix(in srgb, var(--ifm-color-emphasis-900) 6%, transparent)',
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'center',
  justifyContent: 'center',
  gap: '0.4rem',
};

const agentCardSx = {
  position: 'absolute',
  borderRadius: '14px',
  color: '#fff',
  background:
    'linear-gradient(150deg, color-mix(in srgb, var(--ifm-color-primary) 92%, #fff), color-mix(in srgb, var(--ifm-color-primary) 62%, #b3540a))',
  boxShadow: '0 14px 30px color-mix(in srgb, var(--ifm-color-primary) 34%, transparent)',
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'center',
  justifyContent: 'center',
  gap: '0.35rem',
};

const cardTitleSx = {fontSize: '0.95rem', fontWeight: 800};
const cardSubtitleSx = {fontSize: '0.82rem', fontWeight: 700, color: 'var(--ifm-color-emphasis-800)'};
const iconStrokeSx = {'& svg': {fill: 'none', stroke: 'currentColor', strokeLinecap: 'round', strokeLinejoin: 'round', strokeWidth: 1.7}};

const lineSx = {stroke: 'var(--ifm-color-emphasis-400)', strokeWidth: 1.7, fill: 'none'};

// A numbered pill that sits on a connector to label the ordered steps.
function FlowLabel({n, text, left, top}: {n: number; text: string; left: number; top: number}) {
  return (
    <Box
      sx={{
        position: 'absolute',
        left,
        top,
        display: 'inline-flex',
        alignItems: 'center',
        gap: '0.4rem',
        padding: '0.15rem 0.5rem 0.15rem 0.2rem',
        borderRadius: '999px',
        background: 'var(--ifm-background-surface-color)',
        border: '1px solid var(--ifm-color-emphasis-200)',
        boxShadow: '0 2px 6px color-mix(in srgb, var(--ifm-color-emphasis-900) 5%, transparent)',
        whiteSpace: 'nowrap',
      }}
    >
      <Box
        component="span"
        sx={{
          display: 'inline-flex',
          alignItems: 'center',
          justifyContent: 'center',
          width: '1.15rem',
          height: '1.15rem',
          borderRadius: '999px',
          background: 'var(--ifm-color-primary)',
          color: '#fff',
          fontSize: '0.68rem',
          fontWeight: 800,
        }}
      >
        {n}
      </Box>
      <Typography component="span" sx={{fontSize: '0.74rem', fontWeight: 700, color: 'var(--ifm-color-emphasis-800)'}}>
        {text}
      </Typography>
    </Box>
  );
}

function ThunderNode({left, top, width = 150, height = 80}: {left: number; top: number; width?: number; height?: number}) {
  return (
    <Box sx={{...surfaceCardSx, left, top, width, height}}>
      <Box aria-hidden sx={{...iconStrokeSx, color: 'var(--ifm-color-primary)'}}><Zap size={24} /></Box>
      <Typography component="span" sx={{...cardTitleSx, color: 'var(--ifm-font-color-base)'}}>ThunderID</Typography>
    </Box>
  );
}

function UserNode({left, top, width = 125, height = 86}: {left: number; top: number; width?: number; height?: number}) {
  return (
    <Box sx={{...surfaceCardSx, left, top, width, height}}>
      <Box aria-hidden sx={{...iconStrokeSx, color: 'var(--ifm-color-emphasis-700)'}}><User size={24} /></Box>
      <Typography component="span" sx={{...cardTitleSx, color: 'var(--ifm-font-color-base)'}}>User</Typography>
    </Box>
  );
}

function AgentNode({left, top, width = 150, height = 86}: {left: number; top: number; width?: number; height?: number}) {
  return (
    <Box sx={{...agentCardSx, left, top, width, height}}>
      <Box sx={{position: 'absolute', top: 8, right: 8, display: 'inline-flex', alignItems: 'center', gap: '0.2rem', padding: '0.12rem 0.4rem', borderRadius: '999px', background: 'rgba(255,255,255,0.22)', fontSize: '0.62rem', fontWeight: 700}}>
        <Sparkles size={11} /> AI
      </Box>
      <Box aria-hidden sx={iconStrokeSx}><Bot size={28} /></Box>
      <Typography component="span" sx={cardTitleSx}>Agent</Typography>
    </Box>
  );
}

// The "Tool" grouping: a gate that validates, then the resource it reaches.
function ToolNode({left, top, width = 168, height = 168, action, actionIcon}: {left: number; top: number; width?: number; height?: number; action: string; actionIcon: React.ReactNode}) {
  return (
    <Box sx={{...surfaceCardSx, left, top, width, height, flexDirection: 'column', justifyContent: 'center', gap: '0.15rem', padding: '0.5rem 0'}}>
      <Box sx={{display: 'flex', alignItems: 'center', gap: '0.4rem', fontSize: '0.82rem', fontWeight: 800, color: 'var(--ifm-font-color-base)', marginBottom: '0.35rem'}}>
        🛠️ Tool
      </Box>
      <Box sx={{display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '0.35rem', ...iconStrokeSx, color: 'var(--ifm-color-emphasis-700)'}}>
        <ShieldCheck size={24} />
        <Typography component="span" sx={cardSubtitleSx}>Validate</Typography>
      </Box>
      <Box aria-hidden sx={{color: 'var(--ifm-color-emphasis-500)', fontSize: '1rem', lineHeight: 1, margin: '0.15rem 0'}}>↓</Box>
      <Box sx={{display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '0.35rem', ...iconStrokeSx, color: 'var(--ifm-color-emphasis-700)'}}>
        {actionIcon}
        <Typography component="span" sx={cardSubtitleSx}>{action}</Typography>
      </Box>
    </Box>
  );
}

function Figure({label, width, height, children}: {label: string; width: number; height: number; children: React.ReactNode}) {
  return (
    <Box component="figure" aria-label={label} sx={{margin: '1.5rem 0 2rem', border: 0, padding: 0}}>
      <Box sx={{overflowX: 'auto', overflowY: 'hidden', WebkitOverflowScrolling: 'touch', padding: '0.25rem'}}>
        <Box sx={{position: 'relative', width, height, minWidth: width, margin: '0 auto'}}>{children}</Box>
      </Box>
    </Box>
  );
}

const arrowDefs = (id: string) => (
  <defs>
    <marker id={id} viewBox="0 0 10 10" refX="8" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
      <path d="M0,0 L10,5 L0,10 z" fill="var(--ifm-color-emphasis-500)" />
    </marker>
  </defs>
);

// ── Acting on its own ─────────────────────────────────────────────────────
// The user asks a question; the agent gets its own token from ThunderID, then
// calls the tool with it. ThunderID sits above the Agent so the token exchange
// reads as a short round trip.
export function AgentOwnTokenFlow() {
  const W = 600;
  const H = 320;
  return (
    <Figure label="The user asks; the agent gets its own token from ThunderID and calls a tool with it" width={W} height={H}>
      <Box component="svg" viewBox={`0 0 ${W} ${H}`} width={W} height={H} aria-hidden sx={{position: 'absolute', inset: 0, pointerEvents: 'none'}}>
        {arrowDefs('own-arrow')}
        {/* User → Agent: asks */}
        <path d="M108,246 H204" style={lineSx} markerEnd="url(#own-arrow)" />
        {/* Agent → ThunderID: get token (up) */}
        <path d="M250,205 V92" style={lineSx} markerEnd="url(#own-arrow)" />
        {/* ThunderID → Agent: access token (down) */}
        <path d="M300,86 V199" style={lineSx} markerEnd="url(#own-arrow)" />
        {/* Agent → Tool: calls tool */}
        <path d="M340,246 H449" style={lineSx} markerEnd="url(#own-arrow)" />
      </Box>

      <ThunderNode left={210} top={12} width={130} height={72} />
      <UserNode left={8} top={205} width={100} height={82} />
      <AgentNode left={210} top={205} width={130} height={82} />
      <ToolNode left={455} top={78} width={140} action="List modules" actionIcon={<BookOpen size={24} />} />

      <FlowLabel n={1} text="asks" left={122} top={204} />
      <FlowLabel n={2} text="get token" left={136} top={132} />
      <FlowLabel n={3} text="access token" left={310} top={132} />
      <FlowLabel n={4} text="calls tool" left={350} top={204} />
    </Figure>
  );
}

// ── Acting on behalf of a user ────────────────────────────────────────────
// The user asks; the agent prompts them to sign in, the user signs in to
// ThunderID, ThunderID returns a delegated token, and the agent then calls the
// tool with the user's authority.
export function AgentOboFlow() {
  const W = 600;
  const H = 340;
  return (
    <Figure label="The agent has the user authenticate at ThunderID, gets a delegated token, and calls a tool with the user's authority" width={W} height={H}>
      <Box component="svg" viewBox={`0 0 ${W} ${H}`} width={W} height={H} aria-hidden sx={{position: 'absolute', inset: 0, pointerEvents: 'none'}}>
        {arrowDefs('obo-arrow')}
        {/* Agent → User: authenticate (up) */}
        <path d="M90,230 V106" style={lineSx} markerEnd="url(#obo-arrow)" />
        {/* User → ThunderID: signs in (right) */}
        <path d="M155,58 H379" style={lineSx} markerEnd="url(#obo-arrow)" />
        {/* ThunderID → Agent: delegated token (elbow down-left, kept clear of the Tool box) */}
        <path d="M398,100 V152 H150 V224" style={lineSx} markerEnd="url(#obo-arrow)" />
        {/* Agent → Tool: calls tool */}
        <path d="M165,283 H414" style={lineSx} markerEnd="url(#obo-arrow)" />
      </Box>

      <UserNode left={25} top={24} width={130} height={76} />
      <ThunderNode left={385} top={24} width={160} height={76} />
      <AgentNode left={25} top={230} width={140} height={90} />
      <ToolNode left={420} top={158} width={140} action="Enroll module" actionIcon={<GraduationCap size={24} />} />

      <FlowLabel n={1} text="ask to sign in" left={6} top={150} />
      <FlowLabel n={2} text="signs in" left={208} top={22} />
      <FlowLabel n={3} text="delegated token" left={200} top={112} />
      <FlowLabel n={4} text="calls tool" left={252} top={239} />
    </Figure>
  );
}
