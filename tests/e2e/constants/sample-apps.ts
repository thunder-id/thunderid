// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Sample-App Client IDs
 *
 * The React SDK sample app boots as whichever application `public/runtime.json` names
 * (samples/apps/react-sdk-sample/src/config.tsx). Only one real `REACT_SDK_SAMPLE` app is
 * provisioned for it, but three test suites each need it bound to a different authentication
 * flow (default password login, MFA, social login) - rewiring the same app from three suites at
 * once is what they cannot do.
 *
 * `E2E_SAMPLE_MFA` and `E2E_SAMPLE_SOCIAL` are declaratively provisioned, sample-app-shaped
 * clones of `REACT_SDK_SAMPLE` (see tests/e2e/thunderid-config-sample-apps.yaml), each dedicated to one
 * suite. The `sampleAppClientId` fixture option (fixtures/sample-app/sample-app.fixture.ts)
 * intercepts the runtime.json fetch to point the sample app at one of these instead, so that
 * suite's rewiring never touches the shared default app. This is what lets MFA and social login
 * run alongside sample-app-login.spec.ts, which still fans out across all three browser projects
 * against `REACT_SDK_SAMPLE`, and alongside each other.
 *
 * Social login keeps a single dedicated app shared by both vendors: Google and GitHub live in one
 * spec file, so they rewire it one after another in the same worker anyway, and the real
 * cross-vendor contention is the server-wide `identity_provider.<vendor>_base_url` config and the
 * mocks' fixed ports, not the application.
 */
export const SampleAppClientIds = {
  /** The real sample-app application; left at its default (password login) flow bindings. */
  DEFAULT: "REACT_SDK_SAMPLE",

  /** Dedicated app for sample-app-mfa-login.spec.ts. */
  MFA: "E2E_SAMPLE_MFA",

  /** Dedicated app for sample-app-social-login.spec.ts (both vendors). */
  SOCIAL: "E2E_SAMPLE_SOCIAL",
} as const;
