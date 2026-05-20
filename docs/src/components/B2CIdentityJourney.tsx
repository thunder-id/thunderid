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

import React from 'react';

interface RoadmapNode {
  href: string;
  label: string;
  icon: React.ReactNode;
}

const roadmapNodes: RoadmapNode[] = [
  {
    href: '#add-login-to-your-app',
    label: 'Sign In',
    icon: (
      <svg viewBox="0 0 24 24">
        <path d="M14 3h6v18h-6" />
        <path d="M10 12h10" />
        <path d="m7 9 3 3-3 3" />
        <path d="M4 4h8v16H4" />
      </svg>
    ),
  },
  {
    href: '#enable-self-sign-up-and-registration',
    label: 'Self Sign-Up',
    icon: (
      <svg viewBox="0 0 24 24">
        <path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" />
        <circle cx="9" cy="7" r="4" />
        <path d="M19 8v6" />
        <path d="M16 11h6" />
      </svg>
    ),
  },
  {
    href: '#add-a-profile-section',
    label: 'Manage Profile',
    icon: (
      <svg viewBox="0 0 24 24">
        <circle cx="12" cy="8" r="4" />
        <path d="M4 21a8 8 0 0 1 16 0" />
      </svg>
    ),
  },
  {
    href: '#account-recovery',
    label: 'Recover Access',
    icon: (
      <svg viewBox="0 0 24 24">
        <path d="M7 11V9a5 5 0 0 1 10 0v2" />
        <rect x="5" y="11" width="14" height="9" rx="2" />
        <path d="M12 15v2" />
      </svg>
    ),
  },
  {
    href: '#onboard-internal-users',
    label: 'Internal Users',
    icon: (
      <svg viewBox="0 0 24 24">
        <rect x="3" y="7" width="18" height="13" rx="2" />
        <path d="M9 7V5a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2v2" />
        <path d="M3 13h18" />
      </svg>
    ),
  },
];

const solutionPatternNodes: RoadmapNode[] = [
  {
    href: '#redirect-based',
    label: 'Redirect-Based',
    icon: (
      <svg viewBox="0 0 24 24">
        <path d="M4 12h12" />
        <path d="m12 6 6 6-6 6" />
        <circle cx="20" cy="12" r="2" />
      </svg>
    ),
  },
  {
    href: '#app-native',
    label: 'App-Native',
    icon: (
      <svg viewBox="0 0 24 24">
        <rect x="3" y="4" width="7" height="7" rx="1" />
        <rect x="14" y="4" width="7" height="7" rx="1" />
        <rect x="3" y="13" width="7" height="7" rx="1" />
        <rect x="14" y="13" width="7" height="7" rx="1" />
      </svg>
    ),
  },
  {
    href: '#direct-api',
    label: 'Direct API',
    icon: (
      <svg viewBox="0 0 24 24">
        <path d="m8 4-6 8 6 8" />
        <path d="m16 4 6 8-6 8" />
        <path d="M14 4 10 20" />
      </svg>
    ),
  },
];

export function B2CIdentityJourneyRoadmap() {
  return (
    <nav className="uc-b2c-roadmap" aria-label="B2C identity use case roadmap">
      {roadmapNodes.map((node) => (
        <a key={node.href} href={node.href} className="uc-b2c-roadmap__node">
          <span className="uc-b2c-roadmap__icon" aria-hidden>
            {node.icon}
          </span>
          <span className="uc-b2c-roadmap__label">{node.label}</span>
        </a>
      ))}
    </nav>
  );
}

export function B2CSolutionPatternsRoadmap() {
  return (
    <nav className="uc-b2c-roadmap" aria-label="B2C solution pattern roadmap">
      {solutionPatternNodes.map((node) => (
        <a key={node.href} href={node.href} className="uc-b2c-roadmap__node">
          <span className="uc-b2c-roadmap__icon" aria-hidden>
            {node.icon}
          </span>
          <span className="uc-b2c-roadmap__label">{node.label}</span>
        </a>
      ))}
    </nav>
  );
}
