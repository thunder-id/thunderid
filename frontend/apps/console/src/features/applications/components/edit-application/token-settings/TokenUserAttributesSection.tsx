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

import {SettingsCard} from '@thunderid/components';
import {
  Box,
  Stack,
  Typography,
  Chip,
  Alert,
  Tooltip,
  Grid,
  Tab,
  Tabs,
  FormControlLabel,
  Switch,
  Card,
  CardContent,
} from '@wso2/oxygen-ui';
import type React from 'react';
import {useTranslation} from 'react-i18next';
import JwtPreview from './JwtPreview';
import TokenConstants from '../../../constants/token-constants';

/**
 * Props for the {@link TokenUserAttributesSection} component.
 */
interface TokenUserAttributesSectionProps {
  /**
   * Array of all available user attributes from schemas
   */
  userAttributes: string[];
  /**
   * Loading state for user attributes fetch
   */
  isLoadingUserAttributes: boolean;
  /**
   * Set of attributes pending addition (visual feedback)
   */
  pendingAdditions: Set<string>;
  /**
   * Set of attributes pending removal (visual feedback)
   */
  pendingRemovals: Set<string>;
  /**
   * Set of attributes to highlight in the preview
   */
  highlightedAttributes: Set<string>;
  /**
   * Callback function when an attribute chip is clicked
   * @param attr - The attribute name
   * @param tokenType - The token type being modified
   */
  onAttributeClick: (attr: string, tokenType: 'shared' | 'access' | 'id' | 'userinfo') => void;
  // --- OAuth tabbed mode (all three present when using OAuth/OIDC) ---
  /**
   * Current access token user attributes (OAuth mode)
   */
  accessTokenAttributes?: string[];
  /**
   * Current ID token user attributes (OAuth mode)
   */
  idTokenAttributes?: string[];
  /**
   * Current User Info endpoint attributes (OAuth mode)
   */
  userInfoAttributes?: string[];
  /**
   * Currently active tab in OAuth mode
   */
  activeTab?: 'access' | 'id' | 'userinfo';
  /**
   * Callback when the active tab changes
   */
  onTabChange?: (tab: 'access' | 'id' | 'userinfo') => void;
  /**
   * Whether User Info uses custom attributes (vs inheriting from ID token)
   */
  isUserInfoCustomAttributes?: boolean;
  /**
   * Callback to toggle User Info custom attributes mode
   */
  onToggleUserInfo?: (checked: boolean) => void;
  // --- Native / single-token mode ---
  /**
   * Shared token user attributes (native mode)
   */
  sharedAttributes?: string[];
  /**
   * Singular noun used to refer to the entity in user-visible copy (default: 'application').
   */
  entityLabel?: string;
  /**
   * Whether inputs should be disabled (e.g. read-only resource).
   */
  disabled?: boolean;
}

/**
 * Section component for managing user attributes in JWT tokens.
 *
 * Renders in one of two layouts depending on the mode:
 * - **OAuth mode** (`accessTokenAttributes` provided): a single `SettingsCard` containing
 *   three MUI tabs — Access Token, ID Token, and User Info Endpoint — each with a
 *   two-column JWT preview + attribute-selection layout. The ID Token tab also shows
 *   the configured OAuth2 scopes. The User Info tab includes a toggle to either inherit
 *   attributes from the ID token or configure them independently.
 * - **Native mode** (`sharedAttributes` provided): a single-panel layout with no tabs,
 *   reusing the same two-column JWT preview + attribute-selection layout.
 *
 * @param props - Component props
 * @returns User attributes configuration UI within a SettingsCard
 */
