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

/* eslint-disable react-refresh/only-export-components -- Icon registry module: exports a lookup map of icon components, not an HMR component boundary. */

import {ComponentType, JSX} from 'react';

export type BlogHeroIconKey = 'shield' | 'chain' | 'box' | 'globe' | 'graph' | 'chevrons' | 'agent' | 'default';

interface IconProps {size?: number}

function base(children: JSX.Element, size = 24): JSX.Element {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.35"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      {children}
    </svg>
  );
}

function ShieldIcon({size}: IconProps): JSX.Element {
  return base(
    <>
      <path d="M12 3l7 3v5c0 4.5-3 8-7 10-4-2-7-5.5-7-10V6l7-3z" />
      <path d="M9 12l2 2 4-4" />
    </>,
    size,
  );
}

function ChainIcon({size}: IconProps): JSX.Element {
  return base(
    <>
      <path d="M9 15l6-6" />
      <path d="M11 6l1.5-1.5a3.5 3.5 0 0 1 5 5L16 11" />
      <path d="M13 18l-1.5 1.5a3.5 3.5 0 0 1-5-5L8 13" />
    </>,
    size,
  );
}

function BoxIcon({size}: IconProps): JSX.Element {
  return base(
    <>
      <path d="M21 8l-9-5-9 5 9 5 9-5z" />
      <path d="M3 8v8l9 5 9-5V8" />
      <path d="M12 13v8" />
    </>,
    size,
  );
}

function GlobeIcon({size}: IconProps): JSX.Element {
  return base(
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M3 12h18" />
      <path d="M12 3c2.5 2.5 3.8 5.7 3.8 9s-1.3 6.5-3.8 9c-2.5-2.5-3.8-5.7-3.8-9S9.5 5.5 12 3z" />
    </>,
    size,
  );
}

function GraphIcon({size}: IconProps): JSX.Element {
  return base(
    <>
      <circle cx="6" cy="18" r="2" />
      <circle cx="18" cy="6" r="2" />
      <circle cx="18" cy="18" r="2" />
      <path d="M8 17l8-10" />
      <path d="M16 18h0" />
      <path d="M8 18h6" />
    </>,
    size,
  );
}

function ChevronsIcon({size}: IconProps): JSX.Element {
  return base(
    <>
      <polyline points="9 6 4 12 9 18" />
      <polyline points="15 6 20 12 15 18" />
    </>,
    size,
  );
}

function AgentIcon({size}: IconProps): JSX.Element {
  return base(
    <>
      <rect x="5" y="8" width="14" height="11" rx="3" />
      <circle cx="12" cy="4.5" r="1.5" />
      <path d="M12 6v2" />
      <circle cx="9.5" cy="13.5" r="1.2" fill="currentColor" stroke="none" />
      <circle cx="14.5" cy="13.5" r="1.2" fill="currentColor" stroke="none" />
      <path d="M3 12h2" />
      <path d="M19 12h2" />
    </>,
    size,
  );
}

function DefaultIcon({size}: IconProps): JSX.Element {
  return base(
    <>
      <path d="M6 3h9l3 3v15H6z" />
      <path d="M15 3v3h3" />
      <path d="M9 12h6" />
      <path d="M9 16h6" />
    </>,
    size,
  );
}

export const BLOG_HERO_ICONS: Record<BlogHeroIconKey, ComponentType<IconProps>> = {
  shield: ShieldIcon,
  chain: ChainIcon,
  box: BoxIcon,
  globe: GlobeIcon,
  graph: GraphIcon,
  chevrons: ChevronsIcon,
  agent: AgentIcon,
  default: DefaultIcon,
};

export const DEFAULT_HERO_GRADIENT = 'linear-gradient(135deg,#0c2747 0%,#2563c9 100%)';
