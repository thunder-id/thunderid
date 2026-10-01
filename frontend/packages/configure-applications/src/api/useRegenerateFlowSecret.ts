// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import ApplicationQueryKeys from '../constants/application-query-keys';
import type {Application} from '../models/application';

/**
 * Variables for the {@link useRegenerateFlowSecret} mutation.
 *
 * @public
 */
export interface RegenerateFlowSecretVariables {
  /**
   * The unique identifier of the application whose Flow Secret will be regenerated
   */
  applicationId: string;
}

/**
 * Result of the {@link useRegenerateFlowSecret} mutation.
 *
 * @public
 */
export interface RegenerateFlowSecretResult {
  /**
   * The updated application after Flow Secret regeneration
   */
  application: Application;
  /**
   * The new Flow Secret generated during regeneration. Only available immediately after
   * regeneration and must be saved by the user.
   */
  flowSecret: string;
}

/**
 * Generates a cryptographically secure Flow Secret.
 *
 * @remarks
 * Matches the backend secret generator (`GenerateOAuth2ClientSecret()`):
 * - 32 random bytes (256 bits of entropy) via the Web Crypto API
 * - Encoded as base64url (URL-safe, no padding), matching Go's `base64.RawURLEncoding`
 *
 * @returns A base64url-encoded 32-byte (256-bit) secret string
 */
function generateFlowSecret(): string {
  const bytes = new Uint8Array(32);
  crypto.getRandomValues(bytes);

  return btoa(String.fromCharCode(...bytes))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=/g, '');
}

/**
 * Custom React hook to regenerate an application's Flow Secret.
 *
 * Unlike the OAuth client secret, the Flow Secret is a top-level application credential and exists
 * even for embedded server-side apps that carry no OAuth configuration. This hook:
 * 1. Fetches the current application details
 * 2. Generates a new Flow Secret
 * 3. Updates the application with the new Flow Secret via the update API
 *
 * @remarks
 * There is no dedicated regenerate endpoint; the update application endpoint is used. When a
 * dedicated endpoint is added in the backend, this hook can switch to it without UI changes.
 *
 * @returns TanStack Query mutation object for regenerating Flow Secrets
 *
 * @public
 */
export default function useRegenerateFlowSecret(): UseMutationResult<
  RegenerateFlowSecretResult,
  Error,
  RegenerateFlowSecretVariables
> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient = useQueryClient();
  const {t} = useTranslation('applications');
  const {showToast} = useToast();

  return useMutation<RegenerateFlowSecretResult, Error, RegenerateFlowSecretVariables>({
    mutationFn: async ({applicationId}: RegenerateFlowSecretVariables): Promise<RegenerateFlowSecretResult> => {
      const serverUrl: string = getServerUrl();

      // Step 1: Fetch the current application details
      const getResponse: {data: Application} = await http.request({
        url: `${serverUrl}/applications/${applicationId}`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      const currentApplication = getResponse.data;

      // Step 2: Generate a new Flow Secret
      const newFlowSecret = generateFlowSecret();

      // Step 3: Prepare the update request with the new Flow Secret at the top level.
      // Destructure to remove server-generated fields.
      // eslint-disable-next-line @typescript-eslint/no-unused-vars
      const {id, createdAt, updatedAt, flowSecret, ...applicationUpdate}: Application = currentApplication;
      const updateRequest = {...applicationUpdate, flowSecret: newFlowSecret};

      // Step 4: Update the application with the new Flow Secret
      const updateResponse: {data: Application} = await http.request({
        url: `${serverUrl}/applications/${applicationId}`,
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        data: updateRequest,
      } as unknown as Parameters<typeof http.request>[0]);

      return {
        application: updateResponse.data,
        flowSecret: newFlowSecret,
      };
    },
    onSuccess: (_data, variables) => {
      queryClient
        .invalidateQueries({queryKey: [ApplicationQueryKeys.APPLICATION, variables.applicationId]})
        .catch(() => {
          // Ignore invalidation errors
        });
      queryClient.invalidateQueries({queryKey: [ApplicationQueryKeys.APPLICATIONS]}).catch(() => {
        // Ignore invalidation errors
      });
      showToast(t('regenerateFlowSecret.snackbar.success'), 'success');
    },
  });
}
