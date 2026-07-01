/**
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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
  Grid,
  Tab,
  Tabs,
  FormControlLabel,
  Switch,
  Card,
  CardContent,
  Select,
  MenuItem,
  FormControl,
  FormLabel,
  Divider,
} from '@wso2/oxygen-ui';
import type React from 'react';
import {useTranslation} from 'react-i18next';
import JwtPreview from './JwtPreview';
import TokenConstants from '../../../constants/token-constants';
import type {IDTokenResponseType, UserInfoResponseType} from '../../../models/oauth';

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
  /**
   * Current ID token response type (OAuth mode)
   */
  idTokenResponseType?: IDTokenResponseType;
  /**
   * Current ID token encryption key-management algorithm (OAuth mode)
   */
  idTokenEncryptionAlg?: string;
  /**
   * Current ID token encryption content algorithm (OAuth mode)
   */
  idTokenEncryptionEnc?: string;
  /**
   * Callback when an ID token config field changes
   */
  onIdTokenConfigChange?: (field: string, value: string) => void;
  /**
   * Current UserInfo response type (OAuth mode)
   */
  userInfoResponseType?: UserInfoResponseType;
  /**
   * Current UserInfo signing algorithm (OAuth mode)
   */
  userInfoSigningAlg?: string;
  /**
   * Current UserInfo encryption key-management algorithm (OAuth mode)
   */
  userInfoEncryptionAlg?: string;
  /**
   * Current UserInfo encryption content algorithm (OAuth mode)
   */
  userInfoEncryptionEnc?: string;
  /**
   * Callback when a UserInfo config field changes
   */
  onUserInfoConfigChange?: (field: string, value: string) => void;
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
  idTokenResponseType = undefined,
  idTokenEncryptionAlg = undefined,
  idTokenEncryptionEnc = undefined,
  onIdTokenConfigChange = undefined,
  userInfoResponseType = undefined,
  userInfoSigningAlg = undefined,
  userInfoEncryptionAlg = undefined,
  userInfoEncryptionEnc = undefined,
  onUserInfoConfigChange = undefined,
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
   * Build a JOSE header preview for the ID token based on its response type.
   */
  const buildIdTokenHeader = (): Record<string, string> | undefined => {
    const responseType = idTokenResponseType ?? 'JWT';
    if (responseType === 'JWT') {
      return {alg: 'RS256', kid: '<key_id>', typ: 'JWT'};
    }
    if (responseType === 'JWE') {
      return {
        alg: idTokenEncryptionAlg ?? '<encryption_alg>',
        enc: idTokenEncryptionEnc ?? '<encryption_enc>',
        kid: '<key_id>',
        typ: 'JWT',
      };
    }
    // NESTED_JWT: sign-then-encrypt outer header
    return {
      alg: idTokenEncryptionAlg ?? '<encryption_alg>',
      enc: idTokenEncryptionEnc ?? '<encryption_enc>',
      kid: '<key_id>',
      cty: 'JWT',
      typ: 'JWT',
    };
  };

  /**
   * Build a JOSE header preview for the UserInfo response based on its response type.
   */
  const buildUserInfoHeader = (): Record<string, string> | undefined => {
    const responseType = userInfoResponseType ?? 'JSON';
    if (responseType === 'JSON') return undefined;
    if (responseType === 'JWS') {
      return {alg: userInfoSigningAlg ?? '<signing_alg>', kid: '<key_id>', typ: 'JWT'};
    }
    if (responseType === 'JWE') {
      return {
        alg: userInfoEncryptionAlg ?? '<encryption_alg>',
        enc: userInfoEncryptionEnc ?? '<encryption_enc>',
        kid: '<key_id>',
        typ: 'JWT',
      };
    }
    // NESTED_JWT: sign-then-encrypt outer header
    return {
      alg: userInfoEncryptionAlg ?? '<encryption_alg>',
      enc: userInfoEncryptionEnc ?? '<encryption_enc>',
      kid: '<key_id>',
      cty: 'JWT',
      typ: 'JWT',
    };
  };

  /**
   * Renders the attribute chip selector (left column content).
   */
  const renderAttributeChips = (currentAttrs: string[], tokenType: 'shared' | 'access' | 'id' | 'userinfo') => {
    const defaultAttrs =
      tokenType === 'userinfo' ? TokenConstants.USER_INFO_DEFAULT_ATTRIBUTES : TokenConstants.DEFAULT_TOKEN_ATTRIBUTES;
    const isPendingTab = tokenType === 'shared' || activeTab === tokenType;

    const availableAttributes = Array.from(
      new Set([...userAttributes, ...TokenConstants.ADDITIONAL_USER_ATTRIBUTES]),
    ).filter((attr) => !(defaultAttrs as readonly string[]).includes(attr));

    return (
      <Box>
        <Typography variant="body2" sx={{mb: 1}}>
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
                    <Chip
                      key={attr}
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
    );
  };

  /**
   * Two-column layout: attribute selection on the left, JWT preview on the right.
   */
  const renderAttributePanel = (currentAttrs: string[], tokenType: 'shared' | 'access' | 'id' | 'userinfo') => {
    const defaultAttrs =
      tokenType === 'userinfo' ? TokenConstants.USER_INFO_DEFAULT_ATTRIBUTES : TokenConstants.DEFAULT_TOKEN_ATTRIBUTES;
    const jwtPreview = buildPreview(currentAttrs, tokenType);

    return (
      <Grid container spacing={3}>
        <Grid size={{xs: 12, md: 7}}>{renderAttributeChips(currentAttrs, tokenType)}</Grid>
        <Grid size={{xs: 12, md: 5}}>
          <JwtPreview payload={jwtPreview} defaultClaims={defaultAttrs} />
        </Grid>
      </Grid>
    );
  };

  const cardTitle = t('applications:edit.token.token_profile_card.title', 'Token Attributes & Response');
  const cardDescription = t(
    'applications:edit.token.token_profile_card.description',
    'Configure the response types and user attributes included in your tokens and user info responses',
  );
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
            {activeTab === 'access' && <Box>{renderAttributePanel(accessTokenAttributes ?? [], 'access')}</Box>}

            {/* ID Token Tab Panel */}
            {activeTab === 'id' &&
              (() => {
                const idAttrs = idTokenAttributes ?? [];
                const defaultAttrs = TokenConstants.DEFAULT_TOKEN_ATTRIBUTES;
                const jwtPreview = buildPreview(idAttrs, 'id');

                return (
                  <Grid container spacing={3}>
                    {/* Left Column - Attributes + Response Format */}
                    <Grid size={{xs: 12, md: 7}}>
                      <Stack spacing={3}>
                        {renderAttributeChips(idAttrs, 'id')}
                        <Divider />
                        {/* Response Format */}
                        <Box>
                          <Typography variant="subtitle2" sx={{mb: 1}}>
                            {t('applications:edit.token.id_token.response_format_heading', 'Response Format')}
                          </Typography>
                          <Typography variant="body2" color="text.disabled" sx={{mb: 2}}>
                            {t(
                              'applications:edit.token.id_token.response_format_hint',
                              'Configure the format and encryption of the ID token response.',
                            )}
                          </Typography>
                          <Stack spacing={2}>
                            {/* Row 1: Response Type (full width) */}
                            <FormControl size="small" fullWidth>
                              <FormLabel>
                                {t('applications:edit.token.id_token.response_type', 'Response Type')}
                              </FormLabel>
                              <Select
                                displayEmpty
                                value={(idTokenResponseType ?? '') as string}
                                onChange={(e) => onIdTokenConfigChange?.('responseType', String(e.target.value))}
                                disabled={disabled}
                                renderValue={(selected) =>
                                  !selected ? (
                                    <Typography color="text.secondary" variant="body2">
                                      {t('applications:edit.token.id_token.response_type_placeholder')}
                                    </Typography>
                                  ) : (
                                    selected
                                  )
                                }
                              >
                                {TokenConstants.ID_TOKEN_RESPONSE_TYPES.map((type) => (
                                  <MenuItem key={type} value={type}>
                                    {type}
                                  </MenuItem>
                                ))}
                              </Select>
                            </FormControl>

                            {/* Row 2: Encryption fields */}
                            {(idTokenResponseType === 'JWE' || idTokenResponseType === 'NESTED_JWT') && (
                              <Stack direction="row" spacing={2} flexWrap="wrap" useFlexGap>
                                <FormControl size="small" sx={{flex: 1, minWidth: 140}}>
                                  <FormLabel>
                                    {t('applications:edit.token.id_token.encryption_alg', 'Encryption Algorithm')}
                                  </FormLabel>
                                  <Select
                                    displayEmpty
                                    value={idTokenEncryptionAlg ?? ''}
                                    onChange={(e) => onIdTokenConfigChange?.('encryptionAlg', e.target.value)}
                                    disabled={disabled}
                                    renderValue={(selected) =>
                                      !selected ? (
                                        <Typography color="text.secondary" variant="body2">
                                          {t('applications:edit.token.id_token.encryption_alg_placeholder')}
                                        </Typography>
                                      ) : (
                                        selected
                                      )
                                    }
                                  >
                                    {TokenConstants.ID_TOKEN_ENCRYPTION_ALGS.map((alg) => (
                                      <MenuItem key={alg} value={alg}>
                                        {alg}
                                      </MenuItem>
                                    ))}
                                  </Select>
                                </FormControl>

                                <FormControl size="small" sx={{flex: 1, minWidth: 140}}>
                                  <FormLabel>
                                    {t('applications:edit.token.id_token.encryption_enc', 'Content Encryption')}
                                  </FormLabel>
                                  <Select
                                    displayEmpty
                                    value={idTokenEncryptionEnc ?? ''}
                                    onChange={(e) => onIdTokenConfigChange?.('encryptionEnc', e.target.value)}
                                    disabled={disabled}
                                    renderValue={(selected) =>
                                      !selected ? (
                                        <Typography color="text.secondary" variant="body2">
                                          {t('applications:edit.token.id_token.encryption_enc_placeholder')}
                                        </Typography>
                                      ) : (
                                        selected
                                      )
                                    }
                                  >
                                    {TokenConstants.ID_TOKEN_ENCRYPTION_ENCS.map((enc) => (
                                      <MenuItem key={enc} value={enc}>
                                        {enc}
                                      </MenuItem>
                                    ))}
                                  </Select>
                                </FormControl>
                              </Stack>
                            )}
                          </Stack>
                        </Box>
                      </Stack>
                    </Grid>

                    {/* Right Column - JWT Preview */}
                    <Grid size={{xs: 12, md: 5}}>
                      <JwtPreview payload={jwtPreview} defaultClaims={defaultAttrs} header={buildIdTokenHeader()} />
                    </Grid>
                  </Grid>
                );
              })()}

            {/* User Info Endpoint Tab Panel */}
            {activeTab === 'userinfo' &&
              (() => {
                const effectiveAttrs = isUserInfoCustomAttributes
                  ? (userInfoAttributes ?? [])
                  : (idTokenAttributes ?? []);
                const defaultAttrs = TokenConstants.USER_INFO_DEFAULT_ATTRIBUTES;
                const jwtPreview = buildPreview(effectiveAttrs, 'userinfo');

                return (
                  <Grid container spacing={3}>
                    {/* Left Column - Attributes + Response Format */}
                    <Grid size={{xs: 12, md: 7}}>
                      <Stack spacing={3}>
                        {/* User Attributes */}
                        <Box>
                          <FormControlLabel
                            control={
                              <Switch
                                checked={!isUserInfoCustomAttributes}
                                onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
                                  onToggleUserInfo?.(!e.target.checked)
                                }
                                name="userinfo-inherit"
                                size="small"
                                disabled={disabled}
                              />
                            }
                            label={
                              <Box sx={{ml: 0.5}}>
                                <Typography variant="body2" fontWeight={500}>
                                  {t(
                                    'applications:edit.token.inherit_from_id_token',
                                    'Use same attributes as ID Token',
                                  )}
                                </Typography>
                                <Typography variant="caption" color="text.secondary">
                                  {t(
                                    'applications:edit.token.user_info.inherit_hint',
                                    'When enabled, the User Info endpoint returns the same attributes configured for the ID Token',
                                  )}
                                </Typography>
                              </Box>
                            }
                            sx={{mb: 2, alignItems: 'center'}}
                          />
                          {isUserInfoCustomAttributes ? (
                            renderAttributeChips(userInfoAttributes ?? [], 'userinfo')
                          ) : (
                            <Box sx={{opacity: 0.45, pointerEvents: 'none', userSelect: 'none'}}>
                              {renderAttributeChips(idTokenAttributes ?? [], 'userinfo')}
                            </Box>
                          )}
                        </Box>
                        <Divider />
                        {/* Response Format */}
                        <Box>
                          <Typography variant="subtitle2" sx={{mb: 1}}>
                            {t('applications:edit.token.user_info.response_format_heading', 'Response Format')}
                          </Typography>
                          <Typography variant="body2" color="text.disabled" sx={{mb: 2}}>
                            {t(
                              'applications:edit.token.user_info.response_format_hint',
                              'Configure the format and security of the User Info endpoint response.',
                            )}
                          </Typography>
                          <Stack spacing={2}>
                            {/* Row 1: Response Type (full width) */}
                            <FormControl size="small" fullWidth>
                              <FormLabel>
                                {t('applications:edit.token.user_info.response_type', 'Response Type')}
                              </FormLabel>
                              <Select
                                displayEmpty
                                value={(userInfoResponseType ?? '') as string}
                                onChange={(e) => onUserInfoConfigChange?.('responseType', String(e.target.value))}
                                disabled={disabled}
                                renderValue={(selected) =>
                                  !selected ? (
                                    <Typography color="text.secondary" variant="body2">
                                      {t('applications:edit.token.user_info.response_type_placeholder')}
                                    </Typography>
                                  ) : (
                                    selected
                                  )
                                }
                              >
                                {TokenConstants.USER_INFO_RESPONSE_TYPES.map((type) => (
                                  <MenuItem key={type} value={type}>
                                    {type}
                                  </MenuItem>
                                ))}
                              </Select>
                            </FormControl>

                            {/* Row 2: Algorithm fields */}
                            {userInfoResponseType && userInfoResponseType !== 'JSON' && (
                              <Stack direction="row" spacing={2} flexWrap="wrap" useFlexGap>
                                {(userInfoResponseType === 'JWS' || userInfoResponseType === 'NESTED_JWT') && (
                                  <FormControl size="small" sx={{flex: 1, minWidth: 140}}>
                                    <FormLabel>
                                      {t('applications:edit.token.user_info.signing_alg', 'Signing Algorithm')}
                                    </FormLabel>
                                    <Select
                                      displayEmpty
                                      value={userInfoSigningAlg ?? ''}
                                      onChange={(e) => onUserInfoConfigChange?.('signingAlg', e.target.value)}
                                      disabled={disabled}
                                      renderValue={(selected) =>
                                        !selected ? (
                                          <Typography color="text.secondary" variant="body2">
                                            {t('applications:edit.token.user_info.signing_alg_placeholder')}
                                          </Typography>
                                        ) : (
                                          selected
                                        )
                                      }
                                    >
                                      {TokenConstants.USER_INFO_SIGNING_ALGS.map((alg) => (
                                        <MenuItem key={alg} value={alg}>
                                          {alg}
                                        </MenuItem>
                                      ))}
                                    </Select>
                                  </FormControl>
                                )}

                                {(userInfoResponseType === 'JWE' || userInfoResponseType === 'NESTED_JWT') && (
                                  <>
                                    <FormControl size="small" sx={{flex: 1, minWidth: 140}}>
                                      <FormLabel>
                                        {t('applications:edit.token.user_info.encryption_alg', 'Encryption Algorithm')}
                                      </FormLabel>
                                      <Select
                                        displayEmpty
                                        value={userInfoEncryptionAlg ?? ''}
                                        onChange={(e) => onUserInfoConfigChange?.('encryptionAlg', e.target.value)}
                                        disabled={disabled}
                                        renderValue={(selected) =>
                                          !selected ? (
                                            <Typography color="text.secondary" variant="body2">
                                              {t('applications:edit.token.user_info.encryption_alg_placeholder')}
                                            </Typography>
                                          ) : (
                                            selected
                                          )
                                        }
                                      >
                                        {TokenConstants.USER_INFO_ENCRYPTION_ALGS.map((alg) => (
                                          <MenuItem key={alg} value={alg}>
                                            {alg}
                                          </MenuItem>
                                        ))}
                                      </Select>
                                    </FormControl>

                                    <FormControl size="small" sx={{flex: 1, minWidth: 140}}>
                                      <FormLabel>
                                        {t('applications:edit.token.user_info.encryption_enc', 'Content Encryption')}
                                      </FormLabel>
                                      <Select
                                        displayEmpty
                                        value={userInfoEncryptionEnc ?? ''}
                                        onChange={(e) => onUserInfoConfigChange?.('encryptionEnc', e.target.value)}
                                        disabled={disabled}
                                        renderValue={(selected) =>
                                          !selected ? (
                                            <Typography color="text.secondary" variant="body2">
                                              {t('applications:edit.token.user_info.encryption_enc_placeholder')}
                                            </Typography>
                                          ) : (
                                            selected
                                          )
                                        }
                                      >
                                        {TokenConstants.USER_INFO_ENCRYPTION_ENCS.map((enc) => (
                                          <MenuItem key={enc} value={enc}>
                                            {enc}
                                          </MenuItem>
                                        ))}
                                      </Select>
                                    </FormControl>
                                  </>
                                )}
                              </Stack>
                            )}
                          </Stack>
                        </Box>
                      </Stack>
                    </Grid>

                    {/* Right Column - JWT/JSON Preview */}
                    <Grid size={{xs: 12, md: 5}}>
                      <JwtPreview
                        payload={jwtPreview}
                        defaultClaims={defaultAttrs}
                        format={userInfoResponseType === 'JSON' ? 'json' : 'jwt'}
                        header={userInfoResponseType !== 'JSON' ? buildUserInfoHeader() : undefined}
                      />
                    </Grid>
                  </Grid>
                );
              })()}
          </Box>
        </Stack>
      </SettingsCard>
    );
  }

  // Native mode (shared token)
  return (
    <SettingsCard title={cardTitle} description={cardDescription}>
      {renderAttributePanel(sharedAttributes ?? [], 'shared')}
    </SettingsCard>
  );
}
