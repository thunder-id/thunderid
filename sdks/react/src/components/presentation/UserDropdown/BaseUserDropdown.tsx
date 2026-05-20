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

import {cx} from '@emotion/css';
import {
  useFloating,
  autoUpdate,
  offset,
  flip,
  shift,
  useClick,
  useDismiss,
  useRole,
  useInteractions,
  FloatingFocusManager,
  FloatingPortal,
} from '@floating-ui/react';
import {withVendorCSSClassPrefix} from '@thunderid/browser';
import {FC, ReactElement, ReactNode, useState} from 'react';
import useStyles from './BaseUserDropdown.styles';
import useTheme from '../../../contexts/Theme/useTheme';
import getDisplayName from '../../../utils/getDisplayName';
import getMappedUserProfileValue from '../../../utils/getMappedUserProfileValue';
import {Avatar} from '../../primitives/Avatar/Avatar';
import Button from '../../primitives/Button/Button';
import LogOut from '../../primitives/Icons/LogOut';
import User from '../../primitives/Icons/User';
import Typography from '../../primitives/Typography/Typography';

interface MenuItem {
  href?: string;
  icon?: ReactNode;
  label: ReactNode;
  onClick?: () => void;
}

export interface BaseUserDropdownProps {
  /**
   * Mapping of component attribute names to identity provider field names.
   * Allows customizing which user profile fields should be used for each attribute.
   */
  attributeMapping?: {
    [key: string]: string | string[] | undefined;
    firstName?: string | string[];
    lastName?: string | string[];
    picture?: string | string[];
    username?: string | string[];
  };
  /**
   * Optional size for the avatar
   */
  avatarSize?: number;
  /**
   * Optional className for the dropdown container.
   */
  className?: string;
  /**
   * Optional element to render when no user is signed in.
   */
  fallback?: ReactElement;
  /**
   * Whether the user data is currently loading
   */
  isLoading?: boolean;
  /**
   * Menu items to display in the dropdown
   */
  menuItems?: MenuItem[];
  /**
   * Callback function for "Manage Profile" action
   */
  onManageProfile?: () => void;
  /**
   * Callback function for "Sign Out" action
   */
  onSignOut?: () => void;
  /**
   * The HTML element ID where the portal should be mounted
   */
  portalId?: string;
  /**
   * Show dropdown header with user information
   */
  showDropdownHeader?: boolean;
  /**
   * Show user's display name next to avatar in the trigger button
   */
  showTriggerLabel?: boolean;
  /**
   * The user object containing profile information
   */
  user: any;
}

/**
 * BaseUserDropdown component displays a user avatar with a dropdown menu.
 * When clicked, it shows a popover with customizable menu items.
 * This component serves as the base for framework-specific implementations.
 */
