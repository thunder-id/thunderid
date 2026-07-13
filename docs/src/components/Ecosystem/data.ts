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

import {ComponentType} from 'react';
import AndroidLogo from '../icons/AndroidLogo';
import AngularLogo from '../icons/AngularLogo';
import AuthJsLogo from '../icons/AuthJsLogo';
import BetterAuthLogo from '../icons/BetterAuthLogo';
import BrowserLogo from '../icons/BrowserLogo';
import ClaudeLogo from '../icons/ClaudeLogo';
import CodexLogo from '../icons/CodexLogo';
import ExpressLogo from '../icons/ExpressLogo';
import FlutterLogo from '../icons/FlutterLogo';
import GoLogo from '../icons/GoLogo';
import IOSLogo from '../icons/IOSLogo';
import JavaScriptLogo from '../icons/JavaScriptLogo';
import NextLogo from '../icons/NextLogo';
import NodeLogo from '../icons/NodeLogo';
import NuxtAuthLogo from '../icons/NuxtAuthLogo';
import NuxtLogo from '../icons/NuxtLogo';
import PassportLogo from '../icons/PassportLogo';
import PythonLogo from '../icons/PythonLogo';
import ReactLogo from '../icons/ReactLogo';
import ReactRouterLogo from '../icons/ReactRouterLogo';
import SkillsLogo from '../icons/SkillsLogo';
import SpringSecurityLogo from '../icons/SpringSecurityLogo';
import TanStackLogo from '../icons/TanStackLogo';
import VueLogo from '../icons/VueLogo';

export type EcosystemCategory = 'agent' | 'spa' | 'fullstack' | 'backend' | 'mobile' | 'integration' | 'utility';

export type EcosystemPackageManager = 'npm' | 'go' | 'pip' | 'gradle' | 'pod' | 'pub';

export interface EcosystemItem {
  id: string;
  name: string;
  icon: ComponentType<{size?: number}>;
  packageName: string;
  packageManager?: EcosystemPackageManager;
  category: EcosystemCategory;
  description: string;
  ctaLabel: string;
  href?: string;
  soon?: boolean;
}

export const CATEGORY_LABELS: Record<EcosystemCategory, string> = {
  agent: 'Agent tooling',
  spa: 'SPA',
  fullstack: 'Fullstack',
  backend: 'Backend',
  mobile: 'Mobile',
  integration: 'Integration',
  utility: 'Utility',
};

export const FILTER_TABS: {key: 'all' | EcosystemCategory; label: string}[] = [
  {key: 'all', label: 'All'},
  {key: 'agent', label: 'Agent tooling'},
  {key: 'spa', label: 'SPA'},
  {key: 'fullstack', label: 'Fullstack'},
  {key: 'utility', label: 'Utility'},
  {key: 'backend', label: 'Backend'},
  {key: 'mobile', label: 'Mobile'},
  {key: 'integration', label: 'Integrations'},
];

