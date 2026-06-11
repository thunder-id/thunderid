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

import type {JSX} from 'react';
import type {CreationFlow} from './creation-flow';
import type {InboundAuthConfig} from './inbound-auth';
import type {OAuth2Config} from './oauth';

/**
 * Technology-based application template identifiers.
 * Used for framework-specific application configurations (React, Next.js, etc.).
 *
 * @public
 */
export const TechnologyApplicationTemplate = {
  REACT: 'REACT',
  EXPRESS: 'EXPRESS',
  NEXTJS: 'NEXTJS',
  VANILLA_JS: 'VANILLA_JS',
  VUE: 'VUE',
  NUXT: 'NUXT',
  NODEJS: 'NODEJS',
  OTHER: 'OTHER',
} as const;

/**
 * Platform-based application template identifiers.
 * Used for platform-specific application configurations (Browser, Mobile, etc.).
 *
 * @public
 */
export const PlatformApplicationTemplate = {
  BACKEND: 'BACKEND',
  BROWSER: 'BROWSER',
  MOBILE: 'MOBILE',
  FULL_STACK: 'FULL_STACK',
  CUSTOM: 'CUSTOM',
} as const;

/**
 * Integration guide types.
 * Defines the different types of integration guides available for application templates.
 *
 * @public
 */
export interface IntegrationGuide {
  /**
   * Unique identifier for the guide
   */
  id: string;
  /**
   * Display title of the guide
   */
  title: string;
  /**
   * Brief description of what the guide offers
   */
  description: string;
  /**
   * Type of guide (llm for AI-assisted, manual for step-by-step)
   */
  type: 'llm' | 'manual';
  /**
   * Icon identifier for the guide
   */
  icon: string;
  /**
   * Markdown content for LLM prompts
   */
  content?: string;
}

/**
 * Integration step code block.
 *
 * @public
 */
export interface IntegrationStepCode {
  /**
   * Programming language for syntax highlighting
   */
  language: string;
  /**
   * Optional filename to display
   */
  filename?: string;
  /**
   * Code content
   */
  content: string;
  /**
   * Optional tabs for different package managers
   */
  tabs?: string[];
}

/**
 * Integration step for manual integration guide.
 *
 * @public
 */
export interface IntegrationStep {
  /**
   * Step number
   */
  step: number;
  /**
   * Step title
   */
  title: string;
  /**
   * Main description
   */
  description: string;
  /**
   * Optional sub-description
   */
  subDescription?: string;
  /**
   * Optional bullet points
   */
  bullets?: string[];
  /**
   * Optional code block
   */
  code?: IntegrationStepCode;
}

/**
 * Integration guides structure containing LLM prompt and manual steps.
 * Keys represent different integration approaches (e.g., 'inbuilt', 'embedded').
 *
 * @public
 */
export type IntegrationGuides = Record<
  string,
  {
    /**
     * LLM prompt guide option
     */
    llm_prompt: IntegrationGuide;
    /**
     * Manual step-by-step integration guide
     */
    manual_steps: IntegrationStep[];
  }
>;

export interface ApplicationTemplate {
  /**
   * Unique identifier for the template
   * @example 'react', 'nextjs', 'browser'
   */
  id?: string;
  /**
   * User-friendly display name for the template
   * @example 'React', 'Next.js', 'Browser'
   */
  displayName?: string;
  /**
   * Inline creation flow declaring the wizard step sequence for this template.
   * Templates without a `creationFlow` use the default user-facing flow.
   */
  creationFlow?: CreationFlow;
  /**
   * Description of the template
   */
  description?: string;
  /**
   * Default application values applied when creating from this template
   */
  defaults?: {
    name?: string;
    inboundAuthConfig?: InboundAuthConfig[];
    allowedUserTypes?: string[];
  };
  /**
   * Template-driven field constraints for the edit UI
   */
  fieldConstraints?: {
    oauth2?: {
      [K in keyof OAuth2Config]?: {
        readOnly?: boolean;
        value?: OAuth2Config[K];
      };
    };
  };
  /**
   * Optional integration guides for this template
   */
  integrationGuides?: IntegrationGuides;
}

/**
 * Template category used for filtering in the unified template gallery.
 *
 * @public
 */
export type TemplateCategory = 'web' | 'backend' | 'mobile';

export interface ApplicationTemplateMetadata<T = TechnologyApplicationTemplate | PlatformApplicationTemplate> {
  value: T;
  icon: JSX.Element;
  titleKey: string;
  descriptionKey: string;
  template: ApplicationTemplate;
  categories: TemplateCategory[];
  disabled?: boolean;
}

export type TechnologyApplicationTemplate = keyof typeof TechnologyApplicationTemplate;

export type PlatformApplicationTemplate = keyof typeof PlatformApplicationTemplate;
