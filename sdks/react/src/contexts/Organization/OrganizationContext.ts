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

import {AllOrganizationsApiResponse, Organization, CreateOrganizationPayload} from '@thunderid/browser';
import {Context, createContext} from 'react';

/**
 * Props interface of {@link OrganizationContext}
 */
export interface OrganizationContextProps {
  /**
   * Function to create a new organization.
   */
  createOrganization?: (payload: CreateOrganizationPayload, sessionId: string) => Promise<Organization>;
  currentOrganization: Organization | null;
  error: string | null;
  getAllOrganizations: () => Promise<AllOrganizationsApiResponse>;
  isLoading: boolean;
  myOrganizations: Organization[];
  revalidateMyOrganizations: () => Promise<Organization[]>;
  switchOrganization: (organization: Organization) => Promise<void>;
}

/**
 * Context object for managing organization data and related operations.
 */
const OrganizationContext: Context<OrganizationContextProps | null> = createContext<null | OrganizationContextProps>({
  createOrganization: () => null,
  currentOrganization: null,
  error: null,
  getAllOrganizations: () =>
    Promise.resolve({
      count: 0,
      organizations: [],
    }),
  isLoading: false,
  myOrganizations: null,
  revalidateMyOrganizations: () => Promise.resolve([]),
  switchOrganization: () => Promise.resolve(),
});

OrganizationContext.displayName = 'OrganizationContext';

export default OrganizationContext;
