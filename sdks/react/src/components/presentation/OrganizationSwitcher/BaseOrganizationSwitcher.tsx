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

// Removed BEM and vendor prefix utilities
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
import {Preferences} from '@thunderid/browser';
import {CSSProperties, FC, ReactElement, ReactNode, useState} from 'react';
import useStyles from './BaseOrganizationSwitcher.styles';
import useTheme from '../../../contexts/Theme/useTheme';
import useTranslation from '../../../hooks/useTranslation';
import {Avatar} from '../../primitives/Avatar/Avatar';
import Button from '../../primitives/Button/Button';
import Building from '../../primitives/Icons/Building';
import Check from '../../primitives/Icons/Check';
import ChevronDown from '../../primitives/Icons/ChevronDown';
import Typography from '../../primitives/Typography/Typography';

interface MenuItem {
  href?: string;
  icon?: ReactNode;
  label: ReactNode;
  onClick?: () => void;
}

/**
 * Interface for organization data.
 */
export interface Organization {
  /**
   * Avatar URL for the organization.
   */
  avatar?: string;
  /**
   * Unique identifier for the organization.
   */
  id: string;
  /**
   * Number of members in the organization.
   */
  memberCount?: number;
  /**
   * Additional metadata for the organization.
   */
  metadata?: Record<string, any>;
  /**
   * Display name of the organization.
   */
  name: string;
  /**
   * User's role in the organization.
   */
  role?: 'owner' | 'admin' | 'member';
  /**
   * URL slug for the organization.
   */
  slug?: string;
}

/**
 * Props interface for the BaseOrganizationSwitcher component.
 */
export interface BaseOrganizationSwitcherProps {
  /**
   * Optional size for the avatar
   */
  avatarSize?: number;
  /**
   * Custom class name for styling.
   */
  className?: string;
  /**
   * Currently selected organization.
   */
  currentOrganization?: Organization;
  /**
   * Error message to display.
   */
  error?: string;
  /**
   * Optional element to render when no organization is selected.
   */
  fallback?: ReactElement;
  /**
   * Whether the component is in a loading state.
   */
  loading?: boolean;
  /**
   * Additional menu items to display at the bottom of the dropdown.
   */
  menuItems?: MenuItem[];
  /**
   * Handler for when the manage profile button is clicked.
   */
  onManageProfile?: () => void;
  /**
   * Handler for when an organization is selected.
   */
  onOrganizationSwitch: (organization: Organization) => void;
  /**
   * List of available organizations.
   */
  organizations: Organization[];
  /**
   * The HTML element ID where the portal should be mounted
   */
  portalId?: string;
  /**
   * Component-level preferences to override global i18n and theme settings.
   * Preferences are deep-merged with global ones, with component preferences
   * taking precedence. Affects this component and all its descendants.
   */
  preferences?: Preferences;
  /**
   * Custom render function for the error state.
   */
  renderError?: (error: string) => ReactElement;
  /**
   * Custom render function for the loading state.
   */
  renderLoading?: () => ReactElement;
  /**
   * Custom render function for the organization item.
   */
  renderOrganization?: (organization: Organization, isSelected: boolean) => ReactElement;
  /**
   * Whether to show the member count.
   */
  showMemberCount?: boolean;
  /**
   * Whether to show the role badge.
   */
  showRole?: boolean;
  /**
   * Show organization name next to avatar in the trigger button
   */
  showTriggerLabel?: boolean;

  /**
   * Custom styles for the component.
   */
  style?: CSSProperties;
}

/**
 * BaseOrganizationSwitcher component displays an organization selector with a dropdown menu.
 * When clicked, it shows a popover with available organizations to switch between.
 * This component serves as the base for framework-specific implementations.
 */
