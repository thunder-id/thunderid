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

import {Box} from '@wso2/oxygen-ui';
import {Boxes, CheckCircle, Cpu, LayoutGrid, Layers, Monitor, PlayCircle, Zap} from '@wso2/oxygen-ui-icons-react';
import React from 'react';

interface ConceptNode {
  href: string;
  label: string;
  icon: React.ReactNode;
}

const nodeTypeNodes: ConceptNode[] = [
  {href: '#start',          label: 'START',          icon: <PlayCircle  size={28} aria-hidden />},
  {href: '#prompt',         label: 'PROMPT',         icon: <Monitor     size={28} aria-hidden />},
  {href: '#task-execution', label: 'TASK EXECUTION', icon: <Cpu         size={28} aria-hidden />},
  {href: '#end',            label: 'END',            icon: <CheckCircle size={28} aria-hidden />},
];

const buildingBlockNodes: ConceptNode[] = [
  {href: '#widgets',    label: 'Widgets',    icon: <LayoutGrid size={28} aria-hidden />},
  {href: '#steps',      label: 'Steps',      icon: <Layers     size={28} aria-hidden />},
  {href: '#components', label: 'Components', icon: <Boxes      size={28} aria-hidden />},
  {href: '#executors',  label: 'Executors',  icon: <Zap        size={28} aria-hidden />},
];

const roadmapSx = {
  '--uc-node-w': '9rem',
  '--uc-node-h': '5rem',
  '--uc-icon-size': '4rem',
  alignItems: 'flex-start',
  display: 'flex',
  flexWrap: 'wrap',
  gap: '1.25rem 1.4rem',
  justifyContent: 'center',
  margin: '1.5rem 0 2rem',
  padding: '0.5rem 0.2rem',
};

const nodeSx = {
  alignItems: 'center',
  background: 'transparent',
  border: 0,
  color: 'var(--ifm-font-color-base)',
  cursor: 'pointer',
  display: 'flex',
  flexDirection: 'column',
  fontSize: '0.82rem',
  fontWeight: 700,
  gap: '0.55rem',
  justifyContent: 'flex-start',
  lineHeight: 1.2,
  minHeight: 'var(--uc-node-h)',
  textAlign: 'center',
  textDecoration: 'none',
  transition: 'transform 160ms ease',
  width: 'var(--uc-node-w)',
  '&:hover': {
    textDecoration: 'none',
    transform: 'translateY(-2px)',
  },
  '&:hover .flow-node-icon': {
    borderColor: 'color-mix(in srgb, #ffffff 56%, var(--ifm-color-primary))',
    boxShadow: 'inset 0 0 0 1px color-mix(in srgb, #fff 32%, transparent), 0 12px 24px color-mix(in srgb, var(--ifm-color-primary) 34%, transparent)',
  },
  '&:focus-visible': {
    borderRadius: '6px',
    outline: '2px solid color-mix(in srgb, var(--ifm-color-primary) 58%, white)',
    outlineOffset: '4px',
  },
};

const iconSx = {
  alignItems: 'center',
  background: `
    radial-gradient(80px 80px at 28% 18%, color-mix(in srgb, var(--ifm-color-primary) 24%, transparent), transparent),
    linear-gradient(160deg, color-mix(in srgb, var(--ifm-color-primary) 72%, #091629), color-mix(in srgb, var(--ifm-color-primary) 44%, #030712))
  `,
  border: '1px solid color-mix(in srgb, var(--ifm-color-primary) 38%, var(--ifm-color-emphasis-300))',
  borderRadius: '999px',
  boxShadow: 'inset 0 0 0 1px color-mix(in srgb, #fff 24%, transparent), 0 8px 18px color-mix(in srgb, var(--ifm-color-primary) 24%, transparent)',
  color: '#fff',
  display: 'inline-flex',
  height: 'var(--uc-icon-size)',
  justifyContent: 'center',
  minWidth: 'var(--uc-icon-size)',
  width: 'var(--uc-icon-size)',
  '& svg': {
    fill: 'none',
    height: '1.75rem !important',
    stroke: 'currentColor',
    strokeLinecap: 'round',
    strokeLinejoin: 'round',
    strokeWidth: 1.8,
    width: '1.75rem !important',
  },
};

