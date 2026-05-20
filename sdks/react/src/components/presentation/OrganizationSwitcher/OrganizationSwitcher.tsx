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

import {FC, ReactElement, useState} from 'react';

import {
  BaseOrganizationSwitcher as BaseOrganizationSwitcherComponent,
  BaseOrganizationSwitcherProps,
  Organization,
} from './BaseOrganizationSwitcher';
import useOrganization from '../../../contexts/Organization/useOrganization';
import useThunderID from '../../../contexts/ThunderID/useThunderID';
import useTranslation from '../../../hooks/useTranslation';
import BuildingAlt from '../../primitives/Icons/BuildingAlt';
import {CreateOrganization} from '../CreateOrganization/CreateOrganization';
import OrganizationListComponent from '../OrganizationList/OrganizationList';
import OrganizationProfile from '../OrganizationProfile/OrganizationProfile';

/**
 * Props interface for the OrganizationSwitcher component.
 * Makes organizations optional since they'll be retrieved from OrganizationContext.
 */
export interface OrganizationSwitcherProps
  extends Omit<BaseOrganizationSwitcherProps, 'organizations' | 'currentOrganization' | 'onOrganizationSwitch'> {
  /**
   * Optional override for current organization (will use context if not provided)
   */
  currentOrganization?: Organization;
  /**
   * Fallback element to render when the user is not signed in.
   */
  fallback?: ReactElement;
  /**
   * Optional callback for organization switch (will use context if not provided)
   */
  onOrganizationSwitch?: (organization: Organization) => Promise<void> | void;
  /**
   * Optional override for organizations list (will use context if not provided)
   */
  organizations?: Organization[];
}

/**
 * OrganizationSwitcher component that provides organization switching functionality.
 * This component automatically retrieves organizations from the OrganizationContext.
 * You can also override the organizations, currentOrganization, and onOrganizationSwitch
 * by passing them as props.
 *
 * @example
 * ```tsx
 * import { OrganizationSwitcher } from '@thunderid/react';
 *
 * // Basic usage - uses OrganizationContext
 * <OrganizationSwitcher />
 *
 * // With custom organization switch handler
 * <OrganizationSwitcher
 *   onOrganizationSwitch={(org) => {
 *     console.log('Switching to:', org.name);
 *     // Custom logic here
 *   }}
 * />
 *
 * // With fallback for unauthenticated users
 * <OrganizationSwitcher
 *   fallback={<div>Please sign in to view organizations</div>}
 * />
 * ```
 */
export const OrganizationSwitcher: FC<OrganizationSwitcherProps> = ({
  currentOrganization: propCurrentOrganization,
  fallback = null,
  onOrganizationSwitch: propOnOrganizationSwitch,
  organizations: propOrganizations,
  preferences,
  ...props
}: OrganizationSwitcherProps): ReactElement => {
  const {isSignedIn} = useThunderID();
  const {
    currentOrganization: contextCurrentOrganization,
    myOrganizations: contextOrganizations,
    switchOrganization,
    isLoading,
    error,
  } = useOrganization();
  const [isCreateOrgOpen, setIsCreateOrgOpen] = useState(false);
  const [isProfileOpen, setIsProfileOpen] = useState(false);
  const [isOrganizationListOpen, setIsOrganizationListOpen] = useState(false);
  const {t} = useTranslation(preferences?.i18n);

  if (!isSignedIn && fallback) {
    return fallback;
  }

  if (!isSignedIn) {
    return <></>;
  }

  const organizations: Organization[] = propOrganizations || contextOrganizations || [];
  const currentOrganization: Organization | null = propCurrentOrganization || contextCurrentOrganization;
  const onOrganizationSwitch: (organization: Organization) => Promise<void> | void =
    propOnOrganizationSwitch || switchOrganization;

  const handleManageOrganizations = (): void => {
    setIsOrganizationListOpen(true);
  };

  const handleManageOrganization = (): void => {
    setIsProfileOpen(true);
  };

  const defaultMenuItems: {icon?: ReactElement; label: string; onClick: () => void}[] = [];

  if (currentOrganization) {
    defaultMenuItems.push({
      icon: <BuildingAlt />,
      label: t('organization.switcher.manage.organizations'),
      onClick: handleManageOrganizations,
    });
  }

  defaultMenuItems.push({
    icon: (
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
        <path d="M12 5v14m-7-7h14" />
      </svg>
    ),
    label: t('organization.switcher.create.organization'),
    onClick: (): void => setIsCreateOrgOpen(true),
  });

  const menuItems: any[] = props.menuItems ? [...defaultMenuItems, ...props.menuItems] : defaultMenuItems;

  return (
    <>
      <BaseOrganizationSwitcherComponent
        organizations={organizations}
        currentOrganization={currentOrganization}
        onOrganizationSwitch={onOrganizationSwitch}
        loading={isLoading}
        error={error}
        menuItems={menuItems}
        onManageProfile={handleManageOrganization}
        preferences={preferences}
        {...props}
      />
      <CreateOrganization
        mode="popup"
        open={isCreateOrgOpen}
        onOpenChange={setIsCreateOrgOpen}
        onSuccess={(org: Organization): void => {
          if (org && onOrganizationSwitch) {
            onOrganizationSwitch(org);
          }
          setIsCreateOrgOpen(false);
        }}
      />
      {currentOrganization && (
        <OrganizationProfile
          organizationId={currentOrganization.id}
          mode="popup"
          open={isProfileOpen}
          onOpenChange={setIsProfileOpen}
          cardLayout={true}
          loadingFallback={<div>{t('organization.profile.loading')}</div>}
          errorFallback={<div>{t('organization.profile.error')}</div>}
        />
      )}
      <OrganizationListComponent
        mode="popup"
        open={isOrganizationListOpen}
        onOpenChange={setIsOrganizationListOpen}
        title={t('organization.switcher.manage.organizations')}
        onOrganizationSelect={(organization: Organization): void => {
          if (onOrganizationSwitch) {
            onOrganizationSwitch(organization);
          }
          setIsOrganizationListOpen(false);
        }}
      />
    </>
  );
};

export default OrganizationSwitcher;
