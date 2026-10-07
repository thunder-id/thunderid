// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ServerConfigLayers} from '../models/application-administration-flow';

/**
 * Minimal shape of the HTTP client this module needs, so it can be exercised without a React tree.
 */
export interface HttpLike {
  request: (config: unknown) => Promise<{data?: unknown}>;
}

/**
 * The deployment's graceful refresh token rotation policy, as served by the `refreshToken` block of
 * the `oauth` server-config section.
 */
export interface RotationGracePolicy {
  /**
   * Whether graceful rotation is available at all. When false, no application receives a grace
   * window whatever it has configured, so the field is not worth offering.
   */
  enabled: boolean;
  /**
   * The longest window any application may use, in seconds. An application may configure a shorter
   * window, never a longer one.
   */
  ceilingSeconds: number;
}

/**
 * The policy assumed when it cannot be read: the feature is unavailable.
 *
 * A read failure hides the field rather than offering one whose value the server would ignore. The
 * section is admin scoped, so a console user without that scope legitimately cannot read it.
 */
export const ROTATION_GRACE_UNAVAILABLE: RotationGracePolicy = {
  enabled: false,
  ceilingSeconds: 0,
};

/**
 * Shape of the `oauth` section value, which the API serves in layered form. Only the block this
 * module reads is declared.
 */
interface OAuthSection {
  refreshToken?: {
    graceEnabled?: boolean;
    graceCeilingSeconds?: number;
  };
}

/**
 * Reads the deployment's rotation grace policy.
 *
 * Only the merged layer is the effective configuration, so reading the envelope directly would
 * always miss the value. A failure resolves to {@link ROTATION_GRACE_UNAVAILABLE} rather than
 * propagating: the console uses this to decide whether to offer a field, and a token settings page
 * should still render when an unrelated section cannot be read.
 */
export async function fetchRotationGracePolicy(http: HttpLike, serverUrl: string): Promise<RotationGracePolicy> {
  try {
    const response = await http.request({
      url: `${serverUrl}/server-config/oauth`,
      method: 'GET',
    });
    const layers = (response?.data ?? {}) as ServerConfigLayers<OAuthSection>;
    const merged = layers.merged?.refreshToken ?? {};
    const ceilingSeconds = merged.graceCeilingSeconds ?? 0;

    // The server reports no window when the feature is off, whatever ceiling is stored, so an
    // enabled flag with no ceiling is treated the same way here.
    if (merged.graceEnabled !== true || ceilingSeconds <= 0) {
      return ROTATION_GRACE_UNAVAILABLE;
    }

    return {enabled: true, ceilingSeconds};
  } catch {
    return ROTATION_GRACE_UNAVAILABLE;
  }
}
