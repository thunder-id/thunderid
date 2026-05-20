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

import {EmbeddedFlowType, getOrganizationUnitChildren, OrganizationUnitListResponse} from '@thunderid/browser';
import {FC, ReactElement, ReactNode, useCallback} from 'react';
// eslint-disable-next-line import/no-named-as-default
import BaseInviteUser, {BaseInviteUserRenderProps, InviteUserFlowResponse} from './BaseInviteUser';
import useThunderID from '../../../../../contexts/ThunderID/useThunderID';

/**
 * Render props for InviteUser (re-exported for convenience).
 */
export type InviteUserRenderProps = BaseInviteUserRenderProps;

/**
 * Props for the InviteUser component.
 */
export interface InviteUserProps {
  /**
   * Render props function for custom UI.
   * If not provided, default UI will be rendered by the SDK.
   */
  children?: (props: InviteUserRenderProps) => ReactNode;

  /**
   * Custom CSS class name.
   */
  className?: string;

  /**
   * Callback when an error occurs.
   */
  onError?: (error: Error) => void;

  /**
   * Callback when the flow state changes.
   */
  onFlowChange?: (response: InviteUserFlowResponse) => void;

  /**
   * Whether to show the subtitle.
   */
  showSubtitle?: boolean;

  /**
   * Whether to show the title.
   */
  showTitle?: boolean;

  /**
   * Size variant for the component.
   */
  size?: 'small' | 'medium' | 'large';

  /**
   * Theme variant for the component card.
   */
  variant?: 'outlined' | 'elevated';
}

/**
 * InviteUser component for initiating invite user flow.
 *
 * This component is designed for admin users in the thunder-develop app to:
 * 1. Select a user type (if multiple available)
 * 2. Enter user details (username, email)
 * 3. Generate an invite link for the end user
 *
 * The component uses the authenticated ThunderID SDK context to make API calls
 * with the admin's access token (requires 'system' scope).
 *
 * @example
 * ```tsx
 * import { InviteUser } from '@thunderid/react';
 *
 * const InviteUserPage = () => {
 *   const [inviteLink, setInviteLink] = useState<string>();
 *
 *   return (
 *     <InviteUser
 *       onInviteLinkGenerated={(link, executionId) => setInviteLink(link)}
 *       onError={(error) => console.error(error)}
 *     >
 *       {({ values, components, isLoading, handleInputChange, handleSubmit, inviteLink, isInviteGenerated }) => (
 *         <div>
 *           {isInviteGenerated ? (
 *             <div>
 *               <h2>Invite Link Generated!</h2>
 *               <p>{inviteLink}</p>
 *             </div>
 *           ) : (
 *             // Render form based on components
 *           )}
 *         </div>
 *       )}
 *     </InviteUser>
 *   );
 * };
 * ```
 */
const InviteUser: FC<InviteUserProps> = ({
  onError,
  onFlowChange,
  className,
  children,
  size = 'medium',
  variant = 'outlined',
  showTitle = true,
  showSubtitle = true,
}: InviteUserProps): ReactElement => {
  const {http, baseUrl, getAccessToken, isInitialized} = useThunderID();

  /**
   * Initialize the invite user flow.
   * Makes an authenticated request to /flow/execute with flowType: USER_ONBOARDING.
   */
  const handleInitialize = async (payload: Record<string, any>): Promise<InviteUserFlowResponse> => {
    const response: any = await http.request({
      data: {
        ...payload,
        flowType: EmbeddedFlowType.UserOnboarding,
        verbose: true,
      },
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json',
      },
      method: 'POST',
      url: `${baseUrl}/flow/execute`,
    } as any);

    return response.data as InviteUserFlowResponse;
  };

  /**
   * Submit flow step data.
   * Makes an authenticated request to /flow/execute with the step data.
   */
  const handleSubmit = async (payload: Record<string, any>): Promise<InviteUserFlowResponse> => {
    const response: any = await http.request({
      data: {
        ...payload,
        verbose: true,
      },
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json',
      },
      method: 'POST',
      url: `${baseUrl}/flow/execute`,
    } as any);

    return response.data as InviteUserFlowResponse;
  };

  const fetchOrganizationUnitChildren: (
    parentId: string,
    limit: number,
    offset: number,
  ) => Promise<OrganizationUnitListResponse> = useCallback(
    async (parentId: string, limit: number, offset: number): Promise<OrganizationUnitListResponse> => {
      const accessToken: string = await getAccessToken();

      return getOrganizationUnitChildren({
        baseUrl,
        headers: {Authorization: `Bearer ${accessToken}`},
        limit,
        offset,
        organizationUnitId: parentId,
      });
    },
    [baseUrl, getAccessToken],
  );

  return (
    <BaseInviteUser
      onInitialize={handleInitialize}
      onSubmit={handleSubmit}
      onError={onError}
      onFlowChange={onFlowChange}
      className={className}
      fetchOrganizationUnitChildren={fetchOrganizationUnitChildren}
      isInitialized={isInitialized}
      size={size}
      variant={variant}
      showTitle={showTitle}
      showSubtitle={showSubtitle}
    >
      {children}
    </BaseInviteUser>
  );
};

export default InviteUser;
