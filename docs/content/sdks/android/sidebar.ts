// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebar: SidebarsConfig = {
  androidSdkSidebar: [
    {
      type: 'doc',
      id: 'sdks/android/overview',
    },
    {
      type: 'category',
      label: 'APIs',
      collapsed: false,
      className: 'sidebar-section-icon-apis',
      items: [
        {
          type: 'doc',
          id: 'sdks/android/apis/thunderid-client',
          label: 'ThunderIDClient',
        },
        {
          type: 'doc',
          id: 'sdks/android/apis/configuration',
          label: 'Configuration',
        },
        {
          type: 'doc',
          id: 'sdks/android/apis/thunderid-state',
          label: 'ThunderIDState',
        },
        {
          type: 'category',
          label: 'Components',
          collapsed: false,
          items: [
            {
              type: 'doc',
              id: 'sdks/android/apis/components/sign-in',
              label: 'SignIn',
            },
            {
              type: 'doc',
              id: 'sdks/android/apis/components/sign-up',
              label: 'SignUp',
            },
            {
              type: 'doc',
              id: 'sdks/android/apis/components/sign-in-button',
              label: 'SignInButton',
            },
            {
              type: 'doc',
              id: 'sdks/android/apis/components/sign-out-button',
              label: 'SignOutButton',
            },
            {
              type: 'doc',
              id: 'sdks/android/apis/components/signed-in',
              label: 'SignedIn',
            },
            {
              type: 'doc',
              id: 'sdks/android/apis/components/signed-out',
              label: 'SignedOut',
            },
            {
              type: 'doc',
              id: 'sdks/android/apis/components/user-profile',
              label: 'UserProfile',
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
          id: 'sdks/android/guides/try-the-sample-app',
          label: 'Try the Sample App',
        },
        {
          type: 'doc',
          id: 'sdks/android/guides/accessing-protected-apis',
          label: 'Accessing Protected APIs',
        },
      ],
    },
  ],
};

export default sidebar.androidSdkSidebar;
