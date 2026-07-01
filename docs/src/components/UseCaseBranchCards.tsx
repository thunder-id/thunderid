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

import {Bot, Building2, Fingerprint} from '@wso2/oxygen-ui-icons-react';
import React from 'react';

import UseCaseBranchCard from './UseCaseBranchCard';

interface BranchCard {
  href: string;
  animationClass: string;
  icon: React.ReactNode;
  accentColor: string;
  iconBackground: string;
  category: string;
  title: string;
  description: string;
  bullets: string[];
}

const cards: BranchCard[] = [
  {
    href: '/docs/next/use-cases/b2c/',
    animationClass: 'uc-card-1',
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
    animationClass: 'uc-card-2',
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
  },
  {
    href: '/docs/next/use-cases/ai-agents/overview',
    animationClass: 'uc-card-3',
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
