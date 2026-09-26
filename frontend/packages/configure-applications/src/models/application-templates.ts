// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {JSX} from 'react';
import type {ApplicationCreateFlowSignInApproach} from './application-create-flow';
import type {CreationFlow} from './creation-flow';
import type {ApplicationType, InboundAuthConfig, OAuth2Config} from '@thunderid/configure-applications';

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
  IOS: 'IOS',
  ANDROID: 'ANDROID',
  FLUTTER: 'FLUTTER',
  OTHER: 'OTHER',
  MCP_CLIENT: 'MCP_CLIENT',
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
  WALLET: 'WALLET',
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
   * Hosted prompt document for this guide. Either a literal URL, or a
   * `{{documentation.links key}}` reference (e.g.
   * '{{applications.templates.react.llmPrompt.redirectBased}}') resolved against the configured
   * docs site, same convention as {@link QuickstartLink.docsUrl}. The referenced document is
   * fetched at render time and may contain `{{productName}}`, `{{clientId}}`, and
   * `{{applicationId}}` placeholders.
   */
  docsUrl?: string;
}

/**
 * Integration guides structure containing LLM prompt and manual steps.
 * Keys represent different integration approaches (e.g., 'redirect_based', 'embedded').
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
   * Canonical backend application type this template maps to. Sent as the application `type` on
   * creation. Templates that can resolve to more than one type (e.g. MCP client) may override this
   * at creation time based on user selections.
   */
  type?: ApplicationType;
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
    /**
     * Sign-in approach preselected for this template in the creation wizard's Configure step.
     * Falls back to the wizard's own default (hosted pages) when omitted.
     */
    signInApproach?: ApplicationCreateFlowSignInApproach;
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
   * Optional per-template capability flags that gate optional configuration surfaces in the edit UI.
   */
  capabilities?: {
    /**
     * Whether this template supports platform attestation (e.g. Google Play Integrity). When true,
     * the attestation configuration section is shown in the application's advanced settings.
     */
    attestation?: boolean;
    /**
     * Whether this template's Configuration step should offer a CORS Allowed Origins editor.
     * Applies to public-client templates that call the token/userinfo endpoints directly from a
     * browser origin; server-side templates exchange tokens off-browser and don't need it.
     */
    cors?: boolean;
  };
  /**
   * Which device frame the application-creation wizard's live sign-in preview should render at.
   * Defaults to `'desktop'` when omitted. Set to `'mobile'` for templates whose sign-in UI is
   * actually rendered on a phone screen (e.g. the native Android/iOS/Flutter templates and the
   * generic Mobile platform template) — everything else, including other public-client templates
   * like digital wallets, previews at desktop size.
   */
  previewDevice?: 'desktop' | 'mobile';
  /**
   * Optional metadata describing the template's local development server, used to offer a
   * "quick add" shortcut on the Configuration step for its conventional default URL.
   */
  devServer?: {
    /** Identifier used to pick a logo for the quick-add banner (e.g. 'vite', 'nextjs', 'nuxt'). */
    id: string;
    /** Display name of the dev server/tool (e.g. 'Vite', 'Next.js', 'Nuxt'). */
    label: string;
    /** The dev server's conventional default URL (e.g. 'http://localhost:5173'). */
    url: string;
  };
  /**
   * Optional integration guides for this template
   */
  integrationGuides?: IntegrationGuides;
  /**
   * Optional quickstart guides for this template, shown on the application's Overview tab as
   * "read the docs" cards. Most templates have exactly one; templates that cover more than one
   * platform SDK (e.g. the generic Mobile template covering iOS, Android, and Flutter) list one
   * entry per platform.
   */
  quickstarts?: QuickstartLink[];
  /**
   * Optional runnable playgrounds for this template, shown as a banner on the application's
   * Overview tab when there's exactly one. Distinct from {@link quickstarts}: a quickstart is a
   * hosted docs link, a playground is a live, runnable environment (e.g. StackBlitz). Not every
   * template has a runnable playground (e.g. native mobile platforms don't).
   */
  playgrounds?: PlaygroundLink[];
}

/**
 * A single quickstart guide entry for a template.
 *
 * @public
 */
export interface QuickstartLink {
  /** Short label identifying the guide, e.g. 'React', 'iOS', 'Android', 'Flutter'. */
  label: string;
  /**
   * Hosted documentation guide for connecting an application built with this template/platform.
   * Either a literal URL, or a `{{documentation.links key}}` reference (e.g.
   * '{{applications.templates.react.docs}}') resolved against the configured docs site, letting
   * the docs site stay the source of truth.
   */
  docsUrl: string;
}

/**
 * A single runnable playground entry for a template.
 *
 * @public
 */
export interface PlaygroundLink {
  /** Short label identifying the guide, e.g. 'React', 'Next.js', 'Nuxt'. */
  label: string;
  /**
   * The runnable environment this playground is hosted on, e.g. `'stackblitz'`. Only
   * `'stackblitz'` is currently rendered; other values are reserved for future environments the
   * console doesn't support yet, and are silently not rendered.
   */
  environment: string;
  /**
   * URL of the runnable sample. Either a literal URL, or a `{{documentation.links key}}`
   * reference (e.g. '{{applications.templates.react.playground}}'), same resolution rules as
   * {@link QuickstartLink.docsUrl}.
   */
  url: string;
}

/**
 * Template category used for filtering in the unified template gallery.
 *
 * @public
 */
export type TemplateCategory = 'web' | 'backend' | 'mobile' | 'ai';

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
