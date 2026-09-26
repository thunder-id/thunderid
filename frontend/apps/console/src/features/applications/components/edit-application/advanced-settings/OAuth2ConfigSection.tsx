// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {OAuth2Logo, SettingsCard} from '@thunderid/components';
import {OAuth2ResponseTypes, TokenEndpointAuthMethods} from '@thunderid/configure-applications';
import type {OAuth2Config} from '@thunderid/configure-applications';
import {useThunderID} from '@thunderid/react';
import {
  Box,
  Stack,
  Typography,
  Chip,
  FormControl,
  FormLabel,
  Select,
  MenuItem,
  Checkbox,
  ListItemText,
  FormControlLabel,
  Switch,
  Divider,
  Tooltip,
  TextField,
  Button,
  IconButton,
} from '@wso2/oxygen-ui';
import {Lock, Trash, Plus} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useState} from 'react';
import {useTranslation} from 'react-i18next';
import type {ApplicationTemplate} from '../../../models/application-templates';
import {getGrantTypeLabel} from '../../../utils/getGrantTypeLabel';
import isValidRedirectUriFormat, {isValidRedirectUriTransport} from '../../../utils/isValidRedirectUriFormat';
import {
  applyGrantTypesChange,
  applyPublicClientChange,
  applyTokenEndpointAuthMethodChange,
  deriveOAuth2Flags,
  getPkceCaption,
  getPublicClientCaption,
  isGrantItemDisabled,
} from '../../../utils/oauth2Rules';

interface OidcDiscovery {
  grant_types_supported?: string[];
  response_types_supported?: string[];
  token_endpoint_auth_methods_supported?: string[];
  acr_values_supported?: string[];
}

/**
 * Props for the {@link OAuth2ConfigSection} component.
 */
interface OAuth2ConfigSectionProps {
  /**
   * OAuth2 configuration to display
   */
  oauth2Config?: OAuth2Config;
  /**
   * Template-driven field constraints for OAuth2 fields
   */
  oauth2Constraints?: NonNullable<ApplicationTemplate['fieldConstraints']>['oauth2'];
  /**
   * Callback to handle OAuth2 config field changes. When omitted the section is read-only.
   */
  onOAuth2ConfigChange?: (updates: Partial<OAuth2Config>) => void;
  /**
   * Whether inputs should be disabled (e.g. read-only resource).
   */
  disabled?: boolean;
  /**
   * When set, restricts the offered grant types to the intersection of this list and
   * discovery's `grant_types_supported`. Omit to offer every discovery-advertised grant type.
   */
  allowedGrantTypes?: string[];
  /**
   * Whether to show the redirect URI and post-logout redirect URI fields. Hidden for clients
   * with no user-facing grant, and for MCP clients, which manage their redirect URIs on their
   * own Connect tab.
   */
  showRedirectUris?: boolean;
  /**
   * Callback to report whether the redirect URI fields currently have validation errors.
   */
  onValidationChange?: (hasErrors: boolean) => void;
}

/**
 * Section component for displaying and editing OAuth2 configuration.
 *
 * Shows:
 * - Allowed grant types (multi-select dropdown from OIDC discovery)
 * - Allowed response types (multi-select dropdown from OIDC discovery)
 * - Authorized redirect URIs and post-logout redirect URIs (add/remove/edit with validation)
 * - Public client status (toggle)
 * - PKCE requirement status (toggle)
 *
 * When `onOAuth2ConfigChange` is provided the fields are editable; otherwise they are read-only.
 * Returns null if no OAuth2 configuration is provided.
 *
 * @param props - Component props
 * @returns OAuth2 configuration UI within a SettingsCard, or null
 */
