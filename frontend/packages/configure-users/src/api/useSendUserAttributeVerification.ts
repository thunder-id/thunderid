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
 * Variables for the send verification link mutation.
 */
export interface SendUserAttributeVerificationVariables {
  userId: string;
  attribute: string;
  /**
   * Handle of the flow to start, from the server's `adminAttributeVerificationFlow` setting.
   */
  handle: string;
}

/**
 * What the flow reports once it has sent the link. It never carries the link or the full address.
 */
export interface SendUserAttributeVerificationResult {
  channel: string;
  maskedDestination?: string;
}

/**
 * Sends a user a link to verify an attribute by starting the administrator-initiated verification
 * flow the server configures. The flow pauses once the link is sent, and the user finishes it from the link.
 * Failures are left to the caller to show.
 *
 * @returns TanStack Query mutation object for sending a verification link
 */
export default function useSendUserAttributeVerification(): UseMutationResult<
  SendUserAttributeVerificationResult,
  Error,
  SendUserAttributeVerificationVariables
> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {t} = useTranslation('users');
  const {showToast} = useToast();

  return useMutation<SendUserAttributeVerificationResult, Error, SendUserAttributeVerificationVariables>({
    // Not executeAdministrationFlow: it treats INCOMPLETE as a failure, but this flow pauses once the link is sent.
    mutationFn: async ({userId, attribute, handle}): Promise<SendUserAttributeVerificationResult> => {
      const result = await startAdminInitiatedFlow(http as unknown as HttpLike, getServerUrl(), handle, {
        [AdministrationFlowInput.SUBJECT]: userId,
        [AdministrationFlowInput.ATTRIBUTE]: attribute,
      });
      const data = result.data?.additionalData ?? {};
      const channel = data[AdministrationFlowData.CHANNEL];

      if (result.flowStatus === FlowStatus.INCOMPLETE && channel) {
        if (data[AdministrationFlowData.EMAIL_SENT] !== 'true') {
          throw new FlowExecutionFailure({}, 'The verification link was not sent');
        }

        return {channel, maskedDestination: data[AdministrationFlowData.MASKED_DESTINATION] || undefined};
      }

      throw new FlowExecutionFailure(result.error ?? {}, 'The verification flow did not report a sent link');
    },
    onSuccess: ({maskedDestination}) => {
      showToast(
        maskedDestination
          ? t('verification.sent', 'Verification link sent to {{destination}}.', {destination: maskedDestination})
          : t('verification.sentGeneric', 'Verification link sent.'),
        'success',
      );
    },
  });
}
