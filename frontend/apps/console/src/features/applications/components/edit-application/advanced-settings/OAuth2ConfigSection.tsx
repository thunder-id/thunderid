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
} from '@wso2/oxygen-ui';
import {Lock} from '@wso2/oxygen-ui-icons-react';
import {useTranslation} from 'react-i18next';
import type {ApplicationTemplate} from '../../../models/application-templates';
import {OAuth2ResponseTypes, TokenEndpointAuthMethods, type OAuth2Config} from '../../../models/oauth';
import {getGrantTypeLabel} from '../../../utils/getGrantTypeLabel';
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
}

function OAuth2Logo() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="36"
      height="36"
      viewBox="0 0 256 256"
      preserveAspectRatio="xMidYMid"
      aria-label="OAuth2"
      role="img"
    >
      <path
        d="M118.923 0.371C175.484-3.55 212.987 24.128 234.43 57.852c10.752 16.908 21.301 42.59 21.35 68.976.052 28.67-9.236 53.497-21.35 71.713-12.451 18.724-28.555 34.188-48.721 44.342-33.71 16.974-80.82 17.438-111.676.095-34.365-18.2-60.786-44.18-70.618-89.231C-.52 136.717-.972 113.422 4.51 93.435c1.286-4.689 3.623-9.623 5.474-14.234 8.892-22.154 24.028-41.419 42.699-54.743 7.502-5.354 16.604-10.679 24.634-14.233C87.31 5.802 104.132 1.397 118.923.371Z"
        fill="#FFFFFF"
      />
      <path
        d="M226.212 130.016c0 53.456-43.335 96.788-96.79 96.788-53.456 0-96.79-43.332-96.79-96.788 0-53.455 43.334-96.79 96.79-96.79 53.455 0 96.79 43.335 96.79 96.79Z"
        fill="#000000"
      />
      <path
        d="M118.923 0.371C175.484-3.55 212.987 24.128 234.43 57.852c10.752 16.908 21.301 42.59 21.35 68.976.052 28.67-9.236 53.497-21.35 71.713-12.451 18.724-28.555 34.188-48.721 44.342-33.71 16.974-80.82 17.438-111.676.095-34.365-18.2-60.786-44.18-70.618-89.231C-.52 136.717-.972 113.422 4.51 93.435c1.286-4.689 3.623-9.623 5.474-14.234 8.892-22.154 24.028-41.419 42.699-54.743 7.502-5.354 16.604-10.679 24.634-14.233C87.31 5.802 104.132 1.397 118.923.371ZM99.762 9.678C78.754 15.125 63.35 24.883 49.946 35.407 30.619 50.583 18.298 71.76 11.079 97.267c-8.031 28.377-2.872 62.716 8.212 84.304 11.494 22.388 26.927 39.767 48.721 52.006 21.203 11.907 51.025 21.857 81.115 19.334 27.621-5.213 48.6-17.246 65.69-35.036 22.866-23.8 41.244-67.64 29.015-113.865-3.27-12.354-7.543-25.23-14.234-36.13-3.615-5.889-8.92-11.682-14.234-17.518C185.308 27.69 152.893 8.71 111.206 11.395c-7.933.51-14.905 1.33-22.444 3.284Z"
        fill="#000000"
      />
      <path
        d="M209.796 45.261c-10.066-10.066-22.442-19.137-37.225-24.635-15.323-5.698-32.657-10.501-53.101-8.759-32.566 2.776-55.431 17.387-72.809 34.488-12.172 11.978-21.328 26.656-27.372 43.794-8.874 25.166-8.335 54.869 1.642 80.473 8.511 21.84 22.44 39.455 42.152 53.101 18.53 12.828 45.126 22.778 74.998 20.255 36.383-3.073 63.003-21.738 80.472-43.246 4.47-5.502 8.914-11.722 11.496-19.16 2.53-5.359 3.768-9.002 4.927-10.948 4.852-10.872 7.422-22.344 8.211-35.035 2.425-39.037-12.595-69.528-33.393-90.328ZM199.074 164.193c-.07.149-.131.287-.204.44-1.805 5.165-4.914 9.484-8.039 13.305-12.217 14.935-30.832 27.896-56.275 30.03-20.89 1.751-39.489-5.157-52.447-14.065-11.26-7.741-19.82-17.323-25.822-28.839-.366-.693-3.852-8.724-3.862-9.077-4.755-14.749-5.168-38.466-.287-52.698 3.257-9.499 8.686-19.776 15.876-27.919 9.927-11.243 19.86-18.448 35.583-23.54 7.179-2.325 13.605-4.692 22.444-4.926 21.556-.574 43.505 9.104 55.291 19.707 10.83 9.744 23.174 28.724 26.277 46.531 3.193 18.328-.215 36.223-8.534 51.05Z"
        fill="#000000"
      />
      <g transform="translate(83.627 76.373)">
        <path
          d="M77.981 105.401c-5.308 0-9.966-3.361-11.591-8.365L60.478 79.023H30.903L25.41 96.872c-1.658 5.1-6.351 8.515-11.686 8.515-1.281 0-2.553-.203-3.781-.601-6.377-1.915-9.997-8.778-8.043-15.292L26.862 10.423C28.483 5.393 33.254 1.884 38.467 1.884h13.274c5.242 0 10.013 3.449 11.602 8.387l26.253 79.042c2.099 6.462-1.376 13.4-7.744 15.472-1.259.409-2.56.615-3.87.615Z"
          fill="#FFFFFF"
        />
        <path
          d="M77.981 103.694c-4.755 0-8.92-3.006-10.374-7.48L61.694 78.198l-.29-.881H30.903l-.945.003-.278.904-5.493 17.849c-1.48 4.552-5.68 7.611-10.463 7.611-1.145 0-2.284-.181-3.386-.539-5.726-1.719-8.963-7.869-7.212-13.706L28.082 10.381C29.53 5.886 33.811 2.737 38.467 2.737h13.274c4.688 0 8.966 3.092 10.384 7.499l26.256 79.053c1.879 5.784-1.232 11.998-6.925 13.851-1.13.367-2.299.553-3.474.553ZM77.981 106.254c1.444 0 2.879-.228 4.265-.678 7.046-2.293 10.882-9.956 8.567-17.085L64.558 9.441C62.8 3.983 57.537.177 51.74.177H38.467C32.695.177 27.434 4.048 25.643 9.603L.679 88.682c-2.16 7.199 1.832 14.782 8.895 16.903 1.33.432 2.735.655 4.15.655 5.892 0 11.075-3.774 12.903-9.4l5.499-17.868-1.223.904h29.574l-1.216-.881 5.912 18.013c1.796 5.528 6.946 9.245 12.807 9.245Z"
          fill="#000000"
        />
      </g>
      <g transform="translate(61.44 19.2)" fill="#FFFFFF">
        <path d="M2.134 33.858L2.103 33.815C-1.01 29.549-.097 23.581 4.488 20.235c4.584-3.345 10.474-2.406 13.587 1.86l.03.042c3.113 4.266 2.2 10.235-2.385 13.58-4.584 3.346-10.474 2.407-13.586-1.859Zm11.8-8.612L13.903 25.204C12.339 23.06 9.426 22.323 7.156 23.98c-2.25 1.642-2.425 4.567-.861 6.71l.031.043c1.565 2.143 4.477 2.88 6.727 1.239 2.271-1.658 2.446-4.582.881-6.726Z" />
        <path d="M32.147 5.895L36.825 4.394 49.94 19.641l-5.204 1.669-2.279-2.718-6.755 2.167-.248 3.529-5.103 1.638L32.147 5.895Zm7.697 9.37L36.295 10.91l-.379 5.614 3.928-1.26Z" />
        <path d="M58.366 10.482L58.407.155l5.176.02-.04 10.242c-.01 2.654 1.324 3.92 3.374 3.929 2.05.008 3.395-1.195 3.404-3.77L70.363.224l5.176.02-.04 10.195c-.024 5.938-3.424 8.526-8.653 8.505-5.229-.021-8.503-2.689-8.48-8.443Z" />
        <path d="M94.233 8.678L88.956 7.067l1.304-4.272L105.69 7.504l-1.295 4.272-5.277-1.61-4.035 13.319-4.875-1.488 4.065-13.319Z" />
        <path d="M119.472 13.465l4.243 2.824-3.814 5.731 5.447 3.625 3.814-5.731 4.243 2.824-10.19 15.313-4.244-2.824 3.872-5.819-5.447-3.625-3.872 5.819-4.243-2.824 10.19-15.313Z" />
      </g>
      <g transform="translate(65.28 196.267)" fill="#FFFFFF">
        <path d="M130.622 3.79l.033.042c3.274 4.144 2.589 10.144-1.864 13.662-4.454 3.518-10.375 2.805-13.649-1.339l-.033-.042c-3.274-4.145-2.588-10.145 1.865-13.663 4.453-3.518 10.374-2.805 13.648 1.34Zm-11.464 9.057l.032.04c1.645 2.083 4.584 2.708 6.79.966 2.185-1.727 2.249-4.656.604-6.739l-.032-.04c-1.645-2.083-4.584-2.708-6.769-.982-2.206 1.743-2.27 4.671-.625 6.755Z" />
        <path d="M101.848 32.931L97.24 34.636 83.467 19.982l5.125-1.897 2.397 2.615 6.654-2.463.091-3.536 5.027-1.861-.913 20.09Zm-8.102-9.02l3.737 4.192.132-5.624-3.87 1.432Z" />
        <path d="M75.472 29.434l.455 10.316-5.171.228-.451-10.211c-.117-2.651-1.511-3.852-3.559-3.762-2.047.091-3.333 1.357-3.219 3.929l.456 10.343-5.171.228-.45-10.185c-.261-5.932 3.011-8.681 8.235-8.912 5.224-.23 8.622 2.277 8.876 8.026Z" />
        <path d="M39.54 32.807L44.867 34.245 43.703 38.558 28.125 34.353l1.164-4.313 5.328 1.438 3.63-13.447 4.922 1.33-3.63 13.446Z" />
        <path d="M14.357 29.059L9.984 26.439l3.539-5.906-5.612-3.364L4.373 23.075 0 20.454l9.457-15.777 4.372 2.62-3.593 5.996 5.612 3.364 3.593-5.996 4.373 2.62-9.457 15.778Z" />
      </g>
    </svg>
  );
}

