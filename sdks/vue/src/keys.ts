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

import type {InjectionKey} from 'vue';
import type {
  ThunderIDContext,
  BrandingContextValue,
  FlowContextValue,
  FlowMetaContextValue,
  I18nContextValue,
  OrganizationContextValue,
  ThemeContextValue,
  UserContextValue,
} from './models/contexts';

/**
 * Injection key for the core ThunderID authentication context.
 */
export const THUNDERID_KEY: InjectionKey<ThunderIDContext> = Symbol('thunderid');

/**
 * Injection key for the User context (profile, schemas, update operations).
 */
export const USER_KEY: InjectionKey<UserContextValue> = Symbol('thunderid-user');

/**
 * Injection key for the Organization context (list, current org, switching).
 */
export const ORGANIZATION_KEY: InjectionKey<OrganizationContextValue> = Symbol('thunderid-organization');

/**
 * Injection key for the Flow context (embedded flow UI state).
 */
export const FLOW_KEY: InjectionKey<FlowContextValue> = Symbol('thunderid-flow');

/**
 * Injection key for the FlowMeta context (server-driven flow metadata).
 */
export const FLOW_META_KEY: InjectionKey<FlowMetaContextValue> = Symbol('thunderid-flow-meta');

/**
 * Injection key for the Theme context (color scheme, CSS variables, toggle).
 */
export const THEME_KEY: InjectionKey<ThemeContextValue> = Symbol('thunderid-theme');

/**
 * Injection key for the Branding context (branding preferences from server).
 */
export const BRANDING_KEY: InjectionKey<BrandingContextValue> = Symbol('thunderid-branding');

/**
 * Injection key for the I18n context (translation function, language switching).
 */
export const I18N_KEY: InjectionKey<I18nContextValue> = Symbol('thunderid-i18n');
