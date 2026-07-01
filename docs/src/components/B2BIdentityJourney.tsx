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
import {BarChart3, Bot, Building2, Globe, KeyRound, LayoutGrid, LogIn, Palette, ShieldCheck, UserCog, Users, UsersRound, Wrench} from '@wso2/oxygen-ui-icons-react';
import React from 'react';

interface ModelWorkspace {
  name: string;
  items: string[];
}

interface JourneyStep {
  title: string;
  scenario: string;
  thunder: string;
  flow: string[];
}

const workspaces: ModelWorkspace[] = [
  {
    name: 'Customer Workspace A',
    items: ['Users', 'Roles', 'Policies', 'Federation', 'Recovery'],
  },
  {
    name: 'Customer Workspace B',
    items: ['Users', 'Roles', 'Policies', 'Branding'],
  },
  {
    name: 'Customer Workspace C',
    items: ['Admins', 'Invited users', 'Governance'],
  },
];

const journeySteps: JourneyStep[] = [
  {
    title: 'Create Customer Workspace',
    scenario: 'Set up the organization boundary for a customer.',
    thunder: 'Creates the workspace, assigns the initial admin, and applies policy, billing, and branding defaults.',
    flow: ['New customer signs up', 'Create workspace', 'Assign initial admin', 'Apply policy and billing'],
  },
  {
    title: 'Invite Collaborators',
    scenario: 'Bring customer admins and users into the workspace.',
    thunder: 'Handles invitation lifecycle, role assignment, and workspace-scoped access.',
    flow: ['Workspace admin', 'Sends invites', 'Members join', 'Scoped roles applied'],
  },
  {
    title: 'Evolve Identity',
    scenario: 'Move from direct sign-in to social and enterprise federation.',
    thunder: 'Adds social login, OIDC or SAML federation, JIT provisioning, and account linking.',
    flow: ['Direct sign-up', 'Add Google login', 'Enable enterprise SSO', 'JIT or account linking'],
  },
  {
    title: 'Recover Access',
    scenario: 'Let users regain access based on workspace policy.',
    thunder: 'Supports email magic links, email OTP, SMS OTP, and policy-driven recovery routes.',
    flow: ['Access issue detected', 'Recovery method selected', 'Policy checks', 'Access restored'],
  },
  {
    title: 'Govern Operations',
    scenario: 'Delegate administration, audit activity, and manage subscription changes.',
    thunder: 'Supports delegated admin, access governance, audit visibility, and operational controls.',
    flow: ['Delegate admin', 'Track activity', 'Apply governance rules', 'Adjust subscription access'],
  },
  {
    title: 'Prepare for Agent Access',
    scenario: 'Extend the same organization boundary to AI agents and workloads.',
    thunder: 'Uses the same workspace and policy model for future agent identity and control.',
    flow: ['Organization boundary', 'Agent identity added', 'Policy-based access'],
  },
];

const roadmapSx = {
  '--uc-node-w': '8.7rem',
  '--uc-node-h': '5rem',
  '--uc-icon-size': '4.35rem',
  '--uc-gap-x': '0.9rem',
  '--uc-gap-y': '1rem',
  alignItems: 'center',
  display: 'grid',
  gap: 'var(--uc-gap-y) var(--uc-gap-x)',
  gridTemplateColumns: 'repeat(4, minmax(0, 1fr))',
  gridTemplateRows: 'auto auto repeat(5, minmax(0, 1fr))',
  justifyItems: 'center',
  margin: '1.1rem 0 1.45rem',
  minHeight: '50rem',
  padding: '0.55rem 0.2rem',
  position: 'relative',
  '@media (max-width: 900px)': {
    '--uc-node-w': '7rem',
    '--uc-node-h': '4.5rem',
    '--uc-icon-size': '3.5rem',
    '--uc-gap-x': '0.6rem',
    '--uc-gap-y': '0.8rem',
    minHeight: 'auto',
  },
  '@media (max-width: 600px)': {
    '--uc-node-w': '5.5rem',
    '--uc-node-h': '4rem',
    '--uc-icon-size': '2.75rem',
    '--uc-gap-x': '0.4rem',
    '--uc-gap-y': '0.6rem',
    minWidth: '500px',
  },
};

const svgPathSx = {
  height: 'calc(100% - 1.3rem)',
  inset: '0.65rem 0.35rem',
  pointerEvents: 'none',
  position: 'absolute',
  width: 'calc(100% - 0.7rem)',
  zIndex: 0,
  '& path': {
    fill: 'none',
    opacity: 0.82,
    stroke: 'color-mix(in srgb, var(--ifm-color-primary) 44%, var(--ifm-color-emphasis-300))',
    strokeDasharray: '3 4',
    strokeLinecap: 'round',
    strokeWidth: 1.25,
  },
  // Paths are sized for desktop — hide on mobile
  '@media (max-width: 600px)': {
    display: 'none',
  },
};

