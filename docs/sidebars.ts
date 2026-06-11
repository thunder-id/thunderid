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
import browserSdkSidebar from './content/sdks/browser/sidebar';
import expressSdkSidebar from './content/sdks/express/sidebar';
import javascriptSdkSidebar from './content/sdks/javascript/sidebar';
import nextjsSdkSidebar from './content/sdks/nextjs/sidebar';
import nodeSdkSidebar from './content/sdks/node/sidebar';
import nuxtSdkSidebar from './content/sdks/nuxt/sidebar';
import reactSdkSidebar from './content/sdks/react/sidebar';
import reactRouterSdkSidebar from './content/sdks/react-router/sidebar';
import tanstackRouterSdkSidebar from './content/sdks/tanstack-router/sidebar';
import vueSdkSidebar from './content/sdks/vue/sidebar';
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
          id: 'guides/getting-started/register-an-application',
          label: 'Register an Application',
        },
        {
          type: 'doc',
          id: 'guides/getting-started/build-a-flow',
          label: 'Build a Sign-In Flow',
        },
        {
          type: 'category',
          label: 'Connect Your Application',
          link: {
            type: 'doc',
            id: 'guides/getting-started/connect-your-application/index',
          },
          collapsed: true,
          collapsible: true,
          items: [
            {
              type: 'doc',
              id: 'guides/getting-started/connect-your-application/react',
              label: 'React',
            },
            {
              type: 'doc',
              id: 'guides/getting-started/connect-your-application/nextjs',
              label: 'Next.js',
            },
            {
              type: 'doc',
              id: 'guides/getting-started/connect-your-application/express',
              label: 'Express',
            },
            {
              type: 'doc',
              id: 'guides/getting-started/connect-your-application/vue',
              label: 'Vue',
            },
            {
              type: 'doc',
              id: 'guides/getting-started/connect-your-application/nuxt',
              label: 'Nuxt',
            },
            {
              type: 'doc',
              id: 'guides/getting-started/connect-your-application/node',
              label: 'Node.js',
            },
            {
              type: 'doc',
              id: 'guides/getting-started/connect-your-application/browser',
              label: 'Vanilla JavaScript',
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
          id: 'guides/working-with-ai/skills',
          label: 'Skills',
        },
        {
          type: 'doc',
          id: 'guides/working-with-ai/mcp-server',
          label: 'MCP Server',
        },
        {
          type: 'doc',
          id: 'guides/working-with-ai/get-started-with-mcp',
          label: 'Getting Started with MCP',
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
              id: 'guides/guides/users/manage-roles',
              label: 'Manage Roles',
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
          label: 'Agents',
          collapsed: true,
          collapsible: true,
          items: [
            {
              type: 'doc',
              id: 'guides/guides/agents/manage-agents',
              label: 'Manage Agents',
            },
          ],
        },
        {
          type: 'category',
          label: 'Integrations',
          collapsed: true,
          collapsible: true,
          items: [
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
                  label: 'Add OIDC Provider',
                },
                {
                  type: 'doc',
                  id: 'guides/guides/identity-providers/add-oauth-provider',
                  label: 'Add OAuth 2.0 Provider',
                },
                {
                  type: 'doc',
                  id: 'guides/guides/identity-providers/manage-identity-providers',
                  label: 'Manage Identity Providers',
                },
                {
                  type: 'doc',
                  id: 'guides/guides/identity-providers/connect-idp-to-application',
                  label: 'Connect IdP to Application',
                },
              ],
            },
            {
              type: 'category',
              label: 'Notifications',
              collapsed: true,
              collapsible: true,
              items: [
                {
                  type: 'doc',
                  id: 'guides/guides/notifications/sms-providers',
                  label: 'SMS Providers',
                },
              ],
            },
            {
              type: 'doc',
              id: 'guides/guides/smtp-server/smtp-server-configuration',
              label: 'Configure SMTP Server',
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
              id: 'guides/guides/flows/what-are-flows',
              label: 'What Are Flows?',
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
              id: 'guides/guides/flows/advanced-configurations',
              label: 'Advanced Configurations',
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
        {
          type: 'doc',
          id: 'guides/guides/resource-servers',
          label: 'Resource Servers',
        },
        {
          type: 'category',
          label: 'Design',
          collapsed: true,
          collapsible: true,
          items: [
            {
              type: 'doc',
              id: 'guides/guides/design/overview',
              label: 'Style Your Experience',
            },
            {
              type: 'doc',
              id: 'guides/guides/design/themes',
              label: 'Themes',
            },
            {
              type: 'doc',
              id: 'guides/guides/design/layouts',
              label: 'Layouts',
            },
            {
              type: 'doc',
              id: 'guides/guides/design/design-catalog',
              label: 'Design Catalog',
            },
          ],
        },
        {
          type: 'category',
          label: 'Translations',
          collapsed: true,
          collapsible: true,
          items: [
            {
              type: 'doc',
              id: 'guides/guides/i18n/localization',
              label: 'Localization',
            },
            {
              type: 'doc',
              id: 'guides/guides/i18n/manage-translations',
              label: 'Manage Translations',
            },
            {
              type: 'doc',
              id: 'guides/guides/i18n/resolve-translations',
              label: 'Resolve Translations',
            },
          ],
        },
        {
          type: 'category',
          label: 'Protocols & Standards',
          collapsed: true,
          collapsible: true,
          link: {
            type: 'doc',
            id: 'guides/guides/protocols/index',
          },
          items: [
            {
              type: 'category',
              label: 'OAuth & OIDC',
              collapsed: true,
              collapsible: true,
              link: {
                type: 'doc',
                id: 'guides/guides/protocols/oauth-oidc/index',
              },
              items: [
                {
                  type: 'category',
                  label: 'Grant Types',
                  collapsed: true,
                  collapsible: true,
                  items: [
                    {type: 'doc', id: 'guides/guides/protocols/oauth-oidc/authorization-code', label: 'Authorization Code'},
                    {type: 'doc', id: 'guides/guides/protocols/oauth-oidc/client-credentials', label: 'Client Credentials'},
                    {type: 'doc', id: 'guides/guides/protocols/oauth-oidc/refresh-token', label: 'Refresh Token'},
                    {type: 'doc', id: 'guides/guides/protocols/oauth-oidc/token-exchange', label: 'Token Exchange'},
                  ],
                },
                {
                  type: 'category',
                  label: 'Client Authentication',
                  collapsed: true,
                  collapsible: true,
                  items: [
                    {
                      type: 'doc',
                      id: 'guides/guides/protocols/oauth-oidc/client-authentication-methods',
                      label: 'Client Authentication Methods',
                    },
                  ],
                },
                {
                  type: 'category',
                  label: 'Security Extensions',
                  collapsed: true,
                  collapsible: true,
                  items: [
                    {type: 'doc', id: 'guides/guides/protocols/oauth-oidc/pkce', label: 'PKCE'},
                    {type: 'doc', id: 'guides/guides/protocols/oauth-oidc/par', label: 'Pushed Authorization Requests'},
                    {type: 'doc', id: 'guides/guides/protocols/oauth-oidc/dpop', label: 'DPoP — Sender-Constrained Tokens'},
                    {type: 'doc', id: 'guides/guides/protocols/oauth-oidc/issuer-identification', label: 'Issuer Identification'},
                    {type: 'doc', id: 'guides/guides/protocols/oauth-oidc/resource-indicators', label: 'Resource Indicators'},
                  ],
                },
                {
                  type: 'category',
                  label: 'Token Operations',
                  collapsed: true,
                  collapsible: true,
                  items: [
                    {type: 'doc', id: 'guides/guides/protocols/oauth-oidc/token-introspection', label: 'Token Introspection'},
                  ],
                },
                {
                  type: 'category',
                  label: 'Discovery & Registration',
                  collapsed: true,
                  collapsible: true,
                  items: [
                    {type: 'doc', id: 'guides/guides/protocols/oauth-oidc/server-metadata', label: 'Server Metadata'},
                    {type: 'doc', id: 'guides/guides/protocols/oauth-oidc/jwks', label: 'JWKS'},
                    {
                      type: 'doc',
                      id: 'guides/guides/protocols/oauth-oidc/dynamic-client-registration',
                      label: 'Dynamic Client Registration',
                    },
                  ],
                },
                {
                  type: 'category',
                  label: 'OIDC',
                  collapsed: true,
                  collapsible: true,
                  items: [
                    {type: 'doc', id: 'guides/guides/protocols/oauth-oidc/openid-connect', label: 'OpenID Connect'},
                    {type: 'doc', id: 'guides/guides/protocols/oauth-oidc/userinfo', label: 'UserInfo'},
                    {type: 'doc', id: 'guides/guides/protocols/oauth-oidc/claims-and-scopes', label: 'Claims & Scopes'},
                    {type: 'doc', id: 'guides/guides/protocols/oauth-oidc/token-formats', label: 'Token Formats'},
                  ],
                },
              ],
            },
            {
              type: 'category',
              label: 'Verifiable Credentials',
              collapsed: true,
              collapsible: true,
              link: {
                type: 'doc',
                id: 'guides/guides/protocols/openid4vc/index',
              },
              items: [
                {
                  type: 'doc',
                  id: 'guides/guides/protocols/openid4vc/openid4vp',
                  label: 'OpenID for Verifiable Presentations',
                },
              ],
            },
          ],
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
        {type: 'doc', id: 'use-cases/overview', label: 'Choose your usecase'},
        {
          type: 'category',
          label: 'Consumer Applications (B2C)',
          collapsible: true,
          collapsed: true,
          items: [
            {type: 'doc', id: 'use-cases/b2c/customer-identity', label: 'Customer Identity'},
            {type: 'doc', id: 'use-cases/b2c/solution-patterns', label: 'Solution Patterns'},
            {
              type: 'category',
              label: 'Try It Out',
              collapsible: true,
              collapsed: true,
              link: {type: 'doc', id: 'use-cases/b2c/try-it-out'},
              items: [
                {
                  type: 'category',
                  label: 'Walkthroughs',
                  collapsible: true,
                  collapsed: true,
                  key: 'b2c-walkthroughs',
                  items: [
                    {type: 'doc', id: 'use-cases/b2c/try-it-out/add-login', label: 'Login'},
                    {type: 'doc', id: 'use-cases/b2c/try-it-out/self-sign-up', label: 'Self Sign-Up'},
                    {type: 'doc', id: 'use-cases/b2c/try-it-out/profile-section', label: 'View Profile'},
                    {type: 'doc', id: 'use-cases/b2c/try-it-out/account-recovery', label: 'Account Recovery'},
                    {
                      type: 'doc',
                      id: 'use-cases/b2c/try-it-out/onboard-internal-users',
                      label: 'Onboard Internal Users',
                    },
                  ],
                },
                {
                  type: 'category',
                  label: 'Learn More',
                  collapsible: true,
                  collapsed: true,
                  key: 'b2c-learn-more',
                  items: [
                    {type: 'doc', id: 'use-cases/b2c/identity-concepts', label: 'Identity Concepts', key: 'b2c-identity-concepts'},
                    {type: 'doc', id: 'use-cases/b2c/configure-it-yourself', label: 'Configure It Yourself', key: 'b2c-configure-it-yourself'},
                  ],
                },
              ],
            },
            {
              type: 'category',
              label: 'Try In Your Own App',
              collapsible: true,
              collapsed: true,
              link: {type: 'doc', id: 'use-cases/b2c/try-in-your-own-app'},
              items: [
                {type: 'doc', id: 'use-cases/b2c/try-in-your-own-app/add-login', label: 'Login', key: 'own-app-login'},
                {type: 'doc', id: 'use-cases/b2c/try-in-your-own-app/self-sign-up', label: 'Self Sign-Up', key: 'own-app-self-sign-up'},
                {type: 'doc', id: 'use-cases/b2c/try-in-your-own-app/profile-section', label: 'View Profile', key: 'own-app-profile-section'},
                {type: 'doc', id: 'use-cases/b2c/try-in-your-own-app/account-recovery', label: 'Account Recovery', key: 'own-app-account-recovery'},
                {
                  type: 'doc',
                  id: 'use-cases/b2c/try-in-your-own-app/onboard-internal-users',
                  label: 'Onboard Internal Users',
                  key: 'own-app-onboard-internal-users',
                },
              ],
            },
          ],
        },
        {
          type: 'category',
          label: 'SaaS Applications (B2B)',
          collapsible: true,
          collapsed: true,
          items: [{type: 'doc', id: 'use-cases/b2b/multi-tenant-saas', label: 'Multi-Tenant SaaS'}],
        },
        {
          type: 'category',
          label: 'Agent Identity',
          collapsible: true,
          collapsed: true,
          items: [
            {
              type: 'category',
              label: 'Securing AI Agents',
              collapsible: true,
              collapsed: true,
              items: [
                {type: 'doc', id: 'use-cases/ai-agents/overview', label: 'Overview', key: 'ai-agents-overview'},
                {type: 'doc', id: 'use-cases/ai-agents/solution-patterns', label: 'Solution Patterns', key: 'ai-agents-solution-patterns'},
                {
                  type: 'category',
                  label: 'Try It Out',
                  collapsible: true,
                  collapsed: true,
                  key: 'ai-agents-try-it-out',
                  link: {type: 'doc', id: 'use-cases/ai-agents/try-it-out'},
                  items: [
                    {
                      type: 'category',
                      label: 'Walkthroughs',
                      collapsible: true,
                      collapsed: true,
                      key: 'ai-agents-walkthroughs',
                      items: [
                        {type: 'doc', id: 'use-cases/ai-agents/try-it-out/protect-the-agent', label: 'Protect the Agent'},
                        {type: 'doc', id: 'use-cases/ai-agents/try-it-out/act-on-its-own', label: 'Acting on Its Own'},
                        {
                          type: 'doc',
                          id: 'use-cases/ai-agents/try-it-out/act-on-behalf-of-user',
                          label: 'Acting on Behalf of a User',
                        },
                      ],
                    },
                    {
                      type: 'category',
                      label: 'Learn More',
                      collapsible: true,
                      collapsed: true,
                      key: 'ai-agents-learn-more',
                      items: [
                        {type: 'doc', id: 'use-cases/ai-agents/identity-concepts', label: 'Identity Concepts', key: 'ai-agents-identity-concepts'},
                        {type: 'doc', id: 'use-cases/ai-agents/configure-it-yourself', label: 'Configure It Yourself', key: 'ai-agents-configure-it-yourself'},
                      ],
                    },
                  ],
                },
              ],
            },
            {
              type: 'category',
              label: 'Securing MCP',
              collapsible: true,
              collapsed: true,
              items: [
                {type: 'doc', id: 'use-cases/ai-agents/mcp-authorization/overview', label: 'Overview', key: 'mcp-authorization-overview'},
                {type: 'doc', id: 'use-cases/ai-agents/mcp-authorization/solution-patterns', label: 'Solution Patterns', key: 'mcp-authorization-solution-patterns'},
                {
                  type: 'category',
                  label: 'Try It Out',
                  collapsible: true,
                  collapsed: true,
                  key: 'mcp-authorization-try-it-out',
                  link: {type: 'doc', id: 'use-cases/ai-agents/mcp-authorization/try-it-out'},
                  items: [
                    {
                      type: 'category',
                      label: 'Walkthroughs',
                      collapsible: true,
                      collapsed: true,
                      key: 'mcp-authorization-walkthroughs',
                      items: [
                        {
                          type: 'doc',
                          id: 'use-cases/ai-agents/mcp-authorization/try-it-out/connect-via-inspector',
                          label: 'MCP Authorization',
                        },
                      ],
                    },
                    {
                      type: 'category',
                      label: 'Learn More',
                      collapsible: true,
                      collapsed: true,
                      key: 'mcp-authorization-learn-more',
                      items: [
                        {type: 'doc', id: 'use-cases/ai-agents/mcp-authorization/identity-concepts', label: 'Identity Concepts', key: 'mcp-authorization-identity-concepts'},
                        {type: 'doc', id: 'use-cases/ai-agents/mcp-authorization/configure-it-yourself', label: 'Configure It Yourself', key: 'mcp-authorization-configure-it-yourself'},
                      ],
                    },
                  ],
                },
              ],
            },
          ],
        },
        {
          type: 'category',
          label: 'Secure Token Service (STS)',
          collapsible: true,
          collapsed: true,
          items: [
            {type: 'doc', id: 'use-cases/sts/krakend', label: 'Protect APIs on KrakenD'},
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
          collapsible: true,
          collapsed: true,
          items: [
            {
              type: 'category',
              label: 'Passwordless',
              collapsible: true,
              collapsed: true,
              items: [
                {
                  type: 'doc',
                  id: 'guides/key-concepts/authentication/passwordless/passkeys',
                  label: 'Passkeys',
                },
                {
                  type: 'doc',
                  id: 'guides/key-concepts/authentication/passwordless/magiclink',
                  label: 'Magic Link',
                },
              ],
            },
            {
              type: 'doc',
              id: 'guides/key-concepts/authentication/integration-models',
              label: 'Integration Models',
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
          id: 'guides/deployment-patterns/index',
          label: 'Choose Your Deployment',
        },
        {
          type: 'category',
          label: 'Deployment Paths',
          collapsible: true,
          collapsed: true,
          items: [
            {
              type: 'category',
              label: 'Docker',
              collapsible: true,
              collapsed: false,
              link: {
                type: 'doc',
                id: 'guides/deployment-patterns/docker',
              },
              items: [
                {
                  type: 'doc',
                  id: 'guides/deployment-patterns/docker-production',
                  label: 'Production Recommendations',
                },
              ],
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
        {
          type: 'doc',
          id: 'guides/getting-started/configuration',
          label: 'Configure your Instance',
        },
        {
          type: 'doc',
          id: 'guides/deployment-patterns/production-guidelines',
          label: 'Production Guidelines',
        },
        {
          type: 'doc',
          id: 'guides/deployment-patterns/observability',
          label: 'Observability',
        },
      ],
    },
  ],
  expressSdkSidebar,
  nuxtSdkSidebar,
  reactSdkSidebar,
  reactRouterSdkSidebar,
  tanstackRouterSdkSidebar,
  nodeSdkSidebar,
  vueSdkSidebar,
  browserSdkSidebar,
  nextjsSdkSidebar,
  javascriptSdkSidebar,
  communitySidebar: [
    // Community Section
    {
      type: 'html',
      value:
        '<div class="sidebar-section-label"><svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg><span>Community</span></div>',
      className: 'sidebar-html-section-header',
    },
    {
      type: 'category',
      label: 'Community',
      className: 'sidebar-section',
      collapsed: false,
      collapsible: false,
      items: [{type: 'doc', id: 'community/overview', label: 'Join the Community', key: 'community-overview'}],
    },

    // Contribute Section
    {
      type: 'html',
      value:
        '<div class="sidebar-section-label"><svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></svg><span>Contribute</span></div>',
      className: 'sidebar-html-section-header',
    },
    {
      type: 'category',
      label: 'Contribute',
      className: 'sidebar-section',
      collapsed: false,
      collapsible: false,
      items: [
        {type: 'doc', id: 'community/contributing/overview', label: `Contribute to ${productConfig.project.name}`},
        {type: 'doc', id: 'community/contributing/sharing-ideas', label: 'Share Ideas'},
        {type: 'doc', id: 'community/contributing/proposing-features', label: 'Propose Features'},
        {type: 'doc', id: 'community/contributing/contributing-code/prerequisites', label: 'Prerequisites'},
        {type: 'doc', id: 'community/contributing/contributing-code/configure-and-run', label: 'Configure and Run'},
        {
          type: 'doc',
          id: 'community/contributing/contributing-code/development-pipeline',
          label: 'Development Pipeline',
          key: 'code-development-pipeline',
        },
        {type: 'doc', id: 'community/contributing/contributing-code/debugging', label: 'Debugging'},
        {
          type: 'category',
          label: 'Backend Development',
          collapsed: true,
          collapsible: true,
          items: [
            {
              type: 'doc',
              id: 'community/contributing/contributing-code/backend-development/overview',
              label: 'Backend Overview',
              key: 'backend-overview',
            },
            {
              type: 'doc',
              id: 'community/contributing/contributing-code/backend-development/observability',
              label: 'Observability',
            },
          ],
        },
        {
          type: 'category',
          label: 'Frontend Development',
          collapsed: true,
          collapsible: true,
          items: [
            {
              type: 'doc',
              id: 'community/contributing/contributing-code/frontend-development/overview',
              label: 'Frontend Overview',
              key: 'frontend-overview',
            },
            {
              type: 'doc',
              id: 'community/contributing/contributing-code/frontend-development/scaffolding-tool',
              label: 'Scaffolding Tool',
            },
          ],
        },
        {
          type: 'category',
          label: 'Documentation Development',
          collapsed: true,
          collapsible: true,
          items: [
            {
              type: 'doc',
              id: 'community/contributing/contributing-code/documentation-development/overview',
              label: 'Documentation Overview',
              key: 'docs-overview',
            },
            {
              type: 'doc',
              id: 'community/contributing/contributing-code/documentation-development/development-pipeline',
              label: 'Documentation Development Pipeline',
              key: 'docs-development-pipeline',
            },
            {
              type: 'doc',
              id: 'community/contributing/contributing-code/documentation-development/writing-documentation',
              label: 'Writing Documentation',
            },
            {
              type: 'doc',
              id: 'community/contributing/contributing-code/documentation-development/documentation-style-guide',
              label: 'Style Guide',
            },
            {
              type: 'doc',
              id: 'community/contributing/contributing-code/documentation-development/advanced-topics',
              label: 'Advanced Topics',
            },
            {
              type: 'doc',
              id: 'community/contributing/contributing-code/documentation-development/glossary',
              label: 'Glossary',
            },
          ],
        },
      ],
    },

    // Maintenance Section
    {
      type: 'html',
      value:
        '<div class="sidebar-section-label"><svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="3"/><path d="M19.07 4.93a10 10 0 0 1 0 14.14M4.93 4.93a10 10 0 0 0 0 14.14"/></svg><span>Maintenance</span></div>',
      className: 'sidebar-html-section-header',
    },
    {
      type: 'category',
      label: 'Maintenance',
      className: 'sidebar-section',
      collapsed: false,
      collapsible: false,
      items: [{type: 'doc', id: 'community/release-operations', label: 'Release Operations'}],
    },
  ],
};

export default sidebars;
