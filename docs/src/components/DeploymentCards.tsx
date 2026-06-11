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

import Link from '@docusaurus/Link';
import React from 'react';

interface Feature {
  text: string;
  available: boolean;
}

interface Station {
  href: string;
  accentColor: string;
  iconBackground: string;
  title: string;
  chooseIf: string;
  cta: string;
  icon: React.ReactNode;
  features: Feature[];
  featured?: boolean;
  animClass: string;
}

function DockerLogo() {
  return <img src="/img/docker-logo.svg" alt="Docker" style={{objectFit: 'contain', display: 'block'}} />;
}

function KubernetesLogo() {
  return <img src="/img/kubernetes-logo.svg" alt="Kubernetes" style={{objectFit: 'contain', display: 'block'}} />;
}

function OpenChoreoLogo() {
  return (
    <img
      className="dp-openchoreo-logo"
      src="/img/openchoreo-logo.svg"
      alt="OpenChoreo"
      style={{objectFit: 'contain', display: 'block'}}
    />
  );
}

function CheckIcon() {
  return (
    <svg
      width="12"
      height="12"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="3"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <polyline points="20 6 9 17 4 12" />
    </svg>
  );
}

function DashIcon() {
  return (
    <svg
      width="12"
      height="12"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="3"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <line x1="5" y1="12" x2="19" y2="12" />
    </svg>
  );
}

const stations: Station[] = [
  {
    href: './docker',
    accentColor: '#0ea5e9',
    iconBackground: 'rgba(14,165,233,0.12)',
    title: 'Docker',
    chooseIf: 'You want a portable, self-contained deployment with full configuration control and no cluster infrastructure required.',
    cta: 'Deploy with Docker →',
    icon: <DockerLogo />,
    features: [
      {text: 'Quick setup with pre-built images', available: true},
      {text: 'PostgreSQL integration', available: true},
      {text: 'Custom configuration mounting', available: true},
    ],
    animClass: 'dp-row-1',
  },
  {
    href: './kubernetes',
    accentColor: '#326CE5',
    iconBackground: 'rgba(50,108,229,0.12)',
    title: 'Kubernetes',
    chooseIf: 'You want production control while running ThunderID on infrastructure your team manages.',
    cta: 'Deploy on Kubernetes →',
    icon: <KubernetesLogo />,
    features: [
      {text: 'Helm chart deployment', available: true},
      {text: 'Multi-replica support', available: true},
      {text: 'Ingress configuration', available: true},
      {text: 'Database flexibility (PostgreSQL/SQLite)', available: true},
      {text: 'Rolling updates and rollbacks', available: true},
    ],
    animClass: 'dp-row-2',
  },
  {
    href: './openchoreo',
    accentColor: '#8b5cf6',
    iconBackground: 'rgba(139,92,246,0.12)',
    title: 'OpenChoreo',
    chooseIf: 'You want a platform-managed deployment model with environment separation and promotion workflows.',
    cta: 'Deploy on OpenChoreo →',
    icon: <OpenChoreoLogo />,
    features: [
      {text: 'Cell-based deployment model', available: true},
      {text: 'Integrated platform services', available: true},
      {text: 'Advanced networking', available: true},
      {text: 'Service mesh integration', available: true},
    ],
    animClass: 'dp-row-3',
  },
];

export default function DeploymentCards() {
  return (
    <div className="dp-wrap">
      <div className="dp-header">
        <div className="dp-header-kicker">Deployment</div>
        <h1 className="dp-header-title">Where are you deploying?</h1>
        <p className="dp-header-sub">
          Pick the environment that matches your setup. Each option has a dedicated installation guide.
        </p>
      </div>

      {/* 3-column cards */}
      <div className="dp-list">
        {stations.map((s) => (
          <Link
            key={s.href}
            to={s.href}
            className={`dp-row-card ${s.animClass}${s.featured ? ' dp-featured' : ''}`}
            style={{['--dp-accent' as string]: s.accentColor, ['--dp-icon-bg' as string]: s.iconBackground}}
          >
            <div className="dp-card-icon">{s.icon}</div>
            <div className="dp-rc-title">{s.title}</div>

            <ul className="dp-feature-list">
              {s.features.map((f) => (
                <li key={f.text} className={`dp-feature-item ${f.available ? 'dp-feat-check' : 'dp-feat-miss'}`}>
                  <span className="dp-feat-icon">{f.available ? <CheckIcon /> : <DashIcon />}</span>
                  <span>{f.text}</span>
                </li>
              ))}
            </ul>

            <div className="dp-card-choose">
              <div className="dp-card-choose-label">Choose this if…</div>
              <p className="dp-card-choose-text">{s.chooseIf}</p>
            </div>

            <span className="dp-rc-cta">{s.cta}</span>
          </Link>
        ))}
      </div>

    </div>
  );
}
