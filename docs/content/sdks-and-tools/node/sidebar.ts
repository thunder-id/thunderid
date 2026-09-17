// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebar: SidebarsConfig = {
  nodeSdkSidebar: [
    {
      type: 'doc',
      id: 'sdks-and-tools/node/overview',
    },
    {
      type: 'category',
      label: 'APIs',
      collapsed: false,
      items: [
        {
          type: 'category',
          label: 'Clients',
          collapsed: false,
          items: [
            {
              type: 'doc',
              id: 'sdks-and-tools/node/apis/clients/thunderid-node-client',
            },
          ],
        },
        {
          type: 'category',
          label: 'Configuration',
          collapsed: true,
          items: [
            {
              type: 'doc',
              id: 'sdks-and-tools/node/apis/config/thunderid-node-config',
            },
          ],
        },
        {
          type: 'category',
          label: 'Utilities',
          collapsed: true,
          items: [
            {
              type: 'doc',
              id: 'sdks-and-tools/node/apis/utilities/cookie-config',
              label: 'CookieConfig',
            },
            {
              type: 'doc',
              id: 'sdks-and-tools/node/apis/utilities/cookie-options',
              label: 'CookieOptions',
            },
            {
              type: 'doc',
              id: 'sdks-and-tools/node/apis/utilities/generate-session-id',
              label: 'generateSessionId()',
            },
            {
              type: 'doc',
              id: 'sdks-and-tools/node/apis/utilities/get-session-cookie-options',
              label: 'getSessionCookieOptions()',
            },
          ],
        },
      ],
    },
  ],
};

export default sidebar.nodeSdkSidebar;
