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
  flutterSdkSidebar: [
    {
      type: 'doc',
      id: 'sdks/flutter/overview',
    },
    {
      type: 'category',
      label: 'APIs',
      collapsed: false,
      className: 'sidebar-section-icon-apis',
      items: [
        {
          type: 'doc',
          id: 'sdks/flutter/apis/thunder-client',
          label: 'ThunderIDClient',
        },
        {
          type: 'doc',
          id: 'sdks/flutter/apis/configuration',
          label: 'Configuration',
        },
        {
          type: 'doc',
          id: 'sdks/flutter/apis/thunder-state',
          label: 'ThunderIDState',
        },
        {
          type: 'category',
          label: 'Widgets',
          collapsed: false,
          items: [
            {
              type: 'doc',
              id: 'sdks/flutter/apis/components/sign-in',
              label: 'SignIn',
            },
            {
              type: 'doc',
              id: 'sdks/flutter/apis/components/sign-up',
              label: 'SignUp',
            },
            {
              type: 'doc',
              id: 'sdks/flutter/apis/components/sign-in-button',
              label: 'SignInButton',
            },
            {
              type: 'doc',
              id: 'sdks/flutter/apis/components/sign-out-button',
              label: 'SignOutButton',
            },
            {
              type: 'doc',
              id: 'sdks/flutter/apis/components/signed-in',
              label: 'ThunderIDSignedIn',
            },
            {
              type: 'doc',
              id: 'sdks/flutter/apis/components/signed-out',
              label: 'ThunderIDSignedOut',
            },
            {
              type: 'doc',
              id: 'sdks/flutter/apis/components/user-profile',
              label: 'ThunderIDUserProfile',
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
          id: 'sdks/flutter/guides/accessing-protected-apis',
          label: 'Accessing Protected APIs',
        },
      ],
    },
  ],
};

export default sidebar.flutterSdkSidebar;
