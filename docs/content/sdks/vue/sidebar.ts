// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebar: SidebarsConfig = {
  vueSdkSidebar: [
    {
      type: 'doc',
      id: 'sdks/vue/overview',
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
              id: 'sdks/vue/apis/providers/thunderid-provider',
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
                  id: 'sdks/vue/apis/components/sign-in-button',
                  label: '<SignInButton />',
                },
                {
                  type: 'doc',
                  id: 'sdks/vue/apis/components/sign-out-button',
                  label: '<SignOutButton />',
                },
                {
                  type: 'doc',
                  id: 'sdks/vue/apis/components/sign-up-button',
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
                  id: 'sdks/vue/apis/components/signed-in',
                  label: '<SignedIn />',
                },
                {
                  type: 'doc',
                  id: 'sdks/vue/apis/components/signed-out',
                  label: '<SignedOut />',
                },
                {
                  type: 'doc',
                  id: 'sdks/vue/apis/components/loading',
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
                  id: 'sdks/vue/apis/components/user-dropdown',
                  label: '<UserDropdown />',
                },
                {
                  type: 'doc',
                  id: 'sdks/vue/apis/components/user-profile',
                  label: '<UserProfile />',
                },
                {
                  type: 'doc',
                  id: 'sdks/vue/apis/components/user',
                  label: '<User />',
                },
                {
                  type: 'doc',
                  id: 'sdks/vue/apis/components/change-credential',
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
              id: 'sdks/vue/apis/composables/use-thunderid',
              label: 'useThunderID()',
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
          type: 'category',
          label: 'Protecting Routes',
          collapsed: true,
          items: [
            {
              type: 'doc',
              id: 'sdks/vue/guides/protecting-routes/overview',
            },
            {
              type: 'doc',
              id: 'sdks/vue/guides/protecting-routes/vue-router',
              label: 'Vue Router',
            },
            {
              type: 'doc',
              id: 'sdks/vue/guides/protecting-routes/custom',
              label: 'Custom Implementation',
            },
          ],
        },
        {
          type: 'doc',
          id: 'sdks/vue/guides/accessing-protected-apis',
          label: 'Accessing Protected APIs',
        },
      ],
    },
  ],
};

export default sidebar.vueSdkSidebar;
