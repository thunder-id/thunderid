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
import {ArrowLeftRight, Bell, KeyRound, Network, ShieldCheck, UserCheck, Workflow} from '@wso2/oxygen-ui-icons-react';
import React from 'react';

interface RoadmapNode {
  href: string;
  icon: React.ReactNode;
  label: string;
}

const roadmapNodes: RoadmapNode[] = [
  {href: '#protect-your-agent',    icon: <ShieldCheck size={28} />, label: 'Protect Your Agent'},
  {href: '#connect-to-services',   icon: <Network size={28} />,     label: 'Connect to Services'},
  {href: '#multi-agent-workflows', icon: <Workflow size={28} />,    label: 'Multi-Agent Workflows'},
];

const solutionPatternNodes: RoadmapNode[] = [
  {href: '#client-credentials-grant',       icon: <KeyRound size={28} />,        label: 'Client Credentials'},
  {href: '#authorization-code-with-obo',    icon: <UserCheck size={28} />,       label: 'Interactive Delegation'},
  {href: '#backchannel-authorization-ciba', icon: <Bell size={28} />,            label: 'Background Delegation'},
  {href: '#token-exchange',                 icon: <ArrowLeftRight size={28} />,  label: 'Token Exchange'},
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
  '&:hover .ai-roadmap-icon': {
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

function Roadmap({nodes, ariaLabel}: {nodes: RoadmapNode[]; ariaLabel: string}) {
  return (
    <Box component="nav" aria-label={ariaLabel} sx={roadmapSx}>
      {nodes.map(({href, icon, label}) => (
        <Box component="a" href={href} key={href} sx={nodeSx}>
          <Box className="ai-roadmap-icon" component="span" aria-hidden sx={iconSx}>{icon}</Box>
          <Box component="span" sx={labelSx}>{label}</Box>
        </Box>
      ))}
    </Box>
  );
}

export function AIAgentIdentityRoadmap() {
  return <Roadmap nodes={roadmapNodes} ariaLabel="AI agent identity use case roadmap" />;
}

export function AIAgentSolutionPatternsRoadmap() {
  return <Roadmap nodes={solutionPatternNodes} ariaLabel="AI agent solution pattern roadmap" />;
}
