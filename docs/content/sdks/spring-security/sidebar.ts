// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebar: SidebarsConfig = {
  springSecurityIntegrationSidebar: [
    {
      type: 'doc',
      id: 'sdks/spring-security/overview',
    },
    {
      type: 'category',
      label: 'Guides',
      collapsed: false,
      items: [
        {
          type: 'doc',
          id: 'sdks/spring-security/guides/handling-authentication',
          label: 'Handling Authentication',
        }
        // {
        //   type: 'doc',
        //   id: 'sdks/spring-security/guides/protecting-routes',
        //   label: 'Protecting Routes',
        // },
        // {
        //   type: 'doc',
        //   id: 'sdks/spring-security/guides/accessing-protected-apis',
        //   label: 'Accessing Protected APIs',
        // },
      ],
    },
  ],
};

export default sidebar.springSecurityIntegrationSidebar;
