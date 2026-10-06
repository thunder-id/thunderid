// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {AdministrationFlowInput, executeAdministrationFlow, type HttpLike} from '@thunderid/utils';
import {useTranslation} from 'react-i18next';
import {LifecycleActionIncompleteError, type LifecycleAction} from './lifecycleAction';

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
   * Administration flow ids from useLifecycleActions, one per step of the action, in order.
   */
  flowIds: string[];
  /**
   * Operator's note, for the operations that record one. Optional at every layer, so an absent
   * value suspends with no note recorded rather than failing.
   */
  operatorNote?: string;
}

const SUCCESS_KEYS: Record<LifecycleAction, {key: string; fallback: string}> = {
  suspend: {fallback: 'Account suspended', key: 'common:accountStatus.suspend.success'},
  unlock: {fallback: 'Account unlocked', key: 'common:accountStatus.unlock.success'},
  // Deliberately not "Account unsuspended and restored". For a user the release lands on a lock the
  // refetched status will show, and a toast promising more than happened is how the second step
  // gets skipped.
  unsuspend: {fallback: 'Suspension removed', key: 'common:accountStatus.unsuspend.success'},
  unsuspendAndUnlock: {
    fallback: 'Suspension removed and account unlocked',
    key: 'common:accountStatus.unsuspendAndUnlock.success',
  },
};

/**
 * Runs one lifecycle action through its administration flows, one step at a time.
 *
 * A step that fails after an earlier one succeeded throws {@link LifecycleActionIncompleteError},
 * so the caller knows the record changed and has to be refetched.
 *
 * Success toasts here, following every other mutation hook in the tree. Failure does not: the caller
 * decides, because every operation but unlock is fired from a dialog that has somewhere to put the
 * message, and unlock is fired straight from a button that does not.
 *
 * @param onSettled - Invalidation the caller wants after a successful run, since only the caller
 *   knows which query keys hold the record.
 * @returns TanStack Query mutation for the lifecycle operations.
 */
export default function useLifecycleAction(
  onSettled?: (variables: LifecycleActionVariables) => void,
): UseMutationResult<void, Error, LifecycleActionVariables> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {t} = useTranslation('common');
  const {showToast} = useToast();

  return useMutation<void, Error, LifecycleActionVariables>({
    mutationFn: async ({action, flowIds, operatorNote, subject}: LifecycleActionVariables): Promise<void> => {
      // Omit an empty note so the server stores none.
      const inputs: Record<string, string> = {[AdministrationFlowInput.SUBJECT]: subject};

      if (operatorNote) {
        inputs[AdministrationFlowInput.OPERATOR_NOTE] = operatorNote;
      }
      for (const [index, flowId] of flowIds.entries()) {
        try {
          await executeAdministrationFlow(http as unknown as HttpLike, getServerUrl(), flowId, inputs, action);
        } catch (error) {
          throw index > 0 ? new LifecycleActionIncompleteError(error) : error;
        }
      }
    },
    onSuccess: (_data: void, variables: LifecycleActionVariables): void => {
      const {fallback, key} = SUCCESS_KEYS[variables.action];

      showToast(t(key, fallback), 'success');
      onSettled?.(variables);
    },
  });
}
