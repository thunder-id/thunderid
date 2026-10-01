// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The modes an operation that changes state can run in.
 *
 * Use these constants rather than the bare strings, so a console and the packages it mounts agree
 * on one spelling.
 *
 * @public
 * @example
 * ```tsx
 * <AdministrationProvider administration={{mode: AdministrationModes.NATIVE}}>
 * ```
 */
export const AdministrationModes = {
  /**
   * Run the operation through the configured administration flow. The flow handles the runtime
   * side first, for example revoking tokens and ending live sessions, and then changes the record.
   */
  FLOW: 'flow',
  /**
   * Call the management API directly, with no flow involved. Suited to a console whose deployment
   * holds configuration only and has no tokens or sessions to clean up.
   */
  NATIVE: 'native',
} as const;

/**
 * How an operation that changes state is carried out.
 *
 * @public
 */
export type AdministrationMode = (typeof AdministrationModes)[keyof typeof AdministrationModes];
