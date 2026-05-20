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
import {AllOrganizationsApiResponse, Organization, Preferences} from '@thunderid/browser';
import {CSSProperties, FC, MouseEvent, ReactElement, ReactNode, useMemo} from 'react';
import useStyles from './BaseOrganizationList.styles';
import useTheme from '../../../contexts/Theme/useTheme';
import useTranslation from '../../../hooks/useTranslation';
import {Avatar as AvatarPrimitive} from '../../primitives/Avatar/Avatar';
import Button from '../../primitives/Button/Button';
import DialogPrimitive from '../../primitives/Dialog/Dialog';
import Spinner from '../../primitives/Spinner/Spinner';
import Typography from '../../primitives/Typography/Typography';

export interface OrganizationWithSwitchAccess extends Organization {
  canSwitch: boolean;
}

/**
 * Props interface for the BaseOrganizationList component.
 */
export interface BaseOrganizationListProps {
  /**
   * List of organizations discoverable to the signed-in user.
   */
  allOrganizations: AllOrganizationsApiResponse;
  /**
   * Additional CSS class names to apply to the container
   */
  className?: string;
  /**
   * Error message to display
   */
  error?: string | null;
  /**
   * Function called when "Load More" is clicked
   */
  fetchMore?: () => Promise<void>;
  /**
   * Whether there are more organizations to load
   */
  hasMore?: boolean;
  /**
   * Whether the initial data is loading
   */
  isLoading?: boolean;
  /**
   * Whether more data is being loaded
   */
  isLoadingMore?: boolean;
  /**
   * Display mode: 'inline' for normal display, 'popup' for modal dialog
   */
  mode?: 'inline' | 'popup';
  /**
   * List of organizations associated to the signed-in user.
   */
  myOrganizations: Organization[];
  /**
   * Function called when popup open state changes (only used in popup mode)
   */
  onOpenChange?: (open: boolean) => void;
  /**
   * Function called when an organization is selected/clicked
   */
  onOrganizationSelect?: (organization: OrganizationWithSwitchAccess) => void;
  /**
   * Function called when refresh is requested
   */
  onRefresh?: () => Promise<void>;
  /**
   * Whether the popup is open (only used in popup mode)
   */
  open?: boolean;
  /**
   * Component-level preferences to override global i18n and theme settings.
   * Preferences are deep-merged with global ones, with component preferences
   * taking precedence. Affects this component and all its descendants.
   */
  preferences?: Preferences;
  /**
   * Custom renderer for when no organizations are found
   */
  renderEmpty?: () => ReactNode;
  /**
   * Custom renderer for the error state
   */
  renderError?: (error: string) => ReactNode;
  /**
   * Custom renderer for the load more button
   */
  renderLoadMore?: (onLoadMore: () => Promise<void>, isLoading: boolean) => ReactNode;
  /**
   * Custom renderer for the loading state
   */
  renderLoading?: () => ReactNode;
  /**
   * Custom renderer for each organization item
   */
  renderOrganization?: (organization: OrganizationWithSwitchAccess, index: number) => ReactNode;
  /**
   * Whether to show the organization status in the list
   */
  showStatus?: boolean;
  /**
   * Inline styles to apply to the container
   */
  style?: CSSProperties;

  /**
   * Title for the popup dialog (only used in popup mode)
   */
  title?: string;
}

/**
 * Default organization item renderer
 */
const defaultRenderOrganization = (
  organization: OrganizationWithSwitchAccess,
  styles: any,
  t: (key: string, params?: Record<string, string | number>) => string,
  onOrganizationSelect?: (organization: OrganizationWithSwitchAccess) => void,
  showStatus?: boolean,
): ReactNode => (
  <div key={organization.id} className={cx(styles.organizationItem)}>
    <div className={cx(styles.organizationContent)}>
      <AvatarPrimitive variant="square" name={organization.name} size={48} alt={`${organization.name} logo`} />
      <div className={cx(styles.organizationInfo)}>
        <Typography variant="h6" className={cx(styles.organizationName)}>
          {organization.name}
        </Typography>
        <Typography variant="body2" color="textSecondary" className={cx(styles.organizationHandle)}>
          @{organization.orgHandle}
        </Typography>
        {showStatus && (
          <Typography variant="body2" color="textSecondary" className={cx(styles.organizationStatus)}>
            {t('organization.switcher.status.label')}{' '}
            <span
              className={cx(
                styles.statusText,
                organization.status === 'ACTIVE' ? styles.statusTextActive : styles.statusTextInactive,
              )}
            >
              {organization.status}
            </span>
          </Typography>
        )}
      </div>
    </div>
    {organization.canSwitch && (
      <div className={cx(styles.organizationActions)}>
        <Button
          onClick={(e: MouseEvent<HTMLButtonElement>): void => {
            e.stopPropagation();
            onOrganizationSelect(organization);
          }}
          type="button"
          size="small"
        >
          {t('organization.switcher.buttons.switch.text')}
        </Button>
      </div>
    )}
  </div>
);

