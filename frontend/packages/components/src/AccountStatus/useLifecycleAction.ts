// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type QueryKey, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {
  AdministrationFlowData,
  AdministrationFlowInput,
  executeAdministrationFlow,
  type HttpLike,
} from '@thunderid/utils';
import type {TFunction} from 'i18next';
import {useTranslation} from 'react-i18next';
import {LifecycleActionIncompleteError, type LifecycleAction, type LifecycleCategory} from './lifecycleAction';

export interface LifecycleActionVariables {
  /**
   * Entity id the operation acts on.
   */
  subject: string;
  /**
   * Which action to run.
   */
  action: LifecycleAction;
  /**
   * Identity category of the subject, which decides the toast copy.
   */
  category: LifecycleCategory;
  /**
   * Administration flow ids from useAvailableLifecycleActions, one per step of the action, in order.
   */
  flowIds: string[];
  /**
   * Operator's note, for the operations that record one. Optional.
   */
  operatorNote?: string;
}

/**
 * The success toast. Only the combinations offered are distinguished: applications are never locked, and
 * only users are offered Unsuspend and unlock.
 */
function successMessage(action: LifecycleAction, category: LifecycleCategory, t: TFunction): string {
  switch (action) {
    case 'unlock':
      return category === 'agent'
        ? t('common:accountStatus.unlock.success.agent', 'Agent unlocked')
        : t('common:accountStatus.unlock.success.user', 'User unlocked');
    case 'unsuspend':
      if (category === 'agent') {
        return t('common:accountStatus.unsuspend.success.agent', 'Agent unsuspended');
      }
      if (category === 'application') {
        return t('common:accountStatus.unsuspend.success.application', 'Application unsuspended');
      }
      return t('common:accountStatus.unsuspend.success.user', 'User unsuspended');
    case 'unsuspendAndUnlock':
      return t('common:accountStatus.unsuspendAndUnlock.success', 'User unsuspended and unlocked');
    default:
      if (category === 'agent') {
        return t('common:accountStatus.suspend.success.agent', 'Agent suspended');
      }
      if (category === 'application') {
        return t('common:accountStatus.suspend.success.application', 'Application suspended');
      }
      return t('common:accountStatus.suspend.success.user', 'User suspended');
  }
}

/**
 * The warning toast when a user's notification email was not sent. Agents and applications usually hold
 * no email address, so they get the success toast instead.
 */
function emailNotSentMessage(action: LifecycleAction, t: TFunction): string {
  switch (action) {
    case 'unlock':
      return t(
        'common:accountStatus.unlock.emailNotSent.user',
        'User unlocked, but no notification email was sent. Check that the user has an email address and an email provider is configured.',
      );
    case 'unsuspend':
      return t(
        'common:accountStatus.unsuspend.emailNotSent.user',
        'User unsuspended, but no notification email was sent. Check that the user has an email address and an email provider is configured.',
      );
    case 'unsuspendAndUnlock':
      return t(
        'common:accountStatus.unsuspendAndUnlock.emailNotSent',
        'User unsuspended and unlocked, but a notification email was not sent. Check that the user has an email address and an email provider is configured.',
      );
    default:
      return t(
        'common:accountStatus.suspend.emailNotSent.user',
        'User suspended, but no notification email was sent. Check that the user has an email address and an email provider is configured.',
      );
  }
}

/**
 * Runs one lifecycle action through its administration flows, one step at a time. A step that fails
 * after an earlier one succeeded throws {@link LifecycleActionIncompleteError}, since the record changed.
 * Success toasts here; failures are left to the caller.
 *
 * @param listQueryKey - The caller's list query, invalidated after a successful run so its status updates.
 * @param onSuccess - Called after a successful run, so the caller can refetch the record.
 * @returns Mutation resolving to whether any step's notification email was not sent.
 */
export default function useLifecycleAction(
  listQueryKey: QueryKey,
  onSuccess?: (variables: LifecycleActionVariables) => void,
): UseMutationResult<boolean, Error, LifecycleActionVariables> {
  const queryClient = useQueryClient();
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {t} = useTranslation('common');
  const {showToast} = useToast();

  const invalidateList = (): void => {
    queryClient.invalidateQueries({queryKey: listQueryKey}).catch(() => {
      // Ignore invalidation errors
    });
  };

  return useMutation<boolean, Error, LifecycleActionVariables>({
    mutationFn: async ({action, flowIds, operatorNote, subject}: LifecycleActionVariables): Promise<boolean> => {
      const inputs: Record<string, string> = {[AdministrationFlowInput.SUBJECT]: subject};

      if (operatorNote) {
        inputs[AdministrationFlowInput.OPERATOR_NOTE] = operatorNote;
      }
      let emailNotSent = false;
      for (const [index, flowId] of flowIds.entries()) {
        let additionalData: Record<string, string>;
        try {
          additionalData = await executeAdministrationFlow(
            http as unknown as HttpLike,
            getServerUrl(),
            flowId,
            inputs,
            action,
          );
        } catch (error) {
          throw index > 0 ? new LifecycleActionIncompleteError(error) : error;
        }
        // "false" from a best-effort email node that sent nothing; absent when the flow has none.
        if (additionalData[AdministrationFlowData.EMAIL_SENT] === 'false') {
          emailNotSent = true;
        }
      }

      return emailNotSent;
    },
    // An incomplete run changed the record too.
    onError: (error: Error): void => {
      if (error instanceof LifecycleActionIncompleteError) {
        invalidateList();
      }
    },
    onSuccess: (emailNotSent: boolean, variables: LifecycleActionVariables): void => {
      invalidateList();
      if (emailNotSent && variables.category === 'user') {
        showToast(emailNotSentMessage(variables.action, t), 'warning');
      } else {
        showToast(successMessage(variables.action, variables.category, t), 'success');
      }
      onSuccess?.(variables);
    },
  });
}
