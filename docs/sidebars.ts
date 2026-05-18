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

/* eslint-disable @typescript-eslint/no-unsafe-assignment */

import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';
import reactSdkSidebar from './content/sdks/react/sidebar';
import productConfig from './docusaurus.product.config';

// This runs in Node.js - Don't use client-side code here (browser APIs, JSX...)

// TODO: Use `@wso2/oxygen-ui-icons` in the sidebar. Currently, there's only a React wrapper available, so we need to create custom SVG icons for the sidebar until we have a web component version of the icons.

/**
 * Creating a sidebar enables you to:
 - create an ordered group of docs
 - render a sidebar for each doc of that group
 - provide next/previous navigation

 The sidebars can be generated from the filesystem, or explicitly defined here.

 Create as many sidebars as you want.
 */
const sidebars: SidebarsConfig = {
  docsSidebar: [
    {
      type: 'doc',
      id: 'index',
      label: 'Home',
      className: 'sidebar-doc-home',
    },
    // Introduction Section
    {
      type: 'html',
      value:
        '<div class="sidebar-section-label"><svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z"/><path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z"/></svg><span>Getting Started</span></div>',
      className: 'sidebar-html-section-header',
    },
    {
      type: 'category',
      label: 'Getting Started',
      collapsed: false,
      collapsible: false,
      className: 'sidebar-section',
      items: [
        {
          type: 'doc',
          id: 'guides/getting-started/what-is-thunderid',
          label: `What is ${productConfig.project.name}?`,
        },
        {
          type: 'doc',
          id: 'guides/getting-started/get-thunderid',
          label: `Get ${productConfig.project.name}`,
        },
        {
          type: 'doc',
          id: 'guides/quick-start/quickstart',
          label: 'Register an Application',
        },
        {
          type: 'doc',
          id: 'guides/guides/flows/build-a-flow',
          label: 'Build a Sign-In Flow',
        },
        {
          type: 'category',
          label: 'Connect Your Application',
          collapsed: false,
          collapsible: true,
          items: [
            {
              type: 'doc',
              id: 'guides/quick-start/connect-your-application/react',
              label: 'React',
            },
            {
              type: 'doc',
              id: 'guides/quick-start/connect-your-application/vue',
              label: 'Vue',
            },
          ],
        },
      ],
    },

    // Working with AI Section
    {
      type: 'html',
      value:
        '<div class="sidebar-section-label sidebar-section-label--ai"><svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m12 3-1.912 5.813a2 2 0 0 1-1.275 1.275L3 12l5.813 1.912a2 2 0 0 1 1.275 1.275L12 21l1.912-5.813a2 2 0 0 1 1.275-1.275L21 12l-5.813-1.912a2 2 0 0 1-1.275-1.275L12 3Z"/><path d="M5 3v4"/><path d="M19 17v4"/><path d="M3 5h4"/><path d="M17 19h4"/></svg><span>Working with AI</span></div>',
      className: 'sidebar-html-section-header sidebar-persona-iam sidebar-persona-not-devops',
    },
    {
      type: 'category',
      label: 'Working with AI',
      collapsed: false,
      collapsible: false,
      className: 'sidebar-section sidebar-persona-iam sidebar-persona-not-devops',
      items: [
        {
          type: 'doc',
          id: 'guides/working-with-ai/mcp-server',
          label: 'MCP Server',
        },
      ],
    },

    // Guides Section
    {
      type: 'html',
      value:
        '<div class="sidebar-section-label"><svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="10"/><polygon points="16.24 7.76 14.12 14.12 7.76 16.24 9.88 9.88 16.24 7.76"/></svg><span>Guides</span></div>',
      className: 'sidebar-html-section-header sidebar-persona-iam sidebar-persona-not-devops',
    },
    {
      type: 'category',
      label: 'Guides',
      collapsed: false,
      collapsible: false,
      className: 'sidebar-section sidebar-persona-iam sidebar-persona-not-devops',
      items: [
        {
          type: 'category',
          label: 'Applications',
          collapsed: true,
          collapsible: true,
          items: [
            {
              type: 'doc',
              id: 'guides/guides/applications/manage-applications',
              label: 'Manage Applications',
            },
            {
              type: 'doc',
              id: 'guides/guides/applications/application-settings',
              label: 'Application Settings',
            },
            {
              type: 'doc',
              id: 'guides/guides/applications/dynamic-client-registration',
              label: 'Dynamic Client Registration',
            },
          ],
        },
        {
          type: 'category',
          label: 'Users',
          collapsed: true,
          collapsible: true,
          items: [
            {
              type: 'doc',
              id: 'guides/guides/users/manage-users',
              label: 'Manage Users',
            },
            {
              type: 'doc',
              id: 'guides/guides/users/manage-groups',
              label: 'Manage Groups',
            },
            {
              type: 'doc',
              id: 'guides/guides/users/user-types',
              label: 'User Types',
            },
            {
              type: 'doc',
              id: 'guides/guides/users/user-type-reference',
              label: 'User Type Reference',
            },
          ],
        },
        {
          type: 'category',
          label: 'Identity Providers',
          collapsed: true,
          collapsible: true,
          items: [
            {
              type: 'doc',
              id: 'guides/guides/identity-providers/overview',
              label: 'What are Identity Providers?',
            },
            {
              type: 'doc',
              id: 'guides/guides/identity-providers/manage-identity-providers',
              label: 'Manage Identity Providers',
            },
            {
              type: 'doc',
              id: 'guides/guides/identity-providers/add-google',
              label: 'Add Google',
            },
            {
              type: 'doc',
              id: 'guides/guides/identity-providers/add-github',
              label: 'Add GitHub',
            },
            {
              type: 'doc',
              id: 'guides/guides/identity-providers/add-oidc-provider',
              label: 'Add an OIDC Provider',
            },
            {
              type: 'doc',
              id: 'guides/guides/identity-providers/add-oauth-provider',
              label: 'Add an OAuth 2.0 Provider',
            },
            {
              type: 'doc',
              id: 'guides/guides/identity-providers/connect-idp-to-application',
              label: 'Connect to an Application',
            },
          ],
        },
        {
          type: 'doc',
          id: 'guides/guides/organization-units',
          label: 'Organization Units',
        },
        {
          type: 'category',
          label: 'Flows',
          collapsed: true,
          collapsible: true,
          items: [
            {
              type: 'doc',
              id: 'guides/guides/flows/what-is-flows',
              label: 'What are Flows?',
            },
            {
              type: 'doc',
              id: 'guides/guides/flows/flow-concepts',
              label: 'Flow Concepts',
            },
            {
              type: 'doc',
              id: 'guides/guides/flows/build-a-flow',
              label: 'Build a Flow',
            },
            {
              type: 'doc',
              id: 'guides/guides/flows/flow-reference',
              label: 'Flow Reference',
            },
          ],
        },
        {
          type: 'doc',
          id: 'guides/guides/consent',
          label: 'Consent',
        },
        {
          type: 'doc',
          id: 'guides/guides/trusted-issuer',
          label: 'Trusted Issuer',
        },
      ],
    },

    // Use Cases Section
    {
      type: 'html',
      value:
        '<div class="sidebar-section-label"><svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polygon points="12 2 2 7 12 12 22 7 12 2"/><polyline points="2 17 12 22 22 17"/><polyline points="2 12 12 17 22 12"/></svg><span>Use Cases</span></div>',
      className: 'sidebar-html-section-header',
    },
    {
      type: 'category',
      label: 'Use Cases',
      collapsed: false,
      collapsible: false,
      className: 'sidebar-section',
      items: [
        {type: 'doc', id: 'use-cases/overview', label: 'Overview'},
        {
          type: 'category',
          label: 'Consumer Applications (B2C)',
          collapsible: true,
          collapsed: true,
          items: [
            {type: 'doc', id: 'use-cases/b2c/customer-identity', label: 'Customer Identity'},
          ],
        },
        {
          type: 'category',
          label: 'SaaS Applications (B2B)',
          collapsible: true,
          collapsed: true,
          items: [
            {type: 'doc', id: 'use-cases/b2b/multi-tenant-saas', label: 'Multi-Tenant SaaS'},
          ],
        },
        {
          type: 'category',
          label: 'AI Agents and MCP',
          collapsible: true,
          collapsed: true,
          items: [
            {type: 'doc', id: 'use-cases/ai-agents/agent-authentication', label: 'Agent Authentication'},
          ],
        },
      ],
    },

    // Key Concepts Section
    {
      type: 'html',
      value:
        '<div class="sidebar-section-label"><svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M15 14c.2-1 .7-1.7 1.5-2.5 1-.9 1.5-2.2 1.5-3.5A6 6 0 0 0 6 8c0 1 .2 2.2 1.5 3.5.7.7 1.3 1.5 1.5 2.5"/><path d="M9 18h6"/><path d="M10 22h4"/></svg><span>Key Concepts</span></div>',
      className: 'sidebar-html-section-header sidebar-persona-not-devops',
    },
    {
      type: 'category',
      label: 'Key Concepts',
      collapsed: false,
      collapsible: false,
      className: 'sidebar-section sidebar-persona-not-devops',
      items: [
        {
          type: 'category',
          label: 'Authentication',
          collapsed: true,
          items: [
            {
              type: 'doc',
              id: 'guides/key-concepts/authentication/overview',
            },
            {
              type: 'category',
              label: 'Passwordless',
              collapsed: true,
              items: [
                {
                  type: 'doc',
                  id: 'guides/key-concepts/authentication/passwordless/overview',
                },
                {
                  type: 'doc',
                  id: 'guides/key-concepts/authentication/passwordless/passkeys',
                  label: 'Passkeys',
                },
              ],
            },
          ],
        },

        {
          type: 'doc',
          id: 'guides/key-concepts/authorization',
          label: 'Authorization',
        },
        {
          type: 'doc',
          id: 'guides/key-concepts/tokens',
          label: 'Tokens',
        },
        {
          type: 'doc',
          id: 'guides/key-concepts/events',
          label: 'Events',
        },
      ],
    },

    // Deployment Patterns Section
    {
      type: 'html',
      value:
        '<div class="sidebar-section-label"><svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect width="20" height="8" x="2" y="2" rx="2" ry="2"/><rect width="20" height="8" x="2" y="14" rx="2" ry="2"/><line x1="6" x2="6.01" y1="6" y2="6"/><line x1="6" x2="6.01" y1="18" y2="18"/></svg><span>Deployment Patterns</span></div>',
      className: 'sidebar-html-section-header sidebar-persona-iam',
    },
    {
      type: 'category',
      label: 'Deployment Patterns',
      collapsed: false,
      collapsible: false,
      className: 'sidebar-section sidebar-persona-iam',
      items: [
        {
          type: 'doc',
          id: 'guides/getting-started/configuration',
          label: 'Configuration',
        },
        {
          type: 'doc',
          id: 'guides/deployment-patterns/index',
          label: 'Choose Your Deployment',
        },
        {
          type: 'doc',
          id: 'guides/deployment-patterns/docker',
          label: 'Docker',
        },
        {
          type: 'doc',
          id: 'guides/deployment-patterns/kubernetes',
          label: 'Kubernetes',
        },
        {
          type: 'doc',
          id: 'guides/deployment-patterns/openchoreo',
          label: 'OpenChoreo',
        },
      ],
    },
  ],
  reactSdkSidebar,
  communitySidebar: [{type: 'autogenerated', dirName: 'community'}],
};

export default sidebars;
