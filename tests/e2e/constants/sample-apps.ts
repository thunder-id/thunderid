// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Sample-App Client IDs
 *
 * The React SDK sample app boots as whichever application `public/runtime.json` names
 * (samples/apps/react-sdk-sample/src/config.tsx). Only one real `REACT_SDK_SAMPLE` app is
 * provisioned for it, but the MFA suite needs it bound to a different authentication flow than
 * the default password login - rewiring the same app from both suites at once is what they
 * cannot do.
 *
 * `E2E_SAMPLE_MFA` is a declaratively provisioned, sample-app-shaped clone of `REACT_SDK_SAMPLE`
 * (see tests/e2e/thunderid-config-sample-apps.yaml), dedicated to that suite. The
 * `sampleAppClientId` fixture option (fixtures/sample-app/sample-app.fixture.ts) intercepts the
 * runtime.json fetch to point the sample app at it instead, so the MFA suite's rewiring never
 * touches the shared default app. This is what lets MFA run alongside sample-app-login.spec.ts,
 * which still fans out across all three browser projects against `REACT_SDK_SAMPLE`.
 */
export const SampleAppClientIds = {
  /** The real sample-app application; left at its default (password login) flow bindings. */
  DEFAULT: "REACT_SDK_SAMPLE",

  /** Dedicated app for sample-app-mfa-login.spec.ts. */
  MFA: "E2E_SAMPLE_MFA",
} as const;
