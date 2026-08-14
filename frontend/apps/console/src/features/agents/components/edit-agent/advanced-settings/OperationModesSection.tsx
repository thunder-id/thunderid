// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {OAuth2Logo, SettingsCard} from '@thunderid/components';
import {OAuth2GrantTypes, TokenEndpointAuthMethods} from '@thunderid/configure-applications';
import type {OAuth2Config} from '@thunderid/configure-applications';
import {useThunderID} from '@thunderid/react';
import {
  Box,
  Checkbox,
  Chip,
  Divider,
  FormControl,
  FormControlLabel,
  FormLabel,
  ListItemText,
  MenuItem,
  Select,
  Stack,
  Switch,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import type {ReactNode} from 'react';
import {Trans, useTranslation} from 'react-i18next';
import RedirectURIsSection from './RedirectURIsSection';
import {getGrantTypeLabel} from '../../../../applications/utils/getGrantTypeLabel';
import {
  applyGrantTypesChange,
  applyTokenEndpointAuthMethodChange,
  deriveOAuth2Flags,
} from '../../../../applications/utils/oauth2Rules';
import {DELEGATED_ONLY_GRANTS} from '../../../constants/delegationGrants';
import {codeComponents} from '../shared/transCodeComponents';

interface OidcDiscovery {
  token_endpoint_auth_methods_supported?: string[];
}

interface OperationModesSectionProps {
  oauth2Config?: OAuth2Config;
  onOAuth2ConfigChange?: (updates: Partial<OAuth2Config>) => void;
  disabled?: boolean;
}

export default function OperationModesSection({
  oauth2Config = undefined,
  onOAuth2ConfigChange = undefined,
  disabled = false,
}: OperationModesSectionProps) {
  const {t} = useTranslation();
  const {discovery} = useThunderID();

  if (!oauth2Config) return null;

  const isEditable = Boolean(onOAuth2ConfigChange) && !disabled;
  const grantTypes = oauth2Config.grantTypes ?? [];
  const hasDelegatedMode = grantTypes.includes(OAuth2GrantTypes.AUTHORIZATION_CODE);
  const flags = deriveOAuth2Flags(oauth2Config);

  const updateGrants = (next: string[]): void => {
    if (!isEditable) return;
    const updates = applyGrantTypesChange(oauth2Config, next);
    // PKCE is fully derived from authorization_code for agents.
    if (next.includes(OAuth2GrantTypes.AUTHORIZATION_CODE)) {
      updates.pkceRequired = true;
    }
    onOAuth2ConfigChange?.(updates);
  };

  // client_credentials and token_exchange apply no matter which modes are active — the former
  // is mandatory for every agent, the latter is shared across both modes. The delegated-only
  // grants stay in the list at all times (see isGrantLocked) rather than disappearing when
  // Delegated mode is off.
  const availableGrantTypes: string[] = [
    OAuth2GrantTypes.CLIENT_CREDENTIALS,
    ...DELEGATED_ONLY_GRANTS,
    OAuth2GrantTypes.TOKEN_EXCHANGE,
  ];

  const isGrantLocked = (grant: string): boolean =>
    grant === OAuth2GrantTypes.CLIENT_CREDENTIALS ||
    (DELEGATED_ONLY_GRANTS.includes(grant) && (!hasDelegatedMode || grant === OAuth2GrantTypes.AUTHORIZATION_CODE));

  const selectedGrantTypes = grantTypes.filter((grant) => availableGrantTypes.includes(grant));

  const wellKnown = (discovery as {wellKnown?: OidcDiscovery} | undefined)?.wellKnown;
  const availableTokenEndpointAuthMethods: string[] = wellKnown?.token_endpoint_auth_methods_supported ?? [];
  const isTokenMethodLocked = oauth2Config.publicClient === true;
  const effectiveTokenMethod = oauth2Config.publicClient ? 'none' : (oauth2Config.tokenEndpointAuthMethod ?? '');

  const isPkceForced = flags.isPkceForcedByPublicClient;
  // PKCE is fully derived from the authorization_code grant for agents — it is never a
  // manually editable setting, it just reflects whether that grant is selected.
  const isPkceRequired = isPkceForced || flags.hasAuthorizationCodeGrant;

  const getPkceCaption = (): ReactNode => {
    if (isPkceForced) {
      return t(
        'agents:edit.advanced.security.pkce.forced',
        'This agent is set up as a public client, so PKCE is required and cannot be turned off.',
      );
    }
    if (flags.isPkceDisabledByGrants) {
      return (
        <Trans
          i18nKey="agents:edit.advanced.security.pkce.notApplicable"
          defaults="PKCE only applies to the <code>authorization_code</code> grant. Turn that on to enable this setting."
          components={codeComponents}
        />
      );
    }
    return t(
      'agents:edit.advanced.security.pkce.on',
      'authorization_code is on for this agent, so PKCE is required automatically.',
    );
  };

  const handleAudienceChange = (audience: string): void => {
    onOAuth2ConfigChange?.({
      token: {
        ...oauth2Config.token,
        accessToken: {...oauth2Config.token?.accessToken, defaultAudience: audience},
      } as OAuth2Config['token'],
    });
  };

  return (
    <SettingsCard
      title={t('agents:edit.advanced.oauthAccess.title', 'OAuth 2 Configuration')}
      description={t(
        'agents:edit.advanced.oauthAccess.description',
        'The grants and redirect URIs this agent is authorized to use.',
      )}
      titleIcon={<OAuth2Logo />}
    >
      <Stack spacing={3}>
        <Box>
          <FormControl fullWidth size="small">
            <FormLabel htmlFor="agent-grant-types">
              {t('agents:edit.advanced.oauthAccess.grantTypes.label', 'Grant Types')}
            </FormLabel>
            <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 1}}>
              {t(
                'agents:edit.advanced.oauthAccess.grantTypes.hint',
                'The greyed-out grants unlock once you turn on Delegated mode at the top of this tab.',
              )}
            </Typography>
            <Select
              id="agent-grant-types"
              multiple
              displayEmpty
              disabled={!isEditable}
              value={selectedGrantTypes}
              onChange={(e) => updateGrants(e.target.value as string[])}
              renderValue={(selected) => (
                <Stack direction="row" spacing={0.5} flexWrap="wrap" useFlexGap>
                  {selected.map((grant) => (
                    <Chip key={grant} label={getGrantTypeLabel(grant, t)} size="small" />
                  ))}
                </Stack>
              )}
            >
              {availableGrantTypes.map((grant) => (
                <MenuItem key={grant} value={grant} disabled={isGrantLocked(grant)}>
                  <Checkbox checked={grantTypes.includes(grant)} size="small" />
                  <ListItemText primary={getGrantTypeLabel(grant, t)} />
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        </Box>

        <RedirectURIsSection
          oauth2Config={oauth2Config}
          onOAuth2ConfigChange={onOAuth2ConfigChange}
          disabled={disabled}
        />

        <Divider />

        <FormControl fullWidth size="small">
          <FormLabel htmlFor="agent_token_endpoint_auth_method">
            {t('agents:edit.credentials.tokenEndpointAuthMethod.title', 'Client Authentication Method')}
          </FormLabel>
          <Select
            id="agent_token_endpoint_auth_method"
            displayEmpty
            disabled={!isEditable || isTokenMethodLocked}
            value={effectiveTokenMethod}
            onChange={(e) => onOAuth2ConfigChange?.(applyTokenEndpointAuthMethodChange(oauth2Config, e.target.value))}
            renderValue={(selected) =>
              !selected ? (
                <Typography color="text.secondary" variant="body2">
                  {t('agents:edit.credentials.tokenEndpointAuthMethod.placeholder', 'Select an auth method')}
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
                disabled={method === TokenEndpointAuthMethods.NONE && flags.isPublicClientDisabledByGrants}
              >
                {method}
              </MenuItem>
            ))}
          </Select>
          <Typography variant="caption" color="text.secondary" sx={{mt: 0.5}}>
            {isTokenMethodLocked
              ? t(
                  'agents:edit.credentials.tokenEndpointAuthMethod.lockedHint',
                  'Set to "none" because this agent is a public client.',
                )
              : t(
                  'agents:edit.credentials.tokenEndpointAuthMethod.hint',
                  'How this agent proves its identity at protected endpoints.',
                )}
          </Typography>
        </FormControl>

        <Divider />

        <Box>
          <FormControlLabel
            control={
              <Switch
                checked={isPkceRequired}
                disabled
                inputProps={{'aria-label': t('agents:edit.advanced.security.pkce.label', 'Require PKCE')}}
              />
            }
            label={
              <Typography variant="subtitle2">
                {t('agents:edit.advanced.security.pkce.label', 'Require PKCE')}
              </Typography>
            }
          />
          <Typography variant="caption" color="text.secondary" sx={{display: 'block', ml: '52px'}}>
            {getPkceCaption()}
          </Typography>
        </Box>

        <Box>
          <FormControlLabel
            control={
              <Switch
                checked={
                  flags.isParDisabledByGrants ? false : (oauth2Config.requirePushedAuthorizationRequests ?? false)
                }
                disabled={!isEditable || flags.isParDisabledByGrants}
                onChange={(e) => onOAuth2ConfigChange?.({requirePushedAuthorizationRequests: e.target.checked})}
                inputProps={{
                  'aria-label': t('agents:edit.advanced.security.par.label', 'Require Pushed Authorization Requests'),
                }}
              />
            }
            label={
              <Typography variant="subtitle2">
                {t('agents:edit.advanced.security.par.label', 'Require Pushed Authorization Requests')}
              </Typography>
            }
          />
          <Typography variant="caption" color="text.secondary" sx={{display: 'block', ml: '52px'}}>
            {flags.isParDisabledByGrants ? (
              <Trans
                i18nKey="agents:edit.advanced.security.par.notApplicable"
                defaults="Pushed Authorization Requests only apply to the <code>authorization_code</code> grant. Turn that on to enable this setting."
                components={codeComponents}
              />
            ) : (
              t(
                'agents:edit.advanced.security.par.hint',
                'Require this agent to push its authorization request to the PAR endpoint before redirecting a user to sign in.',
              )
            )}
          </Typography>
        </Box>

        <Divider />

        <FormControl fullWidth size="small">
          <FormLabel htmlFor="agent_default_audience">
            {t('applications:edit.advanced.audience.label', 'Default audience (aud)')}
          </FormLabel>
          <TextField
            id="agent_default_audience"
            size="small"
            fullWidth
            value={oauth2Config.token?.accessToken?.defaultAudience ?? ''}
            disabled={!isEditable}
            onChange={(e) => handleAudienceChange(e.target.value.trim())}
            inputProps={{maxLength: 2048}}
            placeholder={t('applications:edit.advanced.audience.placeholder', 'e.g. https://api.example.com')}
            helperText={t('applications:edit.advanced.audience.hint', 'Leave empty to use the {{entity}} client ID.', {
              entity: 'agent',
            })}
          />
        </FormControl>
      </Stack>
    </SettingsCard>
  );
}