const nodeSx = {
  alignItems: 'center',
  background: 'transparent',
  border: 'none',
  borderRadius: 0,
  boxShadow: 'none',
  color: 'var(--ifm-font-color-base)',
  display: 'flex',
  flexDirection: 'column',
  fontSize: '0.72rem',
  fontWeight: 700,
  gap: '0.5rem',
  justifyContent: 'flex-start',
  lineHeight: 1.15,
  marginTop: '0.5rem',
  minHeight: 'var(--uc-node-h)',
  padding: '0.2rem 0.2rem 0',
  position: 'relative',
  textAlign: 'center',
  textDecoration: 'none',
  transform: 'none',
  transition: 'transform 160ms ease',
  width: 'var(--uc-node-w)',
  zIndex: 1,
  '&::before, &::after': {
    content: '""',
    display: 'none !important',
    opacity: 0.9,
    pointerEvents: 'none',
    position: 'absolute',
  },
  '&:hover': {
    textDecoration: 'none',
    transform: 'translateY(-2px)',
  },
  '&:hover .uc-b2b-icon': {
    borderColor: 'color-mix(in srgb, #ffffff 56%, var(--ifm-color-primary))',
    boxShadow: 'inset 0 0 0 1px color-mix(in srgb, #fff 32%, transparent), 0 12px 24px color-mix(in srgb, var(--ifm-color-primary) 34%, transparent)',
  },
  '&:focus-visible': {
    outline: '2px solid color-mix(in srgb, var(--ifm-color-primary) 58%, white)',
    outlineOffset: '2px',
  },
  '@media (max-width: 900px)': {
    fontSize: '0.66rem',
  },
  '@media (max-width: 600px)': {
    fontSize: '0.6rem',
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

const labelSx = {
  color: 'var(--ifm-font-color-base)',
  display: 'block',
  fontSize: '0.82rem',
  fontWeight: 700,
  letterSpacing: '0.01em',
  lineHeight: 1.2,
  maxWidth: '9.4rem',
};

const categorySx = {
  alignItems: 'center',
  display: 'flex',
  justifyContent: 'center',
  marginTop: '2.5rem',
  position: 'relative',
  zIndex: 2,
  '&:hover .uc-b2b-cat-label': {
    background: 'linear-gradient(135deg, color-mix(in srgb, var(--ifm-color-primary) 12%, transparent), color-mix(in srgb, var(--ifm-color-primary) 8%, transparent))',
    borderColor: 'color-mix(in srgb, var(--ifm-color-primary) 45%, transparent)',
    color: 'color-mix(in srgb, var(--ifm-color-primary) 100%, var(--ifm-color-emphasis-600))',
    opacity: 1,
  },
};

const categoryLabelSx = {
  background: 'linear-gradient(135deg, color-mix(in srgb, var(--ifm-color-primary) 8%, transparent), color-mix(in srgb, var(--ifm-color-primary) 5%, transparent))',
  border: '1px solid color-mix(in srgb, var(--ifm-color-primary) 25%, transparent)',
  borderRadius: '0.4rem',
  color: 'color-mix(in srgb, var(--ifm-color-primary) 85%, var(--ifm-color-emphasis-600))',
  fontSize: '0.72rem',
  fontWeight: 800,
  letterSpacing: '0.09em',
  opacity: 0.92,
  padding: '0.48rem 0.72rem',
  textTransform: 'uppercase',
  transition: 'all 200ms ease',
  whiteSpace: 'nowrap',
};

function RoadmapNode({href, icon, label, sx: extraSx = {}}: {
  href: string;
  icon: React.ReactNode;
  label: string;
  sx?: object;
}) {
  return (
    <Box component="a" href={href} sx={{...nodeSx, ...extraSx}}>
      <Box className="uc-b2b-icon" component="span" sx={iconSx}>{icon}</Box>
      <Box component="span" sx={labelSx}>{label}</Box>
    </Box>
  );
}

function RoadmapCategory({label, sx: extraSx = {}}: {label: string; sx?: object}) {
  return (
    <Box sx={{...categorySx, ...extraSx}}>
      <Box className="uc-b2b-cat-label" component="span" sx={categoryLabelSx}>{label}</Box>
    </Box>
  );
}

export function B2BSaaSIdentityModelGraph() {
  return (
    <section className="uc-b2b-model" aria-label="B2B SaaS identity model">
      <div className="uc-b2b-model__root">
        <div className="uc-b2b-model__kicker">Platform Layer</div>
        <div className="uc-b2b-model__title">SaaS Platform</div>
        <div className="uc-b2b-model__stem"></div>
      </div>
      <div className="uc-b2b-model__branches">
        {workspaces.map((workspace) => (
          <article key={workspace.name} className="uc-b2b-model__workspace">
            <h3>{workspace.name}</h3>
            <ul aria-label={`${workspace.name} controls`}>
              {workspace.items.map((item) => (
                <li key={item}>{item}</li>
              ))}
            </ul>
          </article>
        ))}
      </div>
    </section>
  );
}

export function B2BSaaSIdentityJourneyTimeline() {
  return (
    <section className="uc-b2b-journey" aria-label="B2B SaaS identity journey timeline">
      {journeySteps.map((step, index) => (
        <article key={step.title} className="uc-b2b-journey__step">
          <div className="uc-b2b-journey__index" aria-hidden />
          <div className="uc-b2b-journey__content">
            <h3>
              {index + 1}. {step.title}
            </h3>
            <p>
              <strong>Scenario:</strong> {step.scenario}
            </p>
            <p>
              <strong>What ThunderID Handles:</strong> {step.thunder}
            </p>
            <div className="uc-b2b-journey__flow" role="list" aria-label={`${step.title} flow`}>
              {step.flow.map((flowNode, flowIndex) => (
                <React.Fragment key={flowNode}>
                  <span role="listitem" className="uc-b2b-journey__flow-node">
                    {flowNode}
                  </span>
                  {flowIndex < step.flow.length - 1 && <span className="uc-b2b-journey__flow-arrow">→</span>}
                </React.Fragment>
              ))}
            </div>
          </div>
        </article>
      ))}
    </section>
  );
}

export function B2BSaaSIdentityJourneyRoadmap() {
  return (
    <Box sx={{'@media (max-width: 600px)': {overflowX: 'auto', WebkitOverflowScrolling: 'touch'}}}>
    <Box role="navigation" aria-label="B2B use case roadmap" sx={roadmapSx}>
      <Box component="svg" aria-hidden sx={svgPathSx} viewBox="0 0 1000 700" preserveAspectRatio="none">
        <path d="M500 92 V112" />
        <path d="M500 112 L116 112" />
        <path d="M500 112 L372 112" />
        <path d="M500 112 L628 112" />
        <path d="M500 112 L884 112" />
        <path d="M116 112 V124" />
        <path d="M372 112 V124" />
        <path d="M628 112 V124" />
        <path d="M884 112 V124" />
      </Box>

      <RoadmapNode
        href="#organization-onboarding-sign-up"
        icon={<Building2 size={28} />}
        label="Onboard Organization"
        sx={{gridColumn: '2 / span 2', gridRow: 1, marginTop: 0, transform: 'translateY(-0.1rem)'}}
      />

      <RoadmapCategory label="Identity &amp; Access" sx={{gridColumn: 1, gridRow: 2}} />
      <RoadmapCategory label="Administration"       sx={{gridColumn: 2, gridRow: 2}} />
      <RoadmapCategory label="Configuration"        sx={{gridColumn: 3, gridRow: 2}} />
      <RoadmapCategory label="Operations"           sx={{gridColumn: 4, gridRow: 2}} />

      <RoadmapNode href="#organizational-identity-management"                          icon={<Users size={28} />}      label="Manage Identities"                   sx={{gridColumn: 1, gridRow: 3}} />
      <RoadmapNode href="#identity-recovery"                                           icon={<KeyRound size={28} />}   label="Recover Identities"                  sx={{gridColumn: 1, gridRow: 4}} />
      <RoadmapNode href="#organization-workspace-authorization-and-subscription-controls" icon={<LayoutGrid size={28} />} label="Authorize Access"                sx={{gridColumn: 1, gridRow: 5}} />
      <RoadmapNode href="#ai-agents"                                                   icon={<Bot size={28} />}        label="Manage AI Agents"                    sx={{gridColumn: 1, gridRow: 6}} />

      <RoadmapNode href="#delegated-administration" icon={<UserCog size={28} />}    label="Delegate Administrators"          sx={{gridColumn: 2, gridRow: 3}} />
      <RoadmapNode href="#collaboration"            icon={<UsersRound size={28} />} label="Collaborate with Organizations"   sx={{gridColumn: 2, gridRow: 4}} />

      <RoadmapNode href="#organizational-sign-in"           icon={<LogIn size={28} />}       label="Configure Sign-In"                   sx={{gridColumn: 3, gridRow: 3}} />
      <RoadmapNode href="#branding-customization"           icon={<Palette size={28} />}     label="Customize Branding"                  sx={{gridColumn: 3, gridRow: 4}} />
      <RoadmapNode href="#organization-discovery-mechanisms" icon={<Globe size={28} />}      label="Configure Organization Discovery"    sx={{gridColumn: 3, gridRow: 5}} />

      <RoadmapNode href="#privacy-and-compliance"           icon={<ShieldCheck size={28} />} label="Ensure Compliance"    sx={{gridColumn: 4, gridRow: 3}} />
      <RoadmapNode href="#platform-monitoring-and-governance" icon={<BarChart3 size={28} />} label="Platform Audit"       sx={{gridColumn: 4, gridRow: 4}} />
      <RoadmapNode href="#troubleshooting"                  icon={<Wrench size={28} />}      label="Troubleshoot Issues"  sx={{gridColumn: 4, gridRow: 5}} />
    </Box>
    </Box>
  );
}
