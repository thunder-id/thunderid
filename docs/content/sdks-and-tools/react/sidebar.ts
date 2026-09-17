// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebar: SidebarsConfig = {
  reactSdkSidebar: [
    {
      type: 'doc',
      id: 'sdks-and-tools/react/overview',
    },
    {
      type: 'category',
      label: 'APIs',
      collapsed: false,
      className: 'sidebar-section-icon-apis',
      items: [
        {
          type: 'category',
          label: 'Contexts',
          collapsed: true,
          items: [
            {
              type: 'doc',
              id: 'sdks-and-tools/react/apis/contexts/thunderid-provider',
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
                  id: 'sdks-and-tools/react/apis/components/sign-in-button',
                  label: '<SignInButton />',
                },
                {
                  type: 'doc',
                  id: 'sdks-and-tools/react/apis/components/sign-out-button',
                  label: '<SignOutButton />',
                },
                {
                  type: 'doc',
                  id: 'sdks-and-tools/react/apis/components/sign-up-button',
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
                  id: 'sdks-and-tools/react/apis/components/signed-in',
                  label: '<SignedIn />',
                },
                {
                  type: 'doc',
                  id: 'sdks-and-tools/react/apis/components/signed-out',
                  label: '<SignedOut />',
                },
                {
                  type: 'doc',
                  id: 'sdks-and-tools/react/apis/components/loading',
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
                  id: 'sdks-and-tools/react/apis/components/user-dropdown',
                  label: '<UserDropdown />',
                },
                {
                  type: 'doc',
                  id: 'sdks-and-tools/react/apis/components/user-profile',
                  label: '<UserProfile />',
                },
                {
                  type: 'doc',
                  id: 'sdks-and-tools/react/apis/components/user',
                  label: '<User />',
                },
                {
                  type: 'doc',
                  id: 'sdks-and-tools/react/apis/components/change-credential',
                  label: '<ChangeCredential />',
                },
              ],
            },
          ],
        },
        {
          type: 'category',
          label: 'Hooks',
          collapsed: true,
          items: [
            {
              type: 'doc',
              id: 'sdks-and-tools/react/apis/hooks/use-thunderid',
              label: 'useThunderID()',
            },
          ],
        },
      ],
    },
  ],
};

export default sidebar.reactSdkSidebar;
