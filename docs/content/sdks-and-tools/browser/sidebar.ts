// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebar: SidebarsConfig = {
  browserSdkSidebar: [
    {
      type: 'doc',
      id: 'sdks-and-tools/browser/overview',
    },
    {
      type: 'category',
      label: 'APIs',
      collapsed: false,
      className: 'sidebar-section-icon-apis',
      items: [
        {
          type: 'doc',
          id: 'sdks-and-tools/browser/apis/thunderid-browser-client',
          label: 'ThunderIDBrowserClient',
        },
        {
          type: 'doc',
          id: 'sdks-and-tools/browser/apis/configuration',
          label: 'Configuration',
        },
        {
          type: 'doc',
          id: 'sdks-and-tools/browser/apis/hooks',
          label: 'Event Hooks',
        },
        {
          type: 'category',
          label: 'Utilities',
          collapsed: true,
          items: [
            {
              type: 'doc',
              id: 'sdks-and-tools/browser/apis/utilities/update-me-credentials',
              label: 'updateMeCredentials()',
            },
            {
              type: 'doc',
              id: 'sdks-and-tools/browser/apis/utilities/evaluate-password-policy',
              label: 'evaluatePasswordPolicy()',
            },
            {
              type: 'doc',
              id: 'sdks-and-tools/browser/apis/utilities/evaluate-change-password-form',
              label: 'evaluateChangePasswordForm()',
            },
            {
              type: 'doc',
              id: 'sdks-and-tools/browser/apis/utilities/resolve-change-credential-policy',
              label: 'resolveChangeCredentialPolicy()',
            },
            {
              type: 'doc',
              id: 'sdks-and-tools/browser/apis/utilities/supports-credential',
              label: 'supportsCredential()',
            },
            {
              type: 'doc',
              id: 'sdks-and-tools/browser/apis/utilities/map-credential-update-error',
              label: 'mapCredentialUpdateError()',
            },
            {
              type: 'doc',
              id: 'sdks-and-tools/browser/apis/utilities/credential-constants',
              label: 'CredentialConstants',
            },
            {
              type: 'doc',
              id: 'sdks-and-tools/browser/apis/utilities/create-http-client-fetcher',
              label: 'createHttpClientFetcher()',
            },
          ],
        },
      ],
    },
  ],
};

export default sidebar.browserSdkSidebar;
