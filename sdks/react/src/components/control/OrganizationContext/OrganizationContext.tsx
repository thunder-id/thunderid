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

import {FC, PropsWithChildren} from 'react';
import OrganizationContextController from './OrganizationContextController';
import ThunderIDProvider, {ThunderIDProviderProps} from '../../../contexts/ThunderID/ThunderIDProvider';
import useThunderID from '../../../contexts/ThunderID/useThunderID';

export interface OrganizationContextProps extends Omit<ThunderIDProviderProps, 'organizationChain' | 'baseUrl'> {
  /**
   * Optional base URL for the organization context. If not provided, it will default to the source provider's base URL.
   */
  baseUrl?: string;
  /**
   * Instance ID for this organization context. Must be unique across the app if multiple contexts are used.
   */
  instanceId: number;
  /**
   * Optional source instance ID. If not provided, immediate parent provider is used as source.
   */
  sourceInstanceId?: number;
  /**
   * ID of the organization to authenticate with
   */
  targetOrganizationId: string;
}

const OrganizationContext: FC<PropsWithChildren<OrganizationContextProps>> = ({
  instanceId,
  baseUrl,
  clientId,
  afterSignInUrl,
  afterSignOutUrl,
  targetOrganizationId,
  sourceInstanceId,
  scopes,
  children,
  ...rest
}: PropsWithChildren<OrganizationContextProps>) => {
  // Get the source provider's signed-in status
  const {
    isSignedIn: isSourceSignedIn,
    instanceId: sourceInstanceIdFromContext,
    baseUrl: sourceBaseUrl,
    clientId: sourceClientId,
  } = useThunderID();

  return (
    <ThunderIDProvider
      instanceId={instanceId}
      baseUrl={baseUrl || sourceBaseUrl}
      clientId={clientId || sourceClientId}
      afterSignInUrl={afterSignInUrl}
      afterSignOutUrl={afterSignOutUrl}
      scopes={scopes}
      organizationChain={{
        sourceInstanceId: sourceInstanceId || sourceInstanceIdFromContext,
        targetOrganizationId,
      }}
      {...rest}
    >
      <OrganizationContextController targetOrganizationId={targetOrganizationId} isSourceSignedIn={isSourceSignedIn}>
        {children}
      </OrganizationContextController>
    </ThunderIDProvider>
  );
};

export default OrganizationContext;
