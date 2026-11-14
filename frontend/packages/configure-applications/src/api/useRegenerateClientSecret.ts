// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useAdministrationOperation, useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import ApplicationQueryKeys from '../constants/application-query-keys';
import {regenerateClientSecretNatively, type HttpLike} from '../utils/applicationAdministrationFlow';

/**
 * Variables for the {@link useRegenerateClientSecret} mutation.
 *
 * @public
 */
export interface RegenerateSecretVariables {
  /**
   * The unique identifier of the application whose client secret will be regenerated
   */
  applicationId: string;
}

/**
 * Result of the {@link useRegenerateClientSecret} mutation.
 *
 * @public
 */
export interface RegenerateSecretResult {
  /**
   * The new client secret generated during regeneration
   * This is only available immediately after regeneration and should be saved by the user
   */
  clientSecret: string;
  /**
   * Set when the secret is stored but revoking the tokens issued while it was replaced failed.
   * Regenerating again revokes them.
   */
  revocationIncomplete?: boolean;
}

/**
 * Custom React hook to regenerate an application's client secret.
 *
 * The regeneration calls `POST /applications/{id}/client-secret/regenerate`. The server generates the
 * new secret, revokes the tokens issued under the old one, and returns the new secret once. A console
 * that declares its own operation runs that instead.
 *
 * Upon successful regeneration, the cache is invalidated to ensure the UI
 * reflects the latest changes.
 *
 * @returns TanStack Query mutation object for regenerating client secrets with mutate function, loading state, and error information
 *
 * @example
 * ```tsx
 * function RegenerateButton({ applicationId }: { applicationId: string }) {
 *   const regenerateSecret = useRegenerateClientSecret();
 *
 *   const handleRegenerate = () => {
 *     regenerateSecret.mutate(
 *       { applicationId },
 *       {
 *         onSuccess: ({ clientSecret }) => {
 *           // Display the new client secret to the user via a secure UI flow, never log it
 *           // e.g.: showSecretModal(applicationId, clientSecret);
 *           void clientSecret;
 *         },
 *         onError: (error) => {
 *           console.error('Failed to regenerate client secret:', error);
 *         }
 *       }
 *     );
 *   };
 *
 *   return (
 *     <button onClick={handleRegenerate} disabled={regenerateSecret.isPending}>
 *       {regenerateSecret.isPending ? 'Regenerating...' : 'Regenerate Client Secret'}
 *     </button>
 *   );
 * }
 * ```
 *
 * @public
 */
export default function useRegenerateClientSecret(): UseMutationResult<
  RegenerateSecretResult,
  Error,
  RegenerateSecretVariables
> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient = useQueryClient();
  const {t} = useTranslation('applications');
  const {showToast} = useToast();
  const mode = useAdministrationOperation<[string], RegenerateSecretResult>('applications', 'regenerateSecret');

  return useMutation<RegenerateSecretResult, Error, RegenerateSecretVariables>({
    mutationFn: async ({applicationId}: RegenerateSecretVariables): Promise<RegenerateSecretResult> => {
      if (typeof mode === 'function') {
        return mode(applicationId);
      }

      return regenerateClientSecretNatively(http as unknown as HttpLike, getServerUrl(), applicationId);
    },
    onSuccess: (data, variables) => {
      // Invalidate and refetch the specific application
      queryClient
        .invalidateQueries({queryKey: [ApplicationQueryKeys.APPLICATION, variables.applicationId]})
        .catch(() => {
          // Ignore invalidation errors
        });
      // Invalidate and refetch applications list
      queryClient.invalidateQueries({queryKey: [ApplicationQueryKeys.APPLICATIONS]}).catch(() => {
        // Ignore invalidation errors
      });
      if (data.revocationIncomplete) {
        showToast(
          t(
            'regenerateSecret.snackbar.revocationIncomplete',
            'Client secret regenerated, but tokens issued while it was replaced may still be valid. Regenerate it again to revoke them.',
          ),
          'warning',
        );
      } else {
        showToast(t('regenerateSecret.snackbar.success', 'Client secret regenerated successfully.'), 'success');
      }
    },
  });
}