// Note: unlike the design mockup (which marked iOS/Android/Flutter as "soon"), this repo already
// ships real docs for those three native SDKs (content/sdks/{android,ios,flutter}/overview.mdx),
// so they're listed as available here instead. Angular/Go/Python/React Native have no SDK yet.
export const ECOSYSTEM_ITEMS: EcosystemItem[] = [
  {
    id: 'claude-plugin',
    name: 'Claude Plugin',
    icon: ClaudeLogo,
    packageName: 'thunderid (Claude)',
    category: 'agent',
    description:
      'Add {{ProductName}} auth to Claude. Secure your tools, MCP servers, and agent actions with scoped, revocable tokens.',
    ctaLabel: 'Get plugin',
    href: '/docs/next/guides/working-with-ai/skills',
  },
  {
    id: 'codex-plugin',
    name: 'Codex Plugin',
    icon: CodexLogo,
    packageName: 'thunderid (Codex)',
    category: 'agent',
    description: 'Bring {{ProductName}} sign-in and token management to OpenAI Codex and the Codex CLI.',
    ctaLabel: 'Get plugin',
    href: '/docs/next/guides/working-with-ai/skills',
  },
  {
    id: 'agent-skills',
    name: 'Agent Skills',
    icon: SkillsLogo,
    packageName: '@thunderid/skills',
    category: 'agent',
    description:
      'Reusable skills that let any agent issue, verify, and revoke {{ProductName}} credentials as part of a workflow.',
    ctaLabel: 'Browse skills',
    href: '/docs/next/guides/working-with-ai/skills',
  },
  {
    id: 'react',
    name: 'React',
    icon: ReactLogo,
    packageName: '@thunderid/react',
    packageManager: 'npm',
    category: 'spa',
    description: 'Hooks and components to drop {{ProductName}} into a React + Vite app in minutes.',
    ctaLabel: 'Read APIs',
    href: '/docs/next/sdks/react/overview',
  },
  {
    id: 'vue',
    name: 'Vue',
    icon: VueLogo,
    packageName: '@thunderid/vue',
    packageManager: 'npm',
    category: 'spa',
    description: 'Composables and plugins to add authentication to any Vue.js application.',
    ctaLabel: 'Read APIs',
    href: '/docs/next/sdks/vue/overview',
  },
  {
    id: 'browser',
    name: 'Browser',
    icon: BrowserLogo,
    packageName: '@thunderid/browser',
    packageManager: 'npm',
    category: 'spa',
    description: 'Vanilla JavaScript SDK for browser apps without a framework.',
    ctaLabel: 'Read APIs',
    href: '/docs/next/sdks/browser/overview',
  },
  {
    id: 'javascript',
    name: 'JavaScript',
    icon: JavaScriptLogo,
    packageName: '@thunderid/javascript',
    packageManager: 'npm',
    category: 'spa',
    description: 'Framework-agnostic core that powers the browser, Node, and platform SDKs.',
    ctaLabel: 'Read APIs',
    href: '/docs/next/sdks/javascript/overview',
  },
  {
    id: 'nextjs',
    name: 'Next.js',
    icon: NextLogo,
    packageName: '@thunderid/nextjs',
    packageManager: 'npm',
    category: 'fullstack',
    description: 'Authentication for Next.js with full App Router and server-component support.',
    ctaLabel: 'Read APIs',
    href: '/docs/next/sdks/nextjs/overview',
  },
  {
    id: 'nuxt',
    name: 'Nuxt',
    icon: NuxtLogo,
    packageName: '@thunderid/nuxt',
    packageManager: 'npm',
    category: 'fullstack',
    description: 'Server-rendered authentication for Nuxt applications, out of the box.',
    ctaLabel: 'Read APIs',
    href: '/docs/next/sdks/nuxt/overview',
  },
  {
    id: 'react-router',
    name: 'React Router',
    icon: ReactRouterLogo,
    packageName: '@thunderid/react-router',
    packageManager: 'npm',
    category: 'utility',
    description: 'Authentication for React Router (formerly Remix) applications.',
    ctaLabel: 'Read APIs',
    href: '/docs/next/sdks/react-router/overview',
  },
  {
    id: 'tanstack-router',
    name: 'TanStack Router',
    icon: TanStackLogo,
    packageName: '@thunderid/tanstack-router',
    packageManager: 'npm',
    category: 'utility',
    description: 'Type-safe authentication for TanStack Router applications.',
    ctaLabel: 'Read APIs',
    href: '/docs/next/sdks/tanstack-router/overview',
  },
  {
    id: 'express',
    name: 'Express',
    icon: ExpressLogo,
    packageName: '@thunderid/express',
    packageManager: 'npm',
    category: 'backend',
    description: 'Middleware to protect Express routes and APIs with {{ProductName}}.',
    ctaLabel: 'Read APIs',
    href: '/docs/next/sdks/express/overview',
  },
  {
    id: 'node',
    name: 'Node.js',
    icon: NodeLogo,
    packageName: '@thunderid/node',
    packageManager: 'npm',
    category: 'backend',
    description: 'Server-side SDK for verifying tokens and securing Node.js services.',
    ctaLabel: 'Read APIs',
    href: '/docs/next/sdks/node/overview',
  },
  {
    id: 'nextauth-provider',
    name: 'Auth.js (NextAuth)',
    icon: AuthJsLogo,
    packageName: 'next-auth',
    category: 'integration',
    description: 'Official {{ProductName}} provider built into Auth.js — configure and go, no extra package.',
    ctaLabel: '',
    soon: true,
  },
  {
    id: 'passport-strategy',
    name: 'Passport Strategy',
    icon: PassportLogo,
    packageName: 'passport',
    category: 'integration',
    description: '{{ProductName}} strategy shipped directly in Passport.js for Node and Express back ends.',
    ctaLabel: '',
    soon: true,
  },
  {
    id: 'nuxt-auth-provider',
    name: 'Nuxt Auth Provider',
    icon: NuxtAuthLogo,
    packageName: '@sidebase/nuxt-auth',
    category: 'integration',
    description: '{{ProductName}} provider built into @sidebase/nuxt-auth — just add your credentials.',
    ctaLabel: '',
    soon: true,
  },
  {
    id: 'better-auth',
    name: 'Better Auth',
    icon: BetterAuthLogo,
    packageName: 'better-auth',
    category: 'integration',
    description: '{{ProductName}} provider for Better Auth — drop-in support for the TypeScript-native auth framework.',
    ctaLabel: '',
    soon: true,
  },
  {
    id: 'spring-security',
    name: 'Spring Security',
    icon: SpringSecurityLogo,
    packageName: 'io.thunderid:spring-security',
    category: 'integration',
    description: '{{ProductName}} integration for Spring Security — secure Java and Kotlin back ends with minimal configuration.',
    ctaLabel: '',
    soon: true,
  },
  {
    id: 'ios',
    name: 'iOS',
    icon: IOSLogo,
    packageName: 'ThunderID',
    packageManager: 'pod',
    category: 'mobile',
    description: 'Native Swift SDK with hosted login and secure token storage.',
    ctaLabel: 'Read APIs',
    href: '/docs/next/sdks/ios/overview',
  },
  {
    id: 'android',
    name: 'Android',
    icon: AndroidLogo,
    packageName: 'io.thunderid:android',
    packageManager: 'gradle',
    category: 'mobile',
    description: 'Native Kotlin SDK with hosted login and secure token storage.',
    ctaLabel: 'Read APIs',
    href: '/docs/next/sdks/android/overview',
  },
  {
    id: 'flutter',
    name: 'Flutter',
    icon: FlutterLogo,
    packageName: 'thunderid_flutter',
    packageManager: 'pub',
    category: 'mobile',
    description: 'Cross-platform authentication for Flutter apps.',
    ctaLabel: 'Read APIs',
    href: '/docs/next/sdks/flutter/overview',
  },
  {
    id: 'angular',
    name: 'Angular',
    icon: AngularLogo,
    packageName: '@thunderid/angular',
    category: 'spa',
    description: 'Guards, services, and components for {{ProductName}} in Angular apps.',
    ctaLabel: '',
    soon: true,
  },
  {
    id: 'go',
    name: 'Go',
    icon: GoLogo,
    packageName: 'github.com/thunder-id/go',
    category: 'backend',
    description: 'Idiomatic Go SDK for verifying tokens and protecting services.',
    ctaLabel: '',
    soon: true,
  },
  {
    id: 'python',
    name: 'Python',
    icon: PythonLogo,
    packageName: 'thunderid',
    category: 'backend',
    description: 'Python SDK for Django, FastAPI, and Flask back ends.',
    ctaLabel: '',
    soon: true,
  },
  {
    id: 'react-native',
    name: 'React Native',
    icon: ReactLogo,
    packageName: '@thunderid/react-native',
    category: 'mobile',
    description: 'Cross-platform mobile authentication for React Native apps.',
    ctaLabel: '',
    soon: true,
  },
];
