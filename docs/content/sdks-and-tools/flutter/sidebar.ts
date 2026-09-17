// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebar: SidebarsConfig = {
  flutterSdkSidebar: [
    {
      type: 'doc',
      id: 'sdks-and-tools/flutter/overview',
    },
    {
      type: 'category',
      label: 'APIs',
      collapsed: false,
      className: 'sidebar-section-icon-apis',
      items: [
        {
          type: 'doc',
          id: 'sdks-and-tools/flutter/apis/thunder-client',
          label: 'ThunderIDClient',
        },
        {
          type: 'doc',
          id: 'sdks-and-tools/flutter/apis/configuration',
          label: 'Configuration',
        },
        {
          type: 'doc',
          id: 'sdks-and-tools/flutter/apis/thunder-state',
          label: 'ThunderIDState',
        },
        {
          type: 'category',
          label: 'Widgets',
          collapsed: false,
          items: [
            {
              type: 'doc',
              id: 'sdks-and-tools/flutter/apis/components/sign-in',
              label: 'SignIn',
            },
            {
              type: 'doc',
              id: 'sdks-and-tools/flutter/apis/components/sign-up',
              label: 'SignUp',
            },
            {
              type: 'doc',
              id: 'sdks-and-tools/flutter/apis/components/sign-in-button',
              label: 'SignInButton',
            },
            {
              type: 'doc',
              id: 'sdks-and-tools/flutter/apis/components/sign-out-button',
              label: 'SignOutButton',
            },
            {
              type: 'doc',
              id: 'sdks-and-tools/flutter/apis/components/signed-in',
              label: 'SignedIn',
            },
            {
              type: 'doc',
              id: 'sdks-and-tools/flutter/apis/components/signed-out',
              label: 'SignedOut',
            },
            {
              type: 'doc',
              id: 'sdks-and-tools/flutter/apis/components/user-profile',
              label: 'UserProfile',
            },
          ],
        },
      ],
    },
  ],
};

export default sidebar.flutterSdkSidebar;
