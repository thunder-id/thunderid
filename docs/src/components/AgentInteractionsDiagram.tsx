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
import {
  AppWindow,
  Bot,
  Brain,
  Calendar,
  Cloud,
  Code2,
  Cpu,
  Database,
  FileSpreadsheet,
  GitHub,
  Mail,
  MCP,
  MessageSquare,
  MonitorSmartphone,
  MoreHorizontal,
  Settings,
  Share2,
  Sparkles,
  User,
  Video,
} from '@wso2/oxygen-ui-icons-react';
import React from 'react';

// ── Canvas geometry ──
// Fixed canvas; the figure scrolls horizontally on screens narrower than W.
// Every element (labels included) sits inside [0, W] × [0, H] with margins so
// nothing is clipped by the scroll container.
const W = 920;
const H = 560;

const lineSx = {stroke: 'var(--ifm-color-emphasis-400)', strokeWidth: 1.6, fill: 'none'};

const iconSx = {
  color: 'var(--ifm-color-emphasis-700)',
  '& svg': {
    fill: 'none',
    stroke: 'currentColor',
    strokeLinecap: 'round',
    strokeLinejoin: 'round',
    strokeWidth: 1.7,
  },
};

const groupCardSx = {
  position: 'absolute',
  borderRadius: '16px',
  border: '1px solid var(--ifm-color-emphasis-200)',
  background: 'var(--ifm-background-surface-color)',
  boxShadow: '0 6px 18px color-mix(in srgb, var(--ifm-color-emphasis-900) 6%, transparent)',
};

const groupHeaderSx = {
  display: 'flex',
  alignItems: 'center',
  gap: '0.5rem',
  padding: '0.6rem 0.85rem',
  fontSize: '0.85rem',
  fontWeight: 700,
  color: 'var(--ifm-font-color-base)',
};

const captionSx = {
  position: 'absolute',
  textAlign: 'center',
  fontSize: '0.8rem',
  fontWeight: 700,
  color: 'var(--ifm-color-emphasis-700)',
  lineHeight: 1.3,
};

function Tile({icon, label}: {icon: React.ReactNode; label: string}) {
  return (
    <Box sx={{display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '0.3rem', textAlign: 'center'}}>
      <Box aria-hidden sx={iconSx}>{icon}</Box>
      <Typography component="span" sx={{fontSize: '0.72rem', fontWeight: 600, color: 'var(--ifm-color-emphasis-700)'}}>
        {label}
      </Typography>
    </Box>
  );
}

