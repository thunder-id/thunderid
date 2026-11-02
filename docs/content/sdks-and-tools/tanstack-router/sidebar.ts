// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebar: SidebarsConfig = {
  tanstackRouterSdkSidebar: [
    {
      type: 'doc',
      id: 'sdks-and-tools/tanstack-router/overview',
    },
    {
      type: 'category',
      label: 'APIs',
      collapsed: false,
      items: [
        {
          type: 'doc',
          id: 'sdks-and-tools/tanstack-router/apis/protected-route',
          label: '<ProtectedRoute />',
        },
        {
          type: 'doc',
          id: 'sdks-and-tools/tanstack-router/apis/callback-route',
          label: '<CallbackRoute />',
        },
      ],
    },
  ],
};

export default sidebar.tanstackRouterSdkSidebar;
