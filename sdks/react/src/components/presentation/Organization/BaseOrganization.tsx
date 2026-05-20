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

import {Organization as IOrganization} from '@thunderid/browser';
import {FC, ReactElement, ReactNode} from 'react';

/**
 * Props for the BaseOrganization component.
 */
export interface BaseOrganizationProps {
  /**
   * Render prop that takes the organization object and returns a ReactNode.
   * @param organization - The organization object from ThunderID.
   * @returns A ReactNode to render.
   */
  children: (organization: IOrganization | null) => ReactNode;

  /**
   * Optional element to render when no organization is provided.
   */
  fallback?: ReactNode;

  /**
   * The organization object to display. If not provided, the component will render the fallback.
   */
  organization: IOrganization | null;
}

/**
 * Base Organization component that provides the core functionality for displaying organization information.
 * This component takes an organization object as a prop and uses render props to expose it.
 *
 * @remarks This is the base component that can be used in any context where you have
 * an organization object available. For React applications, use the Organization component which
 * automatically retrieves the current organization from Organization context.
 *
 * @example
 * ```tsx
 * import { BaseOrganization } from '@thunderid/auth-react';
 *
 * const MyComponent = ({ organization }) => {
 *   return (
 *     <BaseOrganization organization={organization} fallback={<p>No organization data</p>}>
 *       {(org) => (
 *         <div>
 *           <h1>Organization: {org.name}</h1>
 *           <p>ID: {org.id}</p>
 *         </div>
 *       )}
 *     </BaseOrganization>
 *   );
 * }
 * ```
 */
const BaseOrganization: FC<BaseOrganizationProps> = ({
  children,
  fallback = null,
  organization,
}: BaseOrganizationProps): ReactElement => {
  if (!organization) {
    return <>{fallback}</>;
  }

  return <>{children(organization)}</>;
};

BaseOrganization.displayName = 'BaseOrganization';

export default BaseOrganization;