/**
 * Default loading renderer
 */
const defaultRenderLoading = (
  t: (key: string, params?: Record<string, string | number>) => string,
  styles: any,
): ReactNode => (
  <div className={cx(styles.loadingContainer)}>
    <Spinner size="medium" />
    <Typography variant="body1" color="textSecondary" className={cx(styles.loadingText)}>
      {t('organization.switcher.loading.placeholder.organizations')}
    </Typography>
  </div>
);

/**
 * Default error renderer
 */
const defaultRenderError = (
  errorMessage: string,
  t: (key: string, params?: Record<string, string | number>) => string,
  styles: any,
): ReactNode => (
  <div className={cx(styles.errorContainer)}>
    <Typography variant="body1" color="error">
      <strong>{t('organization.switcher.error.prefix')}</strong> {errorMessage}
    </Typography>
  </div>
);

/**
 * Default load more button renderer
 */
const defaultRenderLoadMore = (
  onLoadMore: () => Promise<void>,
  isLoadingMore: boolean,
  t: (key: string, params?: Record<string, string | number>) => string,
  styles: any,
): ReactNode => (
  <Button onClick={onLoadMore} disabled={isLoadingMore} className={cx(styles.loadMoreButton)} type="button" fullWidth>
    {isLoadingMore ? t('organization.switcher.loading.more') : t('organization.switcher.buttons.load_more.text')}
  </Button>
);

/**
 * Default empty state renderer
 */
const defaultRenderEmpty = (
  t: (key: string, params?: Record<string, string | number>) => string,
  styles: any,
): ReactNode => (
  <div className={cx(styles.emptyContainer)}>
    <Typography variant="body1" color="textSecondary" className={cx(styles.emptyText)}>
      {t('organization.switcher.no.organizations')}
    </Typography>
  </div>
);

/**
 * BaseOrganizationList component displays a list of organizations with pagination support.
 * This component serves as the base for framework-specific implementations.
 *
 * @example
 * ```tsx
 * <BaseOrganizationList
 *   data={organizations}
 *   isLoading={isLoading}
 *   hasMore={hasMore}
 *   fetchMore={fetchMore}
 *   error={error}
 * />
 * ```
 */
