// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useActiveVersion} from '@docusaurus/plugin-content-docs/client';
import {Box} from '@wso2/oxygen-ui';
import {Bot, Building2, Fingerprint} from '@wso2/oxygen-ui-icons-react';
import React from 'react';

import UseCaseBranchCard from './UseCaseBranchCard';
import {useDocsUrl} from '@site/src/hooks/useDocsUrl';

interface BranchCard {
  href: string;
  animationDelay: number;
  icon: React.ReactNode;
  accentColor: string;
  iconBackground: string;
  category: string;
  title: string;
  description: string;
  bullets: string[];
  hiddenInVersions?: string[];
}

const cards: BranchCard[] = [
  {
    href: '/docs/next/use-cases/b2c/',
    animationDelay: 300,
    icon: <Fingerprint size={26} />,
    accentColor: '#3b82f6',
    iconBackground: 'rgba(59,130,246,0.10)',
    category: 'Consumer Apps',
    title: 'B2C - Overview',
    description:
      'Frictionless sign-up and sign-in for consumer apps. Passkeys, social login, and step-up authentication.',
    bullets: [
      'Your users are individual consumers on web or mobile',
      'Fast onboarding and low-friction sign-in are priorities',
      'You need social login, passkeys, or step-up auth',
    ],
  },
  {
    href: '/docs/next/use-cases/b2b/multi-tenant-saas',
    animationDelay: 420,
    icon: <Building2 size={26} />,
    accentColor: '#10b981',
    iconBackground: 'rgba(16,185,129,0.10)',
    category: 'SaaS Apps',
    title: 'B2B - Multi-Tenant SaaS',
    description:
      'Organizations, invitations, enterprise SSO, delegated admin, and workspace-level policies.',
    bullets: [
      'Each customer is a business with its own workspace',
      'You need org-scoped roles, policies, and branding',
      'Enterprise SSO or federated identity is required',
    ],
    hiddenInVersions: ['v1.0.x'],
  },
  {
    href: '/docs/next/use-cases/ai-agents/overview',
    animationDelay: 540,
    icon: <Bot size={26} />,
    accentColor: '#8b5cf6',
    iconBackground: 'rgba(139,92,246,0.10)',
    category: 'AI & Automation',
    title: 'Identity for AI Agents',
    description:
      'Authenticate agents, authorize actions, secure MCP servers, and audit every interaction across single and multi-agent workflows.',
    bullets: [
      'Users interact with your AI agent securely',
      'Agents call APIs and MCP servers on their own or on behalf of users',
      'Multi-agent workflows need trust propagation and audit trails',
    ],
  },
];

export default function UseCaseBranchCards() {
  const docsUrl = useDocsUrl();
  const activeVersion = useActiveVersion(undefined);
  const visibleCards = cards.filter((card) => !card.hiddenInVersions?.includes(activeVersion?.name ?? ''));
  return (
    <Box
      sx={{
        display: 'flex',
        flexWrap: 'wrap',
        gap: '1.25rem',
        justifyContent: 'flex-start',
        maxWidth: '900px',
        width: '100%',
      }}
    >
      {visibleCards.map((card) => (
        <UseCaseBranchCard
          key={card.href}
          href={docsUrl(card.href)}
          animationDelay={card.animationDelay}
          icon={card.icon}
          accentColor={card.accentColor}
          iconBackground={card.iconBackground}
          category={card.category}
          title={card.title}
          description={card.description}
          bullets={card.bullets}
        />
      ))}
    </Box>
  );
}
