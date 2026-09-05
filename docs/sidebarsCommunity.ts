// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

// This runs in Node.js - Don't use client-side code here (browser APIs, JSX...)

/**
 * Sidebar for the community docs plugin instance.
 *
 * Community content is intentionally unversioned: it documents how to contribute to
 * the project as it is today, not how a released version behaves. It therefore lives
 * in its own docs plugin instance (see `docusaurus.config.ts`) and is served from
 * `/community/` instead of a versioned `/docs/<version>/` path.
 */
const sidebars: SidebarsConfig = {
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
      items: [
        {type: 'doc', id: 'overview', label: 'Join the Community', key: 'community-overview'},
        {type: 'doc', id: 'contributors', label: 'Contributors'},
        {type: 'doc', id: 'code-of-conduct', label: 'Code of Conduct'},
      ],
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
        {type: 'doc', id: 'contributing/report-a-bug', label: 'Report a Bug'},
        {type: 'doc', id: 'contributing/propose-a-feature', label: 'Propose a Feature'},
        {type: 'doc', id: 'contributing/propose-a-design', label: 'Propose a Design'},
        {
          type: 'category',
          label: 'Contribute Code',
          collapsed: false,
          collapsible: true,
          items: [
            {type: 'doc', id: 'contributing/contributing-code/prerequisites', label: 'Prerequisites'},
            {type: 'doc', id: 'contributing/contributing-code/configure-and-run', label: 'Configure and Run'},
            {
              type: 'category',
              label: 'Optional Setup',
              key: 'code-optional-setup',
              collapsed: true,
              collapsible: true,
              items: [
                {type: 'doc', id: 'contributing/contributing-code/optional-setup/build-the-project', label: 'Build the Project'},
                {type: 'doc', id: 'contributing/contributing-code/optional-setup/useful-commands', label: 'Useful Commands'},
                {type: 'doc', id: 'contributing/contributing-code/optional-setup/setup-and-data-seeding', label: 'Setup and Data Seeding'},
                {type: 'doc', id: 'contributing/contributing-code/optional-setup/advanced-setup', label: 'Advanced Setup (Manual Mode)'},
              ],
            },
            {type: 'doc', id: 'contributing/contributing-code/debugging', label: 'Debugging'},
            {
              type: 'category',
              label: 'Backend Development',
              collapsed: true,
              collapsible: true,
              items: [
                {
                  type: 'doc',
                  id: 'contributing/contributing-code/backend-development/overview',
                  label: 'Overview',
                  key: 'backend-overview',
                },
                {
                  type: 'doc',
                  id: 'contributing/contributing-code/backend-development/observability',
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
                  id: 'contributing/contributing-code/frontend-development/overview',
                  label: 'Overview',
                  key: 'frontend-overview',
                },
                {
                  type: 'doc',
                  id: 'contributing/contributing-code/frontend-development/conventions',
                  label: 'Conventions',
                  key: 'frontend-conventions',
                },
                {
                  type: 'doc',
                  id: 'contributing/contributing-code/frontend-development/best-practices',
                  label: 'Best Practices',
                  key: 'frontend-best-practices',
                },
              ],
            },
            {
              type: 'category',
              label: 'SDK Development',
              collapsed: true,
              collapsible: true,
              items: [
                {
                  type: 'doc',
                  id: 'contributing/contributing-code/sdk-development/overview',
                  label: 'Overview',
                  key: 'sdk-overview',
                },
              ],
            },
            {
              type: 'category',
              label: 'Documentation Development',
              collapsed: true,
              collapsible: true,
              items: [
                {type: 'doc', id: 'contributing/documentation-guide/overview', label: 'Overview'},
                {
                  type: 'doc',
                  id: 'contributing/documentation-guide/configure-and-run',
                  label: 'Configure & Run',
                },
                {
                  type: 'category',
                  label: 'Optional Setup',
                  key: 'docs-optional-setup',
                  collapsed: true,
                  collapsible: true,
                  items: [
                    {
                      type: 'doc',
                      id: 'contributing/documentation-guide/build-the-documentation',
                      label: 'Build the Documentation',
                    },
                    {
                      type: 'doc',
                      id: 'contributing/documentation-guide/useful-commands',
                      label: 'Useful Commands',
                      key: 'docs-useful-commands',
                    },
                  ],
                },
                {
                  type: 'doc',
                  id: 'contributing/documentation-guide/style-guide',
                  label: 'Style Guide',
                },
                {
                  type: 'doc',
                  id: 'contributing/documentation-guide/writing-guide',
                  label: 'Writing Guide',
                },
                {
                  type: 'doc',
                  id: 'contributing/documentation-guide/advanced-topics',
                  label: 'Advanced Topics',
                },
              ],
            },
            {
              type: 'doc',
              id: 'contributing/contributing-code/pull-request-workflow',
              label: 'Pull Request Workflow',
              key: 'code-development-pipeline',
            },
          ],
        },
        {
          type: 'doc',
          id: 'contributing/documentation-guide/glossary',
          label: 'Glossary',
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
      items: [{type: 'doc', id: 'release-operations', label: 'Release Operations'}],
    },
  ],
};

export default sidebars;