export const BaseOrganizationSwitcher: FC<BaseOrganizationSwitcherProps> = ({
  organizations,
  currentOrganization,
  loading = false,
  error,
  onOrganizationSwitch,
  onManageProfile,
  className = '',
  style,
  renderOrganization,
  renderLoading,
  renderError,
  showRole = false,
  showMemberCount = true,
  menuItems = [],
  portalId = 'thunderid-organization-switcher',
  showTriggerLabel = true,
  avatarSize = 24,
  fallback = null,
  preferences,
}: BaseOrganizationSwitcherProps): ReactElement => {
  const {theme, colorScheme, direction} = useTheme();
  const styles: Record<string, string> = useStyles(theme, colorScheme);
  const [isOpen, setIsOpen] = useState(false);
  const [hoveredItemIndex, setHoveredItemIndex] = useState<number | null>(null);
  const {t} = useTranslation(preferences?.i18n);
  const isRTL: boolean = direction === 'rtl';

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

  if (fallback && !currentOrganization && !loading && organizations.length === 0) {
    return fallback;
  }

  const handleOrganizationSwitch = (organization: Organization): void => {
    onOrganizationSwitch(organization);
    setIsOpen(false);
  };

  const handleMenuItemClick = (item: MenuItem): void => {
    if (item.onClick) {
      item.onClick();
    }
    setIsOpen(false);
  };

  // Filter out current organization from switchable list
  const switchableOrganizations: Organization[] = organizations.filter(
    (org: Organization): boolean => org.id !== currentOrganization?.id,
  );

  const defaultRenderOrganization = (organization: Organization, isSelected: boolean): ReactElement => (
    <>
      <Avatar
        variant="square"
        imageUrl={organization.avatar}
        name={organization.name}
        size={avatarSize * 1.25}
        alt={`${organization.name} avatar`}
      />
      <div className={cx(styles['organizationInfo'])}>
        <Typography variant="body2" fontWeight="medium" className={cx(styles['organizationName'])}>
          {organization.name}
        </Typography>
        <div className={cx(styles['organizationMeta'])}>
          {showMemberCount && organization.memberCount !== undefined && (
            <span>
              {organization.memberCount}{' '}
              {organization.memberCount === 1 ? t('organization.switcher.member') : t('organization.switcher.members')}
            </span>
          )}
          {showRole && organization.role && showMemberCount && organization.memberCount !== undefined && (
            <span> • </span>
          )}
          {showRole && organization.role && <span className={cx(styles['roleCapitalized'])}>{organization.role}</span>}
        </div>
      </div>
      {isSelected && <Check width="16" height="16" color={theme.vars.colors.text.primary} />}
    </>
  );

  const defaultRenderLoading = (): ReactElement => (
    <div className={cx(styles['loadingContainer'])}>
      <Typography variant="caption" className={cx(styles['loadingText'])}>
        {t('organization.switcher.loading.placeholder.organizations')}
      </Typography>
    </div>
  );

  const defaultRenderError = (errorMessage: string): ReactElement => (
    <div className={cx(styles['errorContainer'])}>
      <Typography variant="caption" className={cx(styles['errorText'])}>
        {errorMessage}
      </Typography>
    </div>
  );

  return (
    <div className={cx(styles['root'], className)} style={style}>
      <Button
        ref={refs.setReference}
        className={cx(styles['trigger'])}
        color="tertiary"
        variant="outline"
        size="medium"
        {...getReferenceProps()}
      >
        {currentOrganization ? (
          <>
            <Avatar
              variant="square"
              imageUrl={currentOrganization.avatar}
              name={currentOrganization.name}
              size={avatarSize}
              alt={`${currentOrganization.name} avatar`}
            />
            {showTriggerLabel && (
              <Typography variant="body2" className={cx(styles['triggerLabel'])}>
                {currentOrganization.name}
              </Typography>
            )}
          </>
        ) : (
          <>
            <Building width={avatarSize} height={avatarSize} />
            {showTriggerLabel && (
              <Typography variant="body2" className={cx(styles['triggerLabel'])}>
                {t('elements.fields.organization.select.label')}
              </Typography>
            )}
          </>
        )}
        <span style={{display: 'inline-flex', transform: isRTL ? 'scaleX(-1)' : 'none'}}>
          <ChevronDown width="16" height="16" />
        </span>
      </Button>

      {isOpen && (
        <FloatingPortal id={portalId}>
          <FloatingFocusManager context={context} modal={false} initialFocus={-1}>
            <div
              ref={refs.setFloating}
              className={cx(styles['content'])}
              style={floatingStyles}
              {...getFloatingProps()}
            >
              {/* Header - Current Organization */}
              {currentOrganization && (
                <div className={cx(styles['header'])}>
                  <Avatar
                    variant="square"
                    imageUrl={currentOrganization.avatar}
                    name={currentOrganization.name}
                    size={avatarSize * 1.5}
                    alt={`${currentOrganization.name} avatar`}
                  />
                  <div className={cx(styles['headerInfo'])}>
                    <Typography noWrap className={cx(styles['headerName'])} variant="body1" fontWeight="medium">
                      {currentOrganization.name}
                    </Typography>
                    <div className={cx(styles['headerMeta'])}>
                      {showMemberCount && currentOrganization.memberCount !== undefined && (
                        <Typography
                          noWrap
                          // ...existing code...
                          variant="caption"
                          color="secondary"
                        >
                          {currentOrganization.memberCount}{' '}
                          {currentOrganization.memberCount === 1
                            ? t('organization.switcher.member')
                            : t('organization.switcher.members')}
                          {showRole && currentOrganization.role && <span> • {currentOrganization.role}</span>}
                        </Typography>
                      )}
                      {showRole &&
                        currentOrganization.role &&
                        (!showMemberCount || currentOrganization.memberCount === undefined) && (
                          <Typography noWrap className={cx(styles['headerRole'])} variant="caption" color="secondary">
                            {currentOrganization.role}
                          </Typography>
                        )}
                    </div>
                  </div>
                  {onManageProfile && (
                    <Button
                      onClick={onManageProfile}
                      color="tertiary"
                      variant="outline"
                      size="small"
                      aria-label="Manage Organization Profile"
                      className={cx(styles['manageButton'])}
                      endIcon={
                        <svg
                          width="16"
                          height="16"
                          viewBox="0 0 24 24"
                          fill="none"
                          stroke="currentColor"
                          strokeWidth="2"
                          strokeLinecap="round"
                          strokeLinejoin="round"
                        >
                          <circle cx="12" cy="12" r="3" />
                          <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1 1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z" />
                        </svg>
                      }
                    >
                      {t('organization.switcher.buttons.manage.text')}
                    </Button>
                  )}
                </div>
              )}

              {/* Section Header for Other Organizations */}
              {organizations.length > 1 && (
                <div
                  className={cx(styles['header'], styles['sectionHeaderContainer'])}
                  style={{
                    borderTop: currentOrganization ? `1px solid ${theme.vars.colors.border}` : 'none',
                  }}
                >
                  <Typography variant="caption" fontWeight={600} className={cx(styles['sectionHeader'])}>
                    {t('organization.switcher.switch.organization')}
                  </Typography>
                </div>
              )}

              {/* Content */}
              <div className={cx(styles['menu'])}>
                {((): ReactElement => {
                  if (loading) {
                    return renderLoading ? renderLoading() : defaultRenderLoading();
                  }
                  if (error) {
                    return renderError ? renderError(error) : defaultRenderError(error);
                  }
                  return (
                    <>
                      {switchableOrganizations.map((organization: Organization): ReactElement => {
                        const isSelected = false; // Never selected since we exclude current org
                        return (
                          <Button
                            key={organization.id}
                            onClick={(): void => handleOrganizationSwitch(organization)}
                            className={cx(styles['menuItem'])}
                            color="tertiary"
                            variant="text"
                            size="small"
                            style={{
                              backgroundColor:
                                hoveredItemIndex === switchableOrganizations.indexOf(organization)
                                  ? theme.vars.colors.action?.hover
                                  : 'transparent',
                            }}
                            onMouseEnter={(): void =>
                              setHoveredItemIndex(switchableOrganizations.indexOf(organization))
                            }
                            onMouseLeave={(): void => setHoveredItemIndex(null)}
                          >
                            {renderOrganization
                              ? renderOrganization(organization, isSelected)
                              : defaultRenderOrganization(organization, isSelected)}
                          </Button>
                        );
                      })}

                      {/* Menu Items */}
                      {menuItems.length > 0 && (
                        <>
                          <div className={cx(styles['menuDivider'])} />
                          {menuItems.map(
                            (item: any, index: number): ReactElement => (
                              <div key={index}>
                                {item.href ? (
                                  <a
                                    href={item.href}
                                    style={{
                                      backgroundColor:
                                        hoveredItemIndex === switchableOrganizations.length + index
                                          ? theme.vars.colors.action?.hover
                                          : 'transparent',
                                    }}
                                    className={cx(styles['menuItem'])}
                                    onMouseEnter={(): void =>
                                      setHoveredItemIndex(switchableOrganizations.length + index)
                                    }
                                    onMouseLeave={(): void => setHoveredItemIndex(null)}
                                    onFocus={(): void => setHoveredItemIndex(switchableOrganizations.length + index)}
                                    onBlur={(): void => setHoveredItemIndex(null)}
                                  >
                                    {item.icon}
                                    <span>{item.label}</span>
                                  </a>
                                ) : (
                                  <Button
                                    onClick={(): void => handleMenuItemClick(item)}
                                    style={{
                                      backgroundColor:
                                        hoveredItemIndex === switchableOrganizations.length + index
                                          ? theme.vars.colors.action?.hover
                                          : 'transparent',
                                    }}
                                    className={cx(styles['menuItem'])}
                                    color="tertiary"
                                    variant="text"
                                    size="small"
                                    startIcon={item.icon}
                                    onMouseEnter={(): void =>
                                      setHoveredItemIndex(switchableOrganizations.length + index)
                                    }
                                    onMouseLeave={(): void => setHoveredItemIndex(null)}
                                  >
                                    {item.label}
                                  </Button>
                                )}
                              </div>
                            ),
                          )}
                        </>
                      )}
                    </>
                  );
                })()}
              </div>
            </div>
          </FloatingFocusManager>
        </FloatingPortal>
      )}
    </div>
  );
};

export default BaseOrganizationSwitcher;
