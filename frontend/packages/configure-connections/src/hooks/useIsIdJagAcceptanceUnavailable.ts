// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import useAuthorizationServerMetadata from '../api/useAuthorizationServerMetadata';
import ConnectionConstants from '../constants/connection-constants';

/**
 * Whether the runtime is known not to accept identity assertions (ID-JAG), read from its
 * authorization server metadata: the runtime lists the ID-JAG grant profile in
 * `authorization_grant_profiles_supported` only while it accepts them, and leaves the field out
 * when it lists no profile.
 *
 * Returns false while the metadata is loading, after it fails to load, and when it carries no
 * `grant_types_supported` (which the runtime always sends), so nothing is blocked on missing
 * information.
 *
 * @returns True when the metadata has loaded and does not list the ID-JAG grant profile
 *
 * @example
 * ```tsx
 * const isAcceptanceUnavailable = useIsIdJagAcceptanceUnavailable();
 * <Switch disabled={isAcceptanceUnavailable && !idJagEnabled} />
 * ```
 *
 * @public
 */
export default function useIsIdJagAcceptanceUnavailable(): boolean {
  const {data: serverMetadata} = useAuthorizationServerMetadata();

  if (!Array.isArray(serverMetadata?.grant_types_supported)) return false;

  return !(serverMetadata.authorization_grant_profiles_supported ?? []).includes(
    ConnectionConstants.ID_JAG_GRANT_PROFILE,
  );
}
