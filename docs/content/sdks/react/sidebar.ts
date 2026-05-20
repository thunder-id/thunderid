/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebar: SidebarsConfig = {
  reactSdkSidebar: [
    {
      type: 'doc',
      id: 'sdks/react/overview',
    },
    {
      type: 'category',
      label: 'APIs',
      collapsed: false,
      items: [
        {
          type: 'category',
          label: 'Contexts',
          collapsed: true,
          items: [
            {
              type: 'doc',
              id: 'sdks/react/apis/contexts/thunderid-provider',
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
                  id: 'sdks/react/apis/components/sign-in-button',
                  label: '<SignInButton />',
                },
                {
                  type: 'doc',
                  id: 'sdks/react/apis/components/sign-out-button',
                  label: '<SignOutButton />',
                },
                {
                  type: 'doc',
                  id: 'sdks/react/apis/components/sign-up-button',
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
                  id: 'sdks/react/apis/components/signed-in',
                  label: '<SignedIn />',
                },
                {
                  type: 'doc',
                  id: 'sdks/react/apis/components/signed-out',
                  label: '<SignedOut />',
                },
                {
                  type: 'doc',
                  id: 'sdks/react/apis/components/loading',
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
                  id: 'sdks/react/apis/components/user-dropdown',
                  label: '<UserDropdown />',
                },
                {
                  type: 'doc',
                  id: 'sdks/react/apis/components/user-profile',
                  label: '<UserProfile />',
                },
                {
                  type: 'doc',
                  id: 'sdks/react/apis/components/user',
                  label: '<User />',
                },
              ],
            },
            {
              type: 'category',
              label: 'Organization Components (B2B)',
              collapsed: true,
              items: [
                {
                  type: 'doc',
                  id: 'sdks/react/apis/components/create-organization',
                  label: '<CreateOrganization />',
                },
                {
                  type: 'doc',
                  id: 'sdks/react/apis/components/organization-profile',
                  label: '<OrganizationProfile />',
                },
                {
                  type: 'doc',
                  id: 'sdks/react/apis/components/organization-switcher',
                  label: '<OrganizationSwitcher />',
                },
                {
                  type: 'doc',
                  id: 'sdks/react/apis/components/organization-list',
                  label: '<OrganizationList />',
                },
                {
                  type: 'doc',
                  id: 'sdks/react/apis/components/organization',
                  label: '<Organization />',
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
              id: 'sdks/react/apis/hooks/use-thunderid',
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
      items: [
        {
          type: 'category',
          label: 'Protecting Routes',
          collapsed: true,
          items: [
            {
              type: 'doc',
              id: 'sdks/react/guides/protecting-routes/overview',
            },
            {
              type: 'doc',
              id: 'sdks/react/guides/protecting-routes/react-router',
              label: 'React Router',
            },
            {
              type: 'doc',
              id: 'sdks/react/guides/protecting-routes/tanstack-router',
              label: 'TanStack Router',
            },
            {
              type: 'doc',
              id: 'sdks/react/guides/protecting-routes/custom',
              label: 'Custom Implementation',
            },
          ],
        },
        {
          type: 'doc',
          id: 'sdks/react/guides/accessing-protected-apis',
          label: 'Accessing Protected APIs',
        },
      ],
    },
  ],
};

export default sidebar.reactSdkSidebar;
