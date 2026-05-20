import React from 'react';

import FingerprintIcon from './icons/FingerprintIcon';
import KeypadIcon from './icons/KeypadIcon';
import MagicLinkIcon from './icons/MagicLinkIcon';
import UseCaseBranchCard from './UseCaseBranchCard';

type BranchCard = {
  href: string;
  animationClass: string;
  icon: React.ReactNode;
  accentColor: string;
  iconBackground: string;
  category: string;
  title: string;
  description: string;
  bullets: string[];
};

const cards: BranchCard[] = [
  {
    href: '/docs/next/use-cases/b2c/customer-identity',
    animationClass: 'uc-card-1',
    icon: <FingerprintIcon size={26} />,
    accentColor: '#3b82f6',
    iconBackground: 'rgba(59,130,246,0.10)',
    category: 'Consumer Apps',
    title: 'B2C - Customer Identity',
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
    animationClass: 'uc-card-2',
    icon: <KeypadIcon size={26} />,
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
  },
  {
    href: '/docs/next/use-cases/ai-agents/overview',
    animationClass: 'uc-card-3',
    icon: <MagicLinkIcon size={26} />,
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
  return (
    <div className="uc-branch-grid">
      {cards.map((card) => (
        <UseCaseBranchCard
          key={card.href}
          href={card.href}
          animationClass={card.animationClass}
          icon={card.icon}
          accentColor={card.accentColor}
          iconBackground={card.iconBackground}
          category={card.category}
          title={card.title}
          description={card.description}
          bullets={card.bullets}
        />
      ))}
    </div>
  );
}
