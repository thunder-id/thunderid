// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Every value an entry's `icon:` field may take.
 *
 * This list is deliberately free of JSX so it can be imported from both sides:
 * `iconRegistry.ts` maps each name to a real component (and fails to compile if
 * one is missing), while `plugins/ecosystemPlugin` imports it in Node to reject
 * an unknown `icon:` at build time.
 *
 * Most names resolve to a component in `src/components/icons/`; `AndroidLogo`
 * and `FlutterLogo` come from `@thunderid/components`.
 */
export const ECOSYSTEM_ICON_NAMES = [
  'AndroidLogo',
  'AngularLogo',
  'AuthJsLogo',
  'BetterAuthLogo',
  'BrowserLogo',
  'ClaudeLogo',
  'CodexLogo',
  'ExpressLogo',
  'FlutterLogo',
  'GoLogo',
  'IOSLogo',
  'JavaScriptLogo',
  'NextLogo',
  'NodeLogo',
  'NuxtAuthLogo',
  'NuxtLogo',
  'PassportLogo',
  'PythonLogo',
  'ReactLogo',
  'ReactRouterLogo',
  'SkillsLogo',
  'SpringSecurityLogo',
  'TanStackLogo',
  'VueLogo',
] as const;

export type EcosystemIconName = (typeof ECOSYSTEM_ICON_NAMES)[number];
