// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebar: SidebarsConfig = {
  vueSdkSidebar: [
    {
      type: 'doc',
      id: 'sdks-and-tools/vue/overview',
    },
    {
      type: 'category',
      label: 'APIs',
      collapsed: false,
      className: 'sidebar-section-icon-apis',
      items: [
        {
          type: 'category',
          label: 'Providers',
          collapsed: true,
          items: [
            {
              type: 'doc',
              id: 'sdks-and-tools/vue/apis/providers/thunderid-provider',
              label: '<ThunderIDProvider />',
            },
          ],
        },
        {
          type: 'category',
          label: 'Components',
          collapsed: false,
          items: [
            {
              type: 'category',
              label: 'Action Components',
              collapsed: true,
              items: [
                {
                  type: 'doc',
                  id: 'sdks-and-tools/vue/apis/components/sign-in-button',
                  label: '<SignInButton />',
                },
                {
                  type: 'doc',
                  id: 'sdks-and-tools/vue/apis/components/sign-out-button',
                  label: '<SignOutButton />',
                },
                {
                  type: 'doc',
                  id: 'sdks-and-tools/vue/apis/components/sign-up-button',
                  label: '<SignUpButton />',
                },
              ],
            },
            {
              type: 'category',
              label: 'Control Components',
              collapsed: true,
              items: [
                {
                  type: 'doc',
                  id: 'sdks-and-tools/vue/apis/components/signed-in',
                  label: '<SignedIn />',
                },
                {
                  type: 'doc',
                  id: 'sdks-and-tools/vue/apis/components/signed-out',
                  label: '<SignedOut />',
                },
                {
                  type: 'doc',
                  id: 'sdks-and-tools/vue/apis/components/loading',
                  label: '<Loading />',
                },
              ],
            },
            {
              type: 'category',
              label: 'User Self-care Components',
              collapsed: true,
              items: [
                {
                  type: 'doc',
                  id: 'sdks-and-tools/vue/apis/components/user-dropdown',
                  label: '<UserDropdown />',
                },
                {
                  type: 'doc',
                  id: 'sdks-and-tools/vue/apis/components/user-profile',
                  label: '<UserProfile />',
                },
                {
                  type: 'doc',
                  id: 'sdks-and-tools/vue/apis/components/user',
                  label: '<User />',
                },
                {
                  type: 'doc',
                  id: 'sdks-and-tools/vue/apis/components/change-credential',
                  label: '<ChangeCredential />',
                },
              ],
            },
          ],
        },
        {
          type: 'category',
          label: 'Composables',
          collapsed: true,
          items: [
            {
              type: 'doc',
              id: 'sdks-and-tools/vue/apis/composables/use-thunderid',
              label: 'useThunderID()',
            },
          ],
        },
      ],
    },
  ],
};

export default sidebar.vueSdkSidebar;