const labelSx = {display: 'block', maxWidth: '9.4rem'};

function Roadmap({nodes, ariaLabel}: {nodes: ConceptNode[]; ariaLabel: string}) {
  return (
    <Box component="nav" aria-label={ariaLabel} sx={roadmapSx}>
      {nodes.map(({href, icon, label}) => (
        <Box component="a" href={href} key={href} sx={nodeSx}>
          <Box className="flow-node-icon" component="span" aria-hidden sx={iconSx}>{icon}</Box>
          <Box component="span" sx={labelSx}>{label}</Box>
        </Box>
      ))}
    </Box>
  );
}

export function FlowNodeTypesRoadmap() {
  return <Roadmap nodes={nodeTypeNodes} ariaLabel="Flow node types" />;
}

export function FlowBuildingBlocksRoadmap() {
  return <Roadmap nodes={buildingBlockNodes} ariaLabel="Flow building blocks" />;
}

export function BuildAFlowDiagram() {
  return (
    <figure
      className="flow-node-diagram"
      role="img"
      aria-label="Multi-factor sign-in flow: Login Screen with two options — Username and Password leads to SMS OTP Screen then Login Success; Continue with Google leads directly to Login Success."
    >
      <svg
        viewBox="0 0 678 210"
        style={{ width: '100%', overflow: 'visible', display: 'block', fontFamily: 'inherit' }}
        aria-hidden="true"
      >
        <defs>
          <marker id="baf-arr" markerWidth="8" markerHeight="6" refX="7" refY="3" orient="auto">
            <polygon points="0 0, 8 3, 0 6" style={{ fill: 'context-stroke' }} />
          </marker>
        </defs>

        {/* ── Login Screen ────────────────────────────────────────────────── */}
        <rect x="16" y="18" width="194" height="148" rx="8" className="fnd-node fnd-node--prompt" />
        <text x="113" y="44" textAnchor="middle" dominantBaseline="central" className="fnd-label">
          Login Screen
        </text>
        <line x1="28" y1="68" x2="198" y2="68" className="fnd-divider" />

        {/* Option 1 — Username + Password */}
        <rect x="28" y="72" width="170" height="38" rx="5" className="fnd-option" />
        <g transform="translate(50,91)">
          <rect x="-5" y="-1" width="10" height="8" rx="1.5" className="fnd-option-icon" />
          <path d="M-3.5,-1 V-5 Q-3.5,-9 0,-9 Q3.5,-9 3.5,-5 V-1" className="fnd-option-icon-stroke" />
        </g>
        <text x="68" y="91" dominantBaseline="central" className="fnd-option-label">
          Username + Password
        </text>

        {/* Separator */}
        <line x1="28" y1="112" x2="198" y2="112" className="fnd-divider" />

        {/* Option 2 — Continue with Google */}
        <rect x="28" y="116" width="170" height="38" rx="5" className="fnd-option" />
        <circle cx="50" cy="135" r="8" className="fnd-google-circle" />
        <text x="50" y="135" textAnchor="middle" dominantBaseline="central" className="fnd-google-g">
          G
        </text>
        <text x="68" y="135" dominantBaseline="central" className="fnd-option-label">
          Continue with Google
        </text>

        {/* ── SMS OTP Screen ───────────────────────────────────────────────── */}
        <rect x="278" y="72" width="160" height="38" rx="6" className="fnd-node fnd-node--prompt" />
        <text x="358" y="91" textAnchor="middle" dominantBaseline="central" className="fnd-label">
          SMS OTP Screen
        </text>

        {/* ── Login Success ─────────────────────────────────────────────────── */}
        <rect x="506" y="72" width="152" height="38" rx="19" className="fnd-node fnd-node--end" />
        <text x="582" y="91" textAnchor="middle" dominantBaseline="central" className="fnd-label fnd-label--light">
          Login Success
        </text>

        {/* ── Arrows ────────────────────────────────────────────────────────── */}
        <line x1="210" y1="91" x2="275" y2="91" className="fnd-edge" markerEnd="url(#baf-arr)" />
        <line x1="438" y1="91" x2="503" y2="91" className="fnd-edge" markerEnd="url(#baf-arr)" />
        <path d="M 210,135 H 582 V 113" className="fnd-edge" markerEnd="url(#baf-arr)" />
      </svg>
    </figure>
  );
}

