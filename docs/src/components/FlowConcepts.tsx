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
import React from 'react';

const roadmapSx = {
  '--uc-node-w': '9rem',
  '--uc-node-h': '5rem',
  '--uc-icon-size': '4rem',
  display: 'flex',
  flexWrap: 'wrap',
  justifyContent: 'center',
  alignItems: 'flex-start',
  gap: '1.25rem 1.4rem',
  margin: '1.5rem 0 2rem',
  padding: '0.5rem 0.2rem',
  '@media (max-width: 640px)': {
    '--uc-node-w': '7.5rem',
    '--uc-node-h': '4.4rem',
    '--uc-icon-size': '3.4rem',
    gap: '1rem 0.8rem',
  },
} as const;

const roadmapNodeSx = {
  width: 'var(--uc-node-w)',
  minHeight: 'var(--uc-node-h)',
  border: 0,
  background: 'transparent',
  cursor: 'pointer',
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'center',
  justifyContent: 'flex-start',
  gap: '0.55rem',
  textAlign: 'center',
  textDecoration: 'none',
  color: 'var(--ifm-font-color-base)',
  fontWeight: 700,
  fontSize: '0.82rem',
  lineHeight: 1.2,
  transition: 'transform 160ms ease',
  '&:hover': {transform: 'translateY(-2px)', textDecoration: 'none'},
  '&:focus-visible': {
    outline: '2px solid color-mix(in srgb, var(--ifm-color-primary) 58%, white)',
    outlineOffset: '4px',
    borderRadius: '6px',
  },
} as const;

const roadmapIconSx = {
  width: 'var(--uc-icon-size)',
  minWidth: 'var(--uc-icon-size)',
  height: 'var(--uc-icon-size)',
  borderRadius: '999px',
  border: '1px solid color-mix(in srgb, var(--ifm-color-primary) 38%, var(--ifm-color-emphasis-300))',
  background: `radial-gradient(80px 80px at 28% 18%, color-mix(in srgb, var(--ifm-color-primary) 24%, transparent), transparent),
    linear-gradient(160deg, color-mix(in srgb, var(--ifm-color-primary) 72%, #091629), color-mix(in srgb, var(--ifm-color-primary) 44%, #030712))`,
  display: 'inline-flex',
  alignItems: 'center',
  justifyContent: 'center',
  boxShadow: 'inset 0 0 0 1px color-mix(in srgb, #fff 24%, transparent), 0 8px 18px color-mix(in srgb, var(--ifm-color-primary) 24%, transparent)',
  '& svg': {
    width: '1.75rem',
    height: '1.75rem',
    stroke: '#fff',
    fill: 'none',
    strokeWidth: '1.8',
    strokeLinecap: 'round' as const,
    strokeLinejoin: 'round' as const,
  },
} as const;

interface ConceptNode {
  href: string;
  label: string;
  icon: React.ReactNode;
}

const nodeTypeNodes: ConceptNode[] = [
  {
    href: '#start',
    label: 'START',
    icon: (
      <svg viewBox="0 0 24 24">
        <polygon points="5,3 19,12 5,21" />
      </svg>
    ),
  },
  {
    href: '#prompt',
    label: 'PROMPT',
    icon: (
      <svg viewBox="0 0 24 24">
        <rect x="2" y="4" width="20" height="14" rx="2" />
        <path d="M8 20h8" />
        <path d="M12 18v2" />
      </svg>
    ),
  },
  {
    href: '#task-execution',
    label: 'TASK EXECUTION',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <rect x="3" y="3" width="18" height="18" rx="2" />
        <path d="M10 8l6 4-6 4V8z" />
      </svg>
    ),
  },
  {
    href: '#call',
    label: 'CALL',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <polygon points="5,3 19,12 5,21" />
        <path d="M14 3v18" />
      </svg>
    ),
  },
  {
    href: '#end',
    label: 'END',
    icon: (
      <svg viewBox="0 0 24 24">
        <circle cx="12" cy="12" r="9" />
        <path d="m8 12 3 3 5-5" />
      </svg>
    ),
  },
];

