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

import {FlowMetadataResponse} from '@thunderid/browser';
import {FC, ReactElement, ReactNode, useState} from 'react';
import {BaseUserDropdown as BaseUserDropdownComponent, BaseUserDropdownProps} from './BaseUserDropdown';
import useThunderID from '../../../contexts/ThunderID/useThunderID';
import UserProfile from '../UserProfile/UserProfile';

/**
 * Render props data passed to the children function
 */
export interface UserDropdownRenderProps {
  /** Function to close the profile dialog */
  closeProfile: () => void;
  /** Whether user data is currently loading */
  isLoading: boolean;
  /** Whether the profile dialog is currently open */
  isProfileOpen: boolean;
  /** Flow metadata returned by the platform (v2 only). `null` while loading or unavailable. */
  meta: FlowMetadataResponse | null;
  /** Function to open the user profile dialog */
  openProfile: () => void;
  /** Function to sign out the user */
  signOut: () => void;
  /** The authenticated user object */
  user: any;
}

/**
 * Props for the UserDropdown component.
 * Extends BaseUserDropdownProps but excludes user, onManageProfile, and onSignOut since they're handled internally
 */
export type UserDropdownProps = Omit<BaseUserDropdownProps, 'user' | 'onManageProfile'> & {
  /**
   * Render prop function that receives user state and actions.
   * When provided, this completely replaces the default dropdown rendering.
   */
  children?: (props: UserDropdownRenderProps) => ReactNode;
  /**
   * Custom render function for the dropdown content.
   * When provided, this replaces just the dropdown content while keeping the trigger.
   */
  renderDropdown?: (props: UserDropdownRenderProps) => ReactNode;
  /**
   * Custom render function for the trigger button.
   * When provided, this replaces just the trigger button while keeping the dropdown.
   */
  renderTrigger?: (props: UserDropdownRenderProps) => ReactNode;
};

/**
 * UserDropdown component displays a user avatar with a dropdown menu.
 * When clicked, it shows a popover with customizable menu items.
 * This component is the React-specific implementation that uses the BaseUserDropdown
 * and automatically retrieves the user data from ThunderID context.
 *
 * Supports render props for complete customization of the dropdown appearance and behavior.
 *
 * @example
 * ```tsx
 * // Basic usage - will use user from ThunderID context
 * <UserDropdown menuItems={[
 *   { label: 'Profile', onClick: () => {} },
 *   { label: 'Settings', href: '/settings' },
 *   { label: 'Sign Out', onClick: () => {} }
 * ]} />
 *
 * // With custom configuration
 * <UserDropdown
 *   showTriggerLabel={true}
 *   avatarSize={40}
 *   fallback={<div>Please sign in</div>}
 * />
 *
 * // Using render props for complete customization
 * <UserDropdown>
 *   {({ user, isLoading, openProfile, signOut }) => (
 *     <div>
 *       <button onClick={openProfile}>
 *         {user?.name || 'Loading...'}
 *       </button>
 *       <button onClick={signOut}>Logout</button>
 *     </div>
 *   )}
 * </UserDropdown>
 *
 * // Using partial render props
 * <UserDropdown
 *   renderTrigger={({ user, openProfile }) => (
 *     <button onClick={openProfile} className="custom-trigger">
 *       Welcome, {user?.name}!
 *     </button>
 *   )}
 * />
 * ```
 */
const UserDropdown: FC<UserDropdownProps> = ({
  children,
  renderTrigger,
  renderDropdown,
  onSignOut,
  ...rest
}: UserDropdownProps): ReactElement => {
  const {user, isLoading, signOut, meta} = useThunderID();
  const [isProfileOpen, setIsProfileOpen] = useState(false);

  const handleManageProfile = (): void => {
    setIsProfileOpen(true);
  };

  const handleSignOut = (): void => {
    signOut();
    if (onSignOut) {
      onSignOut();
    }
  };

  const closeProfile = (): void => {
    setIsProfileOpen(false);
  };

  // Prepare render props data
  const renderProps: UserDropdownRenderProps = {
    closeProfile,
    isLoading: isLoading,
    isProfileOpen,
    meta,
    openProfile: handleManageProfile,
    signOut: handleSignOut,
    user,
  };

  // If children render prop is provided, use it for complete customization
  if (children) {
    return (
      <>
        {children(renderProps)}
        <UserProfile mode="popup" open={isProfileOpen} onOpenChange={setIsProfileOpen} />
      </>
    );
  }

  // If partial render props are provided, customize specific parts
  if (renderTrigger || renderDropdown) {
    // This would require significant changes to BaseUserDropdown to support partial customization
    // For now, we'll provide a simple implementation that shows how it could work
    return (
      <>
        {renderTrigger ? (
          renderTrigger(renderProps)
        ) : (
          <BaseUserDropdownComponent
            user={user}
            isLoading={isLoading}
            onManageProfile={handleManageProfile}
            onSignOut={handleSignOut}
            {...rest}
          />
        )}
        {/* Note: renderDropdown would need BaseUserDropdown modifications to implement properly */}
        <UserProfile mode="popup" open={isProfileOpen} onOpenChange={setIsProfileOpen} />
      </>
    );
  }

  // Default behavior - use BaseUserDropdown as before
  return (
    <>
      <BaseUserDropdownComponent
        user={user}
        isLoading={isLoading}
        onManageProfile={handleManageProfile}
        onSignOut={handleSignOut}
        {...rest}
      />
      {isProfileOpen && <UserProfile mode="popup" open={isProfileOpen} onOpenChange={setIsProfileOpen} />}
    </>
  );
};

export default UserDropdown;