export function FlowNodeDiagram() {
  return (
    <figure
      className="flow-node-diagram"
      role="img"
      aria-label="Node connection diagram: START connects to PROMPT, which connects to TASK EXECUTION. TASK EXECUTION has three paths: success leading to END, failure leading to a PROMPT node, and incomplete leading to a PROMPT node."
    >
      <svg
        viewBox="0 0 700 210"
        style={{ width: '100%', overflow: 'visible', display: 'block', fontFamily: 'inherit' }}
        aria-hidden="true"
      >
        <defs>
          <marker id="fnd-arr" markerWidth="8" markerHeight="6" refX="7" refY="3" orient="auto">
            <polygon points="0 0, 8 3, 0 6" style={{ fill: 'context-stroke' }} />
          </marker>
        </defs>

        <rect x="10" y="82" width="80" height="36" rx="18" className="fnd-node fnd-node--start" />
        <text x="50" y="100" textAnchor="middle" dominantBaseline="central" className="fnd-label fnd-label--light">START</text>

        <rect x="140" y="82" width="100" height="36" rx="6" className="fnd-node fnd-node--prompt" />
        <text x="190" y="100" textAnchor="middle" dominantBaseline="central" className="fnd-label">PROMPT</text>

        <rect x="295" y="74" width="150" height="52" rx="8" className="fnd-node fnd-node--task" />
        <text x="370" y="93" textAnchor="middle" dominantBaseline="central" className="fnd-label fnd-label--task">TASK</text>
        <text x="370" y="109" textAnchor="middle" dominantBaseline="central" className="fnd-label fnd-label--task">EXECUTION</text>

        <rect x="570" y="12" width="75" height="36" rx="18" className="fnd-node fnd-node--end" />
        <text x="607" y="30" textAnchor="middle" dominantBaseline="central" className="fnd-label fnd-label--light">END</text>

        <rect x="570" y="82" width="100" height="36" rx="6" className="fnd-node fnd-node--prompt" />
        <text x="620" y="100" textAnchor="middle" dominantBaseline="central" className="fnd-label">PROMPT</text>

        <rect x="570" y="162" width="100" height="36" rx="6" className="fnd-node fnd-node--prompt" />
        <text x="620" y="180" textAnchor="middle" dominantBaseline="central" className="fnd-label">PROMPT</text>

        <line x1="90" y1="100" x2="136" y2="100" className="fnd-edge" markerEnd="url(#fnd-arr)" />
        <line x1="240" y1="100" x2="291" y2="100" className="fnd-edge" markerEnd="url(#fnd-arr)" />
        <path d="M 445,90 L 490,90 L 490,30 L 566,30" className="fnd-edge fnd-edge--success" markerEnd="url(#fnd-arr)" />
        <line x1="445" y1="100" x2="566" y2="100" className="fnd-edge fnd-edge--failure" markerEnd="url(#fnd-arr)" />
        <path d="M 445,110 L 490,110 L 490,180 L 566,180" className="fnd-edge fnd-edge--incomplete" markerEnd="url(#fnd-arr)" />

        <text x="498" y="60" dominantBaseline="central" className="fnd-edge-label fnd-edge-label--success">success</text>
        <text x="505" y="92" dominantBaseline="central" className="fnd-edge-label fnd-edge-label--failure">failure</text>
        <text x="498" y="148" dominantBaseline="central" className="fnd-edge-label fnd-edge-label--incomplete">incomplete</text>
      </svg>
    </figure>
  );
}
