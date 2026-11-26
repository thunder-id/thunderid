// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AdministrationModes, useAdministrationOperation, useCapabilitiesContext} from '@thunderid/contexts';
import type {AccessStatus} from '@thunderid/types';
import {AdministrationFlowConfigKey} from '@thunderid/utils';
import {useTranslation} from 'react-i18next';
import useGetAdministrationFlowHandle from '../api/useGetAdministrationFlowHandle';

/**
 * Whether the Console offers to send a user a credential reset link, and why it is held for them.
 */
export interface CredentialResetLink {
  /**
   * Handle of the flow to start, or an empty string when the action is not offered.
   */
  handle: string;
  /**
   * Why the user cannot be sent a link, or an empty string when they can.
   */
  heldReason: string;
}

/**
 * Offers the credential reset link only where administration runs through flows, to operators who can
 * update users and read the server configuration, and when the server configures the flow. A suspended
 * or expired user is refused by the flow, so the caller disables the action with the held reason. A
 * locked user can be sent a link.
 *
 * @param accessStatus - The user's effective access status
 * @returns The configured handle and the held reason for the user
 */
export default function useCredentialResetLink(accessStatus?: AccessStatus): CredentialResetLink {
  const {t} = useTranslation('users');
  const {can} = useCapabilitiesContext();
  const mode = useAdministrationOperation('users', 'resetCredential');
  // The handle comes from the server configuration, so an operator who cannot read it is not offered the
  // action, even when another session left the handle in the query cache.
  const offered = mode === AdministrationModes.FLOW && can('user', 'update') && can('serverconfig', 'view');
  const {data: handle} = useGetAdministrationFlowHandle(
    AdministrationFlowConfigKey.ADMIN_USER_CREDENTIAL_RESET,
    offered,
  );

  let heldReason = '';
  if (accessStatus === 'SUSPENDED') {
    heldReason = t('users:credentialResetLink.heldSuspended', 'Unsuspend the user to send a reset link.');
  } else if (accessStatus === 'EXPIRED') {
    heldReason = t(
      'users:credentialResetLink.heldExpired',
      "This user's account has expired, so a reset link cannot be sent.",
    );
  }

  return {handle: offered && handle ? handle : '', heldReason};
}
