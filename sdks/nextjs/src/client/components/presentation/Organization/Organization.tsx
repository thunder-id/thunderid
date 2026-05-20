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

'use client';

import {Organization as IOrganization} from '@thunderid/node';
import {BaseOrganization, BaseOrganizationProps, useOrganization} from '@thunderid/react';
import {FC, ReactElement, ReactNode} from 'react';

/**
 * Props for the Organization component.
 * Extends BaseOrganizationProps but makes the organization prop optional since it will be obtained from useOrganization
 */
export interface OrganizationProps extends Omit<BaseOrganizationProps, 'organization'> {
  /**
   * Render prop that takes the organization object and returns a ReactNode.
   * @param organization - The current organization object from Organization context.
   * @returns A ReactNode to render.
   */
  children: (organization: IOrganization | null) => ReactNode;

  /**
   * Optional element to render when no organization is selected.
   */
  fallback?: ReactNode;
}

/**
 * A component that uses render props to expose the current organization object.
 * This component automatically retrieves the current organization from Organization context.
 *
 * @remarks This component is only supported in browser based React applications (CSR).
 *
 * @example
 * ```tsx
 * import { Organization } from '@thunderid/auth-react';
 *
 * const App = () => {
 *   return (
 *     <Organization fallback={<p>No organization selected</p>}>
 *       {(organization) => (
 *         <div>
 *           <h1>Current Organization: {organization.name}!</h1>
 *           <p>ID: {organization.id}</p>
 *           <p>Role: {organization.role}</p>
 *           {organization.memberCount && (
 *             <p>Members: {organization.memberCount}</p>
 *           )}
 *         </div>
 *       )}
 *     </Organization>
 *   );
 * }
 * ```
 */
const Organization: FC<OrganizationProps> = ({children, fallback = null}: OrganizationProps): ReactElement => {
  const {currentOrganization} = useOrganization();

  return (
    <BaseOrganization organization={currentOrganization} fallback={fallback}>
      {children}
    </BaseOrganization>
  );
};

Organization.displayName = 'Organization';

export default Organization;
