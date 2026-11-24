// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {
  AdministrationFlowData,
  AdministrationFlowInput,
  FlowExecutionFailure,
  FlowStatus,
  startAdminInitiatedFlow,
  type HttpLike,
} from '@thunderid/utils';
import {useTranslation} from 'react-i18next';

/**
 * Variables for the send credential reset link mutation.
 */
export interface SendUserCredentialResetVariables {
  userId: string;
  /**
   * Name of the credential attribute to reset, such as `password`.
   */
  credential: string;
  /**
   * Display name of the credential, for the success toast.
   */
  label: string;
  /**
   * Handle of the flow to start, from the server's `adminUserCredentialResetFlow` setting.
   */
  handle: string;
}

/**
 * Asks a user to set a new credential by starting the administrator-initiated credential reset flow
 * the server configures. The flow emails the user a link, removes the current credential, and pauses;
 * the user finishes it from the link. Failures are left to the caller to show.
 *
 * @returns TanStack Query mutation object for sending a credential reset link
 */
export default function useSendUserCredentialReset(): UseMutationResult<void, Error, SendUserCredentialResetVariables> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {t} = useTranslation('users');
  const {showToast} = useToast();

  return useMutation<void, Error, SendUserCredentialResetVariables>({
    mutationFn: async ({userId, credential, handle}): Promise<void> => {
      const result = await startAdminInitiatedFlow(http as unknown as HttpLike, getServerUrl(), handle, {
        [AdministrationFlowInput.SUBJECT]: userId,
        [AdministrationFlowInput.CREDENTIAL]: credential,
      });

      // Only a paused flow that reports a sent email has removed the credential and sent the link.
      if (
        result.flowStatus === FlowStatus.INCOMPLETE &&
        result.data?.additionalData?.[AdministrationFlowData.EMAIL_SENT] === 'true'
      ) {
        return;
      }

      throw new FlowExecutionFailure(result.error ?? {}, 'The credential reset flow did not report a sent link');
    },
    onSuccess: (_data, {label}) => {
      showToast(
        t(
          'credentialResetLink.sent',
          'Reset link sent. {{credential}} no longer works until the user sets a new one.',
          {
            credential: label,
          },
        ),
        'success',
      );
    },
  });
}