export function AgentInteractionsDiagram() {
  return (
    <Box component="figure" aria-label="An AI agent at the center of its interactions" sx={{margin: '2rem 0 2.5rem', border: 0, padding: 0, overflow: 'visible'}}>
      <Box sx={{overflowX: 'auto', overflowY: 'hidden', WebkitOverflowScrolling: 'touch', padding: '0.25rem'}}>
        <Box sx={{position: 'relative', width: W, height: H, minWidth: W, margin: '0 auto'}}>

          {/* ── Connector layer ── */}
          <Box component="svg" viewBox={`0 0 ${W} ${H}`} width={W} height={H} aria-hidden sx={{position: 'absolute', inset: 0, pointerEvents: 'none'}}>
            <defs>
              <marker id="aid-arrow" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
                <path d="M0,0 L10,5 L0,10 z" fill="var(--ifm-color-emphasis-500)" />
              </marker>
            </defs>
            {/* Invoking Parties → Agent */}
            <path d="M170,290 H358" style={lineSx} markerEnd="url(#aid-arrow)" />
            {/* Agent → AI Providers (up) */}
            <path d="M460,238 V134" style={lineSx} markerEnd="url(#aid-arrow)" />
            {/* Agent → Sub Agent (down) */}
            <path d="M460,342 V436" style={lineSx} markerEnd="url(#aid-arrow)" />
            {/* Agent → Internal Services (elbow, up-right) */}
            <path d="M562,272 H626 V132 H688" style={lineSx} markerEnd="url(#aid-arrow)" />
            {/* Agent → External Systems (elbow, down-right) */}
            <path d="M562,308 H626 V416 H688" style={lineSx} markerEnd="url(#aid-arrow)" />
          </Box>

          {/* ── Invoking Parties (left) ── */}
          <Typography component="span" sx={{...captionSx, left: 50, top: 152, width: 120}}>
            Invoking<br />Parties
          </Typography>
          <Box sx={{position: 'absolute', left: 50, top: 190, width: 120, height: 200, borderRadius: '999px', border: '1px solid var(--ifm-color-emphasis-200)', background: 'var(--ifm-background-surface-color)', boxShadow: '0 6px 18px color-mix(in srgb, var(--ifm-color-emphasis-900) 6%, transparent)', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: '1rem'}}>
            <Box aria-hidden sx={iconSx}><User size={30} /></Box>
            <Box aria-hidden sx={iconSx}><MonitorSmartphone size={30} /></Box>
            <Box aria-hidden sx={iconSx}><Bot size={30} /></Box>
          </Box>

          {/* ── AI Providers and LLMs (top) ── */}
          <Typography component="span" sx={{...captionSx, left: 300, top: 16, width: 320}}>
            AI Providers and LLMs
          </Typography>
          <Box sx={{...groupCardSx, left: 300, top: 46, width: 320, height: 84}}>
            <Box sx={{display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '1.4rem', height: '100%', ...iconSx}}>
              <Sparkles size={26} /><Brain size={26} /><Cpu size={26} /><Bot size={26} /><Cloud size={26} />
            </Box>
          </Box>

          {/* ── Agent (center) ── */}
          <Box sx={{position: 'absolute', left: 360, top: 240, width: 200, height: 100, borderRadius: '14px', color: '#fff', background: 'linear-gradient(150deg, color-mix(in srgb, var(--ifm-color-primary) 92%, #fff), color-mix(in srgb, var(--ifm-color-primary) 62%, #b3540a))', boxShadow: '0 14px 30px color-mix(in srgb, var(--ifm-color-primary) 34%, transparent)', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: '0.35rem'}}>
            <Box sx={{position: 'absolute', top: 10, right: 10, display: 'inline-flex', alignItems: 'center', gap: '0.2rem', padding: '0.15rem 0.45rem', borderRadius: '999px', background: 'rgba(255,255,255,0.22)', fontSize: '0.68rem', fontWeight: 700}}>
              <Sparkles size={12} /> AI
            </Box>
            <Box aria-hidden sx={{'& svg': {fill: 'none', stroke: 'currentColor', strokeLinecap: 'round', strokeLinejoin: 'round', strokeWidth: 1.7}}}><Bot size={34} /></Box>
            <Typography component="span" sx={{fontSize: '1.05rem', fontWeight: 800}}>Agent</Typography>
          </Box>

          {/* ── Sub Agent (bottom) ── */}
          <Box sx={{position: 'absolute', left: 390, top: 438, width: 140, height: 80, borderRadius: '12px', border: '1px solid var(--ifm-color-emphasis-200)', background: 'var(--ifm-background-surface-color)', boxShadow: '0 6px 18px color-mix(in srgb, var(--ifm-color-emphasis-900) 6%, transparent)', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: '0.3rem', ...iconSx}}>
            <Bot size={26} />
            <Typography component="span" sx={{fontSize: '0.82rem', fontWeight: 700, color: 'var(--ifm-color-emphasis-800)'}}>Sub Agent</Typography>
          </Box>

          {/* ── Internal Services (top-right) ── */}
          <Box sx={{...groupCardSx, left: 690, top: 46, width: 200, height: 190}}>
            <Box sx={groupHeaderSx}><Settings size={16} /> Internal Services</Box>
            <Box sx={{display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.9rem 0.5rem', padding: '0.4rem 0.9rem'}}>
              <Tile icon={<MCP size={26} />} label="MCPs" />
              <Tile icon={<Code2 size={26} />} label="APIs" />
              <Tile icon={<Database size={26} />} label="KBs" />
              <Tile icon={<AppWindow size={26} />} label="Apps" />
            </Box>
            <Box aria-hidden sx={{...iconSx, position: 'absolute', right: 14, bottom: 8}}><MoreHorizontal size={20} /></Box>
          </Box>

          {/* ── External Systems (bottom-right) ── */}
          <Box sx={{...groupCardSx, left: 690, top: 320, width: 200, height: 200}}>
            <Box sx={groupHeaderSx}><Settings size={16} /> External Systems</Box>
            <Box sx={{display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: '0.85rem', placeItems: 'center', padding: '0.5rem 0.9rem', ...iconSx}}>
              <Mail size={24} /><Calendar size={24} /><FileSpreadsheet size={24} />
              <MessageSquare size={24} /><Video size={24} /><Share2 size={24} />
              <GitHub size={24} /><Cloud size={24} /><MoreHorizontal size={24} />
            </Box>
          </Box>

        </Box>
      </Box>
    </Box>
  );
}
