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
