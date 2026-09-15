// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebar: SidebarsConfig = {
  browserSdkSidebar: [
    {
      type: 'doc',
      id: 'sdks/browser/overview',
    },
    {
      type: 'category',
      label: 'APIs',
      collapsed: false,
      className: 'sidebar-section-icon-apis',
      items: [
        {
          type: 'doc',
          id: 'sdks/browser/apis/thunderid-browser-client',
          label: 'ThunderIDBrowserClient',
        },
        {
          type: 'doc',
          id: 'sdks/browser/apis/configuration',
          label: 'Configuration',
        },
        {
          type: 'doc',
          id: 'sdks/browser/apis/hooks',
          label: 'Event Hooks',
        },
        {
          type: 'category',
          label: 'Utilities',
          collapsed: true,
          items: [
            {
              type: 'doc',
              id: 'sdks/browser/apis/utilities/update-me-credentials',
              label: 'updateMeCredentials()',
            },
            {
              type: 'doc',
              id: 'sdks/browser/apis/utilities/evaluate-password-policy',
              label: 'evaluatePasswordPolicy()',
            },
            {
              type: 'doc',
              id: 'sdks/browser/apis/utilities/evaluate-change-password-form',
              label: 'evaluateChangePasswordForm()',
            },
            {
              type: 'doc',
              id: 'sdks/browser/apis/utilities/resolve-change-credential-policy',
              label: 'resolveChangeCredentialPolicy()',
            },
            {
              type: 'doc',
              id: 'sdks/browser/apis/utilities/supports-credential',
              label: 'supportsCredential()',
            },
            {
              type: 'doc',
              id: 'sdks/browser/apis/utilities/map-credential-update-error',
              label: 'mapCredentialUpdateError()',
            },
            {
              type: 'doc',
              id: 'sdks/browser/apis/utilities/credential-constants',
              label: 'CredentialConstants',
            },
            {
              type: 'doc',
              id: 'sdks/browser/apis/utilities/create-http-client-fetcher',
              label: 'createHttpClientFetcher()',
            },
          ],
        },
      ],
    },
    {
      type: 'category',
      label: 'Guides',
      collapsed: false,
      className: 'sidebar-section-icon-guides',
      items: [
        {
          type: 'doc',
          id: 'sdks/browser/guides/accessing-protected-apis',
          label: 'Accessing Protected APIs',
        },
      ],
    },
  ],
};

export default sidebar.browserSdkSidebar;
