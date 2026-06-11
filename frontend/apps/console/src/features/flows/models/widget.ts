/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import type {Base} from './base';

export type Widget = Base<WidgetExtendedConfig>;

/**
 * Interface for the properties of a widget.
 */
export interface WidgetExtendedConfig {
  /**
   * Version of the widget.
   */
  version?: string;
  data: unknown;
}

export const WidgetCategories = {
  Composite: 'COMPOSITE',
  Flow: 'FLOW',
  Security: 'SECURITY',
} as const;

export type WidgetCategories = (typeof WidgetCategories)[keyof typeof WidgetCategories];

export const WidgetTypes = {
  IdentifierPassword: 'IDENTIFIER_PASSWORD',
  SMSOTP: 'SMS_OTP',
  GoogleFederation: 'GOOGLE_FEDERATION',
  GithubFederation: 'GITHUB_FEDERATION',
  EUDIWallet: 'EUDI_WALLET',
  PasskeyAuthentication: 'PASSKEY_AUTHENTICATION',
  Provisioning: 'PROVISIONING',
  SelfSignUpLink: 'SELF_SIGN_UP_LINK',
} as const;

export type WidgetTypes = (typeof WidgetTypes)[keyof typeof WidgetTypes];
