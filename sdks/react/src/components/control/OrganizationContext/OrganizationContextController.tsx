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

import {Organization} from '@thunderid/browser';
import {FC, useEffect, useRef} from 'react';
import useThunderID from '../../../contexts/ThunderID/useThunderID';

interface OrganizationContextControllerProps {
  /**
   * Children to render
   */
  children: React.ReactNode;
  /**
   * Whether the source provider is signed in
   */
  isSourceSignedIn: boolean;
  /**
   * ID of the organization to authenticate with
   */
  targetOrganizationId: string;
}

const OrganizationContextController: FC<OrganizationContextControllerProps> = ({
  targetOrganizationId,
  isSourceSignedIn,
  children,
}: OrganizationContextControllerProps) => {
  const {isInitialized, isSignedIn, switchOrganization, isLoading} = useThunderID();
  const hasAuthenticatedRef: React.MutableRefObject<boolean> = useRef(false);
  const isAuthenticatingRef: React.MutableRefObject<boolean> = useRef(false);

  /**
   * Handle the organization switch when:
   * - Current instance is initialized and NOT signed in
   * - Source provider IS signed in
   * Uses the `switchOrganization` function from the ThunderID context.
   */
  useEffect(() => {
    const performOrganizationSwitch = async (): Promise<void> => {
      // Prevent multiple authentication attempts
      if (hasAuthenticatedRef.current || isAuthenticatingRef.current) {
        return;
      }

      // Wait for initialization to complete
      if (!isInitialized || isLoading) {
        return;
      }

      // Only proceed if user is not already signed in to this instance
      if (isSignedIn) {
        hasAuthenticatedRef.current = true;
        return;
      }

      // CRITICAL: Only proceed if source provider is signed in
      if (!isSourceSignedIn) {
        return;
      }

      try {
        isAuthenticatingRef.current = true;
        hasAuthenticatedRef.current = true;

        // Build the organization object for authentication
        const targetOrganization: Organization = {
          id: targetOrganizationId,
          name: '', // Name will be populated after authentication
          orgHandle: '', // Will be populated after authentication
        };

        // Call the switchOrganization API from context (handles token exchange)
        await switchOrganization(targetOrganization);
      } catch (error) {
        // eslint-disable-next-line no-console
        console.error('Linked organization authentication failed:', error);

        // Reset the flag to allow retry
        hasAuthenticatedRef.current = false;
      } finally {
        isAuthenticatingRef.current = false;
      }
    };

    performOrganizationSwitch();
  }, [isInitialized, isSignedIn, isLoading, isSourceSignedIn, targetOrganizationId, switchOrganization]);

  return <>{children}</>;
};

export default OrganizationContextController;