export const BaseOrganizationList: FC<BaseOrganizationListProps> = ({
  className = '',
  allOrganizations,
  myOrganizations,
  error,
  fetchMore,
  hasMore = false,
  isLoading = false,
  isLoadingMore = false,
  mode = 'inline',
  onOpenChange,
  onOrganizationSelect,
  onRefresh,
  open = false,
  renderEmpty,
  renderError,
  renderLoading,
  renderLoadMore,
  renderOrganization,
  style,
  title = 'Organizations',
  showStatus,
  preferences,
}: BaseOrganizationListProps): ReactElement => {
  const {theme, colorScheme} = useTheme();
  const styles: ReturnType<typeof useStyles> = useStyles(theme, colorScheme);
  const {t} = useTranslation(preferences?.i18n);

  const organizationsWithSwitchAccess: OrganizationWithSwitchAccess[] = useMemo(() => {
    if (!allOrganizations?.organizations) {
      return [];
    }

    const myOrgIds = new Set<string>(myOrganizations?.map((org: Organization) => org.id) || []);

    return allOrganizations.organizations.map((org: Organization) => ({
      ...org,
      canSwitch: myOrgIds.has(org.id),
    }));
  }, [allOrganizations?.organizations, myOrganizations]);

  const renderLoadingWithStyles: () => ReactNode = renderLoading || ((): ReactNode => defaultRenderLoading(t, styles));
  const renderErrorWithStyles: (errorMsg: string) => ReactNode =
    renderError || ((errorMsg: string): ReactNode => defaultRenderError(errorMsg, t, styles));
  const renderEmptyWithStyles: () => ReactNode = renderEmpty || ((): ReactNode => defaultRenderEmpty(t, styles));
  const renderLoadMoreWithStyles: (onLoadMore: () => Promise<void>, loadingMore: boolean) => ReactNode =
    renderLoadMore ||
    ((onLoadMore: () => Promise<void>, loadingMore: boolean): ReactNode =>
      defaultRenderLoadMore(onLoadMore, loadingMore, t, styles));
  const renderOrganizationWithStyles: (org: OrganizationWithSwitchAccess, index: number) => ReactNode =
    renderOrganization ||
    ((org: OrganizationWithSwitchAccess): ReactNode =>
      defaultRenderOrganization(org, styles, t, onOrganizationSelect, showStatus));

  if (isLoading && organizationsWithSwitchAccess?.length === 0) {
    const loadingContent: ReactElement = (
      <div className={cx(styles['root'], className)} style={style}>
        {renderLoadingWithStyles()}
      </div>
    );

    if (mode === 'popup') {
      return (
        <DialogPrimitive open={open} onOpenChange={onOpenChange}>
          <DialogPrimitive.Content>
            <DialogPrimitive.Heading>{title}</DialogPrimitive.Heading>
            <div className={cx(styles['popupContent'])}>{loadingContent}</div>
          </DialogPrimitive.Content>
        </DialogPrimitive>
      );
    }

    return loadingContent;
  }

  if (error && organizationsWithSwitchAccess?.length === 0) {
    const errorContent: ReactElement = (
      <div className={cx(styles['root'], className)} style={style}>
        {renderErrorWithStyles(error)}
      </div>
    );

    if (mode === 'popup') {
      return (
        <DialogPrimitive open={open} onOpenChange={onOpenChange}>
          <DialogPrimitive.Content>
            <DialogPrimitive.Heading>{title}</DialogPrimitive.Heading>
            <div className={cx(styles['popupContent'])}>{errorContent}</div>
          </DialogPrimitive.Content>
        </DialogPrimitive>
      );
    }

    return errorContent;
  }

  if (!isLoading && organizationsWithSwitchAccess?.length === 0) {
    const emptyContent: ReactElement = (
      <div className={cx(styles['root'], className)} style={style}>
        {renderEmptyWithStyles()}
      </div>
    );

    if (mode === 'popup') {
      return (
        <DialogPrimitive open={open} onOpenChange={onOpenChange}>
          <DialogPrimitive.Content>
            <DialogPrimitive.Heading>{title}</DialogPrimitive.Heading>
            <div className={cx(styles['popupContent'])}>{emptyContent}</div>
          </DialogPrimitive.Content>
        </DialogPrimitive>
      );
    }

    return emptyContent;
  }

  const organizationListContent: ReactElement = (
    <div className={cx(styles['root'], className)} style={style}>
      {/* Header with total count and refresh button */}
      <div className={cx(styles['header'])}>
        <div className={cx(styles['headerInfo'])}>
          <Typography variant="body2" color="textSecondary" className={cx(styles['subtitle'])}>
            {t('organization.switcher.showing.count', {
              showing: organizationsWithSwitchAccess?.length,
              total: allOrganizations?.organizations?.length || 0,
            })}
          </Typography>
        </div>
        {onRefresh && (
          <Button
            onClick={onRefresh}
            className={cx(styles['refreshButton'])}
            type="button"
            variant="outline"
            size="small"
          >
            {t('organization.switcher.buttons.refresh.text')}
          </Button>
        )}
      </div>

      {/* Organizations list */}
      <div className={cx(styles['listContainer'])}>
        {organizationsWithSwitchAccess?.map((organization: OrganizationWithSwitchAccess, index: number) =>
          renderOrganizationWithStyles(organization, index),
        )}
      </div>

      {/* Error message for additional data */}
      {error && organizationsWithSwitchAccess?.length > 0 && (
        <div className={cx(styles['errorMargin'])}>{renderErrorWithStyles(error)}</div>
      )}

      {/* Load more button */}
      {hasMore && fetchMore && (
        <div className={cx(styles['loadMoreMargin'])}>{renderLoadMoreWithStyles(fetchMore, isLoadingMore)}</div>
      )}
    </div>
  );

  if (mode === 'popup') {
    return (
      <DialogPrimitive open={open} onOpenChange={onOpenChange}>
        <DialogPrimitive.Content>
          <DialogPrimitive.Heading>{title}</DialogPrimitive.Heading>
          <div className={cx(styles['popupContent'])}>{organizationListContent}</div>
        </DialogPrimitive.Content>
      </DialogPrimitive>
    );
  }

  return organizationListContent;
};

export default BaseOrganizationList;
