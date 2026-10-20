// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebar: SidebarsConfig = {
  pythonSdkSidebar: [
    {
      type: 'doc',
      id: 'sdks-and-tools/python/overview',
    },
    {
      type: 'category',
      label: 'APIs',
      collapsed: false,
      className: 'sidebar-section-icon-apis',
      items: [
        {
          type: 'doc',
          id: 'sdks-and-tools/python/reference',
          label: 'Client and Configuration',
        },
      ],
    },
  ],
};

export default sidebar.pythonSdkSidebar;
