/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {getErrorMessage} from '@thunderid/utils';
import {useTranslation} from 'react-i18next';
import ApplicationQueryKeys from '../constants/application-query-keys';
import type {Application} from '../models/application';

/**
 * Variables for the {@link useRegenerateAppSecret} mutation.
 *
 * @public
 */
export interface RegenerateAppSecretVariables {
  /**
   * The unique identifier of the application whose App Secret will be regenerated
   */
  applicationId: string;
}

/**
 * Result of the {@link useRegenerateAppSecret} mutation.
 *
 * @public
 */
export interface RegenerateAppSecretResult {
  /**
   * The updated application after App Secret regeneration
   */
  application: Application;
  /**
   * The new App Secret generated during regeneration. Only available immediately after
   * regeneration and must be saved by the user.
   */
  appSecret: string;
}

/**
 * Generates a cryptographically secure App Secret.
 *
 * @remarks
 * Matches the backend secret generator (`GenerateOAuth2ClientSecret()`):
 * - 32 random bytes (256 bits of entropy) via the Web Crypto API
 * - Encoded as base64url (URL-safe, no padding), matching Go's `base64.RawURLEncoding`
 *
 * @returns A base64url-encoded 32-byte (256-bit) secret string
 */
function generateAppSecret(): string {
  const bytes = new Uint8Array(32);
  crypto.getRandomValues(bytes);

  return btoa(String.fromCharCode(...bytes))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=/g, '');
}

/**
 * Custom React hook to regenerate an application's App Secret.
 *
 * Unlike the OAuth client secret, the App Secret is a top-level application credential and exists
 * even for embedded server-side apps that carry no OAuth configuration. This hook:
 * 1. Fetches the current application details
 * 2. Generates a new App Secret
 * 3. Updates the application with the new App Secret via the update API
 *
 * @remarks
 * There is no dedicated regenerate endpoint; the update application endpoint is used. When a
 * dedicated endpoint is added in the backend, this hook can switch to it without UI changes.
 *
 * @returns TanStack Query mutation object for regenerating App Secrets
 *
 * @public
 */
export default function useRegenerateAppSecret(): UseMutationResult<
  RegenerateAppSecretResult,
  Error,
  RegenerateAppSecretVariables
> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient = useQueryClient();
  const {t} = useTranslation('applications');
  const {showToast} = useToast();

  return useMutation<RegenerateAppSecretResult, Error, RegenerateAppSecretVariables>({
    mutationFn: async ({applicationId}: RegenerateAppSecretVariables): Promise<RegenerateAppSecretResult> => {
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

      // Step 2: Generate a new App Secret
      const newAppSecret = generateAppSecret();

      // Step 3: Prepare the update request with the new App Secret at the top level.
      // Destructure to remove server-generated fields.
      // eslint-disable-next-line @typescript-eslint/no-unused-vars
      const {id, createdAt, updatedAt, appSecret, ...applicationUpdate}: Application = currentApplication;
      const updateRequest = {...applicationUpdate, appSecret: newAppSecret};

      // Step 4: Update the application with the new App Secret
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
        appSecret: newAppSecret,
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
      showToast(t('regenerateAppSecret.snackbar.success'), 'success');
    },
    onError: (error) => {
      showToast(getErrorMessage(error, t, 'regenerateAppSecret.dialog.error'), 'error');
    },
  });
}
