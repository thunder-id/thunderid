// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
  Accordion,
  AccordionDetails,
  AccordionSummary,
} from '@wso2/oxygen-ui';
import {ChevronDown} from '@wso2/oxygen-ui-icons-react';
import type React from 'react';
import type {ReactNode} from 'react';
import {useState} from 'react';
import {useTranslation} from 'react-i18next';
import JwtPreview from './JwtPreview';
import TokenConstants from '../../../constants/token-constants';
import type {IDTokenResponseType, UserInfoResponseType} from '@thunderid/configure-applications';

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
   * The algorithm the deployment signs tokens with, from the OIDC discovery document. Shown
   * read-only because signing always uses the server key and is not a per-application choice.
   */
  signingAlg?: string;
  /**
   * Whether an OAuth client certificate is configured on the application. Encrypted response
   * formats (JWE, NESTED_JWT) require one, so they are disabled in the format dropdowns when false.
   */
  hasCertificate?: boolean;
  /**
   * Name of the tab where the OAuth client certificate is configured, used in the
   * certificate-required hint. Applications use "Advanced"; agents use "Credentials".
   */
  certificateLocation?: string;
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
  /**
   * Whether to show the "User Info Endpoint" tab (OAuth mode only). Defaults to true;
   * agents hide this tab since they don't expose a userinfo endpoint of their own.
   */
  showUserInfoTab?: boolean;
  /**
   * Whether the access token preview should include the RFC 8693 `act` (actor) claim.
   * The backend always adds this claim to access tokens issued to an agent acting on
   * behalf of a user, so agents pass true; applications only get it when they've opted
   * in to `IncludeActClaim`, which isn't exposed in this UI, so they default to false.
   */
  showActorClaim?: boolean;
  /**
   * Value shown for `act.sub` in the actor claim preview (the acting agent's ID).
   */
  actorSub?: string;
  /**
   * Scope-to-claims mapping UI, rendered once beneath the ID Token and User Info tab panels.
   * The mapping feeds both, so it is a single shared instance rather than a copy per tab, and it
   * is not shown on the Access Token tab, whose attributes are not scope-driven.
   */
  scopeMapping?: ReactNode;
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
  signingAlg = undefined,
  hasCertificate = false,
  certificateLocation = 'Advanced',
  idTokenResponseType = undefined,
  idTokenEncryptionAlg = undefined,
  idTokenEncryptionEnc = undefined,
  onIdTokenConfigChange = undefined,
  userInfoResponseType = undefined,
  userInfoEncryptionAlg = undefined,
  userInfoEncryptionEnc = undefined,
  onUserInfoConfigChange = undefined,
  showUserInfoTab = true,
  showActorClaim = false,
  actorSub = '<agent-id>',
  scopeMapping = undefined,
}: TokenUserAttributesSectionProps) {
  const {t} = useTranslation();
  // Which of the two sinks the combined tab was last showing, so switching away to Access Token
  // and back returns to the same one.
  const [lastUserSinkTab, setLastUserSinkTab] = useState<'id' | 'userinfo'>('id');

  const isOAuthMode = accessTokenAttributes !== undefined;

  /**
   * Friendly label and one-line description for a response-format value. The dropdown values are
   * the raw JOSE format identifiers sent to the backend; these translations make them readable.
   * Signing is always done with the server's signing key, so there is no signing-algorithm choice.
   */
  const responseTypeOption = (
    section: 'id_token' | 'user_info',
    value: string,
  ): {label: string; description: string} => ({
    label: t(`applications:edit.token.${section}.response_type_options.${value}.label`, value),
    description: t(`applications:edit.token.${section}.response_type_options.${value}.description`, ''),
  });

  const isEncryptedFormat = (value?: string): boolean => value === 'JWE' || value === 'NESTED_JWT';

  // Placeholder shown in previews and the read-only signing line until discovery resolves.
  const signingAlgDisplay = signingAlg ?? '<server_key_alg>';

  // Encrypted formats need a client certificate, so disable them when none is configured.
  const isFormatOptionDisabled = (value: string): boolean => isEncryptedFormat(value) && !hasCertificate;

  /**
   * Build the JWT/JSON preview object for a given token type.
   */
  const buildPreview = (
    currentAttrs: string[],
    tokenType: 'shared' | 'access' | 'id' | 'userinfo',
  ): Record<string, unknown> => {
    const preview: Record<string, unknown> = {};
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

    // The backend always adds this claim to access tokens an agent uses to act on
    // behalf of a user, identifying the acting agent by its ID.
    if (tokenType === 'access' && showActorClaim) {
      preview['act'] = {sub: actorSub};
    }

    return preview;
  };

  /**
   * Build a JOSE header preview for the ID token based on its response type.
   */
  const buildIdTokenHeader = (): Record<string, string> | undefined => {
    const responseType = idTokenResponseType ?? 'JWT';
    if (responseType === 'JWT') {
      return {alg: signingAlgDisplay, kid: '<key_id>', typ: 'JWT'};
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
      return {alg: signingAlgDisplay, kid: '<key_id>', typ: 'JWT'};
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

    // Include already selected attributes so ones dropped from the schema stay visible and removable.
    const availableAttributes = Array.from(
      new Set([...userAttributes, ...TokenConstants.ADDITIONAL_USER_ATTRIBUTES, ...currentAttrs]),
    ).filter((attr) => !(defaultAttrs as readonly string[]).includes(attr));

    // On the ID Token and User Info tabs the scope mapping below is the primary control, so the
    // attribute allow-list is collapsed behind a disclosure. The access token has no scope mapping,
    // so its picker stays open.
    const isCollapsible = tokenType === 'id' || tokenType === 'userinfo';

    const attributeCard = (
      <Card>
        <CardContent>
          {isLoadingUserAttributes && (
            <Typography variant="body2" color="text.secondary">
              {t('applications:edit.token.loading_attributes', 'Loading user attributes...')}
            </Typography>
          )}
          {!isLoadingUserAttributes && (userAttributes.length > 0 || currentAttrs.length > 0) && (
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
                      '&:hover': isActive ? {backgroundColor: 'primary.dark'} : {backgroundColor: 'action.hover'},
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
    );

    // Accordion rather than a bare button: it carries the aria-expanded/aria-controls semantics of
    // a disclosure for free. Its card chrome is stripped since this already sits inside a card.
    if (isCollapsible) {
      return (
        <Accordion
          disableGutters
          square
          sx={{bgcolor: 'transparent', boxShadow: 'none', '&:before': {display: 'none'}}}
        >
          <AccordionSummary
            expandIcon={<ChevronDown size={16} />}
            sx={{px: 0, minHeight: 'auto', '& .MuiAccordionSummary-content': {my: 1}}}
          >
            <Typography variant="body2">
              {t('applications:edit.token.configure_attributes.toggle', 'Allowed Attributes ({{count}})', {
                count: currentAttrs.length,
              })}
            </Typography>
          </AccordionSummary>
          <AccordionDetails sx={{px: 0, pt: 0}}>{attributeCard}</AccordionDetails>
        </Accordion>
      );
    }

    return (
      <Box>
        <Typography variant="body2" sx={{mb: 1}}>
          {t('applications:edit.token.configure_attributes', 'Add or Remove Attributes')}
        </Typography>
        <Typography variant="body2" color="text.disabled" sx={{mb: 2}}>
          {t('applications:edit.token.configure_attributes.hint', 'Click on user attributes to add them to the token.')}
        </Typography>
        {attributeCard}
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
        <Grid size={{xs: 12, lg: 7}}>{renderAttributeChips(currentAttrs, tokenType)}</Grid>
        <Grid size={{xs: 12, lg: 5}} sx={{minWidth: 0}}>
          {tokenType === 'access' && showActorClaim && (
            <Alert severity="info" sx={{mb: 2}}>
              {t(
                'applications:edit.token.actorClaim.description',
                'The "act" claim identifies this {{entity}} as the party acting on behalf of the subject (sub). It is added automatically and is not one of the attributes you pick below.',
                {entity: entityLabel},
              )}
            </Alert>
          )}
          <JwtPreview payload={jwtPreview} defaultClaims={defaultAttrs} />
        </Grid>
      </Grid>
    );
  };

  // Native mode issues a single token and offers no response-format controls, so it gets its own
  // title and description rather than the multi-token, multi-response wording.
  const resolveCardTitle = (): string => {
    if (!isOAuthMode) {
      return t('applications:edit.token.token_profile_card.title.native', 'Token Attributes');
    }
    return t('applications:edit.token.token_profile_card.title', 'Token Attributes & Response');
  };

  const resolveCardDescription = (): string => {
    if (!isOAuthMode) {
      return t(
        'applications:edit.token.token_profile_card.description.native',
        'Choose the attributes included in the token issued to this {{entity}}.',
        {entity: entityLabel},
      );
    }
    if (showUserInfoTab) {
      return t(
        'applications:edit.token.token_profile_card.description',
        'Choose the attributes in each token and the user info response, and how each is returned.',
      );
    }
    return t(
      'applications:edit.token.token_profile_card.description.noUserInfo',
      'Choose the attributes in each token issued to this {{entity}}, and how each is returned.',
      {entity: entityLabel},
    );
  };

  const cardTitle = resolveCardTitle();
  const cardDescription = resolveCardDescription();
  if (isOAuthMode) {
    // The ID token and UserInfo share one scope-to-claims mapping, so they live under a single
    // top-level tab with the mapping above them: rendering the mapping inside either tab panel
    // would make one shared control look like two separate per-tab ones.
    const isUserSinkTab = activeTab === 'id' || activeTab === 'userinfo';

    return (
      <SettingsCard slotProps={{content: {sx: {p: 0}}}} title={cardTitle} description={cardDescription}>
        <Stack spacing={3}>
          <Tabs
            value={activeTab === 'access' ? 0 : 1}
            onChange={(_, newValue: number) => {
              // Returning to the combined tab restores whichever sink was open last.
              onTabChange?.(newValue === 0 ? 'access' : lastUserSinkTab);
            }}
            sx={{borderBottom: 1, borderColor: 'divider'}}
          >
            <Tab label={t('applications:edit.token.tabs.access_token', 'Access Token')} />
            <Tab
              label={
                showUserInfoTab
                  ? t('applications:edit.token.tabs.id_token_and_user_info', 'ID Token & User Info')
                  : t('applications:edit.token.tabs.id_token', 'ID Token')
              }
            />
          </Tabs>

          <Box sx={{p: 3}}>
            {/* Access Token Tab Panel */}
            {activeTab === 'access' && <Box>{renderAttributePanel(accessTokenAttributes ?? [], 'access')}</Box>}

            {isUserSinkTab && (
              <Stack spacing={3} sx={{mb: 3}}>
                {/* One shared mapping, above the sinks it feeds. */}
                {scopeMapping}
                {showUserInfoTab && (
                  <Tabs
                    value={activeTab === 'id' ? 0 : 1}
                    onChange={(_, newValue: number) => {
                      const sink = newValue === 0 ? 'id' : 'userinfo';
                      setLastUserSinkTab(sink);
                      onTabChange?.(sink);
                    }}
                    sx={{borderBottom: 1, borderColor: 'divider'}}
                  >
                    <Tab label={t('applications:edit.token.tabs.id_token', 'ID Token')} />
                    <Tab label={t('applications:edit.token.tabs.user_info_endpoint', 'User Info Endpoint')} />
                  </Tabs>
                )}
              </Stack>
            )}

            {/* ID Token Tab Panel */}
            {activeTab === 'id' &&
              (() => {
                const idAttrs = idTokenAttributes ?? [];
                const defaultAttrs = TokenConstants.DEFAULT_TOKEN_ATTRIBUTES;
                const jwtPreview = buildPreview(idAttrs, 'id');

                return (
                  <Grid container spacing={3}>
                    {/* Left Column - Attributes + Response Format */}
                    <Grid size={{xs: 12, lg: 7}}>
                      <Stack spacing={3}>
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
                                      {t(
                                        'applications:edit.token.id_token.response_type_placeholder',
                                        'Select response type',
                                      )}
                                    </Typography>
                                  ) : (
                                    responseTypeOption('id_token', String(selected)).label
                                  )
                                }
                              >
                                {TokenConstants.ID_TOKEN_RESPONSE_TYPES.map((type) => {
                                  const option = responseTypeOption('id_token', type);
                                  return (
                                    <MenuItem key={type} value={type} disabled={isFormatOptionDisabled(type)}>
                                      <Box>
                                        <Typography variant="body2">{option.label}</Typography>
                                        {option.description && (
                                          <Typography variant="caption" color="text.secondary">
                                            {option.description}
                                          </Typography>
                                        )}
                                      </Box>
                                    </MenuItem>
                                  );
                                })}
                              </Select>
                            </FormControl>

                            {/* Read-only signing algorithm (determined by the server key). Only
                                shown for signed formats, and only once resolved from discovery. */}
                            {signingAlg && (idTokenResponseType ?? 'JWT') !== 'JWE' && (
                              <Typography variant="caption" color="text.secondary">
                                {t('applications:edit.token.signed_with', 'Signed with {{alg}}.', {
                                  alg: signingAlg,
                                })}
                              </Typography>
                            )}

                            {/* Certificate requirement for encrypted formats */}
                            {!hasCertificate && (
                              <Alert severity="info">
                                {t(
                                  'applications:edit.token.encryption_requires_certificate',
                                  'Encrypted formats require an OAuth client certificate (JWKS or JWKS URI) configured under the {{location}} tab.',
                                  {location: certificateLocation},
                                )}
                              </Alert>
                            )}

                            {/* Row 2: Encryption fields */}
                            {isEncryptedFormat(idTokenResponseType) && (
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
                                          {t(
                                            'applications:edit.token.id_token.encryption_alg_placeholder',
                                            'Select encryption algorithm',
                                          )}
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
                                          {t(
                                            'applications:edit.token.id_token.encryption_enc_placeholder',
                                            'Select content encryption',
                                          )}
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
                        <Divider />
                        {renderAttributeChips(idAttrs, 'id')}
                      </Stack>
                    </Grid>

                    {/* Right Column - JWT Preview */}
                    <Grid size={{xs: 12, lg: 5}} sx={{minWidth: 0}}>
                      <JwtPreview payload={jwtPreview} defaultClaims={defaultAttrs} header={buildIdTokenHeader()} />
                    </Grid>
                  </Grid>
                );
              })()}

            {/* User Info Endpoint Tab Panel */}
            {showUserInfoTab &&
              activeTab === 'userinfo' &&
              (() => {
                const effectiveAttrs = isUserInfoCustomAttributes
                  ? (userInfoAttributes ?? [])
                  : (idTokenAttributes ?? []);
                const defaultAttrs = TokenConstants.USER_INFO_DEFAULT_ATTRIBUTES;
                const jwtPreview = buildPreview(effectiveAttrs, 'userinfo');

                return (
                  <Grid container spacing={3}>
                    {/* Left Column - Attributes + Response Format */}
                    <Grid size={{xs: 12, lg: 7}}>
                      <Stack spacing={3}>
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
                                      {t(
                                        'applications:edit.token.user_info.response_type_placeholder',
                                        'Select response type',
                                      )}
                                    </Typography>
                                  ) : (
                                    responseTypeOption('user_info', String(selected)).label
                                  )
                                }
                              >
                                {TokenConstants.USER_INFO_RESPONSE_TYPES.map((type) => {
                                  const option = responseTypeOption('user_info', type);
                                  return (
                                    <MenuItem key={type} value={type} disabled={isFormatOptionDisabled(type)}>
                                      <Box>
                                        <Typography variant="body2">{option.label}</Typography>
                                        {option.description && (
                                          <Typography variant="caption" color="text.secondary">
                                            {option.description}
                                          </Typography>
                                        )}
                                      </Box>
                                    </MenuItem>
                                  );
                                })}
                              </Select>
                            </FormControl>

                            {/* Read-only signing algorithm for signed formats (determined by the
                                server key). Only shown once resolved from discovery. */}
                            {signingAlg &&
                              (userInfoResponseType === 'JWS' || userInfoResponseType === 'NESTED_JWT') && (
                                <Typography variant="caption" color="text.secondary">
                                  {t('applications:edit.token.signed_with', 'Signed with {{alg}}.', {
                                    alg: signingAlg,
                                  })}
                                </Typography>
                              )}

                            {/* Certificate requirement for encrypted formats */}
                            {!hasCertificate && (
                              <Alert severity="info">
                                {t(
                                  'applications:edit.token.encryption_requires_certificate',
                                  'Encrypted formats require an OAuth client certificate (JWKS or JWKS URI) configured under the {{location}} tab.',
                                  {location: certificateLocation},
                                )}
                              </Alert>
                            )}

                            {/* Row 2: Encryption fields */}
                            {isEncryptedFormat(userInfoResponseType) && (
                              <Stack direction="row" spacing={2} flexWrap="wrap" useFlexGap>
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
                                          {t(
                                            'applications:edit.token.user_info.encryption_alg_placeholder',
                                            'Select encryption algorithm',
                                          )}
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
                                          {t(
                                            'applications:edit.token.user_info.encryption_enc_placeholder',
                                            'Select content encryption',
                                          )}
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
                              </Stack>
                            )}
                          </Stack>
                        </Box>
                        <Divider />
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
                      </Stack>
                    </Grid>

                    {/* Right Column - JWT/JSON Preview */}
                    <Grid size={{xs: 12, lg: 5}} sx={{minWidth: 0}}>
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