export const BaseUserDropdown: FC<BaseUserDropdownProps> = ({
  fallback = null,
  className = '',
  user,
  isLoading = false,
  portalId = 'thunderid-user-dropdown',
  menuItems = [],
  showTriggerLabel = false,
  avatarSize = 32,
  onManageProfile,
  onSignOut,
  attributeMapping = {},
}: BaseUserDropdownProps): ReactElement => {
  const {theme, colorScheme} = useTheme();
  const styles: Record<string, string> = useStyles(theme, colorScheme);
  const [isOpen, setIsOpen] = useState(false);
  const [hoveredItemIndex, setHoveredItemIndex] = useState<number | null>(null);

  const {refs, floatingStyles, context} = useFloating({
    middleware: [offset(5), flip({fallbackAxisSideDirection: 'end'}), shift({padding: 5})],
    onOpenChange: setIsOpen,
    open: isOpen,
    placement: 'bottom-end',
    whileElementsMounted: autoUpdate,
  });

  const click: any = useClick(context);
  const dismiss: any = useDismiss(context);
  const role: any = useRole(context);

  const {getReferenceProps, getFloatingProps} = useInteractions([click, dismiss, role]);

  const defaultAttributeMappings: Record<string, string | string[] | undefined> = {
    email: ['emails'],
    firstName: ['name.givenName', 'given_name'],
    lastName: ['name.familyName', 'family_name'],
    picture: ['profile', 'profileUrl', 'picture', 'URL'],
    username: ['userName', 'username', 'user_name'],
  };

  const mergedMappings: Record<string, string | string[] | undefined> = {
    ...defaultAttributeMappings,
    ...attributeMapping,
  };

  if (fallback && !user && !isLoading) {
    return fallback;
  }

  const handleMenuItemClick = (item: MenuItem): void => {
    if (item.onClick) {
      item.onClick();
    }
    setIsOpen(false);
  };

  const defaultMenuItems: MenuItem[] = [];

  if (onManageProfile) {
    defaultMenuItems.push({
      icon: <User width="16" height="16" />,
      label: 'Manage Profile',
      onClick: onManageProfile,
    });
  }

  if (onSignOut) {
    defaultMenuItems.push({
      icon: <LogOut width="16" height="16" />,
      label: 'Sign Out',
      onClick: onSignOut,
    });
  }

  const allMenuItems: MenuItem[] = [...menuItems];

  if (defaultMenuItems.length > 0) {
    if (menuItems.length > 0) {
      allMenuItems.push({label: '', onClick: undefined});
    }

    allMenuItems.push(...defaultMenuItems);
  }

  return (
    <div className={cx(withVendorCSSClassPrefix('user-dropdown'), className)}>
      <Button
        ref={refs.setReference}
        className={cx(withVendorCSSClassPrefix('user-dropdown__trigger'), styles['trigger'])}
        color="tertiary"
        variant="text"
        size="medium"
        data-testid="thunderid-user-dropdown-trigger"
        {...getReferenceProps()}
      >
        <Avatar
          imageUrl={getMappedUserProfileValue('picture', mergedMappings, user)}
          name={getDisplayName(mergedMappings, user)}
          size={avatarSize}
          alt={`${getDisplayName(mergedMappings, user)}'s avatar`}
        />
        {showTriggerLabel && (
          <Typography
            variant="body2"
            className={cx(withVendorCSSClassPrefix('user-dropdown__trigger-label'), styles['userName'])}
          >
            {getDisplayName(mergedMappings, user)}
          </Typography>
        )}
      </Button>

      {isOpen && (
        <FloatingPortal id={portalId}>
          <FloatingFocusManager context={context} modal={false} initialFocus={-1}>
            <div
              ref={refs.setFloating}
              className={cx(withVendorCSSClassPrefix('user-dropdown__content'), styles['dropdownContent'])}
              style={{
                ...floatingStyles,
                // Floating UI doesn't set a z-index by default, so we set a high value to ensure the dropdown appears above other elements.
                // https://floating-ui.com/docs/misc#z-index-stacking
                zIndex: 9999,
              }}
              {...getFloatingProps()}
            >
              <div className={cx(withVendorCSSClassPrefix('user-dropdown__header'), styles['dropdownHeader'])}>
                <Avatar
                  imageUrl={getMappedUserProfileValue('picture', mergedMappings, user)}
                  name={getDisplayName(mergedMappings, user)}
                  size={avatarSize * 1.25}
                  alt={`${getDisplayName(mergedMappings, user)}'s avatar`}
                />
                <div className={cx(withVendorCSSClassPrefix('user-dropdown__header-info'), styles['headerInfo'])}>
                  <Typography
                    noWrap
                    className={withVendorCSSClassPrefix('user-dropdown__header-name')}
                    variant="body1"
                    fontWeight="medium"
                  >
                    {getDisplayName(mergedMappings, user)}
                  </Typography>
                  <Typography
                    noWrap
                    className={withVendorCSSClassPrefix('user-dropdown__header-email')}
                    variant="caption"
                    color="secondary"
                  >
                    {getMappedUserProfileValue('username', mergedMappings, user) ||
                      getMappedUserProfileValue('email', mergedMappings, user)}
                  </Typography>
                </div>
              </div>
              <div className={cx(withVendorCSSClassPrefix('user-dropdown__menu'), styles['dropdownMenu'])}>
                {allMenuItems.map((item: any, index: number) => (
                  <div key={index}>
                    {((): ReactElement => {
                      if (item.label === '') {
                        return (
                          <div
                            className={cx(withVendorCSSClassPrefix('user-dropdown__menu-divider'), styles['divider'])}
                          />
                        );
                      }
                      if (item.href) {
                        return (
                          <a
                            href={item.href}
                            style={{
                              backgroundColor:
                                hoveredItemIndex === index ? theme.vars.colors.action?.hover : 'transparent',
                            }}
                            className={cx(
                              withVendorCSSClassPrefix('user-dropdown__menu-item'),
                              styles['menuItemAnchor'],
                            )}
                            onMouseEnter={(): void => setHoveredItemIndex(index)}
                            onMouseLeave={(): void => setHoveredItemIndex(null)}
                            onFocus={(): void => setHoveredItemIndex(index)}
                            onBlur={(): void => setHoveredItemIndex(null)}
                          >
                            {item.icon}
                            <span>{item.label}</span>
                          </a>
                        );
                      }
                      return (
                        <Button
                          onClick={(): void => handleMenuItemClick(item)}
                          style={{
                            backgroundColor:
                              hoveredItemIndex === index ? theme.vars.colors.action?.hover : 'transparent',
                          }}
                          className={cx(withVendorCSSClassPrefix('user-dropdown__menu-item'), styles['menuItem'])}
                          color="tertiary"
                          variant="text"
                          size="small"
                          startIcon={item.icon}
                          onMouseEnter={(): void => setHoveredItemIndex(index)}
                          onMouseLeave={(): void => setHoveredItemIndex(null)}
                        >
                          {item.label}
                        </Button>
                      );
                    })()}
                  </div>
                ))}
              </div>
            </div>
          </FloatingFocusManager>
        </FloatingPortal>
      )}
    </div>
  );
};

export default BaseUserDropdown;
