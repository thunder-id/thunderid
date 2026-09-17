// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebar: SidebarsConfig = {
  nextjsSdkSidebar: [
    {
      type: 'doc',
      id: 'sdks-and-tools/nextjs/overview',
    },
    {
      type: 'category',
      label: 'APIs',
      collapsed: false,
      items: [
        {
          type: 'doc',
          id: 'sdks-and-tools/nextjs/apis/thunderid-provider',
          label: '<ThunderIDProvider />',
        },
        {
          type: 'doc',
          id: 'sdks-and-tools/nextjs/apis/middleware',
          label: 'Middleware',
        },
        {
          type: 'doc',
          id: 'sdks-and-tools/nextjs/apis/server-actions',
          label: 'Server Actions',
        },
        {
          type: 'doc',
          id: 'sdks-and-tools/nextjs/apis/configuration',
          label: 'Configuration',
        },
        {
              type: 'category',
              label: 'Components',
              collapsed: true,
              items: [
                {
                  type: 'category',
                  label: 'User Self-care Components',
                  collapsed: false,
                  items: [
                    {
                      type: 'doc',
                      id: 'sdks-and-tools/nextjs/apis/components/change-credential',
                      label: '<ChangeCredential />',
                    },
                  ],
                },
              ],
        },
      ],
    },
  ],
};

export default sidebar.nextjsSdkSidebar;