const buildingBlockNodes: ConceptNode[] = [
  {
    href: '#widgets',
    label: 'Widgets',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M11 10.27 7 3.34" />
        <path d="m11 13.73-4 6.93" />
        <path d="M12 22v-2" />
        <path d="M12 2v2" />
        <path d="M14 12h8" />
        <path d="m17 20.66-1-1.73" />
        <path d="m17 3.34-1 1.73" />
        <path d="M2 12h2" />
        <path d="m20.66 17-1.73-1" />
        <path d="m20.66 7-1.73 1" />
        <path d="m3.34 17 1.73-1" />
        <path d="m3.34 7 1.73 1" />
        <circle cx="12" cy="12" r="2" />
        <circle cx="12" cy="12" r="8" />
      </svg>
    ),
  },
  {
    href: '#steps',
    label: 'Steps',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M21 8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16Z" />
        <path d="m3.3 7 8.7 5 8.7-5" />
        <path d="M12 22V12" />
      </svg>
    ),
  },
  {
    href: '#components',
    label: 'Components',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M2.97 12.92A2 2 0 0 0 2 14.63v3.24a2 2 0 0 0 .97 1.71l3 1.8a2 2 0 0 0 2.06 0L12 19v-5.5l-5-3-4.03 2.42Z" />
        <path d="m7 16.5-4.74-2.85" />
        <path d="m7 16.5 5-3" />
        <path d="M7 16.5v5.17" />
        <path d="M12 13.5V19l3.97 2.38a2 2 0 0 0 2.06 0l3-1.8a2 2 0 0 0 .97-1.71v-3.24a2 2 0 0 0-.97-1.71L17 10.5l-5 3Z" />
        <path d="m17 16.5-5-3" />
        <path d="m17 16.5 4.74-2.85" />
        <path d="M17 16.5v5.17" />
        <path d="M7.97 4.42A2 2 0 0 0 7 6.13v4.37l5 3 5-3V6.13a2 2 0 0 0-.97-1.71l-3-1.8a2 2 0 0 0-2.06 0l-3 1.8Z" />
        <path d="M12 8 7.26 5.15" />
        <path d="m12 8 4.74-2.85" />
        <path d="M12 13.5V8" />
      </svg>
    ),
  },
  {
    href: '#executors',
    label: 'Executors',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M4 14a1 1 0 0 1-.78-1.63l9.9-10.2a.5.5 0 0 1 .86.46l-1.92 6.02A1 1 0 0 0 13 10h7a1 1 0 0 1 .78 1.63l-9.9 10.2a.5.5 0 0 1-.86-.46l1.92-6.02A1 1 0 0 0 11 14z" />
      </svg>
    ),
  },
];

function RoadmapGrid({nodes, label}: {nodes: ConceptNode[]; label: string}): React.ReactElement {
  return (
    <Box component="nav" sx={roadmapSx} aria-label={label}>
      {nodes.map((node) => (
        <Box key={node.href} component="a" href={node.href} sx={roadmapNodeSx}>
          <Box component="span" sx={roadmapIconSx} aria-hidden>
            {node.icon}
          </Box>
          <Typography component="span" sx={{display: 'block', maxWidth: '9.4rem', fontSize: 'inherit', fontWeight: 'inherit'}}>
            {node.label}
          </Typography>
        </Box>
      ))}
    </Box>
  );
}

export function FlowNodeTypesRoadmap(): React.ReactElement {
  return <RoadmapGrid nodes={nodeTypeNodes} label="Flow node types" />;
}

export function FlowBuildingBlocksRoadmap(): React.ReactElement {
  return <RoadmapGrid nodes={buildingBlockNodes} label="Flow building blocks" />;
}