export default function TokenUserAttributesSection({
  userAttributes,
  isLoadingUserAttributes,
  pendingAdditions,
  pendingRemovals,
  highlightedAttributes,
  onAttributeClick,
  accessTokenAttributes = undefined,
  idTokenAttributes = undefined,
  userInfoAttributes = undefined,
  activeTab = 'access',
  onTabChange = undefined,
  isUserInfoCustomAttributes = false,
  onToggleUserInfo = undefined,
  sharedAttributes = undefined,
  entityLabel = 'application',
  disabled = false,
}: TokenUserAttributesSectionProps) {
  const {t} = useTranslation();

  const isOAuthMode = accessTokenAttributes !== undefined;

  /**
   * Build the JWT/JSON preview object for a given token type.
   */
  const buildPreview = (
    currentAttrs: string[],
    tokenType: 'shared' | 'access' | 'id' | 'userinfo',
  ): Record<string, string> => {
    const preview: Record<string, string> = {};
    const defaultAttrs =
      tokenType === 'userinfo' ? TokenConstants.USER_INFO_DEFAULT_ATTRIBUTES : TokenConstants.DEFAULT_TOKEN_ATTRIBUTES;

    defaultAttrs.forEach((attr) => {
      preview[attr] = `<${attr}>`;
    });

    const isPendingTab = tokenType === 'shared' || activeTab === tokenType;

    currentAttrs.forEach((attr) => {
      if (!(pendingRemovals.has(attr) && isPendingTab)) {
        preview[attr] = `<${attr}>`;
      }
    });

    if (isPendingTab) {
      pendingAdditions.forEach((attr) => {
        preview[attr] = `<${attr}>`;
      });
    }

    return preview;
  };

  /**
   * Shared two-column layout: JWT preview on the left, attribute selection on the right.
   */
  const renderAttributePanel = (
    currentAttrs: string[],
    tokenType: 'shared' | 'access' | 'id' | 'userinfo',
    previewTitle: string,
  ) => {
    const defaultAttrs =
      tokenType === 'userinfo' ? TokenConstants.USER_INFO_DEFAULT_ATTRIBUTES : TokenConstants.DEFAULT_TOKEN_ATTRIBUTES;

    const jwtPreview = buildPreview(currentAttrs, tokenType);
    const isPendingTab = tokenType === 'shared' || activeTab === tokenType;

    const availableAttributes = Array.from(
      new Set([...userAttributes, ...TokenConstants.ADDITIONAL_USER_ATTRIBUTES]),
    ).filter((attr) => !(defaultAttrs as readonly string[]).includes(attr));

    return (
      <Grid container spacing={3}>
        {/* Left Column - JWT Preview */}
        <Grid size={{xs: 12, md: 6}}>
          <Stack spacing={2}>
            <Box>
              <Typography variant="body1" sx={{mb: 1}}>
                {t('applications:edit.token.configure_attributes', 'Add or Remove Attributes')}
              </Typography>
              <Typography variant="body2" color="text.disabled" sx={{mb: 2}}>
                {t(
                  'applications:edit.token.configure_attributes.hint',
                  'Click on user attributes to add them to your token.',
                )}
              </Typography>

              <Card>
                <CardContent>
                  {isLoadingUserAttributes && (
                    <Typography variant="body2" color="text.secondary">
                      {t('applications:edit.token.loading_attributes', 'Loading user attributes...')}
                    </Typography>
                  )}
                  {!isLoadingUserAttributes && userAttributes.length > 0 && (
                    <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
                      {availableAttributes.sort().map((attr) => {
                        const isAdded = currentAttrs.includes(attr);
                        const isPendingAddition = pendingAdditions.has(attr) && isPendingTab;
                        const isPendingRemoval = pendingRemovals.has(attr) && isPendingTab;
                        const isHighlighted = highlightedAttributes.has(attr);
                        const isActive = (isAdded && !isPendingRemoval) || isPendingAddition;

                        return (
                          <Tooltip
                            key={attr}
                            title={
                              isActive
                                ? t('applications:edit.token.click_to_remove', 'Click to remove')
                                : t('applications:edit.token.click_to_add', 'Click to add')
                            }
                          >
                            <Chip
                              label={attr}
                              size="small"
                              variant={isActive ? 'filled' : 'outlined'}
                              color={isActive ? 'primary' : 'default'}
                              onClick={disabled ? undefined : () => onAttributeClick(attr, tokenType)}
                              sx={{
                                cursor: 'pointer',
                                transition: 'all 0.3s ease',
                                transform: isHighlighted ? 'scale(1.05)' : 'scale(1)',
                                boxShadow: isHighlighted ? '0 0 0 2px rgba(25, 118, 210, 0.4)' : 'none',
                                '&:hover': {backgroundColor: 'action.hover'},
                              }}
                            />
                          </Tooltip>
                        );
                      })}
                    </Stack>
                  )}
                  {!isLoadingUserAttributes && userAttributes.length === 0 && (
                    <Alert severity="info">
                      {t(
                        'applications:edit.token.no_user_attributes',
                        'No user attributes available. Configure allowed user types for this {{entity}}.',
                        {entity: entityLabel},
                      )}
                    </Alert>
                  )}
                </CardContent>
              </Card>
            </Box>
          </Stack>
        </Grid>

        {/* Right Column - Attribute Selection */}
        <Grid size={{xs: 12, md: 6}}>
          <JwtPreview title={previewTitle} payload={jwtPreview} defaultClaims={defaultAttrs} />
        </Grid>
      </Grid>
    );
  };

  const cardTitle = t('applications:edit.token.user_attributes_card.title', 'User Attributes');
  const cardDescription = t(
    'applications:edit.token.user_attributes_card.description',
    'Configure the user attributes to include in your tokens & user info response',
  );
  const previewTitle = t('applications:edit.token.token_preview.title', 'Decoded Payload');

  if (isOAuthMode) {
    let tabIndex = 2;

    if (activeTab === 'access') {
      tabIndex = 0;
    } else if (activeTab === 'id') {
      tabIndex = 1;
    }

    return (
      <SettingsCard slotProps={{content: {sx: {p: 0}}}} title={cardTitle} description={cardDescription}>
        <Stack spacing={3}>
          <Tabs
            value={tabIndex}
            onChange={(_, newValue: number) => {
              const tabs: ('access' | 'id' | 'userinfo')[] = ['access', 'id', 'userinfo'];
              onTabChange?.(tabs[newValue]);
            }}
            sx={{borderBottom: 1, borderColor: 'divider'}}
          >
            <Tab label={t('applications:edit.token.tabs.access_token', 'Access Token')} />
            <Tab label={t('applications:edit.token.tabs.id_token', 'ID Token')} />
            <Tab label={t('applications:edit.token.tabs.user_info_endpoint', 'User Info Endpoint')} />
          </Tabs>

          <Box sx={{p: 3}}>
            {/* Access Token Tab Panel */}
            {activeTab === 'access' && (
              <Box>{renderAttributePanel(accessTokenAttributes ?? [], 'access', previewTitle)}</Box>
            )}

            {/* ID Token Tab Panel */}
            {activeTab === 'id' && <Box>{renderAttributePanel(idTokenAttributes ?? [], 'id', previewTitle)}</Box>}

            {/* User Info Endpoint Tab Panel */}
            {activeTab === 'userinfo' && (
              <Box>
                <FormControlLabel
                  control={
                    <Switch
                      checked={!isUserInfoCustomAttributes}
                      onChange={(e: React.ChangeEvent<HTMLInputElement>) => onToggleUserInfo?.(!e.target.checked)}
                      name="userinfo-inherit"
                      size="small"
                      disabled={disabled}
                    />
                  }
                  label={
                    <Box sx={{ml: 0.5}}>
                      <Typography variant="body2" fontWeight={500}>
                        {t('applications:edit.token.inherit_from_id_token', 'Use same attributes as ID Token')}
                      </Typography>
                      <Typography variant="caption" color="text.secondary">
                        {t(
                          'applications:edit.token.user_info.inherit_hint',
                          'When enabled, the User Info endpoint returns the same attributes configured for the ID Token',
                        )}
                      </Typography>
                    </Box>
                  }
                  sx={{mb: 3, alignItems: 'center'}}
                />
                {isUserInfoCustomAttributes ? (
                  renderAttributePanel(
                    userInfoAttributes ?? [],
                    'userinfo',
                    t('applications:edit.token.token_preview.title', 'Decoded Payload'),
                  )
                ) : (
                  <Box sx={{opacity: 0.45, pointerEvents: 'none', userSelect: 'none'}}>
                    {renderAttributePanel(
                      idTokenAttributes ?? [],
                      'userinfo',
                      t('applications:edit.token.token_preview.title', 'Decoded Payload'),
                    )}
                  </Box>
                )}
              </Box>
            )}
          </Box>
        </Stack>
      </SettingsCard>
    );
  }

  // Native mode (shared token)
  return (
    <SettingsCard title={cardTitle} description={cardDescription}>
      {renderAttributePanel(sharedAttributes ?? [], 'shared', previewTitle)}
    </SettingsCard>
  );
}
