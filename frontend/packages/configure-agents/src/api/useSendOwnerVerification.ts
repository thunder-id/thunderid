// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, type UseMutationResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {
  AdministrationFlowData,
  AdministrationFlowInput,
  FlowExecutionFailure,
  FlowStatus,
  startAdminInitiatedFlow,
  type FlowExecutionResponse,
  type HttpLike,
} from '@thunderid/utils';

export interface SendOwnerVerificationVariables {
  agentId: string;
  /** Handle of the flow to start, from the server's `adminAgentOwnerVerificationFlow` setting. */
  handle: string;
}

/**
 * Starts the agent owner verification flow for an agent. The flow sends the agent's owner a link; the
 * owner is verified once they open it and confirm, so nothing about the agent changes here. The caller shows
 * where the link went, or the failure, where the administrator sent it from.
 */
export default function useSendOwnerVerification(): UseMutationResult<
  FlowExecutionResponse,
  Error,
  SendOwnerVerificationVariables
> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useMutation<FlowExecutionResponse, Error, SendOwnerVerificationVariables>({
    // Not executeAdministrationFlow: it treats INCOMPLETE as a failure, but this flow pauses once the link is sent.
    mutationFn: async ({agentId, handle}: SendOwnerVerificationVariables): Promise<FlowExecutionResponse> => {
      const result = await startAdminInitiatedFlow(http as unknown as HttpLike, getServerUrl(), handle, {
        [AdministrationFlowInput.SUBJECT]: agentId,
      });
      const data = result.data?.additionalData ?? {};

      // The flow pauses once the link is sent, naming the channel it went by.
      if (result.flowStatus === FlowStatus.INCOMPLETE && data[AdministrationFlowData.CHANNEL]) {
        if (data[AdministrationFlowData.EMAIL_SENT] !== 'true' && data[AdministrationFlowData.SMS_SENT] !== 'true') {
          throw new FlowExecutionFailure({}, 'The verification link was not sent.');
        }

        return result;
      }

      throw new FlowExecutionFailure(result.error ?? {}, 'The verification link could not be sent.');
    },
  });
}