export function BuildAFlowDiagram() {
  // Layout
  //   Login Screen:   x=16,  y=18, w=194, h=148 → right x=210, bottom y=166
  //     title:        centre y=44
  //     divider:      y=68
  //     Option 1 row: x=28, y=72,  w=170, h=38 → centre_y=91
  //     separator:    y=112
  //     Option 2 row: x=28, y=116, w=170, h=38 → centre_y=135
  //                   12 px bottom padding to node edge (y=154 → y=166)
  //   SMS OTP Screen: x=278, y=72,  w=160, h=38 → centre_y=91, right x=438
  //   Login Success:  x=506, y=72,  w=152, h=38, rx=19
  //                   → centre_y=91, centre_x=582, bottom y=110
  //
  //   Gaps:  Login→SMS 68 px,  SMS→LoginSuccess 68 px
  //
  //   Arrows (all orthogonal, no diagonals)
  //     Option 1 → SMS OTP:        horizontal  y=91
  //     SMS OTP  → Login Success:  horizontal  y=91
  //     Google   → Login Success:  right → up (enters bottom centre)
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

        {/* Option 1 → SMS OTP Screen (straight horizontal at y=91) */}
        <line x1="210" y1="91" x2="275" y2="91" className="fnd-edge" markerEnd="url(#baf-arr)" />

        {/* SMS OTP Screen → Login Success (straight horizontal at y=91) */}
        <line x1="438" y1="91" x2="503" y2="91" className="fnd-edge" markerEnd="url(#baf-arr)" />

        {/* Google → Login Success: right → up into bottom centre */}
        {/* horizontal at y=135 clears below all nodes; vertical enters Login Success bottom (y=110) */}
        <path
          d="M 210,135 H 582 V 113"
          className="fnd-edge"
          markerEnd="url(#baf-arr)"
        />
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

        {/* ── Nodes ──────────────────────────────────────────────────── */}

        {/* START */}
        <rect x="10" y="82" width="80" height="36" rx="18" className="fnd-node fnd-node--start" />
        <text x="50" y="100" textAnchor="middle" dominantBaseline="central" className="fnd-label fnd-label--light">
          START
        </text>

        {/* PROMPT A */}
        <rect x="140" y="82" width="100" height="36" rx="6" className="fnd-node fnd-node--prompt" />
        <text x="190" y="100" textAnchor="middle" dominantBaseline="central" className="fnd-label">
          PROMPT
        </text>

        {/* TASK EXECUTION */}
        <rect x="295" y="74" width="150" height="52" rx="8" className="fnd-node fnd-node--task" />
        <text x="370" y="93" textAnchor="middle" dominantBaseline="central" className="fnd-label fnd-label--task">
          TASK
        </text>
        <text x="370" y="109" textAnchor="middle" dominantBaseline="central" className="fnd-label fnd-label--task">
          EXECUTION
        </text>

        {/* END */}
        <rect x="570" y="12" width="75" height="36" rx="18" className="fnd-node fnd-node--end" />
        <text x="607" y="30" textAnchor="middle" dominantBaseline="central" className="fnd-label fnd-label--light">
          END
        </text>

        {/* PROMPT B — failure */}
        <rect x="570" y="82" width="100" height="36" rx="6" className="fnd-node fnd-node--prompt" />
        <text x="620" y="100" textAnchor="middle" dominantBaseline="central" className="fnd-label">
          PROMPT
        </text>

        {/* PROMPT C — incomplete */}
        <rect x="570" y="162" width="100" height="36" rx="6" className="fnd-node fnd-node--prompt" />
        <text x="620" y="180" textAnchor="middle" dominantBaseline="central" className="fnd-label">
          PROMPT
        </text>

        {/* ── Arrows ─────────────────────────────────────────────────── */}

        {/* START → PROMPT A */}
        <line x1="90" y1="100" x2="136" y2="100" className="fnd-edge" markerEnd="url(#fnd-arr)" />

        {/* PROMPT A → TASK EXECUTION */}
        <line x1="240" y1="100" x2="291" y2="100" className="fnd-edge" markerEnd="url(#fnd-arr)" />

        {/* TASK EXECUTION → END (success, curves up) */}
        <path d="M 445,90 L 490,90 L 490,30 L 566,30" className="fnd-edge fnd-edge--success" markerEnd="url(#fnd-arr)" />

        {/* TASK EXECUTION → PROMPT B (failure, straight) */}
        <line x1="445" y1="100" x2="566" y2="100" className="fnd-edge fnd-edge--failure" markerEnd="url(#fnd-arr)" />

        {/* TASK EXECUTION → PROMPT C (incomplete, curves down) */}
        <path d="M 445,110 L 490,110 L 490,180 L 566,180" className="fnd-edge fnd-edge--incomplete" markerEnd="url(#fnd-arr)" />

        {/* ── Edge Labels ────────────────────────────────────────────── */}
        <text x="498" y="60" dominantBaseline="central" className="fnd-edge-label fnd-edge-label--success">
          success
        </text>
        <text x="505" y="92" dominantBaseline="central" className="fnd-edge-label fnd-edge-label--failure">
          failure
        </text>
        <text x="498" y="148" dominantBaseline="central" className="fnd-edge-label fnd-edge-label--incomplete">
          incomplete
        </text>
      </svg>
    </figure>
  );
}