export default function OAuth2ConfigSection({
  oauth2Config = undefined,
  oauth2Constraints = undefined,
  onOAuth2ConfigChange = undefined,
  disabled = false,
  allowedGrantTypes = undefined,
  showRedirectUris = true,
  onValidationChange = undefined,
}: OAuth2ConfigSectionProps) {
  const {t} = useTranslation();
  const {discovery} = useThunderID();

  const [redirectUris, setRedirectUris] = useState<string[]>(() => oauth2Config?.redirectUris ?? []);
  const [uriErrors, setUriErrors] = useState<Record<number, string>>({});

  const [postLogoutRedirectUris, setPostLogoutRedirectUris] = useState<string[]>(
    () => oauth2Config?.postLogoutRedirectUris ?? [],
  );
  const [postLogoutUriErrors, setPostLogoutUriErrors] = useState<Record<number, string>>({});

  useEffect(() => {
    const hasUriErrors =
      showRedirectUris && (Object.keys(uriErrors).length > 0 || Object.keys(postLogoutUriErrors).length > 0);
    onValidationChange?.(hasUriErrors);
  }, [uriErrors, postLogoutUriErrors, onValidationChange, showRedirectUris]);

  if (!oauth2Config) return null;

  // An empty list renders as a bare "Add URI" button with no field to type into, which reads as
  // broken rather than "nothing added yet". Show one empty row by default instead; typing into it
  // (or removing it) operates on the real (currently empty) list via the index above.
  const displayRedirectUris = redirectUris.length > 0 ? redirectUris : [''];
  const displayPostLogoutRedirectUris = postLogoutRedirectUris.length > 0 ? postLogoutRedirectUris : [''];

  const validateUri = (uri: string, index: number): boolean => {
    if (!uri || uri.trim() === '') {
      setUriErrors((prev) => ({...prev, [index]: t('applications:edit.general.redirectUris.error.empty')}));
      return false;
    }
    if (!isValidRedirectUriFormat(uri) || !isValidRedirectUriTransport(uri)) {
      setUriErrors((prev) => ({...prev, [index]: t('applications:edit.general.redirectUris.error.invalid')}));
      return false;
    }
    setUriErrors((prev) => {
      const newErrors = {...prev};
      delete newErrors[index];
      return newErrors;
    });
    return true;
  };

  // Post-signout redirect URIs are optional: an empty row is allowed (filtered out on save), only a
  // non-empty malformed value is an error.
  const validatePostLogoutUri = (uri: string, index: number): boolean => {
    if (!uri || uri.trim() === '') {
      setPostLogoutUriErrors((prev) => {
        const newErrors = {...prev};
        delete newErrors[index];
        return newErrors;
      });
      return false;
    }
    if (!isValidRedirectUriFormat(uri)) {
      setPostLogoutUriErrors((prev) => ({
        ...prev,
        [index]: t('applications:edit.general.postLogoutRedirectUris.error.invalid', 'Enter a valid URI'),
      }));
      return false;
    }
    setPostLogoutUriErrors((prev) => {
      const newErrors = {...prev};
      delete newErrors[index];
      return newErrors;
    });
    return true;
  };

  // Drop the error for a removed row and shift higher-indexed errors down by one.
  const reindexErrors = (errors: Record<number, string>, removedIndex: number): Record<number, string> => {
    const next = {...errors};
    delete next[removedIndex];
    const reindexed: Record<number, string> = {};
    Object.entries(next).forEach(([key, value]) => {
      const oldIndex = parseInt(key, 10);
      if (oldIndex > removedIndex) {
        reindexed[oldIndex - 1] = value;
      } else if (oldIndex < removedIndex) {
        reindexed[oldIndex] = value;
      }
    });
    return reindexed;
  };

  // Appends relative to what's on screen (displayRedirectUris), not the possibly-empty backing
  // `redirectUris` array — otherwise the first click while the list is empty turns [] into [''],
  // which renders identically to the placeholder row already shown and looks like nothing happened.
  const handleAddUri = () => {
    setRedirectUris([...displayRedirectUris, '']);
  };

  const handleRemoveUri = (index: number) => {
    const newUris = redirectUris.filter((_, i) => i !== index);
    setRedirectUris(newUris);
    setUriErrors((prev) => reindexErrors(prev, index));
    onOAuth2ConfigChange?.({redirectUris: newUris.filter((uri) => uri.trim() !== '')});
  };

  const handleUriChange = (index: number, value: string) => {
    setRedirectUris((prev) => {
      const newUris = [...prev];
      newUris[index] = value;

      return newUris;
    });

    if (value.trim() !== '') {
      setUriErrors((prev) => {
        const newErrors = {...prev};
        delete newErrors[index];

        return newErrors;
      });
    }
  };

  const handleUriBlur = (index: number) => {
    const uri = redirectUris[index];
    if (validateUri(uri, index) && uri.trim() !== '') {
      onOAuth2ConfigChange?.({redirectUris: redirectUris.filter((u) => u.trim() !== '')});
    }
  };

  // See handleAddUri: appends relative to the displayed array so the click is never a no-op.
  const handleAddPostLogoutUri = () => {
    setPostLogoutRedirectUris([...displayPostLogoutRedirectUris, '']);
  };

  const handleRemovePostLogoutUri = (index: number) => {
    const newUris = postLogoutRedirectUris.filter((_, i) => i !== index);
    setPostLogoutRedirectUris(newUris);
    setPostLogoutUriErrors((prev) => reindexErrors(prev, index));
    onOAuth2ConfigChange?.({postLogoutRedirectUris: newUris.filter((uri) => uri.trim() !== '')});
  };

  const handlePostLogoutUriChange = (index: number, value: string) => {
    setPostLogoutRedirectUris((prev) => {
      const newUris = [...prev];
      newUris[index] = value;

      return newUris;
    });

    if (value.trim() !== '') {
      setPostLogoutUriErrors((prev) => {
        const newErrors = {...prev};
        delete newErrors[index];

        return newErrors;
      });
    }
  };

  const handlePostLogoutUriBlur = (index: number) => {
    const uri = postLogoutRedirectUris[index];
    if (validatePostLogoutUri(uri, index) && uri.trim() !== '') {
      onOAuth2ConfigChange?.({postLogoutRedirectUris: postLogoutRedirectUris.filter((u) => u.trim() !== '')});
    }
  };

  const discoveredGrantTypes = discovery?.wellKnown?.grant_types_supported ?? [];
  const availableGrantTypes = allowedGrantTypes
    ? discoveredGrantTypes.filter((grant) => allowedGrantTypes.includes(grant))
    : discoveredGrantTypes;
  const availableResponseTypes = discovery?.wellKnown?.response_types_supported ?? [];
  const availableTokenEndpointAuthMethods: string[] =
    (discovery as {wellKnown?: OidcDiscovery} | undefined)?.wellKnown?.token_endpoint_auth_methods_supported ?? [];
  const availableAcrValues: string[] =
    (discovery as {wellKnown?: OidcDiscovery} | undefined)?.wellKnown?.acr_values_supported ?? [];
  const isEditable = Boolean(onOAuth2ConfigChange) && !disabled;

  // Constraint helpers
  const constraint = <K extends keyof OAuth2Config>(field: K) => oauth2Constraints?.[field];

  const isPublicClientLocked = constraint('publicClient')?.readOnly ?? false;
  const isPkceLocked = constraint('pkceRequired')?.readOnly ?? false;
  const isParLocked = constraint('requirePushedAuthorizationRequests')?.readOnly ?? false;
  const tokenMethodConstraint = constraint('tokenEndpointAuthMethod');
  const isTokenMethodLocked = tokenMethodConstraint?.readOnly ?? oauth2Config.publicClient === true;
  const effectiveTokenMethod =
    tokenMethodConstraint?.value ?? (oauth2Config.publicClient ? 'none' : (oauth2Config.tokenEndpointAuthMethod ?? ''));

  const grantTypes = oauth2Config.grantTypes ?? [];
  const flags = deriveOAuth2Flags(oauth2Config);
  const {
    hasAuthorizationCodeGrant,
    isPkceDisabledByGrants,
    isPkceForcedByPublicClient,
    isPublicClientDisabledByGrants,
    isParDisabledByGrants,
  } = flags;

  return (
    <SettingsCard
      title={t('applications:edit.advanced.labels.oauth2Config')}
      description={t('applications:edit.advanced.oauth2Config.intro', 'Manage OAuth 2 settings for this {{entity}}.', {
        entity: 'application',
      })}
      titleIcon={<OAuth2Logo />}
    >
      <Stack spacing={3}>
        {/* Grant Types */}
        <FormControl fullWidth size="small">
          <FormLabel htmlFor="grant_types">{t('applications:edit.advanced.labels.grantTypes')}</FormLabel>
          <Select
            id="grant_types"
            multiple
            displayEmpty
            disabled={!isEditable}
            value={grantTypes}
            onChange={(e) => onOAuth2ConfigChange?.(applyGrantTypesChange(oauth2Config, e.target.value as string[]))}
            renderValue={(selected) =>
              selected.length === 0 ? (
                <Typography color="text.secondary" variant="body2">
                  {t('applications:edit.advanced.grantTypes.placeholder')}
                </Typography>
              ) : (
                <Stack direction="row" spacing={0.5} flexWrap="wrap" useFlexGap>
                  {selected.map((v) => (
                    <Chip key={v} label={v} size="small" />
                  ))}
                </Stack>
              )
            }
          >
            {availableGrantTypes.map((grant) => (
              <MenuItem key={grant} value={grant} disabled={isGrantItemDisabled(grant, grantTypes)}>
                <Checkbox checked={grantTypes.includes(grant)} size="small" />
                <ListItemText primary={getGrantTypeLabel(grant, t)} />
              </MenuItem>
            ))}
          </Select>
          <Typography variant="caption" color="text.secondary" sx={{mt: 0.5}}>
            {t(
              'applications:edit.advanced.grantTypes.hint',
              'OAuth 2 flows this {{entity}} can use (e.g., authorization_code, client_credentials, refresh_token).',
              {entity: 'application'},
            )}
          </Typography>
        </FormControl>

        {/* Response Types */}
        <FormControl fullWidth size="small">
          <FormLabel htmlFor="response_types">{t('applications:edit.advanced.labels.responseTypes')}</FormLabel>
          <Select
            id="response_types"
            multiple
            displayEmpty
            disabled={!isEditable || !hasAuthorizationCodeGrant}
            value={oauth2Config.responseTypes ?? []}
            onChange={(e) => onOAuth2ConfigChange?.({responseTypes: e.target.value as string[]})}
            renderValue={(selected) =>
              selected.length === 0 ? (
                <Typography color="text.secondary" variant="body2">
                  {t('applications:edit.advanced.responseTypes.placeholder')}
                </Typography>
              ) : (
                <Stack direction="row" spacing={0.5} flexWrap="wrap" useFlexGap>
                  {selected.map((v) => (
                    <Chip key={v} label={v} size="small" />
                  ))}
                </Stack>
              )
            }
          >
            {availableResponseTypes.map((rt) => (
              <MenuItem key={rt} value={rt} disabled={rt === OAuth2ResponseTypes.CODE && hasAuthorizationCodeGrant}>
                <Checkbox checked={(oauth2Config.responseTypes ?? []).includes(rt)} size="small" />
                <ListItemText primary={rt} />
              </MenuItem>
            ))}
          </Select>
          <Typography variant="caption" color="text.secondary" sx={{mt: 0.5}}>
            {hasAuthorizationCodeGrant
              ? t(
                  'applications:edit.advanced.responseTypes.codeRequiredHint',
                  'Required for the authorization code flow.',
                )
              : t(
                  'applications:edit.advanced.responseTypes.notApplicable',
                  'Response types apply only to the authorization code flow.',
                )}
          </Typography>
        </FormControl>

        {showRedirectUris && (
          <FormControl fullWidth>
            <FormLabel htmlFor="redirect-uris-section">{t('applications:edit.general.redirectUris.title')}</FormLabel>
            <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 2}}>
              {t('applications:edit.general.redirectUris.description')}
            </Typography>

            <Stack spacing={2} id="redirect-uris-section">
              {displayRedirectUris.map((uri, index) => (
                // IMPORTANT: Do not remove the suppression since it affects functionality.
                // eslint-disable-next-line react/no-array-index-key
                <Stack key={index} direction="row" spacing={1} alignItems="flex-start">
                  <FormControl fullWidth required sx={{flex: 1}}>
                    <TextField
                      fullWidth
                      id={`redirect-uri-${index}-input`}
                      value={uri}
                      onChange={(e) => handleUriChange(index, e.target.value)}
                      onBlur={() => handleUriBlur(index)}
                      error={!!uriErrors[index]}
                      helperText={uriErrors[index]}
                      placeholder="https://example.com/callback"
                      disabled={!isEditable}
                    />
                  </FormControl>
                  <Tooltip title={t('common:actions.delete')}>
                    <IconButton
                      onClick={() => handleRemoveUri(index)}
                      color="error"
                      sx={{mt: 1}}
                      disabled={!isEditable}
                    >
                      <Trash size={20} />
                    </IconButton>
                  </Tooltip>
                </Stack>
              ))}

              <Box>
                <Button
                  variant="text"
                  color="primary"
                  startIcon={<Plus />}
                  onClick={handleAddUri}
                  size="small"
                  disabled={!isEditable}
                >
                  {t('applications:edit.general.redirectUris.addUri')}
                </Button>
              </Box>
            </Stack>
          </FormControl>
        )}

        {showRedirectUris && (
          <FormControl fullWidth>
            <FormLabel htmlFor="post-logout-redirect-uris-section">
              {t('applications:edit.general.postLogoutRedirectUris.title', 'Post-Logout Redirect URIs')}
            </FormLabel>
            <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 2}}>
              {t(
                'applications:edit.general.postLogoutRedirectUris.description',
                'Allowed URIs to redirect to after signout. A post_logout_redirect_uri passed to the signout endpoint must match one of these.',
              )}
            </Typography>

            <Stack spacing={2} id="post-logout-redirect-uris-section">
              {displayPostLogoutRedirectUris.map((uri, index) => (
                // IMPORTANT: Do not remove the suppression since it affects functionality.
                // eslint-disable-next-line react/no-array-index-key
                <Stack key={index} direction="row" spacing={1} alignItems="flex-start">
                  <FormControl fullWidth sx={{flex: 1}}>
                    <TextField
                      fullWidth
                      id={`post-logout-redirect-uri-${index}-input`}
                      value={uri}
                      onChange={(e) => handlePostLogoutUriChange(index, e.target.value)}
                      onBlur={() => handlePostLogoutUriBlur(index)}
                      error={!!postLogoutUriErrors[index]}
                      helperText={postLogoutUriErrors[index]}
                      placeholder="https://example.com/logged-out"
                      disabled={!isEditable}
                    />
                  </FormControl>
                  <Tooltip title={t('common:actions.delete')}>
                    <IconButton
                      onClick={() => handleRemovePostLogoutUri(index)}
                      color="error"
                      sx={{mt: 1}}
                      disabled={!isEditable}
                    >
                      <Trash size={20} />
                    </IconButton>
                  </Tooltip>
                </Stack>
              ))}

              <Box>
                <Button
                  variant="text"
                  color="primary"
                  startIcon={<Plus />}
                  onClick={handleAddPostLogoutUri}
                  size="small"
                  disabled={!isEditable}
                >
                  {t('applications:edit.general.postLogoutRedirectUris.addUri', 'Add URI')}
                </Button>
              </Box>
            </Stack>
          </FormControl>
        )}

        {/* Token Endpoint Auth Method */}
        <FormControl fullWidth size="small">
          <FormLabel htmlFor="token_endpoint_auth_method">
            {t('applications:edit.advanced.labels.tokenEndpointAuthMethod', 'Client Authentication Method')}
          </FormLabel>
          <Select
            id="token_endpoint_auth_method"
            displayEmpty
            disabled={!isEditable || isTokenMethodLocked}
            value={effectiveTokenMethod}
            onChange={(e) => onOAuth2ConfigChange?.(applyTokenEndpointAuthMethodChange(oauth2Config, e.target.value))}
            renderValue={(selected) =>
              !selected ? (
                <Typography color="text.secondary" variant="body2">
                  {t('applications:edit.advanced.tokenEndpointAuthMethod.placeholder')}
                </Typography>
              ) : (
                selected
              )
            }
          >
            {availableTokenEndpointAuthMethods.map((method) => (
              <MenuItem
                key={method}
                value={method}
                disabled={method === TokenEndpointAuthMethods.NONE && isPublicClientDisabledByGrants}
              >
                {method}
              </MenuItem>
            ))}
          </Select>
          <Typography variant="caption" color="text.secondary" sx={{mt: 0.5}}>
            {isTokenMethodLocked
              ? tokenMethodConstraint?.readOnly
                ? t(
                    'applications:edit.advanced.tokenEndpointAuthMethod.templateLockedHint',
                    'Locked by the template configuration.',
                  )
                : t(
                    'applications:edit.advanced.tokenEndpointAuthMethod.lockedHint',
                    'Locked to "none" because the client is public.',
                  )
              : t(
                  'applications:edit.advanced.tokenEndpointAuthMethod.hint',
                  'Defines how the client authenticates at protected endpoints.',
                )}
          </Typography>
        </FormControl>

        {/* ACR Values */}
        {availableAcrValues.length > 0 && (
          <FormControl fullWidth size="small">
            <FormLabel htmlFor="acr_values">{t('applications:edit.advanced.labels.acrValues')}</FormLabel>
            <Select
              id="acr_values"
              multiple
              displayEmpty
              disabled={!isEditable}
              value={oauth2Config.acrValues ?? []}
              onChange={(e) => onOAuth2ConfigChange?.({acrValues: e.target.value as string[]})}
              renderValue={(selected) =>
                selected.length === 0 ? (
                  <Typography color="text.secondary" variant="body2">
                    {t('applications:edit.advanced.acrValues.placeholder', 'Select ACR values')}
                  </Typography>
                ) : (
                  <Stack direction="row" spacing={0.5} flexWrap="wrap" useFlexGap>
                    {selected.map((v) => (
                      <Chip key={v} label={v} size="small" />
                    ))}
                  </Stack>
                )
              }
            >
              {availableAcrValues.map((acr) => (
                <MenuItem key={acr} value={acr}>
                  <Checkbox checked={(oauth2Config.acrValues ?? []).includes(acr)} size="small" />
                  <ListItemText primary={acr} />
                </MenuItem>
              ))}
            </Select>
            <Typography variant="caption" color="text.secondary" sx={{mt: 0.5}}>
              {t(
                'applications:edit.advanced.acrValues.hint',
                'When acr_values is included in the authorization request, only values configured here are accepted.',
              )}
            </Typography>
          </FormControl>
        )}

        <Divider />

        {/* Public Client Toggle */}
        <Box>
          <FormControlLabel
            control={
              <Switch
                checked={oauth2Config.publicClient ?? false}
                disabled={!isEditable || isPublicClientLocked || isPublicClientDisabledByGrants}
                onChange={(e) => onOAuth2ConfigChange?.(applyPublicClientChange(oauth2Config, e.target.checked))}
                inputProps={{'aria-label': t('applications:edit.advanced.labels.publicClient')}}
              />
            }
            label={
              <Stack direction="row" alignItems="center" spacing={0.5}>
                <Typography variant="subtitle2">{t('applications:edit.advanced.labels.publicClient')}</Typography>
                {isPublicClientLocked && (
                  <Tooltip title={t('applications:edit.advanced.lockedByTemplate', 'Set by template')}>
                    <Lock size={14} />
                  </Tooltip>
                )}
              </Stack>
            }
          />
          <Typography variant="caption" color="text.secondary" sx={{display: 'block', ml: '52px'}}>
            {t(...getPublicClientCaption(flags, oauth2Config))}
          </Typography>
        </Box>

        {/* PKCE Required Toggle */}
        <Box>
          <FormControlLabel
            control={
              <Switch
                checked={isPkceForcedByPublicClient ? true : (oauth2Config.pkceRequired ?? false)}
                disabled={!isEditable || isPkceLocked || isPkceDisabledByGrants || isPkceForcedByPublicClient}
                onChange={(e) => onOAuth2ConfigChange?.({pkceRequired: e.target.checked})}
                inputProps={{'aria-label': t('applications:edit.advanced.labels.pkceRequired')}}
              />
            }
            label={
              <Stack direction="row" alignItems="center" spacing={0.5}>
                <Typography variant="subtitle2">{t('applications:edit.advanced.labels.pkceRequired')}</Typography>
                {isPkceLocked && (
                  <Tooltip title={t('applications:edit.advanced.lockedByTemplate', 'Set by template')}>
                    <Lock size={14} />
                  </Tooltip>
                )}
              </Stack>
            }
          />
          <Typography variant="caption" color="text.secondary" sx={{display: 'block', ml: '52px'}}>
            {t(...getPkceCaption(flags, oauth2Config))}
          </Typography>
        </Box>

        {/* Require Pushed Authorization Requests (PAR) Toggle */}
        <Box>
          <FormControlLabel
            control={
              <Switch
                checked={isParDisabledByGrants ? false : (oauth2Config.requirePushedAuthorizationRequests ?? false)}
                disabled={!isEditable || isParLocked || isParDisabledByGrants}
                onChange={(e) => onOAuth2ConfigChange?.({requirePushedAuthorizationRequests: e.target.checked})}
                inputProps={{
                  'aria-label': t(
                    'applications:edit.advanced.labels.requirePAR',
                    'Require Pushed Authorization Requests',
                  ),
                }}
              />
            }
            label={
              <Stack direction="row" alignItems="center" spacing={0.5}>
                <Typography variant="subtitle2">
                  {t('applications:edit.advanced.labels.requirePAR', 'Require Pushed Authorization Requests')}
                </Typography>
                {isParLocked && (
                  <Tooltip title={t('applications:edit.advanced.lockedByTemplate', 'Set by template')}>
                    <Lock size={14} />
                  </Tooltip>
                )}
              </Stack>
            }
          />
          <Typography variant="caption" color="text.secondary" sx={{display: 'block', ml: '52px'}}>
            {isParDisabledByGrants
              ? t(
                  'applications:edit.advanced.par.requiresAuthorizationCode',
                  'Available only when the authorization code grant is enabled.',
                )
              : t(
                  'applications:edit.advanced.par.hint',
                  'Require the client to use the PAR endpoint before authorization.',
                )}
          </Typography>
        </Box>
      </Stack>
    </SettingsCard>
  );
}