/**
 * Section component for displaying and editing OAuth2 configuration.
 *
 * Shows:
 * - Allowed grant types (multi-select dropdown from OIDC discovery)
 * - Allowed response types (multi-select dropdown from OIDC discovery)
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
}: OAuth2ConfigSectionProps) {
  const {t} = useTranslation();
  const {discovery} = useThunderID();

  if (!oauth2Config) return null;

  const availableGrantTypes = discovery?.wellKnown?.grant_types_supported ?? [];
  const availableResponseTypes = discovery?.wellKnown?.response_types_supported ?? [];
  const availableTokenEndpointAuthMethods: string[] =
    (discovery as {wellKnown?: OidcDiscovery} | undefined)?.wellKnown?.token_endpoint_auth_methods_supported ?? [];
  const isEditable = Boolean(onOAuth2ConfigChange) && !disabled;

  // Constraint helpers
  const constraint = <K extends keyof OAuth2Config>(field: K) => oauth2Constraints?.[field];

  const isPublicClientLocked = constraint('publicClient')?.readOnly ?? false;
  const isPkceLocked = constraint('pkceRequired')?.readOnly ?? false;
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
  } = flags;

  return (
    <SettingsCard
      title={t('applications:edit.advanced.labels.oauth2Config')}
      description={t(
        'applications:edit.advanced.oauth2Config.intro',
        'Configure OAuth 2.0 settings for this {{entity}}.',
        {entity: 'application'},
      )}
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
              'OAuth 2.0 flows this {{entity}} can use (e.g., authorization_code, client_credentials, refresh_token).',
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

        {/* Token Endpoint Auth Method */}
        <FormControl fullWidth size="small">
          <FormLabel htmlFor="token_endpoint_auth_method">
            {t('applications:edit.advanced.labels.tokenEndpointAuthMethod', 'Token Endpoint Auth Method')}
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
              ? t(
                  'applications:edit.advanced.tokenEndpointAuthMethod.lockedHint',
                  'Locked to "none" because the client is public.',
                )
              : t(
                  'applications:edit.advanced.tokenEndpointAuthMethod.hint',
                  'Defines how the client authenticates at the token endpoint.',
                )}
          </Typography>
        </FormControl>

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
      </Stack>
    </SettingsCard>
  );
}
