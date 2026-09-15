// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {ApplicationQueryKeys} from '@thunderid/configure-applications';
import type {Application, InboundAuthConfig} from '@thunderid/configure-applications';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import {regenerateClientSecretViaFlow, type HttpLike} from '../utils/applicationAdministrationFlow';

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
   * The updated application after client secret regeneration.
   *
   * Only present on the native fallback path, which rotates the secret by updating the application.
   * The regeneration flow rotates the credential without returning the application.
   */
  application?: Application;
  /**
   * The new client secret generated during regeneration
   * This is only available immediately after regeneration and should be saved by the user
   */
  clientSecret: string;
}

/**
 * Generates a cryptographically secure OAuth 2 client secret.
 *
 * @remarks
 * Matches the backend implementation in `GenerateOAuth2ClientSecret()`:
 * - 32 random bytes (256 bits of entropy) via the Web Crypto API
 * - Encoded as base64url (URL-safe, no padding) using the same scheme as
 *   Go's `base64.RawURLEncoding`
 *
 * The Web Crypto API (`crypto.getRandomValues`) is a CSPRNG available in all
 * modern browsers and Node.js ≥ 15, making it the correct choice for
 * security-sensitive values such as OAuth client secrets.
 *
 * @returns A base64url-encoded 32-byte (256-bit) client secret string
 */
function generateClientSecret(): string {
  const bytes = new Uint8Array(32);
  crypto.getRandomValues(bytes);

  // Convert to base64url without padding, matching Go's base64.RawURLEncoding
  return btoa(String.fromCharCode(...bytes))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=/g, '');
}

/**
 * Custom React hook to regenerate an application's client secret.
 *
 * The regeneration runs through the configured administration flow, which revokes the artifacts
 * issued under the old secret before rotating it and returns the new secret the server generated.
 *
 * When no such flow is configured it falls back to rotating the secret through the update API:
 * 1. Fetching the current application details
 * 2. Generating a new client secret
 * 3. Updating the application with the new client secret via the update API
 *
 * Upon successful regeneration, the cache is invalidated to ensure the UI
 * reflects the latest changes.
 *
 * @remarks
 * The fallback path generates the secret in the browser because the update endpoint has no way to
 * ask the server for one. The flow path does not, so its secret carries the server's entropy.
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
 *         onSuccess: ({ application, clientSecret }) => {
 *           // Only log the non-sensitive identifier, never log the secret itself
 *           console.log('Client secret regenerated for:', application.id);
 *           // Display the new client secret to the user via a secure UI flow
 *           // e.g.: showSecretModal(application.id, clientSecret);
 *           void clientSecret; // Secret must be handled by the UI, not logged
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

  return useMutation<RegenerateSecretResult, Error, RegenerateSecretVariables>({
    mutationFn: async ({applicationId}: RegenerateSecretVariables): Promise<RegenerateSecretResult> => {
      const serverUrl: string = getServerUrl();

      const flowSecret: string | null = await regenerateClientSecretViaFlow(
        http as unknown as HttpLike,
        serverUrl,
        applicationId,
      );

      if (flowSecret) {
        return {clientSecret: flowSecret};
      }

      // Step 1: Fetch the current application details
      const getResponse: {data: Application} = await http.request({
        url: `${serverUrl}/applications/${applicationId}`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      const currentApplication = getResponse.data;

      // Step 2: Generate a new client secret
      const newClientSecret = generateClientSecret();

      // Step 3: Prepare the update request with the new client secret
      // Destructure to remove server-generated fields
      // eslint-disable-next-line @typescript-eslint/no-unused-vars
      const {id, createdAt, updatedAt, inboundAuthConfig, ...applicationUpdate}: Application = currentApplication;
      const updateRequest = {...applicationUpdate, inboundAuthConfig};

      // Update the OAuth2 config with the new client secret
      const oauth2Config: InboundAuthConfig | undefined = inboundAuthConfig?.find(
        (config: InboundAuthConfig) => config.type === 'oauth2',
      );

      if (!oauth2Config) {
        throw new Error('Application does not have an OAuth2 configuration. Cannot regenerate client secret.');
      }

      oauth2Config.config.clientSecret = newClientSecret;

      // Step 4: Update the application with the new client secret
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
        clientSecret: newClientSecret,
      };
    },
    onSuccess: (_data, variables) => {
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
      showToast(t('regenerateSecret.snackbar.success'), 'success');
    },
  });
}
