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
import type {FlowType} from './flows';
import type {FlowNode} from './responses';
import type {Step} from './steps';

/**
 * Template placeholder replacer.
 */
export interface TemplateReplacer {
  placeholder: string;
  value: string;
  [key: string]: unknown;
}

/**
 * Generation metadata for template placeholders.
 */
export interface TemplateGenerationMeta {
  /**
   * Replacers for template placeholders.
   */
  replacers?: TemplateReplacer[];
  [key: string]: unknown;
}

/**
 * Template-specific configuration data.
 */
export interface TemplateConfigData {
  /**
   * Steps contained in the template.
   */
  steps: Step[];
  /**
   * Generation metadata for template placeholders.
   */
  __generationMeta__?: TemplateGenerationMeta;
}

/**
 * Template-specific configuration that extends the base config.
 */
export interface TemplateConfig {
  /**
   * Template data containing steps.
   */
  data: TemplateConfigData;
}

export type Template = Base<TemplateConfig>;

export const TemplateCategories = {
  Starter: 'STARTER',
  Password: 'PASSWORD',
  SocialLogin: 'SOCIAL_LOGIN',
  Mfa: 'MFA',
  Passwordless: 'PASSWORDLESS',
} as const;

export type TemplateCategories = (typeof TemplateCategories)[keyof typeof TemplateCategories];

export const TemplateTypes = {
  Blank: 'BLANK',
  Basic: 'BASIC',
  BasicFederated: 'BASIC_FEDERATED',
  GeneratedWithAI: 'GENERATE_WITH_AI',
  PasskeyLogin: 'PASSKEY_LOGIN',
  Default: 'DEFAULT',
  BasicAuth: 'BASIC_AUTH',
  Google: 'GOOGLE',
  Github: 'GITHUB',
  GoogleGithub: 'GOOGLE_GITHUB',
  BasicGoogle: 'BASIC_GOOGLE',
  BasicGithub: 'BASIC_GITHUB',
  BasicGoogleGithub: 'BASIC_GOOGLE_GITHUB',
  BasicGoogleGithubSms: 'BASIC_GOOGLE_GITHUB_SMS',
  SmsOtp: 'SMS_OTP',
  Passkey: 'PASSKEY',
  BasicPasskey: 'BASIC_PASSKEY',
  BasicWithPrompt: 'BASIC_WITH_PROMPT',
  SelfInvite: 'SELF_INVITE',
  MagicLink: 'MAGIC_LINK',
  BasicMagicLink: 'BASIC_MAGIC_LINK',
} as const;

export type TemplateTypes = (typeof TemplateTypes)[keyof typeof TemplateTypes];

/**
 * Config shape for flow-level templates (used in the Create Flow wizard).
 * Holds the full flow graph that will be submitted when creating a new flow.
 */
export interface FlowTemplateConfig {
  name: string;
  handle: string;
  nodes: FlowNode[];
}

/**
 * A flow-level template entry from templates.json.
 * Distinct from the step-level Template used inside the flow builder.
 */
export interface FlowTemplate {
  resourceType: 'TEMPLATE';
  category: string;
  type: string;
  flowType: FlowType;
  display: {
    label: string;
    description?: string;
    image: string;
    showOnResourcePanel: boolean;
  };
  config: FlowTemplateConfig;
}
