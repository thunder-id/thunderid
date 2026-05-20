/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {
  ThunderIDRuntimeError,
  Organization,
  AllOrganizationsApiResponse,
  CreateOrganizationPayload,
  TokenResponse,
} from '@thunderid/browser';
import {FC, PropsWithChildren, ReactElement, useCallback, useMemo, useState} from 'react';
import OrganizationContext, {OrganizationContextProps} from './OrganizationContext';

/**
 * Props interface of {@link OrganizationProvider}
 */
export interface OrganizationProviderProps {
  /**
   * Whether to automatically fetch organizations on mount
   */
  autoFetch?: boolean;
  /**
   * Function to create a new organization.
   */
  createOrganization?: (payload: CreateOrganizationPayload, sessionId: string) => Promise<Organization>;
  /**
   * Initial current organization
   */
  currentOrganization?: Organization | null;
  /**
   * Initial list of organizations
   */
  getAllOrganizations?: () => Promise<AllOrganizationsApiResponse>;
  /**
   * List of organizations the signed-in user belongs to.
   */
  myOrganizations?: Organization[];
  /**
   * Callback function called when an error occurs
   */
  onError?: (error: string) => void;
  /**
   * Callback function called when switching organizations
   */
  onOrganizationSwitch?: (organization: Organization) => Promise<TokenResponse | Response>;
  /**
   * Refetch the my organizations list.
   * @returns
   */
  revalidateMyOrganizations: () => Promise<Organization[]>;
}

/**
 * OrganizationProvider component that manages organization data and provides it through OrganizationContext.
 *
 * This provider:
 * - Fetches organization data from the organizations endpoint
 * - Manages current organization state
 * - Provides functions for switching organizations and refreshing data
 * - Handles loading states and errors
 *
 * @example
 * ```tsx
 * // Basic usage with auto-fetch (uses internal API)
 * <OrganizationProvider>
 *   <App />
 * </OrganizationProvider>
 *
 * // With custom error handling
 * <OrganizationProvider onError={(error) => console.error('Organization error:', error)}>
 *   <App />
 * </OrganizationProvider>
 *
 * // With custom organization switch handler
 * <OrganizationProvider
 *   onOrganizationSwitch={(org) => console.log('Switched to:', org.name)}
 * >
 *   <App />
 * </OrganizationProvider>
 *
 * // Disable auto-fetch (fetch manually using revalidateMyOrganizations)
 * <OrganizationProvider autoFetch={false}>
 *   <App />
 * </OrganizationProvider>
 * ```
 */
const OrganizationProvider: FC<PropsWithChildren<OrganizationProviderProps>> = ({
  children,
  currentOrganization,
  onError,
  myOrganizations,
  onOrganizationSwitch,
  revalidateMyOrganizations,
  getAllOrganizations,
  createOrganization,
}: PropsWithChildren<OrganizationProviderProps>): ReactElement => {
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  /**
   * Switches to a different organization
   */
  const switchOrganization: (organization: Organization) => Promise<void> = useCallback(
    async (organization: Organization): Promise<void> => {
      if (!onOrganizationSwitch) {
        throw new ThunderIDRuntimeError(
          'onOrganizationSwitch callback is required',
          'OrganizationProvider-SwitchError-001',
          'react',
          'The onOrganizationSwitch callback must be provided to handle organization switching.',
        );
      }

      setIsLoading(true);
      setError(null);

      try {
        await onOrganizationSwitch(organization);
        // The organization state will be updated by the parent provider
      } catch (switchError) {
        const errorMessage: string =
          switchError instanceof Error ? switchError.message : 'Failed to switch organization';
        setError(errorMessage);
        if (onError) {
          onError(errorMessage);
        }
        throw switchError; // Re-throw so the UI can handle it
      } finally {
        setIsLoading(false);
      }
    },
    [onOrganizationSwitch, onError],
  );

  const contextValue: OrganizationContextProps = useMemo(
    () => ({
      createOrganization,
      currentOrganization,
      error,
      getAllOrganizations,
      isLoading,
      myOrganizations,
      revalidateMyOrganizations,
      switchOrganization,
    }),
    [
      currentOrganization,
      error,
      isLoading,
      myOrganizations,
      switchOrganization,
      revalidateMyOrganizations,
      getAllOrganizations,
      createOrganization,
    ],
  );

  return <OrganizationContext.Provider value={contextValue}>{children}</OrganizationContext.Provider>;
};

export default OrganizationProvider;
